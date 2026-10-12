//go:build nightly

package cli

// Steps are on the Nightly channel while they're new.
func init() {
	extraCommands["steps"] = cmdSteps
	extraCommands["step"] = cmdStep
	jsonCommands["steps"] = true
	jsonCommands["step"] = true
	extraUsage += stepsUsage
}
