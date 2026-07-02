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

func TestXdfileBatchRenamePreviewAndCancelDoNotWrite(t *testing.T) {
	workspace := t.TempDir()
	alpha := filepath.Join(workspace, "alpha.txt")
	beta := filepath.Join(workspace, "beta.txt")
	mustWriteFile(t, alpha, "alpha")
	mustWriteFile(t, beta, "beta")

	m := newBatchRenameTestModel(t, workspace)
	m.panels[0].MarkedPaths = map[string]struct{}{
		alpha: {},
		beta:  {},
	}

	if cmd := m.openBatchRenameModal(); cmd != nil {
		t.Fatal("opening batch rename should be synchronous")
	}
	if m.modal.Kind != xdfileModalInput || m.modal.Action != xdfileActionModalBatchRename {
		t.Fatalf("expected batch rename input modal, got %#v", m.modal)
	}
	m.modal.Input.SetValue("renamed-{index}{ext}")
	if cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Fatal("preview should not start a write command")
	}
	if m.modal.Kind != xdfileModalText || m.modal.Action != xdfileActionBatchRenamePreview {
		t.Fatalf("expected preview modal, got %#v", m.modal)
	}
	if !strings.Contains(m.modal.Text, "- alpha.txt") || !strings.Contains(m.modal.Text, "+ renamed-1.txt") {
		t.Fatalf("preview text did not include expected diff:\n%s", m.modal.Text)
	}
	assertPathExists(t, alpha)
	assertPathExists(t, beta)
	assertPathMissing(t, filepath.Join(workspace, "renamed-1.txt"))

	m.handleModalKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.pendingBatchRename != nil {
		t.Fatal("canceling preview should discard pending batch rename")
	}
	assertPathExists(t, alpha)
	assertPathExists(t, beta)
	assertPathMissing(t, filepath.Join(workspace, "renamed-1.txt"))
}

func TestXdfileBatchRenameConfirmRenamesAfterPreview(t *testing.T) {
	workspace := t.TempDir()
	alpha := filepath.Join(workspace, "alpha.txt")
	beta := filepath.Join(workspace, "beta.txt")
	mustWriteFile(t, alpha, "alpha")
	mustWriteFile(t, beta, "beta")

	m := newBatchRenameTestModel(t, workspace)
	m.panels[0].MarkedPaths = map[string]struct{}{
		alpha: {},
		beta:  {},
	}
	m.openBatchRenameModal()
	m.modal.Input.SetValue("renamed-{index}{ext}")
	m.handleModalKey(tea.KeyMsg{Type: tea.KeyEnter})

	cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyEnter})
	msg := firstBatchMsg[xdfileFileOperationDoneMsg](t, cmd)
	if msg.Err != nil || len(msg.Failures) != 0 || msg.Count != 2 {
		t.Fatalf("batch rename failed: count=%d failures=%#v err=%v", msg.Count, msg.Failures, msg.Err)
	}
	assertPathMissing(t, alpha)
	assertPathMissing(t, beta)
	assertPathExists(t, filepath.Join(workspace, "renamed-1.txt"))
	assertPathExists(t, filepath.Join(workspace, "renamed-2.txt"))
}

func TestXdfileBatchRenamePlanRejectsInvalidTargetsAndConflicts(t *testing.T) {
	workspace := t.TempDir()
	alpha := filepath.Join(workspace, "alpha.txt")
	beta := filepath.Join(workspace, "beta.txt")
	existing := filepath.Join(workspace, "existing.txt")
	mustWriteFile(t, alpha, "alpha")
	mustWriteFile(t, beta, "beta")
	mustWriteFile(t, existing, "existing")

	entries := []xdfileEntry{
		{Name: "alpha.txt", Path: alpha},
		{Name: "beta.txt", Path: beta},
	}
	for _, tc := range []struct {
		name     string
		template string
		want     string
	}{
		{name: "empty", template: " ", want: "cannot be empty"},
		{name: "path separator", template: "bad/{name}", want: "must be a file name"},
		{name: "duplicate target", template: "same.txt", want: "duplicated"},
		{name: "no change", template: "{name}", want: "would not change"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := xdfileBuildBatchRenamePlan(entries, tc.template, true)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}

	_, err := xdfileBuildBatchRenamePlan(entries[:1], "existing.txt", true)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing target error, got %v", err)
	}
}

func TestXdfileBatchRenameCasePolicyDetectsCaseOnlyConflicts(t *testing.T) {
	workspace := t.TempDir()
	sourceA := filepath.Join(workspace, "a.txt")
	sourceB := filepath.Join(workspace, "b.txt")
	mustWriteFile(t, sourceA, "a")
	mustWriteFile(t, sourceB, "b")

	items := []xdfileBatchRenameItem{
		{SourcePath: sourceA, TargetPath: filepath.Join(workspace, "Readme.txt"), OldName: "a.txt", NewName: "Readme.txt"},
		{SourcePath: sourceB, TargetPath: filepath.Join(workspace, "README.txt"), OldName: "b.txt", NewName: "README.txt"},
	}
	if err := xdfileValidateBatchRenameItems(items, true); err != nil {
		t.Fatalf("case-sensitive policy should allow distinct case targets: %v", err)
	}
	if err := xdfileValidateBatchRenameItems(items, false); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("case-insensitive policy should reject case-only target conflict, got %v", err)
	}
}

func TestXdfileBatchRenamePartialFailureIsReported(t *testing.T) {
	workspace := t.TempDir()
	alpha := filepath.Join(workspace, "alpha.txt")
	beta := filepath.Join(workspace, "beta.txt")
	alphaTarget := filepath.Join(workspace, "renamed-alpha.txt")
	betaTarget := filepath.Join(workspace, "renamed-beta.txt")
	mustWriteFile(t, alpha, "alpha")
	mustWriteFile(t, beta, "beta")

	originalRename := xdfileOSRenameFunc
	xdfileOSRenameFunc = func(oldPath string, newPath string) error {
		if oldPath == beta {
			return errors.New("permission denied")
		}
		return os.Rename(oldPath, newPath)
	}
	t.Cleanup(func() {
		xdfileOSRenameFunc = originalRename
	})

	count, failures, err := xdfileRunBatchRenameFileOperation(context.Background(), []xdfileBatchRenameItem{
		{SourcePath: alpha, TargetPath: alphaTarget, OldName: "alpha.txt", NewName: "renamed-alpha.txt"},
		{SourcePath: beta, TargetPath: betaTarget, OldName: "beta.txt", NewName: "renamed-beta.txt"},
	}, nil)
	if err != nil {
		t.Fatalf("batch rename should collect item failures, got err=%v", err)
	}
	if count != 1 || len(failures) != 1 || failures[0].SourcePath != beta {
		t.Fatalf("unexpected partial result: count=%d failures=%#v", count, failures)
	}
	assertPathExists(t, alphaTarget)
	assertPathExists(t, beta)
	assertPathMissing(t, alpha)
	assertPathMissing(t, betaTarget)
}

func newBatchRenameTestModel(t *testing.T, workspace string) *xdfileModel {
	t.Helper()
	originalCase := xdfileBatchRenameCaseSensitiveDirFunc
	xdfileBatchRenameCaseSensitiveDirFunc = func(string) bool { return true }
	t.Cleanup(func() {
		xdfileBatchRenameCaseSensitiveDirFunc = originalCase
	})

	m := newPinTestModel(workspace, filepath.Join(workspace, "pins.json"))
	m.layout.panelRects[0] = xdfileRect{x: 0, y: 2, w: 60, h: 12}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload panel: %v", err)
	}
	return m
}
