package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	charmansi "github.com/charmbracelet/x/ansi"
)

type xdfileWorkspace struct {
	Title       string
	Panels      [2]xdfilePanel
	ActivePanel int
	TerminalCwd string
	PanelSearch xdfilePanelSearchState
	PanelFilter xdfilePanelFilterState
	PanelFuzzy  xdfilePanelFuzzyState
	QuickView   xdfileQuickView
}

func (m *xdfileModel) ensureWorkspacesInitialized() {
	if m == nil {
		return
	}
	if len(m.workspaces) == 0 {
		m.activeWorkspace = 0
		m.workspaces = []xdfileWorkspace{m.captureWorkspaceState("Workspace 1")}
		return
	}
	m.activeWorkspace = xdfileClamp(m.activeWorkspace, 0, len(m.workspaces)-1)
}

func (m *xdfileModel) initializeWorkspacesFromLayout(prefs xdfileLayoutPrefs, useSavedTabs bool) {
	if m == nil {
		return
	}
	prefs = prefs.normalized()
	if !useSavedTabs || len(prefs.WorkspaceTabs) == 0 {
		m.ensureWorkspacesInitialized()
		return
	}

	workspaces := make([]xdfileWorkspace, 0, len(prefs.WorkspaceTabs))
	for i, tab := range prefs.WorkspaceTabs {
		left := xdfileResolveWorkspaceLayoutPath(tab.LeftPath, m.panels[0].Cwd)
		right := xdfileResolveWorkspaceLayoutPath(tab.RightPath, left)
		title := strings.TrimSpace(tab.Title)
		if title == "" {
			title = fmt.Sprintf("Workspace %d", i+1)
		}
		panels := [2]xdfilePanel{
			{Label: "LEFT", Cwd: left, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: right, RangeAnchor: -1},
		}
		workspaces = append(workspaces, xdfileWorkspace{
			Title:       title,
			Panels:      panels,
			ActivePanel: xdfileClamp(tab.ActivePanel, 0, 1),
			TerminalCwd: panels[xdfileClamp(tab.ActivePanel, 0, 1)].Cwd,
			PanelSearch: xdfilePanelSearchState{Panel: -1},
			PanelFilter: xdfilePanelFilterState{Panel: -1},
			PanelFuzzy:  xdfilePanelFuzzyState{Panel: -1},
		})
	}
	if len(workspaces) == 0 {
		m.ensureWorkspacesInitialized()
		return
	}
	m.workspaces = workspaces
	m.activeWorkspace = xdfileClamp(prefs.ActiveWorkspaceIndex, 0, len(workspaces)-1)
	m.applyWorkspaceState(workspaces[m.activeWorkspace])
}

func xdfileResolveWorkspaceLayoutPath(value string, fallback string) string {
	if path, err := xdfileNormalizePath(value); err == nil {
		return path
	}
	if path, err := xdfileNormalizePath(fallback); err == nil {
		return path
	}
	return fallback
}

func (m *xdfileModel) captureActiveWorkspace() {
	if m == nil {
		return
	}
	m.ensureWorkspacesInitialized()
	m.workspaces[m.activeWorkspace] = m.captureWorkspaceState(m.workspaces[m.activeWorkspace].Title)
}

func (m *xdfileModel) captureWorkspaceState(title string) xdfileWorkspace {
	if strings.TrimSpace(title) == "" {
		title = fmt.Sprintf("Workspace %d", max(1, m.activeWorkspace+1))
	}
	return xdfileWorkspace{
		Title:       title,
		Panels:      xdfileClonePanels(m.panels),
		ActivePanel: xdfileClamp(m.activePanel, 0, 1),
		TerminalCwd: strings.TrimSpace(m.terminal.Cwd),
		PanelSearch: m.panelSearch,
		PanelFilter: m.panelFilter,
		PanelFuzzy:  m.panelFuzzy,
		QuickView:   m.quickView,
	}
}

func (m *xdfileModel) applyWorkspaceState(workspace xdfileWorkspace) {
	m.panels = xdfileClonePanels(workspace.Panels)
	m.activePanel = xdfileClamp(workspace.ActivePanel, 0, 1)
	m.panelSearch = workspace.PanelSearch
	m.panelFilter = workspace.PanelFilter
	m.panelFuzzy = workspace.PanelFuzzy
	m.quickView = workspace.QuickView
	if strings.TrimSpace(m.terminal.Cwd) != "" && workspace.TerminalCwd == "" {
		workspace.TerminalCwd = m.terminal.Cwd
	}
	if workspace.TerminalCwd == "" {
		workspace.TerminalCwd = m.panels[m.activePanel].Cwd
	}
	m.terminal.Cwd = workspace.TerminalCwd
	m.syncManagedTerminalPrompt()
}

func xdfileClonePanels(panels [2]xdfilePanel) [2]xdfilePanel {
	return [2]xdfilePanel{
		xdfileClonePanel(panels[0]),
		xdfileClonePanel(panels[1]),
	}
}

func xdfileClonePanel(panel xdfilePanel) xdfilePanel {
	panel.Entries = append([]xdfileEntry(nil), panel.Entries...)
	if panel.MarkedPaths != nil {
		marked := make(map[string]struct{}, len(panel.MarkedPaths))
		for path := range panel.MarkedPaths {
			marked[path] = struct{}{}
		}
		panel.MarkedPaths = marked
	}
	return panel
}

func (m *xdfileModel) workspaceLayoutsForSave() []xdfileWorkspaceLayout {
	if m == nil {
		return nil
	}
	m.captureActiveWorkspace()
	layouts := make([]xdfileWorkspaceLayout, 0, len(m.workspaces))
	for i, workspace := range m.workspaces {
		title := strings.TrimSpace(workspace.Title)
		if title == "" {
			title = fmt.Sprintf("Workspace %d", i+1)
		}
		layouts = append(layouts, xdfileWorkspaceLayout{
			Title:       title,
			LeftPath:    workspace.Panels[0].Cwd,
			RightPath:   workspace.Panels[1].Cwd,
			ActivePanel: xdfileClamp(workspace.ActivePanel, 0, 1),
		})
	}
	if len(layouts) <= 1 {
		return nil
	}
	return layouts
}

func (m *xdfileModel) newWorkspace() tea.Cmd {
	if m.workspaceSwitchBlocked() {
		return nil
	}
	m.captureActiveWorkspace()
	baseLeft := m.panels[m.activePanel].Cwd
	baseRight := baseLeft
	index := len(m.workspaces)
	workspace := xdfileWorkspace{
		Title:       fmt.Sprintf("Workspace %d", index+1),
		ActivePanel: 0,
		TerminalCwd: baseLeft,
		Panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: baseLeft, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: baseRight, RangeAnchor: -1},
		},
		PanelSearch: xdfilePanelSearchState{Panel: -1},
		PanelFilter: xdfilePanelFilterState{Panel: -1},
		PanelFuzzy:  xdfilePanelFuzzyState{Panel: -1},
	}
	m.workspaces = append(m.workspaces, workspace)
	m.activeWorkspace = index
	m.applyWorkspaceState(workspace)
	m.reloadAllPanels()
	m.setStatus("Created workspace tab %d", index+1)
	return nil
}

func (m *xdfileModel) closeWorkspace() tea.Cmd {
	if m.workspaceSwitchBlocked() {
		return nil
	}
	m.ensureWorkspacesInitialized()
	if len(m.workspaces) <= 1 {
		m.resetCurrentWorkspace()
		m.setStatus("Reset the last workspace tab")
		return nil
	}
	closed := m.activeWorkspace
	m.workspaces = append(m.workspaces[:closed], m.workspaces[closed+1:]...)
	if closed >= len(m.workspaces) {
		closed = len(m.workspaces) - 1
	}
	m.activeWorkspace = closed
	m.applyWorkspaceState(m.workspaces[m.activeWorkspace])
	m.reloadAllPanels()
	m.setStatus("Closed workspace tab")
	return nil
}

func (m *xdfileModel) resetCurrentWorkspace() {
	cwd := m.panels[m.activePanel].Cwd
	workspace := xdfileWorkspace{
		Title:       "Workspace 1",
		ActivePanel: 0,
		TerminalCwd: cwd,
		Panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: cwd, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: cwd, RangeAnchor: -1},
		},
		PanelSearch: xdfilePanelSearchState{Panel: -1},
		PanelFilter: xdfilePanelFilterState{Panel: -1},
		PanelFuzzy:  xdfilePanelFuzzyState{Panel: -1},
	}
	m.workspaces = []xdfileWorkspace{workspace}
	m.activeWorkspace = 0
	m.applyWorkspaceState(workspace)
	m.reloadAllPanels()
}

func (m *xdfileModel) nextWorkspace() tea.Cmd {
	return m.switchWorkspace(1)
}

func (m *xdfileModel) previousWorkspace() tea.Cmd {
	return m.switchWorkspace(-1)
}

func (m *xdfileModel) switchWorkspace(delta int) tea.Cmd {
	if m.workspaceSwitchBlocked() {
		return nil
	}
	m.ensureWorkspacesInitialized()
	if len(m.workspaces) <= 1 {
		m.setStatus("Only one workspace tab")
		return nil
	}
	m.captureActiveWorkspace()
	m.activeWorkspace = (m.activeWorkspace + delta + len(m.workspaces)) % len(m.workspaces)
	m.applyWorkspaceState(m.workspaces[m.activeWorkspace])
	m.reloadAllPanels()
	m.setStatus("Switched to workspace tab %d/%d", m.activeWorkspace+1, len(m.workspaces))
	return nil
}

func (m *xdfileModel) workspaceSwitchBlocked() bool {
	if m == nil {
		return true
	}
	if m.terminal.Busy || m.terminalUsesPTY() || m.exclusiveTerminalActive() || m.backgroundTaskBusy {
		m.setStatus("Workspace tabs are unavailable while a terminal or background task is active")
		return true
	}
	return false
}

func (m *xdfileModel) workspaceHeaderLabel(width int) string {
	if m == nil {
		return ""
	}
	m.ensureWorkspacesInitialized()
	workspace := m.workspaces[m.activeWorkspace]
	title := strings.TrimSpace(workspace.Title)
	if title == "" {
		title = filepath.Base(m.panels[m.activePanel].Cwd)
	}
	if title == "." || title == string(filepath.Separator) || title == "" {
		title = fmt.Sprintf("Workspace %d", m.activeWorkspace+1)
	}
	label := fmt.Sprintf("Tab %d/%d %s", m.activeWorkspace+1, len(m.workspaces), title)
	if width <= 0 {
		return label
	}
	return charmansi.Truncate(label, width, "...")
}
