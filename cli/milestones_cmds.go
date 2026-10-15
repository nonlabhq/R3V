package cli

// Milestones (in every build since 0.1.81); tag and tags
// are what an agent that knows Git tries first.
func init() {
	for _, c := range []string{"milestone", "tag"} {
		extraCommands[c] = cmdMilestone
		jsonCommands[c] = true
	}
	for _, c := range []string{"milestones", "tags"} {
		extraCommands[c] = cmdMilestones
		jsonCommands[c] = true
	}
	extraUsage += milestonesUsage
}
