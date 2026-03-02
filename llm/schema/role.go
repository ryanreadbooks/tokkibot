package schema

import "github.com/ryanreadbooks/tokkibot/llm/schema/param"

type Role = param.Role

const (
	RoleSystem    = param.RoleSystem
	RoleUser      = param.RoleUser
	RoleAssistant = param.RoleAssistant
	RoleTool      = param.RoleTool
)
