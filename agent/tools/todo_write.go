package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ryanreadbooks/tokkibot/agent/tools/description"
	"github.com/ryanreadbooks/tokkibot/component/tool"
)

// in-memory todo manager
type TodoManager struct {
	mu    sync.RWMutex
	todos map[string][]TodoWriteItem
}

var todoManager = &TodoManager{
	todos: make(map[string][]TodoWriteItem),
}

func (m *TodoManager) GetTodos(key string) []TodoWriteItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.todos[key]
}

func (m *TodoManager) SetTodos(key string, todos []TodoWriteItem) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.todos[key] = todos
}

func renderTodos(todos []TodoWriteItem) string {
	if len(todos) == 0 {
		return "No todos"
	}

	var buf strings.Builder
	finished := 0
	for _, todo := range todos {
		marker := ""
		workingInProgress := ""
		switch todo.Status {
		case todoStatusPending:
			marker = "[ ]"
		case todoStatusInProgress:
			marker = "[>]"
			workingInProgress = " <- (working in progress)"
		case todoStatusCompleted:
			marker = "[x]"
			finished++
		}

		fmt.Fprintf(&buf, "%s %s%s\n", marker, todo.Content, workingInProgress)
	}
	if finished == len(todos) {
		fmt.Fprintf(&buf, "%d/%d todos completed", finished, len(todos))
	}

	return buf.String()
}

type todoStatus string

const (
	todoStatusPending    todoStatus = "pending"
	todoStatusInProgress todoStatus = "in_progress"
	todoStatusCompleted  todoStatus = "completed"
)

func isValidTodoStatus(status todoStatus) bool {
	return status == todoStatusPending || status == todoStatusInProgress || status == todoStatusCompleted
}

type TodoWriteItem struct {
	Id      string     `json:"id"      jsonschema:"description=The id of the todo item"`
	Content string     `json:"content" jsonschema:"description=The content of the todo item,minLength=1"`
	Status  todoStatus `json:"status"  jsonschema:"description=The status of the todo item,enum=pending,enum=in_progress,enum=completed"`
}

type TodoWriteInput struct {
	Todos []TodoWriteItem `json:"todos" jsonschema:"description=The updated todo list"`
}

func doTodoWriteInvoke(ctx context.Context, meta tool.InvokeMeta, input *TodoWriteInput) (string, error) {
	if len(input.Todos) > 10 {
		return "", fmt.Errorf("todo list too long, max 10 todos")
	}

	// validate todos from llm is valid
	validTodo := make([]TodoWriteItem, 0, len(input.Todos))
	progressCount := 0
	for _, todo := range input.Todos {
		content := strings.TrimSpace(todo.Content)
		status := strings.TrimSpace(strings.ToLower(string(todo.Status)))
		id := todo.Id
		if content == "" {
			return "", fmt.Errorf("todo item %s requires content", id)
		}

		if !isValidTodoStatus(todoStatus(status)) {
			return "", fmt.Errorf("invalid todo status: %s", status)
		}

		if status == string(todoStatusInProgress) {
			progressCount++
			if progressCount > 1 {
				return "", fmt.Errorf("only one todo can be in progress at a time")
			}
		}

		validTodo = append(validTodo, TodoWriteItem{
			Id:      id,
			Content: content,
			Status:  todoStatus(status),
		})
	}
	key := fmt.Sprintf("%s-%s", meta.Channel, meta.ChatId)
	todoManager.SetTodos(key, validTodo)
	todoManager.SetTodos(key, validTodo)

	return renderTodos(validTodo), nil
}

func TodoWrite() tool.Invoker {
	info := tool.Info{
		Name:        ToolNameTodoWrite,
		Description: description.TodoWriteDescription,
	}

	return tool.NewInvoker(info, doTodoWriteInvoke)
}
