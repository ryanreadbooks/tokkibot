package schema

import (
	"encoding/json"
	"strings"

	"github.com/ryanreadbooks/tokkibot/pkg/schema"
)

type StringParam struct {
	Value string `json:"value"`
}

func (p *StringParam) GetValue() string {
	if p != nil {
		return p.Value
	}

	return ""
}

type TextParam struct {
	Value string `json:"value"`
}

func (p *TextParam) GetValue() string {
	if p != nil {
		return p.Value
	}

	return ""
}

func TextParamsContent(texts []*TextParam) string {
	var vv strings.Builder
	for _, t := range texts {
		vv.WriteString(t.GetValue())
	}

	return vv.String()
}

type ContentUnionParam struct {
	Text     *TextParam     `json:"text,omitzero"`
	ImageURL *ImageURLParam `json:"image_url,omitzero"`
}

func (p *ContentUnionParam) GetContent() string {
	return p.Text.GetValue() + p.ImageURL.GetURL()
}

func ContentUnionParamsContent(c []*ContentUnionParam) string {
	var vv strings.Builder
	for _, u := range c {
		vv.WriteString(u.GetContent())
	}

	return vv.String()
}

type ToolCallFunctionParam struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (p *ToolCallFunctionParam) GetContent() string {
	if p != nil {
		return p.Id + p.Name + p.Arguments
	}

	return ""
}

type ToolCallParam struct {
	Function *ToolCallFunctionParam `json:"function,omitzero"`
}

func (p *ToolCallParam) GetContent() string {
	return p.Function.GetContent()
}

func ToolCallParamsContent(tcs []*ToolCallParam) string {
	var vv strings.Builder
	for _, tc := range tcs {
		vv.WriteString(tc.GetContent())
	}

	return vv.String()
}

type ImageURLParam struct {
	URL string
}

func (p *ImageURLParam) GetURL() string {
	if p != nil {
		return p.URL
	}

	return ""
}

type SystemMessageParam struct {
	String *StringParam `json:"string,omitzero"`
	Text   []*TextParam `json:"text,omitzero"`
}

func (p *SystemMessageParam) GetContent() string {
	if p == nil {
		return ""
	}

	if c := p.String.GetValue(); c != "" {
		return c
	}

	return TextParamsContent(p.Text)
}

func NewSystemMessageParam[T string | []*TextParam](text T) MessageParam {
	sys := SystemMessageParam{}
	switch v := any(text).(type) {
	case string:
		sys.String = &StringParam{Value: v}
	case []*TextParam:
		sys.Text = v
	}

	return MessageParam{
		SystemMessageParam: &sys,
	}
}

type UserMessageParam struct {
	String       *StringParam         `json:"string,omitzero"`
	ContentParts []*ContentUnionParam `json:"content_parts,omitzero"`
}

func (p *UserMessageParam) GetContent() string {
	if p == nil {
		return ""
	}

	if c := p.String.GetValue(); c != "" {
		return c
	}

	return ContentUnionParamsContent(p.ContentParts)
}

func NewUserMessageParam[T string | []*ContentUnionParam](msg T) MessageParam {
	user := UserMessageParam{}
	switch v := any(msg).(type) {
	case string:
		user.String = &StringParam{Value: v}
	case []*ContentUnionParam:
		user.ContentParts = v
	}

	return MessageParam{
		UserMessageParam: &user,
	}
}

type AssistantMessageParam struct {
	Content          *StringParam     `json:"content,omitzero"`
	Texts            []*TextParam     `json:"texts,omitzero"`
	ToolCalls        []*ToolCallParam `json:"tool_calls,omitzero"`
	ReasoningContent *StringParam     `json:"reasoning_content,omitzero"`
}

func (p *AssistantMessageParam) GetContent() string {
	if p == nil {
		return ""
	}

	return p.Content.GetValue() +
		p.ReasoningContent.GetValue() +
		TextParamsContent(p.Texts) +
		ToolCallParamsContent(p.ToolCalls)
}

func NewAssistantMessageParam[T string | []*TextParam](
	msg T,
	toolCalls []*ToolCallParam,
	reasoningContent *StringParam,
) MessageParam {
	assistant := AssistantMessageParam{}
	switch v := any(msg).(type) {
	case string:
		assistant.Content = &StringParam{Value: v}
	case []*TextParam:
		assistant.Texts = v
	}

	assistant.ToolCalls = toolCalls
	assistant.ReasoningContent = reasoningContent

	return MessageParam{
		AssistantMessageParam: &assistant,
	}
}

type ToolMessageParam struct {
	ToolCallId string       `json:"tool_call_id"`
	String     *StringParam `json:"string,omitzero"`
	Texts      []*TextParam `json:"texts,omitzero"`
}

func (p *ToolMessageParam) GetContent() string {
	if p == nil {
		return ""
	}

	return p.ToolCallId + p.String.GetValue() + TextParamsContent(p.Texts)
}

// Tool call result message
func NewToolMessageParam[T string | []*TextParam](toolCallId string, msg T) MessageParam {
	tool := ToolMessageParam{
		ToolCallId: toolCallId,
	}
	switch v := any(msg).(type) {
	case string:
		tool.String = &StringParam{Value: v}
	case []*TextParam:
		tool.Texts = v
	}

	return MessageParam{
		ToolMessageParam: &tool,
	}
}

type MessageParam struct {
	SystemMessageParam    *SystemMessageParam    `json:"systen,omitzero"`
	UserMessageParam      *UserMessageParam      `json:"user,omitzero"`
	AssistantMessageParam *AssistantMessageParam `json:"assistant,omitzero"`
	ToolMessageParam      *ToolMessageParam      `json:"tool,omitzero"`
}

func (p *MessageParam) Role() Role {
	if p.SystemMessageParam != nil {
		return RoleSystem
	}

	if p.UserMessageParam != nil {
		return RoleUser
	}

	if p.AssistantMessageParam != nil {
		return RoleAssistant
	}

	if p.ToolMessageParam != nil {
		return RoleTool
	}

	return ""
}

type ToolDefinitionParam struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (p *ToolDefinitionParam) GetContent() string {
	if p != nil {
		return p.Name + p.Description
	}

	return ""
}

type ToolParam struct {
	Definition *ToolDefinitionParam `json:"definition"`
	Parameters map[string]any       `json:"parameters"`
}

func (t *ToolParam) GetContent() string {
	if t != nil {
		tp, _ := json.Marshal(t.Parameters)
		return t.Definition.GetContent() + string(tp)
	}

	return ""
}

func NewToolParam[InputT any](name, description string) ToolParam {
	return ToolParam{
		Definition: &ToolDefinitionParam{
			Name:        name,
			Description: description,
		},
		Parameters: GetToolInputSchemaParam[InputT]().Map(),
	}
}

func NewToolParamWithSchemaParam(name, description string, schemaParam schema.Schema) ToolParam {
	return ToolParam{
		Definition: &ToolDefinitionParam{
			Name:        name,
			Description: description,
		},
		Parameters: toolInputSchemaParam{
			Properties: schemaParam.Properties,
			Required:   schemaParam.Required,
		}.Map(),
	}
}

func GetToolInputSchemaParam[T any]() toolInputSchemaParam {
	sch := schema.Get[T]()
	return toolInputSchemaParam{
		Properties: sch.Properties,
		Required:   sch.Required,
	}
}

type toolInputSchemaParam struct {
	// Type string // always "object"
	Properties any      `json:"properties"`
	Required   []string `json:"required"`
}

func (m toolInputSchemaParam) Map() map[string]any {
	return map[string]any{
		"type":       "object", // always "object"
		"properties": m.Properties,
		"required":   m.Required,
	}
}
