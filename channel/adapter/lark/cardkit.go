package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	cardkitv1 "github.com/larksuite/oapi-sdk-go/v3/service/cardkit/v1"
	"github.com/ryanreadbooks/tokkibot/channel/adapter/lark/card"
)

const (
	thinkingElementId = "thinking_element_1"
)

var patchThinkingElementToFinishedAction = fmt.Sprintf(`
[
	{
		"action": "partial_update_element",
		"params": {
			"element_id": "%s",
			"partial_element": {
				"expanded": false,
				"header": {
					"title": {
						"content": "💡完成思考",
						"tag": "plain_text"
					}
				}
			}
		}
	}	
]`, thinkingElementId)

func constructThinkingElement(elementId string, headerTitle string, expanded bool) *card.CollapsiblePanelElement {
	thinkingElement := card.NewCollapsiblePanelElement(thinkingElementId)
	thinkingElement.WithHeaderTitle(headerTitle)
	thinkingElement.WithExpanded(expanded)
	thinkingElement.WithBorder(&card.CollapsiblePanelBorder{
		Color:        "grey-200",
		CornerRadius: "5px",
	})
	markdownElement := card.NewBodyMarkdownElement("") // placeholder
	markdownElement.WithElementId(elementId)
	thinkingElement.AppendElement(markdownElement)
	thinkingElement.WithBackgroundColor("grey")
	return thinkingElement
}

func (a *LarkAdapter) patchThinkingElementToFinished(ctx context.Context, cardId string, seq int) error {
	body := cardkitv1.NewBatchUpdateCardReqBodyBuilder().
		Uuid(uuid.NewString()).
		Sequence(seq).
		Actions(patchThinkingElementToFinishedAction).
		Build()

	req := cardkitv1.NewBatchUpdateCardReqBuilder().CardId(cardId).Body(body).Build()
	resp, err := a.cli.Cardkit.V1.Card.BatchUpdate(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("failed to patch thinking element to finished: %s", resp.ErrorResp())
	}
	return nil
}

func (a *LarkAdapter) createCardEntityForStream(ctx context.Context,
	contentElementId string,
	reasoningContentElementId string,
	thinkingEnabled bool,
) (string, error) {
	bd := card.NewCardV2Builder()
	if thinkingEnabled {
		bd.AppendBodyElement(constructThinkingElement(reasoningContentElementId, "💭思考中...", true))
	}

	c := bd.AppendBodyElement(
		card.NewBodyMarkdownElement("").WithElementId(contentElementId),
	).Build()
	c.Config = &card.Config{
		StreamingMode: true,
		StreamingConfig: &card.StreamingConfig{
			PrintStrategy: card.StreamingConfigPrintStrategyDelay,
		},
	}
	cardJson, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	body := cardkitv1.NewCreateCardReqBodyBuilder().
		Type("card_json").
		Data(string(cardJson)).
		Build()
	req := cardkitv1.NewCreateCardReqBuilder().Body(body).Build()
	resp, err := a.cli.Cardkit.V1.Card.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("failed to create card entity: %s", resp.ErrorResp())
	}

	return *resp.Data.CardId, nil
}

func (a *LarkAdapter) updateCardEntityForStream(
	ctx context.Context,
	cardId, elementId, content string,
	seq int,
) error {
	body := cardkitv1.NewContentCardElementReqBodyBuilder().
		Uuid(uuid.NewString()).
		Content(content).
		Sequence(seq).
		Build()

	req := cardkitv1.NewContentCardElementReqBuilder().
		CardId(cardId).
		ElementId(elementId).
		Body(body).
		Build()

	resp, err := a.cli.Cardkit.V1.CardElement.Content(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update card entity for stream", "error", err)
		return err
	}
	if !resp.Success() {
		slog.ErrorContext(ctx, "failed to update card entity for stream", "error", resp.ErrorResp())
		return fmt.Errorf("failed to update card entity: %v", resp.ErrorResp())
	}

	return nil
}

func (a *LarkAdapter) stopCardEntityStream(ctx context.Context, cardId string, seq int) {
	config := `{"config":{"streaming_mode":false}}`
	settings := cardkitv1.NewSettingsCardReqBodyBuilder().
		Settings(config).
		Uuid(uuid.NewString()).
		Sequence(seq).
		Build()
	req := cardkitv1.NewSettingsCardReqBuilder().
		CardId(cardId).
		Body(settings).
		Build()
	resp, err := a.cli.Cardkit.V1.Card.Settings(ctx, req)
	if err != nil {
		slog.WarnContext(ctx, "failed to stop card entity stream", "error", err)
		return
	}

	if !resp.Success() {
		slog.WarnContext(ctx, "failed to stop card entity stream", "error", resp.ErrorResp())
	}
}
