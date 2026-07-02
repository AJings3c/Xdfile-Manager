package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestXdfilePanelFilterEmptyQueryShowsAllAndEscRestoresCursor(t *testing.T) {
	m := baselineRenderModel(80, 24)
	m.computeLayout()
	entries := []xdfileEntry{{Name: "..", Path: "/tmp", IsDir: true, IsParent: true}}
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("file%02d.txt", i)
		entries = append(entries, xdfileEntry{Name: name, Path: filepath.Join("/tmp/project", name)})
	}
	m.panels[0].Entries = entries
	m.panels[0].Cursor = 10
	m.panels[0].Scroll = 5

	if cmd := m.handlePanelKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}); cmd != nil {
		_ = cmd()
	}
	if !m.panelFilter.Active {
		t.Fatal("slash should start panel filtering")
	}
	indexes := m.panelViewIndexes(0)
	if len(indexes) != len(m.panels[0].Entries) {
		t.Fatalf("empty filter should show all entries, got %d want %d", len(indexes), len(m.panels[0].Entries))
	}

	m.setPanelFilterQuery("file15")
	if m.panels[0].Cursor != 16 {
		t.Fatalf("filter should focus file15.txt, cursor=%d", m.panels[0].Cursor)
	}
	m.handlePanelFilterKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.panelFilter.Active {
		t.Fatal("esc should close panel filtering")
	}
	if m.panels[0].Cursor != 10 || m.panels[0].Scroll != 5 {
		t.Fatalf("filter close should restore cursor/scroll, cursor=%d scroll=%d", m.panels[0].Cursor, m.panels[0].Scroll)
	}
}

func TestXdfilePanelFilterCaseInsensitiveSubstringAndNoMatch(t *testing.T) {
	m := baselineRenderModel(80, 24)
	m.computeLayout()
	m.startPanelFilter()

	m.setPanelFilterQuery("SRC")
	indexes := m.panelViewIndexes(0)
	if len(indexes) != 1 || m.panels[0].Entries[indexes[0]].Name != "src" {
		t.Fatalf("case-insensitive substring should match src, indexes=%#v", indexes)
	}
	if got := m.panelFilterMatchCount(0); got != 1 {
		t.Fatalf("match count = %d, want 1", got)
	}

	m.setPanelFilterQuery("missing")
	if got := m.panelFilterMatchCount(0); got != 0 {
		t.Fatalf("missing query match count = %d, want 0", got)
	}
	if indexes := m.panelViewIndexes(0); len(indexes) != 0 {
		t.Fatalf("missing query should render no entries, got %#v", indexes)
	}
}

func TestXdfilePanelFilterPreservesMarkedSelection(t *testing.T) {
	m := baselineRenderModel(80, 24)
	m.computeLayout()
	m.panels[0].MarkedPaths = map[string]struct{}{
		"/tmp/project/README.md": {},
		"/tmp/project/src":       {},
	}

	m.startPanelFilter()
	m.setPanelFilterQuery("read")
	if got := m.panels[0].markedCount(); got != 2 {
		t.Fatalf("filter should not alter marked selection, got %d", got)
	}
	m.closePanelFilter(true)
	if got := m.panels[0].markedCount(); got != 2 {
		t.Fatalf("closing filter should preserve marked selection, got %d", got)
	}
}

func TestXdfilePanelFilterSurvivesDirectoryRefresh(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "alpha.txt"), "alpha")
	mustWriteFile(t, filepath.Join(workspace, "beta.txt"), "beta")

	m := newPinTestModel(workspace, filepath.Join(workspace, "pins.json"))
	m.layout.panelRects[0] = xdfileRect{x: 0, y: 2, w: 40, h: 12}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("initial reload: %v", err)
	}

	m.startPanelFilter()
	m.setPanelFilterQuery("alp")
	if got := m.panelFilterMatchCount(0); got != 1 {
		t.Fatalf("initial filter matches = %d, want 1", got)
	}

	mustWriteFile(t, filepath.Join(workspace, "alphabet.txt"), "alphabet")
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload after new file: %v", err)
	}
	if !m.panelFilter.Active || m.panelFilter.Query != "alp" {
		t.Fatalf("filter should remain active after reload: %#v", m.panelFilter)
	}
	if got := m.panelFilterMatchCount(0); got != 2 {
		names := make([]string, 0)
		for _, index := range m.panelViewIndexes(0) {
			names = append(names, m.panels[0].Entries[index].Name)
		}
		t.Fatalf("filter matches after reload = %d, want 2: %#v", got, names)
	}
}

func TestXdfilePanelFilterEnterOpensCurrentMatch(t *testing.T) {
	workspace := t.TempDir()
	child := filepath.Join(workspace, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}
	mustWriteFile(t, filepath.Join(workspace, "note.txt"), "note")

	m := newPinTestModel(workspace, filepath.Join(workspace, "pins.json"))
	m.layout.panelRects[0] = xdfileRect{x: 0, y: 2, w: 40, h: 12}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("initial reload: %v", err)
	}

	m.startPanelFilter()
	m.setPanelFilterQuery("child")
	m.handlePanelFilterKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.panelFilter.Active {
		t.Fatal("enter should close filter before opening the match")
	}
	if m.panels[0].Cwd != child {
		t.Fatalf("enter should open matching directory, cwd=%s want %s", m.panels[0].Cwd, child)
	}
}
