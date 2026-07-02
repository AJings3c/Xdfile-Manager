package cmd

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	variable "github.com/s0x401/xdfile-manager/src/config"
)

func TestXdfileStageDeletePathsAndRestore(t *testing.T) {
	workspace := t.TempDir()
	stageRoot := filepath.Join(workspace, "undo-root")
	t.Setenv(xdfileDeleteUndoRootEnv, stageRoot)

	filePath := filepath.Join(workspace, "alpha.txt")
	dirPath := filepath.Join(workspace, "folder")
	childPath := filepath.Join(dirPath, "child.txt")
	mustWriteFile(t, filePath, "alpha")
	mustWriteFile(t, childPath, "child")

	progress := &xdfileFileOperationProgress{}
	batch, failures, err := xdfileStageDeletePathsContext(
		context.Background(),
		[]string{filePath, dirPath, filePath},
		progress,
	)
	if err != nil {
		t.Fatalf("stage delete failed: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("unexpected delete failures: %#v", failures)
	}
	if len(batch.Items) != 2 {
		t.Fatalf("expected 2 staged items after de-duplication, got %d", len(batch.Items))
	}
	if progress.Items.Load() == 0 {
		t.Fatal("expected delete staging to report progress")
	}
	assertPathMissing(t, filePath)
	assertPathMissing(t, dirPath)
	assertPathExists(t, batch.Root)
	for _, item := range batch.Items {
		assertPathExists(t, item.StagedPath)
	}

	if err := xdfileRestoreDeleteUndoBatch(batch); err != nil {
		t.Fatalf("restore delete batch failed: %v", err)
	}
	assertFileContent(t, filePath, "alpha")
	assertFileContent(t, childPath, "child")
	for _, item := range batch.Items {
		assertPathMissing(t, item.StagedPath)
	}
}

func TestXdfileStageDeleteCanceledBeforeMutation(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv(xdfileDeleteUndoRootEnv, filepath.Join(workspace, "undo-root"))
	filePath := filepath.Join(workspace, "alpha.txt")
	mustWriteFile(t, filePath, "alpha")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	batch, failures, err := xdfileStageDeletePathsContext(ctx, []string{filePath}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if len(batch.Items) != 0 || len(failures) != 0 {
		t.Fatalf("expected no staged work on pre-canceled context, got batch=%#v failures=%#v", batch, failures)
	}
	assertFileContent(t, filePath, "alpha")
}

func TestXdfileDeleteConfirmCancelAndUndo(t *testing.T) {
	workspace := t.TempDir()
	stateDir := filepath.Join(workspace, "state")
	originalMainDir := variable.XdfileMainDir
	variable.XdfileMainDir = stateDir
	t.Cleanup(func() {
		variable.XdfileMainDir = originalMainDir
	})
	t.Setenv(xdfileDeleteUndoRootEnv, filepath.Join(workspace, "undo-root"))

	filePath := filepath.Join(workspace, "alpha.txt")
	mustWriteFile(t, filePath, "alpha")

	m := newPinTestModel(workspace, filepath.Join(workspace, "pins.json"))
	m.layout.panelRects[0] = xdfileRect{x: 0, y: 2, w: 40, h: 12}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload panel: %v", err)
	}
	rows := m.panels[0].visibleRows(m.layout.panelRects[0].h)
	if !m.panels[0].focusPath(filePath, rows) {
		t.Fatalf("test file was not selectable: %s", filePath)
	}

	if cmd := m.executeAction(xdfileActionDelete); cmd != nil {
		t.Fatalf("delete confirmation should be synchronous, got %T", cmd)
	}
	if m.modal.Kind != xdfileModalConfirm || m.modal.Action != xdfileActionDelete {
		t.Fatalf("delete action should open a confirmation modal, got %#v", m.modal)
	}
	m.handleModalKey(tea.KeyMsg{Type: tea.KeyEsc})
	assertFileContent(t, filePath, "alpha")
	if len(m.deleteUndoStack) != 0 {
		t.Fatalf("canceling delete should not push undo state: %#v", m.deleteUndoStack)
	}

	if cmd := m.executeAction(xdfileActionDelete); cmd != nil {
		t.Fatalf("delete confirmation should be synchronous, got %T", cmd)
	}
	cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyEnter})
	done := firstBatchMsg[xdfileFileOperationDoneMsg](t, cmd)
	if done.Err != nil || len(done.Failures) != 0 || done.Count != 1 {
		t.Fatalf("delete operation failed: count=%d failures=%#v err=%v", done.Count, done.Failures, done.Err)
	}
	_, _ = m.Update(done)
	assertPathMissing(t, filePath)
	if len(m.deleteUndoStack) != 1 {
		t.Fatalf("confirmed delete should push one undo batch, got %#v", m.deleteUndoStack)
	}

	if cmd := m.executeAction(xdfileActionUndoDelete); cmd != nil {
		_ = cmd()
	}
	assertFileContent(t, filePath, "alpha")
	if len(m.deleteUndoStack) != 0 {
		t.Fatalf("undo should consume delete undo stack: %#v", m.deleteUndoStack)
	}
}

func TestXdfileCopyMoveGuardsAndUniqueTargets(t *testing.T) {
	workspace := t.TempDir()
	sourceDir := filepath.Join(workspace, "source")
	mustWriteFile(t, filepath.Join(sourceDir, "nested.txt"), "nested")

	insideTarget := filepath.Join(sourceDir, "copy")
	if err := xdfileCopyPathContext(context.Background(), sourceDir, insideTarget, nil); err == nil || !strings.Contains(err.Error(), "target is inside source") {
		t.Fatalf("expected inside-source copy guard, got %v", err)
	}
	assertPathMissing(t, insideTarget)

	sourceFile := filepath.Join(workspace, "source.txt")
	targetFile := filepath.Join(workspace, "target.txt")
	mustWriteFile(t, sourceFile, "source")
	mustWriteFile(t, targetFile, "target")

	if err := xdfileCopyPathContext(context.Background(), sourceFile, targetFile, nil); err == nil || !strings.Contains(err.Error(), "target already exists") {
		t.Fatalf("expected existing-target copy guard, got %v", err)
	}
	assertFileContent(t, targetFile, "target")

	if err := xdfileMovePathContext(context.Background(), sourceFile, sourceFile, nil); err == nil || !strings.Contains(err.Error(), "source and target are the same") {
		t.Fatalf("expected same-path move guard, got %v", err)
	}
	assertFileContent(t, sourceFile, "source")

	copyTarget, err := xdfileUniqueCopyTarget(targetFile)
	if err != nil {
		t.Fatalf("unique copy target failed: %v", err)
	}
	if filepath.Base(copyTarget) != "target (2).txt" {
		t.Fatalf("unexpected numbered copy target: %s", copyTarget)
	}

	sameFolderTarget, err := xdfileUniqueSameFolderCopyTarget(targetFile)
	if err != nil {
		t.Fatalf("same-folder copy target failed: %v", err)
	}
	if filepath.Base(sameFolderTarget) != "target - Copy.txt" {
		t.Fatalf("unexpected same-folder copy target: %s", sameFolderTarget)
	}
}

func TestXdfileMovePathFallsBackWhenRenameFails(t *testing.T) {
	workspace := t.TempDir()
	sourcePath := filepath.Join(workspace, "source.txt")
	targetPath := filepath.Join(workspace, "moved.txt")
	mustWriteFile(t, sourcePath, "source")

	originalRename := xdfileOSRenameFunc
	renameCalls := 0
	xdfileOSRenameFunc = func(oldPath string, newPath string) error {
		renameCalls++
		return errors.New("simulated cross-device rename failure")
	}
	t.Cleanup(func() {
		xdfileOSRenameFunc = originalRename
	})

	progress := &xdfileFileOperationProgress{}
	if err := xdfileMovePathContext(context.Background(), sourcePath, targetPath, progress); err != nil {
		t.Fatalf("move fallback failed: %v", err)
	}
	if renameCalls != 1 {
		t.Fatalf("expected one rename attempt, got %d", renameCalls)
	}
	assertPathMissing(t, sourcePath)
	assertFileContent(t, targetPath, "source")
	if progress.Items.Load() == 0 || progress.Bytes.Load() == 0 {
		t.Fatalf("expected fallback copy progress, got items=%d bytes=%d", progress.Items.Load(), progress.Bytes.Load())
	}
}

func TestXdfileClipboardPasteConflictPolicies(t *testing.T) {
	workspace := t.TempDir()
	sourcePath := filepath.Join(workspace, "source.txt")
	targetPath := filepath.Join(workspace, "target.txt")
	mustWriteFile(t, sourcePath, "source")
	mustWriteFile(t, targetPath, "target")

	if err := xdfileReplacePathContext(context.Background(), sourcePath, targetPath, false, nil); err != nil {
		t.Fatalf("replace policy failed: %v", err)
	}
	assertFileContent(t, sourcePath, "source")
	assertFileContent(t, targetPath, "source")

	m := &xdfileModel{}
	cutPending := &xdfilePendingClipboardPaste{CutMode: true}
	cmd, err := m.applyPendingClipboardPasteConflictAction(
		cutPending,
		xdfileActionPasteConflictSkip,
		sourcePath,
		targetPath,
		true,
	)
	if err != nil || cmd != nil {
		t.Fatalf("skip policy returned cmd=%v err=%v", cmd, err)
	}
	if cutPending.Skipped != 1 || len(cutPending.RemainingSources) != 1 || cutPending.RemainingSources[0] != sourcePath {
		t.Fatalf("skip policy did not preserve cut source: %#v", cutPending)
	}

	keepBothTarget, err := xdfileUniquePasteCopyTarget(sourcePath, targetPath)
	if err != nil {
		t.Fatalf("keep-both target failed: %v", err)
	}
	if err := xdfileCopyPathContext(context.Background(), sourcePath, keepBothTarget, nil); err != nil {
		t.Fatalf("keep-both copy failed: %v", err)
	}
	assertFileContent(t, targetPath, "source")
	assertFileContent(t, keepBothTarget, "source")
	if filepath.Base(keepBothTarget) != "target (2).txt" {
		t.Fatalf("unexpected keep-both target name: %s", keepBothTarget)
	}

	applyAllSource := filepath.Join(workspace, "apply-source.txt")
	applyAllTarget := filepath.Join(workspace, "apply-target.txt")
	mustWriteFile(t, applyAllSource, "apply source")
	mustWriteFile(t, applyAllTarget, "apply target")
	applyAllPending := &xdfilePendingClipboardPaste{
		ConflictPolicy:       xdfileActionPasteConflictSkip,
		ConflictVirtualIndex: -1,
	}
	conflict, cmd, err := m.applyPendingClipboardPasteItem(applyAllPending, xdfilePendingClipboardPasteItem{
		SourcePath: applyAllSource,
		TargetPath: applyAllTarget,
		TopLevel:   true,
	})
	if err != nil || cmd != nil || conflict {
		t.Fatalf("apply-all skip returned conflict=%v cmd=%v err=%v", conflict, cmd, err)
	}
	if applyAllPending.Skipped != 1 {
		t.Fatalf("apply-all skip policy was not reused: %#v", applyAllPending)
	}
	assertFileContent(t, applyAllTarget, "apply target")
}

func TestXdfileNetBoxURLAndTarSafety(t *testing.T) {
	remoteURL := xdfileNetBoxURL("prod", "/var/log/app one/文件.txt")
	remote, ok := xdfileParseNetBoxPath(remoteURL)
	if !ok {
		t.Fatalf("expected valid netbox URL: %s", remoteURL)
	}
	if remote.Profile != "prod" || remote.Path != "/var/log/app one/文件.txt" {
		t.Fatalf("unexpected parsed remote path: %#v", remote)
	}
	if !xdfileNetBoxPathsEqual(remoteURL, "xdssh://PROD/var/log/app%20one/%E6%96%87%E4%BB%B6.txt") {
		t.Fatalf("expected case-insensitive profile equality for %s", remoteURL)
	}

	root := t.TempDir()
	validArchive := tarArchive(t, map[string]string{
		"dir/file.txt": "safe",
	})
	if err := xdfileExtractNetBoxTarArchive(bytes.NewReader(validArchive), root); err != nil {
		t.Fatalf("valid tar extraction failed: %v", err)
	}
	assertFileContent(t, filepath.Join(root, "dir", "file.txt"), "safe")

	escapePath := filepath.Join(filepath.Dir(root), "escape.txt")
	unsafeArchive := tarArchive(t, map[string]string{
		"../escape.txt": "escape",
	})
	if err := xdfileExtractNetBoxTarArchive(bytes.NewReader(unsafeArchive), root); err == nil || !strings.Contains(err.Error(), "unsafe remote archive entry") {
		t.Fatalf("expected unsafe tar entry error, got %v", err)
	}
	assertPathMissing(t, escapePath)

	if _, err := xdfileNetBoxTarTargetPath(root, "/absolute.txt"); err == nil {
		t.Fatal("expected absolute tar entry to be rejected")
	}
	if _, err := xdfileNetBoxTarTargetPath(root, `..\escape.txt`); err == nil {
		t.Fatal("expected backslash traversal tar entry to be rejected")
	}
}

func TestXdfileExclusiveTUICommandResolution(t *testing.T) {
	dir := t.TempDir()
	vimPath := makeExecutable(t, dir, "vim")
	nvimPath := makeExecutable(t, dir, "nvim")
	lessPath := makeExecutable(t, dir, "less")
	fzfPath := makeExecutable(t, dir, "fzf")
	lazygitPath := makeExecutable(t, dir, "lazygit")
	yaziPath := makeExecutable(t, dir, "yazi")
	cmdPath := makeExecutable(t, dir, "cmd.exe")
	_ = cmdPath

	resolved, ok := xdfileResolveExclusiveTUICommand(dir, "vim notes.txt")
	if !ok {
		t.Fatal("expected vim to resolve as exclusive TUI command")
	}
	if resolved.Path != vimPath {
		t.Fatalf("expected vim path %s, got %s", vimPath, resolved.Path)
	}
	if len(resolved.Args) < 3 || resolved.Args[len(resolved.Args)-1] != "notes.txt" {
		t.Fatalf("expected vim args to preserve file argument and inject mouse setup, got %#v", resolved.Args)
	}

	for command, expectedPath := range map[string]string{
		"nvim init.lua":  nvimPath,
		"less README.md": lessPath,
		"fzf":            fzfPath,
		"lazygit":        lazygitPath,
		"yazi":           yaziPath,
	} {
		resolved, ok := xdfileResolveExclusiveTUICommand(dir, command)
		if !ok {
			t.Fatalf("expected %q to resolve as exclusive TUI command", command)
		}
		if resolved.Path != expectedPath {
			t.Fatalf("expected %q path %s, got %s", command, expectedPath, resolved.Path)
		}
	}

	resolved, ok = xdfileResolveExclusiveTUICommand(dir, `cmd.exe /c "vim wrapped.txt"`)
	if !ok {
		t.Fatal("expected cmd.exe /c wrapper to resolve nested vim")
	}
	if resolved.Path != vimPath || resolved.Args[len(resolved.Args)-1] != "wrapped.txt" {
		t.Fatalf("unexpected wrapped vim resolution: %#v", resolved)
	}

	if _, ok := xdfileResolveExclusiveTUICommand(dir, "vim notes.txt | cat"); ok {
		t.Fatal("commands with shell operators must not enter exclusive mode")
	}
	if _, ok := xdfileResolveExclusiveTUICommand(dir, "vim --version"); ok {
		t.Fatal("vim --version must not enter exclusive mode")
	}
	if _, ok := xdfileResolveExclusiveTUICommand(dir, "nvim --headless +'quit'"); ok {
		t.Fatal("nvim --headless must not enter exclusive mode")
	}
	if _, ok := xdfileResolveExclusiveTUICommand(dir, "less --version"); ok {
		t.Fatal("less --version must not enter exclusive mode")
	}
	if _, ok := xdfileResolveExclusiveTUICommand(dir, `cmd.exe /c "nvim --headless"`); ok {
		t.Fatal("cmd.exe /c nvim --headless must not enter exclusive mode")
	}
	if _, ok := xdfileResolveExclusiveTUICommand(dir, "unknown-tui"); ok {
		t.Fatal("unknown command must not enter exclusive mode")
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertFileContent(t *testing.T, path string, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != expected {
		t.Fatalf("unexpected content for %s: got %q want %q", path, string(data), expected)
	}
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be missing, got err=%v", path, err)
	}
}

func tarArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatalf("write tar header %s: %v", name, err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatalf("write tar body %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	return buffer.Bytes()
}

func makeExecutable(t *testing.T, dir string, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs executable %s: %v", path, err)
	}
	return abs
}
