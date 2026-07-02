package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestXdfileLoadPluginsRequiresExplicitEnable(t *testing.T) {
	root := t.TempDir()
	disabledDir := filepath.Join(root, "disabled")
	if err := os.MkdirAll(disabledDir, 0o755); err != nil {
		t.Fatalf("mkdir disabled plugin: %v", err)
	}
	writePluginManifest(t, disabledDir, `{
  "name": "Disabled",
  "version": "0.1.0",
  "command": "disabled",
  "enabled": false
}`)

	enabledDir := filepath.Join(root, "enabled")
	if err := os.MkdirAll(enabledDir, 0o755); err != nil {
		t.Fatalf("mkdir enabled plugin: %v", err)
	}
	writePluginManifest(t, enabledDir, fmt.Sprintf(`{
  "name": "Enabled",
  "version": "0.1.0",
  "command": %q,
  "capabilities": ["terminal_command"],
  "timeout_ms": 500,
  "enabled": true
}`, xdfilePluginHelperCommand("command")))

	plugins, err := xdfileLoadPlugins(root)
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}
	if len(plugins) != 1 || plugins[0].Name != "Enabled" {
		t.Fatalf("expected only explicitly enabled plugin, got %#v", plugins)
	}
}

func TestXdfileLoadPluginsRejectsInvalidManifest(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "bad")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir bad plugin: %v", err)
	}
	writePluginManifest(t, pluginDir, `{
  "name": "Bad",
  "command": "bad",
  "timeout_ms": -1,
  "enabled": true
}`)
	if _, err := xdfileLoadPlugins(root); err == nil || !strings.Contains(err.Error(), "timeout_ms") {
		t.Fatalf("expected invalid timeout error, got %v", err)
	}
}

func TestXdfilePluginShowTextAndCommandDraftActions(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "selected.txt")
	mustWriteFile(t, filePath, "selected")
	t.Setenv("XDFILE_PLUGIN_HELPER", "1")

	showPlugin := xdfilePluginManifest{
		Name:      "Show",
		Command:   xdfilePluginHelperCommand("show_text"),
		TimeoutMS: 500,
		Enabled:   true,
		Dir:       workspace,
	}
	m := newPinTestModel(workspace, filepath.Join(workspace, "pins.json"))
	m.plugins = []xdfilePluginManifest{showPlugin}
	m.panels[0].Entries = []xdfileEntry{{Name: "selected.txt", Path: filePath}}
	m.panels[0].Cursor = 0

	done := firstBatchMsg[xdfilePluginActionDoneMsg](t, m.runPluginAction(0))
	if done.Err != nil {
		t.Fatalf("plugin action failed: %v", done.Err)
	}
	_, _ = m.Update(done)
	if m.modal.Kind != xdfileModalText || !strings.Contains(m.modal.Text, "selected=1") {
		t.Fatalf("show_text plugin should open text modal with context, got %#v", m.modal)
	}

	_, _ = m.Update(xdfilePluginActionDoneMsg{
		Plugin:   xdfilePluginManifest{Name: "Command"},
		Response: xdfilePluginResponse{Action: xdfilePluginActionCommandDraft, Command: "git status"},
	})
	if m.modal.Kind != xdfileModalConfirm || m.modal.Action != xdfileActionPluginConfirm {
		t.Fatalf("plugin command draft should require confirmation, got %#v", m.modal)
	}
	if got := m.terminal.Input.Value(); got != "" {
		t.Fatalf("plugin command draft should not be inserted before confirmation, got %q", got)
	}
	if cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		_ = cmd()
	}
	if got := m.terminal.Input.Value(); got != "git status" {
		t.Fatalf("plugin command draft = %q, want git status", got)
	}
}

func TestXdfilePluginCopyTextActionUsesTextClipboard(t *testing.T) {
	workspace := t.TempDir()
	m := newPinTestModel(workspace, filepath.Join(workspace, "pins.json"))

	originalWrite := xdfileWriteClipboardTextFunc
	var copied string
	xdfileWriteClipboardTextFunc = func(text string) error {
		copied = text
		return nil
	}
	t.Cleanup(func() {
		xdfileWriteClipboardTextFunc = originalWrite
	})

	cmd := m.applyPluginActionDone(xdfilePluginActionDoneMsg{
		Plugin:   xdfilePluginManifest{Name: "Copy"},
		Response: xdfilePluginResponse{Action: xdfilePluginActionCopyText, Text: "from plugin"},
	})
	if cmd != nil {
		t.Fatalf("plugin copy should open confirmation synchronously, got %T", cmd)
	}
	if copied != "" {
		t.Fatalf("plugin copy should not write clipboard before confirmation, got %q", copied)
	}
	if m.modal.Kind != xdfileModalConfirm || m.modal.Action != xdfileActionPluginConfirm {
		t.Fatalf("plugin copy should require confirmation, got %#v", m.modal)
	}
	cmd = m.handleModalKey(tea.KeyMsg{Type: tea.KeyEnter})
	msg := firstBatchMsg[xdfileClipboardTextWriteResultMsg](t, cmd)
	if msg.Err != nil {
		t.Fatalf("copy text failed: %v", msg.Err)
	}
	if copied != "from plugin" {
		t.Fatalf("copied text = %q, want from plugin", copied)
	}
}

func TestXdfilePluginInvalidJSONAndTimeoutAreIsolated(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("XDFILE_PLUGIN_HELPER", "1")

	_, err := xdfileRunPlugin(xdfilePluginManifest{
		Name:      "Invalid JSON",
		Command:   xdfilePluginHelperCommand("invalid_json"),
		TimeoutMS: 500,
		Enabled:   true,
		Dir:       workspace,
	}, xdfilePluginContext{})
	if err == nil || !strings.Contains(err.Error(), "parse plugin response") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}

	_, err = xdfileRunPlugin(xdfilePluginManifest{
		Name:      "Slow",
		Command:   xdfilePluginHelperCommand("sleep"),
		TimeoutMS: 20,
		Enabled:   true,
		Dir:       workspace,
	}, xdfilePluginContext{})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}

	_, err = xdfileRunPlugin(xdfilePluginManifest{
		Name:      "Fail",
		Command:   xdfilePluginHelperCommand("fail"),
		TimeoutMS: 500,
		Enabled:   true,
		Dir:       workspace,
	}, xdfilePluginContext{})
	if err == nil || !strings.Contains(err.Error(), "plugin failed") {
		t.Fatalf("expected non-zero exit error, got %v", err)
	}

	_, err = xdfileRunPlugin(xdfilePluginManifest{
		Name:      "Missing",
		Command:   filepath.Join(workspace, "missing-plugin"),
		TimeoutMS: 500,
		Enabled:   true,
		Dir:       workspace,
	}, xdfilePluginContext{})
	if err == nil || !strings.Contains(err.Error(), "plugin failed") {
		t.Fatalf("expected missing command error, got %v", err)
	}
}

func TestXdfilePluginHelperProcess(t *testing.T) {
	if os.Getenv("XDFILE_PLUGIN_HELPER") != "1" {
		return
	}
	args := os.Args
	pluginArg := ""
	for i, arg := range args {
		if arg == "--" && i+1 < len(args) {
			pluginArg = args[i+1]
			break
		}
	}
	switch pluginArg {
	case "show_text":
		var contextPayload xdfilePluginContext
		_ = json.NewDecoder(os.Stdin).Decode(&contextPayload)
		fmt.Printf(`{"action":"show_text","title":"Helper","text":"cwd=%s selected=%d"}`+"\n", contextPayload.Cwd, len(contextPayload.SelectedPaths))
	case "command":
		fmt.Print(`{"action":"terminal_command","command":"git status"}`)
	case "invalid_json":
		fmt.Print("not json")
	case "sleep":
		time.Sleep(2 * time.Second)
		fmt.Print(`{"action":"show_text","text":"late"}`)
	case "fail":
		fmt.Fprint(os.Stderr, "intentional plugin failure")
		os.Exit(3)
	default:
		fmt.Fprintf(os.Stderr, "unknown helper action: %s", pluginArg)
		os.Exit(2)
	}
	os.Exit(0)
}

func xdfilePluginHelperCommand(action string) string {
	return strconv.Quote(os.Args[0]) + " -test.run=TestXdfilePluginHelperProcess -- " + action
}

func writePluginManifest(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, xdfilePluginManifestName), []byte(content), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
}
