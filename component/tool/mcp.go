package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/pkg/schema"
	"github.com/ryanreadbooks/tokkibot/pkg/xstring"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

const maxAllowedMcpToolOutputChars = 20000

// Wrap mcp to tool
type McpTool struct {
	serverName string
	sc         config.McpServer // server config, unchanged
	raw        mcp.Tool
	client     *mcpclient.Client
}

func (m *McpTool) ServerName() string {
	return m.serverName
}

func (m *McpTool) ToolName() string {
	return m.raw.Name
}

var _ Invoker = &McpTool{}

func toToolInfo(serverName string, tool mcp.Tool) Info {
	return Info{
		Name:        formatMcpToolKey(serverName, tool),
		Description: tool.Description,
		Schema:      toSchema(tool.InputSchema),
	}
}

func toSchema(sch mcp.ToolInputSchema) *schema.Schema {
	return &schema.Schema{
		Properties:           sch.Properties,
		Required:             sch.Required,
		AdditionalProperties: sch.AdditionalProperties,
	}
}

func formatMcpToolKey(serverName string, tool mcp.Tool) string {
	return fmt.Sprintf("%s_%s", serverName, tool.Name)
}

func (m *McpTool) IsValid() bool {
	return m.sc.Validate() == nil
}

func (m *McpTool) Info() Info {
	return toToolInfo(m.serverName, m.raw)
}

func (m *McpTool) Invoke(ctx context.Context, meta InvokeMeta, arguments string) (string, error) {
	// parse arguments
	resp, err := m.client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      m.raw.Name,
			Arguments: json.RawMessage(arguments),
		},
	})
	if err != nil {
		return "", fmt.Errorf("fail to call mcp tool: %w", err)
	}

	if resp.IsError {
		return "", fmt.Errorf("mcp tool call is error")
	}

	var output []byte
	if resp.StructuredContent != nil {
		output, err = json.Marshal(resp.StructuredContent)
	} else {
		output, err = json.Marshal(resp.Content)
	}
	if err != nil {
		return "", fmt.Errorf("failed to marshal structured content: %w", err)
	}

	// truncated if output is too long
	rawOutput := string(output)
	truncatedOutput := xstring.Truncate(rawOutput, maxAllowedMcpToolOutputChars)
	if utf8.RuneCountInString(truncatedOutput) < utf8.RuneCountInString(rawOutput) {
		return truncatedOutput + fmt.Sprintf("... (truncated, %d more chars)",
			utf8.RuneCountInString(rawOutput)-utf8.RuneCountInString(truncatedOutput)), nil
	}

	return rawOutput, nil
}

type McpToolManager struct {
	mu    sync.RWMutex
	tools map[string]*McpTool
}

func NewMcpToolManager() *McpToolManager {
	m := McpToolManager{
		tools: make(map[string]*McpTool),
	}

	return &m
}

func (m *McpToolManager) Init(ctx context.Context, c config.McpConfig) error {
	mcpTools := m.initMcpTools(ctx, c)
	m.mu.Lock()
	defer m.mu.Unlock()
	for serverName, tools := range mcpTools {
		for _, tool := range tools {
			m.tools[formatMcpToolKey(serverName, tool.raw)] = tool
		}
	}

	return nil
}

func (m *McpToolManager) ListTools() []*McpTool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tools := make([]*McpTool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool)
	}

	// sort by tool name
	slices.SortFunc(tools, func(a, b *McpTool) int {
		return strings.Compare(a.raw.Name, b.raw.Name)
	})

	return tools
}

// name is prepended with the server name for better uniqueness
func (m *McpToolManager) GetTool(name string) (*McpTool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tool, ok := m.tools[name]
	if ok {
		return tool, true
	}

	return nil, false
}

func (m *McpToolManager) startAndInitializeMcpClient(ctx context.Context, cli *mcpclient.Client, serverName string, sc config.McpServer) error {
	err := cli.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start mcp client: %w", err)
	}
	_, err = cli.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo:      mcp.Implementation{Name: "tokkibot-mcp-client"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize mcp client: %w", err)
	}

	// register notification
	cli.OnNotification(func(notification mcp.JSONRPCNotification) {
		if notification.Method == "notifications/tools/list_changed" {
			// cli get list again
			newCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()
			newTools, err := cli.ListTools(newCtx, mcp.ListToolsRequest{})
			if err != nil {
				return
			}

			m.mu.Lock()
			defer m.mu.Unlock()
			// update tools to newTools
			for _, tool := range newTools.Tools {
				key := formatMcpToolKey(serverName, tool)
				if oldTool, ok := m.tools[key]; ok {
					m.tools[key] = &McpTool{
						serverName: serverName,
						sc:         oldTool.sc,
						raw:        tool,
						client:     cli,
					}
				} else {
					m.tools[key] = &McpTool{
						serverName: serverName,
						sc:         sc,
						raw:        tool,
						client:     cli,
					}
				}
			}
		}
	})

	return nil
}

func (m *McpToolManager) listMcpStdioServerTools(ctx context.Context, serverName string, server config.McpServer) ([]*McpTool, error) {
	envs := make([]string, 0, len(server.Env))
	for name, value := range server.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", name, value))
	}
	cli, err := mcpclient.NewStdioMCPClient(server.Command, envs, server.Args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdio mcp client: %w", err)
	}

	if err = m.startAndInitializeMcpClient(ctx, cli, serverName, server); err != nil {
		return nil, err
	}

	response, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	tools := make([]*McpTool, 0, len(response.Tools))
	for _, tool := range response.Tools {
		tools = append(tools, &McpTool{
			sc:     server,
			raw:    tool,
			client: cli,
		})
	}

	return tools, nil
}

func (m *McpToolManager) listMcpStreamableHttpServerTools(ctx context.Context, serverName string, server config.McpServer) ([]*McpTool, error) {
	cli, err := mcpclient.NewStreamableHttpClient(server.Url,
		transport.WithHTTPHeaderFunc(
			func(ctx context.Context) map[string]string {
				return server.Headers
			},
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create streamable http mcp client: %w", err)
	}

	if err = m.startAndInitializeMcpClient(ctx, cli, serverName, server); err != nil {
		return nil, err
	}

	response, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	tools := make([]*McpTool, 0, len(response.Tools))
	for _, tool := range response.Tools {
		tools = append(tools, &McpTool{
			serverName: serverName,
			raw:        tool,
			client:     cli,
			sc:         server,
		})
	}

	return tools, nil
}

func (m *McpToolManager) listMcpSseServerTools(ctx context.Context, serverName string, server config.McpServer) ([]*McpTool, error) {
	cli, err := mcpclient.NewSSEMCPClient(server.Url,
		transport.WithHeaderFunc(func(ctx context.Context) map[string]string {
			return server.Headers
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sse mcp client: %w", err)
	}

	if err = m.startAndInitializeMcpClient(ctx, cli, serverName, server); err != nil {
		return nil, err
	}

	response, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	tools := make([]*McpTool, 0, len(response.Tools))
	for _, tool := range response.Tools {
		tools = append(tools, &McpTool{
			serverName: serverName,
			sc:         server,
			raw:        tool,
			client:     cli,
		})
	}

	return tools, nil
}

// use streamable http over sse
func (m *McpToolManager) listMcpHttpServerTools(ctx context.Context, serverName string, server config.McpServer) ([]*McpTool, error) {
	if tools, err := m.listMcpStreamableHttpServerTools(ctx, serverName, server); err == nil {
		return tools, nil
	}

	return m.listMcpSseServerTools(ctx, serverName, server)
}

func (m *McpToolManager) initMcpTools(ctx context.Context, mcpConfig config.McpConfig) map[string][]*McpTool {
	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	tools := make(map[string][]*McpTool, len(mcpConfig.McpServers))
	for serverName, server := range mcpConfig.McpServers {
		wg.Go(func() {
			err := server.Validate()
			if err != nil {
				return
			}

			if server.Command != "" {
				if mcpTools, err := m.listMcpStdioServerTools(ctx, serverName, server); err == nil {
					mu.Lock()
					tools[serverName] = append(tools[serverName], mcpTools...)
					mu.Unlock()
				}
			} else {
				if mcpTools, err := m.listMcpHttpServerTools(ctx, serverName, server); err == nil {
					mu.Lock()
					tools[serverName] = append(tools[serverName], mcpTools...)
					mu.Unlock()
				}
			}
		})
	}

	wg.Wait()

	return tools
}

func (m *McpToolManager) Close() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, tool := range m.tools {
		tool.client.Close()
	}
}
