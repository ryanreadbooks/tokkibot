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

Edit `~/.tokkibot/config.json`:

```json
{
  "providers": {
    "openai": {
      "apiKey": "${OPENAI_API_KEY}",
      "baseURL": "https://api.openai.com/v1",
      "defaultModel": "gpt-4o-mini"
    },
    "deepseek": {
      "apiKey": "${DEEPSEEK_API_KEY}",
      "baseURL": "https://api.deepseek.com/v1",
      "defaultModel": "deepseek-reasoner",
      "enableThinking": true
    }
  },
  "agents": [
    {
      "id": "main",
      "maxIteration": 30,
      "provider": "openai",
      "model": "gpt-4o",
      "binding": {
        "match": { "channel": "lark", "account": "default" }
      }
    }
  ],
  "channels": [
    {
      "name": "lark",
      "account": {
        "default": { "appId": "your-app-id", "appSecret": "your-app-secret" }
      }
    }
  ]
}
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

| Command | Description |
|---------|-------------|
| `/stop` | Stop current task |
| `/new` | Start new session |
| `/compact` | Compress context |
| `/skill list` | List all available skills |
| `/skill info <name>` | Show skill details |
| `/mcp list` | List all MCP servers and status |
| `/mcp info <server>` | Show server tools |
| `/help` | Show help |

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

### Skills

Skills extend the agent's capabilities with domain-specific knowledge and tools. Install skills using [clawhub](https://github.com/openclaw/clawhub):

```bash
# Install a skill
clawhub install tavily-search --dir ~/.tokkibot/skills

# Install with specific version
clawhub install tavily-search@1.0.0 --dir ~/.tokkibot/skills
```

Skills are automatically loaded from `~/.tokkibot/skills/` on startup. Each skill contains:
- `SKILL.md` - Skill definition and instructions
- Additional resources (prompts, templates, etc.)

### MCP (Model Context Protocol)

Tokkibot supports MCP servers for extended tool capabilities. Configure in `~/.tokkibot/mcp.json`:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@anthropics/mcp-filesystem", "/path/to/allowed/dir"]
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@anthropics/mcp-github"],
      "env": {
        "GITHUB_TOKEN": "${GITHUB_TOKEN}"
      }
    },
    "remote-server": {
      "url": "http://localhost:8080/sse",
      "headers": {
        "Authorization": "Bearer ${API_TOKEN}"
      }
    }
  }
}
```

**Configuration fields (command mode):**
- `command` - Executable command to start the MCP server
- `args` - Command line arguments
- `env` - Environment variables (supports `${VAR}` syntax for expansion)

**Configuration fields (URL mode):**
- `url` - SSE endpoint URL for remote MCP server
- `headers` - HTTP headers (supports `${VAR}` syntax for expansion)

MCP servers are started automatically and their tools become available to the agent.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | OpenAI API Key |
| `DEEPSEEK_API_KEY` | DeepSeek API Key |
| `MOONSHOT_API_KEY` | Moonshot API Key |

## License

[Apache 2.0](LICENSE)
