# Tokkibot

<p align="center">
  <img src="docs/tokkibot.png" alt="Tokkibot" width="200">
</p>

<p align="center">
  <a href="README.md">English</a>
</p>

Tokkibot 是一个通用型 AI Agent，支持多通道交互（CLI / 飞书）、工具调用、长期记忆和定时任务。

## 特性

- **多通道支持**：CLI 交互式终端、飞书群聊/IM 机器人
- **工具调用**：文件读写、Shell 执行、Web 抓取、Skill 扩展
- **上下文管理**：自动压缩、历史摘要，控制 Token 占用
- **长期记忆**：跨会话持久化记忆
- **定时任务**：Cron 调度，支持结果投递到飞书
- **流式输出**：实时显示生成内容

## 快速开始

### 初始化

```bash
tokkibot onboard
```

这会在 `~/.tokkibot/` 创建配置文件和工作区。

### 配置

编辑 `~/.tokkibot/config.yaml`：

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

## 使用

### CLI 交互

```bash
# 启动交互式 TUI
tokkibot agent

# 单次问答
tokkibot agent --message "帮我写一个 Python 脚本"

# 恢复会话
tokkibot agent --resume <session-id>
```

### 飞书机器人

```bash
tokkibot gateway
```

启动后，机器人会监听飞书消息并自动回复。

**控制命令：**
- `/stop` - 停止当前任务
- `/new` - 开始新会话
- `/compact` - 压缩上下文
- `/help` - 显示帮助

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

# 手动执行
tokkibot cron run daily-report

# 启用/禁用
tokkibot cron enable daily-report
tokkibot cron disable daily-report

# 删除
tokkibot cron delete daily-report
```

## 工作区

默认工作区位于 `~/.tokkibot/`：

```
~/.tokkibot/
├── config.yaml      # 配置文件
├── prompts/         # 系统提示词（可自定义）
├── memory/          # 长期记忆
│   └── LONG-TERM.md
├── crons/           # 定时任务
│   └── <task-name>/
│       ├── meta.json
│       └── prompt.md
└── refs/            # 引用内容
```

## 环境变量

| 变量 | 描述 |
|------|------|
| `OPENAI_API_KEY` | OpenAI API Key |
| `MOONSHOT_API_KEY` | Moonshot API Key |

## License

[Apache 2.0](LICENSE)
