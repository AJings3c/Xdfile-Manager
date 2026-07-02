package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pelletier/go-toml/v2"

	variable "github.com/s0x401/xdfile-manager/src/config"
	"github.com/s0x401/xdfile-manager/src/internal/common"
)

type xdfileKeymapPreset string

const (
	xdfileKeymapPresetDefault xdfileKeymapPreset = "default"
	xdfileKeymapPresetVim     xdfileKeymapPreset = "vim"
)

type xdfileKeymapAction string

const (
	xdfileKeymapActionHelp             xdfileKeymapAction = "help"
	xdfileKeymapActionCommands         xdfileKeymapAction = "commands"
	xdfileKeymapActionPreview          xdfileKeymapAction = "preview"
	xdfileKeymapActionQuickView        xdfileKeymapAction = "quick_view"
	xdfileKeymapActionRename           xdfileKeymapAction = "rename"
	xdfileKeymapActionPanelCopy        xdfileKeymapAction = "panel_copy"
	xdfileKeymapActionPanelMove        xdfileKeymapAction = "panel_move"
	xdfileKeymapActionMkdir            xdfileKeymapAction = "mkdir"
	xdfileKeymapActionDelete           xdfileKeymapAction = "delete"
	xdfileKeymapActionHidden           xdfileKeymapAction = "hidden"
	xdfileKeymapActionQuit             xdfileKeymapAction = "quit"
	xdfileKeymapActionUndo             xdfileKeymapAction = "undo"
	xdfileKeymapActionTerminalExpand   xdfileKeymapAction = "terminal_expand"
	xdfileKeymapActionSortName         xdfileKeymapAction = "sort_name"
	xdfileKeymapActionSortExt          xdfileKeymapAction = "sort_ext"
	xdfileKeymapActionPanelNext        xdfileKeymapAction = "panel_next"
	xdfileKeymapActionPanelPrevious    xdfileKeymapAction = "panel_previous"
	xdfileKeymapActionPanelUp          xdfileKeymapAction = "panel_up"
	xdfileKeymapActionPanelDown        xdfileKeymapAction = "panel_down"
	xdfileKeymapActionPanelPageUp      xdfileKeymapAction = "panel_page_up"
	xdfileKeymapActionPanelPageDown    xdfileKeymapAction = "panel_page_down"
	xdfileKeymapActionPanelPageLeft    xdfileKeymapAction = "panel_page_left"
	xdfileKeymapActionPanelPageRight   xdfileKeymapAction = "panel_page_right"
	xdfileKeymapActionPanelHome        xdfileKeymapAction = "panel_home"
	xdfileKeymapActionPanelEnd         xdfileKeymapAction = "panel_end"
	xdfileKeymapActionPanelClear       xdfileKeymapAction = "panel_clear"
	xdfileKeymapActionPanelOpen        xdfileKeymapAction = "panel_open"
	xdfileKeymapActionPanelParent      xdfileKeymapAction = "panel_parent"
	xdfileKeymapActionPanelRefresh     xdfileKeymapAction = "panel_refresh"
	xdfileKeymapActionPanelFilter      xdfileKeymapAction = "panel_filter"
	xdfileKeymapActionPanelFuzzySearch xdfileKeymapAction = "panel_fuzzy_search"
	xdfileKeymapActionSelectUp         xdfileKeymapAction = "select_up"
	xdfileKeymapActionSelectDown       xdfileKeymapAction = "select_down"
	xdfileKeymapActionSelectPageUp     xdfileKeymapAction = "select_page_up"
	xdfileKeymapActionSelectPageDown   xdfileKeymapAction = "select_page_down"
	xdfileKeymapActionClipboardCopy    xdfileKeymapAction = "clipboard_copy"
	xdfileKeymapActionClipboardCut     xdfileKeymapAction = "clipboard_cut"
	xdfileKeymapActionClipboardPaste   xdfileKeymapAction = "clipboard_paste"
	xdfileKeymapActionCopyCurrentPath  xdfileKeymapAction = "copy_current_path"
	xdfileKeymapActionCopyCurrentDir   xdfileKeymapAction = "copy_current_dir"
	xdfileKeymapActionPins             xdfileKeymapAction = "pins"
	xdfileKeymapActionArchive          xdfileKeymapAction = "archive"
	xdfileKeymapActionExtractArchive   xdfileKeymapAction = "extract_archive"
	xdfileKeymapActionZoxide           xdfileKeymapAction = "zoxide"
)

type xdfileKeymap struct {
	Preset   xdfileKeymapPreset
	Bindings map[xdfileKeymapAction][]string
}

func xdfileNormalizeKeymapPreset(value xdfileKeymapPreset) xdfileKeymapPreset {
	switch xdfileKeymapPreset(strings.ToLower(strings.TrimSpace(string(value)))) {
	case xdfileKeymapPresetVim:
		return xdfileKeymapPresetVim
	default:
		return xdfileKeymapPresetDefault
	}
}

func xdfileDefaultKeymap() xdfileKeymap {
	return xdfileKeymap{
		Preset: xdfileKeymapPresetDefault,
		Bindings: map[xdfileKeymapAction][]string{
			xdfileKeymapActionHelp:             {"f1"},
			xdfileKeymapActionCommands:         {"f2"},
			xdfileKeymapActionPreview:          {"f3"},
			xdfileKeymapActionQuickView:        {"ctrl+q"},
			xdfileKeymapActionRename:           {"f4"},
			xdfileKeymapActionPanelCopy:        {"f5"},
			xdfileKeymapActionPanelMove:        {"f6"},
			xdfileKeymapActionMkdir:            {"f7"},
			xdfileKeymapActionDelete:           {"f8"},
			xdfileKeymapActionHidden:           {"f9"},
			xdfileKeymapActionQuit:             {"f10"},
			xdfileKeymapActionUndo:             {"ctrl+z"},
			xdfileKeymapActionTerminalExpand:   {"ctrl+o"},
			xdfileKeymapActionSortName:         {"ctrl+3"},
			xdfileKeymapActionSortExt:          {"ctrl+4", "ctrl+\\"},
			xdfileKeymapActionPanelNext:        {"tab"},
			xdfileKeymapActionPanelPrevious:    {"shift+tab"},
			xdfileKeymapActionPanelUp:          {"up"},
			xdfileKeymapActionPanelDown:        {"down"},
			xdfileKeymapActionPanelPageUp:      {"pgup"},
			xdfileKeymapActionPanelPageDown:    {"pgdown"},
			xdfileKeymapActionPanelPageLeft:    {"left"},
			xdfileKeymapActionPanelPageRight:   {"right"},
			xdfileKeymapActionPanelHome:        {"home"},
			xdfileKeymapActionPanelEnd:         {"end"},
			xdfileKeymapActionPanelClear:       {"esc"},
			xdfileKeymapActionPanelOpen:        {"enter"},
			xdfileKeymapActionPanelRefresh:     {"r"},
			xdfileKeymapActionPanelFilter:      {"/"},
			xdfileKeymapActionPanelFuzzySearch: {"ctrl+f"},
			xdfileKeymapActionSelectUp:         {"shift+up"},
			xdfileKeymapActionSelectDown:       {"shift+down"},
			xdfileKeymapActionSelectPageUp:     {"shift+left"},
			xdfileKeymapActionSelectPageDown:   {"shift+right"},
			xdfileKeymapActionClipboardCopy:    {"ctrl+shift+c"},
			xdfileKeymapActionClipboardCut:     {"ctrl+x"},
			xdfileKeymapActionClipboardPaste:   {"ctrl+shift+v"},
		},
	}.normalized()
}

func xdfileKeymapForPreset(preset xdfileKeymapPreset) (xdfileKeymap, error) {
	preset = xdfileNormalizeKeymapPreset(preset)
	switch preset {
	case xdfileKeymapPresetVim:
		return xdfileVimKeymap()
	default:
		keymap := xdfileDefaultKeymap()
		return keymap, xdfileValidateKeymapConflicts(keymap.Bindings)
	}
}

func xdfileVimKeymap() (xdfileKeymap, error) {
	data := common.VimHotkeysTomlString
	if strings.TrimSpace(data) == "" {
		for _, candidate := range []string{
			filepath.FromSlash(variable.EmbedVimHotkeysFile),
			filepath.Join("..", "xdfile_config", "vimHotkeys.toml"),
		} {
			if fallback, err := os.ReadFile(candidate); err == nil {
				data = string(fallback)
				break
			}
		}
	}
	if strings.TrimSpace(data) == "" {
		return xdfileDefaultKeymap(), fmt.Errorf("vim keymap preset is unavailable")
	}
	return xdfileVimKeymapFromTOML(data)
}

func xdfileVimKeymapFromTOML(data string) (xdfileKeymap, error) {
	var hotkeys common.HotkeysType
	if err := toml.Unmarshal([]byte(data), &hotkeys); err != nil {
		return xdfileDefaultKeymap(), fmt.Errorf("parse vim keymap preset: %w", err)
	}
	return xdfileKeymapFromHotkeys(xdfileKeymapPresetVim, hotkeys)
}

func xdfileKeymapFromHotkeys(preset xdfileKeymapPreset, hotkeys common.HotkeysType) (xdfileKeymap, error) {
	overrides := map[xdfileKeymapAction][]string{
		xdfileKeymapActionHelp:            hotkeys.OpenHelpMenu,
		xdfileKeymapActionQuickView:       hotkeys.ToggleFilePreviewPanel,
		xdfileKeymapActionRename:          hotkeys.FilePanelItemRename,
		xdfileKeymapActionMkdir:           hotkeys.FilePanelItemCreate,
		xdfileKeymapActionDelete:          hotkeys.DeleteItems,
		xdfileKeymapActionHidden:          hotkeys.ToggleDotFile,
		xdfileKeymapActionPanelNext:       hotkeys.NextFilePanel,
		xdfileKeymapActionPanelPrevious:   hotkeys.PreviousFilePanel,
		xdfileKeymapActionPanelUp:         hotkeys.ListUp,
		xdfileKeymapActionPanelDown:       hotkeys.ListDown,
		xdfileKeymapActionPanelPageUp:     hotkeys.PageUp,
		xdfileKeymapActionPanelPageDown:   hotkeys.PageDown,
		xdfileKeymapActionPanelOpen:       hotkeys.Confirm,
		xdfileKeymapActionPanelParent:     hotkeys.ParentDirectory,
		xdfileKeymapActionPanelFilter:     hotkeys.SearchBar,
		xdfileKeymapActionSelectUp:        hotkeys.FilePanelSelectModeItemsSelectUp,
		xdfileKeymapActionSelectDown:      hotkeys.FilePanelSelectModeItemsSelectDown,
		xdfileKeymapActionClipboardCopy:   hotkeys.CopyItems,
		xdfileKeymapActionClipboardCut:    hotkeys.CutItems,
		xdfileKeymapActionClipboardPaste:  hotkeys.PasteItems,
		xdfileKeymapActionCopyCurrentPath: hotkeys.CopyPath,
		xdfileKeymapActionCopyCurrentDir:  hotkeys.CopyPWD,
		xdfileKeymapActionPins:            hotkeys.PinnedDirectory,
		xdfileKeymapActionArchive:         hotkeys.CompressFile,
		xdfileKeymapActionExtractArchive:  hotkeys.ExtractFile,
		xdfileKeymapActionZoxide:          hotkeys.OpenZoxide,
	}
	return xdfileApplyKeymapOverrides(xdfileDefaultKeymap(), xdfileNormalizeKeymapPreset(preset), overrides)
}

func xdfileApplyKeymapOverrides(base xdfileKeymap, preset xdfileKeymapPreset, overrides map[xdfileKeymapAction][]string) (xdfileKeymap, error) {
	bindings := xdfileCloneKeymapBindings(base.Bindings)
	overrideBindings := map[xdfileKeymapAction][]string{}
	overrideKeyOwners := map[string]xdfileKeymapAction{}

	for action, keys := range overrides {
		normalized := xdfileNormalizeKeyBindings(keys)
		if len(normalized) == 0 {
			continue
		}
		for _, key := range normalized {
			if owner, exists := overrideKeyOwners[key]; exists && owner != action {
				return base, fmt.Errorf("key %q is bound to both %s and %s", key, owner, action)
			}
			overrideKeyOwners[key] = action
		}
		overrideBindings[action] = normalized
	}

	for action, keys := range bindings {
		if _, overridden := overrideBindings[action]; overridden {
			continue
		}
		filtered := keys[:0]
		for _, key := range keys {
			if _, taken := overrideKeyOwners[key]; !taken {
				filtered = append(filtered, key)
			}
		}
		bindings[action] = append([]string(nil), filtered...)
	}
	for action, keys := range overrideBindings {
		bindings[action] = keys
	}

	keymap := xdfileKeymap{Preset: preset, Bindings: bindings}.normalized()
	if err := xdfileValidateKeymapConflicts(keymap.Bindings); err != nil {
		return base, err
	}
	return keymap, nil
}

func xdfileCloneKeymapBindings(bindings map[xdfileKeymapAction][]string) map[xdfileKeymapAction][]string {
	clone := make(map[xdfileKeymapAction][]string, len(bindings))
	for action, keys := range bindings {
		clone[action] = append([]string(nil), keys...)
	}
	return clone
}

func (k xdfileKeymap) normalized() xdfileKeymap {
	k.Preset = xdfileNormalizeKeymapPreset(k.Preset)
	if k.Bindings == nil {
		k.Bindings = map[xdfileKeymapAction][]string{}
	}
	for action, keys := range k.Bindings {
		normalized := xdfileNormalizeKeyBindings(keys)
		if len(normalized) == 0 {
			delete(k.Bindings, action)
			continue
		}
		k.Bindings[action] = normalized
	}
	return k
}

func xdfileNormalizeKeyBindings(keys []string) []string {
	normalized := make([]string, 0, len(keys))
	seen := map[string]struct{}{}
	for _, key := range keys {
		key = xdfileNormalizeKeyBinding(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, key)
	}
	return normalized
}

func xdfileNormalizeKeyBinding(key string) string {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, `"'`)
	if key == "" {
		return ""
	}
	key = strings.ReplaceAll(key, " ", "")
	if strings.Contains(key, "+") {
		parts := strings.Split(key, "+")
		for i := range parts {
			parts[i] = strings.ToLower(strings.TrimSpace(parts[i]))
		}
		key = strings.Join(parts, "+")
	} else if len([]rune(key)) != 1 {
		key = strings.ToLower(key)
	}
	switch strings.ToLower(key) {
	case "return":
		return "enter"
	case "escape":
		return "esc"
	case "del":
		return "delete"
	case "ins":
		return "insert"
	default:
		return key
	}
}

func xdfileValidateKeymapConflicts(bindings map[xdfileKeymapAction][]string) error {
	owners := map[string]xdfileKeymapAction{}
	actions := make([]string, 0, len(bindings))
	actionByName := map[string]xdfileKeymapAction{}
	for action := range bindings {
		name := string(action)
		actions = append(actions, name)
		actionByName[name] = action
	}
	sort.Strings(actions)
	for _, name := range actions {
		action := actionByName[name]
		for _, key := range xdfileNormalizeKeyBindings(bindings[action]) {
			if owner, ok := owners[key]; ok && owner != action {
				return fmt.Errorf("key %q is bound to both %s and %s", key, owner, action)
			}
			owners[key] = action
		}
	}
	return nil
}

func (k xdfileKeymap) Matches(msg tea.KeyMsg, action xdfileKeymapAction) bool {
	key := xdfileKeyMsgBinding(msg)
	if key == "" {
		return false
	}
	return k.MatchesBinding(key, action)
}

func (k xdfileKeymap) MatchesBinding(key string, action xdfileKeymapAction) bool {
	key = xdfileNormalizeKeyBinding(key)
	if key == "" {
		return false
	}
	for _, bound := range k.normalized().Bindings[action] {
		if bound == key {
			return true
		}
	}
	return false
}

func (k xdfileKeymap) PrimaryBinding(action xdfileKeymapAction) string {
	for _, key := range k.normalized().Bindings[action] {
		if key != "" {
			return key
		}
	}
	return ""
}

func xdfileKeyMsgBinding(msg tea.KeyMsg) string {
	if value := xdfileNormalizeKeyBinding(msg.String()); value != "" {
		return value
	}
	if len(msg.Runes) > 0 {
		return xdfileNormalizeKeyBinding(string(msg.Runes))
	}
	return ""
}

func (m *xdfileModel) keyMatches(msg tea.KeyMsg, action xdfileKeymapAction) bool {
	keymap := m.effectiveKeymap()
	return keymap.Matches(msg, action)
}

func (m *xdfileModel) panelShortcutAllowed(msg tea.KeyMsg) bool {
	if m == nil || !m.terminalFocused || m.terminalAutoFocused {
		return true
	}
	return !xdfileKeyBindingLooksText(xdfileKeyMsgBinding(msg))
}

func xdfileKeyBindingLooksText(key string) bool {
	key = xdfileNormalizeKeyBinding(key)
	return len([]rune(key)) == 1
}

func (m *xdfileModel) effectiveKeymap() xdfileKeymap {
	if m == nil || len(m.keymap.Bindings) == 0 {
		return xdfileDefaultKeymap()
	}
	return m.keymap
}

func (m *xdfileModel) keymapBindingLabel(action xdfileKeymapAction, fallback string) string {
	key := m.effectiveKeymap().PrimaryBinding(action)
	if key == "" {
		return fallback
	}
	return xdfileDisplayKeyBinding(key)
}

func xdfileDisplayKeyBinding(key string) string {
	key = xdfileNormalizeKeyBinding(key)
	if key == "" {
		return ""
	}
	parts := strings.Split(key, "+")
	for i, part := range parts {
		switch part {
		case "ctrl":
			parts[i] = "Ctrl"
		case "shift":
			parts[i] = "Shift"
		case "alt":
			parts[i] = "Alt"
		case "pgup":
			parts[i] = "PgUp"
		case "pgdown":
			parts[i] = "PgDn"
		case "esc":
			parts[i] = "Esc"
		case "enter":
			parts[i] = "Enter"
		case "up", "down", "left", "right", "home", "end", "tab", "backspace", "delete", "insert":
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		default:
			parts[i] = part
		}
	}
	return strings.Join(parts, "+")
}

func xdfileKeymapModeLabel(preset xdfileKeymapPreset) string {
	switch xdfileNormalizeKeymapPreset(preset) {
	case xdfileKeymapPresetVim:
		return "Vim"
	default:
		return "Default"
	}
}

func xdfileKeymapToggleLabel(preset xdfileKeymapPreset) string {
	return "Keymap: " + xdfileKeymapModeLabel(preset)
}

func (m *xdfileModel) toggleKeymapPreset() tea.Cmd {
	current := xdfileNormalizeKeymapPreset(m.layoutPrefs.KeymapPreset)
	next := xdfileKeymapPresetVim
	if current == xdfileKeymapPresetVim {
		next = xdfileKeymapPresetDefault
	}
	keymap, err := xdfileKeymapForPreset(next)
	if err != nil {
		m.setStatusErr(err)
		return nil
	}
	m.layoutPrefs.KeymapPreset = next
	m.keymap = keymap
	m.setStatus("Keymap switched to %s", xdfileKeymapModeLabel(next))
	return nil
}
