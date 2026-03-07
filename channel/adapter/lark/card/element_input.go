package card

type InputType string

const (
	InputTypeText          InputType = "text"
	InputTypeMultilineText InputType = "multiline_text"
	InputTypePassword      InputType = "password"
)

type TextWidth string

const (
	TextWidthDefault TextWidth = "default"
	TextWidthFill    TextWidth = "fill"
)

type BodyInputElement struct {
	ElementId   string       `json:"element_id,omitempty"`
	Margin      string       `json:"margin,omitempty"`
	Name        string       `json:"name,omitempty"`
	Disabled    bool         `json:"disabled,omitempty"`
	Placeholder *TextElement `json:"placeholder,omitempty"`
	Width       TextWidth    `json:"width,omitempty"`
	MaxLength   int          `json:"max_length,omitempty"`
	InputType   InputType    `json:"input_type,omitempty"`
	ShowIcon    bool         `json:"show_icon,omitempty"`
	Rows        int          `json:"rows,omitempty"`
	AutoResize  bool         `json:"auto_resize,omitempty"`
	MaxRows     int          `json:"max_rows,omitempty"`
	Value       string       `json:"value,omitempty"`
}

var _ BodyElement = (*BodyInputElement)(nil)

func (e *BodyInputElement) Tag() string {
	return "input"
}

func (e *BodyInputElement) MarshalJSON() ([]byte, error) {
	return messageCardElementJson(e)
}

func NewBodyInputElement() *BodyInputElement {
	return &BodyInputElement{
		InputType: InputTypeText,
	}
}

func (e *BodyInputElement) WithElementId(elementId string) *BodyInputElement {
	e.ElementId = elementId
	return e
}

func (e *BodyInputElement) WithMargin(margin string) *BodyInputElement {
	e.Margin = margin
	return e
}

func (e *BodyInputElement) WithName(name string) *BodyInputElement {
	e.Name = name
	return e
}

func (e *BodyInputElement) WithDisabled(disabled bool) *BodyInputElement {
	e.Disabled = disabled
	return e
}

func (e *BodyInputElement) WithPlaceholder(placeholder string) *BodyInputElement {
	e.Placeholder = NewTextElement(placeholder)
	return e
}

func (e *BodyInputElement) WithWidth(width TextWidth) *BodyInputElement {
	e.Width = width
	return e
}

func (e *BodyInputElement) WithMaxLength(maxLength int) *BodyInputElement {
	e.MaxLength = maxLength
	return e
}

func (e *BodyInputElement) WithInputType(inputType InputType) *BodyInputElement {
	e.InputType = inputType
	return e
}

func (e *BodyInputElement) WithShowIcon(showIcon bool) *BodyInputElement {
	e.ShowIcon = showIcon
	return e
}

func (e *BodyInputElement) WithRows(rows int) *BodyInputElement {
	e.Rows = rows
	return e
}

func (e *BodyInputElement) WithAutoResize(autoResize bool) *BodyInputElement {
	e.AutoResize = autoResize
	return e
}

func (e *BodyInputElement) WithMaxRows(maxRows int) *BodyInputElement {
	e.MaxRows = maxRows
	return e
}

func (e *BodyInputElement) WithValue(value string) *BodyInputElement {
	e.Value = value
	return e
}
