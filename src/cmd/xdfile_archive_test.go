package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestXdfileCreateZipArchiveIncludesFilesAndDirectories(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "docs")
	mustWriteFile(t, filepath.Join(sourceDir, "README.md"), "hello")
	mustWriteFile(t, filepath.Join(sourceDir, "nested", "note.txt"), "note")
	target := filepath.Join(root, "docs.zip")

	count, err := xdfileCreateArchiveContext(context.Background(), []string{sourceDir}, target, false, nil)
	if err != nil {
		t.Fatalf("create zip archive failed: %v", err)
	}
	if count != 4 {
		t.Fatalf("zip entry count = %d, want 4", count)
	}

	entries := readZipEntries(t, target)
	assertArchiveContent(t, entries, "docs/README.md", "hello")
	assertArchiveContent(t, entries, "docs/nested/note.txt", "note")
	for _, name := range []string{"docs/", "docs/nested/"} {
		if _, ok := entries[name]; !ok {
			t.Fatalf("missing directory entry %s in %#v", name, sortedArchiveNames(entries))
		}
	}
}

func TestXdfileCreateTarGZArchiveIncludesSelectedFiles(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	b := filepath.Join(root, "b.txt")
	mustWriteFile(t, a, "alpha")
	mustWriteFile(t, b, "beta")
	target := filepath.Join(root, "selection.tar.gz")

	count, err := xdfileCreateArchiveContext(context.Background(), []string{a, b}, target, false, nil)
	if err != nil {
		t.Fatalf("create tar.gz archive failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("tar.gz entry count = %d, want 2", count)
	}

	entries := readTarGZEntries(t, target)
	assertArchiveContent(t, entries, "a.txt", "alpha")
	assertArchiveContent(t, entries, "b.txt", "beta")
}

func TestXdfileCreateArchiveReplaceKeepsExistingFileUntilArchiveCompletes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	target := filepath.Join(root, "target.zip")
	mustWriteFile(t, source, "new")
	mustWriteFile(t, target, "old")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := xdfileCreateArchiveContext(ctx, []string{source}, target, true, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled archive creation, got %v", err)
	}
	assertFileContent(t, target, "old")

	if _, err := xdfileCreateArchiveContext(context.Background(), []string{source}, target, true, nil); err != nil {
		t.Fatalf("replace archive failed: %v", err)
	}
	entries := readZipEntries(t, target)
	assertArchiveContent(t, entries, "source.txt", "new")
}

func TestXdfileArchiveRejectsTargetInsideSourceDirectory(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "src")
	mustWriteFile(t, filepath.Join(sourceDir, "file.txt"), "content")
	target := filepath.Join(sourceDir, "src.zip")

	if _, err := xdfileCreateArchiveContext(context.Background(), []string{sourceDir}, target, false, nil); err == nil {
		t.Fatal("expected target-inside-source archive to be rejected")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target should not be created after rejection, stat err=%v", err)
	}
}

func TestXdfileUniqueArchiveTargetPreservesTarGZExtension(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "bundle.tar.gz")
	mustWriteFile(t, target, "old")

	renamed, err := xdfileUniqueArchiveTarget(target)
	if err != nil {
		t.Fatalf("unique archive target failed: %v", err)
	}
	expected := filepath.Join(root, "bundle (2).tar.gz")
	if renamed != expected {
		t.Fatalf("unique target = %s, want %s", renamed, expected)
	}
}

func TestXdfileArchiveModalConflictKeepBoth(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "item.txt")
	target := filepath.Join(root, "item.zip")
	mustWriteFile(t, source, "content")
	mustWriteFile(t, target, "old")

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd:         root,
				Cursor:      0,
				RangeAnchor: -1,
				Entries: []xdfileEntry{
					{Name: "item.txt", Path: source},
				},
			},
			{Cwd: root, RangeAnchor: -1},
		},
		modal: xdfileModal{
			Input: xdfileNewModalInput(),
		},
	}

	if cmd := m.openArchiveModal(); cmd != nil {
		t.Fatalf("open archive modal should not return command, got %T", cmd)
	}
	if m.modal.Kind != xdfileModalInput || m.modal.Action != xdfileActionModalArchive {
		t.Fatalf("expected archive input modal, got kind=%v action=%q", m.modal.Kind, m.modal.Action)
	}
	cmd := m.applyModal()
	if cmd != nil {
		t.Fatalf("conflict prompt should wait for user choice, got %T", cmd)
	}
	if m.modal.Action != xdfileActionArchiveConflictPrompt || m.pendingArchive == nil {
		t.Fatalf("expected archive conflict prompt, action=%q pending=%#v", m.modal.Action, m.pendingArchive)
	}

	cmd = m.executeAction(xdfileActionArchiveConflictRename)
	msg := firstArchiveBatchMsg(t, cmd)
	if msg.Err != nil || len(msg.Failures) > 0 {
		t.Fatalf("archive keep-both operation failed: err=%v failures=%#v", msg.Err, msg.Failures)
	}
	keptTarget := filepath.Join(root, "item (2).zip")
	entries := readZipEntries(t, keptTarget)
	assertArchiveContent(t, entries, "item.txt", "content")
	assertFileContent(t, target, "old")
}

func TestXdfileArchiveConflictSkipLeavesTargetUnchanged(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "item.zip")
	mustWriteFile(t, target, "old")
	m := &xdfileModel{
		pendingArchive: &xdfilePendingArchive{
			SourcePaths: []string{filepath.Join(root, "item.txt")},
			TargetPath:  target,
			PanelIndex:  0,
		},
		modal: xdfileModal{
			Action: xdfileActionArchiveConflictPrompt,
			Input:  xdfileNewModalInput(),
		},
	}

	if cmd := m.executeAction(xdfileActionArchiveConflictSkip); cmd != nil {
		t.Fatalf("skip should not start a command, got %T", cmd)
	}
	assertFileContent(t, target, "old")
	if m.pendingArchive != nil {
		t.Fatalf("pending archive should be cleared after skip: %#v", m.pendingArchive)
	}
}

func TestXdfileArchiveModalRequiresSelection(t *testing.T) {
	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Cwd: t.TempDir(), RangeAnchor: -1},
		},
		modal: xdfileModal{Input: xdfileNewModalInput()},
	}
	if cmd := m.openArchiveModal(); cmd != nil {
		t.Fatalf("empty archive selection should not return command, got %T", cmd)
	}
	if m.modal.Kind != xdfileModalNone {
		t.Fatalf("empty archive selection should not open modal, got %v", m.modal.Kind)
	}
	if m.statusText == "" {
		t.Fatal("empty archive selection should set a status message")
	}
}

func TestXdfileExtractZipArchiveSafely(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	writeZipTestArchive(t, archivePath, map[string]string{
		"docs/readme.txt": "hello",
		"plain.txt":       "plain",
	})
	target := filepath.Join(root, "bundle")

	count, err := xdfileExtractArchiveContext(context.Background(), archivePath, target, "", nil)
	if err != nil {
		t.Fatalf("extract zip failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("extract count = %d, want 2", count)
	}
	assertFileContent(t, filepath.Join(target, "docs", "readme.txt"), "hello")
	assertFileContent(t, filepath.Join(target, "plain.txt"), "plain")
}

func TestXdfileExtractTarGZArchiveSafely(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.tar.gz")
	writeTarGZTestArchive(t, archivePath, map[string]string{
		"docs/readme.txt": "hello",
		"plain.txt":       "plain",
	})
	target := filepath.Join(root, "bundle")

	count, err := xdfileExtractArchiveContext(context.Background(), archivePath, target, "", nil)
	if err != nil {
		t.Fatalf("extract tar.gz failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("extract count = %d, want 2", count)
	}
	assertFileContent(t, filepath.Join(target, "docs", "readme.txt"), "hello")
	assertFileContent(t, filepath.Join(target, "plain.txt"), "plain")
}

func TestXdfileExtractRejectsZipPathTraversal(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "evil.zip")
	writeZipTestArchive(t, archivePath, map[string]string{
		"../evil.txt": "owned",
	})
	target := filepath.Join(root, "out")

	if _, err := xdfileExtractArchiveContext(context.Background(), archivePath, target, "", nil); err == nil {
		t.Fatal("expected path traversal zip to be rejected")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target should not be created for malicious archive, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "evil.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("escaped file should not exist, stat err=%v", err)
	}
}

func TestXdfileExtractRejectsAbsoluteZipPath(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "absolute.zip")
	writeZipTestArchive(t, archivePath, map[string]string{
		"/tmp/evil.txt": "owned",
	})
	target := filepath.Join(root, "out")

	if _, err := xdfileExtractArchiveContext(context.Background(), archivePath, target, "", nil); err == nil {
		t.Fatal("expected absolute zip path to be rejected")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target should not be created for malicious archive, stat err=%v", err)
	}
}

func TestXdfileExtractRejectsTarSymlink(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "link.tar.gz")
	writeTarGZSymlinkArchive(t, archivePath)
	target := filepath.Join(root, "out")

	if _, err := xdfileExtractArchiveContext(context.Background(), archivePath, target, "", nil); err == nil {
		t.Fatal("expected tar symlink to be rejected")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target should not be created for symlink archive, stat err=%v", err)
	}
}

func TestXdfileExtractRejectsTarHardlink(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "hardlink.tar.gz")
	writeTarGZHardlinkArchive(t, archivePath)
	target := filepath.Join(root, "out")

	if _, err := xdfileExtractArchiveContext(context.Background(), archivePath, target, "", nil); err == nil {
		t.Fatal("expected tar hardlink to be rejected")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target should not be created for hardlink archive, stat err=%v", err)
	}
}

func TestXdfileExtractReplaceKeepsExistingTargetUntilArchiveCompletes(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	writeZipTestArchive(t, archivePath, map[string]string{"new.txt": "new"})
	target := filepath.Join(root, "bundle")
	mustWriteFile(t, filepath.Join(target, "old.txt"), "old")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := xdfileExtractArchiveContext(ctx, archivePath, target, xdfileActionExtractConflictReplace, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled extraction, got %v", err)
	}
	assertFileContent(t, filepath.Join(target, "old.txt"), "old")

	if _, err := xdfileExtractArchiveContext(context.Background(), archivePath, target, xdfileActionExtractConflictReplace, nil); err != nil {
		t.Fatalf("replace extraction failed: %v", err)
	}
	assertFileContent(t, filepath.Join(target, "new.txt"), "new")
	if _, err := os.Stat(filepath.Join(target, "old.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old target should be replaced, stat err=%v", err)
	}
}

func TestXdfileExtractConflictSkipLeavesTargetUnchanged(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	writeZipTestArchive(t, archivePath, map[string]string{"new.txt": "new"})
	target := filepath.Join(root, "bundle")
	mustWriteFile(t, filepath.Join(target, "old.txt"), "old")

	count, err := xdfileExtractArchiveContext(context.Background(), archivePath, target, xdfileActionExtractConflictSkip, nil)
	if err != nil {
		t.Fatalf("skip extraction should not fail: %v", err)
	}
	if count != 0 {
		t.Fatalf("skip extraction count = %d, want 0", count)
	}
	assertFileContent(t, filepath.Join(target, "old.txt"), "old")
}

func TestXdfileExtractModalConflictKeepBoth(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	writeZipTestArchive(t, archivePath, map[string]string{"new.txt": "new"})
	target := filepath.Join(root, "bundle")
	mustWriteFile(t, filepath.Join(target, "old.txt"), "old")

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd:         root,
				Cursor:      0,
				RangeAnchor: -1,
				Entries: []xdfileEntry{
					{Name: "bundle.zip", Path: archivePath},
				},
			},
			{Cwd: root, RangeAnchor: -1},
		},
		modal: xdfileModal{Input: xdfileNewModalInput()},
	}

	if cmd := m.openExtractArchiveModal(); cmd != nil {
		t.Fatalf("open extract modal should not return command, got %T", cmd)
	}
	if m.modal.Kind != xdfileModalInput || m.modal.Action != xdfileActionModalExtract {
		t.Fatalf("expected extract input modal, got kind=%v action=%q", m.modal.Kind, m.modal.Action)
	}
	cmd := m.applyModal()
	if cmd != nil {
		t.Fatalf("conflict prompt should wait for user choice, got %T", cmd)
	}
	if m.modal.Action != xdfileActionExtractConflictPrompt || m.pendingExtract == nil {
		t.Fatalf("expected extract conflict prompt, action=%q pending=%#v", m.modal.Action, m.pendingExtract)
	}

	msg := firstArchiveBatchMsg(t, m.executeAction(xdfileActionExtractConflictRename))
	if msg.Err != nil || len(msg.Failures) > 0 {
		t.Fatalf("extract keep-both operation failed: err=%v failures=%#v", msg.Err, msg.Failures)
	}
	assertFileContent(t, filepath.Join(root, "bundle (2)", "new.txt"), "new")
	assertFileContent(t, filepath.Join(target, "old.txt"), "old")
}

func firstArchiveBatchMsg(t *testing.T, cmd tea.Cmd) xdfileFileOperationDoneMsg {
	t.Helper()
	return firstBatchMsg[xdfileFileOperationDoneMsg](t, cmd)
}

func readZipEntries(t *testing.T, path string) map[string]string {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip %s: %v", path, err)
	}
	defer reader.Close()

	entries := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			entries[file.Name] = ""
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", file.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s: %v", file.Name, err)
		}
		entries[file.Name] = string(data)
	}
	return entries
}

func readTarGZEntries(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open tar.gz %s: %v", path, err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("open gzip %s: %v", path, err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	entries := make(map[string]string)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read tar entry: %v", err)
		}
		if header.FileInfo().IsDir() {
			entries[header.Name] = ""
			continue
		}
		data, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("read tar entry %s: %v", header.Name, err)
		}
		entries[header.Name] = string(data)
	}
	return entries
}

func assertArchiveContent(t *testing.T, entries map[string]string, name string, expected string) {
	t.Helper()
	got, ok := entries[name]
	if !ok {
		t.Fatalf("missing archive entry %s in %#v", name, sortedArchiveNames(entries))
	}
	if got != expected {
		t.Fatalf("archive entry %s = %q, want %q", name, got, expected)
	}
}

func sortedArchiveNames(entries map[string]string) []string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func writeZipTestArchive(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir zip parent: %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip %s: %v", path, err)
	}
	writer := zip.NewWriter(file)
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file %s: %v", path, err)
	}
}

func writeTarGZTestArchive(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir tar parent: %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tar.gz %s: %v", path, err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range entries {
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header %s: %v", name, err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("write tar entry %s: %v", name, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar %s: %v", path, err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close tar.gz file %s: %v", path, err)
	}
}

func writeTarGZSymlinkArchive(t *testing.T, path string) {
	t.Helper()
	writeTarGZLinkArchive(t, path, tar.TypeSymlink)
}

func writeTarGZHardlinkArchive(t *testing.T, path string) {
	t.Helper()
	writeTarGZLinkArchive(t, path, tar.TypeLink)
}

func writeTarGZLinkArchive(t *testing.T, path string, typeflag byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir tar parent: %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tar.gz %s: %v", path, err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	header := &tar.Header{
		Name:     "link",
		Typeflag: typeflag,
		Linkname: "target",
		Mode:     0o777,
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("write symlink header: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar %s: %v", path, err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close tar.gz file %s: %v", path, err)
	}
}
