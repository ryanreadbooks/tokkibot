package param

import "strings"

type String struct {
	Value string `json:"value"`
}

func (p *String) GetValue() string {
	if p != nil {
		return p.Value
	}

	return ""
}

type Text struct {
	Value string `json:"value"`
}

func (p *Text) GetValue() string {
	if p != nil {
		return p.Value
	}

	return ""
}

func TextsContent(texts []*Text) string {
	var vv strings.Builder
	for _, t := range texts {
		vv.WriteString(t.GetValue())
	}

	return vv.String()
}

type ImageURL struct {
	URL string `json:"url"`
	Key string `json:"key,omitempty"`
}

func (p *ImageURL) GetURL() string {
	if p != nil {
		return p.URL
	}

	return ""
}

type ContentUnion struct {
	Text     *Text     `json:"text,omitzero"`
	ImageURL *ImageURL `json:"image_url,omitzero"`
	Key      string    `json:"key,omitempty"`
}

func (p *ContentUnion) GetContent() string {
	return p.Text.GetValue()
}

func ContentUnionsContent(c []*ContentUnion) string {
	var vv strings.Builder
	for _, u := range c {
		vv.WriteString(u.GetContent())
	}

	return vv.String()
}

type ToolCallFunction struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (p *ToolCallFunction) GetContent() string {
	if p != nil {
		return p.Id + p.Name + p.Arguments
	}

	return ""
}

type ToolCall struct {
	Function *ToolCallFunction `json:"function,omitzero"`
}

func (p *ToolCall) GetContent() string {
	return p.Function.GetContent()
}

func ToolCallsContent(tcs []*ToolCall) string {
	var vv strings.Builder
	for _, tc := range tcs {
		vv.WriteString(tc.GetContent())
	}

	return vv.String()
}

type SystemMessage struct {
	String *String `json:"string,omitzero"`
	Text   []*Text `json:"text,omitzero"`
}

func (p *SystemMessage) GetContent() string {
	if p == nil {
		return ""
	}

	if c := p.String.GetValue(); c != "" {
		return c
	}

	return TextsContent(p.Text)
}

type UserMessage struct {
	String       *String         `json:"string,omitzero"`
	ContentParts []*ContentUnion `json:"content_parts,omitzero"`
}

func (p *UserMessage) GetContent() string {
	if p == nil {
		return ""
	}

	if c := p.String.GetValue(); c != "" {
		return c
	}

	return ContentUnionsContent(p.ContentParts)
}

type AssistantMessage struct {
	Content          *String     `json:"content,omitzero"`
	Texts            []*Text     `json:"texts,omitzero"`
	ToolCalls        []*ToolCall `json:"tool_calls,omitzero"`
	ReasoningContent *String     `json:"reasoning_content,omitzero"`
}

func (p *AssistantMessage) GetContent() string {
	if p == nil {
		return ""
	}

	return p.Content.GetValue() +
		p.ReasoningContent.GetValue() +
		TextsContent(p.Texts) +
		ToolCallsContent(p.ToolCalls)
}

type ToolMessage struct {
	ToolCallId string  `json:"tool_call_id"`
	String     *String `json:"string,omitzero"`
	Texts      []*Text `json:"texts,omitzero"`
}

func (p *ToolMessage) GetContent() string {
	if p == nil {
		return ""
	}

	return p.ToolCallId + p.String.GetValue() + TextsContent(p.Texts)
}

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

func (r Role) System() bool {
	return r == RoleSystem
}

func (r Role) User() bool {
	return r == RoleUser
}

func (r Role) Assistant() bool {
	return r == RoleAssistant
}

func (r Role) Tool() bool {
	return r == RoleTool
}

type Message struct {
	System    *SystemMessage    `json:"system,omitzero"`
	User      *UserMessage      `json:"user,omitzero"`
	Assistant *AssistantMessage `json:"assistant,omitzero"`
	Tool      *ToolMessage      `json:"tool,omitzero"`
}

func (p *Message) Role() Role {
	if p.System != nil {
		return RoleSystem
	}

	if p.User != nil {
		return RoleUser
	}

	if p.Assistant != nil {
		return RoleAssistant
	}

	if p.Tool != nil {
		return RoleTool
	}

	return ""
}

func NewSystemMessage[T string | []*Text](text T) Message {
	sys := SystemMessage{}
	switch v := any(text).(type) {
	case string:
		sys.String = &String{Value: v}
	case []*Text:
		sys.Text = v
	}

	return Message{
		System: &sys,
	}
}

func NewUserMessage[T string | []*ContentUnion](msg T) Message {
	user := UserMessage{}
	switch v := any(msg).(type) {
	case string:
		user.String = &String{Value: v}
	case []*ContentUnion:
		user.ContentParts = v
	}

	return Message{
		User: &user,
	}
}

func NewAssistantMessage[T string | []*Text](
	msg T,
	toolCalls []*ToolCall,
	reasoningContent *String,
) Message {
	assistant := AssistantMessage{}
	switch v := any(msg).(type) {
	case string:
		assistant.Content = &String{Value: v}
	case []*Text:
		assistant.Texts = v
	}

	assistant.ToolCalls = toolCalls
	assistant.ReasoningContent = reasoningContent

	return Message{
		Assistant: &assistant,
	}
}

// NewToolMessage creates a tool call result message
func NewToolMessage[T string | []*Text](toolCallId string, msg T) Message {
	tool := ToolMessage{
		ToolCallId: toolCallId,
	}
	switch v := any(msg).(type) {
	case string:
		tool.String = &String{Value: v}
	case []*Text:
		tool.Texts = v
	}

	return Message{
		Tool: &tool,
	}
}
