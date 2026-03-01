package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/pkg/xstring"
)

var mentionStripRegexp = regexp.MustCompile(`@_user_\d+`)

// TextMessageContent represents the structure of a text message
type TextMessageContent struct {
	Text string `json:"text"`
}

func (a *LarkAdapter) handleTextMessage(content string) (string, error) {
	var text TextMessageContent

	err := json.Unmarshal(xstring.ToBytes(content), &text)
	if err != nil {
		return "", err
	}

	// if at someone, strip, format: @_user_X, X is the order of mentioned user
	// strip using regexp
	text.Text = mentionStripRegexp.ReplaceAllString(text.Text, "")

	return text.Text, nil
}

// PostMessageContent represents the structure of a post message
type PostMessageContent struct {
	Title   string             `json:"title,omitempty"`
	Content [][]PostElementRaw `json:"content,omitempty"`
}

// PostElementRaw represents a raw post element for deserialization
type PostElementRaw struct {
	Tag       string   `json:"tag"`
	Text      string   `json:"text,omitempty"`
	UnEscape  bool     `json:"un_escape,omitempty"`
	Href      string   `json:"href,omitempty"`
	UserId    string   `json:"user_id,omitempty"`
	UserName  string   `json:"user_name,omitempty"`
	ImageKey  string   `json:"image_key,omitempty"`
	FileKey   string   `json:"file_key,omitempty"`
	EmojiType string   `json:"emoji_type,omitempty"`
	Language  string   `json:"language,omitempty"`
	Style     []string `json:"style,omitempty"`
}

type ImageMessageContent struct {
	ImageKey string `json:"image_key"`
}

func (a *LarkAdapter) handlePostMessage(ctx context.Context, content, messageId string) (string, []*model.IncomingMessageAttachment, error) {
	var post PostMessageContent
	err := json.Unmarshal(xstring.ToBytes(content), &post)
	if err != nil {
		return "", nil, err
	}

	var textBuilder strings.Builder

	// Add title
	if post.Title != "" {
		textBuilder.WriteString(post.Title)
		textBuilder.WriteString("\n")
	}

	attachments := make([]*model.IncomingMessageAttachment, 0)

	// Iterate through all lines
	for lineIdx, line := range post.Content {
		if lineIdx > 0 {
			textBuilder.WriteString("\n")
		}

		// Iterate through all elements in the line
		for _, element := range line {
			switch element.Tag {
			case "text":
				textBuilder.WriteString(element.Text)
			case "a":
				// Hyperlink displayed as: text(link)
				textBuilder.WriteString(element.Text)
				if element.Href != "" {
					textBuilder.WriteString("(")
					textBuilder.WriteString(element.Href)
					textBuilder.WriteString(")")
				}
			case "at":
				// @user displayed as: @username
				if element.UserName != "" {
					textBuilder.WriteString("@")
					textBuilder.WriteString(element.UserName)
				} else if element.UserId != "" {
					textBuilder.WriteString("@")
					textBuilder.WriteString(element.UserId)
				}
			case "img":
				// Image displayed as placeholder
				textBuilder.WriteString("[Image:")
				textBuilder.WriteString(element.ImageKey)
				textBuilder.WriteString("]")
				if data, err := a.downloadImage(ctx, messageId, element.ImageKey); err == nil {
					attachments = append(attachments, &model.IncomingMessageAttachment{
						Key:  wrapResourceKey(element.ImageKey),
						Type: model.AttachmentImage,
						Data: data,
					})
				}
			case "media":
				// Media displayed as placeholder
				textBuilder.WriteString("[Media:")
				textBuilder.WriteString(element.FileKey)
				textBuilder.WriteString(",Cover:")
				textBuilder.WriteString(element.ImageKey)
				textBuilder.WriteString("]")
				// download cover and video content
				if coverData, err := a.downloadImage(ctx, messageId, element.ImageKey); err == nil {
					attachments = append(attachments, &model.IncomingMessageAttachment{
						Key:  wrapResourceKey(element.ImageKey),
						Type: model.AttachmentImage,
						Data: coverData,
					})
				}
				if videoData, err := a.downloadFile(ctx, element.FileKey); err == nil {
					attachments = append(attachments, &model.IncomingMessageAttachment{
						Key:  wrapResourceKey(element.FileKey),
						Type: model.AttachmentVideo,
						Data: videoData,
					})
				}

			case "emotion":
				// Emotion displayed as placeholder
				textBuilder.WriteString("[Emotion:")
				textBuilder.WriteString(element.EmojiType)
				textBuilder.WriteString("]")
			case "hr":
				// Horizontal rule
				textBuilder.WriteString("\n---\n")
			case "code_block":
				// Code block
				textBuilder.WriteString("\n```")
				if element.Language != "" {
					textBuilder.WriteString(element.Language)
				}
				textBuilder.WriteString("\n")
				textBuilder.WriteString(element.Text)
				textBuilder.WriteString("\n```\n")
			}
		}
	}

	return textBuilder.String(), attachments, nil
}

func (a *LarkAdapter) handleImageMessage(ctx context.Context, messageId, content string) (string, []byte, error) {
	var image ImageMessageContent
	err := json.Unmarshal(xstring.ToBytes(content), &image)
	if err != nil {
		return "", nil, err
	}

	if len(image.ImageKey) == 0 {
		return "", nil, fmt.Errorf("image key is empty")
	}

	data, err := a.downloadImage(ctx, messageId, image.ImageKey)
	if err != nil {
		return "", nil, err
	}

	return image.ImageKey, data, nil
}

func (a *LarkAdapter) handleFileMessage() string {
	return ""
}

func (a *LarkAdapter) handleAudioMessage() string {
	return ""
}

func (a *LarkAdapter) handleMediaMessage() string {
	return ""
}

func (a *LarkAdapter) handleStickerMessage() string {
	return ""
}

func (a *LarkAdapter) handleInteractiveMessage() string {
	return ""
}

func (a *LarkAdapter) handleShareChatMessage() string {
	return ""
}

func (a *LarkAdapter) handleShareUserMessage() string {
	return ""
}
