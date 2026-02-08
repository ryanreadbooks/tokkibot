package schema

import "github.com/invopop/jsonschema"

type Schema struct {
	Properties any
	Required   []string
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
	return Schema{
		Properties: schema.Properties,
		Required:   schema.Required,
	}
}
