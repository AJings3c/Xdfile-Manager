package system

import "os/exec"

type ShellClipboardFile struct {
	Name  string
	IsDir bool
	Size  int64
}

type CommandMenuListEncoding int

const (
	CommandMenuListEncodingOEM CommandMenuListEncoding = iota
	CommandMenuListEncodingANSI
	CommandMenuListEncodingUTF8
	CommandMenuListEncodingUTF16LE
)

func ReadClipboardPaths() ([]string, error) {
	return readClipboardPaths()
}

func ReadClipboardCut() (bool, error) {
	return readClipboardCut()
}

func WriteClipboardPaths(paths []string, cut bool) error {
	return writeClipboardPaths(paths, cut)
}

func WriteClipboardText(text string) error {
	return writeClipboardText(text)
}

func ReadClipboardVirtualFiles() ([]ShellClipboardFile, error) {
	return readClipboardVirtualFiles()
}

func CopyClipboardVirtualFile(index int, expectedName string, targetPath string) error {
	return copyClipboardVirtualFile(index, expectedName, targetPath)
}

func OpenPath(path string) error {
	return openPath(path)
}

func OpenPathDirect(path string) error {
	return openPath(path)
}

func ShowProperties(path string) error {
	return showProperties(path)
}

func ShowContextMenu(paths []string) error {
	return showContextMenu(paths)
}

func ConfigureManagedExternalCommand(cmd *exec.Cmd) {
	configureManagedExternalCommand(cmd)
}

func CommandMenuShortPath(path string) (string, error) {
	return commandMenuShortPath(path)
}

func CommandMenuEncodeWithCodePage(text string, encoding CommandMenuListEncoding) ([]byte, error) {
	return commandMenuEncodeWithCodePage(text, encoding)
}
