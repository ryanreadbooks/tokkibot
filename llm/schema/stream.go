package schema

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ryanreadbooks/tokkibot/pkg/safe"
	"github.com/ryanreadbooks/tokkibot/pkg/xmap"
)

func SyncReadStream(ch <-chan *StreamResponseChunk) ([]StreamChoice, error) {
	// choice index -> choice
	choicesMap := make(map[int64]StreamChoice)

	// choice index -> tool call index -> tool call
	choicesToolCallsMap := make(map[int64]map[int64]StreamChoiceDeltaToolCall)
	for chunk := range ch {
		if chunk.Err != nil {
			return nil, chunk.Err
		}

		for _, choice := range chunk.Choices {
			curIdx := choice.Index
			if existing, ok := choicesMap[curIdx]; ok {
				existing.Delta.Content += choice.Delta.Content
				if choice.FinishReason != "" {
					existing.FinishReason = FinishReason(choice.FinishReason)
				}
				choicesMap[curIdx] = existing
			} else {
				choicesMap[curIdx] = choice
			}

			if choice.Delta.HasToolCalls() {
				for _, toolCall := range choice.Delta.ToolCalls {
					if existing, ok := choicesToolCallsMap[curIdx]; ok {
						if existingToolCall, ok := existing[toolCall.Index]; ok {
							existingToolCall.Function.Arguments += toolCall.Function.Arguments
							choicesToolCallsMap[curIdx][toolCall.Index] = existingToolCall
						} else {
							choicesToolCallsMap[curIdx][toolCall.Index] = toolCall
						}
					} else {
						choicesToolCallsMap[curIdx] = make(map[int64]StreamChoiceDeltaToolCall)
						choicesToolCallsMap[curIdx][toolCall.Index] = toolCall
					}
				}
			}
		}
	}

	// assign tool calls to corresponding choices
	for idx, choice := range choicesMap {
		if toolCalls, ok := choicesToolCallsMap[choice.Index]; ok {
			tcs := xmap.Values(toolCalls)
			old := choicesMap[idx]
			old.Delta.ToolCalls = tcs

			// sort tool calls by index
			sort.Slice(old.Delta.ToolCalls, func(i, j int) bool {
				return old.Delta.ToolCalls[i].Index < old.Delta.ToolCalls[j].Index
			})
			choicesMap[idx] = old
		}
	}

	choices := xmap.Values(choicesMap)
	sort.Slice(choices, func(i, j int) bool {
		return choices[i].Index < choices[j].Index
	})

	return choices, nil
}

type streamToolCallBuffer struct {
	Index     int64
	Id        string
	Name      string
	Arguments *strings.Builder
	Type      ToolCallType
}

type StreamToolCallHandler func(ctx context.Context, tc StreamChoiceDeltaToolCall)

type StreamContentFragment struct {
	Content          string
	ReasoningContent string
	FinishReason     FinishReason
}

type StreamToolCallFragment struct {
	Id               string
	Name             string
	ArgumentFragment string
}

type StreamResponsePack struct {
	Content  <-chan *StreamContentFragment
	ToolCall <-chan *StreamToolCallFragment
}

func onStreamToolCallAccumulated(
	ctx context.Context,
	handler StreamToolCallHandler,
) func(tcbs []*streamToolCallBuffer) {
	return func(tcbs []*streamToolCallBuffer) {
		for _, tc := range tcbs {
			handler(ctx, StreamChoiceDeltaToolCall{
				Index: tc.Index,
				Type:  tc.Type,
				Id:    tc.Id,
				Function: CompletionToolCallFunction{
					Name:      tc.Name,
					Arguments: tc.Arguments.String(),
				},
			})
		}
	}
}

// We take the first choice forever. N > 1 not supported
//
// handler will be called in an other goroutine, so you don't need to open another goroutine for it.
func StreamResponseHandler(ctx context.Context,
	chunkCh <-chan *StreamResponseChunk,
	handler StreamToolCallHandler,
) *StreamResponsePack {
	contentCh := make(chan *StreamContentFragment, 256) // try avoid blocking
	toolCallCh := make(chan *StreamToolCallFragment, 256)

	// when this goroutine finished, tool are already called, and content channel is closed.
	safe.Go(func() {
		readStreamResponseChunk(ctx,
			chunkCh, contentCh, toolCallCh,
			onStreamToolCallAccumulated(ctx, handler))
	})

	return &StreamResponsePack{
		Content:  contentCh,
		ToolCall: toolCallCh,
	}
}

func readStreamResponseChunk(
	ctx context.Context,
	ch <-chan *StreamResponseChunk,
	contentCh chan<- *StreamContentFragment,
	toolCallCh chan<- *StreamToolCallFragment,
	onToolAccumulated func(tcbs []*streamToolCallBuffer),
) {
	var (
		tcBuffers       = make(map[int64]*streamToolCallBuffer)
		closeContentCh  = closeOnce(contentCh)
		closeToolCallCh = closeOnce(toolCallCh)
	)

	defer func() {
		closeContentCh()
		closeToolCallCh()
	}()

	for chunk := range ch {
		if chunk.Err != nil {
			contentCh <- &StreamContentFragment{
				Content:      fmt.Sprintf("error: %v", chunk.Err),
				FinishReason: FinishReasonStop,
			}
			break
		}

		curChoice := chunk.FirstChoice()
		delta := curChoice.Delta

		if curChoice.FinishReason.IsStopped() {
			break
		}

		if delta.Content != "" || delta.ReasoningContent != "" {
			select {
			case contentCh <- &StreamContentFragment{
				Content:          delta.Content,
				ReasoningContent: delta.ReasoningContent,
				FinishReason:     curChoice.FinishReason,
			}:
			case <-ctx.Done():
				return
			}
		}

		if delta.HasToolCalls() {
			// we accumulate tool calls until we get the final tool call result
			for _, tc := range delta.ToolCalls {
				if buf, ok := tcBuffers[tc.Index]; ok {
					buf.Arguments.WriteString(tc.Function.Arguments)
					select {
					case toolCallCh <- &StreamToolCallFragment{
						Id:               buf.Id,
						Name:             buf.Name,
						ArgumentFragment: tc.Function.Arguments,
					}:
					case <-ctx.Done():
						return
					}
				} else {
					var sb strings.Builder
					sb.Grow(64)
					sb.WriteString(tc.Function.Arguments)
					// this is a new tool call, we create it
					tcBuffers[tc.Index] = &streamToolCallBuffer{
						Index:     tc.Index,
						Name:      tc.Function.Name,
						Arguments: &sb,
						Id:        tc.Id,
						Type:      tc.Type,
					}
					select {
					case toolCallCh <- &StreamToolCallFragment{
						Id:               tc.Id,
						Name:             tc.Function.Name,
						ArgumentFragment: tc.Function.Arguments,
					}:
					case <-ctx.Done():
						return
					}
				}
			}
		}

		if curChoice.FinishReason.IsToolCalls() {
			// tool call parameters are accumulated, we can call the tool now
			break
		}
	}

	if len(tcBuffers) > 0 {
		if onToolAccumulated != nil {
			// sort by index
			sortedTcs := xmap.Values(tcBuffers)
			sort.Slice(sortedTcs, func(i, j int) bool {
				return sortedTcs[i].Index < sortedTcs[j].Index
			})
			onToolAccumulated(sortedTcs)
		}
	}
}

func closeOnce[T any](ch chan<- T) func() {
	once := sync.Once{}
	return func() {
		once.Do(func() {
			close(ch)
		})
	}
}
