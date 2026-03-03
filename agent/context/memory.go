package context

import (
	"os"
	"path/filepath"
)

type MemoryManagerConfig struct {
	Workspace string
}

const (
	longTermMemoryFile = "LONG-TERM.md"
)

type MemoryManager struct {
	Workspace string
}

func NewMemoryManager(c MemoryManagerConfig) *MemoryManager {
	return &MemoryManager{
		Workspace: filepath.Join(c.Workspace, "memory"),
	}
}

func (m *MemoryManager) Load() string {
	fp := filepath.Join(m.Workspace, longTermMemoryFile)
	content, err := os.ReadFile(fp)
	if err != nil {
		return ""
	}

	return string(content)
}
