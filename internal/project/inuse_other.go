//go:build !windows

package project

// inUse: another program holds the file so it can't be read now (only
// Windows locks files that way).
func inUse(error) bool { return false }
