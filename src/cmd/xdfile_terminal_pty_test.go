package cmd

import (
	"net/url"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	vt "github.com/charmbracelet/x/vt"
)

func TestXdfileNormalizeTerminalWorkingDirectory(t *testing.T) {
	localDir := t.TempDir()
	localURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(localDir)}).String()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "local file URL",
			raw:  localURL,
			want: filepath.Clean(localDir),
		},
		{
			name: "localhost file URL",
			raw:  "file://localhost" + filepath.ToSlash(localDir),
			want: filepath.Clean(localDir),
		},
		{
			name: "remote host file URL",
			raw:  "file://example.com/tmp/project",
			want: "",
		},
		{
			name: "non file URL",
			raw:  "https://example.com/tmp/project",
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := xdfileNormalizeTerminalWorkingDirectory(tc.raw); got != tc.want {
				t.Fatalf("normalize cwd = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestXdfileSendWorkingDirectoryFiltersInvalidOSC7(t *testing.T) {
	target := t.TempDir()
	events := make(chan tea.Msg, 4)
	session := &xdfileTerminalPTYSession{
		events: events,
		mode:   xdfileTerminalPTYModeShell,
	}

	session.sendWorkingDirectory((&url.URL{Scheme: "file", Path: filepath.ToSlash(target)}).String())
	if msg := <-events; msg != (xdfileTerminalCwdMsg{Cwd: filepath.Clean(target)}) {
		t.Fatalf("expected terminal cwd msg for local OSC 7, got %#v", msg)
	}

	session.sendWorkingDirectory("file://example.com/tmp/project")
	session.sendWorkingDirectory(filepath.Join(target, "missing"))
	select {
	case msg := <-events:
		t.Fatalf("invalid cwd should not emit an event, got %#v", msg)
	default:
	}
}

func TestXdfileExclusiveTerminalCwdAppliedOnExit(t *testing.T) {
	start := t.TempDir()
	target := t.TempDir()
	m := newPinTestModel(start, filepath.Join(start, "pins.json"))
	m.terminal.Exclusive = xdfileExclusiveTerminal{
		Command: "yazi",
		Cwd:     target,
		Session: &xdfileTerminalPTYSession{emulator: vt.NewSafeEmulator(80, 24)},
	}

	m.finishExclusiveTerminal(nil)

	if m.terminal.Cwd != filepath.Clean(target) {
		t.Fatalf("terminal cwd = %s, want %s", m.terminal.Cwd, target)
	}
	if m.panels[m.activePanel].Cwd != filepath.Clean(target) {
		t.Fatalf("active panel cwd = %s, want %s", m.panels[m.activePanel].Cwd, target)
	}
}

func TestXdfileExclusiveTerminalIgnoresInvalidCwdOnExit(t *testing.T) {
	start := t.TempDir()
	missing := filepath.Join(start, "missing")
	m := newPinTestModel(start, filepath.Join(start, "pins.json"))
	m.terminal.Exclusive = xdfileExclusiveTerminal{
		Command: "yazi",
		Cwd:     missing,
		Session: &xdfileTerminalPTYSession{emulator: vt.NewSafeEmulator(80, 24)},
	}

	m.finishExclusiveTerminal(nil)

	if m.terminal.Cwd != filepath.Clean(start) {
		t.Fatalf("terminal cwd changed for invalid exclusive cwd: %s", m.terminal.Cwd)
	}
	if m.panels[m.activePanel].Cwd != filepath.Clean(start) {
		t.Fatalf("active panel cwd changed for invalid exclusive cwd: %s", m.panels[m.activePanel].Cwd)
	}
}
