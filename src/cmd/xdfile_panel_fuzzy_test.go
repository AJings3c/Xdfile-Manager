package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestXdfilePanelFuzzyRankingAndKeyboardSelection(t *testing.T) {
	entries := []xdfileEntry{
		{Name: "..", Path: "/tmp", IsParent: true, IsDir: true},
		{Name: "farboat.txt", Path: "/tmp/project/farboat.txt"},
		{Name: "foo-bar.txt", Path: "/tmp/project/foo-bar.txt"},
		{Name: "folder_backup.txt", Path: "/tmp/project/folder_backup.txt"},
	}
	matches := xdfilePanelFuzzyMatches(entries, "fb")
	if got := fuzzyMatchNames(entries, matches); got != "foo-bar.txt\nfolder_backup.txt\nfarboat.txt" {
		t.Fatalf("unexpected fuzzy ranking:\n%s", got)
	}

	m := baselineRenderModel(80, 24)
	m.computeLayout()
	m.panels[0].Entries = entries
	m.startPanelFuzzySearch()
	m.setPanelFuzzyQuery("fb")
	if selected := m.panels[0].Entries[m.panelFuzzy.Matches[m.panelFuzzy.Cursor].EntryIndex].Name; selected != "foo-bar.txt" {
		t.Fatalf("initial fuzzy cursor = %s, want foo-bar.txt", selected)
	}
	m.handlePanelFuzzyKey(tea.KeyMsg{Type: tea.KeyDown})
	if selected := m.panels[0].Entries[m.panelFuzzy.Matches[m.panelFuzzy.Cursor].EntryIndex].Name; selected != "folder_backup.txt" {
		t.Fatalf("down should select the next candidate, got %s", selected)
	}
}

func TestXdfilePanelFuzzyShortcutStartsSearch(t *testing.T) {
	m := baselineRenderModel(80, 24)
	m.computeLayout()
	m.keymap = xdfileDefaultKeymap()

	if _, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlF}); !handled {
		t.Fatal("ctrl+f should start fuzzy search")
	}
	if !m.panelFuzzy.Active {
		t.Fatal("fuzzy search should be active")
	}
	if len(m.panelFuzzy.Matches) != 2 {
		t.Fatalf("initial fuzzy matches = %d, want current panel selectable entries", len(m.panelFuzzy.Matches))
	}
}

func TestXdfilePanelFuzzyEmptyQueryShowsAllInPanelOrder(t *testing.T) {
	entries := []xdfileEntry{
		{Name: "..", Path: "/tmp", IsParent: true, IsDir: true},
		{Name: "README.md", Path: "/tmp/project/README.md"},
		{Name: "src", Path: "/tmp/project/src", IsDir: true},
		{Name: "main.go", Path: "/tmp/project/main.go"},
	}
	matches := xdfilePanelFuzzyMatches(entries, "")
	if got := fuzzyMatchNames(entries, matches); got != "README.md\nsrc\nmain.go" {
		t.Fatalf("empty query should preserve selectable panel order:\n%s", got)
	}
}

func TestXdfilePanelFuzzyStableForDuplicateLikeNames(t *testing.T) {
	entries := []xdfileEntry{
		{Name: "readme-a.md", Path: "/tmp/project/readme-a.md"},
		{Name: "readme-b.md", Path: "/tmp/project/readme-b.md"},
		{Name: "readme-c.md", Path: "/tmp/project/readme-c.md"},
	}
	matches := xdfilePanelFuzzyMatches(entries, "readme")
	if got := fuzzyMatchNames(entries, matches); got != "readme-a.md\nreadme-b.md\nreadme-c.md" {
		t.Fatalf("same-score fuzzy matches should remain stable:\n%s", got)
	}
}

func TestXdfilePanelFuzzyEscRestoresCursorAndScroll(t *testing.T) {
	m := baselineRenderModel(80, 24)
	m.computeLayout()
	m.layout.panelRects[0] = xdfileRect{x: 0, y: 2, w: 40, h: 6}
	m.panels[0].Entries = []xdfileEntry{
		{Name: "..", Path: "/tmp", IsParent: true, IsDir: true},
		{Name: "alpha.txt", Path: "/tmp/project/alpha.txt"},
		{Name: "beta.txt", Path: "/tmp/project/beta.txt"},
		{Name: "gamma.txt", Path: "/tmp/project/gamma.txt"},
	}
	m.panels[0].Cursor = 2
	m.panels[0].Scroll = 1

	m.startPanelFuzzySearch()
	m.setPanelFuzzyQuery("ga")
	m.handlePanelFuzzyKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.panelFuzzy.Active {
		t.Fatal("esc should close fuzzy search")
	}
	if m.panels[0].Cursor != 2 || m.panels[0].Scroll != 1 {
		t.Fatalf("esc should restore cursor/scroll, cursor=%d scroll=%d", m.panels[0].Cursor, m.panels[0].Scroll)
	}
}

func TestXdfilePanelFuzzyEnterJumpsWithoutOpening(t *testing.T) {
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
	originalCwd := m.panels[0].Cwd

	m.startPanelFuzzySearch()
	m.setPanelFuzzyQuery("child")
	m.handlePanelFuzzyKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.panelFuzzy.Active {
		t.Fatal("enter should close fuzzy search")
	}
	if m.panels[0].Cwd != originalCwd {
		t.Fatalf("enter should jump, not open the directory; cwd=%s want %s", m.panels[0].Cwd, originalCwd)
	}
	entry, ok := m.panels[0].selected()
	if !ok || entry.Name != "child" {
		t.Fatalf("enter should focus child entry, got %#v", entry)
	}
}

func TestXdfilePanelFuzzySurvivesDirectoryRefresh(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "alpha.txt"), "alpha")
	mustWriteFile(t, filepath.Join(workspace, "beta.txt"), "beta")

	m := newPinTestModel(workspace, filepath.Join(workspace, "pins.json"))
	m.layout.panelRects[0] = xdfileRect{x: 0, y: 2, w: 40, h: 12}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("initial reload: %v", err)
	}

	m.startPanelFuzzySearch()
	m.setPanelFuzzyQuery("alp")
	if got := len(m.panelFuzzy.Matches); got != 1 {
		t.Fatalf("initial fuzzy matches = %d, want 1", got)
	}

	mustWriteFile(t, filepath.Join(workspace, "alphabet.txt"), "alphabet")
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload after new file: %v", err)
	}
	if !m.panelFuzzy.Active || m.panelFuzzy.Query != "alp" {
		t.Fatalf("fuzzy search should remain active after reload: %#v", m.panelFuzzy)
	}
	if got := fuzzyMatchNames(m.panels[0].Entries, m.panelFuzzy.Matches); got != "alpha.txt\nalphabet.txt" {
		t.Fatalf("fuzzy matches after reload:\n%s", got)
	}
}

func fuzzyMatchNames(entries []xdfileEntry, matches []xdfilePanelFuzzyMatch) string {
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		if match.EntryIndex >= 0 && match.EntryIndex < len(entries) {
			names = append(names, entries[match.EntryIndex].Name)
		}
	}
	return strings.Join(names, "\n")
}
