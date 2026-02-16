package model

import (
	"github.com/ryanreadbooks/tokkibot/pkg/schema"
)

type StringParam struct {
	Value string
}

type TextParam struct {
	Value string
}

type ContentUnionParam struct {
	Text     *TextParam
	ImageURL *ImageURLParam
}

type ToolCallFunctionParam struct {
	Id        string
	Name      string
	Arguments string
}

type ToolCallParam struct {
	Function *ToolCallFunctionParam
}

type ImageURLParam struct {
	URL string
}

type SystemMessageParam struct {
	String *StringParam
	Text   []*TextParam
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
	String       *StringParam
	ContentParts []*ContentUnionParam
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
	Content          *StringParam
	Texts            []*TextParam
	ToolCalls        []*ToolCallParam
	ReasoningContent *StringParam
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
	ToolCallId string
	String     *StringParam
	Texts      []*TextParam
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
	SystemMessageParam    *SystemMessageParam
	UserMessageParam      *UserMessageParam
	AssistantMessageParam *AssistantMessageParam
	ToolMessageParam      *ToolMessageParam
}

type ToolDefinitionParam struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ToolParam struct {
	Definition *ToolDefinitionParam `json:"definition"`
	Parameters map[string]any       `json:"parameters"`
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
