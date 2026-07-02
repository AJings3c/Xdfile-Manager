package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXdfileManagedShellHelpListsNewCommands(t *testing.T) {
	dir := t.TempDir()

	result, handled := xdfileRunManagedShellCommand(dir, "help")

	if !handled {
		t.Fatal("expected help to be handled")
	}
	if result.Err != nil {
		t.Fatalf("help returned error: %v", result.Err)
	}
	for _, expected := range []string{"preview [path]", "tree [path] [depth]", "grep [-i] <text> <file>"} {
		if !strings.Contains(result.Output, expected) {
			t.Fatalf("help output missing %q:\n%s", expected, result.Output)
		}
	}
}

func TestXdfileManagedShellStatAndTree(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(nested, "needle.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	statResult, statHandled := xdfileRunManagedShellCommand(dir, "stat nested/needle.txt")
	if !statHandled {
		t.Fatal("expected stat to be handled")
	}
	if statResult.Err != nil {
		t.Fatalf("stat returned error: %v", statResult.Err)
	}
	if !strings.Contains(statResult.Output, "needle.txt") || !strings.Contains(statResult.Output, "5B") {
		t.Fatalf("unexpected stat output:\n%s", statResult.Output)
	}

	treeResult, treeHandled := xdfileRunManagedShellCommand(dir, "tree . 2")
	if !treeHandled {
		t.Fatal("expected tree to be handled")
	}
	if treeResult.Err != nil {
		t.Fatalf("tree returned error: %v", treeResult.Err)
	}
	for _, expected := range []string{"nested", "needle.txt"} {
		if !strings.Contains(treeResult.Output, expected) {
			t.Fatalf("tree output missing %q:\n%s", expected, treeResult.Output)
		}
	}
}

func TestXdfileManagedShellFindGrepHeadTail(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(nested, "notes.txt")
	content := "alpha\nneedle one\nbeta\nNeedle two\nomega\n"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	findResult, findHandled := xdfileRunManagedShellCommand(dir, "find notes .")
	if !findHandled {
		t.Fatal("expected find to be handled")
	}
	if findResult.Err != nil {
		t.Fatalf("find returned error: %v", findResult.Err)
	}
	if !strings.Contains(findResult.Output, filepath.Join("nested", "notes.txt")) {
		t.Fatalf("unexpected find output:\n%s", findResult.Output)
	}

	grepResult, grepHandled := xdfileRunManagedShellCommand(dir, "grep -i needle nested/notes.txt")
	if !grepHandled {
		t.Fatal("expected grep to be handled")
	}
	if grepResult.Err != nil {
		t.Fatalf("grep returned error: %v", grepResult.Err)
	}
	if !strings.Contains(grepResult.Output, "needle one") || !strings.Contains(grepResult.Output, "Needle two") {
		t.Fatalf("unexpected grep output:\n%s", grepResult.Output)
	}

	headResult, headHandled := xdfileRunManagedShellCommand(dir, "head -n 2 nested/notes.txt")
	if !headHandled {
		t.Fatal("expected head to be handled")
	}
	if headResult.Err != nil {
		t.Fatalf("head returned error: %v", headResult.Err)
	}
	if !strings.Contains(headResult.Output, "alpha\nneedle one") || strings.Contains(headResult.Output, "omega") {
		t.Fatalf("unexpected head output:\n%s", headResult.Output)
	}

	tailResult, tailHandled := xdfileRunManagedShellCommand(dir, "tail nested/notes.txt 2")
	if !tailHandled {
		t.Fatal("expected tail to be handled")
	}
	if tailResult.Err != nil {
		t.Fatalf("tail returned error: %v", tailResult.Err)
	}
	if !strings.Contains(tailResult.Output, "Needle two\nomega") || strings.Contains(tailResult.Output, "alpha") {
		t.Fatalf("unexpected tail output:\n%s", tailResult.Output)
	}
}

func TestXdfileManagedShellPathSuggestionsRespectArgumentPosition(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, input := range []string{"grep ", "grep -i ", "find ", "search ", "head -n ", "head alpha.txt "} {
		if suggestions := xdfileManagedShellPathSuggestions(input, dir); len(suggestions) != 0 {
			t.Fatalf("expected no path suggestions for %q, got %#v", input, suggestions)
		}
	}

	for _, input := range []string{"grep needle ", "grep -i needle ", "find alpha ", "search alpha ", "head -n 2 "} {
		suggestions := xdfileManagedShellPathSuggestions(input, dir)
		if !xdfileTestSuggestionsContain(suggestions, "alpha.txt") {
			t.Fatalf("expected alpha.txt suggestion for %q, got %#v", input, suggestions)
		}
	}
}

func xdfileTestSuggestionsContain(suggestions []string, value string) bool {
	for _, suggestion := range suggestions {
		if strings.Contains(suggestion, value) {
			return true
		}
	}
	return false
}
