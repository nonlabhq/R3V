//go:build nightly

package project

// Parking changes when switching is on the Nightly channel while it's new.
func init() { Parking = true }
