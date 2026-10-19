//go:build nightly

package cli

// Teams by name are on the Nightly channel while they're new.
func init() {
	teamByName = true
}
