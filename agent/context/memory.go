package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MemoryManagerConfig struct {
	Workspace string
}

const (
	longTermMemoryFile = "LONG-TERM.md"
)

// memory manager for the agent
type MemoryManager struct {
	Workspace string
}

func NewMemoryManager(c MemoryManagerConfig) *MemoryManager {
	return &MemoryManager{
		Workspace: filepath.Join(c.Workspace, "memory"),
	}
}

// Load long-term memory from the workspace
func (m *MemoryManager) loadLongTerm() (string, error) {
	fp := filepath.Join(m.Workspace, longTermMemoryFile)
	content, err := os.ReadFile(fp)
	if err != nil {
		return "", fmt.Errorf("failed to read long-term memory file: %w", err)
	}

	return string(content), nil
}

func (m *MemoryManager) loadShortTerm() (string, error) {
	// default load short-term memory for the past 3 days
	now := time.Now()
	content := strings.Builder{}
	content.Grow(1024 * 3)
	for i := 0; i < 3; i++ {
		memFile := filepath.Join(m.Workspace, now.AddDate(0, 0, -i).Format(time.DateOnly), "MEMORY.md")
		memContent, err := os.ReadFile(memFile)
		if err != nil {
			continue
		}

		_, _ = content.Write(memContent)
	}

	return content.String(), nil
}

func (m *MemoryManager) Load() (string, error) {
	longTerm, err := m.loadLongTerm()
	if err != nil {
		return "", fmt.Errorf("failed to load long-term memory: %w", err)
	}

	shortTerm, err := m.loadShortTerm()
	if err != nil {
		return "", fmt.Errorf("failed to load short-term memory: %w", err)
	}

	return longTerm + "\n\n---\n\n" + shortTerm, nil
}
