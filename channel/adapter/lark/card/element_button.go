package card

type ButtonType string

const (
	ButtonTypeDefault       ButtonType = "default"
	ButtonTypePrimary       ButtonType = "primary"
	ButtonTypeDanger        ButtonType = "danger"
	ButtonTypeText          ButtonType = "text"
	ButtonTypePrimaryText   ButtonType = "primary_text"
	ButtonTypeDangerText    ButtonType = "danger_text"
	ButtonTypePrimaryFilled ButtonType = "primary_filled"
	ButtonTypeDangerFilled  ButtonType = "danger_filled"
	ButtonTypeLaser         ButtonType = "laser"
)

type ButtonSize string

const (
	ButtonSizeTiny   ButtonSize = "tiny"
	ButtonSizeSmall  ButtonSize = "small"
	ButtonSizeMedium ButtonSize = "medium"
	ButtonSizeLarge  ButtonSize = "large"
)

type ButtonWidth string

const (
	ButtonWidthDefault ButtonWidth = "default"
	ButtonWidthFull    ButtonWidth = "full"
)

type ButtonText struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

type BehaviorType string

const (
	BehaviorTypeCallback BehaviorType = "callback"
	BehaviorTypeOpenUrl  BehaviorType = "open_url"
)

type Behavior struct {
	Type  BehaviorType      `json:"type"`
	Value map[string]string `json:"value,omitempty"`
}

type BodyButtonElement struct {
	ElementId      string         `json:"element_id,omitempty"`
	Type           ButtonType     `json:"type,omitempty"`
	Margin         string         `json:"margin,omitempty"`
	Size           ButtonSize     `json:"size,omitempty"`
	Width          ButtonWidth    `json:"width,omitempty"`
	Text           *ButtonText    `json:"text,omitempty"`
	Disabled       bool           `json:"disabled,omitempty"`
	Behaviors      []*Behavior    `json:"behaviors,omitempty"`
	Name           string         `json:"name,omitempty"`
	FormActionType FormActionType `json:"form_action_type,omitempty"`
}

var _ BodyElement = (*BodyButtonElement)(nil)

func (e *BodyButtonElement) Tag() string {
	return "button"
}

func (e *BodyButtonElement) MarshalJSON() ([]byte, error) {
	return messageCardElementJson(e)
}

func NewBodyButtonElement(text string) *BodyButtonElement {
	return &BodyButtonElement{
		Text: &ButtonText{
			Tag:     "plain_text",
			Content: text,
		},
	}
}

func (e *BodyButtonElement) WithElementId(elementId string) *BodyButtonElement {
	e.ElementId = elementId
	return e
}

func (e *BodyButtonElement) WithType(buttonType ButtonType) *BodyButtonElement {
	e.Type = buttonType
	return e
}

func (e *BodyButtonElement) WithSize(size ButtonSize) *BodyButtonElement {
	e.Size = size
	return e
}

func (e *BodyButtonElement) WithWidth(width ButtonWidth) *BodyButtonElement {
	e.Width = width
	return e
}

func (e *BodyButtonElement) WithText(text string) *BodyButtonElement {
	e.Text = &ButtonText{
		Tag:     "plain_text",
		Content: text,
	}
	return e
}

func (e *BodyButtonElement) WithBehavior(behavior *Behavior) *BodyButtonElement {
	e.Behaviors = []*Behavior{behavior}
	return e
}

func (e *BodyButtonElement) WithName(name string) *BodyButtonElement {
	e.Name = name
	return e
}

func (e *BodyButtonElement) WithFormActionType(formActionType FormActionType) *BodyButtonElement {
	e.FormActionType = formActionType
	return e
}

func (e *BodyButtonElement) WithDisabled(disabled bool) *BodyButtonElement {
	e.Disabled = disabled
	return e
}
