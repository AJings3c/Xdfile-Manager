//go:build windows

package system

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	shellClipboardDVAspectContent = 1

	shellClipboardTymedHGlobal  = 0x0001
	shellClipboardTymedIStream  = 0x0004
	shellClipboardTymedIStorage = 0x0008

	shellClipboardFDAttributes = 0x00000004
	shellClipboardFDFileSize   = 0x00000040
	shellClipboardFDWriteTime  = 0x00000020

	shellClipboardFileAttributeDirectory = 0x00000010

	shellClipboardDVFormatEtc uintptr = 0x80040064
	shellClipboardDVETymed    uintptr = 0x80040069
)

var (
	ole32 = windows.NewLazySystemDLL("ole32.dll")

	procOleInitialize    = ole32.NewProc("OleInitialize")
	procOleUninitialize  = ole32.NewProc("OleUninitialize")
	procOleGetClipboard  = ole32.NewProc("OleGetClipboard")
	procReleaseStgMedium = ole32.NewProc("ReleaseStgMedium")
)

type shellClipboardDataObject struct {
	Vtbl *shellClipboardDataObjectVtbl
}

type shellClipboardDataObjectVtbl struct {
	QueryInterface        uintptr
	AddRef                uintptr
	Release               uintptr
	GetData               uintptr
	GetDataHere           uintptr
	QueryGetData          uintptr
	GetCanonicalFormatEtc uintptr
	SetData               uintptr
	EnumFormatEtc         uintptr
	DAdvise               uintptr
	DUnadvise             uintptr
	EnumDAdvise           uintptr
}

type shellClipboardStream struct {
	Vtbl *shellClipboardStreamVtbl
}

type shellClipboardStreamVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	Read           uintptr
	Write          uintptr
	Seek           uintptr
	SetSize        uintptr
	CopyTo         uintptr
	Commit         uintptr
	Revert         uintptr
	LockRegion     uintptr
	UnlockRegion   uintptr
	Stat           uintptr
	Clone          uintptr
}

type shellClipboardFormatEtc struct {
	CfFormat uint16
	Ptd      uintptr
	DwAspect uint32
	Lindex   int32
	Tymed    uint32
}

type shellClipboardStgMedium struct {
	Tymed          uint32
	Data           unsafe.Pointer
	PUnkForRelease unsafe.Pointer
}

type shellClipboardFileDescriptorW struct {
	DwFlags          uint32
	Clsid            windows.GUID
	SizelCx          int32
	SizelCy          int32
	PointlX          int32
	PointlY          int32
	DwFileAttributes uint32
	FtCreationTime   windows.Filetime
	FtLastAccessTime windows.Filetime
	FtLastWriteTime  windows.Filetime
	NFileSizeHigh    uint32
	NFileSizeLow     uint32
	CFileName        [windows.MAX_PATH]uint16
}

type shellClipboardDescriptor struct {
	File ShellClipboardFile
	Time time.Time
}

type shellClipboardGetDataError struct {
	Operation string
	HR        uintptr
}

func (err shellClipboardGetDataError) Error() string {
	return fmt.Sprintf("%s failed with HRESULT 0x%08X", err.Operation, uint32(err.HR))
}

func shellClipboardMissingFormat(err error) bool {
	getDataErr, ok := err.(shellClipboardGetDataError)
	if !ok {
		return false
	}
	hr := uint32(getDataErr.HR)
	return hr == uint32(shellClipboardDVFormatEtc) || hr == uint32(shellClipboardDVETymed)
}

func readClipboardVirtualFiles() ([]ShellClipboardFile, error) {
	descriptors, err := readShellClipboardDescriptors()
	if err != nil || len(descriptors) == 0 {
		return nil, err
	}

	files := make([]ShellClipboardFile, 0, len(descriptors))
	for _, descriptor := range descriptors {
		files = append(files, descriptor.File)
	}
	return files, nil
}

func copyClipboardVirtualFile(index int, expectedName string, targetPath string) error {
	targetPath = filepath.Clean(strings.TrimSpace(targetPath))
	if targetPath == "" || targetPath == "." {
		return fmt.Errorf("empty shell clipboard target")
	}
	expectedName, err := cleanShellClipboardVirtualName(expectedName)
	if err != nil {
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	oleInitialized, err := initializeShellClipboardOLE()
	if err != nil {
		return err
	}
	if oleInitialized {
		defer procOleUninitialize.Call()
	}

	dataObject, err := shellClipboardDataObjectFromClipboard()
	if err != nil {
		return err
	}
	defer dataObject.release()

	descriptors, err := shellClipboardDescriptorsFromDataObject(dataObject)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(descriptors) {
		return fmt.Errorf("shell clipboard file index out of range: %d", index)
	}

	descriptor := descriptors[index]
	if !strings.EqualFold(filepath.Clean(descriptor.File.Name), expectedName) {
		return fmt.Errorf("Shell clipboard changed before paste: expected %s, got %s", expectedName, descriptor.File.Name)
	}
	if descriptor.File.IsDir {
		if err := os.MkdirAll(targetPath, 0o755); err != nil {
			return err
		}
		return applyShellClipboardFileTime(targetPath, descriptor.Time)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	medium, err := dataObject.getData(
		shellClipboardFormatFileContents(),
		int32(index),
		shellClipboardTymedIStream|shellClipboardTymedHGlobal|shellClipboardTymedIStorage,
	)
	if err != nil {
		return err
	}
	defer releaseShellClipboardStgMedium(&medium)

	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		_ = targetFile.Close()
		if cleanup {
			_ = os.Remove(targetPath)
		}
	}()

	switch medium.Tymed {
	case shellClipboardTymedIStream:
		if err := writeShellClipboardStreamToFile((*shellClipboardStream)(medium.Data), targetFile); err != nil {
			return err
		}
	case shellClipboardTymedHGlobal:
		if err := writeShellClipboardHGlobalToFile(medium.Data, targetFile); err != nil {
			return err
		}
	case shellClipboardTymedIStorage:
		return fmt.Errorf("Shell clipboard storage-backed directories are not supported yet: %s", descriptor.File.Name)
	default:
		return fmt.Errorf("unsupported Shell clipboard medium 0x%X for %s", medium.Tymed, descriptor.File.Name)
	}

	if err := targetFile.Close(); err != nil {
		return err
	}
	if err := applyShellClipboardFileTime(targetPath, descriptor.Time); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func readShellClipboardDescriptors() ([]shellClipboardDescriptor, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	oleInitialized, err := initializeShellClipboardOLE()
	if err != nil {
		return nil, err
	}
	if oleInitialized {
		defer procOleUninitialize.Call()
	}

	dataObject, err := shellClipboardDataObjectFromClipboard()
	if err != nil {
		return nil, err
	}
	defer dataObject.release()

	return shellClipboardDescriptorsFromDataObject(dataObject)
}

func initializeShellClipboardOLE() (bool, error) {
	hr, _, _ := procOleInitialize.Call(0)
	if hr == 0 || hr == uintptr(sFalse) {
		return true, nil
	}
	return false, shellHRESULTError("OleInitialize", hr)
}

func shellClipboardDataObjectFromClipboard() (*shellClipboardDataObject, error) {
	var dataObject *shellClipboardDataObject
	hr, _, _ := procOleGetClipboard.Call(uintptr(unsafe.Pointer(&dataObject)))
	if shellHRESULTFailed(hr) {
		return nil, shellHRESULTError("OleGetClipboard", hr)
	}
	if dataObject == nil {
		return nil, fmt.Errorf("Windows Shell clipboard is empty")
	}
	return dataObject, nil
}

func shellClipboardDescriptorsFromDataObject(dataObject *shellClipboardDataObject) ([]shellClipboardDescriptor, error) {
	medium, err := dataObject.getData(shellClipboardFormatFileGroupDescriptorW(), -1, shellClipboardTymedHGlobal)
	if err != nil {
		if shellClipboardMissingFormat(err) {
			return nil, nil
		}
		return nil, err
	}
	defer releaseShellClipboardStgMedium(&medium)

	if medium.Tymed != shellClipboardTymedHGlobal || medium.Data == nil {
		return nil, fmt.Errorf("Windows Shell clipboard has no FileGroupDescriptorW data")
	}

	data, _, err := clipboardMemoryBytes(uintptr(medium.Data))
	if err != nil {
		return nil, err
	}
	if len(data) < 4 {
		return nil, fmt.Errorf("invalid FileGroupDescriptorW data")
	}

	count := int(binary.LittleEndian.Uint32(data[:4]))
	descriptorSize := int(unsafe.Sizeof(shellClipboardFileDescriptorW{}))
	if count < 0 || count > (len(data)-4)/descriptorSize {
		return nil, fmt.Errorf("invalid FileGroupDescriptorW item count")
	}

	descriptors := make([]shellClipboardDescriptor, 0, count)
	for index := 0; index < count; index++ {
		offset := 4 + index*descriptorSize
		var raw shellClipboardFileDescriptorW
		copy(
			unsafe.Slice((*byte)(unsafe.Pointer(&raw)), descriptorSize),
			data[offset:offset+descriptorSize],
		)

		name, err := cleanShellClipboardVirtualName(windows.UTF16ToString(raw.CFileName[:]))
		if err != nil {
			return nil, err
		}
		descriptor := shellClipboardDescriptor{
			File: ShellClipboardFile{
				Name: name,
				IsDir: raw.DwFlags&shellClipboardFDAttributes != 0 &&
					raw.DwFileAttributes&shellClipboardFileAttributeDirectory != 0,
			},
		}
		if raw.DwFlags&shellClipboardFDFileSize != 0 {
			descriptor.File.Size = int64(raw.NFileSizeHigh)<<32 | int64(raw.NFileSizeLow)
		}
		if raw.DwFlags&shellClipboardFDWriteTime != 0 && !shellClipboardFiletimeZero(raw.FtLastWriteTime) {
			descriptor.Time = time.Unix(0, raw.FtLastWriteTime.Nanoseconds())
		}
		descriptors = append(descriptors, descriptor)
	}
	return descriptors, nil
}

func shellClipboardFormatFileGroupDescriptorW() uint32 {
	format, _ := registerClipboardFormat("FileGroupDescriptorW")
	return format
}

func shellClipboardFormatFileContents() uint32 {
	format, _ := registerClipboardFormat("FileContents")
	return format
}

func cleanShellClipboardVirtualName(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\x00", ""))
	name = strings.ReplaceAll(name, "/", string(os.PathSeparator))
	name = strings.ReplaceAll(name, "\\", string(os.PathSeparator))
	name = filepath.Clean(name)
	if name == "" || name == "." || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("invalid Shell clipboard file name: %q", name)
	}
	for _, part := range strings.Split(name, string(os.PathSeparator)) {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("unsafe Shell clipboard file name: %q", name)
		}
	}
	return name, nil
}

func shellClipboardFiletimeZero(value windows.Filetime) bool {
	return value.LowDateTime == 0 && value.HighDateTime == 0
}

func applyShellClipboardFileTime(path string, modTime time.Time) error {
	if modTime.IsZero() {
		return nil
	}
	return os.Chtimes(path, modTime, modTime)
}

func (dataObject *shellClipboardDataObject) getData(format uint32, index int32, tymed uint32) (shellClipboardStgMedium, error) {
	var medium shellClipboardStgMedium
	if dataObject == nil {
		return medium, fmt.Errorf("missing Windows Shell clipboard data object")
	}
	if format == 0 || format > 0xFFFF {
		return medium, fmt.Errorf("invalid Windows Shell clipboard format: %d", format)
	}

	formatEtc := shellClipboardFormatEtc{
		CfFormat: uint16(format),
		DwAspect: shellClipboardDVAspectContent,
		Lindex:   index,
		Tymed:    tymed,
	}
	hr, _, _ := syscall.SyscallN(
		dataObject.Vtbl.GetData,
		uintptr(unsafe.Pointer(dataObject)),
		uintptr(unsafe.Pointer(&formatEtc)),
		uintptr(unsafe.Pointer(&medium)),
	)
	if shellHRESULTFailed(hr) {
		return medium, shellClipboardGetDataError{Operation: "IDataObject.GetData", HR: hr}
	}
	return medium, nil
}

func (dataObject *shellClipboardDataObject) release() {
	if dataObject == nil {
		return
	}
	syscall.SyscallN(dataObject.Vtbl.Release, uintptr(unsafe.Pointer(dataObject)))
}

func releaseShellClipboardStgMedium(medium *shellClipboardStgMedium) {
	if medium == nil || medium.Tymed == 0 {
		return
	}
	procReleaseStgMedium.Call(uintptr(unsafe.Pointer(medium)))
	*medium = shellClipboardStgMedium{}
}

func writeShellClipboardStreamToFile(stream *shellClipboardStream, target *os.File) error {
	if stream == nil {
		return fmt.Errorf("missing Shell clipboard stream")
	}

	buffer := make([]byte, 1024*1024)
	for {
		var read uint32
		hr, _, _ := syscall.SyscallN(
			stream.Vtbl.Read,
			uintptr(unsafe.Pointer(stream)),
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(len(buffer)),
			uintptr(unsafe.Pointer(&read)),
		)
		if shellHRESULTFailed(hr) {
			return shellHRESULTError("IStream.Read", hr)
		}
		if read == 0 {
			return nil
		}
		if _, err := target.Write(buffer[:read]); err != nil {
			return err
		}
	}
}

func writeShellClipboardHGlobalToFile(handle unsafe.Pointer, target *os.File) error {
	if handle == nil {
		return nil
	}

	ptr, _, err := procGlobalLock.Call(uintptr(handle))
	if ptr == 0 {
		return err
	}
	defer procGlobalUnlock.Call(uintptr(handle))

	size, _, err := procGlobalSize.Call(uintptr(handle))
	if size == 0 {
		return err
	}

	const chunkSize = 1024 * 1024
	buffer := make([]byte, chunkSize)
	for offset := uintptr(0); offset < size; {
		n := min(chunkSize, int(size-offset))
		copyFromClipboardMemory(buffer[:n], ptr+offset)
		written, writeErr := target.Write(buffer[:n])
		offset += uintptr(written)
		if writeErr != nil {
			return writeErr
		}
		if written != n {
			return io.ErrShortWrite
		}
	}
	return nil
}
