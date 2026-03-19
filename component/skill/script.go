package skill

import (
	"context"
	"fmt"
	"time"

	"github.com/ryanreadbooks/tokkibot/component/sandbox"
)

type Script struct {
	sb sandbox.Sandbox
}

func (s *Script) Execute(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()

	outputStr, err := s.sb.Execute(ctx, command)
	if err != nil {
		return "", fmt.Errorf("script execution failed: %w", err)
	}

	const maxAllowedScriptOutputLen = 15000
	if len(outputStr) > maxAllowedScriptOutputLen {
		more := len(outputStr) - maxAllowedScriptOutputLen
		outputStr = outputStr[:maxAllowedScriptOutputLen] + fmt.Sprintf("\n... (truncated, %d more chars)", more)
	}

	return outputStr, nil
}
