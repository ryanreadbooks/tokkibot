package card

import (
	"encoding/json"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

const (
	CardV2SchemaVersion = "2.0"
)

func messageCardElementJson(e CardV2BodyElement) ([]byte, error) {
	data, err := larkcore.StructToMap(e)
	if err != nil {
		return nil, err
	}
	data["tag"] = e.Tag()
	return json.Marshal(data)
}

type CardV2 struct {
	Schema string        `json:"schema"`
	Config *CardV2Config `json:"config,omitempty"`
	Header *CardV2Header `json:"header,omitempty"`
	Body   *CardV2Body   `json:"body,omitempty"`
}

type CardV2Config struct {
	StreamingMode bool `json:"streaming_mode,omitempty"`
}

type TextTag string

const (
	TextTagPlainText TextTag = "plain_text"
	TextTagLarkMd    TextTag = "lark_md"
)

type TextAlign string

const (
	TextAlignLeft   TextAlign = "left"
	TextAlignCenter TextAlign = "center"
	TextAlignRight  TextAlign = "right"
)

type CardV2Header struct {
	Title    *CardV2HeaderTitle    `json:"title,omitempty"`
	Subtitle *CardV2HeaderSubtitle `json:"subtitle,omitempty"`
}

type CardV2HeaderTitle struct {
	Tag     TextTag `json:"tag"`     // plain_text or lark_md
	Content string  `json:"content"` // title content
}

type CardV2HeaderSubtitle struct {
	Tag     TextTag `json:"tag"`     // plain_text or lark_md
	Content string  `json:"content"` // subtitle content
}

type CardV2Body struct {
	Elements []CardV2BodyElement `json:"elements,omitempty"`
}

type CardV2BodyElement interface {
	Tag() string
	MarshalJSON() ([]byte, error)
}

type CardV2BodyDivElement struct {
	ElementId string                    `json:"element_id,omitempty"`
	Text      *CardV2BodyDivElementText `json:"text,omitempty"`
}

type CardV2BodyDivElementText struct {
	Tag     TextTag `json:"tag"`     // plain_text or lark_md
	Content string  `json:"content"` // text content
}

func (e *CardV2BodyDivElement) Tag() string {
	return "div"
}

func (e *CardV2BodyDivElement) MarshalJSON() ([]byte, error) {
	return messageCardElementJson(e)
}

type CardV2BodyMarkdownElement struct {
	Content   string    `json:"content,omitempty"`
	ElementId string    `json:"element_id,omitempty"`
	TextAlign TextAlign `json:"text_align,omitempty"`
}

func (e *CardV2BodyMarkdownElement) Tag() string {
	return "markdown"
}

func (e *CardV2BodyMarkdownElement) MarshalJSON() ([]byte, error) {
	return messageCardElementJson(e)
}

type CardV2Builder struct {
	card *CardV2
}

func NewCardV2Builder() *CardV2Builder {
	return &CardV2Builder{
		card: &CardV2{
			Schema: CardV2SchemaVersion,
		},
	}
}

func (b *CardV2Builder) Build() *CardV2 {
	return b.card
}

func (b *CardV2Builder) SetHeaderTitle(title string) *CardV2Builder {
	if b.card.Header == nil {
		b.card.Header = &CardV2Header{}
	}
	b.card.Header.Title = &CardV2HeaderTitle{
		Tag:     TextTagPlainText,
		Content: title,
	}
	return b
}

func (b *CardV2Builder) SetHeaderSubtitle(subtitle string) *CardV2Builder {
	if b.card.Header == nil {
		b.card.Header = &CardV2Header{}
	}
	b.card.Header.Subtitle = &CardV2HeaderSubtitle{
		Tag:     TextTagPlainText,
		Content: subtitle,
	}
	return b
}

func (b *CardV2Builder) AppendBodyElement(element CardV2BodyElement) *CardV2Builder {
	if b.card.Body == nil {
		b.card.Body = &CardV2Body{}
	}
	b.card.Body.Elements = append(b.card.Body.Elements, element)
	return b
}

func NewCardV2BodyMarkdownElement(content string) *CardV2BodyMarkdownElement {
	return &CardV2BodyMarkdownElement{
		Content: content,
	}
}
