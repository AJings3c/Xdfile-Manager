package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	charmansi "github.com/charmbracelet/x/ansi"
	stringfunction "github.com/s0x401/xdfile-manager/src/pkg/string_function"
)

const (
	xdfileStartHubHostActionPrefix = "start_hub_host:"
	xdfileStartHubLocalAction      = xdfileAction("start_hub_local")
	xdfileStartHubNewAction        = xdfileAction("start_hub_new")
	xdfileStartHubEditAction       = xdfileAction("start_hub_edit")
	xdfileStartHubDeleteAction     = xdfileAction("start_hub_delete")
)

type xdfileStartHubHostItem struct {
	Connection xdfileNetBoxConnection
	Index      int
}

func xdfileInitialScreen(showStartHub bool) xdfileScreen {
	if showStartHub {
		return xdfileScreenStartHub
	}
	return xdfileScreenWorkbench
}

func xdfileNewStartHubState() xdfileStartHubState {
	return xdfileStartHubState{
		Nav:          xdfileStartHubNavHosts,
		Cursor:       0,
		LastClickRow: -1,
	}
}

func (m *xdfileModel) openStartHub(nav xdfileStartHubNav) tea.Cmd {
	if m == nil {
		return nil
	}
	m.screen = xdfileScreenStartHub
	m.startHub.Nav = nav
	m.startHub.Cursor = 0
	m.startHub.LastClickRow = -1
	m.openMenu = ""
	m.clearMouseHover()
	m.setStatus("Start Hub")
	return nil
}

func (m *xdfileModel) openWorkbenchFromHub() tea.Cmd {
	if m == nil {
		return nil
	}
	m.screen = xdfileScreenWorkbench
	m.startHub.SearchActive = false
	m.openMenu = ""
	m.clearMouseHover()
	m.setStatus("Opened local workbench")
	m.reloadAllPanels()
	return nil
}

func (m *xdfileModel) startHubNavItems() []struct {
	nav   xdfileStartHubNav
	label string
} {
	return []struct {
		nav   xdfileStartHubNav
		label string
	}{
		{xdfileStartHubNavLocal, "Local"},
		{xdfileStartHubNavHosts, "Hosts"},
		{xdfileStartHubNavRecent, "Recent"},
		{xdfileStartHubNavSettings, "Settings"},
	}
}

func (m *xdfileModel) renderStartHub() string {
	m.layout.startHubNavRects = nil
	m.layout.startHubItemRects = nil

	header := m.renderStartHubHeader()
	body := m.renderStartHubBody(max(1, m.height-xdfileHeaderHeight-xdfileFooterHeight))
	footer := m.renderStartHubFooter()
	base := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

	if m.modal.Kind == xdfileModalNone {
		return m.finalizeView(base)
	}
	modal := m.renderModal()
	return m.finalizeView(stringfunction.PlaceOverlay(
		max(0, (m.width-lipgloss.Width(modal))/2),
		max(0, (m.height-lipgloss.Height(modal))/2),
		modal,
		base,
	))
}

func (m *xdfileModel) renderStartHubHeader() string {
	left := xdfileTitleStyle.Render(xdfileProductName)
	right := xdfileTagStyle.Render("Start Hub")
	line0 := xdfileJoinLeftRight(left, right, m.width)

	search := "Search /"
	if m.startHub.Search != "" || m.startHub.SearchActive {
		prefix := "Search "
		if m.startHub.SearchActive {
			prefix = "Search > "
		}
		search = prefix + m.startHub.Search
	}
	line1 := xdfileJoinLeftRight(
		xdfileDimStyle.Render("Enter opens | n new | e edit | d delete | l local"),
		xdfilePathStyle.Render(charmansi.Truncate(search, max(8, m.width/3), "...")),
		m.width,
	)
	return xdfileWrapANSIRender(lipgloss.JoinVertical(
		lipgloss.Left,
		xdfileHeaderLineStyle.Width(m.width).Render(line0),
		xdfileHeaderLineStyle.Width(m.width).Render(line1),
	))
}

func (m *xdfileModel) renderStartHubBody(height int) string {
	if height <= 0 {
		return ""
	}
	navWidth := xdfileStartHubNavWidth(m.width)
	detailVisible := m.width >= 112
	detailWidth := 0
	if detailVisible {
		detailWidth = max(28, min(38, m.width/3))
	}
	gaps := 1
	if detailVisible {
		gaps = 2
	}
	listWidth := max(20, m.width-navWidth-detailWidth-gaps)

	nav := m.renderStartHubNav(navWidth, height)
	list := m.renderStartHubList(listWidth, height)
	if !detailVisible {
		return lipgloss.JoinHorizontal(lipgloss.Top, nav, " ", list)
	}
	detail := m.renderStartHubDetail(detailWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, nav, " ", list, " ", detail)
}

func xdfileStartHubNavWidth(width int) int {
	switch {
	case width < 70:
		return 12
	case width < 100:
		return 14
	default:
		return 16
	}
}

func (m *xdfileModel) renderStartHubNav(width int, height int) string {
	innerW := max(8, width-2)
	contentH := max(1, height-2)
	lines := make([]string, 0, contentH)
	lines = append(lines, xdfileTitleStyle.Render("Xdfile"))
	lines = append(lines, xdfileDimStyle.Render("TUI workbench"))
	lines = append(lines, xdfileBlank(innerW))

	items := m.startHubNavItems()
	y := xdfileHeaderHeight + 4
	for _, item := range items {
		label := xdfilePadRight(item.label, innerW)
		style := xdfileMenuItemStyle
		if item.nav == m.startHub.Nav {
			style = xdfileMenuItemHot
		}
		lines = append(lines, style.Width(innerW).Render(label))
		m.layout.startHubNavRects = append(m.layout.startHubNavRects, xdfileButtonRect{
			Action: xdfileAction("start_hub_nav:" + strconv.Itoa(int(item.nav))),
			Rect:   xdfileRect{x: 1, y: y, w: innerW, h: 1},
		})
		y++
	}
	for len(lines) < contentH-2 {
		lines = append(lines, xdfileBlank(innerW))
	}
	lines = append(lines, xdfileDimStyle.Render("SSH + local"))
	lines = append(lines, xdfileDimStyle.Render("TUI native"))
	for len(lines) < contentH {
		lines = append(lines, xdfileBlank(innerW))
	}

	return xdfileWrapANSIRender(xdfilePanelBorder(false).
		Width(width - 2).
		Height(contentH).
		Render(strings.Join(lines[:contentH], "\n")))
}

func (m *xdfileModel) renderStartHubList(width int, height int) string {
	innerW := max(16, width-2)
	innerH := max(1, height-2)
	lines := make([]string, 0, innerH)

	title, subtitle := m.startHubListTitle()
	lines = append(lines, xdfileJoinLeftRight(
		xdfileTitleStyle.Render(title),
		xdfileDimStyle.Render(subtitle),
		innerW,
	))
	lines = append(lines, xdfileMetaStyle.Render(strings.Repeat("-", innerW)))

	switch m.startHub.Nav {
	case xdfileStartHubNavLocal:
		lines = m.appendStartHubLocalRows(lines, innerW)
	case xdfileStartHubNavHosts:
		lines = m.appendStartHubHostRows(lines, innerW)
	case xdfileStartHubNavRecent:
		lines = m.appendStartHubRecentRows(lines, innerW)
	case xdfileStartHubNavSettings:
		lines = m.appendStartHubSettingsRows(lines, innerW)
	}

	blank := xdfileBlank(innerW)
	for len(lines) < innerH {
		lines = append(lines, blank)
	}
	return xdfileWrapANSIRender(xdfilePanelBorder(true).
		Width(width - 2).
		Height(innerH).
		Render(strings.Join(lines[:innerH], "\n")))
}

func (m *xdfileModel) startHubListTitle() (string, string) {
	switch m.startHub.Nav {
	case xdfileStartHubNavLocal:
		return "Local", "open this machine"
	case xdfileStartHubNavRecent:
		return "Recent", "this session"
	case xdfileStartHubNavSettings:
		return "Settings", "runtime"
	default:
		count := len(m.startHubFilteredHosts())
		return "Hosts", fmt.Sprintf("%d saved", count)
	}
}

func (m *xdfileModel) appendStartHubLocalRows(lines []string, width int) []string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return append(lines, m.renderStartHubRow(
		0,
		"Open local workbench",
		cwd,
		"Two panels, terminal, F2 menu",
		width,
		xdfileStartHubLocalAction,
	))
}

func (m *xdfileModel) appendStartHubHostRows(lines []string, width int) []string {
	hosts := m.startHubFilteredHosts()
	if len(hosts) == 0 {
		message := "No saved SSH hosts. Press n to add one."
		if strings.TrimSpace(m.startHub.Search) != "" {
			message = "No hosts match the current search."
		}
		lines = append(lines, xdfileDimStyle.Render(charmansi.Truncate(message, width, "...")))
		return lines
	}
	m.startHub.Cursor = xdfileClamp(m.startHub.Cursor, 0, len(hosts)-1)
	for row, host := range hosts {
		action := xdfileAction(xdfileStartHubHostActionPrefix + strconv.Itoa(host.Index))
		title := host.Connection.Name
		subtitle := host.Connection.hostSubtitle()
		meta := host.Connection.authSubtitle()
		lines = append(lines, m.renderStartHubRow(row, title, subtitle, meta, width, action))
	}
	return lines
}

func (m *xdfileModel) appendStartHubRecentRows(lines []string, width int) []string {
	lines = append(lines,
		xdfileDimStyle.Render(charmansi.Truncate("Recent sessions are kept in memory for this run.", width, "...")),
		xdfileDimStyle.Render(charmansi.Truncate("Use Hosts to open a saved SSH workspace.", width, "...")),
	)
	return lines
}

func (m *xdfileModel) appendStartHubSettingsRows(lines []string, width int) []string {
	items := []struct {
		title    string
		subtitle string
		meta     string
		action   xdfileAction
	}{
		{"New SSH connection", "Create a saved host", "encrypted password optional", xdfileStartHubNewAction},
		{"Open local workbench", "Continue with local panels", "keeps current layout", xdfileStartHubLocalAction},
	}
	m.startHub.Cursor = xdfileClamp(m.startHub.Cursor, 0, len(items)-1)
	for i, item := range items {
		lines = append(lines, m.renderStartHubRow(i, item.title, item.subtitle, item.meta, width, item.action))
	}
	return lines
}

func (m *xdfileModel) renderStartHubRow(row int, title string, subtitle string, meta string, width int, action xdfileAction) string {
	selected := row == m.startHub.Cursor
	prefix := "  "
	if selected {
		prefix = "> "
	}
	titleW := max(8, width/3)
	metaW := max(10, width/4)
	bodyW := max(8, width-lipgloss.Width(prefix)-titleW-metaW-4)
	line := prefix +
		xdfilePadRight(charmansi.Truncate(title, titleW, "..."), titleW) + "  " +
		xdfilePadRight(charmansi.Truncate(subtitle, bodyW, "..."), bodyW) + "  " +
		xdfileAlignRight(charmansi.Truncate(meta, metaW, "..."), metaW)
	if selected {
		line = xdfileSelectedLineStyle(true).Render(xdfilePadRight(line, width))
	} else {
		line = xdfileMenuItemStyle.Render(xdfilePadRight(line, width))
	}
	m.layout.startHubItemRects = append(m.layout.startHubItemRects, xdfileButtonRect{
		Action: action,
		Rect: xdfileRect{
			x: xdfileStartHubNavWidth(m.width) + 2,
			y: xdfileHeaderHeight + 3 + row,
			w: width,
			h: 1,
		},
	})
	return line
}

func (m *xdfileModel) renderStartHubDetail(width int, height int) string {
	innerW := max(16, width-2)
	innerH := max(1, height-2)
	lines := make([]string, 0, innerH)
	lines = append(lines, xdfileTitleStyle.Render("Details"))
	lines = append(lines, xdfileMetaStyle.Render(strings.Repeat("-", innerW)))

	switch m.startHub.Nav {
	case xdfileStartHubNavHosts:
		if host, ok := m.selectedStartHubHost(); ok {
			c := host.Connection
			lines = append(lines,
				xdfileTagStyle.Render("Name")+" "+xdfilePathStyle.Render(c.Name),
				xdfileTagStyle.Render("Host")+" "+xdfilePathStyle.Render(c.hostSubtitle()),
				xdfileTagStyle.Render("Path")+" "+xdfilePathStyle.Render(c.RemotePath),
				xdfileTagStyle.Render("Auth")+" "+xdfilePathStyle.Render(c.authSubtitle()),
				"",
				xdfileDimStyle.Render("Enter open SSH workspace"),
				xdfileDimStyle.Render("e edit | d delete"),
			)
		} else {
			lines = append(lines, xdfileDimStyle.Render("No host selected"))
		}
	case xdfileStartHubNavLocal:
		lines = append(lines,
			xdfilePathStyle.Render("Open local workbench"),
			xdfileDimStyle.Render("Panels, terminal, F2 menu"),
			xdfileDimStyle.Render("No remote session started"),
		)
	default:
		lines = append(lines,
			xdfileDimStyle.Render("Keyboard"),
			xdfileDimStyle.Render("Tab switches sections"),
			xdfileDimStyle.Render("/ filters hosts"),
			xdfileDimStyle.Render("n creates SSH hosts"),
		)
	}

	blank := xdfileBlank(innerW)
	for len(lines) < innerH {
		lines = append(lines, blank)
	}
	return xdfileWrapANSIRender(xdfilePanelBorder(false).
		Width(width - 2).
		Height(innerH).
		Render(strings.Join(lines[:innerH], "\n")))
}

func (m *xdfileModel) renderStartHubFooter() string {
	line0 := xdfileJoinLeftRight(
		xdfileDimStyle.Render("Native TUI: Go + Bubble Tea + Lip Gloss"),
		m.renderStatusText(max(0, m.width/2)),
		m.width,
	)
	line1 := xdfileJoinLeftRight(
		xdfileButtonKeyStyle.Render("Enter")+" "+xdfileMenuItemStyle.Render("Open")+"  "+
			xdfileButtonKeyStyle.Render("N")+" "+xdfileMenuItemStyle.Render("New")+"  "+
			xdfileButtonKeyStyle.Render("/")+" "+xdfileMenuItemStyle.Render("Search"),
		xdfileButtonKeyStyle.Render("F10")+" "+xdfileMenuItemStyle.Render("Quit"),
		m.width,
	)
	return xdfileWrapANSIRender(lipgloss.JoinVertical(
		lipgloss.Left,
		xdfileFooterLineStyle.Width(m.width).Render(line0),
		xdfileFooterLineStyle.Width(m.width).Render(line1),
	))
}

func (m *xdfileModel) startHubFilteredHosts() []xdfileStartHubHostItem {
	query := strings.ToLower(strings.TrimSpace(m.startHub.Search))
	items := make([]xdfileStartHubHostItem, 0, len(m.netboxConnections))
	for i, connection := range m.netboxConnections {
		connection = connection.normalized()
		if query != "" && !connection.matchesStartHubQuery(query) {
			continue
		}
		items = append(items, xdfileStartHubHostItem{Connection: connection, Index: i})
	}
	return items
}

func (m *xdfileModel) selectedStartHubHost() (xdfileStartHubHostItem, bool) {
	hosts := m.startHubFilteredHosts()
	if len(hosts) == 0 {
		return xdfileStartHubHostItem{}, false
	}
	m.startHub.Cursor = xdfileClamp(m.startHub.Cursor, 0, len(hosts)-1)
	return hosts[m.startHub.Cursor], true
}

func (c xdfileNetBoxConnection) matchesStartHubQuery(query string) bool {
	text := strings.ToLower(strings.Join([]string{
		c.Name,
		c.Host,
		c.User,
		c.RemotePath,
		strconv.Itoa(c.Port),
	}, " "))
	return strings.Contains(text, query)
}

func (c xdfileNetBoxConnection) hostSubtitle() string {
	c = c.normalized()
	target := c.Host
	if c.User != "" {
		target = c.User + "@" + target
	}
	if c.Port > 0 && c.Port != 22 {
		target += ":" + strconv.Itoa(c.Port)
	}
	return target
}

func (c xdfileNetBoxConnection) authSubtitle() string {
	switch {
	case c.Password != "":
		return "password saved"
	case c.IdentityFile != "":
		return "identity file"
	default:
		return "ssh config"
	}
}

func (m *xdfileModel) handleStartHubKey(msg tea.KeyMsg) tea.Cmd {
	if m.startHub.SearchActive {
		return m.handleStartHubSearchKey(msg)
	}

	switch msg.String() {
	case "ctrl+c", "q", "f10":
		return m.openQuitConfirm()
	case "tab", "right":
		m.moveStartHubNav(1)
		return nil
	case "shift+tab", "left":
		m.moveStartHubNav(-1)
		return nil
	case "up":
		m.moveStartHubCursor(-1)
		return nil
	case "down":
		m.moveStartHubCursor(1)
		return nil
	case "home":
		m.startHub.Cursor = 0
		return nil
	case "end":
		m.startHub.Cursor = max(0, m.startHubItemCount()-1)
		return nil
	case "/":
		m.startHub.SearchActive = true
		m.startHub.Nav = xdfileStartHubNavHosts
		m.startHub.Cursor = 0
		return nil
	case "esc":
		if m.startHub.Search != "" {
			m.startHub.Search = ""
			m.startHub.Cursor = 0
			return nil
		}
		return nil
	case "n":
		return m.openNetBoxConnectionForm(nil)
	case "e":
		return m.editSelectedStartHubHost()
	case "d", "delete", "del":
		return m.deleteSelectedStartHubHost()
	case "l":
		return m.openWorkbenchFromHub()
	case "enter":
		return m.activateStartHubSelection()
	}
	return nil
}

func (m *xdfileModel) handleStartHubSearchKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		m.startHub.SearchActive = false
		return m.activateStartHubSelection()
	case "esc":
		m.startHub.SearchActive = false
		if m.startHub.Search != "" {
			m.startHub.Search = ""
			m.startHub.Cursor = 0
		}
		return nil
	case "backspace":
		if m.startHub.Search != "" {
			runes := []rune(m.startHub.Search)
			m.startHub.Search = string(runes[:len(runes)-1])
			m.startHub.Cursor = 0
		}
		return nil
	case "up":
		m.moveStartHubCursor(-1)
		return nil
	case "down":
		m.moveStartHubCursor(1)
		return nil
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		m.startHub.Search += string(msg.Runes)
		m.startHub.Cursor = 0
		return nil
	}
	return nil
}

func (m *xdfileModel) handleStartHubMouse(msg tea.MouseMsg) tea.Cmd {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return nil
	}
	for _, hit := range m.layout.startHubNavRects {
		if hit.Rect.contains(msg.X, msg.Y) {
			navText := strings.TrimPrefix(string(hit.Action), "start_hub_nav:")
			if navIndex, err := strconv.Atoi(navText); err == nil {
				m.startHub.Nav = xdfileStartHubNav(navIndex)
				m.startHub.Cursor = 0
				m.startHub.SearchActive = false
			}
			return nil
		}
	}
	for row, hit := range m.layout.startHubItemRects {
		if !hit.Rect.contains(msg.X, msg.Y) {
			continue
		}
		m.startHub.Cursor = row
		now := time.Now()
		if m.startHub.LastClickRow == row && now.Sub(m.startHub.LastClickAt) < 450*time.Millisecond {
			m.startHub.LastClickRow = -1
			return m.activateStartHubSelection()
		}
		m.startHub.LastClickRow = row
		m.startHub.LastClickAt = now
		return nil
	}
	return nil
}

func (m *xdfileModel) moveStartHubNav(delta int) {
	items := m.startHubNavItems()
	if len(items) == 0 {
		return
	}
	m.startHub.Nav = xdfileStartHubNav((int(m.startHub.Nav) + delta + len(items)) % len(items))
	m.startHub.Cursor = 0
	m.startHub.LastClickRow = -1
}

func (m *xdfileModel) startHubItemCount() int {
	switch m.startHub.Nav {
	case xdfileStartHubNavLocal:
		return 1
	case xdfileStartHubNavHosts:
		return len(m.startHubFilteredHosts())
	case xdfileStartHubNavSettings:
		return 2
	default:
		return 0
	}
}

func (m *xdfileModel) moveStartHubCursor(delta int) {
	count := m.startHubItemCount()
	if count <= 0 {
		m.startHub.Cursor = 0
		return
	}
	m.startHub.Cursor = (m.startHub.Cursor + delta + count) % count
}

func (m *xdfileModel) activateStartHubSelection() tea.Cmd {
	switch m.startHub.Nav {
	case xdfileStartHubNavLocal:
		return m.openWorkbenchFromHub()
	case xdfileStartHubNavHosts:
		host, ok := m.selectedStartHubHost()
		if !ok {
			m.setStatus("No SSH host selected")
			return nil
		}
		return m.openNetBoxConnectionWorkspace(host.Index)
	case xdfileStartHubNavSettings:
		if m.startHub.Cursor == 0 {
			return m.openNetBoxConnectionForm(nil)
		}
		return m.openWorkbenchFromHub()
	default:
		m.setStatus("No recent session selected")
		return nil
	}
}

func (m *xdfileModel) openNetBoxConnectionWorkspace(index int) tea.Cmd {
	connection, ok := m.netBoxConnectionAt(index)
	if !ok {
		m.setStatus("SSH connection not found")
		return nil
	}
	connection = connection.normalized()
	target := xdfileNetBoxURL(connection.Name, connection.RemotePath)
	workspace := xdfileWorkspace{
		Title:       connection.Name,
		ActivePanel: 0,
		TerminalCwd: target,
		Panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: target, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: target, RangeAnchor: -1},
		},
		PanelSearch: xdfilePanelSearchState{Panel: -1},
		PanelFilter: xdfilePanelFilterState{Panel: -1},
		PanelFuzzy:  xdfilePanelFuzzyState{Panel: -1},
	}
	m.ensureWorkspacesInitialized()
	m.captureActiveWorkspace()
	m.workspaces = append(m.workspaces, workspace)
	m.activeWorkspace = len(m.workspaces) - 1
	m.screen = xdfileScreenWorkbench
	m.startHub.SearchActive = false
	m.openMenu = ""
	m.closeModal()
	m.applyWorkspaceState(workspace)
	m.computeLayout()
	m.reloadAllPanels()
	m.setStatus("Opening SSH workspace %s", connection.Name)
	return m.startNetBoxInteractiveTerminal(connection, target)
}

func (m *xdfileModel) editSelectedStartHubHost() tea.Cmd {
	host, ok := m.selectedStartHubHost()
	if !ok {
		m.setStatus("No SSH host selected")
		return nil
	}
	connection := host.Connection
	return m.openNetBoxConnectionForm(&connection)
}

func (m *xdfileModel) deleteSelectedStartHubHost() tea.Cmd {
	host, ok := m.selectedStartHubHost()
	if !ok {
		m.setStatus("No SSH host selected")
		return nil
	}
	return m.confirmDeleteNetBoxConnection(host.Index)
}
