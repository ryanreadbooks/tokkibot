package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

type MemoryManagerConfig struct {
	workspace string
}

const (
	longTermMemoryFile = "LONG-TERM.md"
)

// memory manager for the agent
type MemoryManager struct {
	workspace string
}

func NewMemoryManager(c MemoryManagerConfig) *MemoryManager {
	return &MemoryManager{
		workspace: filepath.Join(c.workspace, "memory"),
	}
}

// Load long-term memory from the workspace
func (m *MemoryManager) loadLongTerm() (string, error) {
	fp := filepath.Join(m.workspace, longTermMemoryFile)
	content, err := os.ReadFile(fp)
	if err != nil {
		return "", fmt.Errorf("failed to read long-term memory file: %w", err)
	}

	return string(content), nil
}

func (m *MemoryManager) Load() (string, error) {
	return m.loadLongTerm()
}