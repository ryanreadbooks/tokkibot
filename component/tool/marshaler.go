package tool

import (
	"context"
	"encoding/json"
)

// Unmarshal the arguments string into the desired type
type ArgumentUnMarshaler func(ctx context.Context, arguments string) (any, error)

// Marshal the output into a string
type OutputMarshaler func(ctx context.Context, output any) (string, error)

func defaultOutputMarshal(output any) (string, error) {
	if s, ok := output.(string); ok {
		return s, nil
	}

	b, err := json.Marshal(output)
	return string(b), err
}
