# Tools Guide

## Response Format

All tools return: `{"success": bool, "data": "...", "err": "..."}`. Always check `success` before using `data`.

## Key Rules

**File Operations:**
- Use `edit_file` for modifications (safer than `write_file` which overwrites entirely)
- Include enough context in `old_string` to ensure uniqueness

**Shell:**
- Use dedicated tools instead: `read_file` not `cat`, `list_dir` not `ls`
- Shell for: git, npm, pip, compilation, etc.
- Limits: 60s timeout, 15000 chars output

**Cron:**
- Tasks auto-deliver results to current chat
- `one_shot=true` for one-time tasks that auto-disable after execution
- Expression format: `min hour day month weekday` (e.g., `0 9 * * *` = daily 9am)

## Quick Reference

| Task | Tool |
|------|------|
| Read file | `read_file` |
| Modify file | `edit_file` |
| Create file | `write_file` |
| List directory | `list_dir` |
| Run command | `shell` |
| Fetch URL | `web_fetch` |
| Schedule task | `schedule_cron` / `list_cron` / `delete_cron` |
| Load reference | `load_ref` |
| Use skill | `use_skill` |
| Todo/Task| `todo_write` |
