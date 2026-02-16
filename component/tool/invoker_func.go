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

	invr := &InvokeResult{Success: true}
	// invoke the function
	output, errOutput := t.fn(ctx, input)
	if errOutput != nil {
		invr.Success = false
		invr.Err = fmt.Errorf("tool %s invoke err: %w", t.info.Name, errOutput).Error()
		return invr.Json(), nil
	}

	// json marshal the output
	result, errMarshal := defaultOutputMarshal(output)
	if errMarshal != nil {
		invr.Err = fmt.Errorf("tool %s success but output marshal err: %w", t.info.Name, errMarshal).Error()
	}

	invr.Data = result

	return invr.Json(), nil
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
