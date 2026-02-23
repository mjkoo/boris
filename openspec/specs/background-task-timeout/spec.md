### Requirement: Optional background task safety-net timeout
When `--bg-timeout` is set to a positive duration (in seconds), each background task SHALL have a maximum lifetime enforced by a timer. When the timer expires, the task's process group SHALL receive SIGTERM, followed by SIGKILL after 5 seconds if still running. The task SHALL be marked as timed out so that `task_output` reports the timeout. When `--bg-timeout` is 0 (the default), no per-task timeout is applied.

#### Scenario: Background task killed after timeout
- **WHEN** `--bg-timeout=60` is configured and a background task runs for more than 60 seconds
- **THEN** the task's process group receives SIGTERM, and after 5 seconds receives SIGKILL if still running

#### Scenario: Task output reports timeout
- **WHEN** a background task is killed by the safety-net timeout and `task_output` is called with its task ID
- **THEN** the response includes a message indicating the task was killed due to timeout

#### Scenario: No timeout by default
- **WHEN** `--bg-timeout` is not specified and a background task runs for an extended period
- **THEN** no per-task timeout is applied; the task runs until it completes or the session is closed

#### Scenario: Task completes before timeout
- **WHEN** `--bg-timeout=300` is configured and a background task completes in 10 seconds
- **THEN** the timeout timer is cancelled and has no effect on the task
