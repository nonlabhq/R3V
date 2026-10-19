package cli

// Steps, in every build since 0.3.
func init() {
	extraCommands["steps"] = cmdSteps
	extraCommands["step"] = cmdStep
	jsonCommands["steps"] = true
	jsonCommands["step"] = true
	extraUsage += stepsUsage
}
