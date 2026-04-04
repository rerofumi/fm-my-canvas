package types

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	CreatedAt  string     `json:"created_at"`
}

type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Message `json:"messages"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}
