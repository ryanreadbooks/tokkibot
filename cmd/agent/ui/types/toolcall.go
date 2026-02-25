package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolCallInfo represents parsed tool call information for display
type ToolCallInfo struct {
	Name      string
	Arguments map[string]interface{}
}

// ParseToolCallArgs parses JSON arguments into a map
func ParseToolCallArgs(argsJSON string) (map[string]interface{}, error) {
	if argsJSON == "" {
		return nil, nil
	}

	var args map[string]interface{}
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
	case "read_file":
		return formatReadFileArgs(args)
	case "write_file":
		return formatWriteFileArgs(args)
	case "edit_file":
		return formatEditFileArgs(args)
	case "list_dir":
		return formatListDirArgs(args)
	case "shell":
		return formatShellArgs(args)
	default:
		return formatGenericArgs(args, maxLen)
	}
}

func formatReadFileArgs(args map[string]interface{}) string {
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

func formatWriteFileArgs(args map[string]interface{}) string {
	if path, ok := args["path"].(string); ok {
		contentLen := 0
		if content, ok := args["content"].(string); ok {
			contentLen = len(content)
		}
		return fmt.Sprintf("📝 %s (%d bytes)", shortenPath(path, 40), contentLen)
	}
	return formatGenericArgs(args, 100)
}

func formatEditFileArgs(args map[string]interface{}) string {
	var parts []string

	if path, ok := args["path"].(string); ok {
		parts = append(parts, fmt.Sprintf("✏️  %s", shortenPath(path, 40)))
	}

	if oldStr, ok := args["old_string"].(string); ok {
		parts = append(parts, fmt.Sprintf("replace %d chars", len(oldStr)))
	}

	if newStr, ok := args["new_string"].(string); ok {
		parts = append(parts, fmt.Sprintf("with %d chars", len(newStr)))
	}

	if len(parts) == 0 {
		return formatGenericArgs(args, 100)
	}

	return strings.Join(parts, ", ")
}

func formatListDirArgs(args map[string]interface{}) string {
	if path, ok := args["path"].(string); ok {
		return fmt.Sprintf("📁 %s", shortenPath(path, 60))
	}
	return formatGenericArgs(args, 100)
}

func formatShellArgs(args map[string]interface{}) string {
	if cmd, ok := args["command"].(string); ok {
		return fmt.Sprintf("$ %s", truncateString(cmd, 70))
	}
	return formatGenericArgs(args, 100)
}

func formatGenericArgs(args map[string]interface{}, maxLen int) string {
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
