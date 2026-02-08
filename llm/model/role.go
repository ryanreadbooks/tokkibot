package model

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

func (r Role) String() string {
	return string(r)
}

func (r Role) System() bool {
	return r == RoleSystem
}

func (r Role) User() bool {
	return r == RoleUser
}

func (r Role) Assistant() bool {
	return r == RoleAssistant
}

func (r Role) Tool() bool {
	return r == RoleTool
}
