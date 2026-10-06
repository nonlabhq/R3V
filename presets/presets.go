// Package presets registers the project kinds that are still in testing
// (Unity, Unreal, Godot, code, design files): the Nightly channel's builds import
// it (see cmd/r3v/nightly.go); Stable builds leave them out.
package presets

import (
	_ "github.com/nonlabhq/r3v/presets/code"
	_ "github.com/nonlabhq/r3v/presets/design"
	_ "github.com/nonlabhq/r3v/presets/godot"
	_ "github.com/nonlabhq/r3v/presets/unity"
	_ "github.com/nonlabhq/r3v/presets/unreal"
)
