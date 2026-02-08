package model

type CompletionUsage struct {
	CompletionTokens int64
	// Number of tokens in the prompt.
	PromptTokens int64
	// Total number of tokens used in the request (prompt + completion).
	TotalTokens int64
}
