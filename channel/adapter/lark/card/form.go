package card

type BodyFormElement struct {
	ElementId         string          `json:"element_id,omitempty"`
	Direction         Direction       `json:"direction,omitempty"`
	Margin            string          `json:"margin,omitempty"`
	Padding           string          `json:"padding,omitempty"`
	HorizontalSpacing Spacing         `json:"horizontal_spacing,omitempty"`
	HorizontalAlign   HorizontalAlign `json:"horizontal_align,omitempty"`
	VerticalAlign     VerticalAlign   `json:"vertical_align,omitempty"`
	VerticalSpacing   Spacing         `json:"vertical_spacing,omitempty"`
	Name              string          `json:"name,omitempty"` // required
	Elements          []BodyElement   `json:"elements,omitempty"`
}

var _ BodyElement = (*BodyFormElement)(nil)

func (e *BodyFormElement) Tag() string {
	return "form"
}

func (e *BodyFormElement) MarshalJSON() ([]byte, error) {
	return messageCardElementJson(e)
}

type FormActionType string

const (
	FormActionTypeSubmit FormActionType = "submit"
	FormActionTypeReset  FormActionType = "reset"
)

func NewBodyFormElement(name string) *BodyFormElement {
	return &BodyFormElement{
		Name:            name,
		Direction:       DirectionVertical,
		HorizontalAlign: HorizontalAlignLeft,
		VerticalAlign:   VerticalAlignTop,
	}
}

func (e *BodyFormElement) WithElementId(elementId string) *BodyFormElement {
	e.ElementId = elementId
	return e
}

func (e *BodyFormElement) WithDirection(direction Direction) *BodyFormElement {
	e.Direction = direction
	return e
}

func (e *BodyFormElement) WithMargin(margin string) *BodyFormElement {
	e.Margin = margin
	return e
}

func (e *BodyFormElement) WithPadding(padding string) *BodyFormElement {
	e.Padding = padding
	return e
}

func (e *BodyFormElement) WithHorizontalSpacing(spacing Spacing) *BodyFormElement {
	e.HorizontalSpacing = spacing
	return e
}

func (e *BodyFormElement) WithHorizontalAlign(align HorizontalAlign) *BodyFormElement {
	e.HorizontalAlign = align
	return e
}

func (e *BodyFormElement) WithVerticalAlign(align VerticalAlign) *BodyFormElement {
	e.VerticalAlign = align
	return e
}

func (e *BodyFormElement) WithVerticalSpacing(spacing Spacing) *BodyFormElement {
	e.VerticalSpacing = spacing
	return e
}

func (e *BodyFormElement) WithName(name string) *BodyFormElement {
	e.Name = name
	return e
}

func (e *BodyFormElement) WithElements(elements ...BodyElement) *BodyFormElement {
	e.Elements = elements
	return e
}

func (e *BodyFormElement) AddElement(element BodyElement) *BodyFormElement {
	e.Elements = append(e.Elements, element)
	return e
}
