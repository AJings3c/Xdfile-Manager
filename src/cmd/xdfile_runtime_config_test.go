package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/s0x401/xdfile-manager/src/internal/common"
)

func TestXdfileLoadRuntimeConfigWhitelist(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "xdfile-config.toml")
	mustWriteFile(t, configPath, `
theme = "persona5"
default_directory = "."
default_open_file_preview = true
show_image_preview = false
enable_file_preview_border = true
nerdfont = true
sidebar_width = 20
zoxide_support = true
`)

	config, err := xdfileLoadRuntimeConfig(configPath)
	if err != nil {
		t.Fatalf("load runtime config failed: %v", err)
	}
	if config.Theme == nil || *config.Theme != "persona5" {
		t.Fatalf("theme was not loaded: %#v", config.Theme)
	}
	if config.DefaultDirectory == nil || *config.DefaultDirectory != "." {
		t.Fatalf("default directory was not loaded: %#v", config.DefaultDirectory)
	}
	if !config.defaultOpenFilePreview() {
		t.Fatal("default_open_file_preview was not loaded")
	}
	if config.showImagePreview() {
		t.Fatal("show_image_preview=false was not loaded")
	}
	if !config.enableFilePreviewBorder() {
		t.Fatal("enable_file_preview_border was not loaded")
	}
	if !config.nerdfont() {
		t.Fatal("nerdfont was not loaded")
	}
	if !config.zoxideSupport() {
		t.Fatal("zoxide_support was not loaded")
	}
}

func TestXdfileLoadRuntimeConfigIgnoresReservedFieldValues(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "xdfile-config.toml")
	mustWriteFile(t, configPath, `
theme = "persona4"
sidebar_width = "wide"
metadata = "maybe later"
`)

	config, err := xdfileLoadRuntimeConfig(configPath)
	if err != nil {
		t.Fatalf("reserved fields should not break runtime config load: %v", err)
	}
	if config.Theme == nil || *config.Theme != "persona4" {
		t.Fatalf("stable fields should still load with reserved fields present: %#v", config.Theme)
	}
}

func TestXdfileLoadRuntimeConfigRejectsInvalidStableFieldValue(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "xdfile-config.toml")
	mustWriteFile(t, configPath, `
theme = "persona4"
show_image_preview = "yes"
`)

	if _, err := xdfileLoadRuntimeConfig(configPath); err == nil {
		t.Fatal("invalid stable field value should be reported")
	}
}

func TestXdfileApplyRuntimeConfigDefaultsCommonValues(t *testing.T) {
	originalConfig := common.Config
	t.Cleanup(func() {
		common.Config = originalConfig
	})

	xdfileApplyRuntimeConfig(xdfileRuntimeConfig{})
	if !common.Config.ShowImagePreview {
		t.Fatal("show image preview should default to current enabled behavior")
	}
	if common.Config.DefaultOpenFilePreview {
		t.Fatal("default open file preview should preserve current closed startup behavior")
	}
	if common.Config.EnableFilePreviewBorder {
		t.Fatal("preview border should default to disabled")
	}
	if common.Config.Nerdfont {
		t.Fatal("nerdfont should default to disabled unless configured")
	}
	if common.Config.ZoxideSupport {
		t.Fatal("zoxide support should default to disabled unless configured")
	}

	xdfileApplyRuntimeConfig(xdfileRuntimeConfig{ZoxideSupport: boolPointer(true)})
	if !common.Config.ZoxideSupport {
		t.Fatal("zoxide_support should apply when configured")
	}
}

func TestXdfileResolveStartPathsPriority(t *testing.T) {
	workspace := t.TempDir()
	cliDir := filepath.Join(workspace, "cli")
	layoutDir := filepath.Join(workspace, "layout")
	configDir := filepath.Join(workspace, "config")
	for _, dir := range []string{cliDir, layoutDir, configDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	prefs := xdfileDefaultLayoutPrefs()
	prefs.StartupLeftPath = layoutDir
	prefs.StartupRightPath = layoutDir

	left, right := xdfileResolveStartPaths([]string{cliDir}, prefs, configDir)
	if left != cliDir || right != cliDir {
		t.Fatalf("CLI path should win, got left=%s right=%s", left, right)
	}

	left, right = xdfileResolveStartPaths(nil, prefs, configDir)
	if left != layoutDir || right != layoutDir {
		t.Fatalf("layout startup paths should win over config default, got left=%s right=%s", left, right)
	}

	prefs.StartupLeftPath = ""
	prefs.StartupRightPath = ""
	left, right = xdfileResolveStartPaths(nil, prefs, configDir)
	if left != configDir || right != configDir {
		t.Fatalf("config default directory should be used when layout paths are empty, got left=%s right=%s", left, right)
	}
}

func TestXdfileRuntimeConfigThemeOnlyAppliesWithoutLayoutFile(t *testing.T) {
	config := xdfileRuntimeConfig{Theme: stringPointer("persona5")}
	prefs := xdfileDefaultLayoutPrefs()

	applied := xdfileApplyRuntimeConfigLayoutDefaults(prefs, false, config)
	if applied.ThemeName != xdfileThemePersona5Name {
		t.Fatalf("expected config theme without layout file, got %s", applied.ThemeName)
	}

	applied = xdfileApplyRuntimeConfigLayoutDefaults(prefs, true, config)
	if applied.ThemeName != xdfileThemePersona3Name {
		t.Fatalf("layout theme should win when layout file exists, got %s", applied.ThemeName)
	}
}

func TestXdfileShowImagePreviewDisablesThumbnailRenderer(t *testing.T) {
	originalConfig := common.Config
	originalRenderer := xdfileRenderPreviewThumbnailFunc
	t.Cleanup(func() {
		common.Config = originalConfig
		xdfileRenderPreviewThumbnailFunc = originalRenderer
	})

	common.Config.ShowImagePreview = false
	called := false
	xdfileRenderPreviewThumbnailFunc = func(m *xdfileModel, path string, width int, height int) (string, bool, error) {
		called = true
		return "thumbnail", true, nil
	}

	imagePath := filepath.Join(t.TempDir(), "image.png")
	mustWriteFile(t, imagePath, "not really a png")

	content := (&xdfileModel{}).buildPreviewContent(imagePath, 20, 10, false, false)
	if called {
		t.Fatal("thumbnail renderer should not be called when show_image_preview=false")
	}
	if content.Visual {
		t.Fatalf("preview should fall back to non-visual content when images are disabled: %#v", content)
	}
}

func stringPointer(value string) *string {
	return &value
}

func boolPointer(value bool) *bool {
	return &value
}
