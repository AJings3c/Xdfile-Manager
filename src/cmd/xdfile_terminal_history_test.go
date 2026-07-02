package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	variable "github.com/s0x401/xdfile-manager/src/config"
)

func TestXdfileRecordTerminalHistoryPersistsFullLog(t *testing.T) {
	originalMainDir := variable.XdfileMainDir
	variable.XdfileMainDir = filepath.Join(t.TempDir(), "xdfile-data")
	t.Cleanup(func() {
		variable.XdfileMainDir = originalMainDir
	})

	m := &xdfileModel{}
	if err := m.recordTerminalHistory("git status", "/workspace/one", false); err != nil {
		t.Fatalf("record first history command: %v", err)
	}
	if err := m.recordTerminalHistory("git status", "/workspace/two", true); err != nil {
		t.Fatalf("record repeated history command: %v", err)
	}

	items, logEntries, deleted, err := xdfileLoadTerminalHistoryState(xdfileTerminalHistoryPath())
	if err != nil {
		t.Fatalf("load terminal history: %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("unexpected deleted history keys: %#v", deleted)
	}
	item := items[xdfileTerminalHistoryKey("git status")]
	if item.Command != "git status" || item.Count != 2 || item.Cwd != "/workspace/two" || !item.LastFailed {
		t.Fatalf("deduped history item was not updated: %#v", item)
	}
	if len(logEntries) != 2 {
		t.Fatalf("full command log should keep both executions, got %#v", logEntries)
	}
	if logEntries[0].Command != "git status" || logEntries[0].Cwd != "/workspace/one" || logEntries[0].Failed {
		t.Fatalf("first log entry mismatch: %#v", logEntries[0])
	}
	if logEntries[1].Command != "git status" || logEntries[1].Cwd != "/workspace/two" || !logEntries[1].Failed {
		t.Fatalf("second log entry mismatch: %#v", logEntries[1])
	}
}

func TestXdfileTerminalHistoryLoadBackfillsLogFromLegacyItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	base := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	items := map[string]xdfileTerminalHistoryItem{
		"git status": {
			Command:  "git status",
			Cwd:      "/repo",
			Count:    3,
			LastUsed: base.Add(-2 * time.Minute),
		},
		"go test ./...": {
			Command:  "go test ./...",
			Cwd:      "/repo",
			Count:    1,
			LastUsed: base.Add(-1 * time.Minute),
		},
	}

	if err := xdfileSaveTerminalHistoryState(path, items, nil, nil); err != nil {
		t.Fatalf("save legacy-shaped terminal history: %v", err)
	}

	_, logEntries, _, err := xdfileLoadTerminalHistoryState(path)
	if err != nil {
		t.Fatalf("load terminal history: %v", err)
	}
	if len(logEntries) != 2 {
		t.Fatalf("legacy history items should backfill log entries, got %#v", logEntries)
	}
	if got := terminalHistoryCommandsForTest(logEntries); got != "git status\ngo test ./..." {
		t.Fatalf("unexpected backfilled log order:\n%s", got)
	}
}

func TestXdfileTerminalHistoryModalShowsFullLogNewestFirst(t *testing.T) {
	base := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	m := newTerminalHistoryTestModel([]xdfileTerminalHistoryLogEntry{
		{Command: "git status", Cwd: "/old", UsedAt: base.Add(1 * time.Minute)},
		{Command: "git status", Cwd: "/new", UsedAt: base.Add(2 * time.Minute), Failed: true},
		{Command: "printf 'a  b'", Cwd: "/space", UsedAt: base.Add(3 * time.Minute)},
		{Command: "rm -rf tmp", Cwd: "/repo", UsedAt: base.Add(4 * time.Minute)},
	})
	m.terminal.HistoryDeleted = map[string]struct{}{
		xdfileTerminalHistoryKey("rm -rf tmp"): {},
	}

	if cmd := m.openTerminalHistoryModal(); cmd != nil {
		t.Fatalf("history modal should open synchronously, got %T", cmd)
	}
	if m.modal.Kind != xdfileModalChoice || m.modal.Action != xdfileActionTerminalHistory {
		t.Fatalf("expected terminal history choice modal, got %#v", m.modal)
	}
	if len(m.modal.ChoiceItems) != 3 {
		t.Fatalf("expected three visible log rows, got %#v", m.modal.ChoiceItems)
	}
	if got := terminalHistoryChoiceLabelsForTest(m.modal.ChoiceItems); got != "printf 'a  b'\ngit status\ngit status" {
		t.Fatalf("history modal should show newest visible log rows with duplicates:\n%s", got)
	}
	if !strings.Contains(m.modal.ChoiceItems[1].Description, "/new") || !strings.Contains(m.modal.ChoiceItems[1].Description, "failed") {
		t.Fatalf("history row should include cwd and failure metadata: %q", m.modal.ChoiceItems[1].Description)
	}
}

func TestXdfileTerminalHistoryModalPasteInsertsWithoutExecuting(t *testing.T) {
	base := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	m := newTerminalHistoryTestModel([]xdfileTerminalHistoryLogEntry{
		{Command: "printf 'a  b'", Cwd: "/repo", UsedAt: base},
	})
	m.terminal.Input.SetValue("draft")

	m.openTerminalHistoryModal()
	beforeLogCount := len(m.terminal.HistoryLog)
	if cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}); cmd != nil {
		t.Fatalf("pasting a history command should be synchronous, got %T", cmd)
	}
	if m.modal.Kind != xdfileModalNone {
		t.Fatalf("history modal should close after paste, got %#v", m.modal)
	}
	if got := m.terminal.Input.Value(); got != "printf 'a  b'" {
		t.Fatalf("history command should be inserted exactly, got %q", got)
	}
	if !m.terminalFocused || m.terminal.Busy {
		t.Fatalf("history paste should focus the idle terminal without executing: focused=%v busy=%v", m.terminalFocused, m.terminal.Busy)
	}
	if len(m.terminal.HistoryLog) != beforeLogCount {
		t.Fatal("pasting a history command should not append a new history execution")
	}

	m.openTerminalHistoryModal()
	m.terminal.Input.SetValue("")
	if cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Fatalf("enter should insert the selected history command synchronously, got %T", cmd)
	}
	if got := m.terminal.Input.Value(); got != "printf 'a  b'" {
		t.Fatalf("enter should insert selected history command exactly, got %q", got)
	}
}

func TestXdfileTerminalHistoryModalCopyWritesSystemClipboard(t *testing.T) {
	originalWriteText := xdfileWriteClipboardTextFunc
	t.Cleanup(func() {
		xdfileWriteClipboardTextFunc = originalWriteText
	})

	var writes []string
	xdfileWriteClipboardTextFunc = func(text string) error {
		writes = append(writes, text)
		return nil
	}

	m := newTerminalHistoryTestModel([]xdfileTerminalHistoryLogEntry{
		{Command: "printf 'a  b'", Cwd: "/repo", UsedAt: time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)},
	})
	m.clipboardPaths = []string{"/keep/file.txt"}
	m.clipboardPath = "/keep/file.txt"
	m.clipboardCut = true

	m.openTerminalHistoryModal()
	cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	msg := assertTerminalHistoryClipboardTextMsgOK(t, cmd)
	_, _ = m.Update(msg)

	if got := strings.Join(writes, "\n"); got != "printf 'a  b'" {
		t.Fatalf("history copy should write exact command, got %q", got)
	}
	if m.modal.Kind != xdfileModalChoice {
		t.Fatalf("copying history should keep the modal open, got %#v", m.modal)
	}
	if got := strings.Join(m.clipboardPaths, "\n"); got != "/keep/file.txt" {
		t.Fatalf("text copy should not replace file clipboard paths: %q", got)
	}
	if !m.clipboardCut || m.clipboardPath != "/keep/file.txt" {
		t.Fatalf("text copy should not change file clipboard mode/path: cut=%v path=%q", m.clipboardCut, m.clipboardPath)
	}
}

func TestXdfileTerminalHistoryModalEmptyAndBusyStates(t *testing.T) {
	empty := newTerminalHistoryTestModel(nil)
	if cmd := empty.openTerminalHistoryModal(); cmd != nil {
		t.Fatalf("empty history should not produce a command, got %T", cmd)
	}
	if empty.modal.Kind != xdfileModalNone {
		t.Fatalf("empty history should not open a modal, got %#v", empty.modal)
	}
	if !strings.Contains(empty.statusText, "No command history") {
		t.Fatalf("empty history status mismatch: %q", empty.statusText)
	}

	busy := newTerminalHistoryTestModel([]xdfileTerminalHistoryLogEntry{
		{Command: "go test ./...", UsedAt: time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)},
	})
	busy.terminal.Busy = true
	if cmd := busy.openTerminalHistoryModal(); cmd != nil {
		t.Fatalf("busy terminal should not produce a command, got %T", cmd)
	}
	if busy.modal.Kind != xdfileModalNone {
		t.Fatalf("busy terminal should not open a modal, got %#v", busy.modal)
	}
	if !strings.Contains(busy.statusText, "command is running") {
		t.Fatalf("busy terminal status mismatch: %q", busy.statusText)
	}
}

func newTerminalHistoryTestModel(logEntries []xdfileTerminalHistoryLogEntry) *xdfileModel {
	m := &xdfileModel{}
	m.terminal.Input = textinput.New()
	m.terminal.HistoryLog = append([]xdfileTerminalHistoryLogEntry(nil), logEntries...)
	m.terminal.HistoryItems = make(map[string]xdfileTerminalHistoryItem)
	for _, entry := range logEntries {
		key := xdfileTerminalHistoryKey(entry.Command)
		if key == "" {
			continue
		}
		item := m.terminal.HistoryItems[key]
		if item.Command == "" {
			item.Command = entry.Command
		}
		item.Cwd = entry.Cwd
		item.Count++
		item.LastUsed = entry.UsedAt
		item.LastFailed = entry.Failed
		m.terminal.HistoryItems[key] = xdfileNormalizeTerminalHistoryItem(item)
	}
	m.syncTerminalHistoryCommands()
	return m
}

func terminalHistoryCommandsForTest(entries []xdfileTerminalHistoryLogEntry) string {
	commands := make([]string, 0, len(entries))
	for _, entry := range entries {
		commands = append(commands, entry.Command)
	}
	return strings.Join(commands, "\n")
}

func terminalHistoryChoiceLabelsForTest(items []xdfileModalChoiceItem) string {
	labels := make([]string, 0, len(items))
	for _, item := range items {
		labels = append(labels, item.Label)
	}
	return strings.Join(labels, "\n")
}

func assertTerminalHistoryClipboardTextMsgOK(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected clipboard text command")
	}
	msg := cmd()
	result, ok := msg.(xdfileClipboardTextWriteResultMsg)
	if !ok {
		t.Fatalf("expected clipboard text result, got %T", msg)
	}
	if result.Err != nil {
		t.Fatalf("clipboard text write failed: %v", result.Err)
	}
	return msg
}
