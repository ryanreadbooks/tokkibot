package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ryanreadbooks/tokkibot/pkg/schema"
)

type InvokerFunc[T, O any] func(ctx context.Context, input T) (O, error)

type invoker[T, O any] struct {
	info Info
	fn   InvokerFunc[T, O]

	// for future use maybe
	customArgumentUnMarshaler ArgumentUnMarshaler
	customOutputMarshaler     OutputMarshaler
}

func (t *invoker[T, O]) Info() Info {
	return t.info
}

func (t *invoker[T, O]) Invoke(ctx context.Context, arguments string) (string, error) {
	var (
		input T
		err   error
	)

	if t.customArgumentUnMarshaler != nil {
		var val any
		val, err = t.customArgumentUnMarshaler(ctx, arguments)
		if err != nil {
			return "", fmt.Errorf("tool arguments unmarshal failed, tool_name=%s, error=%w", t.info.Name, err)
		}

		// type assertion to desired type
		if tmp, ok := val.(T); ok {
			input = tmp
		} else {
			return "", fmt.Errorf("tool arguments unmarshal failed, tool_name=%s, type=%T, expected=%T", t.info.Name, val, input)
		}
	} else {
		err = json.Unmarshal([]byte(arguments), &input)
	}

	if err != nil {
		return "", fmt.Errorf("tool arguments unmarshal json failed, tool_name=%s, error=%w", t.info.Name, err)
	}

	// invoke the function
	output, err := t.fn(ctx, input)
	if err != nil {
		return "", fmt.Errorf("tool invoke failed, tool_name=%s, error=%w", t.info.Name, err)
	}

	var result string
	if t.customOutputMarshaler != nil {
		result, err = t.customOutputMarshaler(ctx, output)
	} else {
		// json marshal the output
		result, err = defaultOutputMarshal(output)
	}
	if err != nil {
		return "", fmt.Errorf("tool output marshal failed, tool_name=%s, error=%w", t.info.Name, err)
	}

	return result, nil
}

// Heler function to create an InvokableTool from a function.
func NewInvoker[T, O any](info Info, fn InvokerFunc[T, O]) Invoker {
	if info.Schema == nil {
		sch := schema.Get[T]()
		info.Schema = &sch
	}

	return &invoker[T, O]{
		info: info,
		fn:   fn,
	}
}
