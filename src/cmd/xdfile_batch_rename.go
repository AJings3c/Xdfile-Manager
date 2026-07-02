package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var xdfileBatchRenameCaseSensitiveDirFunc = xdfileDetectCaseSensitiveDir

func (m *xdfileModel) openBatchRenameModal() tea.Cmd {
	entries := m.activeFileSelectionEntries()
	if len(entries) == 0 {
		m.setStatus("Select one or more local items to batch rename")
		return nil
	}

	sourcePaths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsParent {
			continue
		}
		if xdfileIsNetBoxPath(entry.Path) {
			m.setStatus("Remote batch rename is unavailable")
			return nil
		}
		sourcePaths = append(sourcePaths, entry.Path)
	}
	if len(sourcePaths) == 0 {
		m.setStatus("Select one or more local items to batch rename")
		return nil
	}

	m.openInputModal(
		xdfileActionModalBatchRename,
		"Batch Rename",
		"Template tokens: {base}, {ext}, {name}, {index}. Preview is required before writing.",
		m.activePanel,
		sourcePaths[0],
		"{base}-{index}{ext}",
	)
	m.modal.SourcePaths = sourcePaths
	return nil
}

func (m *xdfileModel) prepareBatchRenamePreview() tea.Cmd {
	panelIndex := m.modal.PanelIndex
	if !m.validPanelIndex(panelIndex) {
		m.setStatus("Invalid panel")
		return nil
	}
	if xdfileIsNetBoxPath(m.panels[panelIndex].Cwd) {
		m.setStatus("Remote batch rename is unavailable")
		return nil
	}

	sourcePaths := append([]string(nil), m.modal.SourcePaths...)
	if len(sourcePaths) == 0 && m.modal.SourcePath != "" {
		sourcePaths = []string{m.modal.SourcePath}
	}
	entries, err := xdfileBatchRenameEntriesFromSources(sourcePaths)
	if err != nil {
		m.setStatusErr(err)
		return nil
	}

	template := m.modal.Input.Value()
	caseSensitive := xdfileBatchRenameCaseSensitiveDirFunc(m.panels[panelIndex].Cwd)
	items, err := xdfileBuildBatchRenamePlan(entries, template, caseSensitive)
	if err != nil {
		m.setStatusErr(err)
		return nil
	}

	pending := &xdfilePendingBatchRename{
		Items:      items,
		PanelIndex: panelIndex,
		Template:   xdfileNormalizeBatchRenameTemplate(template),
	}
	m.pendingBatchRename = pending
	m.openTextModalWithAction(
		xdfileActionBatchRenamePreview,
		"Batch Rename Preview",
		xdfileBatchRenamePreviewText(pending),
		"Enter rename | Esc cancel | Up/Down/PgUp/PgDn scroll",
	)
	return nil
}

func (m *xdfileModel) confirmBatchRenamePreview() tea.Cmd {
	pending := m.pendingBatchRename
	if pending == nil || len(pending.Items) == 0 {
		m.closeModal()
		m.setStatus("Batch rename preview expired")
		return nil
	}
	items := append([]xdfileBatchRenameItem(nil), pending.Items...)
	panelIndex := pending.PanelIndex
	m.pendingBatchRename = nil
	m.closeModal()
	return m.startFileOperation(xdfileFileOperation{
		Kind:        xdfileFileOperationBatchRename,
		PanelIndex:  panelIndex,
		RenameItems: items,
	})
}

func xdfileBatchRenameEntriesFromSources(sourcePaths []string) ([]xdfileEntry, error) {
	entries := make([]xdfileEntry, 0, len(sourcePaths))
	for _, sourcePath := range sourcePaths {
		sourcePath = strings.TrimSpace(sourcePath)
		if sourcePath == "" {
			continue
		}
		if xdfileIsNetBoxPath(sourcePath) {
			return nil, fmt.Errorf("remote batch rename is unavailable: %s", sourcePath)
		}
		info, err := os.Lstat(sourcePath)
		if err != nil {
			return nil, err
		}
		entries = append(entries, xdfileEntry{
			Name:  filepath.Base(sourcePath),
			Path:  filepath.Clean(sourcePath),
			IsDir: info.IsDir(),
		})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("select one or more local items to batch rename")
	}
	return entries, nil
}

func xdfileBuildBatchRenamePlan(entries []xdfileEntry, template string, caseSensitive bool) ([]xdfileBatchRenameItem, error) {
	template = xdfileNormalizeBatchRenameTemplate(template)
	if template == "" {
		return nil, fmt.Errorf("batch rename template cannot be empty")
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("select one or more local items to batch rename")
	}

	items := make([]xdfileBatchRenameItem, 0, len(entries))
	for i, entry := range entries {
		if entry.IsParent {
			continue
		}
		if xdfileIsNetBoxPath(entry.Path) {
			return nil, fmt.Errorf("remote batch rename is unavailable: %s", entry.Path)
		}
		sourcePath := filepath.Clean(entry.Path)
		if _, err := os.Lstat(sourcePath); err != nil {
			return nil, err
		}
		newName := xdfileExpandBatchRenameTemplate(entry, template, i+1)
		if err := xdfileValidateBatchRenameName(newName); err != nil {
			return nil, err
		}
		targetPath := filepath.Join(filepath.Dir(sourcePath), newName)
		items = append(items, xdfileBatchRenameItem{
			SourcePath: sourcePath,
			TargetPath: filepath.Clean(targetPath),
			OldName:    filepath.Base(sourcePath),
			NewName:    newName,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("select one or more local items to batch rename")
	}
	if err := xdfileValidateBatchRenameItems(items, caseSensitive); err != nil {
		return nil, err
	}
	return items, nil
}

func xdfileNormalizeBatchRenameTemplate(template string) string {
	return strings.TrimSpace(template)
}

func xdfileExpandBatchRenameTemplate(entry xdfileEntry, template string, index int) string {
	name := entry.Name
	if name == "" {
		name = filepath.Base(entry.Path)
	}
	base := name
	ext := ""
	if !entry.IsDir {
		ext = filepath.Ext(name)
		base = strings.TrimSuffix(name, ext)
		if base == "" {
			base = name
			ext = ""
		}
	}
	result := template
	result = strings.ReplaceAll(result, "{name}", name)
	result = strings.ReplaceAll(result, "{base}", base)
	result = strings.ReplaceAll(result, "{ext}", ext)
	result = strings.ReplaceAll(result, "{index}", fmt.Sprintf("%d", index))
	return strings.TrimSpace(result)
}

func xdfileValidateBatchRenameName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("batch rename target name cannot be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("batch rename target name is invalid: %q", name)
	}
	if filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, 0) {
		return fmt.Errorf("batch rename target must be a file name, got %q", name)
	}
	return nil
}

func xdfileValidateBatchRenameItems(items []xdfileBatchRenameItem, caseSensitive bool) error {
	sourceOwners := make(map[string]int, len(items))
	targetOwners := make(map[string]int, len(items))
	changed := 0
	for i, item := range items {
		sourceKey := xdfileBatchRenamePathKey(item.SourcePath, caseSensitive)
		targetKey := xdfileBatchRenamePathKey(item.TargetPath, caseSensitive)
		if owner, ok := sourceOwners[sourceKey]; ok {
			return fmt.Errorf("batch rename source is duplicated: %s and %s", items[owner].SourcePath, item.SourcePath)
		}
		sourceOwners[sourceKey] = i
		if owner, ok := targetOwners[targetKey]; ok {
			return fmt.Errorf("batch rename target is duplicated: %s and %s", items[owner].NewName, item.NewName)
		}
		targetOwners[targetKey] = i
		if filepath.Clean(item.SourcePath) != filepath.Clean(item.TargetPath) {
			changed++
		}
	}
	if changed == 0 {
		return fmt.Errorf("batch rename would not change any item")
	}

	for i, item := range items {
		sourceKey := xdfileBatchRenamePathKey(item.SourcePath, caseSensitive)
		targetKey := xdfileBatchRenamePathKey(item.TargetPath, caseSensitive)
		if owner, ok := sourceOwners[targetKey]; ok && owner != i {
			return fmt.Errorf("batch rename target conflicts with selected source: %s", item.TargetPath)
		}
		if _, err := os.Lstat(item.TargetPath); err == nil {
			if sourceKey == targetKey {
				continue
			}
			return fmt.Errorf("batch rename target already exists: %s", item.TargetPath)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func xdfileBatchRenamePathKey(path string, caseSensitive bool) string {
	key := filepath.Clean(path)
	if !caseSensitive {
		key = strings.ToLower(key)
	}
	return key
}

func xdfileBatchRenamePreviewText(pending *xdfilePendingBatchRename) string {
	if pending == nil {
		return ""
	}
	lines := []string{
		fmt.Sprintf("Template: %s", pending.Template),
		fmt.Sprintf("Items: %d", len(pending.Items)),
		"",
	}
	for _, item := range pending.Items {
		lines = append(lines, fmt.Sprintf("- %s", item.OldName))
		lines = append(lines, fmt.Sprintf("+ %s", item.NewName))
		lines = append(lines, "")
	}
	lines = append(lines, "Press Enter to rename these items. Press Esc to cancel without writing.")
	return strings.Join(lines, "\n")
}

func xdfileRunBatchRenameFileOperation(
	ctx context.Context,
	items []xdfileBatchRenameItem,
	progress *xdfileFileOperationProgress,
) (int, []xdfileFileOperationFailure, error) {
	successes := 0
	failures := make([]xdfileFileOperationFailure, 0)
	for _, item := range items {
		if err := xdfileCheckFileOperationContext(ctx); err != nil {
			return successes, failures, err
		}
		if err := xdfileRenamePath(item.SourcePath, item.TargetPath); err != nil {
			failures = append(failures, xdfileFileOperationFailure{
				SourcePath: item.SourcePath,
				TargetPath: item.TargetPath,
				Err:        err,
			})
			continue
		}
		successes++
		progress.addItem()
	}
	return successes, failures, nil
}

func xdfileDetectCaseSensitiveDir(dir string) bool {
	temp, err := os.CreateTemp(dir, ".xdfile-case-")
	if err != nil {
		return true
	}
	name := filepath.Base(temp.Name())
	_ = temp.Close()
	defer os.Remove(temp.Name())

	upperName := strings.ToUpper(name)
	if upperName == name {
		upperName = strings.ToLower(name)
	}
	if upperName == name {
		return true
	}
	_, err = os.Lstat(filepath.Join(dir, upperName))
	return os.IsNotExist(err)
}
