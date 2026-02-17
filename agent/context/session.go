package context

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mitchellh/mapstructure"
	"github.com/ryanreadbooks/tokkibot/agent/ref"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	llmmodel "github.com/ryanreadbooks/tokkibot/llm/model"
	"github.com/ryanreadbooks/tokkibot/pkg/process"
)

var (
	compactThreshold = 5000
)

const (
	extraToolCallsKey  = "tool_calls"
	extraToolCallIdKey = "tool_call_id"
)

func getSessionLogKey(channel, chatId string) string {
	return fmt.Sprintf("%s_%s", channel, chatId)
}

// SessionLogItem every messages into session file
type SessionLogItem struct {
	Role             llmmodel.Role  `json:"role"`
	Content          string         `json:"content"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	Created          int64          `json:"created"`
	Extras           map[string]any `json:"extras,omitempty"` // extras message
}

func (s *SessionLogItem) IsFromUser() bool {
	return s.Role == llmmodel.RoleUser
}

func (s *SessionLogItem) IsFromAssistant() bool {
	return s.Role == llmmodel.RoleAssistant
}

func (s *SessionLogItem) IsFromTool() bool {
	return s.Role == llmmodel.RoleTool
}

func (s *SessionLogItem) Json() string {
	c, _ := json.Marshal(s)
	return string(c)
}

// nothing will happen if fail
func (s *SessionLogItem) compactToolCall() {
	if !s.IsFromTool() {
		return
	}

	// compact tool call if content is too long
	if l := utf8.RuneCountInString(s.Content); l >= compactThreshold {
		refName, err := ref.Save(s.Content)
		if err != nil {
			return
		}
		// tool content is json format
		var invr tool.InvokeResult
		err = json.Unmarshal([]byte(s.Content), &invr)
		if err != nil {
			s.Content = refName
		} else {
			// update content
			invr.Data = refName // refs data field
			newContent, err := json.Marshal(invr)
			if err == nil {
				s.Content = string(newContent)
			}
		}
	}
}

// compact assistant tool call instruction arguments
func (s *SessionLogItem) compactAssistant() {
	if !s.IsFromAssistant() {
		return
	}

	toolCall := s.Extras[extraToolCallsKey]
	tcs, _ := toolCall.([]llmmodel.CompletionToolCall)
	for idx, tc := range tcs {
		// check tc arguments
		if len(tc.Function.Arguments) >= compactThreshold {
			newRefName, err := ref.Save(tc.Function.Arguments)
			if err == nil {
				tcs[idx].Function.Arguments = newRefName
			}
		}
	}
}

// Complete conversation SessionLog for a single chat
type SessionLog struct {
	msgMu sync.RWMutex

	channel string
	chatId  string

	// incremented message list, not all of them is here
	incrLogList []SessionLogItem

	// unix timestamp in second
	createdAt int64

	// unix timestamp in second
	updatedAt int64
}

func (s *SessionLog) logFileName() []string {
	return []string{s.channel, s.chatId, "log.jsonl"}
}

// ~/sessions/channel/chatid/log.jsonl
func (s *SessionLog) fullLogFileName(root string) string {
	elems := []string{root}
	elems = append(elems, s.logFileName()...)

	return filepath.Join(elems...)
}

func (s *SessionLog) retrieve(root string) ([]SessionLogItem, error) {
	f, err := os.OpenFile(s.fullLogFileName(root), os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

	historyList := make([]SessionLogItem, 0, 128)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// one line is one log item
		var msg SessionLogItem
		err = json.Unmarshal(line, &msg)
		if err != nil {
			continue
		}

		if msg.Role.Assistant() {
			toolCalls := []llmmodel.CompletionToolCall{}
			if msg.Extras == nil {
				continue
			}

			if extraToolCalls, ok := msg.Extras[extraToolCallsKey]; ok {
				// tc is []any
				anyTcs, _ := extraToolCalls.([]any)
				for _, ttc := range anyTcs {
					// map structure
					var completionToolCall llmmodel.CompletionToolCall
					err = mapstructure.Decode(ttc, &completionToolCall)
					if err == nil {
						toolCalls = append(toolCalls, completionToolCall)
					}
				}
			}

			if len(toolCalls) > 0 {
				// cover msg extras
				msg.Extras[extraToolCallsKey] = toolCalls
			}
		}

		historyList = append(historyList, msg)
	}

	return historyList, nil
}

func (s *SessionLog) save(root string) error {
	// append to the session file
	path := s.fullLogFileName(root)
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("failed to create parent directories for %s: %w", path, err)
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

	s.msgMu.Lock()
	incrMsgList := slices.Clone(s.incrLogList)
	s.incrLogList = s.incrLogList[:0]

	s.msgMu.Unlock()

	if len(incrMsgList) == 0 {
		return nil
	}

	for _, msg := range incrMsgList {
		if content := msg.Json(); len(content) > 0 {
			_, err = f.WriteString(content + "\n") // ignore error here
			if err != nil {
				slog.Error("[session] failed to write session file", "error", err)
			}
		}
	}

	return nil
}

func (s *SessionLog) addUserMessage(msg string) {
	s.msgMu.Lock()
	defer s.msgMu.Unlock()

	s.incrLogList = append(s.incrLogList, SessionLogItem{
		Role:    llmmodel.RoleUser,
		Content: msg,
		Created: time.Now().Unix(),
	})
}

func (s *SessionLog) addAssistantMessage(
	content string,
	toolCalls []llmmodel.CompletionToolCall,
	reasoningContent string,
) {
	s.msgMu.Lock()
	defer s.msgMu.Unlock()

	s.incrLogList = append(s.incrLogList, SessionLogItem{
		Role:             llmmodel.RoleAssistant,
		Content:          content,
		ReasoningContent: reasoningContent,
		Created:          time.Now().Unix(),
		Extras: map[string]any{
			extraToolCallsKey: toolCalls,
		},
	})
}

func (s *SessionLog) addToolMessage(toolCallId, msg string) {
	s.msgMu.Lock()
	defer s.msgMu.Unlock()

	s.incrLogList = append(s.incrLogList, SessionLogItem{
		Role:    llmmodel.RoleTool,
		Content: msg,
		Created: time.Now().Unix(),
		Extras: map[string]any{
			extraToolCallIdKey: toolCallId,
		},
	})
}

type SessionLogManager struct {
	workspace    string
	saveInterval time.Duration

	mu       sync.RWMutex
	sessions map[string]*SessionLog
}

type SessionLogManagerConfig struct {
	workspace string

	saveInterval time.Duration
}

func NewSessionLogManager(ctx context.Context, c SessionLogManagerConfig) *SessionLogManager {
	mgr := &SessionLogManager{
		sessions:     make(map[string]*SessionLog),
		workspace:    filepath.Join(c.workspace, "sessions"),
		saveInterval: c.saveInterval,
	}
	if mgr.saveInterval <= 0 {
		mgr.saveInterval = 30 * time.Second
	}
	go mgr.startSaveLoop(ctx)

	return mgr
}

func (s *SessionLogManager) startSaveLoop(ctx context.Context) {
	defer func() {
		if err := recover(); err != nil {
			slog.Error("[session] save loop panic", "error", err)
		}
	}()

	ticker := time.NewTicker(s.saveInterval)
	defer ticker.Stop()

	wg := process.GetRootWaitGroup(ctx)
	if wg != nil {
		wg.Add(1)
		defer wg.Done()
	}

	for {
		select {
		case <-ctx.Done():
			s.saveAll() // flush all before exit
			return
		case <-ticker.C:
			s.saveAll()
		}
	}
}

func (s *SessionLogManager) saveAll() {
	// snap shot
	s.mu.RLock()
	snapshot := maps.Clone(s.sessions)
	s.mu.RUnlock()

	for _, sess := range snapshot {
		err := sess.save(s.workspace)
		if err != nil {
			slog.Error("[session] failed to save session", "error", err)
		}
	}
}

func (s *SessionLogManager) get(channel, chatId string) *SessionLog {
	s.mu.RLock()

	key := getSessionLogKey(channel, chatId)

	if sess, ok := s.sessions[key]; ok {
		s.mu.RUnlock()
		return sess
	}

	s.mu.RUnlock()

	now := time.Now().Unix()
	sess := &SessionLog{
		createdAt: now,
		updatedAt: now,
		channel:   channel,
		chatId:    chatId,
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if tmpSess, ok := s.sessions[key]; ok {
		// create new session
		return tmpSess
	} else {
		s.sessions[key] = sess
	}

	return sess
}

func (s *SessionLogManager) getHistory(channel, chatId string) ([]SessionLogItem, error) {
	sess, err := s.get(channel, chatId).retrieve(s.workspace)
	if err != nil {
		return nil, err
	}

	return sess, nil
}

func UnmarshalExtraToolCalls(extras map[string]any) []*llmmodel.ToolCallParam {
	val := extras[extraToolCallsKey] // []map[string]any
	var (
		toolCalls           []*llmmodel.ToolCallParam
		completionToolCalls []llmmodel.CompletionToolCall
	)

	switch val := val.(type) {
	case []llmmodel.CompletionToolCall:
		completionToolCalls = val
	default:
		err := mapstructure.Decode(val, &completionToolCalls)
		if err == nil {
			toolCalls = make([]*llmmodel.ToolCallParam, 0, len(completionToolCalls))
		}
	}

	for _, toolCall := range completionToolCalls {
		// we have to make sure tool call id exists
		if toolCall.Id != "" {
			toolCalls = append(toolCalls, toolCall.ToToolCallParam())
		}
	}

	return toolCalls
}

func UnmarshalExtraToolCallId(extras map[string]any) string {
	callId, _ := extras[extraToolCallIdKey].(string)
	return callId
}
