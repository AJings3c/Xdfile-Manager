package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	charmansi "github.com/charmbracelet/x/ansi"
	stringfunction "github.com/s0x401/xdfile-manager/src/pkg/string_function"
)

const (
	xdfilePanelFilterContentWidth = 28
	xdfilePanelFilterOverlayXPad  = 2
)

func (m *xdfileModel) handlePanelFilterShortcut(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m == nil || m.terminalFocused || msg.Paste {
		return nil, false
	}
	if !m.keyMatches(msg, xdfileKeymapActionPanelFilter) {
		return nil, false
	}
	return m.startPanelFilter(), true
}

func (m *xdfileModel) handlePanelFilterKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m == nil || !m.panelFilter.Active {
		return nil, false
	}
	if m.panelFilter.Panel < 0 || m.panelFilter.Panel >= len(m.panels) {
		m.closePanelFilter(false)
		return nil, false
	}

	switch {
	case m.keyMatches(msg, xdfileKeymapActionPanelClear):
		m.closePanelFilter(true)
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelOpen):
		if m.panelFilterMatchCount(m.panelFilter.Panel) == 0 {
			m.setStatus("Filter has no matches")
			return nil, true
		}
		m.closePanelFilter(false)
		return m.activateSelection(), true
	case msg.String() == "backspace" || msg.String() == "ctrl+h":
		m.trimPanelFilterRune()
		return nil, true
	case msg.String() == "ctrl+u" || msg.String() == "ctrl+y":
		m.setPanelFilterQuery("")
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelUp):
		m.movePanelFilterCursor(-1)
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelDown):
		m.movePanelFilterCursor(1)
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelPageUp):
		m.movePanelFilterCursor(-m.panelFilterRows())
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelPageDown):
		m.movePanelFilterCursor(m.panelFilterRows())
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelHome):
		m.movePanelFilterCursorToEdge(false)
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelEnd):
		m.movePanelFilterCursorToEdge(true)
		return nil, true
	}

	if msg.Paste && len(msg.Runes) > 0 {
		m.extendPanelFilterText(strings.TrimSpace(string(msg.Runes)))
		return nil, true
	}
	if msg.Type == tea.KeySpace {
		m.extendPanelFilterText(" ")
		return nil, true
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		m.extendPanelFilterText(string(msg.Runes))
		return nil, true
	}

	return nil, true
}

func (m *xdfileModel) startPanelFilter() tea.Cmd {
	panelIndex := m.activePanel
	if panelIndex < 0 || panelIndex >= len(m.panels) {
		return nil
	}
	panel := &m.panels[panelIndex]
	m.panelFilter = xdfilePanelFilterState{
		Active:       true,
		Panel:        panelIndex,
		CursorBefore: panel.Cursor,
		ScrollBefore: panel.Scroll,
	}
	m.syncPanelFilterCursor()
	m.setStatus("Filter: type to narrow the active panel")
	return nil
}

func (m *xdfileModel) closePanelFilter(restore bool) {
	if !m.panelFilter.Active {
		return
	}
	panelIndex := m.panelFilter.Panel
	cursor := m.panelFilter.CursorBefore
	scroll := m.panelFilter.ScrollBefore
	m.panelFilter = xdfilePanelFilterState{Panel: -1}
	if restore && panelIndex >= 0 && panelIndex < len(m.panels) {
		panel := &m.panels[panelIndex]
		panel.Cursor = max(0, min(cursor, len(panel.Entries)-1))
		panel.Scroll = max(0, min(scroll, max(0, len(panel.Entries)-1)))
		panel.ensureVisible(panel.visibleRows(m.layout.panelRects[panelIndex].h))
		m.syncQuickViewViewport()
		m.setStatus("Filter closed")
		return
	}
	m.setStatus("Filter closed")
}

func (m *xdfileModel) trimPanelFilterRune() {
	if m.panelFilter.Query == "" {
		m.setStatus("Filter")
		return
	}
	query := []rune(m.panelFilter.Query)
	m.setPanelFilterQuery(string(query[:len(query)-1]))
}

func (m *xdfileModel) extendPanelFilterText(text string) {
	if text == "" {
		return
	}
	m.setPanelFilterQuery(m.panelFilter.Query + text)
}

func (m *xdfileModel) setPanelFilterQuery(query string) {
	query = xdfileNormalizePanelFilterQuery(query)
	if m.panelFilter.Query != query {
		m.panelFilter.Scroll = 0
	}
	m.panelFilter.Query = query
	m.syncPanelFilterCursor()
	m.setPanelFilterStatus()
}

func xdfileNormalizePanelFilterQuery(query string) string {
	return strings.TrimSpace(query)
}

func (m *xdfileModel) setPanelFilterStatus() {
	query := m.panelFilter.Query
	if query == "" {
		m.setStatus("Filter: type to narrow the active panel")
		return
	}
	matches := m.panelFilterMatchCount(m.panelFilter.Panel)
	m.setStatus("Filter: %s (%d match%s)", query, matches, xdfilePluralSuffix(matches))
}

func xdfilePanelFilterEntryMatches(entry xdfileEntry, query string) bool {
	if entry.IsParent {
		return false
	}
	query = xdfileNormalizePanelFilterQuery(query)
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(entry.Name), strings.ToLower(query))
}

func (m *xdfileModel) panelViewIndexes(panelIndex int) []int {
	if panelIndex < 0 || panelIndex >= len(m.panels) {
		return nil
	}
	panel := &m.panels[panelIndex]
	if !m.panelFilterActiveFor(panelIndex) {
		indexes := make([]int, len(panel.Entries))
		for i := range panel.Entries {
			indexes[i] = i
		}
		return indexes
	}
	query := m.panelFilter.Query
	if query == "" {
		indexes := make([]int, len(panel.Entries))
		for i := range panel.Entries {
			indexes[i] = i
		}
		return indexes
	}
	indexes := make([]int, 0, len(panel.Entries))
	for i, entry := range panel.Entries {
		if xdfilePanelFilterEntryMatches(entry, query) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func (m *xdfileModel) panelFilterActiveFor(panelIndex int) bool {
	return m != nil && m.panelFilter.Active && m.panelFilter.Panel == panelIndex
}

func (m *xdfileModel) panelFilterMatchCount(panelIndex int) int {
	if !m.panelFilterActiveFor(panelIndex) {
		return 0
	}
	if m.panelFilter.Query == "" {
		return m.panels[panelIndex].entryCount()
	}
	return len(m.panelViewIndexes(panelIndex))
}

func (m *xdfileModel) panelFilterRows() int {
	panelIndex := m.panelFilter.Panel
	if panelIndex < 0 || panelIndex >= len(m.panels) {
		return 1
	}
	return m.panels[panelIndex].visibleRows(m.layout.panelRects[panelIndex].h)
}

func (m *xdfileModel) panelViewScroll(panelIndex int) int {
	if m.panelFilterActiveFor(panelIndex) {
		return m.panelFilter.Scroll
	}
	return m.panels[panelIndex].Scroll
}

func (m *xdfileModel) panelViewCursorPosition(panelIndex int, indexes []int) int {
	if panelIndex < 0 || panelIndex >= len(m.panels) {
		return -1
	}
	cursor := m.panels[panelIndex].Cursor
	for i, index := range indexes {
		if index == cursor {
			return i
		}
	}
	return -1
}

func (m *xdfileModel) panelViewCursorEntryNumber(panelIndex int, indexes []int) int {
	position := m.panelViewCursorPosition(panelIndex, indexes)
	if position < 0 {
		return 0
	}
	if m.panelFilterActiveFor(panelIndex) && m.panelFilter.Query != "" {
		return position + 1
	}
	count := 0
	for i := 0; i <= position && i < len(indexes); i++ {
		entry := m.panels[panelIndex].Entries[indexes[i]]
		if !entry.IsParent {
			count++
		}
	}
	return count
}

func (m *xdfileModel) panelViewEntryCount(panelIndex int, indexes []int) int {
	if !m.panelFilterActiveFor(panelIndex) || m.panelFilter.Query == "" {
		return m.panels[panelIndex].entryCount()
	}
	return len(indexes)
}

func (m *xdfileModel) syncPanelFilterCursor() {
	panelIndex := m.panelFilter.Panel
	if !m.panelFilterActiveFor(panelIndex) {
		return
	}
	panel := &m.panels[panelIndex]
	indexes := m.panelViewIndexes(panelIndex)
	if len(indexes) == 0 {
		m.panelFilter.Scroll = 0
		m.syncQuickViewViewport()
		return
	}
	if m.panelViewCursorPosition(panelIndex, indexes) < 0 {
		panel.Cursor = indexes[0]
	}
	m.ensurePanelFilterVisible()
	m.syncQuickViewViewport()
}

func (m *xdfileModel) ensurePanelFilterVisible() {
	panelIndex := m.panelFilter.Panel
	if !m.panelFilterActiveFor(panelIndex) {
		return
	}
	indexes := m.panelViewIndexes(panelIndex)
	rows := m.panelFilterRows()
	position := m.panelViewCursorPosition(panelIndex, indexes)
	if position < 0 {
		m.panelFilter.Scroll = 0
		return
	}
	if position < m.panelFilter.Scroll {
		m.panelFilter.Scroll = position
	}
	if position >= m.panelFilter.Scroll+rows {
		m.panelFilter.Scroll = position - rows + 1
	}
	maxScroll := max(0, len(indexes)-rows)
	m.panelFilter.Scroll = max(0, min(m.panelFilter.Scroll, maxScroll))
}

func (m *xdfileModel) movePanelFilterCursor(delta int) {
	panelIndex := m.panelFilter.Panel
	if !m.panelFilterActiveFor(panelIndex) || delta == 0 {
		return
	}
	indexes := m.panelViewIndexes(panelIndex)
	if len(indexes) == 0 {
		m.setStatus("Filter has no matches")
		return
	}
	position := m.panelViewCursorPosition(panelIndex, indexes)
	if position < 0 {
		position = 0
	} else {
		position = max(0, min(position+delta, len(indexes)-1))
	}
	m.panels[panelIndex].Cursor = indexes[position]
	m.ensurePanelFilterVisible()
	m.syncQuickViewViewport()
}

func (m *xdfileModel) movePanelFilterCursorToEdge(end bool) {
	panelIndex := m.panelFilter.Panel
	if !m.panelFilterActiveFor(panelIndex) {
		return
	}
	indexes := m.panelViewIndexes(panelIndex)
	if len(indexes) == 0 {
		m.setStatus("Filter has no matches")
		return
	}
	position := 0
	if end {
		position = len(indexes) - 1
	}
	m.panels[panelIndex].Cursor = indexes[position]
	m.ensurePanelFilterVisible()
	m.syncQuickViewViewport()
}

func (m *xdfileModel) scrollPanelView(panelIndex int, delta int, rows int) {
	if m.panelFilterActiveFor(panelIndex) {
		indexes := m.panelViewIndexes(panelIndex)
		maxScroll := max(0, len(indexes)-rows)
		m.panelFilter.Scroll = max(0, min(m.panelFilter.Scroll+delta, maxScroll))
		return
	}
	m.panels[panelIndex].scroll(delta, rows)
}

func (m *xdfileModel) panelViewIndexAt(panelIndex int, row int, rows int) (int, bool) {
	indexes := m.panelViewIndexes(panelIndex)
	if len(indexes) == 0 {
		return 0, false
	}
	scroll := m.panelViewScroll(panelIndex)
	position := scroll + row
	if row < 0 {
		position = scroll
	}
	if row >= rows {
		position = scroll + rows - 1
	}
	position = max(0, min(position, len(indexes)-1))
	return indexes[position], true
}

func (m *xdfileModel) renderPanelFilterOverlay(index int, rendered string, rect xdfileRect) string {
	if m == nil || !m.panelFilterActiveFor(index) || rect.w < 14 || rect.h < 6 {
		return rendered
	}

	contentWidth := min(xdfilePanelFilterContentWidth, max(10, rect.w-4))
	query := m.panelFilter.Query
	if query == "" {
		query = " "
	}
	matches := m.panelFilterMatchCount(index)
	inputLine := xdfileMenuItemHot.Width(contentWidth).Render(charmansi.Truncate(query, contentWidth, "..."))
	titleLine := xdfileTitleStyle.Width(contentWidth).Render("Filter")
	countLine := xdfileDimStyle.Width(contentWidth).Render(charmansi.Truncate(fmt.Sprintf("%d match%s", matches, xdfilePluralSuffix(matches)), contentWidth, "..."))
	popup := xdfileMenuBorder().Width(contentWidth).Render(titleLine + "\n" + inputLine + "\n" + countLine)
	popupW := lipgloss.Width(popup)
	popupH := lipgloss.Height(popup)
	x := min(xdfilePanelFilterOverlayXPad, max(1, rect.w-popupW-1))
	y := max(1, rect.h-popupH-1)
	return stringfunction.PlaceOverlay(x, y, popup, rendered)
}
