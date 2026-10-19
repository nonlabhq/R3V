package cli

// Undoing a merge, in every build since 0.3.
func init() {
	extraCommands["undo-merge"] = cmdUndoMerge
	jsonCommands["undo-merge"] = true
	extraUsage += undoMergeUsage
}
