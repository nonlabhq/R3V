package cli

// The file commands for any kind of file (diff, info, merge-files), in
// every build since 0.1.81.
func init() {
	fileCommands = true
	extraCommands["merge-files"] = cmdMergeFiles
	for _, c := range []string{"diff", "info", "merge-files"} {
		jsonCommands[c] = true
	}
	extraUsage += fileDiffsUsage
}
