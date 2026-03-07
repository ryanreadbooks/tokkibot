Search for files using glob patterns. Supports `*`, `?`, and `**` (recursive).

**Example use cases:**
- `*.py` - Python files in current dir
- `src/**/*.js` - JS files under src/ recursively
- `test_*.py` - Test files with prefix
- `*.config.{js,ts}` - Config files with multiple extensions

**Forbidden patterns:**
- `**` or `**/*.py` - Patterns starting with `**` are rejected (too broad, may overflow context)
- `node_modules/**/*` - Avoid recursive search in large directories (`node_modules`, `venv`, `__pycache__`, `target`)

**Rule:** Always scope patterns to specific directories (e.g., `src/**/*.py` not `**/*.py`)