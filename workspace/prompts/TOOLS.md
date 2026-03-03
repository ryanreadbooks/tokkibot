# Tools Reference

## Response Format

All tools return JSON with this structure:

```json
{
  "success": true,
  "data": "...",
  "err": ""
}
```

Always check `success` before using `data`. If `success` is false, handle the error in `err`.

---

## read_file

Read file contents with line numbers.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Absolute path to file |
| `offset` | No | Starting line (1-indexed) |
| `limit` | No | Number of lines to read |

```json
{ "path": "/path/to/file.txt" }
```

Output format: `LINE_NUMBER|LINE_CONTENT` per line.

---

## write_file

Create or overwrite a file. Creates parent directories automatically.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Absolute path to file |
| `content` | Yes | Content to write |

```json
{ "path": "/path/to/file.txt", "content": "Hello" }
```

⚠️ **Warning**: This overwrites the entire file. Use `edit_file` for modifications.

---

## edit_file

Replace specific text in a file. Safer than `write_file` for modifications.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `file_name` | Yes | Absolute path to file |
| `old_string` | Yes | Text to find (must be unique) |
| `new_string` | Yes | Replacement text |
| `replace_all` | No | Replace all occurrences (default: false) |

```json
{
  "file_name": "/path/to/file",
  "old_string": "foo",
  "new_string": "bar"
}
```

**Best practice**: Include enough context in `old_string` to make it unique.

---

## list_dir

List directory contents.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Absolute path to directory |

```json
{ "path": "/path/to/directory" }
```

Output uses prefixes: `📁` for directories, `📄` for files.

---

## shell

Execute a shell command.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `command` | Yes | Command to execute |
| `working_dir` | No | Working directory (default: cwd) |

```json
{ "command": "git status", "working_dir": "/project" }
```

**Limits:**
- Max execution time: 60 seconds
- Max output: 15000 characters
- Dangerous commands are blocked

**When to use shell vs other tools:**
- Use `read_file` instead of `cat`
- Use `list_dir` instead of `ls`
- Use `shell` for: git, npm, pip, grep, compilation, etc.

---

## load_ref

Load a previously stored reference.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `name` | Yes | Reference ID (format: `@refs/{id}`) |

```json
{ "name": "@refs/abc123" }
```

Use when tool output was truncated and stored as a reference.

---

## use_skill

Interact with skills for specialized capabilities.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `name` | Yes | Skill name |
| `action` | Yes | `activate`, `load`, or `script` |
| `args` | For load/script | Action arguments |

**Actions:**

| Action | Purpose | Args |
|--------|---------|------|
| `activate` | Load skill instructions | None |
| `load` | Get specific resources | Comma-separated paths |
| `script` | Run automation | Command line |

```json
{ "name": "my-skill", "action": "activate" }
{ "name": "my-skill", "action": "load", "args": "references/guide.md" }
{ "name": "my-skill", "action": "script", "args": "python analyze.py" }
```

---

## web_fetch

Fetch and convert web content.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `url` | Yes | URL (http:// or https://) |

```json
{ "url": "https://example.com/page" }
```

**Output:**

```json
{
  "content": "...",
  "content_type": "text/html",
  "status_code": 200,
  "truncated": false,
  "is_binary": false
}
```

**Behavior:**
- HTML → converted to markdown
- JSON/Text → returned as-is
- Binary → base64url encoded

**Limits:** 30s timeout, 10MB max, 50000 chars output

---

## Quick Reference

| Task | Tool | Example |
|------|------|---------|
| Read a file | `read_file` | `{"path": "/file.txt"}` |
| Modify a file | `edit_file` | `{"file_name": "/file.txt", "old_string": "a", "new_string": "b"}` |
| Create a file | `write_file` | `{"path": "/new.txt", "content": "..."}` |
| List files | `list_dir` | `{"path": "/dir"}` |
| Run command | `shell` | `{"command": "git status"}` |
| Fetch URL | `web_fetch` | `{"url": "https://..."}` |
