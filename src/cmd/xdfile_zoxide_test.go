package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestXdfileZoxideQueryDetectsMissingExecutable(t *testing.T) {
	restore := stubZoxideQueryDeps(t)
	defer restore()
	xdfileZoxideLookPathFunc = func(string) (string, error) {
		return "", exec.ErrNotFound
	}

	_, err := xdfileQueryZoxideCandidates("work", t.TempDir(), 6)
	if !errors.Is(err, errXdfileZoxideUnavailable) {
		t.Fatalf("missing zoxide should return unavailable sentinel, got %v", err)
	}
}

func TestXdfileZoxideQueryParsesFakeOutput(t *testing.T) {
	restore := stubZoxideQueryDeps(t)
	defer restore()

	var gotExe string
	var gotArgs []string
	var gotCwd string
	xdfileZoxideLookPathFunc = func(name string) (string, error) {
		if name != "zoxide" {
			t.Fatalf("lookpath name = %s, want zoxide", name)
		}
		return "/fake/zoxide", nil
	}
	xdfileZoxideCommandOutputFunc = func(_ context.Context, exe string, args []string, cwd string) ([]byte, error) {
		gotExe = exe
		gotArgs = append([]string(nil), args...)
		gotCwd = cwd
		return []byte("/work/src\n/work/src\n/work/docs\n"), nil
	}

	candidates, err := xdfileQueryZoxideCandidates("src docs", "/work", 6)
	if err != nil {
		t.Fatalf("query zoxide failed: %v", err)
	}
	if gotExe != "/fake/zoxide" {
		t.Fatalf("exe = %s", gotExe)
	}
	if want := []string{"query", "-l", "src", "docs"}; !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("args = %#v, want %#v", gotArgs, want)
	}
	if gotCwd != "/work" {
		t.Fatalf("cwd = %s", gotCwd)
	}
	if got := zoxideCandidatePaths(candidates); got != "/work/src\n/work/docs" {
		t.Fatalf("candidates:\n%s", got)
	}
}

func TestXdfileZoxideDisabledDegradesWithoutModal(t *testing.T) {
	m := baselineRenderModel(80, 24)
	m.computeLayout()
	m.zoxideEnabled = false

	if cmd := m.executeAction(xdfileActionZoxideJump); cmd != nil {
		t.Fatal("disabled zoxide should not start a command")
	}
	if m.modal.Kind != xdfileModalNone {
		t.Fatalf("disabled zoxide should not open modal: %#v", m.modal)
	}
	if !strings.Contains(m.statusText, "disabled") {
		t.Fatalf("status should mention disabled zoxide, got %q", m.statusText)
	}
}

func TestXdfileZoxideFiltersNonexistentCandidatesAndOpensChoice(t *testing.T) {
	workspace := t.TempDir()
	target := filepath.Join(workspace, "target")
	filePath := filepath.Join(workspace, "file.txt")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	mustWriteFile(t, filePath, "not a dir")

	m := newZoxideTestModel(t, workspace)
	m.applyZoxideQueryDone(xdfileZoxideQueryDoneMsg{
		Query:      "target",
		PanelIndex: 0,
		Candidates: []xdfileZoxideCandidate{
			{Path: filepath.Join(workspace, "missing")},
			{Path: filePath},
			{Path: target},
		},
	})

	if m.modal.Kind != xdfileModalChoice || m.modal.Action != xdfileActionZoxideJump {
		t.Fatalf("expected zoxide choice modal, got %#v", m.modal)
	}
	if got := zoxideCandidatePaths(m.zoxideCandidates); got != target {
		t.Fatalf("filtered candidates = %q, want %q", got, target)
	}
}

func TestXdfileZoxideChooseCandidateChangesCurrentPanel(t *testing.T) {
	workspace := t.TempDir()
	target := filepath.Join(workspace, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	m := newZoxideTestModel(t, workspace)
	m.openZoxideChoiceModal("target", 0, []xdfileZoxideCandidate{{Path: target}})

	if cmd := m.executeAction(xdfileZoxideOpenAction(0)); cmd != nil {
		_ = cmd()
	}
	if m.panels[0].Cwd != target {
		t.Fatalf("zoxide choose should change cwd to %s, got %s", target, m.panels[0].Cwd)
	}
	if len(m.zoxideCandidates) != 0 {
		t.Fatal("zoxide candidates should be cleared after jump")
	}
}

func TestXdfileZoxideNonexistentChoiceDoesNotChangeDirectory(t *testing.T) {
	workspace := t.TempDir()
	m := newZoxideTestModel(t, workspace)
	originalCwd := m.panels[0].Cwd
	m.openZoxideChoiceModal("missing", 0, []xdfileZoxideCandidate{{Path: filepath.Join(workspace, "missing")}})

	if cmd := m.executeAction(xdfileZoxideOpenAction(0)); cmd != nil {
		t.Fatal("nonexistent zoxide candidate should not start a command")
	}
	if m.panels[0].Cwd != originalCwd {
		t.Fatalf("cwd changed after missing candidate: got %s want %s", m.panels[0].Cwd, originalCwd)
	}
	if !strings.Contains(m.statusText, "no longer exists") {
		t.Fatalf("status should mention missing path, got %q", m.statusText)
	}
}

func TestXdfileZoxideQueryErrorShowsStatus(t *testing.T) {
	workspace := t.TempDir()
	m := newZoxideTestModel(t, workspace)
	m.applyZoxideQueryDone(xdfileZoxideQueryDoneMsg{
		Query:      "bad",
		PanelIndex: 0,
		Err:        errors.New("broken database"),
	})
	if !m.statusError || !strings.Contains(m.statusText, "broken database") {
		t.Fatalf("expected zoxide error status, got error=%v text=%q", m.statusError, m.statusText)
	}
	if m.modal.Kind != xdfileModalNone {
		t.Fatalf("query error should not open modal: %#v", m.modal)
	}
}

func TestXdfileZoxideModalFlowUsesFakeQuery(t *testing.T) {
	workspace := t.TempDir()
	target := filepath.Join(workspace, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	m := newZoxideTestModel(t, workspace)

	originalQuery := xdfileZoxideQueryCandidatesFunc
	xdfileZoxideQueryCandidatesFunc = func(query string, cwd string, limit int) ([]xdfileZoxideCandidate, error) {
		if query != "target" || cwd != workspace || limit != xdfileZoxideChoiceLimit {
			t.Fatalf("query args = (%q, %q, %d)", query, cwd, limit)
		}
		return []xdfileZoxideCandidate{{Path: target}}, nil
	}
	t.Cleanup(func() {
		xdfileZoxideQueryCandidatesFunc = originalQuery
	})

	if cmd := m.openZoxideQueryModal(); cmd != nil {
		t.Fatal("opening zoxide query modal should be synchronous")
	}
	m.modal.Input.SetValue("target")
	cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyEnter})
	msg := firstBatchMsg[xdfileZoxideQueryDoneMsg](t, cmd)
	m.applyZoxideQueryDone(msg)
	if got := zoxideCandidatePaths(m.zoxideCandidates); got != target {
		t.Fatalf("modal flow candidates = %q, want %q", got, target)
	}
}

func newZoxideTestModel(t *testing.T, workspace string) *xdfileModel {
	t.Helper()
	m := newPinTestModel(workspace, filepath.Join(workspace, "pins.json"))
	m.layout.panelRects[0] = xdfileRect{x: 0, y: 2, w: 60, h: 12}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload panel: %v", err)
	}
	m.zoxideEnabled = true
	return m
}

func stubZoxideQueryDeps(t *testing.T) func() {
	t.Helper()
	originalLookPath := xdfileZoxideLookPathFunc
	originalCommandOutput := xdfileZoxideCommandOutputFunc
	return func() {
		xdfileZoxideLookPathFunc = originalLookPath
		xdfileZoxideCommandOutputFunc = originalCommandOutput
	}
}

func zoxideCandidatePaths(candidates []xdfileZoxideCandidate) string {
	paths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		paths = append(paths, candidate.Path)
	}
	return strings.Join(paths, "\n")
}
