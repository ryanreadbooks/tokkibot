package skill

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/ryanreadbooks/tokkibot/pkg/bash"
)

type Script struct {
	// working directory of the script
	Path string
}

func (s *Script) Execute(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()

	instruct, args := bash.ParseCommand(command)
	cmd := exec.CommandContext(ctx, instruct, args...)
	cmd.Dir = s.Path // working directory of the script

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, string(output))
	}

	outputStr := string(output)

	const maxAllowedScriptOutputLen = 15000
	// truncate output
	if len(outputStr) > maxAllowedScriptOutputLen {
		more := len(outputStr) - maxAllowedScriptOutputLen
		outputStr = outputStr[:maxAllowedScriptOutputLen] + fmt.Sprintf("\n... (truncated, %d more chars)", more)
	}

	return outputStr, nil
}
