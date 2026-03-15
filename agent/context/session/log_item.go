package session

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jinzhu/copier"
	"github.com/ryanreadbooks/tokkibot/agent/ref/media"
	"github.com/ryanreadbooks/tokkibot/llm/schema/param"
)

func NewLogItemId() string {
	id := uuid.Must(uuid.NewV7())
	return strings.ReplaceAll(id.String(), "-", "")
}

type LogItem struct {
	Id       string         `json:"id"`
	Role     param.Role     `json:"role"`
	Created  int64          `json:"created"`
	Message  *param.Message `json:"message,omitzero"`
	Metadata *LogItemMeta   `json:"metadata,omitzero"`
}

type LogItemMeta struct {
	ImageRef map[int]string `json:"image_ref,omitzero"`
}

// MarshalJSON replaces inline image data with media refs for disk storage.
func (item *LogItem) MarshalJSON() ([]byte, error) {
	type alias LogItem

	if !item.Role.User() || !item.HasImageRef() {
		return json.Marshal((*alias)(item))
	}

	var copied LogItem
	copier.CopyWithOption(&copied, item, copier.Option{DeepCopy: true})

	if copied.Message != nil && copied.Message.User != nil &&
		len(copied.Message.User.ContentParts) > 0 {
		for idx, part := range copied.Message.User.ContentParts {
			if part == nil || part.ImageURL == nil {
				continue
			}
			if imageRefName, ok := copied.Metadata.ImageRef[idx]; ok {
				part.ImageURL.URL = fmt.Sprintf("[image](%s)", imageRefName)
			}
		}
	}

	return json.Marshal((*alias)(&copied))
}

// UnmarshalJSON expands media refs back to actual image data.
func (item *LogItem) UnmarshalJSON(data []byte) error {
	type alias LogItem

	if err := json.Unmarshal(data, (*alias)(item)); err != nil {
		return err
	}

	if !item.Role.User() || !item.HasImageRef() ||
		item.Message == nil || item.Message.User == nil ||
		len(item.Message.User.ContentParts) == 0 {
		return nil
	}

	for _, part := range item.Message.User.ContentParts {
		if part == nil || part.ImageURL == nil {
			continue
		}

		matches := regMediaRef.FindStringSubmatch(part.ImageURL.URL)
		if len(matches) > 0 {
			refName := matches[1]
			if actualData, err := media.LoadMedia(refName); err == nil && len(actualData) > 0 {
				part.ImageURL.URL = string(actualData)
			}
		}
	}

	return nil
}

func (item *LogItem) HasImageRef() bool {
	return item.Metadata != nil && len(item.Metadata.ImageRef) > 0
}

func (item *LogItem) IsFromUser() bool {
	return item.Role == param.RoleUser
}

func (item *LogItem) IsFromAssistant() bool {
	return item.Role == param.RoleAssistant
}

func (item *LogItem) IsFromTool() bool {
	return item.Role == param.RoleTool
}

func (item *LogItem) Json() string {
	c, _ := json.Marshal(item)
	return string(c)
}
