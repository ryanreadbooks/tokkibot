package agent

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

	channelmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	llmmodel "github.com/ryanreadbooks/tokkibot/llm/model"
)

func getSessionKey(channel channelmodel.Type, chatId string) string {
	return fmt.Sprintf("%s_%s", channel, chatId)
}

type SessionMessage struct {
	Role    llmmodel.Role  `json:"role"`
	Content string         `json:"content"`
	Created int64          `json:"created"`
	Extras  map[string]any `json:"extras,omitempty"` // extras message
}

func (s *SessionMessage) IsFromUser() bool {
	return s.Role == llmmodel.RoleUser
}

func (s *SessionMessage) IsFromAssistant() bool {
	return s.Role == llmmodel.RoleAssistant
}

func (s *SessionMessage) IsFromTool() bool {
	return s.Role == llmmodel.RoleTool
}

func (s *SessionMessage) Json() string {
	c, _ := json.Marshal(s)
	return string(c)
}

// conversation Session for a single chat
type Session struct {
	msgMu sync.RWMutex

	// channel:chatId
	sessionId string

	// incremented message list, not all of them is here
	incrMsgList []SessionMessage

	// unix timestamp in second
	createdAt int64

	// unix timestamp in second
	updatedAt int64
}

func (s *Session) retrieve(dir string) ([]SessionMessage, error) {
	path := filepath.Join(dir, s.sessionId+".json")

	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

	historyList := make([]SessionMessage, 0, 128)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg SessionMessage
		err = json.Unmarshal(line, &msg)
		if err != nil {
			continue
		}

		historyList = append(historyList, msg)
	}

	return historyList, nil
}

func (s *Session) clear() {
	s.msgMu.Lock()
	defer s.msgMu.Unlock()
	s.incrMsgList = s.incrMsgList[:0]
}

func (s *Session) save(dir string) error {
	// append to the session file
	path := filepath.Join(dir, s.sessionId+".json")
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
	incrMsgList := slices.Clone(s.incrMsgList)
	s.incrMsgList = s.incrMsgList[:0]

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

func (s *Session) addUserMessage(msg string) {
	s.msgMu.Lock()
	defer s.msgMu.Unlock()

	s.incrMsgList = append(s.incrMsgList, SessionMessage{
		Role:    llmmodel.RoleUser,
		Content: msg,
		Created: time.Now().Unix(),
	})
}

func (s *Session) addAssistantMessage(msg string, toolCalls []llmmodel.CompletionToolCall) {
	s.msgMu.Lock()
	defer s.msgMu.Unlock()

	s.incrMsgList = append(s.incrMsgList, SessionMessage{
		Role:    llmmodel.RoleAssistant,
		Content: msg,
		Created: time.Now().Unix(),
		Extras: map[string]any{
			"tool_calls": toolCalls,
		},
	})
}

func (s *Session) addToolMessage(toolCallId, msg string) {
	s.msgMu.Lock()
	defer s.msgMu.Unlock()

	s.incrMsgList = append(s.incrMsgList, SessionMessage{
		Role:    llmmodel.RoleTool,
		Content: msg,
		Created: time.Now().Unix(),
		Extras: map[string]any{
			"tool_call_id": toolCallId,
		},
	})
}

type SessionManager struct {
	workspace    string
	saveInterval time.Duration

	mu       sync.RWMutex
	sessions map[string]*Session
}

type SessionManagerConfig struct {
	workspace string

	saveInterval time.Duration
}

func NewSessionManager(ctx context.Context, c SessionManagerConfig) *SessionManager {
	mgr := &SessionManager{
		sessions:     make(map[string]*Session),
		workspace:    filepath.Join(c.workspace, "sessions"),
		saveInterval: c.saveInterval,
	}
	if mgr.saveInterval <= 0 {
		mgr.saveInterval = 30 * time.Second
	}
	go mgr.startSaveLoop(ctx)

	return mgr
}

func (s *SessionManager) startSaveLoop(ctx context.Context) {
	defer func() {
		if err := recover(); err != nil {
			slog.Error("[session] save loop panic", "error", err)
		}
	}()

	ticker := time.NewTicker(s.saveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.saveAllSessions() // flush all before exit
			return
		case <-ticker.C:
			s.saveAllSessions()
		}
	}
}

func (s *SessionManager) saveAllSessions() {
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

func (s *SessionManager) SaveSession(channel channelmodel.Type, chatId string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := getSessionKey(channel, chatId)
	sess, ok := s.sessions[key]
	if !ok {
		return fmt.Errorf("session not found")
	}

	return sess.save(s.workspace)
}

func (s *SessionManager) GetSession(channel channelmodel.Type, chatId string) *Session {
	s.mu.RLock()

	key := getSessionKey(channel, chatId)

	if sess, ok := s.sessions[key]; ok {
		s.mu.RUnlock()
		return sess
	}

	s.mu.RUnlock()

	now := time.Now().Unix()
	sess := &Session{
		sessionId: key,
		createdAt: now,
		updatedAt: now,
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

func (s *SessionManager) GetSessionHistory(channel channelmodel.Type, chatId string) ([]SessionMessage, error) {
	sess, err := s.GetSession(channel, chatId).retrieve(s.workspace)
	if err != nil {
		return nil, err
	}

	return sess, nil
}
