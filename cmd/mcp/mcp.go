package mcp

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/spf13/cobra"
)

var McpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP servers",
	Long:  "Add, remove, or list MCP server configurations.",
}

var (
	addTransport string
	addEnvs      []string
	addHeaders   []string
	addGlobal    bool
)

var addCmd = &cobra.Command{
	Use:   "add [flags] <name> [-- <command> [args...]] or add [flags] <name> <url>",
	Short: "Add an MCP server",
	Long: `Add an MCP server configuration.

For stdio transport:
  mcp add --transport stdio <name> -- <command> [args...]
  
  Examples:
    mcp add --transport stdio myserver -- npx -y @anthropic/mcp-server
    mcp add --transport stdio --env KEY=value myserver -- python server.py --port 8080

For http transport:
  mcp add --transport http <name> <url>
  
  Examples:
    mcp add --transport http notion https://mcp.notion.com/mcp
    mcp add --transport http --header "Authorization: Bearer token" serverName https://api.example.com/mcp

For stdio, use -- to separate the server name from the command and arguments.
By default, config is saved to project (.tokkibot/mcp.json). Use --global for ~/.tokkibot/mcp.json.`,
	RunE:               runAdd,
	DisableFlagParsing: false,
}

func runAdd(cmd *cobra.Command, args []string) error {
	if addTransport == "" {
		addTransport = "stdio"
	}

	scope := config.McpConfigScopeProject
	if addGlobal {
		scope = config.McpConfigScopeGlobal
	}

	switch addTransport {
	case "stdio":
		return addStdioServer(cmd, args, scope)
	case "http":
		return addHttpServer(args, scope)
	default:
		return fmt.Errorf("unsupported transport: %s (must be 'stdio' or 'http')", addTransport)
	}
}

func addStdioServer(cmd *cobra.Command, args []string, scope config.McpConfigScope) error {
	dashIndex := cmd.ArgsLenAtDash()
	if dashIndex == -1 {
		return fmt.Errorf("stdio transport requires: mcp add --transport stdio <name> -- <command> [args...]")
	}

	if dashIndex < 1 {
		return fmt.Errorf("server name is required before --")
	}

	serverName := args[dashIndex-1]
	commandArgs := args[dashIndex:]
	if len(commandArgs) == 0 {
		return fmt.Errorf("command is required after --")
	}

	envMap := parseKeyValuePairs(addEnvs)

	server := config.McpServer{
		Command: commandArgs[0],
		Args:    commandArgs[1:],
		Env:     envMap,
	}

	if err := config.AddMcpServer(scope, serverName, server); err != nil {
		return fmt.Errorf("failed to add server: %w", err)
	}

	path, _ := config.GetMcpConfigPath(scope)
	fmt.Printf("MCP server '%s' added successfully.\n", serverName)
	fmt.Printf("  Transport: stdio\n")
	fmt.Printf("  Command: %s %s\n", server.Command, strings.Join(server.Args, " "))
	if len(envMap) > 0 {
		fmt.Printf("  Env: %v\n", envMap)
	}
	fmt.Printf("  Config: %s\n", path)
	return nil
}

func addHttpServer(args []string, scope config.McpConfigScope) error {
	if len(args) < 2 {
		return fmt.Errorf("http transport requires: mcp add --transport http <name> <url>")
	}

	serverName := args[0]
	url := args[1]

	headerMap := make(map[string]string)
	for _, h := range addHeaders {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	server := config.McpServer{
		Url:     url,
		Headers: headerMap,
	}

	if err := config.AddMcpServer(scope, serverName, server); err != nil {
		return fmt.Errorf("failed to add server: %w", err)
	}

	path, _ := config.GetMcpConfigPath(scope)
	fmt.Printf("MCP server '%s' added successfully.\n", serverName)
	fmt.Printf("  Transport: http\n")
	fmt.Printf("  URL: %s\n", url)
	if len(headerMap) > 0 {
		fmt.Printf("  Headers: %v\n", headerMap)
	}
	fmt.Printf("  Config: %s\n", path)
	return nil
}

func parseKeyValuePairs(pairs []string) map[string]string {
	result := make(map[string]string)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all MCP servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.GetMcpConfig()
		if err != nil {
			fmt.Println("No MCP servers configured.")
			return nil
		}

		if len(cfg.McpServers) == 0 {
			fmt.Println("No MCP servers configured.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTRANSPORT\tCOMMAND/URL")
		fmt.Fprintln(w, "----\t---------\t-----------")

		for name, server := range cfg.McpServers {
			transport := "stdio"
			target := server.Command
			if len(server.Args) > 0 {
				target += " " + strings.Join(server.Args, " ")
			}
			if server.Url != "" {
				transport = "http"
				target = server.Url
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", name, transport, target)
		}

		w.Flush()
		return nil
	},
}

var removeGlobal bool

var removeCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove an MCP server",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := args[0]
		scope := config.McpConfigScopeProject
		if removeGlobal {
			scope = config.McpConfigScopeGlobal
		}

		if err := config.RemoveMcpServer(scope, serverName); err != nil {
			return fmt.Errorf("failed to remove server: %w", err)
		}

		fmt.Printf("MCP server '%s' removed from %s config.\n", serverName, scope)
		return nil
	},
}

func init() {
	addCmd.Flags().StringVarP(&addTransport, "transport", "t", "stdio", "Transport type (stdio or http)")
	addCmd.Flags().StringArrayVarP(&addEnvs, "env", "e", nil, "Environment variables for stdio (KEY=value)")
	addCmd.Flags().StringArrayVarP(&addHeaders, "header", "H", nil, "HTTP headers (Key: value)")
	addCmd.Flags().BoolVarP(&addGlobal, "global", "g", false, "Save to global config (~/.tokkibot/mcp.json)")

	removeCmd.Flags().BoolVarP(&removeGlobal, "global", "g", false, "Remove from global config (~/.tokkibot/mcp.json)")

	McpCmd.AddCommand(addCmd)
	McpCmd.AddCommand(listCmd)
	McpCmd.AddCommand(removeCmd)
}
