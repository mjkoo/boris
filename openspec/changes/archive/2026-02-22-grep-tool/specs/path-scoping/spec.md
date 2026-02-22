## MODIFIED Requirements

### Requirement: File tools use the path resolver
All file tools (`view`, `str_replace`, `create_file`, `grep`) SHALL pass their path arguments through the resolver before performing any filesystem operations. If the resolver returns an error, the tool SHALL return that error to the caller without performing the operation.

#### Scenario: File tool respects deny
- **WHEN** `view` is called with a path that matches a deny entry
- **THEN** the tool returns an access denied error without reading the file

#### Scenario: Grep search root respects allow list
- **WHEN** `grep` is called with a `path` that resolves to a location outside the allowed directories
- **THEN** the tool returns an access denied error without performing any search

#### Scenario: Grep silently skips denied files during traversal
- **WHEN** `grep` is searching a directory and encounters a file matching a deny pattern
- **THEN** the file is silently skipped (no error, no result for that file)
