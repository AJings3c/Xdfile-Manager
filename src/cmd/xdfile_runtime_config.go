package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/s0x401/xdfile-manager/src/internal/common"
)

type xdfileRuntimeConfig struct {
	Theme                   *string `toml:"theme"`
	DefaultDirectory        *string `toml:"default_directory"`
	DefaultOpenFilePreview  *bool   `toml:"default_open_file_preview"`
	ShowImagePreview        *bool   `toml:"show_image_preview"`
	EnableFilePreviewBorder *bool   `toml:"enable_file_preview_border"`
	Nerdfont                *bool   `toml:"nerdfont"`
	ZoxideSupport           *bool   `toml:"zoxide_support"`
	AIEnabled               *bool   `toml:"ai_enabled"`
	AIProvider              *string `toml:"ai_provider"`
	AIModel                 *string `toml:"ai_model"`
	AIAPIKeyEnv             *string `toml:"ai_api_key_env"`
}

func xdfileLoadRuntimeConfig(path string) (xdfileRuntimeConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return xdfileRuntimeConfig{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return xdfileRuntimeConfig{}, nil
		}
		return xdfileRuntimeConfig{}, fmt.Errorf("read runtime config: %w", err)
	}

	var config xdfileRuntimeConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return xdfileRuntimeConfig{}, fmt.Errorf("parse runtime config: %w", err)
	}
	return config, nil
}

func xdfileApplyRuntimeConfig(config xdfileRuntimeConfig) {
	common.Config.ShowImagePreview = config.showImagePreview()
	common.Config.DefaultOpenFilePreview = config.defaultOpenFilePreview()
	common.Config.EnableFilePreviewBorder = config.enableFilePreviewBorder()
	common.Config.Nerdfont = config.nerdfont()
	common.Config.ZoxideSupport = config.zoxideSupport()
}

func xdfileApplyRuntimeConfigLayoutDefaults(
	prefs xdfileLayoutPrefs,
	layoutFileExists bool,
	config xdfileRuntimeConfig,
) xdfileLayoutPrefs {
	if !layoutFileExists && config.Theme != nil && strings.TrimSpace(*config.Theme) != "" {
		prefs.ThemeName = strings.TrimSpace(*config.Theme)
	}
	return prefs.normalized()
}

func (c xdfileRuntimeConfig) defaultDirectory() string {
	if c.DefaultDirectory == nil {
		return ""
	}
	return strings.TrimSpace(*c.DefaultDirectory)
}

func (c xdfileRuntimeConfig) defaultOpenFilePreview() bool {
	if c.DefaultOpenFilePreview == nil {
		return false
	}
	return *c.DefaultOpenFilePreview
}

func (c xdfileRuntimeConfig) showImagePreview() bool {
	if c.ShowImagePreview == nil {
		return true
	}
	return *c.ShowImagePreview
}

func (c xdfileRuntimeConfig) enableFilePreviewBorder() bool {
	if c.EnableFilePreviewBorder == nil {
		return false
	}
	return *c.EnableFilePreviewBorder
}

func (c xdfileRuntimeConfig) nerdfont() bool {
	if c.Nerdfont == nil {
		return false
	}
	return *c.Nerdfont
}

func (c xdfileRuntimeConfig) zoxideSupport() bool {
	if c.ZoxideSupport == nil {
		return false
	}
	return *c.ZoxideSupport
}

func (c xdfileRuntimeConfig) aiConfig() xdfileAIConfig {
	config := xdfileAIConfig{
		Enabled: c.aiEnabled(),
	}
	if c.AIProvider != nil {
		config.Provider = strings.TrimSpace(*c.AIProvider)
	}
	if c.AIModel != nil {
		config.Model = strings.TrimSpace(*c.AIModel)
	}
	if c.AIAPIKeyEnv != nil {
		config.APIKeyEnv = strings.TrimSpace(*c.AIAPIKeyEnv)
	}
	return config
}

func (c xdfileRuntimeConfig) aiEnabled() bool {
	if c.AIEnabled == nil {
		return false
	}
	return *c.AIEnabled
}

func xdfilePathExists(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
