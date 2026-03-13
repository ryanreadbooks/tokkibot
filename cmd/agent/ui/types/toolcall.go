package types

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ryanreadbooks/tokkibot/agent/tools"
)

// ToolCallInfo represents parsed tool call information for display
type ToolCallInfo struct {
	Name      string
	Arguments map[string]any
}

// ParseToolCallArgs parses JSON arguments into a map
func ParseToolCallArgs(argsJSON string) (map[string]any, error) {
	if argsJSON == "" {
		return nil, nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, err
	}

	return args, nil
}

// FormatToolCallArgs formats tool call arguments for display
func FormatToolCallArgs(name string, argsJSON string, maxLen int) string {
	args, err := ParseToolCallArgs(argsJSON)
	if err != nil {
		// Fallback: show truncated raw JSON
		return truncateString(argsJSON, maxLen)
	}

	if len(args) == 0 {
		return "(no arguments)"
	}

	// Format based on tool type
	switch name {
	case tools.ToolNameReadFile:
		return formatReadFileArgs(args)
	case tools.ToolNameWriteFile:
		return formatWriteFileArgs(args)
	case tools.ToolNameEditFile:
		return formatEditFileArgs(args)
	case tools.ToolNameListDir:
		return formatListDirArgs(args)
	case tools.ToolNameShell:
		return formatShellArgs(args)
	case tools.ToolNameTodoWrite:
		return formatTodoWriteArgs(args)
	case tools.ToolNameSubagent:
		return formatSubagentArgs(args)
	default:
		return formatGenericArgs(args, maxLen)
	}
}

func formatReadFileArgs(args map[string]any) string {
	var parts []string

	if path, ok := args["path"].(string); ok {
		parts = append(parts, fmt.Sprintf("📄 %s", shortenPath(path, 50)))
	}

	if offset, ok := args["offset"].(float64); ok && offset > 0 {
		parts = append(parts, fmt.Sprintf("from line %d", int(offset)))
	}

	if limit, ok := args["limit"].(float64); ok && limit > 0 {
		parts = append(parts, fmt.Sprintf("%d lines", int(limit)))
	}

	if len(parts) == 0 {
		return formatGenericArgs(args, 100)
	}

	return strings.Join(parts, ", ")
}

func formatWriteFileArgs(args map[string]any) string {
	if path, ok := args["path"].(string); ok {
		var sb strings.Builder
		fmt.Fprintf(&sb, "📝 %s\n", shortenPath(path, 50))

		if content, ok := args["content"].(string); ok {
			preview := formatContentPreview(content, 15, 60)
			sb.WriteString(preview)
		}

		return sb.String()
	}
	return formatGenericArgs(args, 100)
}

func formatEditFileArgs(args map[string]any) string {
	path, _ := args["path"].(string)
	if path == "" {
		path, _ = args["file_name"].(string)
	}
	if path == "" {
		return formatGenericArgs(args, 100)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "✏️  %s\n", shortenPath(path, 50))

	oldStr, hasOld := args["old_string"].(string)
	newStr, hasNew := args["new_string"].(string)

	if hasOld && hasNew {
		sb.WriteString(formatDiff(oldStr, newStr, 10, 65))
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

func formatListDirArgs(args map[string]any) string {
	if path, ok := args["path"].(string); ok {
		return fmt.Sprintf("📁 %s", shortenPath(path, 60))
	}
	return formatGenericArgs(args, 100)
}

func formatShellArgs(args map[string]any) string {
	if cmd, ok := args["command"].(string); ok {
		return fmt.Sprintf("$ %s", cmd)
	}
	return formatGenericArgs(args, 100)
}

func formatTodoWriteArgs(args map[string]any) string {
	todosRaw, ok := args["todos"]
	if !ok {
		return formatGenericArgs(args, 100)
	}

	todos, ok := todosRaw.([]any)
	if !ok {
		return formatGenericArgs(args, 100)
	}

	if len(todos) == 0 {
		return "📋 No todos"
	}

	var sb strings.Builder
	sb.WriteString("📋 Todo List:\n")
	completed := 0
	for _, todoRaw := range todos {
		todo, ok := todoRaw.(map[string]any)
		if !ok {
			continue
		}

		content, _ := todo["content"].(string)
		status, _ := todo["status"].(string)

		marker := ""
		suffix := ""
		switch status {
		case "pending":
			marker = "[ ]"
		case "in_progress":
			marker = "[>]"
			suffix = " ← working"
		case "completed":
			marker = "[x]"
			completed++
		default:
			marker = "[ ]"
		}

		fmt.Fprintf(&sb, "  %s %s%s\n", marker, truncateString(content, 50), suffix)
	}

	sb.WriteString(fmt.Sprintf("  (%d/%d completed)", completed, len(todos)))
	return sb.String()
}

func formatSubagentArgs(args map[string]any) string {
	action, _ := args["action"].(string)

	switch action {
	case "spawn":
		name, _ := args["name"].(string)
		task, _ := args["task"].(string)
		bg, _ := args["background"].(bool)

		var sb strings.Builder
		if bg {
			sb.WriteString("🚀 Spawn (background)")
		} else {
			sb.WriteString("🚀 Spawn")
		}
		if name != "" {
			fmt.Fprintf(&sb, " [%s]", name)
		}
		if task != "" {
			sb.WriteString("\n")
			sb.WriteString(truncateString(task, 80))
		}
		return sb.String()

	case "get_result":
		namesRaw, _ := args["get_names"].([]any)
		waitMode, _ := args["wait_mode"].(string)
		if waitMode == "" {
			waitMode = "all"
		}

		names := make([]string, 0, len(namesRaw))
		for _, n := range namesRaw {
			if s, ok := n.(string); ok {
				names = append(names, s)
			}
		}

		return fmt.Sprintf("📥 Get results (wait: %s)\n%s", waitMode, strings.Join(names, ", "))

	default:
		return fmt.Sprintf("❓ Unknown action: %s", action)
	}
}

func formatGenericArgs(args map[string]any, maxLen int) string {
	var parts []string
	for k, v := range args {
		valueStr := fmt.Sprintf("%v", v)
		if len(valueStr) > 30 {
			valueStr = truncateString(valueStr, 30)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", k, valueStr))
	}

	result := strings.Join(parts, ", ")
	return truncateString(result, maxLen)
}

func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}

	// Try to show filename and parent dir
	parts := strings.Split(path, "/")
	if len(parts) > 2 {
		fileName := parts[len(parts)-1]
		parentDir := parts[len(parts)-2]
		shortened := ".../" + parentDir + "/" + fileName

		if len(shortened) <= maxLen {
			return shortened
		}
	}

	return truncateString(path, maxLen)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

func formatContentPreview(content string, maxLines int, maxLineLen int) string {
	if content == "" {
		return "(empty)"
	}

	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	if totalLines > 0 && lines[totalLines-1] == "" {
		totalLines--
		lines = lines[:totalLines]
	}

	if totalLines == 0 {
		return "(empty)"
	}

	var sb strings.Builder
	showLines := min(maxLines, totalLines)

	for i := 0; i < showLines; i++ {
		line := lines[i]
		if len(line) > maxLineLen {
			line = line[:maxLineLen-3] + "..."
		}
		fmt.Fprintf(&sb, "│ %s\n", line)
	}

	remaining := totalLines - showLines
	if remaining > 0 {
		fmt.Fprintf(&sb, "│ ... +%d lines\n", remaining)
	}

	return sb.String()
}

var (
	diffDelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b6b"))
	diffAddStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ecc71"))
	diffSepStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
)

func formatDiff(oldStr, newStr string, maxLines int, colWidth int) string {
	oldLines := splitContentLines(oldStr)
	newLines := splitContentLines(newStr)

	showOld := clampLines(len(oldLines), maxLines)
	showNew := clampLines(len(newLines), maxLines)

	leftCol := buildDiffColumn(oldLines, showOld, colWidth, "-")
	rightCol := buildDiffColumn(newLines, showNew, colWidth, "+")

	rows := len(leftCol)
	if len(rightCol) > rows {
		rows = len(rightCol)
	}
	for len(leftCol) < rows {
		leftCol = append(leftCol, "")
	}
	for len(rightCol) < rows {
		rightCol = append(rightCol, "")
	}

	sep := diffSepStyle.Render("│")
	var sb strings.Builder
	for i := 0; i < rows; i++ {
		left := padVisual(leftCol[i], colWidth+4)
		fmt.Fprintf(&sb, "  %s %s %s\n", left, sep, rightCol[i])
	}

	return sb.String()
}

func buildDiffColumn(lines []string, showCount int, colWidth int, prefix string) []string {
	var style lipgloss.Style
	if prefix == "-" {
		style = diffDelStyle
	} else {
		style = diffAddStyle
	}

	var result []string
	for i := 0; i < showCount; i++ {
		line := lines[i]
		if len(line) > colWidth {
			line = line[:colWidth-3] + "..."
		}
		result = append(result, style.Render(prefix+" "+line))
	}

	if rem := len(lines) - showCount; rem > 0 {
		result = append(result, style.Render(fmt.Sprintf("%s ... +%d lines", prefix, rem)))
	}

	return result
}

func clampLines(total, max int) int {
	if total <= max {
		return total
	}
	return max
}

// padVisual pads a string (possibly containing ANSI codes) to a visual width
func padVisual(s string, width int) string {
	vw := lipgloss.Width(s)
	if vw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vw)
}

func splitContentLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
