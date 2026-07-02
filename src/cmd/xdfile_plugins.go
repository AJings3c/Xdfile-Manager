package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	variable "github.com/s0x401/xdfile-manager/src/config"
)

const (
	xdfilePluginManifestName       = "xdfile-plugin.json"
	xdfilePluginActionPrefix       = "plugin_action:"
	xdfilePluginDefaultTimeout     = 2 * time.Second
	xdfilePluginMaxTimeout         = 10 * time.Second
	xdfilePluginMaxOutputBytes     = 1024 * 1024
	xdfilePluginActionShowText     = "show_text"
	xdfilePluginActionCommandDraft = "terminal_command"
	xdfilePluginActionCopyText     = "copy_text"
)

type xdfilePluginManifest struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Command      string   `json:"command"`
	Capabilities []string `json:"capabilities"`
	TimeoutMS    int      `json:"timeout_ms"`
	Enabled      bool     `json:"enabled"`
	Dir          string   `json:"-"`
}

type xdfilePluginContext struct {
	Cwd           string   `json:"cwd"`
	SelectedPaths []string `json:"selected_paths"`
}

type xdfilePluginResponse struct {
	Action  string `json:"action"`
	Title   string `json:"title,omitempty"`
	Text    string `json:"text,omitempty"`
	Command string `json:"command,omitempty"`
}

func xdfilePluginsDirPath() string {
	return filepath.Join(variable.XdfileMainDir, "plugins")
}

func xdfilePluginAction(index int) xdfileAction {
	return xdfileAction(fmt.Sprintf("%s%d", xdfilePluginActionPrefix, index))
}

func xdfileParsePluginAction(action xdfileAction) (int, bool) {
	raw := string(action)
	if !strings.HasPrefix(raw, xdfilePluginActionPrefix) {
		return 0, false
	}
	var index int
	if _, err := fmt.Sscanf(raw, xdfilePluginActionPrefix+"%d", &index); err != nil {
		return 0, false
	}
	return index, true
}

func xdfileLoadPlugins(root string) ([]xdfilePluginManifest, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	plugins := make([]xdfilePluginManifest, 0, len(entries))
	if plugin, ok, err := xdfileLoadPluginManifest(filepath.Join(root, xdfilePluginManifestName)); err != nil {
		return nil, err
	} else if ok {
		plugins = append(plugins, plugin)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(root, entry.Name(), xdfilePluginManifestName)
		plugin, ok, err := xdfileLoadPluginManifest(manifestPath)
		if err != nil {
			return nil, err
		}
		if ok {
			plugins = append(plugins, plugin)
		}
	}
	return plugins, nil
}

func xdfileLoadPluginManifest(path string) (xdfilePluginManifest, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return xdfilePluginManifest{}, false, nil
		}
		return xdfilePluginManifest{}, false, err
	}
	var plugin xdfilePluginManifest
	if err := json.Unmarshal(data, &plugin); err != nil {
		return xdfilePluginManifest{}, false, fmt.Errorf("parse plugin manifest %s: %w", path, err)
	}
	plugin.Dir = filepath.Dir(path)
	if !plugin.Enabled {
		return xdfilePluginManifest{}, false, nil
	}
	if err := xdfileValidatePluginManifest(plugin); err != nil {
		return xdfilePluginManifest{}, false, fmt.Errorf("invalid plugin manifest %s: %w", path, err)
	}
	return plugin, true, nil
}

func xdfileValidatePluginManifest(plugin xdfilePluginManifest) error {
	if strings.TrimSpace(plugin.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(plugin.Command) == "" {
		return fmt.Errorf("command is required")
	}
	if plugin.TimeoutMS < 0 {
		return fmt.Errorf("timeout_ms cannot be negative")
	}
	if plugin.TimeoutMS > int(xdfilePluginMaxTimeout/time.Millisecond) {
		return fmt.Errorf("timeout_ms cannot exceed %d", int(xdfilePluginMaxTimeout/time.Millisecond))
	}
	return nil
}

func (m *xdfileModel) pluginMenuDefinition() xdfileMenu {
	items := make([]xdfileButton, 0, max(1, len(m.plugins)))
	for i, plugin := range m.plugins {
		items = append(items, xdfileButton{
			Action: xdfilePluginAction(i),
			Label:  plugin.Name,
		})
	}
	if len(items) == 0 {
		items = append(items, xdfileButton{Label: "No enabled plugins", Disabled: true})
	}
	return xdfileMenu{
		Action: xdfileActionPluginsMenu,
		Label:  "Plugins",
		Items:  items,
	}
}

func (m *xdfileModel) runPluginAction(index int) tea.Cmd {
	if index < 0 || index >= len(m.plugins) {
		m.setStatus("Plugin not found")
		return nil
	}
	plugin := m.plugins[index]
	contextPayload := xdfilePluginContext{
		Cwd:           m.panels[m.activePanel].Cwd,
		SelectedPaths: m.pluginSelectedPaths(),
	}
	m.setStatus("Running plugin %s", plugin.Name)
	return func() tea.Msg {
		response, err := xdfileRunPlugin(plugin, contextPayload)
		return xdfilePluginActionDoneMsg{
			Plugin:   plugin,
			Response: response,
			Err:      err,
		}
	}
}

func (m *xdfileModel) pluginSelectedPaths() []string {
	entries := m.activeFileSelectionEntries()
	if len(entries) == 0 {
		return nil
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	return paths
}

func xdfileRunPlugin(plugin xdfilePluginManifest, payload xdfilePluginContext) (xdfilePluginResponse, error) {
	timeout := xdfilePluginDefaultTimeout
	if plugin.TimeoutMS > 0 {
		timeout = time.Duration(plugin.TimeoutMS) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	parsed, err := xdfileParseShellCommand(plugin.Command)
	if err != nil {
		return xdfilePluginResponse{}, err
	}
	if strings.TrimSpace(parsed.Name) == "" {
		return xdfilePluginResponse{}, fmt.Errorf("plugin command is empty")
	}

	cmd := exec.CommandContext(ctx, parsed.Name, parsed.Args...)
	cmd.Dir = plugin.Dir
	stdin, err := json.Marshal(payload)
	if err != nil {
		return xdfilePluginResponse{}, err
	}
	cmd.Stdin = bytes.NewReader(stdin)

	var stdout xdfileLimitedPluginBuffer
	var stderr xdfileLimitedPluginBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return xdfilePluginResponse{}, fmt.Errorf("plugin timed out after %s", timeout)
		}
		if text := strings.TrimSpace(stderr.String()); text != "" {
			return xdfilePluginResponse{}, fmt.Errorf("plugin failed: %s", text)
		}
		return xdfilePluginResponse{}, fmt.Errorf("plugin failed: %w", err)
	}

	var response xdfilePluginResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return xdfilePluginResponse{}, fmt.Errorf("parse plugin response: %w", err)
	}
	if err := xdfileValidatePluginResponse(response); err != nil {
		return xdfilePluginResponse{}, err
	}
	return response, nil
}

type xdfileLimitedPluginBuffer struct {
	bytes.Buffer
}

func (b *xdfileLimitedPluginBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > xdfilePluginMaxOutputBytes {
		return 0, fmt.Errorf("plugin output exceeds %d bytes", xdfilePluginMaxOutputBytes)
	}
	return b.Buffer.Write(p)
}

func xdfileValidatePluginResponse(response xdfilePluginResponse) error {
	switch strings.TrimSpace(response.Action) {
	case xdfilePluginActionShowText:
		if strings.TrimSpace(response.Text) == "" {
			return fmt.Errorf("plugin show_text response requires text")
		}
	case xdfilePluginActionCommandDraft:
		if strings.TrimSpace(response.Command) == "" {
			return fmt.Errorf("plugin terminal_command response requires command")
		}
	case xdfilePluginActionCopyText:
		if strings.TrimSpace(response.Text) == "" {
			return fmt.Errorf("plugin copy_text response requires text")
		}
	default:
		return fmt.Errorf("plugin action is not allowed: %s", response.Action)
	}
	return nil
}

func (m *xdfileModel) applyPluginActionDone(msg xdfilePluginActionDoneMsg) tea.Cmd {
	if msg.Err != nil {
		m.setStatusErr(msg.Err)
		return nil
	}
	if msg.Response.Action == xdfilePluginActionCommandDraft || msg.Response.Action == xdfilePluginActionCopyText {
		return m.openPluginActionConfirm(msg)
	}
	return m.applyConfirmedPluginResponse(msg)
}

func (m *xdfileModel) openPluginActionConfirm(msg xdfilePluginActionDoneMsg) tea.Cmd {
	m.pendingPluginAction = &msg
	description := ""
	switch msg.Response.Action {
	case xdfilePluginActionCommandDraft:
		description = "Plugin wants to insert a terminal command draft:\n" + strings.TrimSpace(msg.Response.Command)
	case xdfilePluginActionCopyText:
		description = "Plugin wants to copy text to the system clipboard."
	default:
		description = "Plugin wants to perform an action."
	}
	m.modal = xdfileModal{
		Kind:        xdfileModalConfirm,
		Title:       "Confirm Plugin Action",
		Description: description,
		Action:      xdfileActionPluginConfirm,
		Input:       m.modalInputModel(),
	}
	m.setStatus("Review plugin action; Enter confirms, Esc cancels")
	return nil
}

func (m *xdfileModel) confirmPluginAction() tea.Cmd {
	if m.pendingPluginAction == nil {
		m.closeModal()
		m.setStatus("No pending plugin action")
		return nil
	}
	msg := *m.pendingPluginAction
	m.closeModal()
	return m.applyConfirmedPluginResponse(msg)
}

func (m *xdfileModel) applyConfirmedPluginResponse(msg xdfilePluginActionDoneMsg) tea.Cmd {
	title := strings.TrimSpace(msg.Response.Title)
	if title == "" {
		title = msg.Plugin.Name
	}
	switch msg.Response.Action {
	case xdfilePluginActionShowText:
		m.openTextModal(title, msg.Response.Text)
	case xdfilePluginActionCommandDraft:
		m.terminal.Input.SetValue(strings.TrimSpace(msg.Response.Command))
		m.terminal.Input.CursorEnd()
		m.refreshManagedTerminalSuggestions()
		m.focusManagedTerminalInput()
		m.setStatus("Plugin command draft inserted; press Enter yourself to run it")
	case xdfilePluginActionCopyText:
		text := msg.Response.Text
		m.setStatus("Copying plugin text")
		return func() tea.Msg {
			return xdfileClipboardTextWriteResultMsg{
				Err: xdfileWriteClipboardTextFunc(text),
			}
		}
	default:
		m.setStatusErr(fmt.Errorf("plugin action is not allowed: %s", msg.Response.Action))
	}
	return nil
}
