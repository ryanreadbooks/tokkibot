package context

type UserInput struct {
	Channel     string // channel source
	ChatId      string // chat id
	Content     string // user input content
	Created     int64  // created at unix timestamp
	Attachments []*UserInputAttachment
	Control     UserInputControl
}

func (i *UserInput) HasControl() bool {
	return validCommands[i.Control.Command]
}

type AttachmentType string

const (
	ImageAttachment AttachmentType = "image"
	AudioAttachment AttachmentType = "audio"
	VideoAttachment AttachmentType = "video"
	FileAttachment  AttachmentType = "file"
)

// image, audio, video, file, etc
type UserInputAttachment struct {
	Key  string
	Type AttachmentType
	Data []byte
}

type UserInputControl struct {
	Command ControlCommand
}

type ControlCommand string

const (
	ControlCommandStop ControlCommand = "/stop"
	ControlCommandNew  ControlCommand = "/new"
	ControlCommandHelp ControlCommand = "/help"
)

var validCommands = map[ControlCommand]bool{
	ControlCommandStop: true,
	ControlCommandNew:  true,
	ControlCommandHelp: true,
}
