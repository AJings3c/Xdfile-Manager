package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func TestXdfileTerminalHistorySearchFiltersAndRecalls(t *testing.T) {
	now := time.Now()
	m := &xdfileModel{}
	m.terminal.Input = textinput.New()
	m.terminal.Input.SetValue("draft command")
	m.terminal.HistoryItems = map[string]xdfileTerminalHistoryItem{
		"go test ./...": {
			Command:  "go test ./...",
			LastUsed: now.Add(-1 * time.Minute),
		},
		"git status": {
			Command:  "git status",
			LastUsed: now.Add(-2 * time.Minute),
		},
		"go vet ./...": {
			Command:  "go vet ./...",
			LastUsed: now.Add(-3 * time.Minute),
		},
	}

	m.startTerminalHistorySearch()
	if !m.terminal.HistorySearchActive {
		t.Fatal("history search should be active")
	}
	if m.terminal.HistorySearchDraft != "draft command" {
		t.Fatalf("draft was not saved: %q", m.terminal.HistorySearchDraft)
	}

	m.updateTerminalHistorySearchQuery("go")
	if got := joinHistorySearchMatches(m); got != "go test ./...\ngo vet ./..." {
		t.Fatalf("unexpected go matches:\n%s", got)
	}

	m.moveTerminalHistorySearch(1)
	if selected := m.selectedTerminalHistorySearchMatch(); selected != "go vet ./..." {
		t.Fatalf("expected second match after cycling, got %q", selected)
	}

	m.acceptTerminalHistorySearch()
	if m.terminal.HistorySearchActive {
		t.Fatal("history search should close after accepting")
	}
	if got := m.terminal.Input.Value(); got != "go vet ./..." {
		t.Fatalf("accepted command should be filled, got %q", got)
	}
	if m.terminal.Busy {
		t.Fatal("accepting history search should not execute the command")
	}
}

func TestXdfileTerminalHistorySearchKeyHandling(t *testing.T) {
	now := time.Now()
	m := &xdfileModel{}
	m.terminal.Input = textinput.New()
	m.terminal.Input.SetValue("keep me")
	m.terminal.HistoryItems = map[string]xdfileTerminalHistoryItem{
		"docker ps": {
			Command:  "docker ps",
			LastUsed: now.Add(-1 * time.Minute),
		},
		"npm run dev": {
			Command:  "npm run dev",
			LastUsed: now.Add(-2 * time.Minute),
		},
	}

	m.startTerminalHistorySearch()
	m.handleTerminalHistorySearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if got := m.terminal.HistorySearchQuery; got != "d" {
		t.Fatalf("query was not updated from typed rune: %q", got)
	}
	if got := joinHistorySearchMatches(m); got != "docker ps\nnpm run dev" {
		t.Fatalf("unexpected substring matches:\n%s", got)
	}

	m.handleTerminalHistorySearchKey(tea.KeyMsg{Type: tea.KeyCtrlR})
	if selected := m.selectedTerminalHistorySearchMatch(); selected != "npm run dev" {
		t.Fatalf("ctrl+r should cycle candidates, got %q", selected)
	}

	m.handleTerminalHistorySearchKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.terminal.HistorySearchActive {
		t.Fatal("esc should close history search")
	}
	if got := m.terminal.Input.Value(); got != "keep me" {
		t.Fatalf("esc should restore draft, got %q", got)
	}
}

func TestXdfileTerminalHistorySearchSkipsDeletedHistory(t *testing.T) {
	now := time.Now()
	m := &xdfileModel{}
	m.terminal.Input = textinput.New()
	m.terminal.HistoryItems = map[string]xdfileTerminalHistoryItem{
		"git status": {
			Command:  "git status",
			LastUsed: now.Add(-1 * time.Minute),
		},
		"git pull": {
			Command:  "git pull",
			LastUsed: now.Add(-2 * time.Minute),
		},
	}
	m.terminal.HistoryDeleted = map[string]struct{}{
		xdfileTerminalHistoryKey("git status"): {},
	}

	m.startTerminalHistorySearch()
	m.updateTerminalHistorySearchQuery("git")

	if got := joinHistorySearchMatches(m); got != "git pull" {
		t.Fatalf("deleted history item should be hidden, got:\n%s", got)
	}
}

func joinHistorySearchMatches(m *xdfileModel) string {
	return strings.Join(m.terminal.HistorySearchMatches, "\n")
}
