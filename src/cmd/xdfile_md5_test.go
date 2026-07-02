package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestXdfileMD5ChecksumSmallFileShowsResult(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "hello.txt")
	mustWriteFile(t, filePath, "hello")

	m := newMD5TestModel(t, workspace, filePath)
	cmd := m.startMD5ChecksumForSelection()
	done := firstBatchMsg[xdfileMD5ChecksumDoneMsg](t, cmd)
	if done.Err != nil {
		t.Fatalf("md5 checksum failed: %v", done.Err)
	}
	if done.Checksum != "5d41402abc4b2a76b9719d911017c592" {
		t.Fatalf("checksum = %s", done.Checksum)
	}
	if done.Size != 5 {
		t.Fatalf("size = %d, want 5", done.Size)
	}

	m.applyMD5ChecksumDone(done)
	if m.modal.Kind != xdfileModalText || m.modal.Action != xdfileActionMD5Result {
		t.Fatalf("expected md5 result modal, got %#v", m.modal)
	}
	if !strings.Contains(m.modal.Text, done.Checksum) {
		t.Fatalf("result modal missing checksum:\n%s", m.modal.Text)
	}
}

func TestXdfileMD5ChecksumEmptyFile(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "empty.txt")
	mustWriteFile(t, filePath, "")

	checksum, size, err := xdfileComputeMD5FileContext(context.Background(), filePath, nil)
	if err != nil {
		t.Fatalf("empty file md5 failed: %v", err)
	}
	if checksum != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Fatalf("empty checksum = %s", checksum)
	}
	if size != 0 {
		t.Fatalf("empty size = %d, want 0", size)
	}
}

func TestXdfileMD5ChecksumCanceledBeforeLargeRead(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "large.bin")
	data := strings.Repeat("0123456789abcdef", 128*1024)
	mustWriteFile(t, filePath, data)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := xdfileComputeMD5FileContext(ctx, filePath, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled md5, got %v", err)
	}
}

func TestXdfileMD5ChecksumReadErrorIsExplicit(t *testing.T) {
	originalOpen := xdfileMD5OpenFileFunc
	xdfileMD5OpenFileFunc = func(string) (*os.File, error) {
		return nil, errors.New("permission denied")
	}
	t.Cleanup(func() {
		xdfileMD5OpenFileFunc = originalOpen
	})

	_, _, err := xdfileComputeMD5FileContext(context.Background(), "/tmp/nope", nil)
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestXdfileMD5ChecksumResultCanBeCopied(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "hello.txt")
	mustWriteFile(t, filePath, "hello")

	m := newMD5TestModel(t, workspace, filePath)
	m.applyMD5ChecksumDone(xdfileMD5ChecksumDoneMsg{
		Path:     filePath,
		Name:     "hello.txt",
		Checksum: "5d41402abc4b2a76b9719d911017c592",
		Size:     5,
	})

	originalWrite := xdfileWriteClipboardTextFunc
	var copied string
	xdfileWriteClipboardTextFunc = func(text string) error {
		copied = text
		return nil
	}
	t.Cleanup(func() {
		xdfileWriteClipboardTextFunc = originalWrite
	})

	cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cmd == nil {
		t.Fatal("copying md5 result should return a clipboard command")
	}
	msg := cmd()
	if result, ok := msg.(xdfileClipboardTextWriteResultMsg); !ok || result.Err != nil {
		t.Fatalf("unexpected clipboard result: %#v", msg)
	}
	if copied != "5d41402abc4b2a76b9719d911017c592" {
		t.Fatalf("copied checksum = %q", copied)
	}
}

func TestXdfileMD5ChecksumRejectsDirectoryAndRemote(t *testing.T) {
	m := baselineRenderModel(80, 24)
	m.computeLayout()
	m.panels[0].Entries = []xdfileEntry{
		{Name: "dir", Path: "/tmp/project/dir", IsDir: true},
	}
	m.panels[0].Cursor = 0
	if cmd := m.startMD5ChecksumForSelection(); cmd != nil {
		t.Fatal("directory md5 should not start")
	}
	if !strings.Contains(m.statusText, "files only") {
		t.Fatalf("directory status = %q", m.statusText)
	}

	m.panels[0].Entries = []xdfileEntry{
		{Name: "remote.log", Path: "xdssh://prod/var/log/remote.log"},
	}
	if cmd := m.startMD5ChecksumForSelection(); cmd != nil {
		t.Fatal("remote md5 should not start")
	}
	if !strings.Contains(m.statusText, "Remote MD5") {
		t.Fatalf("remote status = %q", m.statusText)
	}
}

func newMD5TestModel(t *testing.T, workspace string, filePath string) *xdfileModel {
	t.Helper()
	m := newPinTestModel(workspace, filepath.Join(workspace, "pins.json"))
	m.layout.panelRects[0] = xdfileRect{x: 0, y: 2, w: 60, h: 12}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload panel: %v", err)
	}
	rows := m.panels[0].visibleRows(m.layout.panelRects[0].h)
	if !m.panels[0].focusPath(filePath, rows) {
		t.Fatalf("test file was not in panel: %s", filePath)
	}
	return m
}
