package gateway

import (
	"fmt"
	"strings"

	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/config"
)

// ControlCommand represents a control command from user
type ControlCommand string

const (
	ControlCmdNone    ControlCommand = ""
	ControlCmdStop    ControlCommand = "/stop"
	ControlCmdNew     ControlCommand = "/new"
	ControlCmdCompact ControlCommand = "/compact"
	ControlCmdSkill   ControlCommand = "/skill"
	ControlCmdMcp     ControlCommand = "/mcp"
	ControlCmdModel   ControlCommand = "/model"
	ControlCmdStatus  ControlCommand = "/status"
	ControlCmdHelp    ControlCommand = "/help"
)

var controlCommands = []ControlCommand{
	ControlCmdStop,
	ControlCmdNew,
	ControlCmdCompact,
	ControlCmdSkill,
	ControlCmdMcp,
	ControlCmdModel,
	ControlCmdStatus,
	ControlCmdHelp,
}

// parseControlCommand extracts control command from message content
func parseControlCommand(content string) ControlCommand {
	content = strings.TrimSpace(content)
	for _, cmd := range controlCommands {
		cmdStr := string(cmd)
		if content == cmdStr || strings.HasPrefix(content, cmdStr+" ") {
			return cmd
		}
	}
	return ControlCmdNone
}

const helpMessage = `**Available Commands:**
- /stop - Stop the current running task
- /new - Start a new session (clear context)
- /compact - Compact context (compress tool calls and summarize history)
- /skill list - List all available skills
- /skill info <name> - Show skill details
- /mcp list - List all MCP servers and status
- /mcp info <server> - Show server tools
- /model - Show current model and available providers
- /model set <provider> [model] - Switch provider and model
- /status - Show current session status (model, context size, etc.)
- /help - Show this help message`

// handleControl handles control commands and returns true if handled
func (g *Gateway) handleControl(rawMsg *chmodel.IncomingMessage, cmd ControlCommand) bool {
	if cmd == ControlCmdNone {
		return false
	}

	switch cmd {
	case ControlCmdStop:
		g.handleStop(rawMsg)
	case ControlCmdNew:
		g.handleNew(rawMsg)
	case ControlCmdCompact:
		g.handleCompact(rawMsg)
	case ControlCmdSkill:
		g.handleSkill(rawMsg)
	case ControlCmdMcp:
		g.handleMcp(rawMsg)
	case ControlCmdModel:
		g.handleModel(rawMsg)
	case ControlCmdStatus:
		g.handleStatus(rawMsg)
	case ControlCmdHelp:
		g.handleHelp(rawMsg)
	}

	return true
}

func (g *Gateway) handleStop(rawMsg *chmodel.IncomingMessage) {
	chatKey := rawMsg.Key()

	g.runningMu.RLock()
	cancel, exists := g.running[chatKey]
	g.runningMu.RUnlock()

	if !exists {
		g.sendResponse(rawMsg, "No running task to stop")
		return
	}

	cancel()
	g.sendResponse(rawMsg, "Task stop signal sent")
}

func (g *Gateway) handleNew(rawMsg *chmodel.IncomingMessage) {
	channel := rawMsg.Channel.String()
	chatId := rawMsg.ChatId
	ag := g.agentForAdapter(rawMsg.Channel)
	if err := ag.ClearContext(channel, chatId); err != nil {
		g.sendResponse(rawMsg, "Failed to clear session: "+err.Error())
		return
	}
	g.sendResponse(rawMsg, "New session started")
}

func (g *Gateway) handleCompact(rawMsg *chmodel.IncomingMessage) {
	channel := rawMsg.Channel.String()
	chatId := rawMsg.ChatId
	ag := g.agentForAdapter(rawMsg.Channel)
	compressed, err := ag.CompactContext(rawMsg.Context(), channel, chatId)
	if err != nil {
		g.sendResponse(rawMsg, "Failed to compact context: "+err.Error())
		return
	}
	g.sendResponse(rawMsg, fmt.Sprintf("Context compacted (compressed %d tool calls)", compressed))
}

func (g *Gateway) handleHelp(rawMsg *chmodel.IncomingMessage) {
	g.sendResponse(rawMsg, helpMessage)
}

func (g *Gateway) handleSkill(rawMsg *chmodel.IncomingMessage) {
	content := strings.TrimSpace(rawMsg.Content)
	args := strings.TrimPrefix(content, string(ControlCmdSkill))
	args = strings.TrimSpace(args)

	// Parse subcommand and arguments
	parts := strings.SplitN(args, " ", 2)
	subCmd := ""
	subArg := ""
	if len(parts) > 0 {
		subCmd = strings.ToLower(parts[0])
	}
	if len(parts) > 1 {
		subArg = strings.TrimSpace(parts[1])
	}

	switch subCmd {
	case "", "list":
		g.handleSkillList(rawMsg)
	case "info":
		g.handleSkillInfo(rawMsg, subArg)
	default:
		g.sendResponse(rawMsg, fmt.Sprintf("Unknown skill subcommand: %s\nUsage: /skill list | /skill info <name>", subCmd))
	}
}

func (g *Gateway) handleSkillList(rawMsg *chmodel.IncomingMessage) {
	ag := g.agentForAdapter(rawMsg.Channel)
	skills := ag.AvailableSkills()
	if len(skills) == 0 {
		g.sendResponse(rawMsg, "No skills available")
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "**Available Skills (%d):**\n", len(skills))
	for _, s := range skills {
		fmt.Fprintf(&sb, "- **%s** - %s\n", s.Name(), s.Description())
	}
	g.sendResponse(rawMsg, sb.String())
}

func (g *Gateway) handleSkillInfo(rawMsg *chmodel.IncomingMessage, name string) {
	if name == "" {
		g.sendResponse(rawMsg, "Usage: /skill info <name>")
		return
	}

	ag := g.agentForAdapter(rawMsg.Channel)
	skills := ag.AvailableSkills()
	for _, s := range skills {
		if s.Name() == name {
			var sb strings.Builder
			fmt.Fprintf(&sb, "**Skill: %s**\n", s.Name())
			fmt.Fprintf(&sb, "- Description: %s\n", s.Description())
			if meta := s.Metadata(); len(meta) > 0 {
				fmt.Fprintf(&sb, "- Metadata:\n")
				for k, v := range meta {
					fmt.Fprintf(&sb, "  - %s: %s\n", k, v)
				}
			}
			g.sendResponse(rawMsg, sb.String())
			return
		}
	}

	g.sendResponse(rawMsg, fmt.Sprintf("Skill not found: %s", name))
}

func (g *Gateway) handleMcp(rawMsg *chmodel.IncomingMessage) {
	content := strings.TrimSpace(rawMsg.Content)
	args := strings.TrimPrefix(content, string(ControlCmdMcp))
	args = strings.TrimSpace(args)

	parts := strings.SplitN(args, " ", 2)
	subCmd := ""
	subArg := ""
	if len(parts) > 0 {
		subCmd = strings.ToLower(parts[0])
	}
	if len(parts) > 1 {
		subArg = strings.TrimSpace(parts[1])
	}

	switch subCmd {
	case "", "list":
		g.handleMcpList(rawMsg)
	case "info":
		g.handleMcpInfo(rawMsg, subArg)
	default:
		g.sendResponse(rawMsg, fmt.Sprintf("Unknown mcp subcommand: %s\nUsage: /mcp list | /mcp info <name>", subCmd))
	}
}

func (g *Gateway) handleMcpList(rawMsg *chmodel.IncomingMessage) {
	ag := g.agentForAdapter(rawMsg.Channel)
	servers := ag.ListMcpServers()
	if len(servers) == 0 {
		g.sendResponse(rawMsg, "No MCP servers configured")
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "**MCP Servers (%d):**\n\n", len(servers))
	fmt.Fprintf(&sb, "| Server | Status | Tools |\n")
	fmt.Fprintf(&sb, "|--------|--------|-------|\n")
	for _, s := range servers {
		status := "✅"
		tools := fmt.Sprintf("%d", s.ToolCount)
		if !s.OK {
			status = "❌"
			tools = "-"
		}
		fmt.Fprintf(&sb, "| %s | %s | %s |\n", s.Name, status, tools)
	}
	fmt.Fprintf(&sb, "\nUse `/mcp info <server>` to see tools")
	g.sendResponse(rawMsg, sb.String())
}

func (g *Gateway) handleModel(rawMsg *chmodel.IncomingMessage) {
	content := strings.TrimSpace(rawMsg.Content)
	args := strings.TrimPrefix(content, string(ControlCmdModel))
	args = strings.TrimSpace(args)

	parts := strings.Fields(args)
	subCmd := ""
	if len(parts) > 0 {
		subCmd = strings.ToLower(parts[0])
	}

	switch subCmd {
	case "", "list":
		g.handleModelList(rawMsg)
	case "set":
		if len(parts) < 2 {
			g.sendResponse(rawMsg, "Usage: /model set <provider> [model]")
			return
		}
		provider := parts[1]
		model := ""
		if len(parts) >= 3 {
			model = parts[2]
		}
		g.handleModelSet(rawMsg, provider, model)
	default:
		g.sendResponse(rawMsg, fmt.Sprintf("Unknown model subcommand: %s\nUsage: /model list | /model set <provider> [model]", subCmd))
	}
}

func (g *Gateway) handleModelList(rawMsg *chmodel.IncomingMessage) {
	ag := g.agentForAdapter(rawMsg.Channel)
	cfg := config.GetConfig()

	var sb strings.Builder
	fmt.Fprintf(&sb, "**Current Model**\n")
	fmt.Fprintf(&sb, "- Provider: `%s`\n", ag.GetProvider())
	fmt.Fprintf(&sb, "- Model: `%s`\n\n", ag.GetModel())

	fmt.Fprintf(&sb, "**Available Providers**\n\n")
	fmt.Fprintf(&sb, "| Provider | Default Model | Thinking |\n")
	fmt.Fprintf(&sb, "|----------|---------------|----------|\n")

	for name, providerCfg := range cfg.Providers {
		thinking := "❌"
		if providerCfg.IsThinkingEnabled() {
			thinking = "✅"
		}
		current := ""
		if name == ag.GetProvider() {
			current = " ✓"
		}
		fmt.Fprintf(&sb, "| %s%s | %s | %s |\n", name, current, providerCfg.DefaultModel, thinking)
	}

	fmt.Fprintf(&sb, "\nUse `/model set <provider> [model]` to switch")
	g.sendResponse(rawMsg, sb.String())
}

func (g *Gateway) handleModelSet(rawMsg *chmodel.IncomingMessage, provider, model string) {
	// Check if there's a running task - cannot switch model during execution
	chatKey := rawMsg.Key()
	g.runningMu.RLock()
	_, isRunning := g.running[chatKey]
	g.runningMu.RUnlock()

	if isRunning {
		g.sendResponse(rawMsg, "Cannot switch model while a task is running. Please wait for the task to complete or use `/stop` first.")
		return
	}

	cfg := config.GetConfig()

	providerCfg, ok := cfg.Providers[provider]
	if !ok {
		var available []string
		for name := range cfg.Providers {
			available = append(available, name)
		}
		g.sendResponse(rawMsg, fmt.Sprintf("Provider not found: %s\nAvailable providers: %s", provider, strings.Join(available, ", ")))
		return
	}

	if model == "" {
		model = providerCfg.DefaultModel
	}

	ag := g.agentForAdapter(rawMsg.Channel)
	oldProvider := ag.GetProvider()
	oldModel := ag.GetModel()

	// Switch provider and model (this recreates the LLM client)
	if err := ag.SetProviderAndModel(provider, model); err != nil {
		g.sendResponse(rawMsg, fmt.Sprintf("Failed to switch model: %s", err.Error()))
		return
	}

	// Persist to config file
	if err := config.UpdateAgentProviderAndModel(ag.Name(), provider, model); err != nil {
		g.sendResponse(rawMsg, fmt.Sprintf("Model switched but failed to save config: %s", err.Error()))
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "**Model Switched**\n\n")
	fmt.Fprintf(&sb, "| | Before | After |\n")
	fmt.Fprintf(&sb, "|--|--------|-------|\n")
	fmt.Fprintf(&sb, "| Provider | %s | %s |\n", oldProvider, provider)
	fmt.Fprintf(&sb, "| Model | %s | %s |\n", oldModel, model)
	fmt.Fprintf(&sb, "\n✅ Config saved")

	g.sendResponse(rawMsg, sb.String())
}

func (g *Gateway) handleStatus(rawMsg *chmodel.IncomingMessage) {
	channel := rawMsg.Channel.String()
	chatId := rawMsg.ChatId
	ag := g.agentForAdapter(rawMsg.Channel)

	// Get context tokens
	contextTokens := ag.GetCurrentContextTokens(channel, chatId)

	// Check if task is running
	chatKey := rawMsg.Key()
	g.runningMu.RLock()
	_, isRunning := g.running[chatKey]
	g.runningMu.RUnlock()

	runningStatus := "Idle"
	if isRunning {
		runningStatus = "Running"
	}

	// Get MCP server count
	mcpServers := ag.ListMcpServers()
	mcpServerCount := len(mcpServers)
	mcpOkCount := 0
	for _, s := range mcpServers {
		if s.OK {
			mcpOkCount++
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "**Session Status**\n\n")
	fmt.Fprintf(&sb, "| Item | Value |\n")
	fmt.Fprintf(&sb, "|------|-------|\n")
	fmt.Fprintf(&sb, "| Model | %s |\n", ag.GetModel())
	fmt.Fprintf(&sb, "| Provider | %s |\n", ag.GetProvider())
	fmt.Fprintf(&sb, "| Context Size | %d tokens |\n", contextTokens)
	fmt.Fprintf(&sb, "| Task Status | %s |\n", runningStatus)
	fmt.Fprintf(&sb, "| Built-in Tools | %d |\n", ag.GetToolCount())
	fmt.Fprintf(&sb, "| MCP Tools | %d |\n", ag.GetMcpToolCount())
	fmt.Fprintf(&sb, "| MCP Servers | %d/%d online |\n", mcpOkCount, mcpServerCount)

	g.sendResponse(rawMsg, sb.String())
}

func (g *Gateway) handleMcpInfo(rawMsg *chmodel.IncomingMessage, serverName string) {
	if serverName == "" {
		g.sendResponse(rawMsg, "Usage: /mcp info <server>")
		return
	}

	ag := g.agentForAdapter(rawMsg.Channel)
	servers := ag.ListMcpServers()
	var targetServer *tool.McpServerStatus
	for _, s := range servers {
		if s.Name == serverName {
			targetServer = s
			break
		}
	}

	if targetServer == nil {
		g.sendResponse(rawMsg, fmt.Sprintf("MCP server not found: %s", serverName))
		return
	}

	var sb strings.Builder
	statusIcon := "✓"
	statusText := "ok"
	if !targetServer.OK {
		statusIcon = "✗"
		statusText = fmt.Sprintf("error: %s", targetServer.Error)
	}
	fmt.Fprintf(&sb, "**MCP Server: %s** %s %s\n", targetServer.Name, statusIcon, statusText)

	if !targetServer.OK {
		g.sendResponse(rawMsg, sb.String())
		return
	}

	tools := ag.ListMcpTools()
	var serverTools []*tool.McpTool
	for _, t := range tools {
		if t.ServerName() == serverName {
			serverTools = append(serverTools, t)
		}
	}

	fmt.Fprintf(&sb, "\n**Tools (%d):**\n", len(serverTools))
	for _, t := range serverTools {
		info := t.Info()
		fmt.Fprintf(&sb, "- **%s** - %s\n", t.ToolName(), info.Description)
	}
	g.sendResponse(rawMsg, sb.String())
}

// sendResponse sends a response back through the message callbacks
func (g *Gateway) sendResponse(rawMsg *chmodel.IncomingMessage, content string) {
	if rawMsg.Stream {
		rawMsg.EmitContent(&chmodel.StreamContent{
			Round:   0,
			Content: content,
		})
		rawMsg.EmitDone()
	}
}
