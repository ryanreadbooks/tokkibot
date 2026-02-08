package tool

import (
	"context"

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
