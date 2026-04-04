package tools

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(sessionID string, args map[string]any) (string, error)
}
