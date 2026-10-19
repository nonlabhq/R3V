package profile

import (
	"errors"
	"fmt"
	"strings"
)

// ProjectHandlers: .r3v.yaml may say which built-in merge takes which files
// (handlers:). In every build since 0.2 (handlers_on.go).
var ProjectHandlers bool

// MergeWhole in a project's handlers: the file is chosen whole when both
// sides changed it (no merge), whatever a preset says.
const MergeWhole = "whole"

// projectMerges are the merges a project's handlers can pick: those that
// need nothing but the file.
var projectMerges = []string{"text", MergeWhole}

func checkProjectHandler(h Handler) error {
	if len(h.Files) == 0 {
		return errors.New("files: names no files")
	}
	for _, pat := range h.Files {
		if err := checkPattern(pat); err != nil {
			return err
		}
	}
	if h.Samples != "" {
		return errors.New("samples: is for presets only")
	}
	for _, m := range projectMerges {
		if h.Merge == m {
			return nil
		}
	}
	return fmt.Errorf("merge: %q is not one a project can pick (%s)", h.Merge, strings.Join(projectMerges, ", "))
}
