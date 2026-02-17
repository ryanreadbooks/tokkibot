package agent

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryanreadbooks/tokkibot/agent"
	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"

	"github.com/spf13/cobra"
)

var (
	agentChatId         string
	resumeSessionChatId string

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

		return runAgent(cmd.Context(), args)
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
	AgentCmd.Flags().StringVar(&resumeSessionChatId, "resume", "", "To resume a existing session, provide the session id.")
	AgentCmd.Flags().StringVar(&oneTimeQuestion, "message", "", "To ask a one-time question, provide the message.")

	AgentSkillsCmd.AddCommand(AgentSkillsListCmd)

	AgentCmd.AddCommand(AgentSkillsCmd)
	AgentCmd.AddCommand(AgentSystemPromptCmd)
}

func runAgentOnce(ctx context.Context, message string) error {
	ag, err := prepareAgent(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare agent: %w", err)
	}

	_, err = restoreHistory(ag)
	if err != nil {
		return err
	}

	answer := ag.Ask(ctx, &agent.UserMessage{
		Channel: chmodel.ChannelCLI.String(),
		ChatId:  "one-time",
		Created: time.Now().Unix(),
		Content: message,
	})

	fmt.Println("Assistant: ", answer)

	return nil
}

func runAgent(ctx context.Context, args []string) error {
	ag, err := prepareAgent(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare agent: %w", err)
	}

	history, err := restoreHistory(ag)
	if err != nil {
		return err
	}

	fmt.Println("Session: ", agentChatId)

	mod := initAgentModel(ctx, ag, history)
	pg := tea.NewProgram(&mod,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion())
	mod.setPg(pg)
	if _, err := pg.Run(); err != nil {
		return err
	}

	fmt.Printf("\nUse %s to resume conversation\n", agentChatId)

	return nil
}

func runAgentListSkills(ctx context.Context) error {
	ag, err := prepareAgent(ctx)
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
	ag, err := prepareAgent(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare agent: %w", err)
	}

	fmt.Println(ag.GetSystemPrompt())
	return nil
}
