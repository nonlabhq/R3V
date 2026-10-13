//go:build nightly

package cli

// Milestones are on the Nightly channel while they're new; tag and tags
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
