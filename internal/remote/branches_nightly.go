//go:build nightly

package remote

// Branch records are on the Nightly channel while they're built.
func init() { BranchRecords = true }
