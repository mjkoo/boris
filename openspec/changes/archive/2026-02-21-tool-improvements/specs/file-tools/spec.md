## MODIFIED Requirements

### Requirement: View file contents
The `view` tool SHALL accept a `path` string parameter (required) and a `view_range` parameter (optional, array of two integers `[start, end]`, 1-indexed). When `path` is a file, the tool SHALL return the file contents with line numbers prefixed to each line (e.g., `   1\tline content`). Individual lines longer than 2,000 characters SHALL be truncated with a suffix indicating the total line length.

#### Scenario: Read entire file
- **WHEN** the tool is called with `path` pointing to a file containing 3 lines
- **THEN** the result contains all 3 lines, each prefixed with its 1-indexed line number

#### Scenario: Read line range
- **WHEN** the tool is called with `path` pointing to a 100-line file and `view_range: [10, 20]`
- **THEN** the result contains only lines 10 through 20, each with correct line number prefixes

#### Scenario: view_range end clamped to file length
- **WHEN** the tool is called with `path` pointing to a 42-line file and `view_range: [10, 100]`
- **THEN** the result contains lines 10 through 42 (end is clamped to file length, no error)

#### Scenario: view_range start exceeds file length
- **WHEN** the tool is called with `view_range: [100, 200]` on a 42-line file
- **THEN** the tool returns an error indicating start exceeds total lines

#### Scenario: Invalid view_range start
- **WHEN** the tool is called with `view_range` where start < 1 or start > end
- **THEN** the tool returns an error describing the valid range

#### Scenario: Long line truncation
- **WHEN** a file contains a line with 5,000 characters (e.g., minified JavaScript)
- **THEN** the line is truncated at 2,000 characters with a suffix like `... [truncated, 5000 chars total]`

#### Scenario: Efficient range reading
- **WHEN** the tool is called with `view_range: [1, 10]` on a file near the max file size limit
- **THEN** the tool reads the file line-by-line without loading the entire file into memory

### Requirement: View detects binary files
When `path` points to a binary file that is not a recognized image format, the tool SHALL NOT return the raw content. Instead it SHALL return a message indicating the file type and size.

#### Scenario: Binary file detected
- **WHEN** the tool is called with `path` pointing to a compiled binary
- **THEN** the result contains a message like "Binary file (2.4 MB)" rather than raw binary content

### Requirement: View lists directories
When `path` is a directory, the tool SHALL return a listing of files and subdirectories up to 2 levels deep. Only `.git/` directories and `node_modules/` directories SHALL be excluded from the listing. All other dotfiles and dotdirectories (e.g., `.github/`, `.dockerignore`, `.env.example`) SHALL be included. Symlinks SHALL be indicated with ` -> target` notation showing the link target.

#### Scenario: Directory listing
- **WHEN** the tool is called with `path` pointing to a directory containing files and subdirectories
- **THEN** the result shows files and directories up to 2 levels deep

#### Scenario: Dotfiles visible except .git
- **WHEN** the tool is called on a directory containing `.github/`, `.dockerignore`, `.env`, `.git/`, `node_modules/`, and `src/`
- **THEN** the result includes `.github/`, `.dockerignore`, `.env`, and `src/` but excludes `.git/` and `node_modules/`

#### Scenario: Symlinks indicated in listing
- **WHEN** the tool is called on a directory containing a symlink `link` pointing to `/usr/local/bin`
- **THEN** the listing shows `link -> /usr/local/bin`

### Requirement: Replace unique string in file
The `str_replace` tool SHALL accept `path` (required), `old_str` (required), `new_str` (optional, defaults to empty string for deletion), and `replace_all` (optional boolean, default false). When `replace_all` is false, the tool SHALL find `old_str` in the file and replace it with `new_str`, requiring the match to be unique — appearing exactly once in the file. When `replace_all` is true, the tool SHALL replace all occurrences of `old_str` with `new_str`.

#### Scenario: Successful unique replacement
- **WHEN** `old_str` appears exactly once in the file and `replace_all` is false
- **THEN** the tool replaces it with `new_str` and returns a confirmation with a snippet of surrounding context showing the replacement

#### Scenario: String not found
- **WHEN** `old_str` does not appear in the file
- **THEN** the tool returns an error indicating the string was not found

#### Scenario: Ambiguous match (multiple occurrences, replace_all false)
- **WHEN** `old_str` appears more than once in the file and `replace_all` is false or omitted
- **THEN** the tool returns an error indicating the count of occurrences and that the match must be unique

#### Scenario: Deletion (empty new_str)
- **WHEN** `old_str` is found and `new_str` is omitted or empty
- **THEN** the tool deletes `old_str` from the file

#### Scenario: Replace all occurrences
- **WHEN** `old_str` appears 15 times in the file and `replace_all: true`
- **THEN** the tool replaces all 15 occurrences with `new_str` and returns a confirmation indicating the count of replacements

#### Scenario: Replace all with no match
- **WHEN** `old_str` does not appear in the file and `replace_all: true`
- **THEN** the tool returns an error indicating the string was not found

### Requirement: Create or overwrite a file
The `create_file` tool SHALL accept `path` (required) and `content` (required). It SHALL write `content` to the file at `path`, overwriting the file if it already exists.

#### Scenario: Create new file
- **WHEN** the file does not exist and `create_file` is called
- **THEN** the file is created with the specified content and the tool returns confirmation with file path and size

#### Scenario: Overwrite existing file
- **WHEN** the file already exists and `create_file` is called
- **THEN** the file is overwritten with the new content

## ADDED Requirements

### Requirement: View returns image files as ImageContent
When `path` points to an image file, the tool SHALL return the file as an `ImageContent` MCP content block with base64-encoded data and the appropriate MIME type, instead of a "Binary file" text message. Image detection SHALL use magic byte sniffing (via the file header bytes already read for binary detection) as the primary method. SVG files, which are text-based and not detectable via magic bytes, SHALL be detected by `.svg` file extension as a fallback. The max file size limit still applies.

#### Scenario: PNG file returned as ImageContent
- **WHEN** the tool is called with `path` pointing to a PNG file (detected via magic bytes) within the size limit
- **THEN** the result contains an `ImageContent` block with MIME type `image/png` and base64-encoded file data

#### Scenario: JPEG file returned as ImageContent
- **WHEN** the tool is called with `path` pointing to a JPEG file (detected via magic bytes)
- **THEN** the result contains an `ImageContent` block with MIME type `image/jpeg`

#### Scenario: Image with wrong extension still detected
- **WHEN** the tool is called with `path` pointing to a file named `photo.dat` that contains PNG magic bytes
- **THEN** the result contains an `ImageContent` block with MIME type `image/png` (detection is based on content, not extension)

#### Scenario: SVG file detected by extension
- **WHEN** the tool is called with `path` pointing to a `.svg` file
- **THEN** the result contains an `ImageContent` block with MIME type `image/svg+xml`

#### Scenario: Image file exceeding size limit
- **WHEN** the tool is called with `path` pointing to an image file larger than `--max-file-size`
- **THEN** the tool returns an error indicating the file exceeds the size limit

#### Scenario: Unrecognized binary file still shows size message
- **WHEN** the tool is called with `path` pointing to a `.exe` or `.wasm` binary file
- **THEN** the result contains a "Binary file (size)" text message (not ImageContent)

## REMOVED Requirements

### Requirement: Create or overwrite a file
**Reason**: The `overwrite` parameter is removed. The tool now always overwrites existing files, matching Claude Code's Write behavior. Safety is the caller's responsibility.
**Migration**: Remove `overwrite: true` from tool calls. The tool now overwrites by default. Clients that relied on the overwrite guard should implement their own existence check if needed.
