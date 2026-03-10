package param

import (
	"encoding/json"

	"github.com/ryanreadbooks/tokkibot/pkg/schema"
)

type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (p *ToolDefinition) GetContent() string {
	if p != nil {
		return p.Name + p.Description
	}

	return ""
}

type Tool struct {
	Definition *ToolDefinition `json:"definition"`
	Parameters map[string]any  `json:"parameters"`

	InputSchema ToolInputSchema `json:"-"` // for internal use
}

func (t *Tool) GetContent() string {
	if t != nil {
		tp, _ := json.Marshal(t.Parameters)
		return t.Definition.GetContent() + string(tp)
	}

	return ""
}

func NewTool[InputT any](name, description string) Tool {
	inputSchema := GetToolInputSchema[InputT]()
	return Tool{
		Definition: &ToolDefinition{
			Name:        name,
			Description: description,
		},
		Parameters:  inputSchema.Map(),
		InputSchema: inputSchema,
	}
}

func NewToolWithSchema(name, description string, sch schema.Schema) Tool {
	inputSchema := ToolInputSchema{
		Properties:           sch.Properties,
		Required:             sch.Required,
		AdditionalProperties: sch.AdditionalProperties,
	}
	return Tool{
		Definition: &ToolDefinition{
			Name:        name,
			Description: description,
		},
		Parameters:  inputSchema.Map(),
		InputSchema: inputSchema,
	}
}

func GetToolInputSchema[T any]() ToolInputSchema {
	sch := schema.Get[T]()
	return ToolInputSchema{
		Properties: sch.Properties,
		Required:   sch.Required,
	}
}

type ToolInputSchema struct {
	Properties           any      `json:"properties"`
	Required             []string `json:"required"`
	AdditionalProperties any      `json:"additionalProperties,omitempty"`
}

func (m ToolInputSchema) Map() map[string]any {
	mm := map[string]any{
		"type":       "object",
		"properties": m.Properties,
		"required":   m.Required,
	}

	if m.AdditionalProperties != nil {
		mm["additionalProperties"] = m.AdditionalProperties
	}

	return mm
}
