package remote

// Hosted teams (R3V Cloud), in every build since 0.2.
func init() {
	HostedTeams = true
	Register("r3v-cloud+http://", openBroker)
	Register("r3v-cloud+https://", openBroker)
}
