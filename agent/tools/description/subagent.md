Manage subagents for delegating complex or time-consuming tasks.

## Actions

### spawn
Create a subagent to handle a task. Set `background: true` to run asynchronously.

### get_result
Retrieve results from background subagents.

Parameters:
- `get_names`: array of subagent names to retrieve results from (required)
- `wait_mode`: "any" (return when first completes) or "all" (wait for all). Default: "all"

## Usage Guidelines

- Use subagents for multi-step tasks, extensive research, or long-running operations
- Foreground mode (default): blocks until subagent completes and returns result directly
- Background mode: returns immediately, you must call `get_result` later

## Important

**Before ending your response, if you spawned any background subagent, you MUST call `get_result` to ensure all results are collected.**
