package estimator

import (
	"context"
	"unicode"

	"github.com/ryanreadbooks/tokkibot/llm/schema"
)

// Simple token estimation
func EstimateToken(text string) int {
	if text == "" {
		return 0
	}

	var tokens float64
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			tokens += 1.0 / 1.2
			continue
		}

		if unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
			tokens += 1.0 / 1.5
			continue
		}

		if unicode.Is(unicode.Cyrillic, r) {
			tokens += 1.0 / 3.0
			continue
		}

		if unicode.Is(unicode.Arabic, r) {
			tokens += 1.0 / 2.5
			continue
		}

		if unicode.Is(unicode.Latin, r) {
			tokens += 1.0 / 3.5
			continue
		}

		if unicode.IsDigit(r) {
			tokens += 1.0 / 4.0
			continue
		}

		if unicode.Is(unicode.So, r) || unicode.Is(unicode.Sk, r) || unicode.Is(unicode.Sm, r) {
			tokens += 1.0
			continue
		}

		if unicode.IsPunct(r) {
			tokens += 1.0 / 2.0
			continue
		}

		if unicode.IsSpace(r) {
			tokens += 1.0 / 5.0
			continue
		}

		tokens += 1.0 / 2.0
	}

	return int(tokens) + 1
}

type RoughEstimator struct{}

func (RoughEstimator) Estimate(_ context.Context, req *schema.Request) (int, error) {
	// messages and tools should be taken into consideration
	if req == nil {
		return 0, nil
	}

	var total int

	for _, msg := range req.Messages {
		if msg.SystemMessageParam != nil {
			total += EstimateToken(msg.SystemMessageParam.GetContent())
		}

		if msg.UserMessageParam != nil {
			total += EstimateToken(msg.UserMessageParam.GetContent())
		}

		if msg.AssistantMessageParam != nil {
			total += EstimateToken(msg.AssistantMessageParam.GetContent())
		}

		if msg.ToolMessageParam != nil {
			total += EstimateToken(msg.ToolMessageParam.GetContent())
		}
	}

	for _, tool := range req.Tools {
		total += EstimateToken(tool.GetContent())
	}

	return total, nil
}
