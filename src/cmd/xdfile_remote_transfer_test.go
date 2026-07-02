package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestXdfilePanelRemoteCopyUploadsLocalSelection(t *testing.T) {
	restore := stubXdfileRemoteTransferDeps(t)
	workspace := t.TempDir()
	first := filepath.Join(workspace, "alpha.txt")
	second := filepath.Join(workspace, "beta.txt")
	mustWriteFile(t, first, "alpha")
	mustWriteFile(t, second, "beta")

	var uploads [][2]string
	restore.upload = func(sourcePath string, target string) error {
		uploads = append(uploads, [2]string{sourcePath, target})
		return nil
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd:    workspace,
				Cursor: 0,
				Entries: []xdfileEntry{
					{Name: "alpha.txt", Path: first},
					{Name: "beta.txt", Path: second},
				},
				MarkedPaths: map[string]struct{}{
					first:  {},
					second: {},
				},
			},
			{Cwd: "xdssh://prod/upload"},
		},
	}

	if cmd := m.openTransferConfirm(xdfileActionCopy); cmd != nil {
		t.Fatal("copy confirm should not start work before confirmation")
	}
	if m.modal.Kind != xdfileModalConfirm || m.modal.Action != xdfileActionCopy {
		t.Fatalf("expected copy confirm modal, got kind=%v action=%q", m.modal.Kind, m.modal.Action)
	}

	cmd := m.applyModal()
	done := firstBatchMsg[xdfileRemoteClipboardPasteDoneMsg](t, cmd)
	if done.Err != nil {
		t.Fatalf("first upload failed: %v", done.Err)
	}
	cmd = m.applyRemoteClipboardPasteDone(done)
	done = firstBatchMsg[xdfileRemoteClipboardPasteDoneMsg](t, cmd)
	if done.Err != nil {
		t.Fatalf("second upload failed: %v", done.Err)
	}
	cmd = m.applyRemoteClipboardPasteDone(done)
	if cmd != nil {
		t.Fatalf("expected no queued command after remote copy finishes, got %T", cmd())
	}

	if len(uploads) != 2 {
		t.Fatalf("expected 2 uploads, got %#v", uploads)
	}
	if uploads[0] != [2]string{first, "xdssh://prod/upload/alpha.txt"} {
		t.Fatalf("unexpected first upload: %#v", uploads[0])
	}
	if uploads[1] != [2]string{second, "xdssh://prod/upload/beta.txt"} {
		t.Fatalf("unexpected second upload: %#v", uploads[1])
	}
	if m.pendingClipboardPaste != nil {
		t.Fatal("pending remote upload paste should be cleared after completion")
	}
}

func TestXdfilePanelRemoteCopyDownloadsThenCopiesToLocalPanel(t *testing.T) {
	restore := stubXdfileRemoteTransferDeps(t)
	destination := t.TempDir()
	cacheRoot := t.TempDir()
	cacheDir := filepath.Join(cacheRoot, "remote-cache")
	cacheFile := filepath.Join(cacheDir, "remote.txt")
	mustWriteFile(t, cacheFile, "remote")

	restore.download = func(paths []string) ([]string, string, error) {
		if strings.Join(paths, "\n") != "xdssh://prod/var/remote.txt" {
			t.Fatalf("unexpected download paths: %#v", paths)
		}
		return []string{cacheFile}, cacheDir, nil
	}

	m := &xdfileModel{
		activePanel: 0,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{{h: 10}, {h: 10}},
		},
		panels: [2]xdfilePanel{
			{
				Cwd:    "xdssh://prod/var",
				Cursor: 0,
				Entries: []xdfileEntry{
					{Name: "remote.txt", Path: "xdssh://prod/var/remote.txt"},
				},
			},
			{Cwd: destination},
		},
	}

	if cmd := m.openTransferConfirm(xdfileActionCopy); cmd != nil {
		t.Fatal("copy confirm should not start work before confirmation")
	}
	cmd := m.applyModal()
	downloadDone := firstBatchMsg[xdfileRemotePanelCopyDownloadDoneMsg](t, cmd)
	if downloadDone.Err != nil {
		t.Fatalf("download failed: %v", downloadDone.Err)
	}

	_, cmd = m.Update(downloadDone)
	localDone := firstBatchMsg[xdfileLocalClipboardPasteDoneMsg](t, cmd)
	if localDone.Err != nil {
		t.Fatalf("local copy failed: %v", localDone.Err)
	}
	_, cmd = m.Update(localDone)
	if cmd != nil {
		t.Fatalf("expected no queued command after local copy finishes, got %T", cmd())
	}

	assertFileContent(t, filepath.Join(destination, "remote.txt"), "remote")
	assertPathMissing(t, cacheDir)
	if m.pendingClipboardPaste != nil {
		t.Fatal("pending remote download paste should be cleared after completion")
	}
}

func TestXdfilePanelRemoteCopyDownloadFailureCleansCache(t *testing.T) {
	restore := stubXdfileRemoteTransferDeps(t)
	destination := t.TempDir()
	cacheRoot := t.TempDir()
	cacheDir := filepath.Join(cacheRoot, "remote-cache")
	mustWriteFile(t, filepath.Join(cacheDir, "remote.txt"), "remote")

	restore.download = func(paths []string) ([]string, string, error) {
		return nil, cacheDir, errors.New("missing tar")
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd:    "xdssh://prod/var",
				Cursor: 0,
				Entries: []xdfileEntry{
					{Name: "remote.txt", Path: "xdssh://prod/var/remote.txt"},
				},
			},
			{Cwd: destination},
		},
	}

	cmd := m.startPanelRemoteCopy([]string{"xdssh://prod/var/remote.txt"}, destination)
	downloadDone := firstBatchMsg[xdfileRemotePanelCopyDownloadDoneMsg](t, cmd)
	if downloadDone.Err == nil {
		t.Fatal("expected download error")
	}

	_, _ = m.Update(downloadDone)
	assertPathMissing(t, cacheDir)
	if !m.statusError || !strings.Contains(m.statusText, "missing tar") {
		t.Fatalf("expected download error status, got error=%v text=%q", m.statusError, m.statusText)
	}
}

func TestXdfilePanelRemoteCopyConflictCancelCleansCache(t *testing.T) {
	restore := stubXdfileRemoteTransferDeps(t)
	destination := t.TempDir()
	mustWriteFile(t, filepath.Join(destination, "remote.txt"), "local")
	cacheRoot := t.TempDir()
	cacheDir := filepath.Join(cacheRoot, "remote-cache")
	cacheFile := filepath.Join(cacheDir, "remote.txt")
	mustWriteFile(t, cacheFile, "remote")

	restore.download = func(paths []string) ([]string, string, error) {
		return []string{cacheFile}, cacheDir, nil
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd:    "xdssh://prod/var",
				Cursor: 0,
				Entries: []xdfileEntry{
					{Name: "remote.txt", Path: "xdssh://prod/var/remote.txt"},
				},
			},
			{Cwd: destination},
		},
	}

	cmd := m.startPanelRemoteCopy([]string{"xdssh://prod/var/remote.txt"}, destination)
	downloadDone := firstBatchMsg[xdfileRemotePanelCopyDownloadDoneMsg](t, cmd)
	_, cmd = m.Update(downloadDone)
	if cmd != nil {
		t.Fatalf("conflict should wait for user choice, got command %T", cmd())
	}
	if m.modal.Action != xdfileActionPasteConflictPrompt || m.pendingClipboardPaste == nil {
		t.Fatalf("expected paste conflict prompt with pending paste, action=%q pending=%#v", m.modal.Action, m.pendingClipboardPaste)
	}

	if cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyEsc}); cmd != nil {
		t.Fatalf("canceling conflict should not produce a command, got %T", cmd())
	}
	assertPathMissing(t, cacheDir)
	assertFileContent(t, filepath.Join(destination, "remote.txt"), "local")
	if m.pendingClipboardPaste != nil {
		t.Fatal("pending paste should be cleared after conflict cancel")
	}
}

func TestXdfilePanelRemoteMoveAndRemoteToRemoteCopyAreBlocked(t *testing.T) {
	restore := stubXdfileRemoteTransferDeps(t)
	workspace := t.TempDir()
	localFile := filepath.Join(workspace, "alpha.txt")
	mustWriteFile(t, localFile, "alpha")

	uploadCalled := false
	removeCalled := false
	restore.upload = func(sourcePath string, target string) error {
		uploadCalled = true
		return nil
	}
	restore.remove = func(target string) error {
		removeCalled = true
		return nil
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd:    workspace,
				Cursor: 0,
				Entries: []xdfileEntry{
					{Name: "alpha.txt", Path: localFile},
				},
			},
			{Cwd: "xdssh://prod/upload"},
		},
	}

	if cmd := m.openTransferConfirm(xdfileActionMove); cmd != nil {
		t.Fatal("remote move should be blocked before producing a command")
	}
	if m.modal.Kind != xdfileModalNone {
		t.Fatalf("remote move should not open a modal, got %#v", m.modal)
	}
	if !strings.Contains(m.statusText, "move is unavailable") {
		t.Fatalf("expected move unavailable status, got %q", m.statusText)
	}

	m.panels[0] = xdfilePanel{
		Cwd:    "xdssh://prod/source",
		Cursor: 0,
		Entries: []xdfileEntry{
			{Name: "remote.txt", Path: "xdssh://prod/source/remote.txt"},
		},
	}
	m.panels[1] = xdfilePanel{Cwd: "xdssh://prod/dest"}
	if cmd := m.openTransferConfirm(xdfileActionCopy); cmd != nil {
		t.Fatal("remote-to-remote copy should be blocked before producing a command")
	}
	if !strings.Contains(m.statusText, "remote-to-remote") {
		t.Fatalf("expected remote-to-remote status, got %q", m.statusText)
	}
	if uploadCalled || removeCalled {
		t.Fatalf("blocked transfer should not call upload/remove: upload=%v remove=%v", uploadCalled, removeCalled)
	}
}

type xdfileRemoteTransferTestStubs struct {
	download func([]string) ([]string, string, error)
	upload   func(string, string) error
	remove   func(string) error
}

func stubXdfileRemoteTransferDeps(t *testing.T) *xdfileRemoteTransferTestStubs {
	t.Helper()
	stubs := &xdfileRemoteTransferTestStubs{}

	originalDownload := xdfileNetBoxDownloadPathsFunc
	originalStat := xdfileNetBoxStatPathFunc
	originalUpload := xdfileNetBoxUploadPathFunc
	originalRemove := xdfileNetBoxRemovePathFunc
	originalUnique := xdfileNetBoxUniquePasteTargetFunc
	originalReadEntries := xdfileNetBoxReadEntriesFunc
	originalRemoveAll := xdfileRemoveAllFunc

	xdfileNetBoxDownloadPathsFunc = func(paths []string) ([]string, string, error) {
		if stubs.download == nil {
			t.Fatalf("unexpected NetBox download: %#v", paths)
		}
		return stubs.download(paths)
	}
	xdfileNetBoxStatPathFunc = func(target string) (xdfileNetBoxFileInfo, error) {
		return xdfileNetBoxFileInfo{}, nil
	}
	xdfileNetBoxUploadPathFunc = func(sourcePath string, target string) error {
		if stubs.upload == nil {
			t.Fatalf("unexpected NetBox upload: %s -> %s", sourcePath, target)
		}
		return stubs.upload(sourcePath, target)
	}
	xdfileNetBoxRemovePathFunc = func(target string) error {
		if stubs.remove != nil {
			return stubs.remove(target)
		}
		t.Fatalf("unexpected NetBox remove: %s", target)
		return nil
	}
	xdfileNetBoxUniquePasteTargetFunc = func(target string) (string, error) {
		return target + "-copy", nil
	}
	xdfileNetBoxReadEntriesFunc = func(dir string, showHidden bool, sortMode xdfileSortMode) ([]xdfileEntry, error) {
		return []xdfileEntry{{Name: "..", Path: dir, IsDir: true, IsParent: true}}, nil
	}
	xdfileRemoveAllFunc = os.RemoveAll

	t.Cleanup(func() {
		xdfileNetBoxDownloadPathsFunc = originalDownload
		xdfileNetBoxStatPathFunc = originalStat
		xdfileNetBoxUploadPathFunc = originalUpload
		xdfileNetBoxRemovePathFunc = originalRemove
		xdfileNetBoxUniquePasteTargetFunc = originalUnique
		xdfileNetBoxReadEntriesFunc = originalReadEntries
		xdfileRemoveAllFunc = originalRemoveAll
	})

	return stubs
}

func firstBatchMsg[T tea.Msg](t *testing.T, cmd tea.Cmd) T {
	t.Helper()
	var zero T
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		if len(batch) == 0 || batch[0] == nil {
			t.Fatalf("expected first batch command, got %#v", batch)
		}
		msg = batch[0]()
	}
	result, ok := msg.(T)
	if !ok {
		t.Fatalf("expected %T, got %T", zero, msg)
	}
	return result
}
