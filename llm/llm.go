package llm

import (
	"context"

	"github.com/ryanreadbooks/tokkibot/llm/schema"
)

type LLM interface {
	ChatCompletion(ctx context.Context, req *schema.Request) (*schema.Response, error)

	// You should read from the returned channel until it is closed.
	ChatCompletionStream(ctx context.Context, req *schema.Request) <-chan *schema.StreamResponseChunk
}

type TokenEstimator interface {
	Estimate(ctx context.Context, req *schema.Request) (int, error)
}
