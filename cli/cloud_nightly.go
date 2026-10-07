//go:build nightly

package cli

// R3V-Cloud is on the Nightly channel while it's built.
func init() {
	extraCommands["login"] = cmdLogin
	extraCommands["logout"] = cmdLogout
	extraUsage = `
R3V-Cloud (hosted teams; Nightly):
  login [--service URL]                  sign in through the browser; your teams appear in ` + "`r3v teams`" + `
  logout [--service URL]                 sign this computer out
`
}
