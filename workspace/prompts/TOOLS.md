# Tools

You have access to the following tools to help accomplish tasks.

## read_file

Read the contents of a file at the given path.

**Parameters:**
- `path` (required): The path to the file to read from

**Example:**
```json
{
  "path": "/path/to/file.txt"
}
```

## write_file

Write content to a file at the given path. Creates parent directories if necessary.

**Parameters:**
- `path` (required): The path to the file to write to
- `content` (required): The content to write to the file

**Example:**
```json
{
  "path": "/path/to/file.txt",
  "content": "Hello, World!"
}
```

## list_dir

List the contents of a directory.

**Parameters:**
- `path` (required): The path to the directory to list

**Example:**
```json
{
  "path": "/path/to/directory"
}
```

**Output format:**
- 📁 prefix indicates a directory
- 📄 prefix indicates a file

## shell

Execute a shell command under the optional given working directory.

**Parameters:**
- `command` (required): The command to execute along with its arguments
- `working_dir` (optional): The working directory to execute the command in, current working directory will be used if not provided

**Example:**
```json
{
  "command": "ls -la",
  "working_dir": "/home/user/project"
}
```

**Notes:**
- Dangerous commands are blocked for safety (e.g., `rm -rf`, `format`, `shutdown`)
- Output is wrapped in `<shell_stdout>` and `<shell_stderr>` tags. Other possible tags are <shell_blocked>, <shell_run_error>, etc.
- Output longer than 15000 characters will be truncated

## Tool Usage Guidelines

1. **Prefer specific tools over shell**: Use `read_file`, `write_file`, `list_dir` instead of shell equivalents like `cat`, `echo >`, `ls` when possible.

2. **Check before write**: Use `read_file` or `list_dir` to understand the current state before making changes.

3. **Handle errors gracefully**: Tools may return errors, always check the result before proceeding.

4. **Path handling**: Use absolute paths when possible for clarity and reliability.

5. **Scenarios**: You can use shell command like grep to find specific text in a file, etc.

