//go:build nightly

package project

// Agent locks are on the Nightly channel while they're new.
func init() { AgentLocks = true }
