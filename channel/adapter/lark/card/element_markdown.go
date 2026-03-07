package card

type BodyMarkdownElement struct {
	Content   string    `json:"content,omitempty"`
	ElementId string    `json:"element_id,omitempty"`
	TextAlign TextAlign `json:"text_align,omitempty"`
}

var _ BodyElement = (*BodyMarkdownElement)(nil)

func (e *BodyMarkdownElement) Tag() string {
	return "markdown"
}

func (e *BodyMarkdownElement) MarshalJSON() ([]byte, error) {
	return messageCardElementJson(e)
}

func NewBodyMarkdownElement(content string) *BodyMarkdownElement {
	return &BodyMarkdownElement{
		Content: content,
	}
}

func (e *BodyMarkdownElement) WithElementId(elementId string) *BodyMarkdownElement {
	e.ElementId = elementId
	return e
}
