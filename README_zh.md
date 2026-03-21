# Tokkibot

<p align="center">
  <img src="docs/tokkibot.png" alt="Tokkibot logo" height="96" style="vertical-align: middle; margin-right: 14px;">
  <img src="docs/tokkibot-title.png" alt="Tokkibot title" height="96" style="vertical-align: middle;">
</p>

<p align="center">
  <a href="README.md">English</a>
</p>

Tokkibot 是一个通用型 AI Agent，支持多通道交互（CLI / 飞书）、工具调用、长期记忆和定时任务。

## 目录

- [✨ 特性](#-特性)
- [🚀 快速开始](#-快速开始)
- [🛠 使用](#-使用)
- [🔐 环境变量](#-环境变量)
- [📄 许可证](#-许可证)

## ✨ 特性

- **多通道支持**：CLI 交互式终端、飞书群聊/IM 机器人
- **工具调用**：文件读写、Shell 执行、Web 抓取、Skill 扩展
- **子agent**：将复杂任务委托给专门的 Subagent
- **上下文管理**：自动压缩、历史摘要，控制 Token 占用
- **长期记忆**：跨会话持久化记忆
- **定时任务**：Cron 调度，支持结果投递到飞书
- **流式输出**：实时显示生成内容

## 🚀 快速开始

### 初始化

```bash
tokkibot onboard

# 为指定 agent 初始化工作区
tokkibot onboard --agent analyst
```

这会在 `~/.tokkibot/` 创建配置文件和工作区。

### 配置

编辑 `~/.tokkibot/config.json`：

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
      "name": "main",
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

## 🛠 使用

### CLI 交互

```bash
# 启动交互式 TUI
tokkibot agent

# 单次问答
tokkibot agent --message "帮我写一个 Python 脚本"

# 恢复会话
tokkibot agent --resume <session-id>

# 指定配置中的 agent
tokkibot agent --agent main

# 输出当前系统提示词
tokkibot agent prompt

# 在 CLI 中查看已安装技能
tokkibot agent skills list
```

### Gateway

```bash
tokkibot gateway
```

启动后，gateway 会监听已配置的 channel，并将消息路由给 agent。
当前已支持飞书，后续可扩展更多 channel。

**支持的 channel（仅 Gateway）：**

| Channel | 状态 | 说明 |
|---------|------|------|
| 飞书（Lark） | ✅ | 群聊 / IM 机器人集成 |

**控制命令：**

| 命令 | 描述 |
|------|------|
| `/stop` | 停止当前任务 |
| `/new` | 开始新会话 |
| `/compact` | 压缩上下文 |
| `/skill list` | 列出所有可用技能 |
| `/skill info <name>` | 显示技能详情 |
| `/mcp list` | 列出所有 MCP 服务器和状态 |
| `/mcp info <server>` | 显示服务器工具 |
| `/model` | 显示当前模型与提供商 |
| `/model set <provider> [model]` | 切换提供商/模型 |
| `/status` | 显示当前会话状态 |
| `/help` | 显示帮助 |

### 定时任务

```bash
# 列出任务
tokkibot cron list

# 添加任务
tokkibot cron add \
  --name "daily-report" \
  --expr "0 9 * * *" \
  --prompt "生成今日工作报告"

# 添加带投递的任务
tokkibot cron add \
  --name "morning-greeting" \
  --expr "0 8 * * *" \
  --prompt "生成一句早安问候" \
  --deliver \
  --channel lark \
  --to "oc_xxxxx"

# 添加一次性任务（执行后自动禁用）
tokkibot cron add \
  --name "deploy-check" \
  --expr "0 10 * * *" \
  --prompt "运行部署检查清单" \
  --once

# 手动执行
tokkibot cron run daily-report

# 启用/禁用
tokkibot cron enable daily-report
tokkibot cron disable daily-report

# 删除
tokkibot cron delete daily-report
```

Cron 任务定义保存在 `~/.tokkibot/crons/`，任务会话 ID 形如 `cron:<task-name>`。

### 技能

技能通过领域知识和工具扩展 Agent 能力。使用 [clawhub](https://github.com/openclaw/clawhub) 安装技能：

```bash
# 安装技能
clawhub install tavily-search --dir ~/.tokkibot/skills

# 安装指定版本
clawhub install tavily-search@1.0.0 --dir ~/.tokkibot/skills
```

技能在启动时自动从 `~/.tokkibot/skills/` 加载。每个技能包含：
- `SKILL.md` - 技能定义和使用说明
- 其他资源（提示词、模板等）

### MCP（模型上下文协议）

Tokkibot 支持 MCP 服务器以扩展工具能力。

你可以通过两种方式配置 MCP：

1. 使用 CLI（推荐）：

```bash
# 向项目配置添加 stdio MCP 服务
tokkibot mcp add --transport stdio myserver -- npx -y @anthropic/mcp-server

# 添加远程 HTTP MCP 服务
tokkibot mcp add --transport http remote-server https://api.example.com/mcp

# 查看与删除服务
tokkibot mcp list
tokkibot mcp remove remote-server
```

2. 手动编辑配置文件（`~/.tokkibot/mcp.json`）：

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
      "url": "http://localhost:8080/mcp",
      "headers": {
        "Authorization": "Bearer ${API_TOKEN}"
      }
    }
  }
}
```

**配置字段（命令模式）：**
- `command` - 启动 MCP 服务器的可执行命令
- `args` - 命令行参数
- `env` - 环境变量（支持 `${VAR}` 语法展开）

**配置字段（URL 模式）：**
- `url` - 远程 MCP 服务器 URL（优先使用 Streamable HTTP，回退 SSE）
- `headers` - HTTP 请求头（支持 `${VAR}` 语法展开）

MCP 会同时加载 `~/.tokkibot/mcp.json` 与 `<project>/.tokkibot/mcp.json`（同名时项目配置覆盖全局配置）。MCP 服务器会自动启动，其工具对 Agent 可用。

## 🔐 环境变量

| 变量 | 描述 |
|------|------|
| `OPENAI_API_KEY` | OpenAI API Key |
| `DEEPSEEK_API_KEY` | DeepSeek API Key |
| `MOONSHOT_API_KEY` | Moonshot API Key |

## 📄 许可证

[Apache 2.0](LICENSE)
