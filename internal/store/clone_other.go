//go:build !windows

package store

// Clones: copies to folder take no room until a file changes (only known
// on Windows: ReFS).
func Clones(folder string) bool { return false }
