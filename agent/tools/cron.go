package tools

import (
	"context"
	"fmt"

	"github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/cron"
)

type ScheduleCronInput struct {
	Name     string `json:"name"               jsonschema:"description=Unique task name for identification"`
	CronExpr string `json:"cron_expr"          jsonschema:"description=Cron expression (5 fields: minute hour day month weekday). Examples: '0 9 * * *' (daily 9am)\\, '*/5 * * * *' (every 5 min)\\, '0 0 * * 1' (every Monday)"`
	Prompt   string `json:"prompt"             jsonschema:"description=The prompt/instruction to execute when triggered"`
	OneShot  bool   `json:"one_shot,omitempty" jsonschema:"description=If true\\, task runs once then auto-disables. Use for one-time scheduled tasks"`
}

func ScheduleCron() tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name: "schedule_cron",
		Description: "Schedule a cron task that runs periodically or once at a specified time. " +
			"Results will be delivered to the current chat. " +
			"Use one_shot=true for one-time scheduled tasks that auto-disable after execution.",
	}, func(ctx context.Context, meta tool.InvokeMeta, input *ScheduleCronInput) (string, error) {
		if input.Name == "" {
			return "", fmt.Errorf("task name is required")
		}
		if input.CronExpr == "" {
			return "", fmt.Errorf("cron expression is required")
		}
		if input.Prompt == "" {
			return "", fmt.Errorf("prompt is required")
		}

		mgr := cron.GetGlobalManager()
		if mgr == nil {
			return "", fmt.Errorf("cron manager not initialized (only available in gateway mode)")
		}

		// auto-configure delivery to current channel/chat
		channelType := model.Type(meta.Channel)
		opts := []cron.TaskOption{
			cron.WithDelivery(channelType, meta.ChatId),
		}
		if input.OneShot {
			opts = append(opts, cron.WithOneShot())
		}

		task := cron.NewTask(input.Name, input.CronExpr, input.Prompt, opts...)

		updated, err := mgr.AddOrUpdateTask(task)
		if err != nil {
			return "", fmt.Errorf("failed to schedule task: %w", err)
		}

		action := "created and scheduled"
		if updated {
			action = "updated and rescheduled"
		}

		result := fmt.Sprintf("Cron task '%s' %s successfully.\n", input.Name, action)
		result += fmt.Sprintf("  Expression: %s\n", input.CronExpr)
		result += fmt.Sprintf("  Deliver to: %s (channel: %s)\n", meta.ChatId, meta.Channel)
		if input.OneShot {
			result += "  One-shot: yes (will auto-disable after first run)\n"
		}

		return result, nil
	})
}

type ListCronInput struct{}

func ListCron() tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name:        "list_cron",
		Description: "List all scheduled cron tasks with their status, expression, and next run time.",
	}, func(ctx context.Context, meta tool.InvokeMeta, input *ListCronInput) (string, error) {
		mgr := cron.GetGlobalManager()
		if mgr == nil {
			return "", fmt.Errorf("cron manager not initialized (only available in gateway mode)")
		}

		tasks := mgr.ListTasks()
		if len(tasks) == 0 {
			return "No cron tasks found.", nil
		}

		result := "Cron Tasks:\n"
		result += "| Name | Expression | Enabled | OneShot | Deliver | Last Run |\n"
		result += "|------|------------|---------|---------|---------|----------|\n"

		for _, task := range tasks {
			lastRun := "-"
			if task.LastRun != nil {
				lastRun = task.LastRun.Format("2006-01-02 15:04")
			}

			deliver := "-"
			if task.Deliver {
				deliver = fmt.Sprintf("%s:%s", task.DeliverChannel, task.DeliverTo)
			}

			result += fmt.Sprintf("| %s | %s | %v | %v | %s | %s |\n",
				task.Name, task.CronExpr, task.Enabled, task.OneShot, deliver, lastRun)
		}

		return result, nil
	})
}

type DeleteCronInput struct {
	Name string `json:"name" jsonschema:"description=Name of the cron task to delete"`
}

func DeleteCron() tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name:        "delete_cron",
		Description: "Delete a scheduled cron task by name.",
	}, func(ctx context.Context, meta tool.InvokeMeta, input *DeleteCronInput) (string, error) {
		if input.Name == "" {
			return "", fmt.Errorf("task name is required")
		}

		mgr := cron.GetGlobalManager()
		if mgr == nil {
			return "", fmt.Errorf("cron manager not initialized (only available in gateway mode)")
		}

		if err := mgr.DeleteTask(input.Name); err != nil {
			return "", fmt.Errorf("failed to delete task: %w", err)
		}

		return fmt.Sprintf("Cron task '%s' deleted successfully.", input.Name), nil
	})
}
