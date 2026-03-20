package agent

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"

	"github.com/ryanreadbooks/tokkibot/agent/tools"
	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	component "github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/pkg/audio"
	"github.com/ryanreadbooks/tokkibot/pkg/safe"
)

var (
	_ tools.SubAgentManager = (*subAgentToolDelegate)(nil)
	_ tools.MessageSender   = (*messageToolDelegate)(nil)
)

//go:embed template/subagent.md
var subagentPrompt string

const (
	subAgentExecuteTimeout = 30 * time.Minute
	subAgentResultExpiry   = 10 * time.Minute
)

type subAgentToolDelegate struct {
	a *Agent
}

func (d *subAgentToolDelegate) spawnSubAgent(meta component.InvokeMeta, input *tools.SubagentInput) (*Agent, error) {
	subAgentName := fmt.Sprintf("subagent-%s-%s", meta.ChatId, uuid.NewString()[:8])
	if input.Name != "" {
		subAgentName = input.Name
	}

	subAgentCfg := Config{
		RootCtx:      d.a.cfg.RootCtx,
		Name:         subAgentName,
		Provider:     d.a.cfg.Provider,
		Model:        d.a.cfg.Model,
		MaxIteration: d.a.cfg.MaxIteration,
		WorkspaceDir: d.a.cfg.WorkspaceDir,
		SessionDir:   config.GetSubAgentSessionsDir(d.a.Name(), subAgentName),

		isSpawned:              true,
		doNotAutoRegisterTools: true,
		subagentPrompt:         subagentPrompt,
	}

	subAgent := NewAgent(d.a.llm, subAgentCfg)
	subAgent.registerBasicTools(d.a.cfg.WorkspaceDir)

	return subAgent, nil
}

func (d *subAgentToolDelegate) SpawnSubagent(
	ctx context.Context,
	meta component.InvokeMeta,
	input *tools.SubagentInput,
) (*tools.SubagentResult, error) {
	subAgent, err := d.spawnSubAgent(meta, input)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn subagent: %w", err)
	}

	msg := &UserMessage{
		Channel: meta.Channel,
		ChatId:  meta.ChatId,
		Content: input.Task,
	}

	if input.Background {
		resultCh := make(chan string, 1)
		agentName := subAgent.Name()

		d.a.subAgentResultsMu.Lock()
		d.a.subAgentResults[agentName] = resultCh
		d.a.subAgentResultsMu.Unlock()

		// expiry timer to cleanup unclaimed results
		time.AfterFunc(subAgentResultExpiry, func() {
			d.a.subAgentResultsMu.Lock()
			delete(d.a.subAgentResults, agentName)
			d.a.subAgentResultsMu.Unlock()
		})

		safe.Go(func() {
			timeoutCtx, cancel := context.WithTimeout(ctx, subAgentExecuteTimeout)
			defer cancel()

			result := subAgent.Ask(timeoutCtx, msg)
			select {
			case resultCh <- result:
			default:
			}
		})

		return &tools.SubagentResult{
			Action:    tools.SubagentActionSpawn,
			AgentName: agentName,
			Result:    "Subagent is running in background. Call get_result with this name to retrieve the result.",
		}, nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, subAgentExecuteTimeout)
	defer cancel()
	result := subAgent.Ask(timeoutCtx, msg)
	return &tools.SubagentResult{
		Action:    tools.SubagentActionSpawn,
		AgentName: subAgent.Name(),
		Result:    result,
	}, nil
}

func (d *subAgentToolDelegate) GetSubagentResults(
	ctx context.Context,
	meta component.InvokeMeta,
	names []string,
	waitMode tools.WaitMode,
) (*tools.SubagentResult, error) {
	d.a.subAgentResultsMu.Lock()
	channels := make(map[string]chan string)
	notFound := make([]string, 0)
	for _, name := range names {
		if ch, exists := d.a.subAgentResults[name]; exists {
			channels[name] = ch
		} else {
			notFound = append(notFound, name)
		}
	}
	d.a.subAgentResultsMu.Unlock()

	results := make([]*tools.SubagentResultItem, 0, len(names))

	for _, name := range notFound {
		results = append(results, &tools.SubagentResultItem{
			Name:  name,
			Error: "no background subagent found with this name",
		})
	}

	if len(channels) == 0 {
		return &tools.SubagentResult{
			Action:  tools.SubagentActionGetResult,
			Results: results,
		}, nil
	}

	if waitMode == tools.WaitModeAny {
		cases := make([]reflect.SelectCase, 0, len(channels)+1)
		names := make([]string, 0, len(channels))

		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ctx.Done()),
		})

		for name, ch := range channels {
			cases = append(cases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(ch),
			})
			names = append(names, name)
		}

		chosen, value, ok := reflect.Select(cases)
		if chosen == 0 {
			return nil, ctx.Err()
		}

		name := names[chosen-1]
		result := ""
		if ok {
			result = value.String()
		}

		results = append(results, &tools.SubagentResultItem{
			Name:   name,
			Result: result,
		})

		d.a.subAgentResultsMu.Lock()
		delete(d.a.subAgentResults, name)
		d.a.subAgentResultsMu.Unlock()

	} else {
		for name, ch := range channels {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case result := <-ch:
				results = append(results, &tools.SubagentResultItem{
					Name:   name,
					Result: result,
				})
			}
		}

		d.a.subAgentResultsMu.Lock()
		for name := range channels {
			delete(d.a.subAgentResults, name)
		}
		d.a.subAgentResultsMu.Unlock()
	}

	return &tools.SubagentResult{
		Action:  tools.SubagentActionGetResult,
		Results: results,
	}, nil
}

// receiveSubAgentResults collects completed results (non-blocking) for auto-push.
func (d *subAgentToolDelegate) receiveSubAgentResults() string {
	d.a.subAgentResultsMu.Lock()
	defer d.a.subAgentResultsMu.Unlock()

	if len(d.a.subAgentResults) == 0 {
		return ""
	}

	var completed []struct {
		name   string
		result string
	}

	for name, ch := range d.a.subAgentResults {
		select {
		case result := <-ch:
			completed = append(completed, struct {
				name   string
				result string
			}{name: name, result: result})
		default:
		}
	}

	for _, c := range completed {
		delete(d.a.subAgentResults, c.name)
	}

	if len(completed) == 0 {
		return ""
	}

	var bd strings.Builder
	bd.Grow(512)
	for _, r := range completed {
		fmt.Fprintf(&bd, "<SubAgentResult>\n")
		fmt.Fprintf(&bd, "\t<Name>%s</Name>\n", r.name)
		fmt.Fprintf(&bd, "\t<Result>%s</Result>\n", r.result)
		fmt.Fprintf(&bd, "</SubAgentResult>\n")
	}

	return bd.String()
}

type messageToolDelegate struct {
	a *Agent
}

func (d *messageToolDelegate) Send(
	ctx context.Context,
	target tools.MessageTarget,
	meta component.InvokeMeta,
	input *tools.SendMessageInput,
) error {
	if target.Channel == "" || target.ChatId == "" {
		return fmt.Errorf("this user can not be reached")
	}

	var err error = fmt.Errorf("no output channel found")
	if msgCh := getXToolMetaMessageChannel(meta); msgCh != nil {
		err = nil
		attachments := make([]*chmodel.OutgoingMessageAttachment, 0, len(input.Attachments))
		if len(input.Attachments) > 0 {
			for _, fileName := range input.Attachments {
				if data, err := os.ReadFile(fileName); err != nil {
					slog.WarnContext(ctx, "failed to read attachment file", "error", err, "file", fileName)
					continue
				} else {
					attachment := chmodel.OutgoingMessageAttachment{
						Data:     data,
						Filename: filepath.Base(fileName),
					}
					mime := mimetype.Detect(data).String()
					if strings.Contains(mime, "image/") {
						attachment.Type = chmodel.AttachmentImage
					} else if strings.Contains(mime, "video/") {
						attachment.Type = chmodel.AttachmentVideo
					} else if strings.Contains(mime, "audio/") {
						attachment.Type = chmodel.AttachmentAudio
						if dur := audio.DetectDurationMs(data, mime); dur > 0 {
							attachment.Extra = &chmodel.OutgoingMessageAttachmentExtra{
								Audio: &chmodel.OutgoingMessageAudioAttachment{
									AudioDuration: dur,
								},
							}
						}
					} else {
						attachment.Type = chmodel.AttachmentFile
					}
					attachments = append(attachments, &attachment)
				}
			}
		}

		select {
		case msgCh.OutChan <- &chmodel.OutgoingMessage{
			ReceiverId:  target.ChatId,
			Channel:     chmodel.Type(target.Channel),
			ChatId:      target.ChatId,
			Content:     input.Content,
			Metadata:    msgCh.Metadata,
			Attachments: attachments,
		}:
		default:
			err = fmt.Errorf("failed to send message: message channel is full")
		}
	}

	return err
}
