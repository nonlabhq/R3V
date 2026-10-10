//go:build nightly

package cli

// Agent locks are on the Nightly channel while they're new. (This file's
// init runs after cloud_nightly.go's, which sets extraUsage: files are
// initialized in name order.)
func init() {
	agentCommands()
	extraUsage += agentUsage
}
