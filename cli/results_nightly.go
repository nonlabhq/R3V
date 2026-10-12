//go:build nightly

package cli

// Results are on the Nightly channel while they're new.
func init() {
	extraCommands["result"] = cmdResult
	extraCommands["results"] = cmdResults
	jsonCommands["result"] = true
	jsonCommands["results"] = true
	extraUsage += resultsUsage
}
