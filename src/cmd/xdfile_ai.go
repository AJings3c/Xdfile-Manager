package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const xdfileAISelectedPathLimit = 8

type xdfileAIConfig struct {
	Enabled   bool
	Provider  string
	Model     string
	APIKeyEnv string
}

type xdfileAICommandRequest struct {
	Prompt        string
	Cwd           string
	SelectedPaths []string
}

type xdfileAIProvider interface {
	GenerateShellCommand(context.Context, xdfileAICommandRequest) (string, error)
}

var xdfileAIProviderFactory = xdfileNewAIProvider

var (
	xdfileAITokenLikePattern = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9_.:/+=-]{23,}`)
	xdfileAIEnvNamePattern   = regexp.MustCompile(`\b[A-Z][A-Z0-9_]*(TOKEN|SECRET|PASSWORD|API_KEY|ACCESS_KEY|PRIVATE_KEY)[A-Z0-9_]*\b`)
)

func xdfileNewAIProvider(config xdfileAIConfig) (xdfileAIProvider, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("AI command generation is disabled")
	}

	provider := strings.ToLower(strings.TrimSpace(config.Provider))
	if provider == "" {
		return nil, fmt.Errorf("AI provider is not configured")
	}
	if config.APIKeyEnv != "" && os.Getenv(config.APIKeyEnv) == "" {
		return nil, fmt.Errorf("AI API key environment variable is not set: %s", config.APIKeyEnv)
	}

	switch provider {
	case "local", "template":
		return xdfileLocalAIProvider{}, nil
	default:
		return nil, fmt.Errorf("AI provider %q is not available in this build", config.Provider)
	}
}

type xdfileLocalAIProvider struct{}

func (xdfileLocalAIProvider) GenerateShellCommand(_ context.Context, request xdfileAICommandRequest) (string, error) {
	prompt := strings.ToLower(strings.TrimSpace(request.Prompt))
	switch {
	case prompt == "":
		return "", fmt.Errorf("AI request cannot be empty")
	case strings.Contains(prompt, "git status"):
		return "git status", nil
	case strings.Contains(prompt, "list") || strings.Contains(prompt, "列出"):
		return "ls -la", nil
	case strings.Contains(prompt, "current directory") || strings.Contains(prompt, "pwd") || strings.Contains(prompt, "当前目录"):
		return "pwd", nil
	case strings.Contains(prompt, "size") || strings.Contains(prompt, "disk") || strings.Contains(prompt, "大小"):
		return "du -sh .", nil
	default:
		return "", fmt.Errorf("local AI provider cannot generate a command for this request")
	}
}

func (m *xdfileModel) openAICommandModal() tea.Cmd {
	if !m.aiConfig.Enabled {
		m.setStatus("AI command generation is disabled")
		return nil
	}
	if m.terminal.Busy || m.exclusiveTerminalActive() {
		m.setStatus("AI command generation is unavailable while the terminal is busy")
		return nil
	}
	if m.terminalUsesPTY() {
		m.setStatus("AI command generation is unavailable while PTY shell owns input")
		return nil
	}

	m.openInputModal(
		xdfileActionModalAICommand,
		"AI Command",
		"Describe a shell command. The result is inserted as a draft only.",
		m.activePanel,
		"",
		"",
	)
	m.modal.Input.Placeholder = "Describe the command to generate"
	m.setStatus("AI command drafts are inserted only; Enter is never sent automatically")
	return nil
}

func (m *xdfileModel) startAICommandGeneration(prompt string) tea.Cmd {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		m.setStatus("AI request cannot be empty")
		return nil
	}

	config := m.aiConfig
	request := xdfileAICommandRequest{
		Prompt:        xdfileAIRedactText(prompt),
		Cwd:           xdfileAIRedactText(m.terminal.Cwd),
		SelectedPaths: m.aiSelectedPathSummary(),
	}

	return func() tea.Msg {
		provider, err := xdfileAIProviderFactory(config)
		if err != nil {
			return xdfileAICommandDoneMsg{Prompt: prompt, Err: err}
		}
		command, err := provider.GenerateShellCommand(context.Background(), request)
		return xdfileAICommandDoneMsg{
			Prompt:  prompt,
			Command: strings.TrimSpace(command),
			Err:     err,
		}
	}
}

func (m *xdfileModel) applyAICommandDone(msg xdfileAICommandDoneMsg) tea.Cmd {
	if msg.Err != nil {
		m.setStatusErr(msg.Err)
		return nil
	}
	command := strings.TrimSpace(msg.Command)
	if command == "" {
		m.setStatus("AI provider returned an empty command")
		return nil
	}
	if reasons := xdfileAIDangerousCommandReasons(command); len(reasons) > 0 {
		m.pendingAICommand = command
		m.modal = xdfileModal{
			Kind:        xdfileModalConfirm,
			Title:       "Review AI Command",
			Description: fmt.Sprintf("Potentially dangerous: %s\n%s", strings.Join(reasons, ", "), command),
			Action:      xdfileActionAICommandDangerConfirm,
			Input:       m.modalInputModel(),
		}
		m.setStatus("Review dangerous AI command draft; Enter inserts, Esc cancels")
		return nil
	}
	m.insertAICommandDraft(command)
	return nil
}

func (m *xdfileModel) confirmAICommandDraft() tea.Cmd {
	command := strings.TrimSpace(m.pendingAICommand)
	m.closeModal()
	m.insertAICommandDraft(command)
	return nil
}

func (m *xdfileModel) insertAICommandDraft(command string) {
	command = strings.TrimSpace(command)
	if command == "" {
		m.setStatus("AI provider returned an empty command")
		return
	}
	m.pendingAICommand = ""
	m.terminal.Input.SetValue(command)
	m.terminal.Input.CursorEnd()
	m.terminal.HistoryIndex = -1
	m.refreshManagedTerminalSuggestions()
	m.focusManagedTerminalInput()
	m.setStatus("AI command draft inserted; press Enter yourself to run it")
}

func (m *xdfileModel) aiSelectedPathSummary() []string {
	entries := m.activeFileSelectionEntries()
	if len(entries) == 0 {
		return nil
	}
	limit := min(len(entries), xdfileAISelectedPathLimit)
	paths := make([]string, 0, limit+1)
	for i := 0; i < limit; i++ {
		paths = append(paths, xdfileAIRedactText(entries[i].Path))
	}
	if len(entries) > limit {
		paths = append(paths, fmt.Sprintf("... %d more selected path(s)", len(entries)-limit))
	}
	return paths
}

func xdfileAIRedactText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		value = strings.ReplaceAll(value, home, "~")
		value = strings.ReplaceAll(value, filepath.ToSlash(home), "~")
	}
	value = xdfileAIEnvNamePattern.ReplaceAllString(value, "<env>")
	value = xdfileAITokenLikePattern.ReplaceAllStringFunc(value, func(token string) string {
		if strings.HasPrefix(token, "file://") || strings.HasPrefix(token, "xdssh://") {
			return token
		}
		return "<secret>"
	})
	return value
}

func xdfileAIDangerousCommandReasons(command string) []string {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	lower := strings.ToLower(command)
	fields := strings.Fields(lower)
	reasons := make([]string, 0, 4)
	if len(fields) > 0 {
		switch fields[0] {
		case "rm", "rmdir", "del", "erase":
			reasons = append(reasons, "delete")
		case "mv", "move":
			reasons = append(reasons, "overwrite or move")
		case "chmod", "chown", "icacls", "takeown":
			reasons = append(reasons, "permission change")
		case "ssh", "scp", "sftp", "rsync":
			reasons = append(reasons, "remote execution or upload")
		case "apt", "apt-get", "brew", "dnf", "yum", "pacman", "npm", "pip", "pipx", "cargo", "go":
			if xdfileAICommandContainsAny(fields[1:], "install", "remove", "uninstall", "upgrade", "update", "get") {
				reasons = append(reasons, "package management")
			}
		}
	}
	if strings.Contains(lower, " rm ") || strings.Contains(lower, ";rm ") || strings.Contains(lower, "&& rm ") {
		reasons = append(reasons, "delete")
	}
	if strings.Contains(command, ">") {
		reasons = append(reasons, "overwrite")
	}
	if strings.Contains(lower, "curl ") && (strings.Contains(lower, " -t ") || strings.Contains(lower, " --upload-file ")) {
		reasons = append(reasons, "network upload")
	}
	return xdfileUniqueAIReasons(reasons)
}

func xdfileAICommandContainsAny(values []string, needles ...string) bool {
	for _, value := range values {
		for _, needle := range needles {
			if value == needle || strings.HasPrefix(value, needle+"=") {
				return true
			}
		}
	}
	return false
}

func xdfileUniqueAIReasons(reasons []string) []string {
	if len(reasons) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(reasons))
	unique := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if _, ok := seen[reason]; ok {
			continue
		}
		seen[reason] = struct{}{}
		unique = append(unique, reason)
	}
	return unique
}
