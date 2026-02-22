## ADDED Requirements

### Requirement: View file contents
The `view` tool SHALL accept a `path` string parameter (required) and a `view_range` parameter (optional, array of two integers `[start, end]`, 1-indexed). When `path` is a file, the tool SHALL return the file contents with line numbers prefixed to each line (e.g., `   1\tline content`).

#### Scenario: Read entire file
- **WHEN** the tool is called with `path` pointing to a file containing 3 lines
- **THEN** the result contains all 3 lines, each prefixed with its 1-indexed line number

#### Scenario: Read line range
- **WHEN** the tool is called with `path` pointing to a 100-line file and `view_range: [10, 20]`
- **THEN** the result contains only lines 10 through 20, each with correct line number prefixes

#### Scenario: Invalid view_range
- **WHEN** the tool is called with `view_range` where start > end, or start < 1, or end > total lines
- **THEN** the tool returns an error describing the valid range

### Requirement: View truncates large files
When a file exceeds 2000 lines and no `view_range` is specified, the tool SHALL return the first 2000 lines followed by a message indicating the total line count and suggesting use of `view_range`.

#### Scenario: Large file truncation
- **WHEN** the tool is called on a file with 5000 lines and no `view_range`
- **THEN** the result contains the first 2000 lines and a message like "Truncated: file has 5000 lines. Use view_range to read specific sections."

### Requirement: View detects binary files
When `path` points to a binary file, the tool SHALL NOT return the raw content. Instead it SHALL return a message indicating the file type and size.

#### Scenario: Binary file detected
- **WHEN** the tool is called with `path` pointing to a compiled binary or image file
- **THEN** the result contains a message like "Binary file (2.4 MB)" rather than raw binary content

### Requirement: View lists directories
When `path` is a directory, the tool SHALL return a listing of files and subdirectories up to 2 levels deep. Hidden files (starting with `.`) and `node_modules` directories SHALL be excluded from the listing.

#### Scenario: Directory listing
- **WHEN** the tool is called with `path` pointing to a directory containing files and subdirectories
- **THEN** the result shows files and directories up to 2 levels deep

#### Scenario: Hidden files and node_modules excluded
- **WHEN** the tool is called on a directory containing `.git/`, `.env`, `node_modules/`, and `src/`
- **THEN** the result includes `src/` but excludes `.git/`, `.env`, and `node_modules/`

### Requirement: View follows symlinks
The tool SHALL follow symbolic links when reading files. The symlink is resolved before reading.

#### Scenario: Symlink followed
- **WHEN** the tool is called with `path` pointing to a symlink that targets a regular file
- **THEN** the result contains the contents of the target file

### Requirement: View resolves relative paths against session cwd
When `path` is relative, the tool SHALL resolve it relative to the session's current working directory (as tracked by the bash tool).

#### Scenario: Relative path resolution
- **WHEN** the session cwd is `/workspace/project` and view is called with `path: "src/main.go"`
- **THEN** the tool reads `/workspace/project/src/main.go`

### Requirement: View respects max file size
The tool SHALL refuse to read files larger than the configured `--max-file-size` (default 10MB), returning an error indicating the file size and the limit.

#### Scenario: File exceeds max size
- **WHEN** the tool is called on a 50MB file with default max-file-size of 10MB
- **THEN** the tool returns an error indicating the file exceeds the size limit

### Requirement: Replace unique string in file
The `str_replace` tool SHALL accept `path` (required), `old_str` (required), and `new_str` (optional, defaults to empty string for deletion). It SHALL find `old_str` in the file and replace it with `new_str`. The match MUST be unique — appearing exactly once in the file.

#### Scenario: Successful replacement
- **WHEN** `old_str` appears exactly once in the file
- **THEN** the tool replaces it with `new_str` and returns a confirmation with a snippet of surrounding context showing the replacement

#### Scenario: String not found
- **WHEN** `old_str` does not appear in the file
- **THEN** the tool returns an error indicating the string was not found

#### Scenario: Ambiguous match (multiple occurrences)
- **WHEN** `old_str` appears more than once in the file
- **THEN** the tool returns an error indicating the count of occurrences and that the match must be unique

#### Scenario: Deletion (empty new_str)
- **WHEN** `old_str` is found and `new_str` is omitted or empty
- **THEN** the tool deletes `old_str` from the file

### Requirement: str_replace preserves file attributes
The tool SHALL preserve file permissions and line endings (LF vs CRLF) when writing the modified content.

#### Scenario: Permissions preserved
- **WHEN** a file with mode 0755 is modified via str_replace
- **THEN** the file retains mode 0755 after the replacement

### Requirement: str_replace resolves paths and checks scoping
The tool SHALL resolve relative paths against the session cwd and pass the resolved path through the path scoping resolver before performing any file operations.

#### Scenario: Path scoping enforced
- **WHEN** `path` resolves to a location outside the allowed directories
- **THEN** the tool returns an error indicating access is denied

### Requirement: Create or overwrite a file
The `create_file` tool SHALL accept `path` (required), `content` (required), and `overwrite` (optional boolean, default false). It SHALL write `content` to the file at `path`.

#### Scenario: Create new file
- **WHEN** the file does not exist and `create_file` is called
- **THEN** the file is created with the specified content and the tool returns confirmation with file path and size

#### Scenario: Refuse overwrite by default
- **WHEN** the file already exists and `overwrite` is false or omitted
- **THEN** the tool returns an error indicating the file already exists

#### Scenario: Overwrite when allowed
- **WHEN** the file exists and `overwrite: true`
- **THEN** the file is overwritten with the new content

### Requirement: create_file creates parent directories
The tool SHALL create any missing parent directories as needed (mode 0755).

#### Scenario: Nested path creation
- **WHEN** `path` is `src/pkg/new/file.go` and `src/pkg/new/` does not exist
- **THEN** the tool creates `src/pkg/new/` and writes the file

### Requirement: create_file sets default permissions
New files SHALL be created with mode 0644.

#### Scenario: File permissions
- **WHEN** a new file is created via create_file
- **THEN** the file has mode 0644

### Requirement: create_file respects max file size
The tool SHALL refuse to create files whose content exceeds `--max-file-size`.

#### Scenario: Content exceeds max size
- **WHEN** `content` is larger than the configured max file size
- **THEN** the tool returns an error indicating the content exceeds the size limit

### Requirement: create_file resolves paths and checks scoping
The tool SHALL resolve relative paths against the session cwd and pass the resolved path through the path scoping resolver before performing any file operations.

#### Scenario: Path scoping enforced
- **WHEN** `path` resolves to a location outside the allowed directories
- **THEN** the tool returns an error indicating access is denied

### Requirement: File tools return errors for nonexistent paths
The `view` and `str_replace` tools SHALL return a clear error message when the target file or directory does not exist.

#### Scenario: File not found
- **WHEN** `view` or `str_replace` is called with a path that does not exist
- **THEN** the tool returns an error indicating the path was not found
