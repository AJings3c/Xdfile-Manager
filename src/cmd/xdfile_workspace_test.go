package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	charmansi "github.com/charmbracelet/x/ansi"
)

func TestXdfileWorkspaceSwitchPreservesPerTabState(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	three := filepath.Join(root, "three")
	for _, dir := range []string{one, two, three} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	mustWriteFile(t, filepath.Join(one, "alpha.txt"), "alpha")
	mustWriteFile(t, filepath.Join(two, "beta.txt"), "beta")

	m := newWorkspaceTestModel(one, two)
	m.ensureWorkspacesInitialized()
	m.panelFilter = xdfilePanelFilterState{Active: true, Panel: 0, Query: "alpha", CursorBefore: 1}
	m.panelSearch = xdfilePanelSearchState{Panel: 0, Active: true, Pattern: "a"}
	m.quickView = xdfileQuickView{Open: true, Path: filepath.Join(one, "alpha.txt")}

	m.newWorkspace()
	m.panels[0].Cwd = three
	m.panels[1].Cwd = one
	m.panelFilter = xdfilePanelFilterState{Panel: -1}
	m.panelSearch = xdfilePanelSearchState{Panel: -1}
	m.quickView = xdfileQuickView{}

	m.previousWorkspace()
	if got := m.panels[0].Cwd; got != one {
		t.Fatalf("workspace 1 left cwd = %s, want %s", got, one)
	}
	if got := m.panels[1].Cwd; got != two {
		t.Fatalf("workspace 1 right cwd = %s, want %s", got, two)
	}
	if !m.panelFilter.Active || m.panelFilter.Query != "alpha" {
		t.Fatalf("workspace 1 filter not restored: %#v", m.panelFilter)
	}
	if !m.panelSearch.Active || m.panelSearch.Pattern != "a" {
		t.Fatalf("workspace 1 search not restored: %#v", m.panelSearch)
	}
	if !m.quickView.Open || m.quickView.Path == "" {
		t.Fatalf("workspace 1 quick view not restored: %#v", m.quickView)
	}

	m.nextWorkspace()
	if got := m.panels[0].Cwd; got != three {
		t.Fatalf("workspace 2 left cwd = %s, want %s", got, three)
	}
	if m.panelFilter.Active || m.panelSearch.Active || m.quickView.Open {
		t.Fatalf("workspace 2 should keep its own clean state: filter=%#v search=%#v quick=%#v", m.panelFilter, m.panelSearch, m.quickView)
	}
}

func TestXdfileWorkspaceGlobalsAreSharedAcrossTabs(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	for _, dir := range []string{one, two} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	m := newWorkspaceTestModel(one, one)
	m.pins = []xdfilePin{{Label: "One", Path: one}}
	m.layoutPrefs.ThemeName = "persona-5"
	m.ensureWorkspacesInitialized()

	m.newWorkspace()
	m.pins = append(m.pins, xdfilePin{Label: "Two", Path: two})
	m.layoutPrefs.ThemeName = "persona-4"

	m.previousWorkspace()
	if len(m.pins) != 2 {
		t.Fatalf("pins should be global, got %#v", m.pins)
	}
	if got := m.layoutPrefs.ThemeName; got != "persona-4" {
		t.Fatalf("theme should be global, got %q", got)
	}
}

func TestXdfileCloseLastWorkspaceResetsInsteadOfQuitting(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	m := newWorkspaceTestModel(dir, dir)
	m.ensureWorkspacesInitialized()
	m.panels[0].MarkedPaths = map[string]struct{}{filepath.Join(dir, "old.txt"): {}}
	m.panelFilter = xdfilePanelFilterState{Active: true, Panel: 0, Query: "old"}
	m.quickView = xdfileQuickView{Open: true, Path: filepath.Join(dir, "old.txt")}

	m.closeWorkspace()
	if len(m.workspaces) != 1 || m.activeWorkspace != 0 {
		t.Fatalf("closing last tab should keep one workspace, count=%d active=%d", len(m.workspaces), m.activeWorkspace)
	}
	if m.panels[0].markedCount() != 0 {
		t.Fatalf("reset workspace should clear marked paths: %#v", m.panels[0].MarkedPaths)
	}
	if m.panelFilter.Active || m.quickView.Open {
		t.Fatalf("reset workspace should clear per-tab UI state: filter=%#v quick=%#v", m.panelFilter, m.quickView)
	}
	if !strings.Contains(m.statusText, "Reset") {
		t.Fatalf("expected reset status, got %q", m.statusText)
	}
}

func TestXdfileWorkspaceLayoutCompatibilityAndRoundTrip(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	other := filepath.Join(root, "other")
	for _, dir := range []string{left, right, other} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	legacy := xdfileLayoutPrefs{
		StartupLeftPath:  left,
		StartupRightPath: right,
	}.normalized()
	m := newWorkspaceTestModel(left, right)
	m.initializeWorkspacesFromLayout(legacy, true)
	if len(m.workspaces) != 1 || m.panels[0].Cwd != left || m.panels[1].Cwd != right {
		t.Fatalf("legacy layout should start as one workspace: count=%d panels=%s/%s", len(m.workspaces), m.panels[0].Cwd, m.panels[1].Cwd)
	}

	prefs := xdfileLayoutPrefs{
		WorkspaceTabs: []xdfileWorkspaceLayout{
			{Title: "Code", LeftPath: left, RightPath: right, ActivePanel: 1},
			{Title: "Docs", LeftPath: other, RightPath: filepath.Join(root, "missing")},
		},
		ActiveWorkspaceIndex: 1,
	}.normalized()
	m = newWorkspaceTestModel(left, right)
	m.initializeWorkspacesFromLayout(prefs, true)
	if len(m.workspaces) != 2 || m.activeWorkspace != 1 {
		t.Fatalf("workspace layout not restored: count=%d active=%d", len(m.workspaces), m.activeWorkspace)
	}
	if got := m.panels[0].Cwd; got != other {
		t.Fatalf("active workspace left cwd = %s, want %s", got, other)
	}
	if got := m.panels[1].Cwd; got != other {
		t.Fatalf("invalid right path should fallback to left cwd, got %s want %s", got, other)
	}

	layouts := m.workspaceLayoutsForSave()
	if len(layouts) != 2 {
		t.Fatalf("workspace layouts saved = %d, want 2", len(layouts))
	}
	if layouts[1].LeftPath != other || layouts[1].RightPath != other {
		t.Fatalf("saved active workspace paths mismatch: %#v", layouts[1])
	}
}

func TestXdfileWorkspaceHeaderFitsSmallAndWideWidths(t *testing.T) {
	m := newWorkspaceTestModel("/tmp/project", "/tmp/project")
	m.workspaces = []xdfileWorkspace{
		{Title: "Project Alpha With A Very Long Name"},
		{Title: "Second Workspace"},
	}
	m.activeWorkspace = 0

	for _, width := range []int{12, 24, 80} {
		label := m.workspaceHeaderLabel(width)
		if got := lipgloss.Width(label); got > width {
			t.Fatalf("workspace label width = %d, want <= %d: %q", got, width, charmansi.Strip(label))
		}
		if !strings.Contains(label, "Tab") {
			t.Fatalf("workspace label should include tab prefix, got %q", label)
		}
	}

	renderModel := baselineRenderModel(80, 24)
	renderModel.ensureWorkspacesInitialized()
	renderModel.newWorkspace()
	rendered := renderModel.renderHeader()
	for i, line := range strings.Split(charmansi.Strip(rendered), "\n") {
		if got := lipgloss.Width(line); got > 80 {
			t.Fatalf("header line %d width = %d, want <= 80: %q", i, got, line)
		}
	}
}

func newWorkspaceTestModel(left string, right string) *xdfileModel {
	input := textinput.New()
	input.Prompt = "XD> "
	input.Width = 20
	return &xdfileModel{
		width:       80,
		height:      24,
		activePanel: 0,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: right, RangeAnchor: -1},
		},
		terminal: xdfileTerminal{
			Cwd:   left,
			Input: input,
		},
		panelSearch: xdfilePanelSearchState{Panel: -1},
		panelFilter: xdfilePanelFilterState{Panel: -1},
		panelFuzzy:  xdfilePanelFuzzyState{Panel: -1},
	}
}
