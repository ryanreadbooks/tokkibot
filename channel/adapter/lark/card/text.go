package card

type TextElement struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

func NewTextElement(content string) *TextElement {
	return &TextElement{
		Tag:     string(TextTagPlainText),
		Content: content,
	}
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
