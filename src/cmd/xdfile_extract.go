package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *xdfileModel) openExtractArchiveModal() tea.Cmd {
	entries := m.activeFileSelectionEntries()
	if len(entries) == 0 {
		m.setStatus("Select an archive to extract")
		return nil
	}
	if len(entries) > 1 {
		m.setStatus("Extract one archive at a time")
		return nil
	}
	entry := entries[0]
	if entry.IsParent || entry.IsDir {
		m.setStatus("Select a .zip or .tar.gz file to extract")
		return nil
	}
	if xdfileIsNetBoxPath(entry.Path) {
		m.setStatus("Remote archive extraction is unavailable")
		return nil
	}
	if _, err := xdfileArchiveFormatForTarget(entry.Path); err != nil {
		m.setStatus("Select a .zip or .tar.gz archive")
		return nil
	}

	m.openInputModal(
		xdfileActionModalExtract,
		"Extract Archive",
		fmt.Sprintf("Extract %s into a local folder.", entry.Name),
		m.activePanel,
		entry.Path,
		xdfileDefaultExtractTargetName(entry.Name),
	)
	m.modal.TargetPath = m.panels[m.activePanel].Cwd
	m.setStatus("Enter extraction folder")
	return nil
}

func xdfileDefaultExtractTargetName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "extracted"
	}
	lower := strings.ToLower(name)
	for _, ext := range []string{".tar.gz", ".tgz", ".zip"} {
		if strings.HasSuffix(lower, ext) {
			base := strings.TrimSpace(name[:len(name)-len(ext)])
			if base != "" {
				return base
			}
		}
	}
	ext := filepath.Ext(name)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	if name == "" {
		return "extracted"
	}
	return name
}

func (m *xdfileModel) resolveExtractTarget(panelIndex int, value string) (string, error) {
	if !m.validPanelIndex(panelIndex) {
		return "", fmt.Errorf("invalid panel")
	}
	panel := &m.panels[panelIndex]
	if xdfileIsNetBoxPath(panel.Cwd) {
		return "", fmt.Errorf("remote extraction target is unavailable")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("extraction folder cannot be empty")
	}
	if value == "." || value == string(os.PathSeparator) {
		return "", fmt.Errorf("choose a dedicated extraction folder")
	}
	targetPath := value
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(panel.Cwd, targetPath)
	}
	targetPath = filepath.Clean(targetPath)
	if xdfilePathsEqual(targetPath, panel.Cwd) {
		return "", fmt.Errorf("choose a dedicated extraction folder")
	}
	return targetPath, nil
}

func (m *xdfileModel) openExtractConflict(pending *xdfilePendingExtract) {
	if pending == nil {
		return
	}
	m.pendingExtract = &xdfilePendingExtract{
		SourcePath: pending.SourcePath,
		TargetPath: pending.TargetPath,
		PanelIndex: pending.PanelIndex,
	}
	items := []xdfileModalChoiceItem{
		{
			Action:      xdfileActionExtractConflictReplace,
			Label:       "Replace",
			Description: "Replace the existing extraction target after the archive is staged successfully.",
		},
		{
			Action:      xdfileActionExtractConflictSkip,
			Label:       "Skip",
			Description: "Cancel this extraction and leave the existing target unchanged.",
		},
		{
			Action:      xdfileActionExtractConflictRename,
			Label:       "Keep both",
			Description: "Extract into a numbered folder next to the existing target.",
		},
	}
	m.openChoiceModal("Extract Target Exists", fmt.Sprintf("%s already exists.", pending.TargetPath), items)
	m.modal.Action = xdfileActionExtractConflictPrompt
	m.setStatus("Choose Replace, Skip, or Keep both")
}

func (m *xdfileModel) resolvePendingExtractConflict(action xdfileAction) tea.Cmd {
	pending := m.pendingExtract
	if pending == nil {
		m.setStatus("No pending extract conflict")
		return nil
	}
	sourcePath := pending.SourcePath
	targetPath := pending.TargetPath
	panelIndex := pending.PanelIndex
	m.pendingExtract = nil

	switch action {
	case xdfileActionExtractConflictReplace:
		m.closeModal()
		return m.startExtractOperation(sourcePath, targetPath, panelIndex, xdfileActionExtractConflictReplace)
	case xdfileActionExtractConflictSkip:
		m.closeModal()
		m.setStatus("Extraction skipped")
		return nil
	case xdfileActionExtractConflictRename:
		renamedTarget, err := xdfileUniqueExtractTarget(targetPath)
		if err != nil {
			m.setStatusErr(err)
			return nil
		}
		m.closeModal()
		return m.startExtractOperation(sourcePath, renamedTarget, panelIndex, "")
	default:
		m.setStatus("Choose Replace, Skip, or Keep both")
		return nil
	}
}

func (m *xdfileModel) startExtractOperation(sourcePath string, targetPath string, panelIndex int, policy xdfileAction) tea.Cmd {
	return m.startFileOperation(xdfileFileOperation{
		Kind:           xdfileFileOperationExtract,
		SourcePath:     sourcePath,
		TargetPath:     targetPath,
		PanelIndex:     panelIndex,
		ConflictPolicy: policy,
	})
}

func xdfileRunExtractFileOperation(
	ctx context.Context,
	op xdfileFileOperation,
	progress *xdfileFileOperationProgress,
) (int, []xdfileFileOperationFailure, error) {
	count, err := xdfileExtractArchiveContext(ctx, op.SourcePath, op.TargetPath, op.ConflictPolicy, progress)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return count, nil, err
		}
		return count, []xdfileFileOperationFailure{{
			SourcePath: op.SourcePath,
			TargetPath: op.TargetPath,
			Err:        err,
		}}, nil
	}
	return count, nil, nil
}

func xdfileExtractArchiveContext(
	ctx context.Context,
	archivePath string,
	targetPath string,
	policy xdfileAction,
	progress *xdfileFileOperationProgress,
) (int, error) {
	if err := xdfileCheckFileOperationContext(ctx); err != nil {
		return 0, err
	}
	archivePath = strings.TrimSpace(archivePath)
	targetPath = filepath.Clean(strings.TrimSpace(targetPath))
	if archivePath == "" || targetPath == "." || targetPath == "" {
		return 0, fmt.Errorf("archive source and extraction target are required")
	}
	if xdfileIsNetBoxPath(archivePath) || xdfileIsNetBoxPath(targetPath) {
		return 0, fmt.Errorf("remote archive extraction is unavailable")
	}
	format, err := xdfileArchiveFormatForTarget(archivePath)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		return 0, err
	}
	if info.IsDir() {
		return 0, fmt.Errorf("archive source is a directory: %s", archivePath)
	}
	if err := xdfileValidateArchiveEntriesForExtraction(archivePath, format); err != nil {
		return 0, err
	}

	targetExists := false
	if existing, err := os.Lstat(targetPath); err == nil {
		targetExists = true
		if existing.Mode()&os.ModeSymlink != 0 {
			return 0, fmt.Errorf("extraction target is a symlink: %s", targetPath)
		}
		if policy == "" {
			return 0, fmt.Errorf("extraction target already exists: %s", targetPath)
		}
		if policy == xdfileActionExtractConflictSkip {
			return 0, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	if policy == xdfileActionExtractConflictRename && targetExists {
		renamed, err := xdfileUniqueExtractTarget(targetPath)
		if err != nil {
			return 0, err
		}
		targetPath = renamed
		targetExists = false
	}

	parent := filepath.Dir(targetPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return 0, err
	}
	stagingRoot, err := os.MkdirTemp(parent, "."+filepath.Base(targetPath)+".extract-*")
	if err != nil {
		return 0, err
	}
	cleanupStaging := true
	defer func() {
		if cleanupStaging {
			_ = os.RemoveAll(stagingRoot)
		}
	}()

	count, err := xdfileExtractArchiveToDir(ctx, archivePath, stagingRoot, format, progress)
	if err != nil {
		return count, err
	}
	if err := xdfileCheckFileOperationContext(ctx); err != nil {
		return count, err
	}

	replaceStage := ""
	if targetExists {
		if policy != xdfileActionExtractConflictReplace {
			return count, fmt.Errorf("extraction target already exists: %s", targetPath)
		}
		replaceStage, err = xdfileUniqueReplaceStageTarget(targetPath)
		if err != nil {
			return count, err
		}
		if err := xdfileOSRenameFunc(targetPath, replaceStage); err != nil {
			return count, err
		}
		restore := true
		defer func() {
			if restore {
				_ = os.RemoveAll(targetPath)
				_ = xdfileOSRenameFunc(replaceStage, targetPath)
			}
		}()
		if err := xdfileOSRenameFunc(stagingRoot, targetPath); err != nil {
			return count, err
		}
		cleanupStaging = false
		if err := os.RemoveAll(replaceStage); err != nil {
			return count, err
		}
		restore = false
		return count, nil
	}

	if err := xdfileOSRenameFunc(stagingRoot, targetPath); err != nil {
		return count, err
	}
	cleanupStaging = false
	return count, nil
}

func xdfileValidateArchiveEntriesForExtraction(archivePath string, format xdfileArchiveFormat) error {
	switch format {
	case xdfileArchiveFormatZIP:
		reader, err := zip.OpenReader(archivePath)
		if err != nil {
			return err
		}
		defer reader.Close()
		for _, file := range reader.File {
			if _, err := xdfileSafeArchiveEntryName(file.Name); err != nil {
				return err
			}
			mode := file.FileInfo().Mode()
			if mode&os.ModeSymlink != 0 {
				return fmt.Errorf("archive symlink entries are not allowed: %s", file.Name)
			}
			if mode.Type() != 0 && !file.FileInfo().IsDir() {
				return fmt.Errorf("archive special entries are not allowed: %s", file.Name)
			}
		}
		return nil
	case xdfileArchiveFormatTarGZ:
		file, err := os.Open(archivePath)
		if err != nil {
			return err
		}
		defer file.Close()
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		tarReader := tar.NewReader(gzipReader)
		for {
			header, err := tarReader.Next()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
			if _, err := xdfileSafeArchiveEntryName(header.Name); err != nil {
				return err
			}
			if !xdfileTarEntryTypeAllowed(header) {
				return fmt.Errorf("archive special entries are not allowed: %s", header.Name)
			}
		}
	default:
		return fmt.Errorf("unsupported archive format: %s", format)
	}
}

func xdfileExtractArchiveToDir(
	ctx context.Context,
	archivePath string,
	targetDir string,
	format xdfileArchiveFormat,
	progress *xdfileFileOperationProgress,
) (int, error) {
	switch format {
	case xdfileArchiveFormatZIP:
		return xdfileExtractZipArchiveToDir(ctx, archivePath, targetDir, progress)
	case xdfileArchiveFormatTarGZ:
		return xdfileExtractTarGZArchiveToDir(ctx, archivePath, targetDir, progress)
	default:
		return 0, fmt.Errorf("unsupported archive format: %s", format)
	}
}

func xdfileExtractZipArchiveToDir(
	ctx context.Context,
	archivePath string,
	targetDir string,
	progress *xdfileFileOperationProgress,
) (int, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	count := 0
	for _, file := range reader.File {
		if err := xdfileCheckFileOperationContext(ctx); err != nil {
			return count, err
		}
		name, err := xdfileSafeArchiveEntryName(file.Name)
		if err != nil {
			return count, err
		}
		targetPath, err := xdfileArchiveEntryTargetPath(targetDir, name)
		if err != nil {
			return count, err
		}
		info := file.FileInfo()
		if info.Mode()&os.ModeSymlink != 0 {
			return count, fmt.Errorf("archive symlink entries are not allowed: %s", file.Name)
		}
		if info.IsDir() {
			if err := os.MkdirAll(targetPath, info.Mode().Perm()); err != nil {
				return count, err
			}
			progress.addItem()
			count++
			continue
		}
		if info.Mode().Type() != 0 {
			return count, fmt.Errorf("archive special entries are not allowed: %s", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return count, err
		}
		rc, err := file.Open()
		if err != nil {
			return count, err
		}
		err = xdfileWriteExtractedFile(ctx, targetPath, info.Mode().Perm(), rc, progress)
		closeErr := rc.Close()
		if err != nil {
			return count, err
		}
		if closeErr != nil {
			return count, closeErr
		}
		_ = os.Chtimes(targetPath, file.Modified, file.Modified)
		progress.addItem()
		count++
	}
	return count, nil
}

func xdfileExtractTarGZArchiveToDir(
	ctx context.Context,
	archivePath string,
	targetDir string,
	progress *xdfileFileOperationProgress,
) (int, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return 0, err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	count := 0
	for {
		if err := xdfileCheckFileOperationContext(ctx); err != nil {
			return count, err
		}
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return count, nil
		}
		if err != nil {
			return count, err
		}
		name, err := xdfileSafeArchiveEntryName(header.Name)
		if err != nil {
			return count, err
		}
		if !xdfileTarEntryTypeAllowed(header) {
			return count, fmt.Errorf("archive special entries are not allowed: %s", header.Name)
		}
		targetPath, err := xdfileArchiveEntryTargetPath(targetDir, name)
		if err != nil {
			return count, err
		}
		mode := os.FileMode(header.Mode).Perm()
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, mode); err != nil {
				return count, err
			}
			progress.addItem()
			count++
		default:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return count, err
			}
			if err := xdfileWriteExtractedFile(ctx, targetPath, mode, tarReader, progress); err != nil {
				return count, err
			}
			_ = os.Chtimes(targetPath, header.ModTime, header.ModTime)
			progress.addItem()
			count++
		}
	}
}

func xdfileTarEntryTypeAllowed(header *tar.Header) bool {
	switch header.Typeflag {
	case tar.TypeReg, tar.TypeRegA, tar.TypeDir:
		return true
	default:
		return false
	}
}

func xdfileWriteExtractedFile(
	ctx context.Context,
	targetPath string,
	perm os.FileMode,
	reader io.Reader,
	progress *xdfileFileOperationProgress,
) error {
	if err := xdfileCheckFileOperationContext(ctx); err != nil {
		return err
	}
	if perm == 0 {
		perm = 0o644
	}
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		_ = target.Close()
		if cleanup {
			_ = os.Remove(targetPath)
		}
	}()

	buffer := make([]byte, 1024*1024)
	for {
		if err := xdfileCheckFileOperationContext(ctx); err != nil {
			return err
		}
		n, readErr := reader.Read(buffer)
		if n > 0 {
			if _, err := target.Write(buffer[:n]); err != nil {
				return err
			}
			progress.addBytes(n)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := target.Close(); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func xdfileSafeArchiveEntryName(name string) (string, error) {
	raw := strings.TrimSpace(name)
	if raw == "" {
		return "", fmt.Errorf("archive entry name is empty")
	}
	raw = strings.ReplaceAll(raw, "\\", "/")
	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "~") || xdfileLooksLikeWindowsAbsArchivePath(raw) {
		return "", fmt.Errorf("archive entry path is unsafe: %s", name)
	}
	cleaned := path.Clean(raw)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("archive entry path is unsafe: %s", name)
	}
	return cleaned, nil
}

func xdfileLooksLikeWindowsAbsArchivePath(value string) bool {
	if len(value) >= 3 && value[1] == ':' && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) {
		return value[2] == '/'
	}
	return strings.HasPrefix(value, "//")
}

func xdfileArchiveEntryTargetPath(root string, entryName string) (string, error) {
	targetPath := filepath.Join(root, filepath.FromSlash(entryName))
	if !xdfilePathWithinRoot(root, targetPath) {
		return "", fmt.Errorf("archive entry path escapes target: %s", entryName)
	}
	return targetPath, nil
}

func xdfileUniqueExtractTarget(targetPath string) (string, error) {
	targetPath = filepath.Clean(targetPath)
	if _, err := os.Stat(targetPath); errors.Is(err, os.ErrNotExist) {
		return targetPath, nil
	} else if err != nil {
		return "", err
	}
	dir := filepath.Dir(targetPath)
	base := filepath.Base(targetPath)
	for i := 2; i <= 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)", base, i))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("unable to find a free extraction target for %s", targetPath)
}
