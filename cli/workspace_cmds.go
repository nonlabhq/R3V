package cli

// Workspaces (several agents on one project), in every build since 0.3.
func init() {
	extraCommands["workspace"] = cmdWorkspace
	jsonCommands["workspace"] = true
	extraUsage += `
workspaces (several agents on one project):
  workspace add [<folder>] --branch NAME [--from BRANCH|VERSION]
                                         another copy of the project on a branch of its own, made
                                         from the files here (in <project>.workspaces beside it
                                         unless <folder>); --agent claude-code sets it up for
                                         the agent (its branch an agent's, hook, R3V_ACTOR)
  workspace list                         each workspace: branch, ahead/behind its base, changes
                                         (--against BRANCH: ahead/behind that branch instead)
  workspace base <folder> <branch>       the branch a workspace merges into from now on
  workspace remove <folder> [--force]    once its branch is merged (archived then; --force: lose
                                         its work)
`
}
