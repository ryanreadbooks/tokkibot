package agent

import (
	"context"

	"github.com/ryanreadbooks/tokkibot/config"
)

type Config struct {
	RootCtx context.Context

	// Agent name, determines workspace and session isolation.
	Name string

	// The provider to use.
	Provider string

	// The model to use.
	Model string

	// Max tool-call iterations per request.
	MaxIteration int

	ResumeSessionId string

	WorkspaceDir    string // workspace directory
	SessionDir      string // where session and context logs are stored
	VolatileContext bool   // if true, context/session data stays in memory only
	EnableCwdAccess bool

	Sandbox *config.SandboxConfig

	isSpawned              bool
	doNotAutoRegisterTools bool
	subagentPrompt         string
}
