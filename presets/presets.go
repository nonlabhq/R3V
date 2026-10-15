// Package presets registers the project kinds beyond Live (Unity, Unreal,
// Godot, code, design files): the app and the command line import it (see
// cmd/r3v/kinds.go); every build since 0.2. A kind still in testing would
// come in through a Nightly-only import of its own.
package presets

import (
	_ "github.com/nonlabhq/r3v/presets/code"
	_ "github.com/nonlabhq/r3v/presets/design"
	_ "github.com/nonlabhq/r3v/presets/godot"
	_ "github.com/nonlabhq/r3v/presets/unity"
	_ "github.com/nonlabhq/r3v/presets/unreal"
)
