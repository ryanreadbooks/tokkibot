package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
)

type McpServerType string

const (
	McpServerTypeStdio      McpServerType = "stdio"
	McpServerTypeSse        McpServerType = "sse"
	McpServerTypeStreamable McpServerType = "streamable"
)

type McpServer struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	Url     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (m *McpServer) Validate() error {
	if m.Command == "" && m.Url == "" {
		return fmt.Errorf("invalid command and url")
	}

	if m.Command != "" && m.Url != "" {
		return fmt.Errorf("both command and url are set")
	}

	return nil
}

const mcpConfigFileName = "mcp.json"

type McpConfig struct {
	McpServers map[string]McpServer `json:"mcpServers,omitempty"`
}

func GetWorkspaceMcpConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(home, ".tokkibot", mcpConfigFileName), nil
}

func GetProjectMcpConfigPath() (string, error) {
	// ./.tokkibot/mcp.yaml
	return filepath.Join(GetProjectDir(), ".tokkibot", mcpConfigFileName), nil
}

func GetMcpConfig() (c McpConfig, err error) {
	var (
		workspaceMcpConfig McpConfig
		projectMcpConfig   McpConfig
	)

	workspaceMcpConfigPath, err := GetWorkspaceMcpConfigPath()
	if err == nil {
		workspaceMcpConfig, _ = getMcpConfigFromPath(workspaceMcpConfigPath)
	}

	projectMcpConfigPath, err := GetProjectMcpConfigPath()
	if err == nil {
		projectMcpConfig, err = getMcpConfigFromPath(projectMcpConfigPath)
	}

	// merge workspace and project config
	// if the same server name is defined in both, the project config will override the workspace config
	c.McpServers = make(map[string]McpServer)
	maps.Copy(c.McpServers, workspaceMcpConfig.McpServers)
	maps.Copy(c.McpServers, projectMcpConfig.McpServers)

	return
}

func getMcpConfigFromPath(path string) (c McpConfig, err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		err = fmt.Errorf("failed to read config file: %w", err)
		return
	}

	// try render env ${ENV_NAME}
	contentStr := os.ExpandEnv(string(content))
	err = json.Unmarshal([]byte(contentStr), &c)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal config: %w", err)
		return
	}

	return
}

type McpConfigScope string

const (
	McpConfigScopeGlobal  McpConfigScope = "global"
	McpConfigScopeProject McpConfigScope = "project"
)

func GetMcpConfigPath(scope McpConfigScope) (string, error) {
	if scope == McpConfigScopeGlobal {
		return GetWorkspaceMcpConfigPath()
	}
	return GetProjectMcpConfigPath()
}

func LoadMcpConfigFromScope(scope McpConfigScope) (McpConfig, error) {
	path, err := GetMcpConfigPath(scope)
	if err != nil {
		return McpConfig{}, err
	}
	return getMcpConfigFromPath(path)
}

func SaveMcpConfig(scope McpConfigScope, c McpConfig) error {
	path, err := GetMcpConfigPath(scope)
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	content, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func AddMcpServer(scope McpConfigScope, name string, server McpServer) error {
	c, _ := LoadMcpConfigFromScope(scope)
	if c.McpServers == nil {
		c.McpServers = make(map[string]McpServer)
	}
	c.McpServers[name] = server
	return SaveMcpConfig(scope, c)
}

func RemoveMcpServer(scope McpConfigScope, name string) error {
	c, err := LoadMcpConfigFromScope(scope)
	if err != nil {
		return err
	}
	if c.McpServers == nil {
		return fmt.Errorf("server '%s' not found", name)
	}
	if _, exists := c.McpServers[name]; !exists {
		return fmt.Errorf("server '%s' not found", name)
	}
	delete(c.McpServers, name)
	return SaveMcpConfig(scope, c)
}
