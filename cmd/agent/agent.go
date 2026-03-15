package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/ryanreadbooks/tokkibot/agent"
	cliadapter "github.com/ryanreadbooks/tokkibot/channel/adapter/cli"
	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/tui"
	cfg "github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/gateway"

	"github.com/spf13/cobra"
)

var (
	agentChatId         string
	resumeSessionChatId string
	agentName           string

	oneTimeQuestion string
)

var AgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Interact with tokkibot agent in a CLI.",
	Long:  "Interact with tokkibot agent in a CLI.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if oneTimeQuestion != "" {
			return runAgentOnce(cmd.Context(), oneTimeQuestion)
		}
		return runAgent(cmd.Context())
	},
}

var (
	AgentSkillsCmd = &cobra.Command{
		Use:   "skills",
		Short: "Manage available skills.",
		Long:  "Manage available skills.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	AgentSkillsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all available skills.",
		Long:  "List all available skills.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentListSkills(cmd.Context())
		},
	}
)

var AgentSystemPromptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Show the system prompt.",
	Long:  "Show the system prompt.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAgentSystemPrompt(cmd.Context())
	},
}

func init() {
	AgentCmd.PersistentFlags().StringVar(&agentName, "agent", cfg.MainAgentName, "Agent name to use")
	AgentCmd.Flags().StringVar(&resumeSessionChatId, "resume", "", "To resume a existing session, provide the session id.")
	AgentCmd.Flags().StringVar(&oneTimeQuestion, "message", "", "To ask a one-time question, provide the message.")

	AgentSkillsCmd.AddCommand(AgentSkillsListCmd)

	AgentCmd.AddCommand(AgentSkillsCmd)
	AgentCmd.AddCommand(AgentSystemPromptCmd)
}

func runAgentOnce(ctx context.Context, message string) error {
	slog.Info("[cmd/agent] running one-time agent", slog.String("agent", agentName), slog.Int("message_len", len(message)))

	gw, err := gateway.NewGateway(ctx,
		gateway.WithAgentNames([]string{agentName}),
		gateway.WithDisableAutoMessageDelivery(),
		gateway.WithEnableCwdAccess(true),
	)
	if err != nil {
		slog.Error("[cmd/agent] failed to create gateway", slog.Any("error", err))
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	cliAdapter := cliadapter.NewAdapter(cliadapter.CLIConfig{
		ChatID: "one-time",
	})
	gw.AddAdapter(cliAdapter, agentName)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		_ = gw.Run(ctx)
	}()

	if err := gw.GetAgent(agentName).InitSession(chmodel.CLI.String(), "one-time"); err != nil {
		return err
	}

	return tui.RunWithSpinner(ctx, gw.GetAgent(agentName), cliAdapter, message)
}

func runAgent(ctx context.Context) error {
	slog.Info("[cmd/agent] starting interactive agent", slog.String("agent", agentName), slog.String("resume_session", resumeSessionChatId))

	gw, err := gateway.NewGateway(ctx,
		gateway.WithAgentNames([]string{agentName}),
		gateway.WithDisableAutoMessageDelivery(),
		gateway.WithEnableCwdAccess(true),
	)
	if err != nil {
		slog.Error("[cmd/agent] failed to create gateway", slog.Any("error", err))
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	if resumeSessionChatId == "" {
		agentChatId = uuid.New().String()
	} else {
		agentChatId = resumeSessionChatId
	}
	slog.Info("[cmd/agent] session initialized", slog.String("chat_id", agentChatId))

	cliAdapter := cliadapter.NewAdapter(cliadapter.CLIConfig{
		ChatID: agentChatId,
	})
	gw.AddAdapter(cliAdapter, agentName)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		_ = gw.Run(ctx)
	}()

	if err := tui.Run(ctx, gw.GetAgent(agentName), cliAdapter); err != nil {
		return err
	}

	fmt.Printf("\nBye, Use %s to resume conversation\n", agentChatId)

	return nil
}

func runAgentListSkills(ctx context.Context) error {
	ag, err := agent.Prepare(ctx, agentName)
	if err != nil {
		return fmt.Errorf("failed to prepare agent: %w", err)
	}

	skills := ag.AvailableSkills()
	if len(skills) == 0 {
		fmt.Println("No skills available.")
		return nil
	}

	for _, skill := range skills {
		fmt.Println("Name: ", skill.Name())
		fmt.Println("Description: ", skill.Description())
		fmt.Println("--------------------------------")
	}

	return nil
}

func runAgentSystemPrompt(ctx context.Context) error {
	ag, err := agent.Prepare(ctx, agentName)
	if err != nil {
		return fmt.Errorf("failed to prepare agent: %w", err)
	}

	fmt.Println(ag.GetSystemPrompt())
	return nil
}
