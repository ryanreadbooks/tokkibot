# Tools Guide

## Response Format

All tools return: `{"success": bool, "data": "...", "err": "..."}`. Always check `success` before using `data`.

## Key Rules

**MUST follow these rules:**
- Read before edit: always `read_file` before using `edit_file`
- Prefer `edit_file` for modifications; avoid `write_file` unless full overwrite is intended
- Use absolute paths to reduce path ambiguity and mistakes
- Check each tool result and handle failures explicitly

**NEVER do these:**
- Run destructive commands (`rm -rf`, `git push --force`) without explicit confirmation
- Assume files exist without checking
- Continue after a tool error without addressing it
- Make up file contents or tool outputs

**File Operations:**
- Use `edit_file` for modifications (safer than `write_file` which overwrites entirely)
- Include enough context in `old_string` to ensure uniqueness
- Use `glob` over `shell` file pattern matching

**Shell:**
- Use dedicated tools instead: `read_file` not `cat`, `list_dir` not `ls`
- Shell for: git, npm, pip, compilation, etc.
- Limits: 60s timeout, 15000 chars output

**Cron:**
- Tasks auto-deliver results to current chat
- `one_shot=true` for one-time tasks that auto-disable after execution
- Expression format: `min hour day month weekday` (e.g., `0 9 * * *` = daily 9am)

**Error Handling:**
- Tool not found → stop calling it, report to user
- Tool failed → read error, fix if possible, then retry once
- Repeated failures → stop and explain the issue
