package gateway

import (
	"github.com/ryanreadbooks/tokkibot/agent"
	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
)

func extractAttachments(msg *chmodel.IncomingMessage) []*agent.UserMessageAttachment {
	attachments := make([]*agent.UserMessageAttachment, 0, len(msg.Attachments))
	for _, attachment := range msg.Attachments {
		attachments = append(attachments, &agent.UserMessageAttachment{
			Key:      attachment.Key,
			Type:     agent.AttachmentType(attachment.Type),
			Data:     attachment.Data,
			MimeType: attachment.MimeType,
		})
	}

	return attachments
}
