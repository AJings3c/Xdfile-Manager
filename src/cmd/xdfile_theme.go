package cmd

import (
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/pelletier/go-toml/v2"
	variable "github.com/s0x401/xdfile-manager/src/config"
	"github.com/s0x401/xdfile-manager/src/internal/common"
)

type xdfileTheme struct {
	Name                     string
	BG                       string
	Surface                  string
	Surface2                 string
	Border                   string
	Accent                   string
	Accent2                  string
	Text                     string
	Dim                      string
	Success                  string
	Danger                   string
	Highlight                string
	SelectionActiveBG        string
	SelectionActiveFG        string
	SelectionInactiveBG      string
	SelectionInactiveFG      string
	MarkedActiveBG           string
	MarkedActiveFG           string
	MarkedInactiveBG         string
	MarkedInactiveFG         string
	TerminalPromptPath       string
	TerminalInputCursor      string
	TerminalSuggestion       string
	TerminalSuggestionCursor string
	TerminalCursorForeground string
	TerminalCursorBackground string
}

type xdfileThemeToken struct {
	Role  string
	Value string
}

const (
	xdfileThemePersona3Name       = "persona3"
	xdfileThemePersona3ReloadName = "persona3reload"
	xdfileThemePersona3KotoneName = "persona3kotone"
	xdfileThemePersona4Name       = "persona4"
	xdfileThemePersona5Name       = "persona5"

	xdfileThemeSelectActionPrefix = "theme_select:"
)

var xdfileCurrentTheme = xdfilePersona3Theme()

func init() {
	xdfileApplyTheme(xdfileCurrentTheme)
}

func xdfilePersona3Theme() xdfileTheme {
	return xdfileTheme{
		Name:                     xdfileThemePersona3Name,
		BG:                       "#071827",
		Surface:                  "#0D2236",
		Surface2:                 "#14314A",
		Border:                   "#377FB2",
		Accent:                   "#59D5FF",
		Accent2:                  "#F2FCFF",
		Text:                     "#F7FCFF",
		Dim:                      "#A7CBE6",
		Success:                  "#8CFAFF",
		Danger:                   "#FF9DB5",
		Highlight:                "#245A85",
		SelectionActiveBG:        "#58CFFF",
		SelectionActiveFG:        "#04131F",
		SelectionInactiveBG:      "#234B6E",
		SelectionInactiveFG:      "#F2FBFF",
		MarkedActiveBG:           "#2A628E",
		MarkedActiveFG:           "#F2FCFF",
		MarkedInactiveBG:         "#1D4567",
		MarkedInactiveFG:         "#B5E9FF",
		TerminalPromptPath:       "#DDF3FF",
		TerminalInputCursor:      "#B9F3FF",
		TerminalSuggestion:       "#6C95B2",
		TerminalSuggestionCursor: "#F2FCFF",
		TerminalCursorForeground: "#071827",
		TerminalCursorBackground: "#B9F3FF",
	}
}

func xdfilePersona3ReloadTheme() xdfileTheme {
	return xdfileTheme{
		Name:                     xdfileThemePersona3ReloadName,
		BG:                       "#0A1924",
		Surface:                  "#122838",
		Surface2:                 "#1B3D55",
		Border:                   "#62AEDD",
		Accent:                   "#92E7FF",
		Accent2:                  "#FFFFFF",
		Text:                     "#FBFDFF",
		Dim:                      "#B4D3E6",
		Success:                  "#A7F8FF",
		Danger:                   "#FFADC2",
		Highlight:                "#2E6C97",
		SelectionActiveBG:        "#9AE8FF",
		SelectionActiveFG:        "#071722",
		SelectionInactiveBG:      "#34698C",
		SelectionInactiveFG:      "#FBFDFF",
		MarkedActiveBG:           "#34719B",
		MarkedActiveFG:           "#FFFFFF",
		MarkedInactiveBG:         "#255573",
		MarkedInactiveFG:         "#C3EEFF",
		TerminalPromptPath:       "#E7F7FF",
		TerminalInputCursor:      "#C8F7FF",
		TerminalSuggestion:       "#7DA3BC",
		TerminalSuggestionCursor: "#FFFFFF",
		TerminalCursorForeground: "#08151F",
		TerminalCursorBackground: "#C8F7FF",
	}
}

func xdfilePersona3KotoneTheme() xdfileTheme {
	return xdfileTheme{
		Name:                     xdfileThemePersona3KotoneName,
		BG:                       "#12101D",
		Surface:                  "#1D1528",
		Surface2:                 "#2D1D37",
		Border:                   "#7661A7",
		Accent:                   "#FF6FAE",
		Accent2:                  "#FFE8F4",
		Text:                     "#FFF7FB",
		Dim:                      "#D8B7CB",
		Success:                  "#FFC6E0",
		Danger:                   "#FF8C8C",
		Highlight:                "#4B274A",
		SelectionActiveBG:        "#FF7AB8",
		SelectionActiveFG:        "#1B0C17",
		SelectionInactiveBG:      "#6F3A5F",
		SelectionInactiveFG:      "#FFEAF4",
		MarkedActiveBG:           "#9D3F73",
		MarkedActiveFG:           "#FFF4FA",
		MarkedInactiveBG:         "#5B2B50",
		MarkedInactiveFG:         "#F4C7DC",
		TerminalPromptPath:       "#FFD2E5",
		TerminalInputCursor:      "#FF9BCB",
		TerminalSuggestion:       "#C28BAA",
		TerminalSuggestionCursor: "#FFF1F7",
		TerminalCursorForeground: "#1A0C16",
		TerminalCursorBackground: "#FF9BCB",
	}
}

func xdfilePersona4Theme() xdfileTheme {
	return xdfileTheme{
		Name:                     xdfileThemePersona4Name,
		BG:                       "#11110A",
		Surface:                  "#1D1B0F",
		Surface2:                 "#2F2A12",
		Border:                   "#C8A728",
		Accent:                   "#FFD84A",
		Accent2:                  "#FFF7B8",
		Text:                     "#FFF9D8",
		Dim:                      "#D7C776",
		Success:                  "#F5F06A",
		Danger:                   "#FF8A6A",
		Highlight:                "#4C4117",
		SelectionActiveBG:        "#FFD94D",
		SelectionActiveFG:        "#181407",
		SelectionInactiveBG:      "#705A18",
		SelectionInactiveFG:      "#FFF6C8",
		MarkedActiveBG:           "#765A12",
		MarkedActiveFG:           "#FFF7D0",
		MarkedInactiveBG:         "#574815",
		MarkedInactiveFG:         "#F2DD7A",
		TerminalPromptPath:       "#FFF3A6",
		TerminalInputCursor:      "#FFE66D",
		TerminalSuggestion:       "#C8A94F",
		TerminalSuggestionCursor: "#FFF7B8",
		TerminalCursorForeground: "#171307",
		TerminalCursorBackground: "#FFE66D",
	}
}

func xdfilePersona5Theme() xdfileTheme {
	return xdfileTheme{
		Name:                     xdfileThemePersona5Name,
		BG:                       "#08090D",
		Surface:                  "#121419",
		Surface2:                 "#1B1F26",
		Border:                   "#C52731",
		Accent:                   "#E63B45",
		Accent2:                  "#FFF4F1",
		Text:                     "#FFF7F3",
		Dim:                      "#C8B8B2",
		Success:                  "#FFB8BE",
		Danger:                   "#FF7A86",
		Highlight:                "#6E111A",
		SelectionActiveBG:        "#F5EDE8",
		SelectionActiveFG:        "#111216",
		SelectionInactiveBG:      "#7B1A24",
		SelectionInactiveFG:      "#FFF7F3",
		MarkedActiveBG:           "#A51F2A",
		MarkedActiveFG:           "#FFF8F5",
		MarkedInactiveBG:         "#521118",
		MarkedInactiveFG:         "#F6DFD9",
		TerminalPromptPath:       "#FFE3DD",
		TerminalInputCursor:      "#FFF4F1",
		TerminalSuggestion:       "#C48D92",
		TerminalSuggestionCursor: "#FFF7F3",
		TerminalCursorForeground: "#090A0D",
		TerminalCursorBackground: "#FFF4F1",
	}
}

func xdfilePersonaThemes() []xdfileTheme {
	return []xdfileTheme{
		xdfilePersona3Theme(),
		xdfilePersona3ReloadTheme(),
		xdfilePersona3KotoneTheme(),
		xdfilePersona4Theme(),
		xdfilePersona5Theme(),
	}
}

func xdfileThemeByName(name string) xdfileTheme {
	themes := xdfileThemeCatalog(variable.ThemeFolder)
	if theme, ok := xdfileThemeFromCatalog(name, themes); ok {
		return theme
	}
	return xdfilePersona3Theme()
}

func xdfileThemeCatalog(themeDir string) []xdfileTheme {
	themes := xdfilePersonaThemes()
	seen := make(map[string]struct{}, len(themes))
	for _, theme := range themes {
		seen[theme.Name] = struct{}{}
	}

	for _, theme := range xdfileLoadTOMLThemes(themeDir) {
		if _, ok := seen[theme.Name]; ok {
			continue
		}
		seen[theme.Name] = struct{}{}
		themes = append(themes, theme)
	}
	return themes
}

func xdfileLoadTOMLThemes(themeDir string) []xdfileTheme {
	themeDir = strings.TrimSpace(themeDir)
	if themeDir == "" {
		return nil
	}
	entries, err := os.ReadDir(themeDir)
	if err != nil {
		return nil
	}

	themes := make([]xdfileTheme, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".toml") {
			continue
		}
		theme, err := xdfileLoadTOMLThemeFile(filepath.Join(themeDir, entry.Name()))
		if err != nil {
			continue
		}
		themes = append(themes, theme)
	}
	sort.SliceStable(themes, func(i int, j int) bool {
		return strings.ToLower(themes[i].Name) < strings.ToLower(themes[j].Name)
	})
	return themes
}

func xdfileLoadTOMLThemeFile(path string) (xdfileTheme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return xdfileTheme{}, err
	}
	var legacy common.ThemeType
	if err := toml.Unmarshal(data, &legacy); err != nil {
		return xdfileTheme{}, err
	}
	name := xdfileNormalizeThemeName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	return xdfileTOMLThemeFromLegacy(name, legacy), nil
}

func xdfileThemeFromCatalog(name string, themes []xdfileTheme) (xdfileTheme, bool) {
	normalized := xdfileNormalizeThemeName(name)
	for _, theme := range themes {
		if theme.Name == normalized {
			return theme, true
		}
	}
	return xdfileTheme{}, false
}

func xdfilePersonaThemeByName(name string) (xdfileTheme, bool) {
	switch xdfileNormalizeThemeName(name) {
	case xdfileThemePersona5Name:
		return xdfilePersona5Theme(), true
	case xdfileThemePersona4Name:
		return xdfilePersona4Theme(), true
	case xdfileThemePersona3KotoneName:
		return xdfilePersona3KotoneTheme(), true
	case xdfileThemePersona3ReloadName:
		return xdfilePersona3ReloadTheme(), true
	case xdfileThemePersona3Name:
		return xdfilePersona3Theme(), true
	default:
		return xdfileTheme{}, false
	}
}

func xdfileNormalizeThemeName(name string) string {
	cleaned := strings.TrimSpace(name)
	cleaned = strings.TrimSuffix(cleaned, ".toml")
	cleaned = strings.TrimSuffix(cleaned, ".TOML")
	cleaned = filepath.Base(strings.ReplaceAll(cleaned, "\\", string(filepath.Separator)))
	cleaned = strings.ToLower(strings.TrimSpace(cleaned))
	switch cleaned {
	case "", xdfileThemePersona3Name, "persona-3", "p3":
		return xdfileThemePersona3Name
	case xdfileThemePersona5Name, "persona-5", "p5":
		return xdfileThemePersona5Name
	case xdfileThemePersona4Name, "persona-4", "p4":
		return xdfileThemePersona4Name
	case xdfileThemePersona3ReloadName, "persona3-reload", "p3r":
		return xdfileThemePersona3ReloadName
	case xdfileThemePersona3KotoneName, "persona3-kotone", "p3p", "kotone", "shiomi":
		return xdfileThemePersona3KotoneName
	default:
		return xdfileSanitizeThemeName(cleaned)
	}
}

func xdfileThemeDisplayName(name string) string {
	switch xdfileNormalizeThemeName(name) {
	case xdfileThemePersona5Name:
		return "Persona 5"
	case xdfileThemePersona4Name:
		return "Persona 4"
	case xdfileThemePersona3KotoneName:
		return "Persona 3 Kotone"
	case xdfileThemePersona3ReloadName:
		return "Persona 3 Reload"
	default:
		return xdfileHumanizeThemeName(name)
	}
}

func xdfileThemeMenuLabel(name string, current string) string {
	label := xdfileThemeDisplayName(name)
	if xdfileNormalizeThemeName(name) == xdfileNormalizeThemeName(current) {
		return label + " (Current)"
	}
	return label
}

func xdfileThemeSelectAction(name string) xdfileAction {
	return xdfileAction(xdfileThemeSelectActionPrefix + xdfileNormalizeThemeName(name))
}

func xdfileParseThemeSelectAction(action xdfileAction) (string, bool) {
	value := string(action)
	if !strings.HasPrefix(value, xdfileThemeSelectActionPrefix) {
		return "", false
	}
	name := xdfileNormalizeThemeName(strings.TrimPrefix(value, xdfileThemeSelectActionPrefix))
	if name == "" {
		return "", false
	}
	return name, true
}

func xdfileThemeMenuAction(name string) xdfileAction {
	switch xdfileNormalizeThemeName(name) {
	case xdfileThemePersona3Name:
		return xdfileActionThemePersona3
	case xdfileThemePersona3ReloadName:
		return xdfileActionThemePersona3Reload
	case xdfileThemePersona3KotoneName:
		return xdfileActionThemePersona3Kotone
	case xdfileThemePersona4Name:
		return xdfileActionThemePersona4
	case xdfileThemePersona5Name:
		return xdfileActionThemePersona5
	default:
		return xdfileThemeSelectAction(name)
	}
}

func xdfileSanitizeThemeName(name string) string {
	var b strings.Builder
	previousDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '.':
			b.WriteRune(r)
			previousDash = false
		case r == '-':
			if !previousDash {
				b.WriteRune(r)
				previousDash = true
			}
		case r == ' ':
			if !previousDash {
				b.WriteByte('-')
				previousDash = true
			}
		}
	}
	cleaned := strings.Trim(b.String(), "-._")
	if cleaned == "" {
		return xdfileThemePersona3Name
	}
	return cleaned
}

func xdfileHumanizeThemeName(name string) string {
	name = xdfileNormalizeThemeName(name)
	if _, ok := xdfilePersonaThemeByName(name); ok {
		switch name {
		case xdfileThemePersona5Name:
			return "Persona 5"
		case xdfileThemePersona4Name:
			return "Persona 4"
		case xdfileThemePersona3KotoneName:
			return "Persona 3 Kotone"
		case xdfileThemePersona3ReloadName:
			return "Persona 3 Reload"
		default:
			return "Persona 3"
		}
	}
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	if len(parts) == 0 {
		return name
	}
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func xdfileTOMLThemeFromLegacy(name string, legacy common.ThemeType) xdfileTheme {
	fallback := xdfilePersona3Theme()
	gradient := func(index int) string {
		if index < 0 || index >= len(legacy.GradientColor) {
			return ""
		}
		return legacy.GradientColor[index]
	}
	pick := func(fallback string, values ...string) string {
		for _, value := range values {
			if colorValue, ok := xdfileNormalizeThemeColor(value); ok {
				return colorValue
			}
		}
		return fallback
	}

	theme := xdfileTheme{
		Name:                     xdfileNormalizeThemeName(name),
		BG:                       pick(fallback.BG, legacy.FullScreenBG, legacy.FilePanelBG, legacy.ModalBG),
		Surface:                  pick(fallback.Surface, legacy.FilePanelBG, legacy.FullScreenBG),
		Surface2:                 pick(fallback.Surface2, legacy.FooterBG, legacy.ModalBG, legacy.SidebarBG, legacy.FilePanelBG),
		Border:                   pick(fallback.Border, legacy.FilePanelBorder, legacy.FooterBorder, legacy.SidebarBorder),
		Accent:                   pick(fallback.Accent, legacy.FilePanelBorderActive, legacy.FilePanelTopDirectoryIcon, legacy.DirectoryIconColor, gradient(0), legacy.FooterBorderActive, legacy.ModalBorderActive),
		Accent2:                  pick(fallback.Accent2, legacy.FilePanelTopPath, legacy.HelpMenuTitle, gradient(1), legacy.FooterBorderActive, legacy.ModalBorderActive),
		Text:                     pick(fallback.Text, legacy.FilePanelFG, legacy.FullScreenFG, legacy.ModalFG),
		Dim:                      pick(fallback.Dim, legacy.Hint, legacy.FooterBorder, legacy.SidebarDivider, legacy.Cancel),
		Success:                  pick(fallback.Success, legacy.Correct, legacy.ModalConfirmBG, gradient(0)),
		Danger:                   pick(fallback.Danger, legacy.Error, legacy.Cancel, legacy.ModalCancelBG),
		Highlight:                pick(fallback.Highlight, legacy.SidebarItemSelectedBG, legacy.FilePanelItemSelectedBG, legacy.ModalBG, legacy.FooterBG),
		SelectionActiveBG:        pick(fallback.SelectionActiveBG, legacy.FilePanelItemSelectedBG, legacy.SidebarItemSelectedBG, legacy.ModalConfirmBG, legacy.FilePanelBorderActive, legacy.FilePanelTopPath),
		SelectionActiveFG:        pick(fallback.SelectionActiveFG, legacy.FilePanelItemSelectedFG, legacy.SidebarItemSelectedFG, legacy.ModalConfirmFG, legacy.FilePanelFG, legacy.FullScreenFG),
		SelectionInactiveBG:      pick(fallback.SelectionInactiveBG, legacy.SidebarItemSelectedBG, legacy.FilePanelItemSelectedBG, legacy.ModalBG, legacy.FooterBG),
		SelectionInactiveFG:      pick(fallback.SelectionInactiveFG, legacy.SidebarItemSelectedFG, legacy.FilePanelItemSelectedFG, legacy.FilePanelFG, legacy.FullScreenFG),
		MarkedActiveBG:           pick(fallback.MarkedActiveBG, legacy.ModalConfirmBG, legacy.SidebarTitle, legacy.FilePanelBorderActive, gradient(0)),
		MarkedActiveFG:           pick(fallback.MarkedActiveFG, legacy.ModalConfirmFG, legacy.FilePanelFG, legacy.FullScreenFG),
		MarkedInactiveBG:         pick(fallback.MarkedInactiveBG, legacy.ModalCancelBG, legacy.SidebarDivider, legacy.FooterBG, legacy.FilePanelBG),
		MarkedInactiveFG:         pick(fallback.MarkedInactiveFG, legacy.ModalCancelFG, legacy.FooterFG, legacy.FilePanelFG, legacy.FullScreenFG),
		TerminalPromptPath:       pick(fallback.TerminalPromptPath, legacy.FilePanelTopPath, legacy.HelpMenuTitle, legacy.Hint),
		TerminalInputCursor:      pick(fallback.TerminalInputCursor, legacy.Cursor, legacy.FilePanelTopDirectoryIcon, legacy.Correct),
		TerminalSuggestion:       pick(fallback.TerminalSuggestion, legacy.Hint, legacy.FooterBorder, legacy.SidebarDivider),
		TerminalSuggestionCursor: pick(fallback.TerminalSuggestionCursor, legacy.HelpMenuHotkey, legacy.Cursor, legacy.FilePanelTopDirectoryIcon),
		TerminalCursorForeground: pick(fallback.TerminalCursorForeground, legacy.FullScreenBG, legacy.FilePanelBG, legacy.ModalBG),
		TerminalCursorBackground: pick(fallback.TerminalCursorBackground, legacy.Cursor, legacy.FilePanelTopDirectoryIcon, legacy.Correct),
	}

	if xdfileThemeSameColor(theme.SelectionActiveBG, theme.Surface) || xdfileThemeSameColor(theme.SelectionActiveBG, theme.BG) {
		theme.SelectionActiveBG = pick(fallback.SelectionActiveBG, legacy.ModalConfirmBG, legacy.FilePanelBorderActive, legacy.FilePanelTopPath, legacy.Cursor)
	}
	if xdfileThemeSameColor(theme.SelectionInactiveBG, theme.Surface) || xdfileThemeSameColor(theme.SelectionInactiveBG, theme.BG) {
		theme.SelectionInactiveBG = pick(fallback.SelectionInactiveBG, legacy.ModalCancelBG, legacy.SidebarDivider, legacy.FooterBorder, legacy.Hint)
	}
	if xdfileThemeSameColor(theme.Highlight, theme.Surface) || xdfileThemeSameColor(theme.Highlight, theme.BG) {
		theme.Highlight = pick(fallback.Highlight, legacy.ModalBG, legacy.FooterBorderActive, legacy.Hint, legacy.FilePanelBorder)
	}

	xdfileGuardThemeContrast(&theme, fallback)
	return theme
}

func xdfileNormalizeThemeColor(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return "", false
	}
	for _, r := range value[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return "", false
		}
	}
	return strings.ToUpper(value), true
}

func xdfileThemeSameColor(a string, b string) bool {
	a, okA := xdfileNormalizeThemeColor(a)
	b, okB := xdfileNormalizeThemeColor(b)
	return okA && okB && a == b
}

func xdfileGuardThemeContrast(theme *xdfileTheme, fallback xdfileTheme) {
	if xdfileThemeContrastRatio(theme.Text, theme.Surface) < 4.5 {
		theme.Text = fallback.Text
		theme.Surface = fallback.Surface
	}
	if xdfileThemeContrastRatio(theme.SelectionActiveFG, theme.SelectionActiveBG) < 4.5 {
		theme.SelectionActiveFG = fallback.SelectionActiveFG
		theme.SelectionActiveBG = fallback.SelectionActiveBG
	}
	if xdfileThemeContrastRatio(theme.SelectionInactiveFG, theme.SelectionInactiveBG) < 3.0 {
		theme.SelectionInactiveFG = fallback.SelectionInactiveFG
		theme.SelectionInactiveBG = fallback.SelectionInactiveBG
	}
	if xdfileThemeContrastRatio(theme.MarkedActiveFG, theme.MarkedActiveBG) < 3.0 {
		theme.MarkedActiveFG = fallback.MarkedActiveFG
		theme.MarkedActiveBG = fallback.MarkedActiveBG
	}
	if xdfileThemeContrastRatio(theme.MarkedInactiveFG, theme.MarkedInactiveBG) < 3.0 {
		theme.MarkedInactiveFG = fallback.MarkedInactiveFG
		theme.MarkedInactiveBG = fallback.MarkedInactiveBG
	}
	if xdfileThemeContrastRatio(theme.TerminalCursorForeground, theme.TerminalCursorBackground) < 3.0 {
		theme.TerminalCursorForeground = fallback.TerminalCursorForeground
		theme.TerminalCursorBackground = fallback.TerminalCursorBackground
	}
}

func xdfileThemeContrastRatio(foreground string, background string) float64 {
	fg, okFg := xdfileParseThemeHexColor(foreground)
	bg, okBg := xdfileParseThemeHexColor(background)
	if !okFg || !okBg {
		return 0
	}
	fgLum := xdfileThemeRelativeLuminance(fg)
	bgLum := xdfileThemeRelativeLuminance(bg)
	if fgLum < bgLum {
		fgLum, bgLum = bgLum, fgLum
	}
	return (fgLum + 0.05) / (bgLum + 0.05)
}

func xdfileParseThemeHexColor(value string) (color.RGBA, bool) {
	normalized, ok := xdfileNormalizeThemeColor(value)
	if !ok {
		return color.RGBA{}, false
	}
	r, errR := strconv.ParseUint(normalized[1:3], 16, 8)
	g, errG := strconv.ParseUint(normalized[3:5], 16, 8)
	b, errB := strconv.ParseUint(normalized[5:7], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return color.RGBA{}, false
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xff}, true
}

func xdfileThemeRelativeLuminance(c color.RGBA) float64 {
	channel := func(value uint8) float64 {
		v := float64(value) / 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

func xdfileThemeTokenTable(theme xdfileTheme) []xdfileThemeToken {
	return []xdfileThemeToken{
		{Role: "bg", Value: theme.BG},
		{Role: "surface", Value: theme.Surface},
		{Role: "surface-2", Value: theme.Surface2},
		{Role: "border", Value: theme.Border},
		{Role: "accent", Value: theme.Accent},
		{Role: "accent-2", Value: theme.Accent2},
		{Role: "text", Value: theme.Text},
		{Role: "dim", Value: theme.Dim},
		{Role: "success", Value: theme.Success},
		{Role: "danger", Value: theme.Danger},
		{Role: "highlight", Value: theme.Highlight},
		{Role: "selection-active-bg", Value: theme.SelectionActiveBG},
		{Role: "selection-active-fg", Value: theme.SelectionActiveFG},
		{Role: "selection-inactive-bg", Value: theme.SelectionInactiveBG},
		{Role: "selection-inactive-fg", Value: theme.SelectionInactiveFG},
		{Role: "marked-active-bg", Value: theme.MarkedActiveBG},
		{Role: "marked-active-fg", Value: theme.MarkedActiveFG},
		{Role: "marked-inactive-bg", Value: theme.MarkedInactiveBG},
		{Role: "marked-inactive-fg", Value: theme.MarkedInactiveFG},
		{Role: "terminal-prompt-path", Value: theme.TerminalPromptPath},
		{Role: "terminal-input-cursor", Value: theme.TerminalInputCursor},
		{Role: "terminal-suggestion", Value: theme.TerminalSuggestion},
		{Role: "terminal-suggestion-cursor", Value: theme.TerminalSuggestionCursor},
		{Role: "terminal-cursor-fg", Value: theme.TerminalCursorForeground},
		{Role: "terminal-cursor-bg", Value: theme.TerminalCursorBackground},
	}
}

func xdfileApplyTheme(theme xdfileTheme) {
	xdfileCurrentTheme = theme

	xdfileColorBG = lipgloss.Color(theme.BG)
	xdfileColorSurface = lipgloss.Color(theme.Surface)
	xdfileColorSurface2 = lipgloss.Color(theme.Surface2)
	xdfileColorBorder = lipgloss.Color(theme.Border)
	xdfileColorAccent = lipgloss.Color(theme.Accent)
	xdfileColorAccent2 = lipgloss.Color(theme.Accent2)
	xdfileColorText = lipgloss.Color(theme.Text)
	xdfileColorDim = lipgloss.Color(theme.Dim)
	xdfileColorSuccess = lipgloss.Color(theme.Success)
	xdfileColorDanger = lipgloss.Color(theme.Danger)
	xdfileColorHighlight = lipgloss.Color(theme.Highlight)
	xdfileColorSelectionActiveBG = lipgloss.Color(theme.SelectionActiveBG)
	xdfileColorSelectionActiveFG = lipgloss.Color(theme.SelectionActiveFG)
	xdfileColorSelectionInactiveBG = lipgloss.Color(theme.SelectionInactiveBG)
	xdfileColorSelectionInactiveFG = lipgloss.Color(theme.SelectionInactiveFG)
	xdfileColorMarkedActiveBG = lipgloss.Color(theme.MarkedActiveBG)
	xdfileColorMarkedActiveFG = lipgloss.Color(theme.MarkedActiveFG)
	xdfileColorMarkedInactiveBG = lipgloss.Color(theme.MarkedInactiveBG)
	xdfileColorMarkedInactiveFG = lipgloss.Color(theme.MarkedInactiveFG)

	xdfileHeaderLineStyle = lipgloss.NewStyle().Foreground(xdfileColorText)
	xdfileFooterLineStyle = lipgloss.NewStyle().Foreground(xdfileColorText)
	xdfileStatusOKStyle = lipgloss.NewStyle().Foreground(xdfileColorSuccess)
	xdfileStatusErrStyle = lipgloss.NewStyle().Foreground(xdfileColorDanger)
	xdfileTagStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent)
	xdfileTitleStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent2)
	xdfileDimStyle = lipgloss.NewStyle().Foreground(xdfileColorDim)
	xdfilePathStyle = lipgloss.NewStyle().Foreground(xdfileColorText)
	xdfileTerminalPromptLabelStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent).Bold(true)
	xdfileTerminalPromptPathStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TerminalPromptPath))
	xdfileTerminalPromptCommandStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent2)
	xdfileTerminalInputPromptStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent2).Bold(true)
	xdfileTerminalInputTextStyle = lipgloss.NewStyle().Foreground(xdfileColorText)
	xdfileTerminalInputCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TerminalInputCursor)).Bold(true)
	xdfileTerminalSuggestionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TerminalSuggestion)).Italic(true)
	xdfileTerminalSuggestionCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TerminalSuggestionCursor)).Bold(true).Underline(true)
	xdfileTerminalSuggestionUVStyle = uv.Style{
		Fg:    xdfileHexRGBA(theme.TerminalSuggestion),
		Attrs: uv.AttrItalic,
	}
	xdfileTerminalSuggestionCursorUVStyle = uv.Style{
		Fg:        xdfileHexRGBA(theme.TerminalSuggestionCursor),
		Underline: uv.UnderlineSingle,
		Attrs:     uv.AttrBold,
	}
	xdfileTerminalCursorUVStyle = uv.Style{
		Fg:        xdfileHexRGBA(theme.TerminalCursorForeground),
		Bg:        xdfileHexRGBA(theme.TerminalCursorBackground),
		Underline: uv.UnderlineSingle,
		Attrs:     uv.AttrBold,
	}
	xdfileDirStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent)
	xdfileFileStyle = lipgloss.NewStyle().Foreground(xdfileColorText)
	xdfileMetaStyle = lipgloss.NewStyle().Foreground(xdfileColorDim)
	xdfileButtonKeyStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent2)
	xdfileMenuButton = lipgloss.NewStyle().Foreground(xdfileColorAccent2).Padding(0, 1)
	xdfileMenuButtonHot = lipgloss.NewStyle().
		Foreground(xdfileColorSelectionActiveFG).
		Background(xdfileColorSelectionActiveBG).
		Padding(0, 1).
		Bold(true)
	xdfileMenuItemStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText)
	xdfileMenuItemHot = lipgloss.NewStyle().
		Foreground(xdfileColorSelectionActiveFG).
		Background(xdfileColorSelectionActiveBG).
		Bold(true)
	xdfileMenuItemKeyStyle = lipgloss.NewStyle().
		Foreground(xdfileColorAccent)
	xdfileMenuItemDisabledStyle = lipgloss.NewStyle().
		Foreground(xdfileColorDim)
	xdfileModalTitleStyle = lipgloss.NewStyle().
		Foreground(xdfileColorAccent).
		Background(xdfileColorSurface).
		Bold(true)

	xdfileSelectedLineActiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorSelectionActiveFG).
		Background(xdfileColorSelectionActiveBG).
		Bold(true)
	xdfileSelectedLineInactiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorSelectionInactiveFG).
		Background(xdfileColorSelectionInactiveBG)
	xdfileInactiveCursorStyle = lipgloss.NewStyle().
		Foreground(xdfileColorSelectionInactiveFG).
		Bold(true)
	xdfileHoveredEntryLineCachedStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Background(xdfileColorHighlight)
	xdfileHoveredEntryMetaCachedStyle = lipgloss.NewStyle().
		Foreground(xdfileColorDim).
		Background(xdfileColorHighlight)
	xdfileHoveredMenuButtonCachedStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Background(xdfileColorHighlight).
		Padding(0, 1)
	xdfileHoveredMenuItemCachedStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Background(xdfileColorHighlight)
	xdfileHoveredFooterKeyCachedStyle = lipgloss.NewStyle().
		Foreground(xdfileColorAccent2).
		Background(xdfileColorHighlight)
	xdfileHoveredFooterLabelCachedStyle = lipgloss.NewStyle().
		Foreground(xdfileColorDim).
		Background(xdfileColorHighlight)
	xdfileMarkedLineActiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorMarkedActiveFG).
		Background(xdfileColorMarkedActiveBG).
		Bold(true)
	xdfileMarkedLineInactiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorMarkedInactiveFG).
		Background(xdfileColorMarkedInactiveBG)
	heavyBorder := xdfileRoundedHeavyBorder()
	xdfilePanelBorderActiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Border(heavyBorder).
		BorderForeground(xdfileColorAccent).
		Padding(0, 0)
	xdfilePanelBorderInactiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Border(heavyBorder).
		BorderForeground(xdfileColorBorder).
		Padding(0, 0)
	xdfileTerminalBorderActiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Border(heavyBorder).
		BorderForeground(xdfileColorAccent2).
		Padding(0, 0)
	xdfileTerminalBorderInactiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Border(heavyBorder).
		BorderForeground(xdfileColorBorder).
		Padding(0, 0)
	xdfileModalBorderStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Background(xdfileColorSurface).
		Border(lipgloss.ThickBorder()).
		BorderForeground(xdfileColorAccent2)
	xdfileMenuBorderStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Border(lipgloss.NormalBorder()).
		BorderForeground(xdfileColorBorder)
	xdfileFileColorStyles = make(map[lipgloss.Color]lipgloss.Style, 64)
	xdfileBlankStrings = map[int]string{0: ""}
	xdfileCompactPathCache = make(map[xdfileCompactPathKey]string, xdfileCompactPathCacheMax)
	xdfileEntryKindRenders = make(map[xdfileEntryKindSpec]string, 32)
	xdfileEntryKindOnRenders = make(map[xdfileEntryKindRenderKey]string, 8)
}

func xdfileHexRGBA(value string) color.RGBA {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return color.RGBA{A: 0xff}
	}

	r, errR := strconv.ParseUint(value[0:2], 16, 8)
	g, errG := strconv.ParseUint(value[2:4], 16, 8)
	b, errB := strconv.ParseUint(value[4:6], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return color.RGBA{A: 0xff}
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xff}
}
