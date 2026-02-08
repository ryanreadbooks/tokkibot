package model

type ToolCallType string

const (
	ToolCallTypeFunction ToolCallType = "function"
)

type CompletionToolCall struct {
	Id       string                     `json:"id"`
	Type     ToolCallType               `json:"type"`
	Function CompletionToolCallFunction `json:"function"`
}

func (t *CompletionToolCall) ToToolCallParam() *ToolCallParam {
	return &ToolCallParam{
		Function: &ToolCallFunctionParam{
			Id:        t.Id,
			Name:      t.Function.Name,
			Arguments: t.Function.Arguments,
		},
	}
}

type CompletionToolCallFunction struct {
	// the arguments to call the function with
	Arguments string `json:"arguments"`

	// the name of the function to call
	Name string `json:"name"`
}
