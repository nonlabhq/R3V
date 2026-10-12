package store

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Clones: copies to folder take no room until a file changes (block
// cloning: ReFS, which Dev Drive volumes are).
func Clones(folder string) bool {
	vol := filepath.VolumeName(folder)
	if vol == "" {
		return false
	}
	root, err := windows.UTF16PtrFromString(vol + `\`)
	if err != nil {
		return false
	}
	fs := make([]uint16, 64)
	if windows.GetVolumeInformation(root, nil, 0, nil, nil, nil, &fs[0], uint32(len(fs))) != nil {
		return false
	}
	return strings.EqualFold(windows.UTF16ToString(fs), "ReFS")
}
