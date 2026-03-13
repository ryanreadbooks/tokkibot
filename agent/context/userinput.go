package context

type UserInput struct {
	Channel     string // channel source
	ChatId      string // chat id
	Content     string // user input content
	Created     int64  // created at unix timestamp
	Attachments []*UserInputAttachment
}

type AttachmentType string

const (
	ImageAttachment AttachmentType = "image"
	FileAttachment  AttachmentType = "file"
	AudioAttachment AttachmentType = "audio" // unsupported yet
	VideoAttachment AttachmentType = "video" // unsupported yet
)

// image, audio, video, file, etc
type UserInputAttachment struct {
	Key      string
	Type     AttachmentType
	Data     []byte
	MimeType string
}
