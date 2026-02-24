## MODIFIED Requirements

### Requirement: Replace unique string in file
The `str_replace` tool SHALL accept `path` (required), `old_str` (required), `new_str` (optional, defaults to empty string for deletion), and `replace_all` (optional boolean, default false). When `RequireViewBeforeEdit` is enabled, the tool SHALL verify the file has been viewed in the current session before performing any replacement; if not viewed, it SHALL return an error with code `FILE_NOT_VIEWED`. When `replace_all` is false, the tool SHALL find `old_str` in the file and replace it with `new_str`, requiring the match to be unique — appearing exactly once in the file. When `replace_all` is true, the tool SHALL replace all occurrences of `old_str` with `new_str`.

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

#### Scenario: Edit rejected without prior view
- **WHEN** `RequireViewBeforeEdit` is enabled and the file has NOT been viewed in the session
- **THEN** the tool returns an error with code `FILE_NOT_VIEWED` before attempting any file read or replacement

### Requirement: Create or overwrite a file
The `create_file` tool SHALL accept `path` (required) and `content` (required). When `RequireViewBeforeEdit` is enabled and the target file already exists, the tool SHALL verify the file has been viewed in the current session before overwriting; if not viewed, it SHALL return an error with code `FILE_NOT_VIEWED`. When the file does not exist, the view check SHALL be skipped. It SHALL write `content` to the file at `path`, overwriting the file if it already exists.

#### Scenario: Create new file
- **WHEN** the file does not exist and `create_file` is called
- **THEN** the file is created with the specified content and the tool returns confirmation with file path and size

#### Scenario: Overwrite existing file
- **WHEN** the file already exists, has been viewed, and `create_file` is called
- **THEN** the file is overwritten with the new content

#### Scenario: Overwrite rejected without prior view
- **WHEN** `RequireViewBeforeEdit` is enabled, the file exists, and it has NOT been viewed in the session
- **THEN** the tool returns an error with code `FILE_NOT_VIEWED`
