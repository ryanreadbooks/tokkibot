package cron

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/ryanreadbooks/tokkibot/agent"
	"github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/cron"
	"github.com/spf13/cobra"
)

var CronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage cron tasks",
	Long:  "Add, delete, list, enable or disable cron tasks.",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cron tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := cron.NewManager()
		if err := mgr.Load(); err != nil {
			fmt.Printf("Warning: failed to load cron tasks: %v\n", err)
		}

		tasks := mgr.ListTasks()
		if len(tasks) == 0 {
			fmt.Println("No cron tasks found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tEXPR\tENABLED\tONCE\tDELIVER\tLAST RUN\tNEXT RUN")
		fmt.Fprintln(w, "----\t----\t-------\t----\t-------\t--------\t--------")

		for _, task := range tasks {
			lastRun := "-"
			if task.LastRun != nil {
				lastRun = task.LastRun.Format(time.RFC3339)
			}

			nextRun := "-"
			if next, ok := mgr.GetNextRun(task.Name); ok {
				nextRun = next.Format(time.RFC3339)
			}

			deliver := "-"
			if task.Deliver {
				deliver = fmt.Sprintf("%s:%s", task.DeliverChannel, task.DeliverTo)
			}

			fmt.Fprintf(w, "%s\t%s\t%v\t%v\t%s\t%s\t%s\n",
				task.Name, task.CronExpr, task.Enabled, task.OneShot, deliver, lastRun, nextRun)
		}
		w.Flush()
		return nil
	},
}

var (
	addName    string
	addExpr    string
	addPrompt  string
	addOnce    bool
	addDeliver bool
	addChannel string
	addTo      string
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new cron task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if addName == "" || addExpr == "" || addPrompt == "" {
			return fmt.Errorf("flags --name, --expr, --prompt are required")
		}

		if addDeliver && (addChannel == "" || addTo == "") {
			return fmt.Errorf("--channel and --to are required when --deliver is enabled")
		}

		var opts []cron.TaskOption
		if addOnce {
			opts = append(opts, cron.WithOneShot())
		}
		if addDeliver {
			channelType := model.Type(addChannel)
			if !model.IsCronDeliveryChannel(channelType) {
				return fmt.Errorf("unsupported channel type: %s", addChannel)
			}
			opts = append(opts, cron.WithDelivery(channelType, addTo))
		}

		task := cron.NewTask(addName, addExpr, addPrompt, opts...)

		mgr := cron.NewManager()
		if err := mgr.Load(); err != nil {
			fmt.Printf("Warning: failed to load existing tasks: %v\n", err)
		}

		updated, err := mgr.AddOrUpdateTask(task)
		if err != nil {
			return fmt.Errorf("failed to save cron task: %w", err)
		}

		action := "added"
		if updated {
			action = "updated"
		}
		fmt.Printf("Cron task '%s' %s successfully.\n", addName, action)
		fmt.Printf("  Directory: %s/%s\n", mgr.CronsDir(), addName)
		fmt.Printf("  Session ID: cron:%s\n", addName)
		if addOnce {
			fmt.Println("  One-shot: yes (will auto-disable after first run)")
		}
		if addDeliver {
			fmt.Printf("  Deliver to: %s (%s)\n", addTo, addChannel)
		}
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <task-name>",
	Short: "Delete a cron task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskName := args[0]

		mgr := cron.NewManager()
		if err := mgr.Load(); err != nil {
			return fmt.Errorf("failed to load cron tasks: %w", err)
		}

		if err := mgr.DeleteTask(taskName); err != nil {
			return fmt.Errorf("failed to delete cron task: %w", err)
		}

		fmt.Printf("Cron task '%s' deleted successfully.\n", taskName)
		return nil
	},
}

var enableCmd = &cobra.Command{
	Use:   "enable <task-name>",
	Short: "Enable a cron task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskName := args[0]

		mgr := cron.NewManager()
		if err := mgr.Load(); err != nil {
			return fmt.Errorf("failed to load cron tasks: %w", err)
		}

		if err := mgr.EnableTask(taskName); err != nil {
			return fmt.Errorf("failed to enable cron task: %w", err)
		}

		fmt.Printf("Cron task '%s' enabled.\n", taskName)
		return nil
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable <task-name>",
	Short: "Disable a cron task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskName := args[0]

		mgr := cron.NewManager()
		if err := mgr.Load(); err != nil {
			return fmt.Errorf("failed to load cron tasks: %w", err)
		}

		if err := mgr.DisableTask(taskName); err != nil {
			return fmt.Errorf("failed to disable cron task: %w", err)
		}

		fmt.Printf("Cron task '%s' disabled.\n", taskName)
		return nil
	},
}

var runCmd = &cobra.Command{
	Use:   "run <task-name>",
	Short: "Manually run a cron task once",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskName := args[0]
		ctx := cmd.Context()

		mgr := cron.NewManager()
		if err := mgr.Load(); err != nil {
			return fmt.Errorf("failed to load cron tasks: %w", err)
		}

		task, exists := mgr.GetTask(taskName)
		if !exists {
			return fmt.Errorf("task '%s' not found", taskName)
		}

		fmt.Printf("Running cron task '%s'...\n", taskName)
		fmt.Printf("Prompt: %s\n\n", task.Prompt())

		ag, err := agent.Prepare(ctx, config.CronsAgentName,
			agent.WithWorkspace(config.GetAgentWorkspaceDir(config.MainAgentName)),
			agent.WithSessionDir(config.GetCronSessionsDir()),
		)
		if err != nil {
			return fmt.Errorf("failed to prepare agent: %w", err)
		}

		// construct user message (use "cron" as channel for all cron tasks)
		userMessage := &agent.UserMessage{
			Channel: "cron",
			ChatId:  task.ChatId(),
			Content: task.Prompt(),
			Created: time.Now().Unix(),
		}

		// execute
		result := ag.Ask(ctx, userMessage)

		fmt.Println("--- Result ---")
		fmt.Println(result)

		return nil
	},
}

func init() {
	addCmd.Flags().StringVar(&addName, "name", "", "Task name")
	addCmd.Flags().StringVar(&addExpr, "expr", "", "Cron expression (e.g., '0 9 * * *' for daily at 9am)")
	addCmd.Flags().StringVar(&addPrompt, "prompt", "", "Prompt to send when triggered")
	addCmd.Flags().BoolVar(&addOnce, "once", false, "Run only once then auto-disable")
	addCmd.Flags().BoolVar(&addDeliver, "deliver", false, "Enable delivery after task completion")
	addCmd.Flags().StringVar(&addChannel, "channel", "", "Delivery channel type (lark)")
	addCmd.Flags().StringVar(&addTo, "to", "", "Delivery target (e.g., chat_id for lark)")

	CronCmd.AddCommand(listCmd)
	CronCmd.AddCommand(addCmd)
	CronCmd.AddCommand(deleteCmd)
	CronCmd.AddCommand(enableCmd)
	CronCmd.AddCommand(disableCmd)
	CronCmd.AddCommand(runCmd)
}
