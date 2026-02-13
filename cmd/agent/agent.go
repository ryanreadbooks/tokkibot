package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/ryanreadbooks/tokkibot/agent"
	channelmodel "github.com/ryanreadbooks/tokkibot/channel/model"

	tea "github.com/charmbracelet/bubbletea"
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
	ag, bus, err := prepareAgent(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare agent: %w", err)
	}

	_, err = restoreHistory(ag)
	if err != nil {
		return err
	}

	ag.Run(ctx) // run in background

	answerChan := make(chan string)

	go func() {
		select {
		case <-ctx.Done():
			return
		case msg := <-bus.GetOutgoingChannel(channelmodel.ChannelCLI).Wait(ctx):
			answerChan <- msg.Content
		}
	}()

	err = bus.GetIncomingChannel(channelmodel.ChannelCLI).Send(ctx, channelmodel.IncomingMessage{
		Channel: channelmodel.ChannelCLI,
		ChatId:  "one-time",
		Created: time.Now().Unix(),
		Content: message,
	})
	if err != nil {
		return fmt.Errorf("failed to send message to agent: %w", err)
	}

	answer := <-answerChan
	fmt.Println("Agent: ", answer)

	return nil
}

func runAgent(ctx context.Context, args []string) error {
	ag, bus, err := prepareAgent(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare agent: %w", err)
	}

	// Create channel for tool call notifications to UI
	toolCallCh := make(chan toolCallMsg, 64)
	ag.SubscribeToolCalling(func(name, argFragment string, state agent.ThinkingState) {
		select {
		case toolCallCh <- toolCallMsg{
			Name:        name,
			ArgFragment: argFragment,
			Done:        state == agent.ThinkingStateDone,
		}:
		default:
			// discard if channel full
		}
	})

	err = ag.RunStream(ctx) // run in background with streaming
	if err != nil {
		return fmt.Errorf("failed to run agent with streaming: %w", err)
	}

	history, err := restoreHistory(ag)
	if err != nil {
		return err
	}

	fmt.Println("session: ", agentChatId)

	p := tea.NewProgram(
		initAgentModel(ctx, ag, bus, history, toolCallCh),
		tea.WithAltScreen(), // Use alternate screen buffer, clears on exit
	)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run agent: %w", err)
	}

	return nil
}

func runAgentListSkills(ctx context.Context) error {
	ag, _, err := prepareAgent(ctx)
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
	ag, _, err := prepareAgent(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare agent: %w", err)
	}

	fmt.Println(ag.GetSystemPrompt())
	return nil
}
