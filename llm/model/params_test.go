package model

import (
	"encoding/json"
	"testing"
)

type GetWeatherInput struct {
	Lat  float64 `json:"lat" jsonschema:"description=The latitude of the location to check weather"`
	Long float64 `json:"long" jsonschema:"description=The longitude of the location to check weather"`
	Unit string  `json:"unit,omitempty" jsonschema:"description=Unit for the output"` // optional
}

func TestGetWeatherInputSchema(t *testing.T) {
	schema := GetToolInputSchemaParam[GetWeatherInput]()
	out, _ := json.MarshalIndent(schema.Map(), "", "  ")
	t.Log(string(out))
}

func TestNewToolParam(t *testing.T) {
	tool := NewToolParam[GetWeatherInput]("get_weather", "Get the weather for a given location")
	out, _ := json.MarshalIndent(tool, "", "  ")
	t.Log(string(out))
}
