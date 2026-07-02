package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	charmansi "github.com/charmbracelet/x/ansi"
)

func TestXdfileFunctionButtonsFitTargetWidths(t *testing.T) {
	cases := []struct {
		name      string
		width     int
		ctrlHints bool
	}{
		{name: "primary 80", width: 80},
		{name: "primary 120", width: 120},
		{name: "ctrl 80", width: 80, ctrlHints: true},
		{name: "ctrl 120", width: 120, ctrlHints: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &xdfileModel{width: tc.width}
			line, hits := m.renderFunctionButtons(xdfileFooterButtons("Quick view", tc.ctrlHints), 1)
			if got := lipgloss.Width(line); got != tc.width {
				t.Fatalf("footer line width = %d, want %d\n%s", got, tc.width, charmansi.Strip(line))
			}
			if len(hits) == 0 {
				t.Fatal("expected at least one footer hit rect")
			}
			for _, hit := range hits {
				if hit.Rect.x < 0 || hit.Rect.w <= 0 || hit.Rect.x+hit.Rect.w > tc.width {
					t.Fatalf("footer hit rect overflows width %d: %#v", tc.width, hit.Rect)
				}
			}
		})
	}
}

func TestXdfileMenuContentWidthClampsToWindow(t *testing.T) {
	if got := xdfileMenuContentWidth(200, 80); got != 78 {
		t.Fatalf("menu content width = %d, want 78", got)
	}
	if got := xdfileMenuContentWidth(18, 80); got != 18 {
		t.Fatalf("menu content width = %d, want 18", got)
	}
}

func TestXdfileOpenMenuClampsLongItemsToWindow(t *testing.T) {
	m := &xdfileModel{
		width:    40,
		height:   12,
		openMenu: xdfileActionContextMenu,
		contextMenu: xdfileMenu{
			Action: xdfileActionContextMenu,
			Label:  "Context",
			Items: []xdfileButton{
				{Action: xdfileActionCopySelectedPaths, Key: "Ctrl+Shift+C", Label: "Copy selected paths with a deliberately long label"},
				{Action: xdfileActionPaste, Key: "Ctrl+Shift+V", Label: "Paste"},
			},
		},
		contextMenuAnchor: xdfileRect{x: 30, y: 3, w: 1, h: 1},
	}

	menu := m.renderOpenMenu()
	if menu == "" {
		t.Fatal("expected rendered menu")
	}
	if m.layout.menuRect.x < 0 || m.layout.menuRect.x+m.layout.menuRect.w > m.width {
		t.Fatalf("menu rect overflows width %d: %#v", m.width, m.layout.menuRect)
	}
	for i, line := range strings.Split(charmansi.Strip(menu), "\n") {
		if got := lipgloss.Width(line); got > m.width {
			t.Fatalf("menu line %d width = %d, want <= %d: %q", i, got, m.width, line)
		}
	}
}

func TestXdfileStatusTextCompactsCommonPrompts(t *testing.T) {
	if got := xdfileCompactStatusText("Press Enter to confirm or Esc to cancel"); got != "Enter confirm | Esc cancel" {
		t.Fatalf("unexpected compact confirm text: %q", got)
	}
	if got := xdfileCompactStatusText("Choose Replace, Skip, Keep both, or Apply all"); got != "Choose conflict action" {
		t.Fatalf("unexpected compact conflict text: %q", got)
	}

	m := &xdfileModel{statusText: "Press Enter to confirm or Esc to cancel"}
	rendered := m.renderStatusText(24)
	if lipgloss.Width(rendered) > 24 {
		t.Fatalf("status width overflow: %d > 24", lipgloss.Width(rendered))
	}
	if stripped := charmansi.Strip(rendered); !strings.Contains(stripped, "Enter") {
		t.Fatalf("expected compact status text, got %q", stripped)
	}
}

func TestXdfileSortModeLabelUsesShortCopy(t *testing.T) {
	if got := xdfileSortModeLabel(xdfileSortModeExt); got != "Ext" {
		t.Fatalf("sort label = %q, want Ext", got)
	}
}

func TestXdfileThemeTokenTableCoversRuntimeTokens(t *testing.T) {
	themes := []xdfileTheme{
		xdfilePersona3Theme(),
		xdfilePersona3ReloadTheme(),
		xdfilePersona3KotoneTheme(),
		xdfilePersona4Theme(),
		xdfilePersona5Theme(),
	}
	for _, theme := range themes {
		t.Run(theme.Name, func(t *testing.T) {
			tokens := xdfileThemeTokenTable(theme)
			if len(tokens) != 25 {
				t.Fatalf("theme token count = %d, want 25", len(tokens))
			}
			seen := make(map[string]struct{}, len(tokens))
			for _, token := range tokens {
				if token.Role == "" || token.Value == "" {
					t.Fatalf("empty theme token: %#v", token)
				}
				if _, ok := seen[token.Role]; ok {
					t.Fatalf("duplicate theme token role: %s", token.Role)
				}
				seen[token.Role] = struct{}{}
			}
		})
	}
}

func TestXdfileLayoutRectsDoNotOverlapTargetSizes(t *testing.T) {
	for _, size := range []struct {
		width  int
		height int
	}{
		{width: 80, height: 24},
		{width: 120, height: 32},
		{width: 160, height: 40},
	} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			m := baselineRenderModel(size.width, size.height)
			m.computeLayout()

			left := m.layout.panelRects[0]
			right := m.layout.panelRects[1]
			terminal := m.layout.terminalRect
			if left.y != xdfileHeaderHeight || right.y != xdfileHeaderHeight {
				t.Fatalf("panel y should start below header: left=%#v right=%#v", left, right)
			}
			if left.x < 0 || right.x <= left.x || left.x+left.w > right.x {
				t.Fatalf("panel rects overlap or are out of order: left=%#v right=%#v", left, right)
			}
			if right.x+right.w > size.width {
				t.Fatalf("right panel overflows width %d: %#v", size.width, right)
			}
			if terminal.y < left.y+left.h || terminal.y+terminal.h > size.height-xdfileFooterHeight {
				t.Fatalf("terminal overlaps panels/footer: panel=%#v terminal=%#v size=%dx%d", left, terminal, size.width, size.height)
			}

			view := m.View()
			lines := strings.Split(strings.TrimSuffix(charmansi.Strip(view), xdfileANSIReset), "\n")
			if len(lines) != size.height {
				t.Fatalf("view height = %d, want %d", len(lines), size.height)
			}
			for i, line := range lines {
				if got := lipgloss.Width(line); got != size.width {
					t.Fatalf("line %d width = %d, want %d", i, got, size.width)
				}
			}
		})
	}
}

func baselineRenderModel(width int, height int) *xdfileModel {
	input := textinput.New()
	input.Prompt = "XD> "
	input.Width = 20

	leftEntries := []xdfileEntry{
		{Name: "..", Path: "/tmp", IsDir: true, IsParent: true},
		{Name: "README.md", Path: "/tmp/project/README.md"},
		{Name: "src", Path: "/tmp/project/src", IsDir: true},
	}
	rightEntries := []xdfileEntry{
		{Name: "..", Path: "/tmp", IsDir: true, IsParent: true},
		{Name: "remote.log", Path: "xdssh://prod/var/log/remote.log"},
		{Name: "dist", Path: "/tmp/project/dist", IsDir: true},
	}

	return &xdfileModel{
		width:       width,
		height:      height,
		activePanel: 0,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "Left", Cwd: "/tmp/project", Entries: leftEntries, Cursor: 1, RangeAnchor: -1},
			{Label: "Right", Cwd: "/tmp/project/dist", Entries: rightEntries, Cursor: 1, RangeAnchor: -1},
		},
		terminal: xdfileTerminal{
			Cwd:   "/tmp/project",
			Input: input,
		},
	}
}
