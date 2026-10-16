//go:build nightly

package cli

// Undoing a merge is on the Nightly channel while it's new.
func init() {
	extraCommands["undo-merge"] = cmdUndoMerge
	jsonCommands["undo-merge"] = true
	extraUsage += undoMergeUsage
}
