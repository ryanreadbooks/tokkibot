package tools

import (
	"context"
	"fmt"

	"github.com/ryanreadbooks/tokkibot/agent/tools/description"
	"github.com/ryanreadbooks/tokkibot/component/tool"
)

type SendMessageInput struct {
	Content     string   `json:"content"               jsonschema:"description=The content of the message to send."`
	Attachments []string `json:"attachments,omitempty" jsonschema:"description=List of file paths to attach (images, audio, documents)"`
}

type MessageTarget struct {
	Channel string
	ChatId  string
}

type MessageSender interface {
	Send(ctx context.Context, target MessageTarget, meta tool.InvokeMeta, input *SendMessageInput) error
}

// send message to meta.Channel with meta.ChatId
func SendMessage(sender MessageSender) tool.Invoker {
	info := tool.Info{
		Name:        ToolNameSendMessage,
		Description: description.SendMessageDescription,
	}

	return tool.NewInvoker(info, func(ctx context.Context, meta tool.InvokeMeta, input *SendMessageInput) (string, error) {
		if len(input.Attachments) > 0 {
			// TODO we have to make sure here, attachments are accessible to prevent security issues
		}

		err := sender.Send(ctx, MessageTarget{
			Channel: meta.Channel,
			ChatId:  meta.ChatId,
		}, meta, input)
		if err != nil {
			return "", fmt.Errorf("failed to send message: %s", err.Error())
		}

		return "Message sent successfully", nil
	})
}
