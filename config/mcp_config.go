package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type McpServerType string

const (
	McpServerTypeStdio      McpServerType = "stdio"
	McpServerTypeSse        McpServerType = "sse"
	McpServerTypeStreamable McpServerType = "streamable"
)

type McpServer struct {
	Command string            `yaml:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`

	Url     string            `yaml:"url,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
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

const mcpConfigFileName = "mcp.yaml"

type McpConfig struct {
	McpServers map[string]McpServer `yaml:"mcp_servers,omitempty"`
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
	err = yaml.Unmarshal([]byte(contentStr), &c)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal config: %w", err)
		return
	}

	return
}
