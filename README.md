# Tokkibot

<p align="center">
  <img src="docs/tokkibot.png" alt="Tokkibot" width="200">
</p>

<p align="center">
  <a href="README_zh.md">中文文档</a>
</p>

Tokkibot is a general-purpose AI Agent that supports multi-channel interaction (CLI / Lark), tool invocation, long-term memory, and scheduled tasks.

## Features

- **Multi-channel Support**: CLI interactive terminal, Lark (Feishu) group chat/IM bot
- **Tool Invocation**: File read/write, Shell execution, Web fetching, Skill extensions
- **Context Management**: Auto-compression, history summarization, Token control
- **Long-term Memory**: Persistent memory across sessions
- **Scheduled Tasks**: Cron scheduling with result delivery to Lark
- **Streaming Output**: Real-time display of generated content

## Quick Start

### Initialization

```bash
tokkibot onboard
```

This creates the configuration file and workspace in `~/.tokkibot/`.

### Configuration

Edit `~/.tokkibot/config.yaml`:

```yaml
default_provider: "openai"

providers:
  openai:
    api_key: "${OPENAI_API_KEY}"
    base_url: "https://api.openai.com/v1"
    default_model: "gpt-4o-mini"

adapters:
  lark:
    app_id: "your-app-id"
    app_secret: "your-app-secret"

agent:
  max_iteration: 30
```

## Usage

### CLI Interaction

```bash
# Start interactive TUI
tokkibot agent

# Single query
tokkibot agent --message "Write a Python script for me"

# Resume session
tokkibot agent --resume <session-id>
```

### Lark Bot

```bash
tokkibot gateway
```

After starting, the bot will listen to Lark messages and respond automatically.

**Control Commands:**
- `/stop` - Stop current task
- `/new` - Start new session
- `/compact` - Compress context
- `/help` - Show help

### Scheduled Tasks

```bash
# List tasks
tokkibot cron list

# Add task
tokkibot cron add \
  --name "daily-report" \
  --expr "0 9 * * *" \
  --prompt "Generate daily work report"

# Add task with delivery
tokkibot cron add \
  --name "morning-greeting" \
  --expr "0 8 * * *" \
  --prompt "Generate a morning greeting" \
  --deliver \
  --channel lark \
  --to "oc_xxxxx"

# Manual execution
tokkibot cron run daily-report

# Enable/Disable
tokkibot cron enable daily-report
tokkibot cron disable daily-report

# Delete
tokkibot cron delete daily-report
```

## Workspace

Default workspace is located at `~/.tokkibot/`:

```
~/.tokkibot/
├── config.yaml      # Configuration file
├── prompts/         # System prompts (customizable)
├── memory/          # Long-term memory
│   └── LONG-TERM.md
├── crons/           # Scheduled tasks
│   └── <task-name>/
│       ├── meta.json
│       └── prompt.md
└── refs/            # Reference content
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | OpenAI API Key |
| `MOONSHOT_API_KEY` | Moonshot API Key |

## License

MIT
