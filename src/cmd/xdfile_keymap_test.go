package cmd

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/s0x401/xdfile-manager/src/internal/common"
)

func TestXdfileDefaultKeymapPreservesPanelNavigation(t *testing.T) {
	m := baselineRenderModel(80, 24)
	m.computeLayout()
	m.keymap = xdfileDefaultKeymap()

	m.handlePanelKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.panels[0].Cursor != 1 {
		t.Fatalf("default keymap should not bind j to panel down, cursor=%d", m.panels[0].Cursor)
	}

	m.handlePanelKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.panels[0].Cursor != 2 {
		t.Fatalf("down arrow should keep moving the panel cursor, cursor=%d", m.panels[0].Cursor)
	}

	m.handlePanelKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.panels[0].Cursor != 1 {
		t.Fatalf("up arrow should keep moving the panel cursor, cursor=%d", m.panels[0].Cursor)
	}
}

func TestXdfileVimKeymapPanelNavigationSelectionAndParent(t *testing.T) {
	keymap, err := xdfileKeymapForPreset(xdfileKeymapPresetVim)
	if err != nil {
		t.Fatalf("load vim keymap: %v", err)
	}

	m := baselineRenderModel(80, 24)
	m.computeLayout()
	m.keymap = keymap

	m.handlePanelKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.panels[0].Cursor != 2 {
		t.Fatalf("vim j should move down, cursor=%d", m.panels[0].Cursor)
	}
	m.handlePanelKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.panels[0].Cursor != 1 {
		t.Fatalf("vim k should move up, cursor=%d", m.panels[0].Cursor)
	}

	m.handlePanelKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})
	if _, ok := m.panels[0].MarkedPaths["/tmp/project/README.md"]; !ok {
		t.Fatalf("vim J should mark current item, marks=%#v", m.panels[0].MarkedPaths)
	}

	workspace := t.TempDir()
	parent := filepath.Join(workspace, "parent")
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}
	m = newPinTestModel(child, filepath.Join(workspace, "pins.json"))
	m.layout.panelRects[0] = xdfileRect{x: 0, y: 2, w: 40, h: 12}
	m.keymap = keymap
	m.handlePanelKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-")})
	if m.panels[0].Cwd != parent {
		t.Fatalf("vim - should open parent directory, cwd=%s want %s", m.panels[0].Cwd, parent)
	}
}

func TestXdfileVimKeymapFileOperationKeys(t *testing.T) {
	keymap, err := xdfileKeymapForPreset(xdfileKeymapPresetVim)
	if err != nil {
		t.Fatalf("load vim keymap: %v", err)
	}
	m := baselineRenderModel(80, 24)
	m.computeLayout()
	m.keymap = keymap

	if _, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}); !handled {
		t.Fatal("vim d should be handled as delete")
	}
	if m.modal.Kind != xdfileModalConfirm || m.modal.Action != xdfileActionDelete {
		t.Fatalf("vim d should open delete confirmation, modal=%#v", m.modal)
	}

	m.closeModal()
	if _, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}); !handled {
		t.Fatal("vim r should be handled as rename")
	}
	if m.modal.Kind != xdfileModalInput || m.modal.Action != xdfileActionModalRename {
		t.Fatalf("vim r should open rename input, modal=%#v", m.modal)
	}

	m.closeModal()
	m.terminalFocused = true
	m.terminalAutoFocused = false
	if _, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}); handled {
		t.Fatal("vim text bindings must not steal input from a focused terminal")
	}
}

func TestXdfileKeymapDetectsConnectedConflicts(t *testing.T) {
	_, err := xdfileKeymapFromHotkeys(xdfileKeymapPresetVim, common.HotkeysType{
		ListDown: []string{"j"},
		ListUp:   []string{"j"},
	})
	if err == nil {
		t.Fatal("expected connected keymap conflict")
	}
}

func TestXdfileToggleKeymapPreset(t *testing.T) {
	m := baselineRenderModel(80, 24)
	m.layoutPrefs = xdfileDefaultLayoutPrefs()
	m.keymap = xdfileDefaultKeymap()

	m.toggleKeymapPreset()
	if m.layoutPrefs.KeymapPreset != xdfileKeymapPresetVim {
		t.Fatalf("toggle should switch to vim, got %s", m.layoutPrefs.KeymapPreset)
	}
	if !m.keymap.MatchesBinding("j", xdfileKeymapActionPanelDown) {
		t.Fatal("vim keymap should bind j to panel down")
	}

	m.toggleKeymapPreset()
	if m.layoutPrefs.KeymapPreset != xdfileKeymapPresetDefault {
		t.Fatalf("toggle should switch back to default, got %s", m.layoutPrefs.KeymapPreset)
	}
	if m.keymap.MatchesBinding("j", xdfileKeymapActionPanelDown) {
		t.Fatal("default keymap should not bind j to panel down")
	}
}
