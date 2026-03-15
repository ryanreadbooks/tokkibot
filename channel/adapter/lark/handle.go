package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/ryanreadbooks/tokkibot/channel/adapter/lark/emoji"
	"github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/pkg/xstring"
)

var mentionStripRegexp = regexp.MustCompile(`@_user_\d+`)

// TextMessageContent represents the structure of a text message
type TextMessageContent struct {
	Text string `json:"text"`
}

func (a *LarkAdapter) handleTextMessage(content string) (string, []string, error) {
	var text TextMessageContent

	err := json.Unmarshal(xstring.ToBytes(content), &text)
	if err != nil {
		return "", nil , err
	}

	// extract mentioned users before stripping, format: @_user_X, X is the order of mentioned user
	mentionedUsers := mentionStripRegexp.FindAllString(text.Text, -1)
	// strip @_user_X from text
	text.Text = mentionStripRegexp.ReplaceAllString(text.Text, "")

	return text.Text, mentionedUsers, nil
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

type FileMessageContent struct {
	FileKey  string `json:"file_key"`
	FileName string `json:"file_name"`
}

type AudioMessageContent struct {
	FileKey  string `json:"file_key"`
	Duration int    `json:"duration"`
}

type ShareUserMessageContent struct {
	UserId string `json:"user_id"` // open_id
}

type ShareChatMessageContent struct {
	ChatId string `json:"chat_id"` // chat_id
}

type LocationMessageContent struct {
	Name      string `json:"name"`
	Longitude string `json:"longitude"` // xx.xx
	Latitude  string `json:"latitude"`  // xx.xx
}

func (a *LarkAdapter) handlePostMessage(ctx context.Context, content, messageId string) (
	string,
	[]*model.IncomingMessageAttachment,
	error,
) {
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
				if data, err := a.downloadMessageResourceImage(ctx, messageId, element.ImageKey); err == nil {
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
				if coverData, err := a.downloadMessageResourceImage(ctx, messageId, element.ImageKey); err == nil {
					attachments = append(attachments, &model.IncomingMessageAttachment{
						Key:  wrapResourceKey(element.ImageKey),
						Type: model.AttachmentImage,
						Data: coverData,
					})
				}
				if videoData, err := a.downloadMessageResourceFile(ctx, messageId, element.FileKey); err == nil {
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

func (a *LarkAdapter) handleImageMessage(ctx context.Context, messageId, content string) (string, []byte, string, error) {
	var image ImageMessageContent
	err := json.Unmarshal(xstring.ToBytes(content), &image)
	if err != nil {
		return "", nil, "", err
	}

	if len(image.ImageKey) == 0 {
		return "", nil, "", fmt.Errorf("image key is empty")
	}

	data, err := a.downloadMessageResourceImage(ctx, messageId, image.ImageKey)
	if err != nil {
		return "", nil, "", err
	}

	// detect mime type
	mimeType := mimetype.Detect(data)

	return image.ImageKey, data, mimeType.String(), nil
}

func (a *LarkAdapter) handleFileMessage(ctx context.Context, messageId, content string) (string, []byte, error) {
	// download file
	var file FileMessageContent
	err := json.Unmarshal(xstring.ToBytes(content), &file)
	if err != nil {
		return "", nil, err
	}

	if len(file.FileKey) == 0 {
		return "", nil, fmt.Errorf("file key is empty")
	}

	data, err := a.downloadMessageResourceFile(ctx, messageId, file.FileKey)
	if err != nil {
		return "", nil, err
	}

	// detect file type, video may be provided here
	mimeType := mimetype.Detect(data)
	// only handle text files for now
	if !strings.Contains(mimeType.String(), "text") {
		return "", nil, fmt.Errorf("[%s] 只支持处理文本文件", emoji.EMBARRASSED)
	}

	return file.FileName, data, nil
}

func (a *LarkAdapter) handleAudioMessage(ctx context.Context, messageId, content string) (string, []byte, error) {
	var audio AudioMessageContent
	err := json.Unmarshal(xstring.ToBytes(content), &audio)
	if err != nil {
		return "", nil, err
	}

	if len(audio.FileKey) == 0 {
		return "", nil, fmt.Errorf("file key is empty")
	}

	data, err := a.downloadMessageResourceFile(ctx, messageId, audio.FileKey)
	if err != nil {
		return "", nil, err
	}

	return audio.FileKey, data, nil
}

func (a *LarkAdapter) handleMediaMessage() (string, error) {
	return "", fmt.Errorf("[%s] 我暂时还看不懂视频~", emoji.EMBARRASSED)
}

func (a *LarkAdapter) handleStickerMessage() (string, error) {
	return "", fmt.Errorf("[%s] 我暂时还看不懂表情包~", emoji.EMBARRASSED)
}

func (a *LarkAdapter) handleInteractiveMessage(content string) string {
	// TODO
	return content
}

func (a *LarkAdapter) handleShareChatMessage(ctx context.Context, messageId, content string) (string, error) {
	var shareChat ShareChatMessageContent
	err := json.Unmarshal(xstring.ToBytes(content), &shareChat)
	if err != nil {
		return "", err
	}

	if len(shareChat.ChatId) == 0 {
		return "", fmt.Errorf("chat id is empty")
	}

	return fmt.Sprintf("用户分享了群聊 %s 的名片", shareChat.ChatId), nil
}

func (a *LarkAdapter) handleShareUserMessage(ctx context.Context, messageId, content string) (string, error) {
	var shareUser ShareUserMessageContent
	err := json.Unmarshal(xstring.ToBytes(content), &shareUser)
	if err != nil {
		return "", err
	}

	if len(shareUser.UserId) == 0 {
		return "", fmt.Errorf("user id is empty")
	}

	return fmt.Sprintf("用户分享了用户 %s 的名片", shareUser.UserId), nil
}

func (a *LarkAdapter) handleLocationMessage(ctx context.Context, messageId, content string) (string, error) {
	var location LocationMessageContent
	err := json.Unmarshal(xstring.ToBytes(content), &location)
	if err != nil {
		return "", err
	}

	if len(location.Name) == 0 {
		return "", fmt.Errorf("name is empty")
	}

	return fmt.Sprintf("用户分享了位置 %s, 经度: %s, 纬度: %s", location.Name, location.Longitude, location.Latitude), nil
}
