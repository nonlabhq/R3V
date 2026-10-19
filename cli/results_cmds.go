package cli

// Results, in every build since 0.3.
func init() {
	extraCommands["result"] = cmdResult
	extraCommands["results"] = cmdResults
	jsonCommands["result"] = true
	jsonCommands["results"] = true
	extraUsage += resultsUsage
}
