package card

import (
	"encoding/json"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

const (
	SchemaVersion = "2.0"
)

func messageCardElementJson(e BodyElement) ([]byte, error) {
	data, err := larkcore.StructToMap(e)
	if err != nil {
		return nil, err
	}
	data["tag"] = e.Tag()
	return json.Marshal(data)
}

type CardV2 struct {
	Schema string  `json:"schema"`
	Config *Config `json:"config,omitempty"`
	Header *Header `json:"header,omitempty"`
	Body   *Body   `json:"body,omitempty"`
}

type StreamingConfig struct {
	PrintFrequencyMs *StreamingConfigPrintFrequencyMs `json:"print_frequency_ms,omitempty"`
	PrintStep        *StreamingConfigPrintStep        `json:"print_step,omitempty"`
	PrintStrategy    StreamingConfigPrintStrategy     `json:"print_strategy,omitempty"`
}

type StreamingConfigPrintFrequencyMs struct {
	Default int `json:"default"`
	Android int `json:"android"`
	Ios     int `json:"ios"`
	Pc      int `json:"pc"`
}

type StreamingConfigPrintStep struct {
	Default int `json:"default"`
	Android int `json:"android"`
	Ios     int `json:"ios"`
	Pc      int `json:"pc"`
}

type StreamingConfigPrintStrategy string

const (
	StreamingConfigPrintStrategyFast  StreamingConfigPrintStrategy = "fast"
	StreamingConfigPrintStrategyDelay StreamingConfigPrintStrategy = "delay"
)

type Summary struct {
	Content string `json:"content"`
}

type Config struct {
	Summary         *Summary         `json:"summary,omitempty"`
	StreamingMode   bool             `json:"streaming_mode,omitempty"`
	StreamingConfig *StreamingConfig `json:"streaming_config,omitempty"`
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

type Header struct {
	Title    *HeaderTitle    `json:"title,omitempty"`
	Subtitle *HeaderSubtitle `json:"subtitle,omitempty"`
}

type HeaderTitle struct {
	Tag     TextTag `json:"tag"`     // plain_text or lark_md
	Content string  `json:"content"` // title content
}

type HeaderSubtitle struct {
	Tag     TextTag `json:"tag"`     // plain_text or lark_md
	Content string  `json:"content"` // subtitle content
}

type Body struct {
	Elements []BodyElement `json:"elements,omitempty"`
}

type BodyElement interface {
	Tag() string
	MarshalJSON() ([]byte, error)
}

type BodyDivElement struct {
	ElementId string              `json:"element_id,omitempty"`
	Text      *BodyDivElementText `json:"text,omitempty"`
}

type BodyDivElementText struct {
	Tag     TextTag `json:"tag"`     // plain_text or lark_md
	Content string  `json:"content"` // text content
}

func (e *BodyDivElement) Tag() string {
	return "div"
}

func (e *BodyDivElement) MarshalJSON() ([]byte, error) {
	return messageCardElementJson(e)
}

type BodyMarkdownElement struct {
	Content   string    `json:"content,omitempty"`
	ElementId string    `json:"element_id,omitempty"`
	TextAlign TextAlign `json:"text_align,omitempty"`
}

func (e *BodyMarkdownElement) Tag() string {
	return "markdown"
}

func (e *BodyMarkdownElement) MarshalJSON() ([]byte, error) {
	return messageCardElementJson(e)
}

type CardV2Builder struct {
	card *CardV2
}

func NewCardV2Builder() *CardV2Builder {
	return &CardV2Builder{
		card: &CardV2{
			Schema: SchemaVersion,
		},
	}
}

func (b *CardV2Builder) Build() *CardV2 {
	return b.card
}

func (b *CardV2Builder) SetHeaderTitle(title string) *CardV2Builder {
	if b.card.Header == nil {
		b.card.Header = &Header{}
	}
	b.card.Header.Title = &HeaderTitle{
		Tag:     TextTagPlainText,
		Content: title,
	}
	return b
}

func (b *CardV2Builder) SetHeaderSubtitle(subtitle string) *CardV2Builder {
	if b.card.Header == nil {
		b.card.Header = &Header{}
	}
	b.card.Header.Subtitle = &HeaderSubtitle{
		Tag:     TextTagPlainText,
		Content: subtitle,
	}
	return b
}

func (b *CardV2Builder) AppendBodyElement(element BodyElement) *CardV2Builder {
	if b.card.Body == nil {
		b.card.Body = &Body{}
	}
	b.card.Body.Elements = append(b.card.Body.Elements, element)
	return b
}

func NewBodyMarkdownElement(content string) *BodyMarkdownElement {
	return &BodyMarkdownElement{
		Content: content,
	}
}

func (e *BodyMarkdownElement) SetElementId(elementId string) *BodyMarkdownElement {
	e.ElementId = elementId
	return e
}