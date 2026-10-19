package cli

// Agent locks, in every build since 0.3. (This file's init runs after
// cloud_cmds.go's, which sets extraUsage: files are initialized in name
// order.)
func init() {
	agentCommands()
	extraUsage += agentUsage
}
