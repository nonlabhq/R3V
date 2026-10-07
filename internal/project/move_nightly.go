//go:build nightly

package project

// Moving projects between teams is on the Nightly channel while it's new.
func init() { MoveProjects = true }
