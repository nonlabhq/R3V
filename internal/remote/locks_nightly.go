//go:build nightly

package remote

// File locks are on the Nightly channel while they're built.
func init() {
	FileLocks = true
	RegisterFeature(FeatureLocks)
}
