package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
)

func TestXdfilePinPrefsRoundTripDeduplicatesPaths(t *testing.T) {
	workspace := t.TempDir()
	first := filepath.Join(workspace, "first")
	second := filepath.Join(workspace, "second")
	if err := os.MkdirAll(first, 0o755); err != nil {
		t.Fatalf("mkdir first: %v", err)
	}
	if err := os.MkdirAll(second, 0o755); err != nil {
		t.Fatalf("mkdir second: %v", err)
	}

	path := filepath.Join(workspace, "pins.json")
	if err := xdfileSavePinPrefs(path, []xdfilePin{
		{Label: "First", Path: first},
		{Label: "Renamed first", Path: first},
		{Path: second},
		{Label: "Blank", Path: "  "},
	}); err != nil {
		t.Fatalf("save pins: %v", err)
	}

	pins, err := xdfileLoadPinPrefs(path)
	if err != nil {
		t.Fatalf("load pins: %v", err)
	}
	if len(pins) != 2 {
		t.Fatalf("pin count = %d, want 2: %#v", len(pins), pins)
	}
	if pins[0].Label != "Renamed first" || pins[0].Path != first {
		t.Fatalf("duplicate path should update first pin, got %#v", pins[0])
	}
	if pins[1].Label != "second" || pins[1].Path != second {
		t.Fatalf("blank label should use directory basename, got %#v", pins[1])
	}
}

func TestXdfilePinPrefsLoadsLegacyPathList(t *testing.T) {
	workspace := t.TempDir()
	first := filepath.Join(workspace, "first")
	second := filepath.Join(workspace, "second")
	data, err := json.Marshal([]string{first, second, first})
	if err != nil {
		t.Fatalf("marshal legacy pins: %v", err)
	}
	path := filepath.Join(workspace, "legacy-pins.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write legacy pins: %v", err)
	}

	pins, err := xdfileLoadPinPrefs(path)
	if err != nil {
		t.Fatalf("load legacy pins: %v", err)
	}
	if len(pins) != 2 {
		t.Fatalf("legacy pin count = %d, want 2: %#v", len(pins), pins)
	}
	if pins[0].Label != "first" || pins[1].Label != "second" {
		t.Fatalf("legacy pins should get default labels: %#v", pins)
	}
}

func TestXdfilePinModalSaveRenameDeleteAndRestartRecovery(t *testing.T) {
	workspace := t.TempDir()
	currentDir := filepath.Join(workspace, "current")
	if err := os.MkdirAll(currentDir, 0o755); err != nil {
		t.Fatalf("mkdir current: %v", err)
	}
	pinsFile := filepath.Join(workspace, "xdfile-pinned.json")
	m := newPinTestModel(currentDir, pinsFile)

	if cmd := m.openAddPinModal(); cmd != nil {
		_ = cmd()
	}
	m.modal.Input.SetValue("Workspace")
	if cmd := m.applyModal(); cmd != nil {
		_ = cmd()
	}
	if len(m.pins) != 1 || m.pins[0].Label != "Workspace" || m.pins[0].Path != currentDir {
		t.Fatalf("pin was not saved from modal: %#v", m.pins)
	}

	loaded, err := xdfileLoadPinPrefs(pinsFile)
	if err != nil {
		t.Fatalf("reload pins after save: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Label != "Workspace" {
		t.Fatalf("restart recovery load mismatch: %#v", loaded)
	}

	if cmd := m.openRenamePinModal(0); cmd != nil {
		_ = cmd()
	}
	m.modal.Input.SetValue("Renamed")
	if cmd := m.applyModal(); cmd != nil {
		_ = cmd()
	}
	if len(m.pins) != 1 || m.pins[0].Label != "Renamed" {
		t.Fatalf("pin was not renamed: %#v", m.pins)
	}

	if cmd := m.deletePinIndex(99); cmd != nil {
		_ = cmd()
	}
	if len(m.pins) != 1 {
		t.Fatalf("deleting a missing pin should be a no-op, got %#v", m.pins)
	}

	if cmd := m.deletePinIndex(0); cmd != nil {
		_ = cmd()
	}
	if len(m.pins) != 0 {
		t.Fatalf("pin was not deleted: %#v", m.pins)
	}
	loaded, err = xdfileLoadPinPrefs(pinsFile)
	if err != nil {
		t.Fatalf("reload pins after delete: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("deleted pin should not survive restart: %#v", loaded)
	}
}

func TestXdfileOpenPinChangesPanelAndTerminalCwd(t *testing.T) {
	workspace := t.TempDir()
	currentDir := filepath.Join(workspace, "current")
	targetDir := filepath.Join(workspace, "target")
	mustWriteFile(t, filepath.Join(currentDir, "current.txt"), "current")
	mustWriteFile(t, filepath.Join(targetDir, "target.txt"), "target")

	m := newPinTestModel(currentDir, filepath.Join(workspace, "pins.json"))
	m.pins = []xdfilePin{{Label: "Target", Path: targetDir}}
	m.layout.panelRects[0] = xdfileRect{x: 0, y: 2, w: 40, h: 12}

	if cmd := m.openPinIndex(0); cmd != nil {
		_ = cmd()
	}
	if m.panels[0].Cwd != targetDir {
		t.Fatalf("panel cwd = %s, want %s", m.panels[0].Cwd, targetDir)
	}
	if m.terminal.Cwd != targetDir {
		t.Fatalf("terminal cwd = %s, want %s", m.terminal.Cwd, targetDir)
	}
	if len(m.panels[0].Entries) == 0 {
		t.Fatal("opening a pin should reload panel entries")
	}
}

func TestXdfileOpenMissingPinDoesNotChangePanel(t *testing.T) {
	workspace := t.TempDir()
	currentDir := filepath.Join(workspace, "current")
	if err := os.MkdirAll(currentDir, 0o755); err != nil {
		t.Fatalf("mkdir current: %v", err)
	}

	m := newPinTestModel(currentDir, filepath.Join(workspace, "pins.json"))
	m.pins = []xdfilePin{{Label: "Missing", Path: filepath.Join(workspace, "missing")}}
	if cmd := m.openPinIndex(0); cmd != nil {
		_ = cmd()
	}
	if m.panels[0].Cwd != currentDir {
		t.Fatalf("missing pin should not change cwd, got %s", m.panels[0].Cwd)
	}
}

func TestXdfilePinsPopupDoesNotChangePanelLayout(t *testing.T) {
	for _, size := range []struct {
		width  int
		height int
	}{
		{width: 80, height: 24},
		{width: 120, height: 32},
	} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			m := baselineRenderModel(size.width, size.height)
			m.pins = []xdfilePin{{Label: "Project", Path: "/tmp/project"}}
			m.computeLayout()
			before := m.layout.panelRects

			if cmd := m.openPinsMenu(); cmd != nil {
				_ = cmd()
			}

			if m.layout.panelRects != before {
				t.Fatalf("pin popup changed panel layout: before=%#v after=%#v", before, m.layout.panelRects)
			}
		})
	}
}

func newPinTestModel(cwd string, pinsFile string) *xdfileModel {
	input := textinput.New()
	input.Prompt = "XD> "
	return &xdfileModel{
		activePanel: 0,
		pinsFile:    pinsFile,
		layoutPrefs: xdfileLayoutPrefs{
			PanelSplitPercent:     50,
			TerminalHeightPercent: 25,
		},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: cwd, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: cwd, RangeAnchor: -1},
		},
		terminal: xdfileTerminal{
			Cwd:   cwd,
			Input: input,
		},
		panelSearch: xdfilePanelSearchState{Panel: -1},
	}
}
