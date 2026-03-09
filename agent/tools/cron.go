package tools

import (
	"context"
	"fmt"

	"github.com/ryanreadbooks/tokkibot/agent/tools/description"
	"github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/cron"
)

type CronInput struct {
	Action   string `json:"action"             jsonschema:"description=Action to perform,enum=schedule,enum=list,enum=delete"`
	Name     string `json:"name,omitempty"     jsonschema:"description=Task name (required for schedule/delete)"`
	CronExpr string `json:"cron_expr,omitempty" jsonschema:"description=Cron expression for schedule action (5 fields: minute hour day month weekday). Examples: '0 9 * * *' (daily 9am)\\, '*/5 * * * *' (every 5 min)\\, '0 0 * * 1' (every Monday)"`
	Prompt   string `json:"prompt,omitempty"   jsonschema:"description=The prompt/instruction to execute when triggered (required for schedule)"`
	OneShot  bool   `json:"one_shot,omitempty" jsonschema:"description=If true\\, task runs once then auto-disables (only for schedule action)"`
}

func Cron() tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name:        ToolNameCron,
		Description: description.CronDescription,
	}, func(ctx context.Context, meta tool.InvokeMeta, input *CronInput) (string, error) {
		mgr := cron.GetGlobalManager()
		if mgr == nil {
			return "", fmt.Errorf("cron manager not initialized (only available in gateway mode)")
		}

		switch input.Action {
		case "schedule":
			return cronSchedule(mgr, meta, input)
		case "list":
			return cronList(mgr)
		case "delete":
			return cronDelete(mgr, input)
		default:
			return "", fmt.Errorf("invalid action '%s', must be one of: schedule, list, delete", input.Action)
		}
	})
}

func cronSchedule(mgr *cron.Manager, meta tool.InvokeMeta, input *CronInput) (string, error) {
	if input.Name == "" {
		return "", fmt.Errorf("task name is required for schedule action")
	}
	if input.CronExpr == "" {
		return "", fmt.Errorf("cron expression is required for schedule action")
	}
	if input.Prompt == "" {
		return "", fmt.Errorf("prompt is required for schedule action")
	}

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
}

func cronList(mgr *cron.Manager) (string, error) {
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
}

func cronDelete(mgr *cron.Manager, input *CronInput) (string, error) {
	if input.Name == "" {
		return "", fmt.Errorf("task name is required for delete action")
	}

	if err := mgr.DeleteTask(input.Name); err != nil {
		return "", fmt.Errorf("failed to delete task: %w", err)
	}

	return fmt.Sprintf("Cron task '%s' deleted successfully.", input.Name), nil
}
