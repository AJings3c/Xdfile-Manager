package cmd

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const xdfileMD5ReadChunkSize = 256 * 1024

var xdfileMD5OpenFileFunc = os.Open

func (m *xdfileModel) startMD5ChecksumForSelection() tea.Cmd {
	entry, ok := m.md5SelectionEntry()
	if !ok {
		return nil
	}
	if m.backgroundTaskBusy {
		m.setStatus("Wait for the current background task to finish")
		return nil
	}
	return m.startMD5Checksum(entry)
}

func (m *xdfileModel) md5SelectionEntry() (xdfileEntry, bool) {
	panel := &m.panels[m.activePanel]
	entry, ok := panel.selected()
	if !ok || entry.IsParent {
		m.setStatus("Select a local file to calculate MD5")
		return xdfileEntry{}, false
	}
	if entry.IsDir {
		m.setStatus("MD5 checksum is available for files only")
		return xdfileEntry{}, false
	}
	if xdfileIsNetBoxPath(entry.Path) {
		m.setStatus("Remote MD5 is unavailable; copy the file locally first")
		return xdfileEntry{}, false
	}
	return entry, true
}

func (m *xdfileModel) startMD5Checksum(entry xdfileEntry) tea.Cmd {
	path := strings.TrimSpace(entry.Path)
	name := entry.Name
	if name == "" {
		name = filepath.Base(path)
	}
	return m.startFileOperationTask(fmt.Sprintf("Computing MD5 for %s", name), func(ctx context.Context, progress *xdfileFileOperationProgress) tea.Msg {
		checksum, size, err := xdfileComputeMD5FileContext(ctx, path, progress)
		return xdfileMD5ChecksumDoneMsg{
			Path:     path,
			Name:     name,
			Checksum: checksum,
			Size:     size,
			Err:      err,
		}
	})
}

func (m *xdfileModel) applyMD5ChecksumDone(msg xdfileMD5ChecksumDoneMsg) tea.Cmd {
	m.finishFileOperationTask()
	if msg.Err != nil {
		if errors.Is(msg.Err, context.Canceled) {
			m.setStatus("MD5 checksum canceled")
			return nil
		}
		m.setStatusErr(fmt.Errorf("MD5 checksum failed: %w", msg.Err))
		return nil
	}

	text := xdfileMD5ResultText(msg)
	m.openTextModalWithAction(
		xdfileActionMD5Result,
		"MD5 Checksum",
		text,
		"c copy checksum | Enter/Esc close | Up/Down/PgUp/PgDn scroll",
	)
	m.modal.SourcePath = msg.Path
	m.modal.TargetPath = msg.Checksum
	m.setStatus("MD5 checksum ready for %s", msg.Name)
	return nil
}

func xdfileComputeMD5FileContext(
	ctx context.Context,
	path string,
	progress *xdfileFileOperationProgress,
) (string, int64, error) {
	if err := xdfileCheckFileOperationContext(ctx); err != nil {
		return "", 0, err
	}
	file, err := xdfileMD5OpenFileFunc(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hasher := md5.New()
	buffer := make([]byte, xdfileMD5ReadChunkSize)
	var size int64
	for {
		if err := xdfileCheckFileOperationContext(ctx); err != nil {
			return "", size, err
		}
		n, readErr := file.Read(buffer)
		if n > 0 {
			if _, err := hasher.Write(buffer[:n]); err != nil {
				return "", size, err
			}
			size += int64(n)
			progress.addBytes(n)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", size, readErr
		}
	}
	progress.addItem()
	return hex.EncodeToString(hasher.Sum(nil)), size, nil
}

func xdfileMD5ResultText(msg xdfileMD5ChecksumDoneMsg) string {
	return strings.Join([]string{
		"File: " + msg.Path,
		"Size: " + xdfileHumanSize(msg.Size),
		"MD5:  " + msg.Checksum,
		"",
		"Press c to copy the checksum.",
	}, "\n")
}

func (m *xdfileModel) copyMD5ChecksumFromModal() tea.Cmd {
	checksum := strings.TrimSpace(m.modal.TargetPath)
	if checksum == "" {
		m.setStatus("No MD5 checksum to copy")
		return nil
	}
	m.setStatus("Copied MD5 checksum")
	return func() tea.Msg {
		return xdfileClipboardTextWriteResultMsg{
			Err: xdfileWriteClipboardTextFunc(checksum),
		}
	}
}
