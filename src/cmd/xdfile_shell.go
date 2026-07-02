package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type xdfileShellCommand struct {
	Raw  string
	Name string
	Args []string
}

var xdfileShellAliasMap = map[string][]string{
	"ls":     {"dir"},
	"ll":     {"dir"},
	"la":     {"dir", "/a"},
	"cat":    {"type"},
	"info":   {"stat"},
	"search": {"find"},
	"view":   {"preview"},
	"where":  {"which"},
}

type xdfileManagedShellCommandSpec struct {
	Name    string
	Usage   string
	Summary string
}

var xdfileManagedShellCommandSpecs = []xdfileManagedShellCommandSpec{
	{Name: "help", Usage: "help [command]", Summary: "show managed TUI command help"},
	{Name: "pwd", Usage: "pwd", Summary: "print the current panel path"},
	{Name: "cd", Usage: "cd <dir>", Summary: "change directory and sync the active panel"},
	{Name: "dir", Usage: "dir [path]", Summary: "list files; aliases: ls, ll, la"},
	{Name: "type", Usage: "type <file>", Summary: "print a file with lightweight highlighting; alias: cat"},
	{Name: "preview", Usage: "preview [path]", Summary: "render the same preview used by the side preview pane"},
	{Name: "stat", Usage: "stat [path]", Summary: "show file metadata; alias: info"},
	{Name: "tree", Usage: "tree [path] [depth]", Summary: "show a compact directory tree"},
	{Name: "find", Usage: "find <name> [path]", Summary: "search file and directory names under a path; alias: search"},
	{Name: "grep", Usage: "grep [-i] <text> <file>", Summary: "search text inside a file"},
	{Name: "head", Usage: "head [-n N] <file>", Summary: "show the first lines of a text file"},
	{Name: "tail", Usage: "tail [-n N] <file>", Summary: "show the last lines of a text file"},
	{Name: "which", Usage: "which <command>", Summary: "locate an executable; alias: where"},
	{Name: "open", Usage: "open <path>", Summary: "open a local file or directory with the OS"},
}

const (
	xdfileManagedShellTextReadLimit  = 2 * 1024 * 1024
	xdfileManagedShellLineDefault    = 20
	xdfileManagedShellLineMax        = 200
	xdfileManagedShellTreeDepth      = 2
	xdfileManagedShellTreeMaxDepth   = 5
	xdfileManagedShellTreeMaxEntries = 160
	xdfileManagedShellFindMaxMatches = 100
	xdfileManagedShellFindMaxVisited = 5000
	xdfileManagedShellGrepMaxMatches = 100
)

func xdfileRunManagedShellCommand(dir string, command string) (xdfileTerminalResultMsg, bool) {
	command = strings.TrimSpace(command)
	result := xdfileTerminalResultMsg{
		Command: command,
		Dir:     dir,
	}
	if command == "" || xdfileContainsShellOperators(command) {
		return result, false
	}

	parsed, err := xdfileParseShellCommand(command)
	if err != nil || parsed.Name == "" {
		return result, false
	}
	resolved := xdfileApplyShellAlias(parsed)

	switch strings.ToLower(resolved.Name) {
	case "pwd":
		result.Output = xdfileTerminalPromptPathStyle.Render(dir)
		return result, true
	case "echo":
		result.Output = strings.Join(resolved.Args, " ")
		return result, true
	case "help", "commands":
		return xdfileRunManagedHelpCommand(dir, parsed, resolved)
	case "cd", "chdir":
		nextDir, handled, cdErr := xdfileBuiltinCD(dir, command)
		if !handled {
			return result, false
		}
		result.Dir = nextDir
		result.Err = cdErr
		result.SyncActivePanel = cdErr == nil
		if cdErr == nil {
			result.Output = xdfileTerminalPromptPathStyle.Render(nextDir)
		}
		return result, true
	case "dir":
		return xdfileRunManagedDirCommand(dir, parsed, resolved)
	case "type":
		return xdfileRunManagedTypeCommand(dir, parsed, resolved)
	case "preview":
		return xdfileRunManagedPreviewCommand(dir, parsed, resolved)
	case "stat":
		return xdfileRunManagedStatCommand(dir, parsed, resolved)
	case "tree":
		return xdfileRunManagedTreeCommand(dir, parsed, resolved)
	case "find":
		return xdfileRunManagedFindCommand(dir, parsed, resolved)
	case "grep":
		return xdfileRunManagedGrepCommand(dir, parsed, resolved)
	case "head", "tail":
		return xdfileRunManagedLineCommand(dir, parsed, resolved)
	case "which":
		return xdfileRunManagedWhichCommand(dir, parsed, resolved)
	case "open":
		return xdfileRunManagedOpenCommand(dir, parsed, resolved)
	default:
		return result, false
	}
}

func xdfileRunManagedDirCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}

	showHidden := false
	target := dir
	pathArgs := 0
	for _, arg := range resolved.Args {
		switch strings.ToLower(arg) {
		case "/a", "-a":
			showHidden = true
		default:
			pathArgs++
			if pathArgs > 1 {
				return result, false
			}
			resolvedPath, err := xdfileResolveShellPath(dir, arg)
			if err != nil {
				result.Err = err
				return result, true
			}
			target = resolvedPath
		}
	}

	entries, err := xdfileReadEntries(target, showHidden, xdfileSortModeName)
	if err != nil {
		result.Err = err
		return result, true
	}

	longForm := strings.EqualFold(parsed.Name, "ll")
	lines := make([]string, 0, len(entries)+2)
	lines = append(lines, xdfileTagStyle.Render("Directory")+" "+xdfileTerminalPromptPathStyle.Render(target))
	lines = append(lines, xdfileDimStyle.Render(xdfileManagedDirSummary(entries)))
	for _, entry := range entries {
		lines = append(lines, xdfileRenderManagedDirEntry(entry, longForm))
	}
	result.Output = strings.Join(lines, "\n")
	return result, true
}

func xdfileRunManagedTypeCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}
	if len(resolved.Args) != 1 {
		return result, false
	}

	path, err := xdfileResolveShellPath(dir, resolved.Args[0])
	if err != nil {
		result.Err = err
		return result, true
	}
	if xdfileIsNetBoxPath(path) {
		result.Err = fmt.Errorf("remote file output is unavailable in the managed shell")
		return result, true
	}
	info, statErr := os.Stat(path)
	if statErr != nil {
		result.Err = statErr
		return result, true
	}
	if info.IsDir() {
		result.Err = fmt.Errorf("not a file: %s", path)
		return result, true
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		result.Err = readErr
		return result, true
	}
	content := strings.TrimRight(strings.ReplaceAll(xdfileDecodeCommandOutput(data), "\r\n", "\n"), "\n")
	result.Output = xdfileHighlightManagedShellText(path, content)
	return result, true
}

func xdfileRunManagedHelpCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}
	if len(resolved.Args) > 1 {
		result.Err = xdfileManagedShellUsageError("help [command]")
		return result, true
	}
	if len(resolved.Args) == 1 {
		query := strings.ToLower(resolved.Args[0])
		if alias, ok := xdfileShellAliasMap[query]; ok && len(alias) > 0 {
			query = strings.ToLower(alias[0])
		}
		for _, spec := range xdfileManagedShellCommandSpecs {
			if spec.Name == query {
				result.Output = strings.Join([]string{
					xdfileTagStyle.Render("Usage") + " " + xdfileTerminalPromptPathStyle.Render(spec.Usage),
					xdfileDimStyle.Render(spec.Summary),
				}, "\n")
				return result, true
			}
		}
		result.Err = fmt.Errorf("unknown managed command: %s", resolved.Args[0])
		return result, true
	}

	lines := []string{xdfileTagStyle.Render("Managed commands")}
	for _, spec := range xdfileManagedShellCommandSpecs {
		lines = append(lines, fmt.Sprintf("  %-24s %s", xdfileTerminalPromptPathStyle.Render(spec.Usage), xdfileDimStyle.Render(spec.Summary)))
	}
	result.Output = strings.Join(lines, "\n")
	return result, true
}

func xdfileRunManagedPreviewCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}
	if len(resolved.Args) > 1 || (len(resolved.Args) == 1 && strings.HasPrefix(resolved.Args[0], "-")) {
		return result, false
	}

	targetArg := ""
	if len(resolved.Args) == 1 {
		targetArg = resolved.Args[0]
	}
	target, err := xdfileResolveManagedLocalShellPath(dir, targetArg)
	if err != nil {
		result.Err = err
		return result, true
	}
	preview, err := xdfileReadPreview(target)
	if err != nil {
		result.Err = err
		return result, true
	}
	result.Output = preview
	return result, true
}

func xdfileRunManagedStatCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}
	if len(resolved.Args) > 1 || (len(resolved.Args) == 1 && strings.HasPrefix(resolved.Args[0], "-")) {
		return result, false
	}

	targetArg := ""
	if len(resolved.Args) == 1 {
		targetArg = resolved.Args[0]
	}
	target, err := xdfileResolveManagedLocalShellPath(dir, targetArg)
	if err != nil {
		result.Err = err
		return result, true
	}
	info, err := os.Lstat(target)
	if err != nil {
		result.Err = err
		return result, true
	}

	lines := []string{
		xdfilePreviewKeyValue("Path", target),
		xdfilePreviewTypeLine(target, xdfileManagedShellFileKind(info)),
		xdfilePreviewKeyValue("Size", xdfileHumanSize(info.Size())),
		xdfilePreviewKeyValue("Modified", info.ModTime().Format("2006-01-02 15:04:05")),
		xdfilePreviewKeyValue("Mode", info.Mode().String()),
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if linkTarget, readErr := os.Readlink(target); readErr == nil {
			lines = append(lines, xdfilePreviewKeyValue("Link", linkTarget))
		}
	}
	if info.IsDir() {
		if children, readErr := os.ReadDir(target); readErr == nil {
			lines = append(lines, xdfilePreviewKeyValue("Children", strconv.Itoa(len(children))))
		}
	}
	result.Output = strings.Join(lines, "\n")
	return result, true
}

func xdfileRunManagedTreeCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}
	targetArg, depth, handled, err := xdfileParseManagedTreeArgs(resolved.Args)
	if !handled {
		return result, false
	}
	if err != nil {
		result.Err = err
		return result, true
	}

	target, err := xdfileResolveManagedLocalShellPath(dir, targetArg)
	if err != nil {
		result.Err = err
		return result, true
	}
	info, err := os.Stat(target)
	if err != nil {
		result.Err = err
		return result, true
	}
	if !info.IsDir() {
		result.Output = xdfilePreviewListEntry(filepath.Base(target), false)
		return result, true
	}

	lines := []string{xdfileTagStyle.Render("Tree") + " " + xdfileTerminalPromptPathStyle.Render(target)}
	count := 0
	truncated, walkErr := xdfileAppendManagedTreeLines(&lines, target, "", 0, depth, &count)
	if walkErr != nil {
		result.Err = walkErr
		return result, true
	}
	lines = append(lines, "")
	if truncated {
		lines = append(lines, xdfileDimStyle.Render(fmt.Sprintf("shown %d entries; output truncated", count)))
	} else {
		lines = append(lines, xdfileDimStyle.Render(fmt.Sprintf("%d entries", count)))
	}
	result.Output = strings.Join(lines, "\n")
	return result, true
}

func xdfileRunManagedFindCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}
	if len(resolved.Args) < 1 || len(resolved.Args) > 2 || strings.HasPrefix(resolved.Args[0], "-") {
		return result, false
	}

	targetArg := ""
	if len(resolved.Args) == 2 {
		targetArg = resolved.Args[1]
		if strings.HasPrefix(targetArg, "-") {
			return result, false
		}
	}
	target, err := xdfileResolveManagedLocalShellPath(dir, targetArg)
	if err != nil {
		result.Err = err
		return result, true
	}
	matches, visited, truncated, err := xdfileFindManagedShellPathNames(target, resolved.Args[0])
	if err != nil {
		result.Err = err
		return result, true
	}

	lines := []string{
		xdfileTagStyle.Render("Find") + " " + xdfileTerminalPromptPathStyle.Render(resolved.Args[0]) + " " + xdfileDimStyle.Render("in") + " " + xdfileTerminalPromptPathStyle.Render(target),
	}
	if len(matches) == 0 {
		lines = append(lines, xdfileDimStyle.Render(fmt.Sprintf("No matches in %d visited entries", visited)))
	} else {
		lines = append(lines, matches...)
		lines = append(lines, "")
		if truncated {
			lines = append(lines, xdfileDimStyle.Render(fmt.Sprintf("shown %d matches; stopped after %d visited entries", len(matches), visited)))
		} else {
			lines = append(lines, xdfileDimStyle.Render(fmt.Sprintf("%d matches in %d visited entries", len(matches), visited)))
		}
	}
	result.Output = strings.Join(lines, "\n")
	return result, true
}

func xdfileRunManagedGrepCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}
	pattern, pathArg, ignoreCase, handled := xdfileParseManagedGrepArgs(resolved.Args)
	if !handled {
		return result, false
	}

	target, err := xdfileResolveManagedLocalShellPath(dir, pathArg)
	if err != nil {
		result.Err = err
		return result, true
	}
	text, err := xdfileReadManagedShellTextFile(target)
	if err != nil {
		result.Err = err
		return result, true
	}

	lines := []string{
		xdfileTagStyle.Render("Grep") + " " + xdfileTerminalPromptPathStyle.Render(pattern) + " " + xdfileDimStyle.Render("in") + " " + xdfileTerminalPromptPathStyle.Render(target),
	}
	matches := 0
	truncated := false
	needle := pattern
	if ignoreCase {
		needle = strings.ToLower(needle)
	}
	for index, line := range xdfileSplitManagedShellTextLines(text) {
		haystack := line
		if ignoreCase {
			haystack = strings.ToLower(haystack)
		}
		if !strings.Contains(haystack, needle) {
			continue
		}
		if matches >= xdfileManagedShellGrepMaxMatches {
			truncated = true
			break
		}
		lineNumber := xdfileMetaStyle.Render(fmt.Sprintf("%5d:", index+1))
		lines = append(lines, lineNumber+" "+xdfileHighlightManagedShellText(target, line))
		matches++
	}
	if matches == 0 {
		lines = append(lines, xdfileDimStyle.Render("No matches"))
	} else if truncated {
		lines = append(lines, "", xdfileDimStyle.Render(fmt.Sprintf("shown %d matches; output truncated", matches)))
	} else {
		lines = append(lines, "", xdfileDimStyle.Render(fmt.Sprintf("%d matches", matches)))
	}
	result.Output = strings.Join(lines, "\n")
	return result, true
}

func xdfileRunManagedLineCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}
	pathArg, count, handled, err := xdfileParseManagedLineArgs(resolved.Args)
	if !handled {
		return result, false
	}
	if err != nil {
		result.Err = err
		return result, true
	}

	target, err := xdfileResolveManagedLocalShellPath(dir, pathArg)
	if err != nil {
		result.Err = err
		return result, true
	}
	text, err := xdfileReadManagedShellTextFile(target)
	if err != nil {
		result.Err = err
		return result, true
	}
	lines := xdfileSplitManagedShellTextLines(text)
	if len(lines) > count {
		if strings.EqualFold(resolved.Name, "tail") {
			lines = lines[len(lines)-count:]
		} else {
			lines = lines[:count]
		}
	}
	result.Output = xdfileHighlightManagedShellText(target, strings.Join(lines, "\n"))
	return result, true
}

func xdfileRunManagedWhichCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}
	if len(resolved.Args) != 1 || strings.HasPrefix(resolved.Args[0], "-") {
		return result, false
	}
	path, ok := xdfileResolveExternalExecutablePath(dir, resolved.Args[0])
	if !ok {
		result.Err = fmt.Errorf("command not found: %s", resolved.Args[0])
		return result, true
	}
	result.Output = xdfileTerminalPromptPathStyle.Render(path)
	return result, true
}

func xdfileRunManagedOpenCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}
	if len(resolved.Args) != 1 || strings.HasPrefix(resolved.Args[0], "-") {
		return result, false
	}
	target, err := xdfileResolveManagedLocalShellPath(dir, resolved.Args[0])
	if err != nil {
		result.Err = err
		return result, true
	}
	if err := xdfileOpenPathFunc(target); err != nil {
		result.Err = err
		return result, true
	}
	result.Output = "Opened " + xdfileTerminalPromptPathStyle.Render(target)
	return result, true
}

func xdfileResolveManagedLocalShellPath(dir string, value string) (string, error) {
	target, err := xdfileResolveShellPath(dir, value)
	if err != nil {
		return "", err
	}
	if xdfileIsNetBoxPath(target) {
		return "", fmt.Errorf("remote paths are unavailable in this managed command")
	}
	return target, nil
}

func xdfileManagedShellUsageError(usage string) error {
	return fmt.Errorf("usage: %s", usage)
}

func xdfileManagedShellFileKind(info os.FileInfo) string {
	mode := info.Mode()
	switch {
	case mode&os.ModeSymlink != 0:
		return "Symlink"
	case mode&os.ModeSocket != 0:
		return "Socket"
	case mode&os.ModeNamedPipe != 0:
		return "Named pipe"
	case mode&os.ModeDevice != 0:
		return "Device"
	case info.IsDir():
		return "Directory"
	default:
		return "File"
	}
}

func xdfileParseManagedTreeArgs(args []string) (string, int, bool, error) {
	target := ""
	depth := xdfileManagedShellTreeDepth
	depthSet := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		lower := strings.ToLower(arg)
		switch {
		case lower == "-d" || lower == "--depth":
			if i+1 >= len(args) {
				return "", 0, true, xdfileManagedShellUsageError("tree [path] [depth]")
			}
			parsed, err := xdfileParseManagedShellDepth(args[i+1])
			if err != nil {
				return "", 0, true, err
			}
			depth = parsed
			depthSet = true
			i++
		case strings.HasPrefix(lower, "-d="):
			parsed, err := xdfileParseManagedShellDepth(arg[3:])
			if err != nil {
				return "", 0, true, err
			}
			depth = parsed
			depthSet = true
		case strings.HasPrefix(lower, "--depth="):
			parsed, err := xdfileParseManagedShellDepth(arg[len("--depth="):])
			if err != nil {
				return "", 0, true, err
			}
			depth = parsed
			depthSet = true
		case strings.HasPrefix(arg, "-"):
			return "", 0, false, nil
		case target == "":
			target = arg
		case !depthSet:
			parsed, err := xdfileParseManagedShellDepth(arg)
			if err != nil {
				return "", 0, true, err
			}
			depth = parsed
			depthSet = true
		default:
			return "", 0, false, nil
		}
	}

	return target, depth, true, nil
}

func xdfileParseManagedShellDepth(value string) (int, error) {
	depth, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || depth < 0 || depth > xdfileManagedShellTreeMaxDepth {
		return 0, fmt.Errorf("depth must be between 0 and %d", xdfileManagedShellTreeMaxDepth)
	}
	return depth, nil
}

func xdfileAppendManagedTreeLines(lines *[]string, dir string, prefix string, level int, maxDepth int, count *int) (bool, error) {
	if level >= maxDepth {
		return false, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		*lines = append(*lines, prefix+xdfileDimStyle.Render("[error] "+err.Error()))
		return false, nil
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		if left.IsDir() != right.IsDir() {
			return left.IsDir()
		}
		return strings.ToLower(left.Name()) < strings.ToLower(right.Name())
	})

	for index, entry := range entries {
		if *count >= xdfileManagedShellTreeMaxEntries {
			return true, nil
		}
		isLast := index == len(entries)-1
		connector := "|-- "
		nextPrefix := prefix + "|   "
		if isLast {
			connector = "`-- "
			nextPrefix = prefix + "    "
		}
		name := xdfilePreviewListEntry(entry.Name(), entry.IsDir())
		if entry.IsDir() {
			name += xdfileDimStyle.Render(string(os.PathSeparator))
		}
		*lines = append(*lines, prefix+connector+name)
		*count++

		if entry.IsDir() {
			truncated, walkErr := xdfileAppendManagedTreeLines(lines, filepath.Join(dir, entry.Name()), nextPrefix, level+1, maxDepth, count)
			if walkErr != nil || truncated {
				return truncated, walkErr
			}
		}
	}
	return false, nil
}

func xdfileFindManagedShellPathNames(root string, pattern string) ([]string, int, bool, error) {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if pattern == "" {
		return nil, 0, false, xdfileManagedShellUsageError("find <name> [path]")
	}

	matches := make([]string, 0, min(16, xdfileManagedShellFindMaxMatches))
	visited := 0
	truncated := false
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		visited++
		if visited > xdfileManagedShellFindMaxVisited {
			truncated = true
			return fs.SkipAll
		}
		if current == root && entry.IsDir() {
			return nil
		}
		if strings.Contains(strings.ToLower(entry.Name()), pattern) {
			matches = append(matches, xdfileRenderManagedFindMatch(root, current, entry.IsDir()))
			if len(matches) >= xdfileManagedShellFindMaxMatches {
				truncated = true
				return fs.SkipAll
			}
		}
		return nil
	})
	return matches, visited, truncated, err
}

func xdfileRenderManagedFindMatch(root string, current string, isDir bool) string {
	label := current
	if rel, err := filepath.Rel(root, current); err == nil && rel != "." {
		label = rel
	} else {
		label = filepath.Base(current)
	}
	if isDir {
		label += string(os.PathSeparator)
	}
	return xdfilePreviewListEntry(label, isDir)
}

func xdfileParseManagedGrepArgs(args []string) (string, string, bool, bool) {
	ignoreCase := false
	if len(args) == 3 && strings.EqualFold(args[0], "-i") {
		ignoreCase = true
		args = args[1:]
	}
	if len(args) != 2 || strings.HasPrefix(args[0], "-") || strings.HasPrefix(args[1], "-") {
		return "", "", false, false
	}
	return args[0], args[1], ignoreCase, true
}

func xdfileParseManagedLineArgs(args []string) (string, int, bool, error) {
	target := ""
	count := xdfileManagedShellLineDefault
	countSet := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		lower := strings.ToLower(arg)
		switch {
		case lower == "-n":
			if i+1 >= len(args) {
				return "", 0, true, xdfileManagedShellUsageError("head [-n N] <file>")
			}
			parsed, err := xdfileParseManagedShellLineCount(args[i+1])
			if err != nil {
				return "", 0, true, err
			}
			count = parsed
			countSet = true
			i++
		case strings.HasPrefix(lower, "-n="):
			parsed, err := xdfileParseManagedShellLineCount(arg[3:])
			if err != nil {
				return "", 0, true, err
			}
			count = parsed
			countSet = true
		case strings.HasPrefix(arg, "-"):
			return "", 0, false, nil
		case target == "":
			target = arg
		case !countSet:
			parsed, err := xdfileParseManagedShellLineCount(arg)
			if err != nil {
				return "", 0, true, err
			}
			count = parsed
			countSet = true
		default:
			return "", 0, false, nil
		}
	}
	if target == "" {
		return "", 0, false, nil
	}
	return target, count, true, nil
}

func xdfileParseManagedShellLineCount(value string) (int, error) {
	count, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || count < 1 || count > xdfileManagedShellLineMax {
		return 0, fmt.Errorf("line count must be between 1 and %d", xdfileManagedShellLineMax)
	}
	return count, nil
}

func xdfileReadManagedShellTextFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("not a file: %s", path)
	}
	if info.Size() > xdfileManagedShellTextReadLimit {
		return "", fmt.Errorf("file is larger than managed text limit (%s): %s", xdfileHumanSize(xdfileManagedShellTextReadLimit), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := strings.ReplaceAll(xdfileDecodeCommandOutput(data), "\r\n", "\n")
	return strings.TrimRight(strings.ReplaceAll(text, "\r", "\n"), "\n"), nil
}

func xdfileSplitManagedShellTextLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func xdfileManagedDirSummary(entries []xdfileEntry) string {
	dirs := 0
	files := 0
	for _, entry := range entries {
		switch {
		case entry.IsParent:
			continue
		case entry.IsDir:
			dirs++
		default:
			files++
		}
	}
	return fmt.Sprintf("%d items | %d dirs | %d files", dirs+files, dirs, files)
}

func xdfileRenderManagedDirEntry(entry xdfileEntry, longForm bool) string {
	kind := xdfileEntryKindSpecForEntry(entry)

	name := entry.Name
	nameStyle := xdfileEntryNameStyle(entry)
	if entry.IsDir && !entry.IsParent {
		separator := string(os.PathSeparator)
		if xdfileIsNetBoxPath(entry.Path) {
			separator = "/"
		}
		name = nameStyle.Render(name) + xdfileDimStyle.Render(separator)
	} else {
		name = nameStyle.Render(name)
	}

	if !longForm {
		return kind.render() + xdfileDimStyle.Render("  ") + name
	}

	size := "-"
	if !entry.IsDir && !entry.IsParent {
		size = xdfileHumanSize(entry.Size)
	}
	modified := "-"
	if !entry.IsParent {
		modified = entry.Modified.Format("2006-01-02 15:04")
	}
	return kind.render() + " " +
		xdfileMetaStyle.Render(fmt.Sprintf("%7s", size)) + " " +
		xdfileDimStyle.Render(modified) + " " +
		name
}

func xdfileResolveShellPath(cwd string, value string) (string, error) {
	value = strings.TrimSpace(strings.Trim(value, `"'`))
	if xdfileIsNetBoxPath(cwd) {
		return xdfileResolveRemoteShellPath(cwd, value)
	}
	if value == "" {
		return cwd, nil
	}
	if strings.HasPrefix(value, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		value = filepath.Join(home, strings.TrimPrefix(value, "~"))
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(cwd, value)
	}
	return filepath.Clean(value), nil
}

func xdfileResolveRemoteShellPath(cwd string, value string) (string, error) {
	remote, ok := xdfileParseNetBoxPath(cwd)
	if !ok {
		return "", fmt.Errorf("invalid SSH panel path: %s", cwd)
	}
	if value == "" || value == "." {
		return xdfileNetBoxURL(remote.Profile, remote.Path), nil
	}
	if xdfileIsNetBoxPath(value) {
		return value, nil
	}

	value = strings.ReplaceAll(value, `\`, "/")
	if strings.HasPrefix(value, "~") {
		switch {
		case value == "~":
			value = "/"
		case strings.HasPrefix(value, "~/"):
			value = "/" + strings.TrimPrefix(value, "~/")
		default:
			return "", fmt.Errorf("unsupported remote home path: %s", value)
		}
	}
	if strings.HasPrefix(value, "/") {
		return xdfileNetBoxURL(remote.Profile, value), nil
	}
	return xdfileNetBoxURL(remote.Profile, path.Join(remote.Path, value)), nil
}

func xdfileParseShellCommand(command string) (xdfileShellCommand, error) {
	fields, err := xdfileSplitShellWords(command)
	if err != nil {
		return xdfileShellCommand{}, err
	}
	if len(fields) == 0 {
		return xdfileShellCommand{Raw: strings.TrimSpace(command)}, nil
	}
	return xdfileShellCommand{
		Raw:  strings.TrimSpace(command),
		Name: fields[0],
		Args: fields[1:],
	}, nil
}

func xdfileSplitShellWords(command string) ([]string, error) {
	var (
		fields []string
		token  strings.Builder
		quote  rune
	)

	flush := func() {
		if token.Len() == 0 {
			return
		}
		fields = append(fields, token.String())
		token.Reset()
	}

	for _, r := range command {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			token.WriteRune(r)
		case r == '"' || r == '\'':
			quote = r
		case unicode.IsSpace(r):
			flush()
		default:
			token.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	return fields, nil
}

func xdfileApplyShellAlias(command xdfileShellCommand) xdfileShellCommand {
	alias, ok := xdfileShellAliasMap[strings.ToLower(command.Name)]
	if !ok || len(alias) == 0 {
		return command
	}
	merged := append(append([]string{}, alias...), command.Args...)
	return xdfileShellCommand{
		Raw:  command.Raw,
		Name: merged[0],
		Args: merged[1:],
	}
}

func xdfileContainsShellOperators(command string) bool {
	var quote rune
	for _, r := range command {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case strings.ContainsRune("|&<>", r):
			return true
		}
	}
	return false
}

const xdfileManagedShellPathSuggestionLimit = 20

type xdfileShellPathSuggestionMode int

const (
	xdfileShellPathSuggestionAny xdfileShellPathSuggestionMode = iota
	xdfileShellPathSuggestionDirs
	xdfileShellPathSuggestionFiles
)

type xdfileManagedShellPathSuggestionMatch struct {
	Value string
	Name  string
	IsDir bool
}

func xdfileManagedShellPathSuggestions(input string, cwd string) []string {
	if xdfileIsNetBoxPath(cwd) {
		return nil
	}

	fields, err := xdfileSplitShellWords(input)
	if err != nil || len(fields) == 0 {
		return nil
	}

	commandName := strings.ToLower(fields[0])
	mode, expectsPath := xdfileShellCommandPathSuggestionMode(commandName)
	trailingSpace := strings.TrimRight(input, " \t") != input
	if !xdfileShellCommandShouldSuggestPath(commandName, fields, trailingSpace, expectsPath) {
		return nil
	}

	lastSpace := strings.LastIndexAny(input, " \t")
	if lastSpace < 0 {
		return nil
	}

	base := input[:lastSpace+1]
	partial := strings.TrimSpace(input[lastSpace+1:])

	quoted := false
	quoteChar := byte(0)
	if partial != "" && (partial[0] == '"' || partial[0] == '\'') {
		quoted = true
		quoteChar = partial[0]
		partial = strings.TrimPrefix(partial, string(quoteChar))
	}

	searchDir := cwd
	namePrefix := partial
	if partial != "" {
		lookup, err := xdfileResolveShellPath(cwd, partial)
		if err != nil {
			return nil
		}
		if dirPart, filePart := filepath.Split(lookup); dirPart != "" {
			searchDir = filepath.Clean(dirPart)
			namePrefix = filePart
		}
	}
	namePrefixLower := strings.ToLower(namePrefix)

	items, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}

	matches := make([]xdfileManagedShellPathSuggestionMatch, 0, min(len(items), xdfileManagedShellPathSuggestionLimit))
	for _, item := range items {
		name := item.Name()
		if !strings.HasPrefix(strings.ToLower(name), namePrefixLower) {
			continue
		}
		isDir := item.IsDir()
		switch mode {
		case xdfileShellPathSuggestionDirs:
			if !isDir {
				continue
			}
		case xdfileShellPathSuggestionFiles:
			if isDir {
				continue
			}
		}

		resolved := name
		if partialDir, _ := filepath.Split(partial); partialDir != "" {
			resolved = filepath.Join(partialDir, name)
		}
		if isDir {
			resolved += string(os.PathSeparator)
		}
		if quoted || strings.ContainsRune(resolved, ' ') {
			resolved = string(maxByte(quoteChar, '"')) + resolved + string(maxByte(quoteChar, '"'))
		}
		matches = append(matches, xdfileManagedShellPathSuggestionMatch{
			Value: base + resolved,
			Name:  name,
			IsDir: isDir,
		})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		left := matches[i]
		right := matches[j]
		if left.IsDir != right.IsDir {
			return left.IsDir
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})
	if len(matches) > xdfileManagedShellPathSuggestionLimit {
		matches = matches[:xdfileManagedShellPathSuggestionLimit]
	}

	results := make([]string, 0, len(matches))
	for _, match := range matches {
		results = append(results, match.Value)
	}
	return results
}

func xdfileShellCommandPathSuggestionMode(name string) (xdfileShellPathSuggestionMode, bool) {
	switch strings.ToLower(name) {
	case "cd", "chdir", "set-location":
		return xdfileShellPathSuggestionDirs, true
	case "type", "cat", "get-content", "grep", "head", "tail":
		return xdfileShellPathSuggestionFiles, true
	case "dir", "ls", "ll", "la", "preview", "view", "stat", "info", "tree", "find", "search", "open", "copy", "move", "del", "rm", "cp", "mv", "explorer", "code", "get-childitem", "copy-item", "move-item", "remove-item":
		return xdfileShellPathSuggestionAny, true
	default:
		return xdfileShellPathSuggestionAny, false
	}
}

func xdfileShellCommandShouldSuggestPath(name string, fields []string, trailingSpace bool, expectsPath bool) bool {
	if !expectsPath {
		return false
	}
	argCount := len(fields) - 1
	switch strings.ToLower(name) {
	case "grep":
		return xdfileShellGrepCommandShouldSuggestPath(fields[1:], trailingSpace)
	case "find", "search":
		if argCount <= 0 {
			return false
		}
		if argCount == 1 {
			return trailingSpace
		}
		return true
	case "head", "tail":
		return xdfileShellLineCommandShouldSuggestPath(fields[1:], trailingSpace)
	default:
		return true
	}
}

func xdfileShellGrepCommandShouldSuggestPath(args []string, trailingSpace bool) bool {
	if len(args) == 0 {
		return false
	}
	offset := 0
	if strings.EqualFold(args[0], "-i") {
		offset = 1
	}
	patternCount := len(args) - offset
	switch {
	case patternCount <= 0:
		return false
	case patternCount == 1:
		return trailingSpace
	default:
		return true
	}
}

func xdfileShellLineCommandShouldSuggestPath(args []string, trailingSpace bool) bool {
	if len(args) == 0 {
		return true
	}
	targetIndex := -1
	for i := 0; i < len(args); i++ {
		arg := args[i]
		lower := strings.ToLower(arg)
		switch {
		case lower == "-n":
			if i == len(args)-1 {
				return false
			}
			i++
		case strings.HasPrefix(lower, "-n="):
			continue
		case strings.HasPrefix(arg, "-"):
			return false
		default:
			targetIndex = i
			i = len(args)
		}
	}
	if targetIndex < 0 {
		return true
	}
	if targetIndex == len(args)-1 {
		return !trailingSpace
	}
	return false
}

func maxByte(left byte, right byte) byte {
	if left != 0 {
		return left
	}
	return right
}
