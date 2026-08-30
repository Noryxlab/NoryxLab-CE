//go:build !enterprise

package handlers

// A Community workspace starts without an assistant configuration. Returning
// an empty string rather than an error keeps the launch path identical
// between editions: the workspace opens, it simply has no assistant.
func (h Handlers) workspaceAssistantConfig(_, _, _, _ string) (string, error) {
	return "", nil
}
