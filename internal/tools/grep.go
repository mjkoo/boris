package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GrepArgs is the input schema for the grep tool (normal MCP mode).
type GrepArgs struct {
	Pattern          string `json:"pattern" jsonschema:"the regex pattern to search for in file contents,required"`
	Path             string `json:"path,omitempty" jsonschema:"file or directory to search in (defaults to cwd)"`
	Include          string `json:"include,omitempty" jsonschema:"glob pattern to filter files (e.g. '*.js' or '*.{ts,tsx}')"`
	Type             string `json:"type,omitempty" jsonschema:"file type to search (e.g. js, py, go, ts)"`
	OutputMode       string `json:"output_mode,omitempty" jsonschema:"output mode: content, files_with_matches (default), or count"`
	CaseInsensitive  bool   `json:"case_insensitive,omitempty" jsonschema:"case-insensitive search"`
	LineNumbers      *bool  `json:"line_numbers,omitempty" jsonschema:"show line numbers in content mode (default true)"`
	Multiline        bool   `json:"multiline,omitempty" jsonschema:"enable multiline mode where . matches newlines"`
	HeadLimit        int    `json:"head_limit,omitempty" jsonschema:"limit output to first N results (0 = unlimited)"`
	Offset           int    `json:"offset,omitempty" jsonschema:"skip first N results before applying head_limit"`
	ContextBefore    *int   `json:"context_before,omitempty" jsonschema:"number of lines to show before each match"`
	ContextAfter     *int   `json:"context_after,omitempty" jsonschema:"number of lines to show after each match"`
	Context          *int   `json:"context,omitempty" jsonschema:"number of lines to show before and after each match"`
}

// GrepCompatArgs is the input schema for the grep tool in --anthropic-compat mode.
type GrepCompatArgs struct {
	Pattern     string `json:"pattern" jsonschema:"the regex pattern to search for in file contents,required"`
	Path        string `json:"path,omitempty" jsonschema:"file or directory to search in (defaults to cwd)"`
	Glob        string `json:"glob,omitempty" jsonschema:"glob pattern to filter files (e.g. '*.js' or '*.{ts,tsx}')"`
	Type        string `json:"type,omitempty" jsonschema:"file type to search (e.g. js, py, go, ts)"`
	OutputMode  string `json:"output_mode,omitempty" jsonschema:"output mode: content, files_with_matches (default), or count"`
	I           bool   `json:"-i,omitempty" jsonschema:"case-insensitive search"`
	N           *bool  `json:"-n,omitempty" jsonschema:"show line numbers in content mode (default true)"`
	Multiline   bool   `json:"multiline,omitempty" jsonschema:"enable multiline mode where . matches newlines"`
	HeadLimit   int    `json:"head_limit,omitempty" jsonschema:"limit output to first N results (0 = unlimited)"`
	Offset      int    `json:"offset,omitempty" jsonschema:"skip first N results before applying head_limit"`
	B           *int   `json:"-B,omitempty" jsonschema:"number of lines to show before each match"`
	A           *int   `json:"-A,omitempty" jsonschema:"number of lines to show after each match"`
	C           *int   `json:"-C,omitempty" jsonschema:"number of lines to show before and after each match"`
	ContextAlias *int  `json:"context,omitempty" jsonschema:"alias for -C"`
}

// grepParams holds the normalized parameters for grep search.
type grepParams struct {
	pattern         string
	path            string
	include         string
	fileType        string
	outputMode      string
	caseInsensitive bool
	lineNumbers     bool
	multiline       bool
	headLimit       int
	offset          int
	contextBefore   int
	contextAfter    int
}

func normalizeGrepArgs(args GrepArgs) grepParams {
	p := grepParams{
		pattern:         args.Pattern,
		path:            args.Path,
		include:         args.Include,
		fileType:        args.Type,
		outputMode:      args.OutputMode,
		caseInsensitive: args.CaseInsensitive,
		lineNumbers:     true,
		multiline:       args.Multiline,
		headLimit:       args.HeadLimit,
		offset:          args.Offset,
	}
	if args.LineNumbers != nil {
		p.lineNumbers = *args.LineNumbers
	}
	// Context: explicit before/after override shorthand
	if args.Context != nil {
		p.contextBefore = *args.Context
		p.contextAfter = *args.Context
	}
	if args.ContextBefore != nil {
		p.contextBefore = *args.ContextBefore
	}
	if args.ContextAfter != nil {
		p.contextAfter = *args.ContextAfter
	}
	return p
}

func normalizeGrepCompatArgs(args GrepCompatArgs) grepParams {
	p := grepParams{
		pattern:         args.Pattern,
		path:            args.Path,
		include:         args.Glob,
		fileType:        args.Type,
		outputMode:      args.OutputMode,
		caseInsensitive: args.I,
		lineNumbers:     true,
		multiline:       args.Multiline,
		headLimit:       args.HeadLimit,
		offset:          args.Offset,
	}
	if args.N != nil {
		p.lineNumbers = *args.N
	}
	// Context: -C and context alias are both shorthand; explicit -B/-A override
	if args.C != nil {
		p.contextBefore = *args.C
		p.contextAfter = *args.C
	}
	if args.ContextAlias != nil {
		p.contextBefore = *args.ContextAlias
		p.contextAfter = *args.ContextAlias
	}
	if args.B != nil {
		p.contextBefore = *args.B
	}
	if args.A != nil {
		p.contextAfter = *args.A
	}
	return p
}

func grepHandler(sess *session.Session, resolver *pathscope.Resolver) mcp.ToolHandlerFor[GrepArgs, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, args GrepArgs) (*mcp.CallToolResult, any, error) {
		return doGrep(sess, resolver, normalizeGrepArgs(args))
	}
}

func grepCompatHandler(sess *session.Session, resolver *pathscope.Resolver) mcp.ToolHandlerFor[GrepCompatArgs, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, args GrepCompatArgs) (*mcp.CallToolResult, any, error) {
		return doGrep(sess, resolver, normalizeGrepCompatArgs(args))
	}
}

// typeGlobs maps file type names to their extension glob patterns.
var typeGlobs = map[string][]string{
	"c":        {"*.c", "*.h"},
	"cpp":      {"*.cpp", "*.cc", "*.cxx", "*.hpp", "*.hh", "*.hxx", "*.h", "*.inl"},
	"css":      {"*.css", "*.scss"},
	"go":       {"*.go"},
	"html":     {"*.html", "*.htm"},
	"java":     {"*.java"},
	"js":       {"*.js", "*.mjs", "*.cjs", "*.jsx"},
	"json":     {"*.json"},
	"markdown": {"*.md", "*.markdown", "*.mdx"},
	"py":       {"*.py", "*.pyi"},
	"rust":     {"*.rs"},
	"ts":       {"*.ts", "*.tsx", "*.mts", "*.cts"},
	"yaml":     {"*.yml", "*.yaml"},
}

// typeAliases maps alias names to canonical type names.
var typeAliases = map[string]string{
	"python":     "py",
	"typescript": "ts",
	"md":         "markdown",
}

// validTypeNames returns sorted list of valid type names for error messages.
func validTypeNames() []string {
	seen := map[string]bool{}
	for k := range typeGlobs {
		seen[k] = true
	}
	for k := range typeAliases {
		seen[k] = true
	}
	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// resolveType resolves a type name (possibly alias) to its glob patterns.
func resolveType(typeName string) ([]string, error) {
	if alias, ok := typeAliases[typeName]; ok {
		typeName = alias
	}
	globs, ok := typeGlobs[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown file type %q; valid types: %s", typeName, strings.Join(validTypeNames(), ", "))
	}
	return globs, nil
}

// isBinaryHeader checks if the given header bytes indicate a binary file
// by scanning for NUL bytes, matching ripgrep's approach.
func isBinaryHeader(header []byte) bool {
	for _, b := range header {
		if b == 0 {
			return true
		}
	}
	return false
}

func doGrep(sess *session.Session, resolver *pathscope.Resolver, p grepParams) (*mcp.CallToolResult, any, error) {
	// Validate pattern
	if p.pattern == "" {
		return toolErr(ErrInvalidInput, "pattern must not be empty")
	}

	// Validate output_mode
	if p.outputMode == "" {
		p.outputMode = "files_with_matches"
	}
	switch p.outputMode {
	case "content", "files_with_matches", "count":
		// valid
	default:
		return toolErr(ErrGrepInvalidOutputMode, "invalid output_mode %q; valid values: content, files_with_matches, count", p.outputMode)
	}

	// Validate type
	var typePatterns []string
	if p.fileType != "" {
		var err error
		typePatterns, err = resolveType(p.fileType)
		if err != nil {
			return toolErr(ErrInvalidInput, "invalid file type: %v", err)
		}
	}

	// Build regex pattern with flags
	patternStr := p.pattern
	if p.multiline {
		patternStr = "(?s)" + patternStr
	}
	if p.caseInsensitive {
		patternStr = "(?i)" + patternStr
	}

	re, err := regexp.Compile(patternStr)
	if err != nil {
		return toolErr(ErrGrepInvalidPattern, "invalid regex pattern: %v", err)
	}

	// Resolve search path
	searchPath := p.path
	if searchPath == "" {
		searchPath = sess.Cwd()
	} else if !filepath.IsAbs(searchPath) {
		searchPath = filepath.Join(sess.Cwd(), searchPath)
	}

	// Check path scoping on the search root
	resolvedRoot, err := resolver.Resolve(sess.Cwd(), p.path)
	if err != nil {
		if p.path == "" {
			// cwd should always be resolvable; use it directly
			resolvedRoot = sess.Cwd()
		} else {
			return toolErr(ErrAccessDenied, "path not allowed: %v", err)
		}
	}

	info, err := os.Lstat(resolvedRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return toolErr(ErrPathNotFound, "%s does not exist", searchPath)
		}
		return toolErr(ErrIO, "could not stat %s: %v", searchPath, err)
	}

	// If it's a symlink, resolve it
	if info.Mode()&os.ModeSymlink != 0 {
		resolvedRoot, err = filepath.EvalSymlinks(resolvedRoot)
		if err != nil {
			return toolErr(ErrIO, "could not resolve symlink %s: %v", searchPath, err)
		}
		info, err = os.Stat(resolvedRoot)
		if err != nil {
			return toolErr(ErrIO, "could not stat %s: %v", searchPath, err)
		}
	}

	if info.IsDir() {
		return grepDirectory(resolver, sess, re, resolvedRoot, p, typePatterns)
	}
	return grepSingleFile(re, resolvedRoot, p.path, p, false)
}

// grepSingleFile searches a single file.
// displayPath is used in output; if empty, uses the file path.
// isPartOfDirSearch indicates if this is part of a directory walk (affects error handling).
func grepSingleFile(re *regexp.Regexp, filePath, displayPath string, p grepParams, isPartOfDirSearch bool) (*mcp.CallToolResult, any, error) {
	if displayPath == "" {
		displayPath = filePath
	}

	f, err := os.Open(filePath)
	if err != nil {
		if isPartOfDirSearch {
			// Silently skip unreadable files during directory walk
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: ""}},
			}, nil, nil
		}
		return toolErr(ErrIO, "could not open %s: %v", displayPath, err)
	}
	defer f.Close()

	// Binary detection
	header := make([]byte, 512)
	n, _ := f.Read(header)
	header = header[:n]
	if isBinaryHeader(header) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: ""}},
		}, nil, nil
	}

	// Reset file for reading
	if _, err := f.Seek(0, 0); err != nil {
		return toolErr(ErrIO, "could not seek %s: %v", displayPath, err)
	}

	if p.multiline {
		return grepFileMultiline(re, f, displayPath, p)
	}
	return grepFileLineByLine(re, f, displayPath, p)
}

// grepFileLineByLine searches file line by line.
func grepFileLineByLine(re *regexp.Regexp, f *os.File, displayPath string, p grepParams) (*mcp.CallToolResult, any, error) {
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var allLines []string
	var matchLineNums []int

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		allLines = append(allLines, line)
		if re.MatchString(line) {
			matchLineNums = append(matchLineNums, lineNum)
		}
	}

	return buildFileResult(displayPath, allLines, matchLineNums, p)
}

// grepFileMultiline searches file content as a whole string.
func grepFileMultiline(re *regexp.Regexp, f *os.File, displayPath string, p grepParams) (*mcp.CallToolResult, any, error) {
	data, err := readAllFile(f)
	if err != nil {
		return toolErr(ErrIO, "could not read %s: %v", displayPath, err)
	}
	content := string(data)

	lines := strings.Split(content, "\n")
	// Remove trailing empty line from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	matches := re.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return buildFileResult(displayPath, lines, nil, p)
	}

	// Map byte ranges to line numbers
	matchLineSet := map[int]bool{}
	for _, m := range matches {
		startLine := byteOffsetToLine(content, m[0])
		endLine := byteOffsetToLine(content, m[1]-1)
		if m[1] > 0 && m[1] <= len(content) && content[m[1]-1] == '\n' {
			// If match ends exactly at a newline, the last line is the previous one
			if endLine > startLine {
				endLine--
			}
		}
		for l := startLine; l <= endLine; l++ {
			matchLineSet[l] = true
		}
	}

	var matchLineNums []int
	for l := range matchLineSet {
		matchLineNums = append(matchLineNums, l)
	}
	sort.Ints(matchLineNums)

	return buildFileResult(displayPath, lines, matchLineNums, p)
}

// byteOffsetToLine converts a byte offset in content to a 1-indexed line number.
func byteOffsetToLine(content string, offset int) int {
	if offset < 0 {
		return 1
	}
	if offset >= len(content) {
		offset = len(content) - 1
	}
	line := 1
	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			line++
		}
	}
	return line
}

// buildFileResult constructs results from matched line numbers.
// matchLineNums are 1-indexed.
func buildFileResult(displayPath string, allLines []string, matchLineNums []int, p grepParams) (*mcp.CallToolResult, any, error) {
	matchCount := len(matchLineNums)

	// Apply offset/head_limit for non-content modes on a single file
	if p.offset > 0 || p.headLimit > 0 {
		switch p.outputMode {
		case "files_with_matches", "count":
			// For these modes on a single file, offset/head_limit
			// have trivial effect (0 or 1 result)
			if p.offset > 0 && matchCount > 0 {
				matchCount = 0
				matchLineNums = nil
			}
		}
	}

	switch p.outputMode {
	case "files_with_matches":
		if matchCount > 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: displayPath}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: ""}},
		}, nil, nil

	case "count":
		if matchCount > 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("%s:%d", displayPath, matchCount)}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: ""}},
		}, nil, nil

	case "content":
		if matchCount == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: ""}},
			}, nil, nil
		}
		lines := formatContentLines(displayPath, allLines, matchLineNums, p)
		// Apply offset/head_limit on all output lines (match + context + separators)
		if p.offset > 0 {
			if p.offset >= len(lines) {
				lines = nil
			} else {
				lines = lines[p.offset:]
			}
		}
		if p.headLimit > 0 && len(lines) > p.headLimit {
			lines = lines[:p.headLimit]
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(lines, "\n")}},
		}, nil, nil
	}

	// unreachable: doGrep validates output_mode before calling buildFileResult
	panic("unreachable: invalid output_mode " + p.outputMode)
}

// outputGroup represents a contiguous range of lines to output (match + context).
type outputGroup struct {
	startLine int // 1-indexed
	endLine   int // 1-indexed, inclusive
}

// formatContentLines formats match and context lines for content output mode.
// Includes `--` separators between non-contiguous groups within the file.
func formatContentLines(displayPath string, allLines []string, matchLineNums []int, p grepParams) []string {
	totalLines := len(allLines)
	matchSet := map[int]bool{}
	for _, ln := range matchLineNums {
		matchSet[ln] = true
	}

	// Build output groups (contiguous ranges of lines to display)
	var groups []outputGroup
	for _, ln := range matchLineNums {
		start := ln - p.contextBefore
		if start < 1 {
			start = 1
		}
		end := ln + p.contextAfter
		if end > totalLines {
			end = totalLines
		}
		// Merge with previous group if overlapping or adjacent
		if len(groups) > 0 && start <= groups[len(groups)-1].endLine+1 {
			if end > groups[len(groups)-1].endLine {
				groups[len(groups)-1].endLine = end
			}
		} else {
			groups = append(groups, outputGroup{startLine: start, endLine: end})
		}
	}

	var result []string
	for gi, g := range groups {
		if gi > 0 {
			result = append(result, "--")
		}
		for ln := g.startLine; ln <= g.endLine; ln++ {
			line := allLines[ln-1]
			if matchSet[ln] {
				// Match line: filepath:linenum:content
				if p.lineNumbers {
					result = append(result, fmt.Sprintf("%s:%d:%s", displayPath, ln, line))
				} else {
					result = append(result, fmt.Sprintf("%s:%s", displayPath, line))
				}
			} else {
				// Context line: filepath-linenum-content
				if p.lineNumbers {
					result = append(result, fmt.Sprintf("%s-%d-%s", displayPath, ln, line))
				} else {
					result = append(result, fmt.Sprintf("%s-%s", displayPath, line))
				}
			}
		}
	}

	return result
}

// grepDirectory searches all files in a directory recursively.
func grepDirectory(resolver *pathscope.Resolver, sess *session.Session, re *regexp.Regexp, rootPath string, p grepParams, typePatterns []string) (*mcp.CallToolResult, any, error) {
	// Gitignore support
	gi := newGitignoreStack()

	// Track visited real paths for symlink cycle detection
	visited := map[string]bool{}
	realRoot, err := filepath.EvalSymlinks(rootPath)
	if err == nil {
		visited[realRoot] = true
	}

	type fileResult struct {
		displayPath string
		lines       []string // for content mode (already formatted)
		count       int      // for count mode
		modTime     int64    // for mtime sorting
		hasMatch    bool
	}

	var results []fileResult

	// Counting for head_limit/offset
	totalMatches := 0
	collected := 0
	limitReached := false

	var walkFn func(dir string) error
	walkFn = func(dir string) error {
		if limitReached {
			return nil
		}

		// Load gitignore at this level
		gi.push(dir)
		defer gi.pop()

		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil // silently skip unreadable directories
		}

		for _, entry := range entries {
			if limitReached {
				return nil
			}

			name := entry.Name()
			entryPath := filepath.Join(dir, name)

			// Skip .git and node_modules
			if name == ".git" || name == "node_modules" {
				continue
			}

			// Check gitignore
			if gi.isIgnored(entryPath, entry.IsDir() || (entry.Type()&os.ModeSymlink != 0 && isSymlinkDir(entryPath))) {
				continue
			}

			if entry.Type()&os.ModeSymlink != 0 {
				// Handle symlink
				realPath, err := filepath.EvalSymlinks(entryPath)
				if err != nil {
					continue
				}
				info, err := os.Stat(realPath)
				if err != nil {
					continue
				}
				if info.IsDir() {
					// Symlink to directory: check cycle, recurse
					if visited[realPath] {
						continue
					}
					visited[realPath] = true
					if err := walkFn(entryPath); err != nil {
						return err
					}
					continue
				}
				// Symlink to file: fall through to file handling
				entry = fakeDirEntry{name: name, info: info}
			}

			if entry.IsDir() {
				// Check cycle detection for real directories too
				realPath, err := filepath.EvalSymlinks(entryPath)
				if err != nil {
					continue
				}
				if visited[realPath] {
					continue
				}
				visited[realPath] = true
				if err := walkFn(entryPath); err != nil {
					return err
				}
				continue
			}

			// Compute relative path early (needed for include matching and display)
			relPath, err := filepath.Rel(rootPath, entryPath)
			if err != nil {
				relPath = entryPath
			}

			// File: apply filters
			if !matchesInclude(relPath, name, p.include) {
				continue
			}
			if !matchesType(name, typePatterns) {
				continue
			}

			// Path scoping: silently skip denied files
			resolvedFile, err := resolver.Resolve(sess.Cwd(), entryPath)
			if err != nil {
				continue
			}

			// Search the file
			fileLines, matchLineNums, matchCount, err := searchFile(re, resolvedFile, p)
			if err != nil || matchCount == 0 {
				continue
			}

			switch p.outputMode {
			case "files_with_matches":
				// Collect ALL matching files; offset applied after mtime sort
				info, err := entry.Info()
				var mtime int64
				if err == nil {
					mtime = info.ModTime().Unix()
				}
				results = append(results, fileResult{
					displayPath: relPath,
					hasMatch:    true,
					modTime:     mtime,
})

			case "count":
				totalMatches++
				if totalMatches <= p.offset {
					continue
				}
				results = append(results, fileResult{
					displayPath: relPath,
					count:       matchCount,
					hasMatch:    true,
				})
				collected++
				if p.headLimit > 0 && collected >= p.headLimit {
					limitReached = true
				}

			case "content":
				formatted := formatContentLines(relPath, fileLines, matchLineNums, p)
				results = append(results, fileResult{
					displayPath: relPath,
					hasMatch:    true,
					lines:       formatted,
				})
			}
		}
		return nil
	}

	if err := walkFn(rootPath); err != nil {
		return toolErr(ErrIO, "could not walk directory %s: %v", rootPath, err)
	}

	// Build output
	var output strings.Builder
	switch p.outputMode {
	case "files_with_matches":
		// Sort by mtime (newest first)
		sort.Slice(results, func(i, j int) bool {
			return results[i].modTime > results[j].modTime
		})
		// Apply offset after sorting
		if p.offset > 0 {
			if p.offset >= len(results) {
				results = nil
			} else {
				results = results[p.offset:]
			}
		}
		// Apply head_limit after offset
		if p.headLimit > 0 && len(results) > p.headLimit {
			results = results[:p.headLimit]
		}
		for i, r := range results {
			if i > 0 {
				output.WriteString("\n")
			}
			output.WriteString(r.displayPath)
		}

	case "count":
		for i, r := range results {
			if i > 0 {
				output.WriteString("\n")
			}
			fmt.Fprintf(&output, "%s:%d", r.displayPath, r.count)
		}

	case "content":
		// Collect all output lines (match + context + inter-file separators)
		var allOutputLines []string
		first := true
		for _, r := range results {
			if !r.hasMatch || len(r.lines) == 0 {
				continue
			}
			if !first {
				allOutputLines = append(allOutputLines, "--")
			}
			first = false
			allOutputLines = append(allOutputLines, r.lines...)
		}
		// Apply offset/head_limit on all output lines uniformly
		if p.offset > 0 {
			if p.offset >= len(allOutputLines) {
				allOutputLines = nil
			} else {
				allOutputLines = allOutputLines[p.offset:]
			}
		}
		if p.headLimit > 0 && len(allOutputLines) > p.headLimit {
			allOutputLines = allOutputLines[:p.headLimit]
		}
		output.WriteString(strings.Join(allOutputLines, "\n"))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: output.String()}},
	}, nil, nil
}

// searchFile searches a single file and returns its lines, match line numbers, and count.
func searchFile(re *regexp.Regexp, filePath string, p grepParams) ([]string, []int, int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, 0, err
	}
	defer f.Close()

	// Binary detection
	header := make([]byte, 512)
	n, _ := f.Read(header)
	header = header[:n]
	if isBinaryHeader(header) {
		return nil, nil, 0, nil
	}

	if _, err := f.Seek(0, 0); err != nil {
		return nil, nil, 0, err
	}

	if p.multiline {
		return searchFileMultiline(re, f)
	}
	return searchFileLineByLine(re, f)
}

func searchFileLineByLine(re *regexp.Regexp, f *os.File) ([]string, []int, int, error) {
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var allLines []string
	var matchLineNums []int

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		allLines = append(allLines, line)
		if re.MatchString(line) {
			matchLineNums = append(matchLineNums, lineNum)
		}
	}

	return allLines, matchLineNums, len(matchLineNums), nil
}

func searchFileMultiline(re *regexp.Regexp, f *os.File) ([]string, []int, int, error) {
	data, err := readAllFile(f)
	if err != nil {
		return nil, nil, 0, err
	}
	content := string(data)

	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	matches := re.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return lines, nil, 0, nil
	}

	matchLineSet := map[int]bool{}
	for _, m := range matches {
		startLine := byteOffsetToLine(content, m[0])
		endLine := byteOffsetToLine(content, m[1]-1)
		if m[1] > 0 && m[1] <= len(content) && content[m[1]-1] == '\n' {
			if endLine > startLine {
				endLine--
			}
		}
		for l := startLine; l <= endLine; l++ {
			matchLineSet[l] = true
		}
	}

	var matchLineNums []int
	for l := range matchLineSet {
		matchLineNums = append(matchLineNums, l)
	}
	sort.Ints(matchLineNums)

	return lines, matchLineNums, len(matchLineNums), nil
}

func readAllFile(f *os.File) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(f)
	return buf.Bytes(), err
}

// matchesInclude checks if a file matches the include glob pattern.
// It first tries matching against the relative path (handles "src/**/*.py"),
// then falls back to the base name (handles "*.py").
func matchesInclude(relPath, baseName, include string) bool {
	if include == "" {
		return true
	}
	// Try matching against relative path first (supports path-qualified globs)
	if matched, err := doublestar.Match(include, relPath); err == nil && matched {
		return true
	}
	// Fall back to base name match (supports simple extension globs)
	if matched, err := doublestar.Match(include, baseName); err == nil && matched {
		return true
	}
	return false
}

// matchesType checks if a filename matches any of the type glob patterns.
func matchesType(name string, typePatterns []string) bool {
	if len(typePatterns) == 0 {
		return true
	}
	for _, pattern := range typePatterns {
		matched, err := doublestar.Match(pattern, name)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// isSymlinkDir checks if a path is a symlink pointing to a directory.
func isSymlinkDir(path string) bool {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	info, err := os.Stat(target)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// fakeDirEntry is a minimal fs.DirEntry for resolved symlinks.
type fakeDirEntry struct {
	name string
	info os.FileInfo
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                 { return f.info.IsDir() }
func (f fakeDirEntry) Type() fs.FileMode           { return f.info.Mode().Type() }
func (f fakeDirEntry) Info() (fs.FileInfo, error)   { return f.info, nil }

// gitignoreStack manages a stack of gitignore matchers for nested directory traversal.
type gitignoreStack struct {
	stack []gitignoreMatcher
}

type gitignoreMatcher struct {
	dir      string
	patterns []gitignorePattern
}

type gitignorePattern struct {
	pattern  string
	negate   bool
	dirOnly  bool
}

func newGitignoreStack() *gitignoreStack {
	return &gitignoreStack{}
}

func (g *gitignoreStack) push(dir string) {
	gitignorePath := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		// No .gitignore at this level
		g.stack = append(g.stack, gitignoreMatcher{dir: dir})
		return
	}

	var patterns []gitignorePattern
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := gitignorePattern{}
		if strings.HasPrefix(line, "!") {
			p.negate = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			p.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		p.pattern = line
		patterns = append(patterns, p)
	}

	g.stack = append(g.stack, gitignoreMatcher{dir: dir, patterns: patterns})
}

func (g *gitignoreStack) pop() {
	if len(g.stack) > 0 {
		g.stack = g.stack[:len(g.stack)-1]
	}
}

func (g *gitignoreStack) isIgnored(path string, isDir bool) bool {
	// Process all gitignore levels, child overrides parent
	ignored := false
	for _, level := range g.stack {
		for _, p := range level.patterns {
			if p.dirOnly && !isDir {
				continue
			}
			// Match against relative path from gitignore location
			relPath, err := filepath.Rel(level.dir, path)
			if err != nil {
				continue
			}
			// Try matching against basename and relative path
			baseName := filepath.Base(path)
			matched := false
			if m, _ := doublestar.Match(p.pattern, baseName); m {
				matched = true
			}
			if !matched {
				if m, _ := doublestar.Match(p.pattern, relPath); m {
					matched = true
				}
			}
			if !matched {
				// Also try with ** prefix for patterns without path separators
				if !strings.Contains(p.pattern, "/") {
					if m, _ := doublestar.Match("**/"+p.pattern, relPath); m {
						matched = true
					}
				}
			}
			if matched {
				if p.negate {
					ignored = false
				} else {
					ignored = true
				}
			}
		}
	}
	return ignored
}
