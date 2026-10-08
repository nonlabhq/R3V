//go:build nightly

package cli

// R3V-Cloud is on the Nightly channel while it's built.
func init() {
	extraCommands["login"] = cmdLogin
	extraCommands["logout"] = cmdLogout
	extraCommands["move-team"] = cmdMoveTeam
	extraUsage = `
R3V-Cloud (hosted teams; Nightly):
  login [--service URL]                  sign in through the browser; your teams appear in ` + "`r3v teams`" + `
  logout [--service URL]                 sign this computer out
  move-team --to ADDRESS --key-id ID     move this project's team to R3V Cloud (the service copies with
            [--secret S] [--no-finish]   a read-only key; R3V_MOVE_SECRET for the secret); --finish to finish
`
}
