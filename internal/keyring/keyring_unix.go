//go:build !windows && !darwin && !linux

package keyring

func set(target, _ string, secret string) error { return fileSet(target, secret) }
func get(target string) (string, error)         { return fileGet(target) }
func del(target string) error                   { return fileDel(target) }
