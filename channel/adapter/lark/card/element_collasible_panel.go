package card

type CollapsiblePanelElement struct {
	ElementId         string          `json:"element_id,omitempty"`
	Direction         Direction       `json:"direction,omitempty"`
	VerticalSpacing   Spacing         `json:"vertical_spacing,omitempty"`
	HorizontalSpacing Spacing         `json:"horizontal_spacing,omitempty"`
	VerticalAlign     VerticalAlign   `json:"vertical_align,omitempty"`
	HorizontalAlign   HorizontalAlign `json:"horizontal_align,omitempty"`
	Padding           string          `json:"padding,omitempty"`
	Expanded          bool            `json:"expanded,omitempty"`

	// See: https://open.feishu.cn/document/feishu-cards/enumerations-for-fields-related-to-color
	BackgroundColor string                  `json:"background_color,omitempty"`
	Header          *CollapsiblePanelHeader `json:"header,omitempty"`
	Border          *CollapsiblePanelBorder `json:"border,omitempty"`
	Elements        []BodyElement           `json:"elements,omitempty"`
}

type CollapsiblePanelWidth string

const (
	CollapsiblePanelWidthFill         CollapsiblePanelWidth = "fill"
	CollapsiblePanelWidthAuto         CollapsiblePanelWidth = "auto"
	CollapsiblePanelWidthAutoWhenFold CollapsiblePanelWidth = "auto_when_fold"
)

type CollapsiblePanelHeaderIcon struct {
	// See: https://open.feishu.cn/document/feishu-cards/enumerations-for-icons
	Tag      string `json:"tag,omitempty"` // standard_icon
	Token    string `json:"token,omitempty"`
	Color    string `json:"color,omitempty"`
	ImageKey string `json:"image_key,omitempty"`
	Size     string `json:"size,omitempty"`
}

type IconPosition string

const (
	IconPositionLeft       IconPosition = "left"
	IconPositionRight      IconPosition = "right"
	IconPositionFollowText IconPosition = "follow_text"
)

type IconExpandedAngle int

const (
	IconExpandedAngleMinus180 IconExpandedAngle = -180
	IconExpandedAngleMinus90  IconExpandedAngle = -90
	IconExpandedAngle90       IconExpandedAngle = 90
	IconExpandedAngle180      IconExpandedAngle = 180 // default
)

type CollapsiblePanelHeader struct {
	Title             *TextElement                `json:"title,omitempty"`
	BackgroudColor    string                      `json:"background_color,omitempty"`
	Width             CollapsiblePanelWidth       `json:"width,omitempty"`
	VerticalAlign     VerticalAlign               `json:"vertical_align,omitempty"`
	Padding           string                      `json:"padding,omitempty"`
	Position          string                      `json:"position,omitempty"` // top
	Icon              *CollapsiblePanelHeaderIcon `json:"icon,omitempty"`
	IconPosition      IconPosition                `json:"icon_position,omitempty"`
	IconExpandedAngle IconExpandedAngle           `json:"icon_expanded_angle,omitempty"`
}

type CollapsiblePanelBorder struct {
	Color        string `json:"color,omitempty"`
	CornerRadius string `json:"corner_radius,omitempty"`
}

var _ BodyElement = (*CollapsiblePanelElement)(nil)

func (e *CollapsiblePanelElement) Tag() string {
	return "collapsible_panel"
}

func (e *CollapsiblePanelElement) MarshalJSON() ([]byte, error) {
	return messageCardElementJson(e)
}

func NewCollapsiblePanelElement(elementId string) *CollapsiblePanelElement {
	return &CollapsiblePanelElement{
		ElementId:       elementId,
		Direction:       DirectionVertical,
		VerticalAlign:   VerticalAlignTop,
		HorizontalAlign: HorizontalAlignLeft,
	}
}

func (e *CollapsiblePanelElement) WithElementId(elementId string) *CollapsiblePanelElement {
	e.ElementId = elementId
	return e
}

func (e *CollapsiblePanelElement) WithExpanded(expanded bool) *CollapsiblePanelElement {
	e.Expanded = expanded
	return e
}

func (e *CollapsiblePanelElement) WithBorder(border *CollapsiblePanelBorder) *CollapsiblePanelElement {
	e.Border = border
	return e
}

func (e *CollapsiblePanelElement) WithHeaderTitle(title string) *CollapsiblePanelElement {
	if e.Header == nil {
		e.Header = &CollapsiblePanelHeader{}
	}

	e.Header.Title = NewTextElement(title)
	return e
}

// form组件不支持作为collapsible_panel子元素
func (e *CollapsiblePanelElement) AppendElement(element BodyElement) *CollapsiblePanelElement {
	e.Elements = append(e.Elements, element)
	return e
}

func (e *CollapsiblePanelElement) WithBackgroundColor(color string) *CollapsiblePanelElement {
	e.BackgroundColor = color
	return e
}
