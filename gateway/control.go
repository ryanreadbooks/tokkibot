package gateway

import (
	"fmt"
	"strings"

	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
)

// ControlCommand represents a control command from user
type ControlCommand string

const (
	ControlCmdNone    ControlCommand = ""
	ControlCmdStop    ControlCommand = "/stop"
	ControlCmdNew     ControlCommand = "/new"
	ControlCmdCompact ControlCommand = "/compact"
	ControlCmdHelp    ControlCommand = "/help"
)

var controlCommands = []ControlCommand{
	ControlCmdStop,
	ControlCmdNew,
	ControlCmdCompact,
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
	if err := g.agent.ClearSession(channel, chatId); err != nil {
		g.sendResponse(rawMsg, "Failed to clear session: "+err.Error())
		return
	}
	g.sendResponse(rawMsg, "New session started")
}

func (g *Gateway) handleCompact(rawMsg *chmodel.IncomingMessage) {
	channel := rawMsg.Channel.String()
	chatId := rawMsg.ChatId
	compressed, err := g.agent.CompactSession(rawMsg.Context(), channel, chatId)
	if err != nil {
		g.sendResponse(rawMsg, "Failed to compact context: "+err.Error())
		return
	}
	g.sendResponse(rawMsg, fmt.Sprintf("Context compacted (compressed %d tool calls)", compressed))
}

func (g *Gateway) handleHelp(rawMsg *chmodel.IncomingMessage) {
	g.sendResponse(rawMsg, helpMessage)
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
