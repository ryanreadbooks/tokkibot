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

type HeaderTemplate string

// https://open.feishu.cn/document/feishu-cards/card-json-v2-components/content-components/title#06994f37
const (
	HeaderTemplateBlue      HeaderTemplate = "blue"
	HeaderTemplateWathet    HeaderTemplate = "wathet"
	HeaderTemplateTurquoise HeaderTemplate = "turquoise"
	HeaderTemplateGreen     HeaderTemplate = "green"
	HeaderTemplateYellow    HeaderTemplate = "yellow"
	HeaderTemplateOrange    HeaderTemplate = "orange"
	HeaderTemplateRed       HeaderTemplate = "red"
	HeaderTemplateCarmine   HeaderTemplate = "carmine"
	HeaderTemplateViolet    HeaderTemplate = "violet"
	HeaderTemplatePurple    HeaderTemplate = "purple"
	HeaderTemplateIndigo    HeaderTemplate = "indigo"
	HeaderTemplateGrey      HeaderTemplate = "grey"
	HeaderTemplateDefault   HeaderTemplate = "default"
)

type Header struct {
	Title    *HeaderTitle    `json:"title,omitempty"`
	Subtitle *HeaderSubtitle `json:"subtitle,omitempty"`
	Template HeaderTemplate  `json:"template,omitempty"`
}

type HeaderTitle struct {
	Tag     TextTag `json:"tag"`
	Content string  `json:"content"`
}

type HeaderSubtitle struct {
	Tag     TextTag `json:"tag"`
	Content string  `json:"content"`
}

type Body struct {
	Elements []BodyElement `json:"elements,omitempty"`
}

type BodyElement interface {
	Tag() string
	MarshalJSON() ([]byte, error)
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

func (b *CardV2Builder) WithHeaderTitle(title string) *CardV2Builder {
	if b.card.Header == nil {
		b.card.Header = &Header{}
	}
	b.card.Header.Title = &HeaderTitle{
		Tag:     TextTagPlainText,
		Content: title,
	}
	return b
}

func (b *CardV2Builder) WithHeaderTemplate(template HeaderTemplate) *CardV2Builder {
	if b.card.Header == nil {
		b.card.Header = &Header{}
	}
	b.card.Header.Template = template
	return b
}

func (b *CardV2Builder) WithHeaderSubtitle(subtitle string) *CardV2Builder {
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
