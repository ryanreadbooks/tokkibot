package schema

import "github.com/invopop/jsonschema"

type Schema struct {
	Properties           any      `json:"properties,omitempty"`
	Required             []string `json:"required,omitempty"`
	AdditionalProperties any      `json:"additionalProperties,omitempty"`
}

func (s Schema) Ptr() *Schema {
	return &s
}

// Usage: see https://github.com/invopop/jsonschema?tab=readme-ov-file
func Get[T any]() Schema {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}

	var v T
	schema := reflector.Reflect(v)
	if schema.Required == nil {
		schema.Required = []string{}
	}

	return Schema{
		Properties:           schema.Properties,
		Required:             schema.Required,
		AdditionalProperties: schema.AdditionalProperties,
	}
}
