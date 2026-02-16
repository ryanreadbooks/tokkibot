package tool

import (
	"context"
	"encoding/json"

	"github.com/ryanreadbooks/tokkibot/pkg/schema"
)

type Info struct {
	Name        string
	Description string
	Schema      *schema.Schema
}

// Invoker is the interface for all tools.
type Invoker interface {
	// Info returns the information about the tool.
	Info() Info

	// Invoke executes the tool with the given arguments and returns the result.
	//
	// The arguments is the JSON-encoded string of the arguments.
	Invoke(ctx context.Context, arguments string) (string, error)
}

type InvokeResult struct {
	Success bool   `json:"success"`
	Data    string `json:"data,omitempty"`
	Err     string `json:"err,omitempty"`
}

func (r *InvokeResult) Json() string {
	o, _ := json.Marshal(r)
	return string(o)
}
