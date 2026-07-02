package cmd

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestXdfileCopyPathTextActions(t *testing.T) {
	originalWriteText := xdfileWriteClipboardTextFunc
	t.Cleanup(func() {
		xdfileWriteClipboardTextFunc = originalWriteText
	})

	var writes []string
	xdfileWriteClipboardTextFunc = func(text string) error {
		writes = append(writes, text)
		return nil
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd:    "/workspace",
				Cursor: 1,
				Entries: []xdfileEntry{
					{Name: "..", Path: "/", IsDir: true, IsParent: true},
					{Name: "local.txt", Path: "/workspace/local.txt"},
					{Name: "remote.log", Path: "xdssh://prod/var/log/remote.log"},
				},
			},
		},
		clipboardPaths: []string{"/keep/file.txt"},
		clipboardPath:  "/keep/file.txt",
		clipboardCut:   true,
	}

	cmd := m.executeAction(xdfileActionCopyCurrentPath)
	assertClipboardTextMsgOK(t, cmd)
	if got := writes[len(writes)-1]; got != "/workspace/local.txt" {
		t.Fatalf("current path text mismatch: %q", got)
	}

	m.panels[0].MarkedPaths = map[string]struct{}{
		"/workspace/local.txt":            {},
		"xdssh://prod/var/log/remote.log": {},
	}
	cmd = m.executeAction(xdfileActionCopySelectedPaths)
	assertClipboardTextMsgOK(t, cmd)
	if got := writes[len(writes)-1]; got != "/workspace/local.txt\nxdssh://prod/var/log/remote.log" {
		t.Fatalf("selected path text mismatch: %q", got)
	}

	m.panels[0].Cwd = "xdssh://prod/var/log"
	cmd = m.executeAction(xdfileActionCopyCurrentDirectory)
	assertClipboardTextMsgOK(t, cmd)
	if got := writes[len(writes)-1]; got != "xdssh://prod/var/log" {
		t.Fatalf("current directory text mismatch: %q", got)
	}

	if got := strings.Join(m.clipboardPaths, "\n"); got != "/keep/file.txt" {
		t.Fatalf("text copy should not change file clipboard paths: %q", got)
	}
	if !m.clipboardCut || m.clipboardPath != "/keep/file.txt" {
		t.Fatalf("text copy should not change file clipboard mode/path: cut=%v path=%q", m.clipboardCut, m.clipboardPath)
	}
}

func TestXdfileCopyPathTextNoSelectionDoesNotWrite(t *testing.T) {
	originalWriteText := xdfileWriteClipboardTextFunc
	t.Cleanup(func() {
		xdfileWriteClipboardTextFunc = originalWriteText
	})

	called := false
	xdfileWriteClipboardTextFunc = func(text string) error {
		called = true
		return nil
	}

	m := &xdfileModel{
		panels: [2]xdfilePanel{
			{
				Cwd:    "/workspace",
				Cursor: 0,
				Entries: []xdfileEntry{
					{Name: "..", Path: "/", IsDir: true, IsParent: true},
				},
			},
		},
	}

	if cmd := m.executeAction(xdfileActionCopyCurrentPath); cmd != nil {
		t.Fatal("copy current path should not produce a command without a selected file")
	}
	if called {
		t.Fatal("clipboard text writer should not be called without a selected file")
	}
	if !strings.Contains(m.statusText, "Select") {
		t.Fatalf("expected selection status, got %q", m.statusText)
	}
}

func TestXdfileCopiedPathTextDoesNotOverrideFileClipboardPayload(t *testing.T) {
	originalWriteText := xdfileWriteClipboardTextFunc
	originalReadPaths := xdfileReadClipboardPathsFunc
	originalReadCut := xdfileReadClipboardCutFunc
	originalReadVirtual := xdfileReadClipboardVirtualFilesFunc
	t.Cleanup(func() {
		xdfileWriteClipboardTextFunc = originalWriteText
		xdfileReadClipboardPathsFunc = originalReadPaths
		xdfileReadClipboardCutFunc = originalReadCut
		xdfileReadClipboardVirtualFilesFunc = originalReadVirtual
	})

	xdfileWriteClipboardTextFunc = func(text string) error {
		return nil
	}
	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{"/workspace/local.txt"}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}
	xdfileReadClipboardVirtualFilesFunc = func() ([]xdfileShellClipboardFile, error) {
		return nil, nil
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd:    "/workspace",
				Cursor: 0,
				Entries: []xdfileEntry{
					{Name: "local.txt", Path: "/workspace/local.txt"},
				},
			},
		},
		clipboardPaths: []string{"/keep/file.txt"},
		clipboardPath:  "/keep/file.txt",
		clipboardCut:   true,
	}

	cmd := m.executeAction(xdfileActionCopyCurrentPath)
	assertClipboardTextMsgOK(t, cmd)

	paths, cut, err := m.currentClipboardPayload()
	if err != nil {
		t.Fatalf("current clipboard payload failed: %v", err)
	}
	if got := strings.Join(paths, "\n"); got != "/keep/file.txt" {
		t.Fatalf("text path should not override internal file clipboard: %q", got)
	}
	if !cut {
		t.Fatal("internal cut mode should be preserved")
	}
}

func TestXdfileCopyPathTextWriteErrorUpdatesStatus(t *testing.T) {
	originalWriteText := xdfileWriteClipboardTextFunc
	t.Cleanup(func() {
		xdfileWriteClipboardTextFunc = originalWriteText
	})

	xdfileWriteClipboardTextFunc = func(text string) error {
		return errors.New("boom")
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd:    "/workspace",
				Cursor: 0,
				Entries: []xdfileEntry{
					{Name: "local.txt", Path: "/workspace/local.txt"},
				},
			},
		},
	}

	cmd := m.executeAction(xdfileActionCopyCurrentPath)
	if cmd == nil {
		t.Fatal("expected clipboard text command")
	}
	msg := cmd()
	result, ok := msg.(xdfileClipboardTextWriteResultMsg)
	if !ok {
		t.Fatalf("expected clipboard text result, got %T", msg)
	}
	if result.Err == nil {
		t.Fatal("expected clipboard text write error")
	}
	_, _ = m.Update(msg)
	if !m.statusError {
		t.Fatal("copy text write error should mark status as error")
	}
	if !strings.Contains(m.statusText, "copy text failed") {
		t.Fatalf("expected copy text error status, got %q", m.statusText)
	}
}

func assertClipboardTextMsgOK(t *testing.T, cmd func() tea.Msg) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected clipboard text command")
	}
	msg := cmd()
	result, ok := msg.(xdfileClipboardTextWriteResultMsg)
	if !ok {
		t.Fatalf("expected clipboard text result, got %T", msg)
	}
	if result.Err != nil {
		t.Fatalf("clipboard text write failed: %v", result.Err)
	}
	return msg
}
