package cmd

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	charmansi "github.com/charmbracelet/x/ansi"
)

func TestXdfileStartHubRenderFitsTargetSizes(t *testing.T) {
	for _, size := range []struct {
		width  int
		height int
	}{
		{width: 80, height: 24},
		{width: 120, height: 32},
		{width: 160, height: 40},
	} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			m := startHubTestModel(size.width, size.height)
			view := m.View()
			stripped := strings.TrimSuffix(charmansi.Strip(view), xdfileANSIReset)
			if strings.Contains(strings.ToLower(stripped), "cloud") {
				t.Fatalf("Start Hub must not expose cloud or sync UI:\n%s", stripped)
			}
			lines := strings.Split(stripped, "\n")
			if len(lines) != size.height {
				t.Fatalf("view height = %d, want %d", len(lines), size.height)
			}
			for i, line := range lines {
				if got := lipgloss.Width(line); got != size.width {
					t.Fatalf("line %d width = %d, want %d: %q", i, got, size.width, line)
				}
			}
		})
	}
}

func TestXdfileStartHubSearchFiltersHosts(t *testing.T) {
	m := startHubTestModel(120, 32)
	m.startHub.Search = "prod"
	hosts := m.startHubFilteredHosts()
	if len(hosts) != 1 {
		t.Fatalf("filtered hosts = %d, want 1", len(hosts))
	}
	if got := hosts[0].Connection.Name; got != "prod" {
		t.Fatalf("filtered host = %q, want prod", got)
	}
}

func TestXdfileStartHubEnterHostOpensRemoteWorkspace(t *testing.T) {
	m := startHubTestModel(120, 32)
	m.startHub.Nav = xdfileStartHubNavHosts
	m.startHub.Cursor = 0

	originalReadEntries := xdfileNetBoxReadEntriesFunc
	originalStartTerminal := xdfileStartNetBoxInteractiveTerminalFunc
	defer func() {
		xdfileNetBoxReadEntriesFunc = originalReadEntries
		xdfileStartNetBoxInteractiveTerminalFunc = originalStartTerminal
	}()

	xdfileNetBoxReadEntriesFunc = func(dir string, _ bool, _ xdfileSortMode) ([]xdfileEntry, error) {
		return []xdfileEntry{{Name: "app", Path: dir + "/app", IsDir: true}}, nil
	}

	var startedConnection xdfileNetBoxConnection
	var startedTarget string
	xdfileStartNetBoxInteractiveTerminalFunc = func(connection xdfileNetBoxConnection, target string, width int, height int) tea.Cmd {
		startedConnection = connection
		startedTarget = target
		if width <= 0 || height <= 0 {
			t.Fatalf("invalid terminal size %dx%d", width, height)
		}
		return func() tea.Msg {
			return xdfileTerminalStartResultMsg{
				Dir:           target,
				Title:         connection.Name,
				RemoteProfile: connection.Name,
			}
		}
	}

	cmd := m.activateStartHubSelection()
	if cmd == nil {
		t.Fatal("expected SSH terminal start command")
	}
	if m.screen != xdfileScreenWorkbench {
		t.Fatalf("screen = %v, want workbench", m.screen)
	}
	if got := m.panels[0].Cwd; got != "xdssh://prod/srv" {
		t.Fatalf("left panel cwd = %q", got)
	}
	if got := m.panels[1].Cwd; got != "xdssh://prod/srv" {
		t.Fatalf("right panel cwd = %q", got)
	}
	if startedConnection.Name != "prod" || startedTarget != "xdssh://prod/srv" {
		t.Fatalf("started terminal for %#v target %q", startedConnection, startedTarget)
	}
}

func startHubTestModel(width int, height int) *xdfileModel {
	xdfileApplyTheme(xdfilePersona3Theme())
	cwd := "/tmp/xdfile"
	return &xdfileModel{
		width:       width,
		height:      height,
		screen:      xdfileScreenStartHub,
		startHub:    xdfileNewStartHubState(),
		statusText:  "Ready",
		activePanel: 0,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: cwd, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: cwd, RangeAnchor: -1},
		},
		terminal: xdfileTerminal{Cwd: cwd},
		netboxConnections: []xdfileNetBoxConnection{
			{Name: "prod", Host: "prod.example.com", User: "deploy", RemotePath: "/srv", Port: 22},
			{Name: "stage", Host: "stage.example.com", User: "deploy", RemotePath: "/srv", Port: 2222},
		},
		panelSearch: xdfilePanelSearchState{Panel: -1},
		panelFilter: xdfilePanelFilterState{Panel: -1},
		panelFuzzy:  xdfilePanelFuzzyState{Panel: -1},
	}
}
