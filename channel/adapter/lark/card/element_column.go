package card

type ColumnWidth string

const (
	ColumnWidthAuto     ColumnWidth = "auto"
	ColumnWidthWeighted ColumnWidth = "weighted"
)

type Spacing string

const (
	SpacingSmall      Spacing = "small"
	SpacingMedium     Spacing = "medium"
	SpacingLarge      Spacing = "large"
	SpacingExtraLarge Spacing = "extra_large"
)

type HorizontalAlign string

const (
	HorizontalAlignLeft   HorizontalAlign = "left"
	HorizontalAlignCenter HorizontalAlign = "center"
	HorizontalAlignRight  HorizontalAlign = "right"
)

type VerticalAlign string

const (
	VerticalAlignTop    VerticalAlign = "top"
	VerticalAlignCenter VerticalAlign = "center"
	VerticalAlignBottom VerticalAlign = "bottom"
)

type Direction string

const (
	DirectionVertical   Direction = "vertical"
	DirectionHorizontal Direction = "horizontal"
)

type FlexMode string

const (
	FlexModeDefault FlexMode = "none"
	FlexModeStretch FlexMode = "stretch"
	FlexModeFlow    FlexMode = "flow"
	FlexModeBisect  FlexMode = "bisect"
	FlexModeTrisect FlexMode = "trisect"
)

type ColumnAction struct {
	MultiUrl *MultiUrl `json:"multi_url,omitempty"`
}

type MultiUrl struct {
	Url        string `json:"url,omitempty"`
	AndroidUrl string `json:"android_url,omitempty"`
	IosUrl     string `json:"ios_url,omitempty"`
	PcUrl      string `json:"pc_url,omitempty"`
}

// BodyColumnSetElement is the column_set container element
type BodyColumnSetElement struct {
	ElementId         string           `json:"element_id,omitempty"`
	HorizontalSpacing Spacing          `json:"horizontal_spacing,omitempty"`
	HorizontalAlign   HorizontalAlign  `json:"horizontal_align,omitempty"`
	Margin            string           `json:"margin,omitempty"`
	FlexMode          FlexMode         `json:"flex_mode,omitempty"`
	BackgroundStyle   string           `json:"background_style,omitempty"`
	Action            *ColumnAction    `json:"action,omitempty"`
	Columns           []*ColumnElement `json:"columns,omitempty"`
}

var _ BodyElement = (*BodyColumnSetElement)(nil)

func (e *BodyColumnSetElement) Tag() string {
	return "column_set"
}

func (e *BodyColumnSetElement) MarshalJSON() ([]byte, error) {
	return messageCardElementJson(e)
}

func NewBodyColumnSetElement() *BodyColumnSetElement {
	return &BodyColumnSetElement{}
}

func (e *BodyColumnSetElement) WithElementId(elementId string) *BodyColumnSetElement {
	e.ElementId = elementId
	return e
}

func (e *BodyColumnSetElement) WithHorizontalSpacing(spacing Spacing) *BodyColumnSetElement {
	e.HorizontalSpacing = spacing
	return e
}

func (e *BodyColumnSetElement) WithHorizontalAlign(align HorizontalAlign) *BodyColumnSetElement {
	e.HorizontalAlign = align
	return e
}

func (e *BodyColumnSetElement) WithMargin(margin string) *BodyColumnSetElement {
	e.Margin = margin
	return e
}

func (e *BodyColumnSetElement) WithFlexMode(mode FlexMode) *BodyColumnSetElement {
	e.FlexMode = mode
	return e
}

func (e *BodyColumnSetElement) WithBackgroundStyle(style string) *BodyColumnSetElement {
	e.BackgroundStyle = style
	return e
}

func (e *BodyColumnSetElement) WithAction(action *ColumnAction) *BodyColumnSetElement {
	e.Action = action
	return e
}

func (e *BodyColumnSetElement) WithColumns(columns ...*ColumnElement) *BodyColumnSetElement {
	e.Columns = columns
	return e
}

func (e *BodyColumnSetElement) AddColumn(column *ColumnElement) *BodyColumnSetElement {
	e.Columns = append(e.Columns, column)
	return e
}

// ColumnElement is the column element inside column_set
type ColumnElement struct {
	Tag               string          `json:"tag"`
	ElementId         string          `json:"element_id,omitempty"`
	BackgroundStyle   string          `json:"background_style,omitempty"`
	Width             ColumnWidth     `json:"width,omitempty"`
	Weight            int             `json:"weight,omitempty"`
	HorizontalSpacing Spacing         `json:"horizontal_spacing,omitempty"`
	HorizontalAlign   HorizontalAlign `json:"horizontal_align,omitempty"`
	VerticalAlign     VerticalAlign   `json:"vertical_align,omitempty"`
	VerticalSpacing   Spacing         `json:"vertical_spacing,omitempty"`
	Direction         Direction       `json:"direction,omitempty"`
	Padding           string          `json:"padding,omitempty"`
	Margin            string          `json:"margin,omitempty"`
	Elements          []BodyElement   `json:"elements,omitempty"`
	Action            *ColumnAction   `json:"action,omitempty"`
}

func NewColumnElement() *ColumnElement {
	return &ColumnElement{
		Tag: "column",
	}
}

func (e *ColumnElement) WithElementId(elementId string) *ColumnElement {
	e.ElementId = elementId
	return e
}

func (e *ColumnElement) WithBackgroundStyle(style string) *ColumnElement {
	e.BackgroundStyle = style
	return e
}

func (e *ColumnElement) WithWidth(width ColumnWidth) *ColumnElement {
	e.Width = width
	return e
}

func (e *ColumnElement) WithWeight(weight int) *ColumnElement {
	e.Weight = weight
	return e
}

func (e *ColumnElement) WithHorizontalSpacing(spacing Spacing) *ColumnElement {
	e.HorizontalSpacing = spacing
	return e
}

func (e *ColumnElement) WithHorizontalAlign(align HorizontalAlign) *ColumnElement {
	e.HorizontalAlign = align
	return e
}

func (e *ColumnElement) WithVerticalAlign(align VerticalAlign) *ColumnElement {
	e.VerticalAlign = align
	return e
}

func (e *ColumnElement) WithVerticalSpacing(spacing Spacing) *ColumnElement {
	e.VerticalSpacing = spacing
	return e
}

func (e *ColumnElement) WithDirection(direction Direction) *ColumnElement {
	e.Direction = direction
	return e
}

func (e *ColumnElement) WithPadding(padding string) *ColumnElement {
	e.Padding = padding
	return e
}

func (e *ColumnElement) WithMargin(margin string) *ColumnElement {
	e.Margin = margin
	return e
}

func (e *ColumnElement) WithElements(elements ...BodyElement) *ColumnElement {
	e.Elements = elements
	return e
}

func (e *ColumnElement) AddElement(element BodyElement) *ColumnElement {
	e.Elements = append(e.Elements, element)
	return e
}

func (e *ColumnElement) WithAction(action *ColumnAction) *ColumnElement {
	e.Action = action
	return e
}

// ColumnAction methods

func NewColumnAction(url string) *ColumnAction {
	return &ColumnAction{
		MultiUrl: &MultiUrl{
			Url: url,
		},
	}
}

func (a *ColumnAction) WithAndroidUrl(url string) *ColumnAction {
	a.MultiUrl.AndroidUrl = url
	return a
}

func (a *ColumnAction) WithIosUrl(url string) *ColumnAction {
	a.MultiUrl.IosUrl = url
	return a
}

func (a *ColumnAction) WithPcUrl(url string) *ColumnAction {
	a.MultiUrl.PcUrl = url
	return a
}
