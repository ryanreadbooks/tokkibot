package agent

type EmittedReasoningContentMetadata struct {
	ThinkingEnabled bool
}

type EmittedContent struct {
	Round            int
	Content          string
	ReasoningContent string
	Metadata         EmittedReasoningContentMetadata
}

// StreamEmitter is the interface for emitting stream events
type StreamEmitter interface {
	EmitContent(content *EmittedContent)
	EmitTool(round int, name, args string)
	EmitDone()
}
