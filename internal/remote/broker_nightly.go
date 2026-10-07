//go:build nightly

package remote

// Hosted teams (R3V-Cloud) are on the Nightly channel while they're built.
func init() {
	Register("r3v-cloud+http://", openBroker)
	Register("r3v-cloud+https://", openBroker)
}
