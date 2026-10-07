//go:build nightly

package remote

// Looks are on the Nightly channel while they're built.
func init() {
	Looks = true
	RegisterFeature(FeatureLooks)
}
