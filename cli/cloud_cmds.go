package cli

// R3V Cloud's commands, in every build since 0.2 (login and logout are in
// the usage's setup section).
func init() {
	extraCommands["login"] = cmdLogin
	extraCommands["logout"] = cmdLogout
	extraCommands["move-team"] = cmdMoveTeam
	extraUsage += `
R3V Cloud (hosted teams):
  move-team --to ADDRESS --key-id ID     move this project's team to R3V Cloud (the service copies with
            [--secret S] [--no-finish]   a read-only key; R3V_MOVE_SECRET for the secret); --finish to finish
`
}
