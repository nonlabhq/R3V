//go:build nightly

package cli

// diff of a project's files is on the Nightly channel while it's new.
func init() {
	fileDiffs = true
	jsonCommands["diff"] = true
	extraUsage += fileDiffsUsage
}
