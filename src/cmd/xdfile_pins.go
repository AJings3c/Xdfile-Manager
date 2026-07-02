package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	variable "github.com/s0x401/xdfile-manager/src/config"
	"github.com/s0x401/xdfile-manager/src/internal/utils"
)

const (
	xdfilePinOpenActionPrefix   = "pin_open:"
	xdfilePinRenameActionPrefix = "pin_rename:"
	xdfilePinDeleteActionPrefix = "pin_delete:"
)

type xdfilePin struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

type xdfilePinPrefs struct {
	Pins []xdfilePin `json:"pins"`
}

func xdfilePinnedPrefsPath() string {
	return variable.PinnedFile
}

func xdfileLoadPinPrefs(path string) ([]xdfilePin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read pin settings: %w", err)
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}

	if len(data) > 0 && data[0] == '[' {
		var legacy []string
		if err := json.Unmarshal(data, &legacy); err == nil {
			pins := make([]xdfilePin, 0, len(legacy))
			for _, path := range legacy {
				pins = append(pins, xdfilePin{Path: path})
			}
			return xdfileNormalizePins(pins), nil
		}

		var pins []xdfilePin
		if err := json.Unmarshal(data, &pins); err != nil {
			return nil, fmt.Errorf("parse pin settings: %w", err)
		}
		return xdfileNormalizePins(pins), nil
	}

	var prefs xdfilePinPrefs
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil, fmt.Errorf("parse pin settings: %w", err)
	}
	return xdfileNormalizePins(prefs.Pins), nil
}

func xdfileSavePinPrefs(path string, pins []xdfilePin) error {
	if err := os.MkdirAll(filepath.Dir(path), utils.ConfigDirPerm); err != nil {
		return fmt.Errorf("create pin config directory: %w", err)
	}
	data, err := json.MarshalIndent(xdfilePinPrefs{Pins: xdfileNormalizePins(pins)}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode pin settings: %w", err)
	}
	if err := os.WriteFile(path, data, utils.ConfigFilePerm); err != nil {
		return fmt.Errorf("write pin settings: %w", err)
	}
	return nil
}

func xdfileNormalizePins(pins []xdfilePin) []xdfilePin {
	normalized := make([]xdfilePin, 0, len(pins))
	for _, pin := range pins {
		pin, ok := pin.normalized()
		if !ok {
			continue
		}
		if index := xdfilePinIndexByPath(normalized, pin.Path); index >= 0 {
			normalized[index] = pin
			continue
		}
		normalized = append(normalized, pin)
	}
	return normalized
}

func (p xdfilePin) normalized() (xdfilePin, bool) {
	p.Path = strings.TrimSpace(p.Path)
	if p.Path == "" {
		return xdfilePin{}, false
	}
	if remote, ok := xdfileParseNetBoxPath(p.Path); ok {
		p.Path = xdfileNetBoxURL(remote.Profile, remote.Path)
	} else {
		path, err := filepath.Abs(p.Path)
		if err == nil {
			p.Path = filepath.Clean(path)
		} else {
			p.Path = filepath.Clean(p.Path)
		}
	}
	p.Label = strings.TrimSpace(p.Label)
	if p.Label == "" {
		p.Label = xdfileDefaultPinLabel(p.Path)
	}
	return p, true
}

func xdfileDefaultPinLabel(pathValue string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && xdfilePathsEqual(pathValue, home) {
		return "Home"
	}
	if remote, ok := xdfileParseNetBoxPath(pathValue); ok {
		base := path.Base(remote.Path)
		if base == "." || base == "/" || base == "" {
			return remote.Profile
		}
		return base
	}
	base := filepath.Base(filepath.Clean(pathValue))
	if base == "." || base == string(os.PathSeparator) || base == "" {
		return pathValue
	}
	return base
}

func xdfilePinIndexByPath(pins []xdfilePin, pathValue string) int {
	for i, pin := range pins {
		if xdfilePathsEqual(pin.Path, pathValue) {
			return i
		}
	}
	return -1
}

func xdfilePinOpenAction(index int) xdfileAction {
	return xdfileAction(xdfilePinOpenActionPrefix + strconv.Itoa(index))
}

func xdfilePinRenameAction(index int) xdfileAction {
	return xdfileAction(xdfilePinRenameActionPrefix + strconv.Itoa(index))
}

func xdfilePinDeleteAction(index int) xdfileAction {
	return xdfileAction(xdfilePinDeleteActionPrefix + strconv.Itoa(index))
}

func xdfileParsePinOpenAction(action xdfileAction) (int, bool) {
	return xdfileParsePinIndexedAction(action, xdfilePinOpenActionPrefix)
}

func xdfileParsePinRenameAction(action xdfileAction) (int, bool) {
	return xdfileParsePinIndexedAction(action, xdfilePinRenameActionPrefix)
}

func xdfileParsePinDeleteAction(action xdfileAction) (int, bool) {
	return xdfileParsePinIndexedAction(action, xdfilePinDeleteActionPrefix)
}

func xdfileParsePinIndexedAction(action xdfileAction, prefix string) (int, bool) {
	value := string(action)
	if !strings.HasPrefix(value, prefix) {
		return 0, false
	}
	index, err := strconv.Atoi(strings.TrimPrefix(value, prefix))
	if err != nil {
		return 0, false
	}
	return index, true
}

func (m *xdfileModel) openPinsMenu() tea.Cmd {
	items := []xdfileButton{
		{Action: xdfileActionOpenHomePin, Key: "H", Label: "Home"},
		{Action: xdfileActionAddPin, Key: "A", Label: "Add current directory"},
	}
	if len(m.pins) == 0 {
		items = append(items, xdfileButton{Label: "No saved pins", Disabled: true})
	} else {
		items = append(items, xdfileButton{Label: "Saved pins", Disabled: true})
		for i, pin := range m.pins {
			key := ""
			if i < 9 {
				key = strconv.Itoa(i + 1)
			}
			items = append(items, xdfileButton{
				Action: xdfilePinOpenAction(i),
				Key:    key,
				Label:  xdfilePinMenuLabel(pin),
			})
		}
		items = append(items, xdfileButton{Label: "Manage pins", Disabled: true})
		for i, pin := range m.pins {
			items = append(items,
				xdfileButton{Action: xdfilePinRenameAction(i), Label: "Rename " + pin.Label},
				xdfileButton{Action: xdfilePinDeleteAction(i), Label: "Delete " + pin.Label},
			)
		}
	}

	anchor := xdfileRect{x: 0, y: xdfileHeaderHeight, w: 1, h: 1}
	if m.validPanelIndex(m.activePanel) {
		rect := m.layout.panelRects[m.activePanel]
		if rect.w > 0 && rect.h > 0 {
			anchor = xdfileRect{x: rect.x + 1, y: rect.y + 1, w: 1, h: 1}
		}
	}
	m.contextMenu = xdfileMenu{
		Action: xdfileActionContextMenu,
		Label:  "Pins",
		Items:  items,
	}
	m.contextMenuAnchor = anchor
	m.openMenu = xdfileActionContextMenu
	m.menuCursor = xdfileFirstSelectableMenuIndex(m.contextMenu)
	m.clearMouseHover()
	m.setStatus("Opened Pins")
	return nil
}

func xdfilePinMenuLabel(pin xdfilePin) string {
	pathLabel := pin.Path
	if remote, ok := xdfileParseNetBoxPath(pin.Path); ok {
		pathLabel = remote.Profile + ":" + remote.Path
	}
	if pin.Label == pathLabel {
		return pin.Label
	}
	return pin.Label + "  " + pathLabel
}

func (m *xdfileModel) openAddPinModal() tea.Cmd {
	if !m.validPanelIndex(m.activePanel) {
		m.setStatus("Invalid panel")
		return nil
	}
	pathValue := strings.TrimSpace(m.panels[m.activePanel].Cwd)
	if pathValue == "" {
		m.setStatus("Current directory cannot be pinned")
		return nil
	}
	m.openInputModal(
		xdfileActionModalPinName,
		"Add Pin",
		fmt.Sprintf("Save %s as a quick jump.", pathValue),
		m.activePanel,
		pathValue,
		xdfileDefaultPinLabel(pathValue),
	)
	m.setStatus("Name this pin")
	return nil
}

func (m *xdfileModel) savePinFromModal() tea.Cmd {
	pathValue := strings.TrimSpace(m.modal.SourcePath)
	if pathValue == "" && m.validPanelIndex(m.modal.PanelIndex) {
		pathValue = m.panels[m.modal.PanelIndex].Cwd
	}
	pin, ok := (xdfilePin{
		Label: m.modal.Input.Value(),
		Path:  pathValue,
	}).normalized()
	if !ok {
		m.setStatus("Pin path cannot be empty")
		return nil
	}
	pins := append([]xdfilePin(nil), m.pins...)
	updated := false
	if index := xdfilePinIndexByPath(pins, pin.Path); index >= 0 {
		pins[index] = pin
		updated = true
	} else {
		pins = append(pins, pin)
	}
	if err := m.savePins(pins); err != nil {
		m.setStatusErr(err)
		return nil
	}
	m.closeModal()
	if updated {
		m.setStatus("Updated pin %s", pin.Label)
	} else {
		m.setStatus("Added pin %s", pin.Label)
	}
	return nil
}

func (m *xdfileModel) openRenamePinModal(index int) tea.Cmd {
	if index < 0 || index >= len(m.pins) {
		m.setStatus("Pin not found")
		return nil
	}
	pin := m.pins[index]
	m.openInputModal(
		xdfileActionModalRenamePin,
		"Rename Pin",
		fmt.Sprintf("Rename quick jump for %s.", pin.Path),
		m.activePanel,
		pin.Path,
		pin.Label,
	)
	m.modal.PanelIndex = index
	m.setStatus("Rename pin %s", pin.Label)
	return nil
}

func (m *xdfileModel) renamePinFromModal() tea.Cmd {
	pathValue := strings.TrimSpace(m.modal.SourcePath)
	index := m.modal.PanelIndex
	if pathValue != "" {
		index = xdfilePinIndexByPath(m.pins, pathValue)
	}
	if index < 0 || index >= len(m.pins) {
		m.setStatus("Pin not found")
		return nil
	}
	pins := append([]xdfilePin(nil), m.pins...)
	renamed, ok := (xdfilePin{
		Label: m.modal.Input.Value(),
		Path:  pins[index].Path,
	}).normalized()
	if !ok {
		m.setStatus("Pin path cannot be empty")
		return nil
	}
	pins[index] = renamed
	if err := m.savePins(pins); err != nil {
		m.setStatusErr(err)
		return nil
	}
	m.closeModal()
	m.setStatus("Renamed pin to %s", renamed.Label)
	return nil
}

func (m *xdfileModel) deletePinIndex(index int) tea.Cmd {
	if index < 0 || index >= len(m.pins) {
		m.setStatus("Pin not found")
		return nil
	}
	pin := m.pins[index]
	pins := append([]xdfilePin(nil), m.pins[:index]...)
	pins = append(pins, m.pins[index+1:]...)
	if err := m.savePins(pins); err != nil {
		m.setStatusErr(err)
		return nil
	}
	m.setStatus("Deleted pin %s", pin.Label)
	return nil
}

func (m *xdfileModel) openHomePin() tea.Cmd {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		m.setStatusErr(fmt.Errorf("home directory unavailable"))
		return nil
	}
	return m.openPinPath(xdfilePin{Label: "Home", Path: home})
}

func (m *xdfileModel) openPinIndex(index int) tea.Cmd {
	if index < 0 || index >= len(m.pins) {
		m.setStatus("Pin not found")
		return nil
	}
	return m.openPinPath(m.pins[index])
}

func (m *xdfileModel) openPinPath(pin xdfilePin) tea.Cmd {
	pin, ok := pin.normalized()
	if !ok {
		m.setStatus("Pin path cannot be empty")
		return nil
	}
	if !xdfileIsNetBoxPath(pin.Path) {
		info, err := os.Stat(pin.Path)
		if err != nil {
			m.setStatusErr(fmt.Errorf("pin unavailable: %w", err))
			return nil
		}
		if !info.IsDir() {
			m.setStatus("Pin is not a directory: %s", pin.Path)
			return nil
		}
	}
	if err := m.changePanelDir(m.activePanel, pin.Path, ""); err != nil {
		m.setStatusErr(err)
		return nil
	}
	m.setStatus("Opened pin %s", pin.Label)
	return m.syncTerminalToPanel(m.activePanel)
}

func (m *xdfileModel) savePins(pins []xdfilePin) error {
	pathValue := m.pinsFile
	if pathValue == "" {
		pathValue = xdfilePinnedPrefsPath()
	}
	normalized := xdfileNormalizePins(pins)
	if err := xdfileSavePinPrefs(pathValue, normalized); err != nil {
		return err
	}
	m.pins = normalized
	return nil
}
