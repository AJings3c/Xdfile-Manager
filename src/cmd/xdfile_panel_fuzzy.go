package cmd

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	charmansi "github.com/charmbracelet/x/ansi"
	stringfunction "github.com/s0x401/xdfile-manager/src/pkg/string_function"
)

const (
	xdfilePanelFuzzyContentWidth = 36
	xdfilePanelFuzzyOverlayXPad  = 4
	xdfilePanelFuzzyResultLimit  = 8
)

type xdfilePanelFuzzyMatch struct {
	EntryIndex int
	Score      int
	Span       int
	Order      int
}

func (m *xdfileModel) handlePanelFuzzyShortcut(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m == nil || m.terminalFocused || msg.Paste {
		return nil, false
	}
	if !m.keyMatches(msg, xdfileKeymapActionPanelFuzzySearch) {
		return nil, false
	}
	return m.startPanelFuzzySearch(), true
}

func (m *xdfileModel) handlePanelFuzzyKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m == nil || !m.panelFuzzy.Active {
		return nil, false
	}
	if m.panelFuzzy.Panel < 0 || m.panelFuzzy.Panel >= len(m.panels) {
		m.closePanelFuzzySearch(false)
		return nil, false
	}

	switch {
	case m.keyMatches(msg, xdfileKeymapActionPanelClear):
		m.closePanelFuzzySearch(true)
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelOpen):
		return m.acceptPanelFuzzySearch(), true
	case msg.String() == "backspace" || msg.String() == "ctrl+h":
		m.trimPanelFuzzyRune()
		return nil, true
	case msg.String() == "ctrl+u" || msg.String() == "ctrl+y":
		m.setPanelFuzzyQuery("")
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelUp):
		m.movePanelFuzzyCursor(-1)
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelDown):
		m.movePanelFuzzyCursor(1)
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelPageUp):
		m.movePanelFuzzyCursor(-xdfilePanelFuzzyResultLimit)
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelPageDown):
		m.movePanelFuzzyCursor(xdfilePanelFuzzyResultLimit)
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelHome):
		m.setPanelFuzzyCursor(0)
		return nil, true
	case m.keyMatches(msg, xdfileKeymapActionPanelEnd):
		m.setPanelFuzzyCursor(len(m.panelFuzzy.Matches) - 1)
		return nil, true
	}

	if msg.Paste && len(msg.Runes) > 0 {
		m.extendPanelFuzzyText(strings.TrimSpace(string(msg.Runes)))
		return nil, true
	}
	if msg.Type == tea.KeySpace {
		m.extendPanelFuzzyText(" ")
		return nil, true
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		m.extendPanelFuzzyText(string(msg.Runes))
		return nil, true
	}

	return nil, true
}

func (m *xdfileModel) startPanelFuzzySearch() tea.Cmd {
	panelIndex := m.activePanel
	if panelIndex < 0 || panelIndex >= len(m.panels) {
		return nil
	}
	if m.panelFilter.Active {
		m.closePanelFilter(false)
	}
	if m.panelSearch.Active {
		m.closePanelSearch()
	}
	panel := &m.panels[panelIndex]
	m.panelFuzzy = xdfilePanelFuzzyState{
		Active:       true,
		Panel:        panelIndex,
		CursorBefore: panel.Cursor,
		ScrollBefore: panel.Scroll,
	}
	m.syncPanelFuzzyMatches()
	m.setPanelFuzzyStatus()
	return nil
}

func (m *xdfileModel) closePanelFuzzySearch(restore bool) {
	if !m.panelFuzzy.Active {
		return
	}
	panelIndex := m.panelFuzzy.Panel
	cursor := m.panelFuzzy.CursorBefore
	scroll := m.panelFuzzy.ScrollBefore
	m.panelFuzzy = xdfilePanelFuzzyState{Panel: -1}
	if restore && panelIndex >= 0 && panelIndex < len(m.panels) {
		panel := &m.panels[panelIndex]
		panel.Cursor = max(0, min(cursor, len(panel.Entries)-1))
		panel.Scroll = max(0, min(scroll, max(0, len(panel.Entries)-1)))
		panel.ensureVisible(panel.visibleRows(m.layout.panelRects[panelIndex].h))
		m.syncQuickViewViewport()
		m.setStatus("Fuzzy search canceled")
		return
	}
	m.setStatus("Fuzzy search closed")
}

func (m *xdfileModel) acceptPanelFuzzySearch() tea.Cmd {
	if !m.panelFuzzy.Active {
		return nil
	}
	match, ok := m.selectedPanelFuzzyMatch()
	if !ok {
		m.setStatus("Fuzzy search has no matches")
		return nil
	}
	panelIndex := m.panelFuzzy.Panel
	m.closePanelFuzzySearch(false)
	if panelIndex < 0 || panelIndex >= len(m.panels) {
		return nil
	}
	panel := &m.panels[panelIndex]
	if match.EntryIndex < 0 || match.EntryIndex >= len(panel.Entries) {
		m.setStatus("Fuzzy match disappeared")
		return nil
	}
	rows := panel.visibleRows(m.layout.panelRects[panelIndex].h)
	panel.setCursor(match.EntryIndex, rows)
	panel.resetRangeAnchor()
	m.activePanel = panelIndex
	focusCmd := m.focusPanel(panelIndex)
	m.syncQuickViewViewport()
	m.setStatus("Fuzzy jumped to %s", panel.Entries[match.EntryIndex].Name)
	return focusCmd
}

func (m *xdfileModel) selectedPanelFuzzyMatch() (xdfilePanelFuzzyMatch, bool) {
	if !m.panelFuzzy.Active || len(m.panelFuzzy.Matches) == 0 {
		return xdfilePanelFuzzyMatch{}, false
	}
	cursor := max(0, min(m.panelFuzzy.Cursor, len(m.panelFuzzy.Matches)-1))
	return m.panelFuzzy.Matches[cursor], true
}

func (m *xdfileModel) trimPanelFuzzyRune() {
	if m.panelFuzzy.Query == "" {
		m.setPanelFuzzyStatus()
		return
	}
	query := []rune(m.panelFuzzy.Query)
	m.setPanelFuzzyQuery(string(query[:len(query)-1]))
}

func (m *xdfileModel) extendPanelFuzzyText(text string) {
	if text == "" {
		return
	}
	m.setPanelFuzzyQuery(m.panelFuzzy.Query + text)
}

func (m *xdfileModel) setPanelFuzzyQuery(query string) {
	m.panelFuzzy.Query = xdfileNormalizePanelFuzzyQuery(query)
	m.panelFuzzy.Cursor = 0
	m.refreshPanelFuzzyMatches(false)
	m.setPanelFuzzyStatus()
}

func xdfileNormalizePanelFuzzyQuery(query string) string {
	return strings.TrimSpace(query)
}

func (m *xdfileModel) syncPanelFuzzyMatches() {
	m.refreshPanelFuzzyMatches(true)
}

func (m *xdfileModel) refreshPanelFuzzyMatches(preserveSelectedPath bool) {
	if !m.panelFuzzy.Active || m.panelFuzzy.Panel < 0 || m.panelFuzzy.Panel >= len(m.panels) {
		return
	}
	panel := &m.panels[m.panelFuzzy.Panel]
	selectedPath := ""
	if preserveSelectedPath {
		if match, ok := m.selectedPanelFuzzyMatch(); ok && match.EntryIndex >= 0 && match.EntryIndex < len(panel.Entries) {
			selectedPath = panel.Entries[match.EntryIndex].Path
		}
	}
	m.panelFuzzy.Matches = xdfilePanelFuzzyMatches(panel.Entries, m.panelFuzzy.Query)
	if len(m.panelFuzzy.Matches) == 0 {
		m.panelFuzzy.Cursor = 0
		return
	}
	if selectedPath != "" {
		for i, match := range m.panelFuzzy.Matches {
			if match.EntryIndex >= 0 && match.EntryIndex < len(panel.Entries) && xdfilePathsEqual(panel.Entries[match.EntryIndex].Path, selectedPath) {
				m.panelFuzzy.Cursor = i
				return
			}
		}
	}
	m.panelFuzzy.Cursor = max(0, min(m.panelFuzzy.Cursor, len(m.panelFuzzy.Matches)-1))
}

func (m *xdfileModel) setPanelFuzzyStatus() {
	query := m.panelFuzzy.Query
	matches := len(m.panelFuzzy.Matches)
	if query == "" {
		m.setStatus("Fuzzy search: type to rank the active panel (%d item%s)", matches, xdfilePluralSuffix(matches))
		return
	}
	m.setStatus("Fuzzy search: %s (%d match%s)", query, matches, xdfilePluralSuffix(matches))
}

func (m *xdfileModel) movePanelFuzzyCursor(delta int) {
	if len(m.panelFuzzy.Matches) == 0 || delta == 0 {
		return
	}
	m.setPanelFuzzyCursor(m.panelFuzzy.Cursor + delta)
}

func (m *xdfileModel) setPanelFuzzyCursor(cursor int) {
	if len(m.panelFuzzy.Matches) == 0 {
		m.panelFuzzy.Cursor = 0
		return
	}
	m.panelFuzzy.Cursor = max(0, min(cursor, len(m.panelFuzzy.Matches)-1))
}

func xdfilePanelFuzzyMatches(entries []xdfileEntry, query string) []xdfilePanelFuzzyMatch {
	query = xdfileNormalizePanelFuzzyQuery(query)
	matches := make([]xdfilePanelFuzzyMatch, 0, len(entries))
	for i, entry := range entries {
		if entry.IsParent {
			continue
		}
		score, span, ok := xdfilePanelFuzzyScore(entry.Name, query)
		if !ok {
			continue
		}
		matches = append(matches, xdfilePanelFuzzyMatch{
			EntryIndex: i,
			Score:      score,
			Span:       span,
			Order:      i,
		})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		left := matches[i]
		right := matches[j]
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if left.Span != right.Span {
			return left.Span < right.Span
		}
		return left.Order < right.Order
	})
	return matches
}

func xdfilePanelFuzzyScore(name string, query string) (int, int, bool) {
	query = xdfileNormalizePanelFuzzyQuery(query)
	if query == "" {
		return 0, 0, true
	}
	nameRunes := []rune(name)
	nameLower := []rune(strings.ToLower(name))
	queryLower := []rune(strings.ToLower(query))
	if len(nameLower) == 0 || len(queryLower) == 0 {
		return 0, 0, false
	}

	score := 1000
	first := -1
	last := -1
	searchFrom := 0
	for _, q := range queryLower {
		found := -1
		for i := searchFrom; i < len(nameLower); i++ {
			if nameLower[i] == q {
				found = i
				break
			}
		}
		if found < 0 {
			return 0, 0, false
		}
		if first < 0 {
			first = found
		}
		if found == last+1 {
			score += 120
		}
		if xdfilePanelFuzzyBoundary(nameRunes, found) {
			score += 45
		}
		score += max(0, 25-found)
		last = found
		searchFrom = found + 1
	}

	lowerName := string(nameLower)
	lowerQuery := string(queryLower)
	if strings.HasPrefix(lowerName, lowerQuery) {
		score += 450
	} else if strings.Contains(lowerName, lowerQuery) {
		score += 260
	}
	span := last - first + 1
	score -= span * 3
	score -= first * 2
	return score, span, true
}

func xdfilePanelFuzzyBoundary(name []rune, index int) bool {
	if index <= 0 {
		return true
	}
	prev := name[index-1]
	current := name[index]
	if prev == '-' || prev == '_' || prev == '.' || prev == ' ' || prev == '/' || prev == '\\' {
		return true
	}
	return unicode.IsLower(prev) && unicode.IsUpper(current)
}

func (m *xdfileModel) renderPanelFuzzyOverlay(index int, rendered string, rect xdfileRect) string {
	if m == nil || !m.panelFuzzy.Active || m.panelFuzzy.Panel != index || rect.w < 18 || rect.h < 7 {
		return rendered
	}

	contentWidth := min(xdfilePanelFuzzyContentWidth, max(14, rect.w-4))
	query := m.panelFuzzy.Query
	if query == "" {
		query = " "
	}
	matches := m.panelFuzzy.Matches
	inputLine := xdfileMenuItemHot.Width(contentWidth).Render(charmansi.Truncate(query, contentWidth, "..."))
	titleLine := xdfileTitleStyle.Width(contentWidth).Render("Fuzzy")
	countLine := xdfileDimStyle.Width(contentWidth).Render(charmansi.Truncate(fmt.Sprintf("%d match%s", len(matches), xdfilePluralSuffix(len(matches))), contentWidth, "..."))

	lines := []string{titleLine, inputLine, countLine}
	limit := min(xdfilePanelFuzzyResultLimit, len(matches))
	start := 0
	if limit > 0 {
		cursor := max(0, min(m.panelFuzzy.Cursor, len(matches)-1))
		if cursor >= start+limit {
			start = cursor - limit + 1
		}
		if start+limit > len(matches) {
			start = max(0, len(matches)-limit)
		}
	}
	for offset := 0; offset < limit; offset++ {
		i := start + offset
		match := matches[i]
		label := " "
		if i == m.panelFuzzy.Cursor {
			label = ">"
		}
		name := ""
		if match.EntryIndex >= 0 && match.EntryIndex < len(m.panels[index].Entries) {
			name = m.panels[index].Entries[match.EntryIndex].Name
		}
		line := label + " " + charmansi.Truncate(name, max(1, contentWidth-2), "...")
		style := xdfileDimStyle
		if i == m.panelFuzzy.Cursor {
			style = xdfileMenuItemHot
		}
		lines = append(lines, style.Width(contentWidth).Render(line))
	}
	if len(matches) == 0 {
		lines = append(lines, xdfileDimStyle.Width(contentWidth).Render("No matches"))
	}

	popup := xdfileMenuBorder().Width(contentWidth).Render(strings.Join(lines, "\n"))
	popupW := lipgloss.Width(popup)
	popupH := lipgloss.Height(popup)
	x := min(xdfilePanelFuzzyOverlayXPad, max(1, rect.w-popupW-1))
	y := max(1, rect.h-popupH-1)
	return stringfunction.PlaceOverlay(x, y, popup, rendered)
}
