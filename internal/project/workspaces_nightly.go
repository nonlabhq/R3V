//go:build nightly

package project

// Making workspaces is on the Nightly channel while it's new.
func init() { AgentWorkspaces = true }
