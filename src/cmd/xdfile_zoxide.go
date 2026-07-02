package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	xdfileZoxideChoiceLimit      = 6
	xdfileZoxideOpenActionPrefix = "zoxide_open:"
	xdfileZoxideQueryTimeout     = 2 * time.Second
)

var (
	errXdfileZoxideUnavailable      = errors.New("zoxide unavailable")
	xdfileZoxideLookPathFunc        = exec.LookPath
	xdfileZoxideCommandOutputFunc   = xdfileZoxideCommandOutput
	xdfileZoxideQueryCandidatesFunc = xdfileQueryZoxideCandidates
)

func (m *xdfileModel) openZoxideQueryModal() tea.Cmd {
	if !m.zoxideEnabled {
		m.setStatus("Zoxide support is disabled")
		return nil
	}
	if xdfileIsNetBoxPath(m.panels[m.activePanel].Cwd) {
		m.setStatus("Zoxide jump is local only")
		return nil
	}
	m.openInputModal(
		xdfileActionModalZoxideQuery,
		"Zoxide Jump",
		"Type a query and choose a directory from zoxide results.",
		m.activePanel,
		"",
		"",
	)
	m.modal.Input.Placeholder = "zoxide query"
	return nil
}

func (m *xdfileModel) startZoxideQuery(query string, panelIndex int) tea.Cmd {
	if !m.zoxideEnabled {
		m.setStatus("Zoxide support is disabled")
		return nil
	}
	if !m.validPanelIndex(panelIndex) {
		m.setStatus("Invalid panel")
		return nil
	}
	cwd := m.panels[panelIndex].Cwd
	if xdfileIsNetBoxPath(cwd) {
		m.setStatus("Zoxide jump is local only")
		return nil
	}
	query = strings.TrimSpace(query)
	m.setStatus("Querying zoxide...")
	return tea.Batch(func() tea.Msg {
		candidates, err := xdfileZoxideQueryCandidatesFunc(query, cwd, xdfileZoxideChoiceLimit)
		return xdfileZoxideQueryDoneMsg{
			Query:      query,
			PanelIndex: panelIndex,
			Candidates: candidates,
			Err:        err,
		}
	}, m.startBackgroundTask())
}

func (m *xdfileModel) applyZoxideQueryDone(msg xdfileZoxideQueryDoneMsg) tea.Cmd {
	m.stopBackgroundTask()
	if msg.Err != nil {
		if errors.Is(msg.Err, errXdfileZoxideUnavailable) {
			m.setStatus("Zoxide is unavailable")
			return nil
		}
		m.setStatusErr(fmt.Errorf("zoxide query failed: %w", msg.Err))
		return nil
	}
	candidates := xdfileExistingZoxideCandidates(msg.Candidates)
	if len(candidates) == 0 {
		m.setStatus("Zoxide returned no existing directories")
		return nil
	}
	m.openZoxideChoiceModal(msg.Query, msg.PanelIndex, candidates)
	return nil
}

func xdfileExistingZoxideCandidates(candidates []xdfileZoxideCandidate) []xdfileZoxideCandidate {
	filtered := make([]xdfileZoxideCandidate, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		path := strings.TrimSpace(candidate.Path)
		if path == "" {
			continue
		}
		clean := filepath.Clean(path)
		key := strings.ToLower(clean)
		if _, ok := seen[key]; ok {
			continue
		}
		info, err := os.Stat(clean)
		if err != nil || !info.IsDir() {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, xdfileZoxideCandidate{Path: clean})
	}
	return filtered
}

func (m *xdfileModel) openZoxideChoiceModal(query string, panelIndex int, candidates []xdfileZoxideCandidate) {
	limit := min(len(candidates), xdfileZoxideChoiceLimit)
	m.zoxideCandidates = append([]xdfileZoxideCandidate(nil), candidates[:limit]...)
	items := make([]xdfileModalChoiceItem, 0, limit)
	for i, candidate := range m.zoxideCandidates {
		items = append(items, xdfileModalChoiceItem{
			Action:      xdfileZoxideOpenAction(i),
			Label:       xdfileZoxideChoiceLabel(candidate.Path),
			Description: candidate.Path,
		})
	}
	description := "Choose a zoxide directory."
	if strings.TrimSpace(query) != "" {
		description = fmt.Sprintf("Choose a zoxide directory for %q.", query)
	}
	m.modal = xdfileModal{
		Kind:        xdfileModalChoice,
		Title:       "Zoxide Jump",
		Description: description,
		Action:      xdfileActionZoxideJump,
		ChoiceItems: items,
		Input:       m.modalInputModel(),
		Viewport:    m.modal.Viewport,
		PanelIndex:  panelIndex,
	}
	m.setStatus("Zoxide returned %d candidate%s", len(m.zoxideCandidates), xdfilePluralSuffix(len(m.zoxideCandidates)))
}

func xdfileZoxideChoiceLabel(path string) string {
	base := filepath.Base(path)
	parent := filepath.Base(filepath.Dir(path))
	if parent == "." || parent == string(filepath.Separator) || parent == "" {
		return base
	}
	return parent + "/" + base
}

func (m *xdfileModel) openZoxideCandidate(index int) tea.Cmd {
	if index < 0 || index >= len(m.zoxideCandidates) {
		m.setStatus("Zoxide candidate not found")
		return nil
	}
	candidate := m.zoxideCandidates[index]
	target := filepath.Clean(candidate.Path)
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		m.setStatus("Zoxide path no longer exists: %s", target)
		return nil
	}
	panelIndex := m.modal.PanelIndex
	if !m.validPanelIndex(panelIndex) {
		panelIndex = m.activePanel
	}
	m.closeModal()
	if err := m.changePanelDir(panelIndex, target, ""); err != nil {
		m.setStatusErr(err)
		return nil
	}
	m.activePanel = panelIndex
	m.setStatus("Zoxide jumped to %s", target)
	return m.syncTerminalToPanel(panelIndex)
}

func xdfileZoxideOpenAction(index int) xdfileAction {
	return xdfileAction(xdfileZoxideOpenActionPrefix + strconv.Itoa(index))
}

func xdfileParseZoxideOpenAction(action xdfileAction) (int, bool) {
	value := string(action)
	if !strings.HasPrefix(value, xdfileZoxideOpenActionPrefix) {
		return 0, false
	}
	index, err := strconv.Atoi(strings.TrimPrefix(value, xdfileZoxideOpenActionPrefix))
	if err != nil {
		return 0, false
	}
	return index, true
}

func xdfileQueryZoxideCandidates(query string, cwd string, limit int) ([]xdfileZoxideCandidate, error) {
	exe, err := xdfileZoxideLookPathFunc("zoxide")
	if err != nil {
		return nil, errXdfileZoxideUnavailable
	}
	args := []string{"query", "-l"}
	if fields := strings.Fields(strings.TrimSpace(query)); len(fields) > 0 {
		args = append(args, fields...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), xdfileZoxideQueryTimeout)
	defer cancel()
	output, err := xdfileZoxideCommandOutputFunc(ctx, exe, args, cwd)
	if err != nil {
		return nil, err
	}
	return xdfileParseZoxideOutput(string(output), limit), nil
}

func xdfileZoxideCommandOutput(ctx context.Context, exe string, args []string, cwd string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	if strings.TrimSpace(cwd) != "" && !xdfileIsNetBoxPath(cwd) {
		cmd.Dir = cwd
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		if text == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", err, text)
	}
	return output, nil
}

func xdfileParseZoxideOutput(output string, limit int) []xdfileZoxideCandidate {
	candidates := make([]xdfileZoxideCandidate, 0)
	seen := map[string]struct{}{}
	for _, line := range strings.Split(output, "\n") {
		path := strings.TrimSpace(line)
		if path == "" {
			continue
		}
		clean := filepath.Clean(path)
		key := strings.ToLower(clean)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		candidates = append(candidates, xdfileZoxideCandidate{Path: clean})
		if limit > 0 && len(candidates) >= limit {
			break
		}
	}
	return candidates
}
