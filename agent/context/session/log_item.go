package session

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jinzhu/copier"
	"github.com/ryanreadbooks/tokkibot/agent/ref/media"
	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
)

func NewLogItemId() string {
	id := uuid.Must(uuid.NewV7())
	return strings.ReplaceAll(id.String(), "-", "")
}

// LogItem every messages into session file
type LogItem struct {
	Id       string               `json:"id"` // unique msg id
	Role     schema.Role          `json:"role"`
	Created  int64                `json:"created"`
	Message  *schema.MessageParam `json:"message,omitzero"`
	Metadata *LogItemMeta         `json:"metadata,omitzero"`
}

type LogItemMeta struct {
	ImageRef map[int]string `json:"image_ref,omitzero"`
}

func (i *LogItem) MarshalJSON() ([]byte, error) {
	type alias LogItem

	if !i.Role.User() || !i.HasImageRef() {
		return json.Marshal((*alias)(i))
	}

	var copied LogItem
	copier.CopyWithOption(&copied, i, copier.Option{DeepCopy: true})

	if copied.Message != nil && copied.Message.UserMessageParam != nil &&
		len(copied.Message.UserMessageParam.ContentParts) > 0 {
		for idx, part := range copied.Message.UserMessageParam.ContentParts {
			if part == nil {
				continue
			}

			if part.ImageURL != nil {
				if imageRefName, ok := copied.Metadata.ImageRef[idx]; ok {
					part.ImageURL.URL = fmt.Sprintf("[image](%s)", imageRefName)
				}
			}
		}
	}

	return json.Marshal((*alias)(&copied))
}

func (i *LogItem) UnmarshalJSON(data []byte) error {
	type Alias LogItem

	aux := (*Alias)(i)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	if !i.Role.User() || !i.HasImageRef() ||
		i.Message == nil || i.Message.UserMessageParam == nil ||
		len(i.Message.UserMessageParam.ContentParts) == 0 {
		return nil
	}

	for _, part := range i.Message.UserMessageParam.ContentParts {
		if part == nil {
			continue
		}

		// expand ref: [image](@medias/xxx) -> data
		matches := regMediaRef.FindStringSubmatch(part.ImageURL.URL)
		if len(matches) > 0 {
			refName := matches[1]
			actualData, err := media.LoadMedia(refName)
			if err == nil && len(actualData) > 0 {
				part.ImageURL.URL = string(actualData)
			}
		}
	}

	return nil
}

func (msg *LogItem) HasImageRef() bool {
	return msg.Metadata != nil && len(msg.Metadata.ImageRef) > 0
}

func (msg *LogItem) IsFromUser() bool {
	return msg.Role == schema.RoleUser
}

func (msg *LogItem) IsFromAssistant() bool {
	return msg.Role == schema.RoleAssistant
}

func (msg *LogItem) IsFromTool() bool {
	return msg.Role == schema.RoleTool
}

func (msg *LogItem) Json() string {
	c, _ := json.Marshal(msg)
	return string(c)
}
