package agent

import "context"

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
	EnableCwdAccess bool

	isSpawned              bool
	doNotAutoRegisterTools bool
	subagentPrompt         string
}
