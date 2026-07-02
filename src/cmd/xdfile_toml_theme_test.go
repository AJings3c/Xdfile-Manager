package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/s0x401/xdfile-manager/src/internal/common"
)

func TestXdfileLoadTOMLThemeFileMapsLegacyTokens(t *testing.T) {
	themePath := filepath.Join(t.TempDir(), "custom-dark.toml")
	mustWriteFile(t, themePath, `
full_screen_fg = "#f8f8f2"
full_screen_bg = "#101018"
file_panel_fg = "#eeeeee"
file_panel_bg = "#15151f"
file_panel_border = "#444466"
file_panel_border_active = "#88ccff"
file_panel_top_path = "#ffdd88"
file_panel_item_selected_fg = "#101018"
file_panel_item_selected_bg = "#88ccff"
footer_bg = "#202030"
cursor = "#ffd866"
correct = "#a6e22e"
error = "#f92672"
hint = "#75715e"
`)

	theme, err := xdfileLoadTOMLThemeFile(themePath)
	if err != nil {
		t.Fatalf("load TOML theme failed: %v", err)
	}
	if theme.Name != "custom-dark" {
		t.Fatalf("theme name = %q, want custom-dark", theme.Name)
	}
	if theme.BG != "#101018" || theme.Surface != "#15151F" {
		t.Fatalf("unexpected base colors: bg=%s surface=%s", theme.BG, theme.Surface)
	}
	if theme.Accent != "#88CCFF" || theme.Accent2 != "#FFDD88" {
		t.Fatalf("unexpected accents: accent=%s accent2=%s", theme.Accent, theme.Accent2)
	}
	if theme.TerminalInputCursor != "#FFD866" {
		t.Fatalf("terminal cursor = %s, want #FFD866", theme.TerminalInputCursor)
	}
}

func TestXdfileTOMLThemeFallbacksForMissingInvalidAndLowContrastColors(t *testing.T) {
	fallback := xdfilePersona3Theme()
	theme := xdfileTOMLThemeFromLegacy("risky", common.ThemeType{
		FullScreenFG:            "#111111",
		FullScreenBG:            "#111111",
		FilePanelFG:             "not-a-color",
		FilePanelBG:             "#111111",
		FilePanelItemSelectedFG: "#222222",
		FilePanelItemSelectedBG: "#222222",
		Cursor:                  "",
		Error:                   "nope",
	})

	if theme.Text != fallback.Text || theme.Surface != fallback.Surface {
		t.Fatalf("low contrast text/surface should fallback, got text=%s surface=%s", theme.Text, theme.Surface)
	}
	if theme.SelectionActiveFG != fallback.SelectionActiveFG || theme.SelectionActiveBG != fallback.SelectionActiveBG {
		t.Fatalf("low contrast selection should fallback, got fg=%s bg=%s", theme.SelectionActiveFG, theme.SelectionActiveBG)
	}
	if theme.Danger != fallback.Danger {
		t.Fatalf("invalid danger color should fallback, got %s", theme.Danger)
	}
	if theme.TerminalInputCursor == "" {
		t.Fatal("missing terminal cursor color should fallback to a non-empty token")
	}
}

func TestXdfileThemeCatalogIncludesTOMLThemesAfterPersona(t *testing.T) {
	themeDir := t.TempDir()
	writeMinimalTOMLTheme(t, filepath.Join(themeDir, "zeta.toml"))
	writeMinimalTOMLTheme(t, filepath.Join(themeDir, "alpha.toml"))
	mustWriteFile(t, filepath.Join(themeDir, "broken.toml"), `full_screen_fg = [`)

	catalog := xdfileThemeCatalog(themeDir)
	if len(catalog) != len(xdfilePersonaThemes())+2 {
		t.Fatalf("catalog size = %d, want persona themes + 2", len(catalog))
	}
	if catalog[0].Name != xdfileThemePersona3Name || catalog[4].Name != xdfileThemePersona5Name {
		t.Fatalf("persona themes should remain first, got first=%s fifth=%s", catalog[0].Name, catalog[4].Name)
	}
	if catalog[5].Name != "alpha" || catalog[6].Name != "zeta" {
		t.Fatalf("TOML themes should be sorted after personas, got %s then %s", catalog[5].Name, catalog[6].Name)
	}
}

func TestXdfileThemeMenuIncludesAndAppliesTOMLTheme(t *testing.T) {
	themeDir := t.TempDir()
	writeMinimalTOMLTheme(t, filepath.Join(themeDir, "mocha.toml"))

	m := &xdfileModel{
		layoutPrefs:  xdfileDefaultLayoutPrefs(),
		themeCatalog: xdfileThemeCatalog(themeDir),
		terminal: xdfileTerminal{
			Input: xdfileNewManagedTerminalInput(),
		},
	}

	var action xdfileAction
	for _, item := range m.themeMenuItems() {
		if strings.Contains(item.Label, "Mocha") {
			action = item.Action
			break
		}
	}
	if action == "" {
		t.Fatalf("theme menu did not include TOML theme: %#v", m.themeMenuItems())
	}
	if cmd := m.executeAction(action); cmd != nil {
		t.Fatal("theme action should not return an async command")
	}
	if m.layoutPrefs.ThemeName != "mocha" {
		t.Fatalf("layout theme = %q, want mocha", m.layoutPrefs.ThemeName)
	}
	if xdfileCurrentTheme.Name != "mocha" {
		t.Fatalf("current theme = %q, want mocha", xdfileCurrentTheme.Name)
	}
}

func TestXdfileLayoutPrefsPreserveTOMLThemeName(t *testing.T) {
	layoutPath := filepath.Join(t.TempDir(), "xdfile-layout.json")
	prefs := xdfileDefaultLayoutPrefs()
	prefs.ThemeName = "catppuccin-mocha"

	if err := xdfileSaveLayoutPrefs(layoutPath, prefs); err != nil {
		t.Fatalf("save layout prefs failed: %v", err)
	}
	loaded, err := xdfileLoadLayoutPrefs(layoutPath)
	if err != nil {
		t.Fatalf("load layout prefs failed: %v", err)
	}
	if loaded.ThemeName != "catppuccin-mocha" {
		t.Fatalf("loaded theme = %q, want catppuccin-mocha", loaded.ThemeName)
	}
}

func TestXdfileRuntimeConfigPreservesTOMLThemeDefault(t *testing.T) {
	config := xdfileRuntimeConfig{Theme: stringPointer("gruvbox")}
	applied := xdfileApplyRuntimeConfigLayoutDefaults(xdfileDefaultLayoutPrefs(), false, config)
	if applied.ThemeName != "gruvbox" {
		t.Fatalf("config TOML theme default = %q, want gruvbox", applied.ThemeName)
	}
}

func TestXdfileTOMLThemeTokenTableCoversRuntimeTokens(t *testing.T) {
	theme := xdfileTOMLThemeFromLegacy("minimal", common.ThemeType{
		FullScreenFG:            "#F8F8F2",
		FullScreenBG:            "#1E1F29",
		FilePanelFG:             "#F8F8F2",
		FilePanelBG:             "#282A36",
		FilePanelBorder:         "#6272A4",
		FilePanelBorderActive:   "#BD93F9",
		FilePanelItemSelectedFG: "#282A36",
		FilePanelItemSelectedBG: "#F8F8F2",
		Cursor:                  "#F8F8F2",
		Correct:                 "#50FA7B",
		Error:                   "#FF5555",
		Hint:                    "#8BE9FD",
	})
	tokens := xdfileThemeTokenTable(theme)
	if len(tokens) != 25 {
		t.Fatalf("theme token count = %d, want 25", len(tokens))
	}
	for _, token := range tokens {
		if token.Role == "" || token.Value == "" {
			t.Fatalf("empty token from TOML theme: %#v", token)
		}
	}
}

func writeMinimalTOMLTheme(t *testing.T, path string) {
	t.Helper()
	mustWriteFile(t, path, `
full_screen_fg = "#f8f8f2"
full_screen_bg = "#1e1f29"
file_panel_fg = "#f8f8f2"
file_panel_bg = "#282a36"
file_panel_border = "#6272a4"
file_panel_border_active = "#bd93f9"
file_panel_top_path = "#8be9fd"
file_panel_item_selected_fg = "#282a36"
file_panel_item_selected_bg = "#f8f8f2"
footer_bg = "#21222c"
cursor = "#f8f8f2"
correct = "#50fa7b"
error = "#ff5555"
hint = "#8be9fd"
`)
}
