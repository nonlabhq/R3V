//go:build nightly

package remote

// Archiving projects is on the Nightly channel while it's new.
func init() { ProjectArchive = true }
