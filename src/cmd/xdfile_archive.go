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
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type xdfileArchiveFormat string

const (
	xdfileArchiveFormatZIP   xdfileArchiveFormat = "zip"
	xdfileArchiveFormatTarGZ xdfileArchiveFormat = "tar.gz"
)

func (m *xdfileModel) openArchiveModal() tea.Cmd {
	entries := m.activeFileSelectionEntries()
	if len(entries) == 0 {
		m.setStatus("Select a file or directory to pack")
		return nil
	}
	panel := &m.panels[m.activePanel]
	sourcePaths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsParent {
			continue
		}
		if xdfileIsNetBoxPath(entry.Path) {
			m.setStatus("Remote archive creation is unavailable")
			return nil
		}
		sourcePaths = append(sourcePaths, entry.Path)
	}
	if len(sourcePaths) == 0 {
		m.setStatus("Select a file or directory to pack")
		return nil
	}

	initial := xdfileDefaultArchiveName(entries)
	description := fmt.Sprintf("Pack %s into a .zip or .tar.gz archive.", entries[0].Name)
	if len(entries) > 1 {
		description = fmt.Sprintf("Pack %d selected items into a .zip or .tar.gz archive.", len(entries))
	}
	m.openInputModal(
		xdfileActionModalArchive,
		"Pack Archive",
		description,
		m.activePanel,
		sourcePaths[0],
		initial,
	)
	m.modal.SourcePaths = sourcePaths
	m.modal.TargetPath = panel.Cwd
	m.setStatus("Enter archive filename, ending with .zip or .tar.gz")
	return nil
}

func xdfileDefaultArchiveName(entries []xdfileEntry) string {
	if len(entries) == 1 {
		name := strings.TrimSpace(entries[0].Name)
		if name == "" {
			name = "archive"
		}
		if !entries[0].IsDir {
			ext := filepath.Ext(name)
			if ext != "" {
				name = strings.TrimSuffix(name, ext)
			}
		}
		return name + ".zip"
	}
	return "selection.zip"
}

func (m *xdfileModel) resolveArchiveTarget(panelIndex int, value string) (string, error) {
	if !m.validPanelIndex(panelIndex) {
		return "", fmt.Errorf("invalid panel")
	}
	panel := &m.panels[panelIndex]
	if xdfileIsNetBoxPath(panel.Cwd) {
		return "", fmt.Errorf("remote archive target is unavailable")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("archive filename cannot be empty")
	}
	if strings.HasSuffix(value, "/") || strings.HasSuffix(value, `\`) || filepath.Base(filepath.Clean(value)) == "." {
		return "", fmt.Errorf("archive filename cannot be a directory")
	}
	value = xdfileArchiveTargetWithDefaultExtension(value)
	if _, err := xdfileArchiveFormatForTarget(value); err != nil {
		return "", err
	}
	targetPath := value
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(panel.Cwd, targetPath)
	}
	return filepath.Clean(targetPath), nil
}

func (m *xdfileModel) openArchiveConflict(pending *xdfilePendingArchive) {
	if pending == nil {
		return
	}
	m.pendingArchive = &xdfilePendingArchive{
		SourcePaths: append([]string(nil), pending.SourcePaths...),
		TargetPath:  pending.TargetPath,
		PanelIndex:  pending.PanelIndex,
	}
	items := []xdfileModalChoiceItem{
		{
			Action:      xdfileActionArchiveConflictReplace,
			Label:       "Replace",
			Description: "Replace the existing archive after the new archive is written successfully.",
		},
		{
			Action:      xdfileActionArchiveConflictSkip,
			Label:       "Skip",
			Description: "Cancel this archive creation and leave the existing file unchanged.",
		},
		{
			Action:      xdfileActionArchiveConflictRename,
			Label:       "Keep both",
			Description: "Create a numbered archive next to the existing file.",
		},
	}
	m.openChoiceModal(
		"Archive Exists",
		fmt.Sprintf("%s already exists.", pending.TargetPath),
		items,
	)
	m.modal.Action = xdfileActionArchiveConflictPrompt
	m.setStatus("Choose Replace, Skip, or Keep both")
}

func (m *xdfileModel) resolvePendingArchiveConflict(action xdfileAction) tea.Cmd {
	pending := m.pendingArchive
	if pending == nil {
		m.setStatus("No pending archive conflict")
		return nil
	}
	sourcePaths := append([]string(nil), pending.SourcePaths...)
	targetPath := pending.TargetPath
	panelIndex := pending.PanelIndex
	m.pendingArchive = nil

	switch action {
	case xdfileActionArchiveConflictReplace:
		m.closeModal()
		return m.startArchiveOperation(sourcePaths, targetPath, panelIndex, true)
	case xdfileActionArchiveConflictSkip:
		m.closeModal()
		m.setStatus("Archive skipped")
		return nil
	case xdfileActionArchiveConflictRename:
		renamedTarget, err := xdfileUniqueArchiveTarget(targetPath)
		if err != nil {
			m.setStatusErr(err)
			return nil
		}
		m.closeModal()
		return m.startArchiveOperation(sourcePaths, renamedTarget, panelIndex, false)
	default:
		m.setStatus("Choose Replace, Skip, or Keep both")
		return nil
	}
}

func (m *xdfileModel) startArchiveOperation(sourcePaths []string, targetPath string, panelIndex int, replace bool) tea.Cmd {
	if len(sourcePaths) == 0 {
		m.setStatus("Select a file or directory to pack")
		return nil
	}
	sourcePath := sourcePaths[0]
	return m.startFileOperation(xdfileFileOperation{
		Kind:          xdfileFileOperationArchive,
		SourcePath:    sourcePath,
		SourcePaths:   sourcePaths,
		TargetPath:    targetPath,
		PanelIndex:    panelIndex,
		ReplaceTarget: replace,
	})
}

func xdfileRunArchiveFileOperation(
	ctx context.Context,
	op xdfileFileOperation,
	progress *xdfileFileOperationProgress,
) (int, []xdfileFileOperationFailure, error) {
	sources := op.transferSources()
	if len(sources) == 0 {
		return 0, nil, nil
	}
	count, err := xdfileCreateArchiveContext(ctx, sources, op.TargetPath, op.ReplaceTarget, progress)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return count, nil, err
		}
		return count, []xdfileFileOperationFailure{{
			SourcePath: strings.Join(sources, "\n"),
			TargetPath: op.TargetPath,
			Err:        err,
		}}, nil
	}
	return count, nil, nil
}

func xdfileCreateArchiveContext(
	ctx context.Context,
	sourcePaths []string,
	targetPath string,
	replace bool,
	progress *xdfileFileOperationProgress,
) (int, error) {
	if err := xdfileCheckFileOperationContext(ctx); err != nil {
		return 0, err
	}
	sources, err := xdfileNormalizeArchiveSources(sourcePaths)
	if err != nil {
		return 0, err
	}
	if len(sources) == 0 {
		return 0, nil
	}

	targetPath = filepath.Clean(strings.TrimSpace(targetPath))
	format, err := xdfileArchiveFormatForTarget(targetPath)
	if err != nil {
		return 0, err
	}
	if err := xdfileValidateArchiveTarget(sources, targetPath, replace); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return 0, err
	}

	tmp, err := os.CreateTemp(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".tmp-*")
	if err != nil {
		return 0, err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	count, err := xdfileWriteArchive(ctx, sources, tmp, format, progress)
	closeErr := tmp.Close()
	if err != nil {
		return count, err
	}
	if closeErr != nil {
		return count, closeErr
	}

	if err := xdfileCheckFileOperationContext(ctx); err != nil {
		return count, err
	}
	if replace {
		info, statErr := os.Stat(targetPath)
		if statErr == nil && info.IsDir() {
			return count, fmt.Errorf("archive target is a directory: %s", targetPath)
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return count, statErr
		}
		if statErr == nil {
			if err := os.Remove(targetPath); err != nil {
				return count, err
			}
		}
	} else if _, err := os.Stat(targetPath); err == nil {
		return count, fmt.Errorf("target already exists: %s", targetPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return count, err
	}

	if err := xdfileOSRenameFunc(tmpPath, targetPath); err != nil {
		return count, err
	}
	cleanup = false
	return count, nil
}

func xdfileNormalizeArchiveSources(sourcePaths []string) ([]string, error) {
	sources := make([]string, 0, len(sourcePaths))
	seen := make(map[string]struct{}, len(sourcePaths))
	for _, source := range sourcePaths {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		if xdfileIsNetBoxPath(source) {
			return nil, fmt.Errorf("remote archive creation is unavailable: %s", source)
		}
		clean := filepath.Clean(source)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		sources = append(sources, clean)
	}
	return sources, nil
}

func xdfileValidateArchiveTarget(sources []string, targetPath string, replace bool) error {
	if targetPath == "." || strings.TrimSpace(targetPath) == "" {
		return fmt.Errorf("archive target cannot be empty")
	}
	if xdfileIsNetBoxPath(targetPath) {
		return fmt.Errorf("remote archive target is unavailable: %s", targetPath)
	}

	targetClean := filepath.Clean(targetPath)
	for _, source := range sources {
		info, err := os.Lstat(source)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not supported in archives yet: %s", source)
		}
		if xdfilePathsEqual(source, targetClean) {
			return fmt.Errorf("archive target matches source: %s", targetClean)
		}
		if info.IsDir() && xdfilePathWithinRoot(source, targetClean) {
			return fmt.Errorf("archive target is inside source directory: %s", targetClean)
		}
	}

	if info, err := os.Stat(targetClean); err == nil {
		if info.IsDir() {
			return fmt.Errorf("archive target is a directory: %s", targetClean)
		}
		if !replace {
			return fmt.Errorf("target already exists: %s", targetClean)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func xdfileArchiveFormatForTarget(targetPath string) (xdfileArchiveFormat, error) {
	name := strings.ToLower(filepath.Base(strings.TrimSpace(targetPath)))
	switch {
	case strings.HasSuffix(name, ".zip"):
		return xdfileArchiveFormatZIP, nil
	case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"):
		return xdfileArchiveFormatTarGZ, nil
	default:
		return "", fmt.Errorf("archive target must end with .zip or .tar.gz: %s", targetPath)
	}
}

func xdfileWriteArchive(
	ctx context.Context,
	sources []string,
	target io.Writer,
	format xdfileArchiveFormat,
	progress *xdfileFileOperationProgress,
) (int, error) {
	switch format {
	case xdfileArchiveFormatZIP:
		writer := zip.NewWriter(target)
		count, err := xdfileWriteZipArchive(ctx, sources, writer, progress)
		closeErr := writer.Close()
		if err != nil {
			return count, err
		}
		return count, closeErr
	case xdfileArchiveFormatTarGZ:
		gzipWriter := gzip.NewWriter(target)
		tarWriter := tar.NewWriter(gzipWriter)
		count, err := xdfileWriteTarGZArchive(ctx, sources, tarWriter, progress)
		tarCloseErr := tarWriter.Close()
		gzipCloseErr := gzipWriter.Close()
		if err != nil {
			return count, err
		}
		if tarCloseErr != nil {
			return count, tarCloseErr
		}
		return count, gzipCloseErr
	default:
		return 0, fmt.Errorf("unsupported archive format: %s", format)
	}
}

func xdfileWriteZipArchive(
	ctx context.Context,
	sources []string,
	writer *zip.Writer,
	progress *xdfileFileOperationProgress,
) (int, error) {
	count := 0
	for _, source := range sources {
		written, err := xdfileAddPathToZip(ctx, writer, source, filepath.Base(source), progress)
		count += written
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func xdfileAddPathToZip(
	ctx context.Context,
	writer *zip.Writer,
	sourcePath string,
	archiveName string,
	progress *xdfileFileOperationProgress,
) (int, error) {
	if err := xdfileCheckFileOperationContext(ctx); err != nil {
		return 0, err
	}
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("symlinks are not supported in archives yet: %s", sourcePath)
	}

	name := xdfileArchiveSlashName(archiveName, info.IsDir())
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return 0, err
	}
	header.Name = name
	if !info.IsDir() {
		header.Method = zip.Deflate
	}
	target, err := writer.CreateHeader(header)
	if err != nil {
		return 0, err
	}
	count := 1
	if !info.IsDir() {
		if err := xdfileCopyArchiveFile(ctx, target, sourcePath, progress); err != nil {
			return count, err
		}
		progress.addItem()
		return count, nil
	}
	progress.addItem()

	children, err := os.ReadDir(sourcePath)
	if err != nil {
		return count, err
	}
	for _, child := range children {
		childSource := filepath.Join(sourcePath, child.Name())
		childName := filepath.ToSlash(filepath.Join(strings.TrimSuffix(name, "/"), child.Name()))
		written, err := xdfileAddPathToZip(ctx, writer, childSource, childName, progress)
		count += written
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func xdfileWriteTarGZArchive(
	ctx context.Context,
	sources []string,
	writer *tar.Writer,
	progress *xdfileFileOperationProgress,
) (int, error) {
	count := 0
	for _, source := range sources {
		written, err := xdfileAddPathToTar(ctx, writer, source, filepath.Base(source), progress)
		count += written
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func xdfileAddPathToTar(
	ctx context.Context,
	writer *tar.Writer,
	sourcePath string,
	archiveName string,
	progress *xdfileFileOperationProgress,
) (int, error) {
	if err := xdfileCheckFileOperationContext(ctx); err != nil {
		return 0, err
	}
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("symlinks are not supported in archives yet: %s", sourcePath)
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return 0, err
	}
	header.Name = xdfileArchiveSlashName(archiveName, info.IsDir())
	if err := writer.WriteHeader(header); err != nil {
		return 0, err
	}
	count := 1
	if !info.IsDir() {
		if err := xdfileCopyArchiveFile(ctx, writer, sourcePath, progress); err != nil {
			return count, err
		}
		progress.addItem()
		return count, nil
	}
	progress.addItem()

	children, err := os.ReadDir(sourcePath)
	if err != nil {
		return count, err
	}
	for _, child := range children {
		childSource := filepath.Join(sourcePath, child.Name())
		childName := filepath.ToSlash(filepath.Join(strings.TrimSuffix(header.Name, "/"), child.Name()))
		written, err := xdfileAddPathToTar(ctx, writer, childSource, childName, progress)
		count += written
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func xdfileCopyArchiveFile(
	ctx context.Context,
	writer io.Writer,
	sourcePath string,
	progress *xdfileFileOperationProgress,
) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	buffer := make([]byte, 1024*1024)
	for {
		if err := xdfileCheckFileOperationContext(ctx); err != nil {
			return err
		}
		n, readErr := source.Read(buffer)
		if n > 0 {
			if _, err := writer.Write(buffer[:n]); err != nil {
				return err
			}
			progress.addBytes(n)
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func xdfileArchiveSlashName(name string, isDir bool) string {
	name = filepath.ToSlash(filepath.Clean(strings.TrimSpace(name)))
	name = strings.TrimPrefix(name, "../")
	name = strings.TrimPrefix(name, "/")
	if name == "." || name == "" {
		name = "item"
	}
	if isDir && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	return name
}

func xdfileArchiveTargetWithDefaultExtension(targetPath string) string {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return targetPath
	}
	if _, err := xdfileArchiveFormatForTarget(targetPath); err == nil {
		return targetPath
	}
	if filepath.Ext(filepath.Base(targetPath)) == "" {
		return targetPath + ".zip"
	}
	return targetPath
}

func xdfileUniqueArchiveTarget(targetPath string) (string, error) {
	targetPath = filepath.Clean(targetPath)
	if _, err := os.Stat(targetPath); errors.Is(err, os.ErrNotExist) {
		return targetPath, nil
	} else if err != nil {
		return "", err
	}

	dir, base, ext := xdfileArchiveNameParts(targetPath)
	for i := 2; i <= 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("unable to find a free archive target for %s", targetPath)
}

func xdfileArchiveNameParts(targetPath string) (string, string, string) {
	targetPath = filepath.Clean(targetPath)
	dir := filepath.Dir(targetPath)
	name := filepath.Base(targetPath)
	lower := strings.ToLower(name)
	for _, ext := range []string{".tar.gz", ".tgz", ".zip"} {
		if strings.HasSuffix(lower, ext) {
			return dir, name[:len(name)-len(ext)], name[len(name)-len(ext):]
		}
	}
	ext := filepath.Ext(name)
	return dir, strings.TrimSuffix(name, ext), ext
}
