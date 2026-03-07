package card

type BodyDivElement struct {
	ElementId string              `json:"element_id,omitempty"`
	Text      *BodyDivElementText `json:"text,omitempty"`
}

type BodyDivElementText struct {
	Tag     TextTag `json:"tag"`
	Content string  `json:"content"`
}

var _ BodyElement = (*BodyDivElement)(nil)

func (e *BodyDivElement) Tag() string {
	return "div"
}

func (e *BodyDivElement) MarshalJSON() ([]byte, error) {
	return messageCardElementJson(e)
}

func NewBodyDivElement(text string) *BodyDivElement {
	return &BodyDivElement{
		Text: &BodyDivElementText{
			Tag:     TextTagPlainText,
			Content: text,
		},
	}
}
