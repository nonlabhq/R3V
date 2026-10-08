package profile

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// File locks (docs/design/locks.md). A team turns file locking on (manual
// locks, which the service enforces) and picks which kinds of file lock by
// themselves when they change (auto-lock). A project's .r3v.yaml can only
// narrow that, never widen it:
//
//	file_locks:
//	  enabled: false          # this project: no locks at all
//	  auto_lock:              # adds to / removes from the team's kinds
//	    add: ["Content/Maps/**"]
//	    remove: ["*.uasset"]
//	  # or: auto_lock: off    # this project: manual locks only

// LockKind is a kind of file a team can have lock by itself.
type LockKind struct {
	ID       string   `json:"id"`
	Patterns []string `json:"patterns"`
	// Default: auto-locked when a team turns locking on.
	Default bool `json:"default"`
}

// LockKinds are the kinds of file a team picks from, in the order offered.
// Live sets aren't one: R3V merges them track by track.
var LockKinds = []LockKind{
	{"unreal", []string{"*.umap", "*.uasset"}, true},
	{"unity", []string{"*.unity", "*.prefab", "*.asset"}, true},
	{"godot", []string{"*.scn", "*.res"}, true}, // binary scenes and resources
	{"blender", []string{"*.blend"}, true},
	{"source-art", []string{"*.psd", "*.psb", "*.kra", "*.spp", "*.ma", "*.mb", "*.max", "*.c4d"}, true},
	{"godot-text", []string{"*.tscn", "*.tres"}, false},
	{"images", []string{"*.png", "*.jpg", "*.jpeg", "*.tga", "*.tif", "*.tiff", "*.exr"}, false},
	{"models", []string{"*.fbx", "*.obj", "*.gltf", "*.glb", "*.usd", "*.usdc", "*.abc"}, false},
	{"audio", []string{"*.wav", "*.aif", "*.aiff", "*.flac", "*.mp3", "*.ogg"}, false},
	{"video", []string{"*.mp4", "*.mov", "*.mkv", "*.avi"}, false},
}

// DefaultLockKinds are the kinds auto-locked when a team turns locking on.
func DefaultLockKinds() []string {
	var out []string
	for _, k := range LockKinds {
		if k.Default {
			out = append(out, k.ID)
		}
	}
	return out
}

// FileLocks is what a project's .r3v.yaml says about file locks.
type FileLocks struct {
	// Disabled: enabled: false (no locks at all in this project).
	Disabled bool `json:"disabled"`
	// AutoOff: auto_lock: off (manual locks only).
	AutoOff bool     `json:"autoOff"`
	Add     []string `json:"add"`
	Remove  []string `json:"remove"`
}

// Set says whether the file says anything about file locks.
func (f FileLocks) Set() bool {
	return f.Disabled || f.AutoOff || len(f.Add) > 0 || len(f.Remove) > 0
}

// fileLocks is the shape of file_locks: in .r3v.yaml.
type fileLocks struct {
	Enabled  *bool    `yaml:"enabled"`
	AutoLock autoLock `yaml:"auto_lock"`
}

type autoLock struct {
	Off         bool
	Add, Remove []string
}

func (a *autoLock) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		switch strings.ToLower(strings.TrimSpace(n.Value)) {
		case "off", "false", "no", "none":
			a.Off = true
			return nil
		}
		return fmt.Errorf("line %d: auto_lock is off, or add: and remove: lists", n.Line)
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i < len(n.Content); i += 2 {
			if k := n.Content[i]; k.Value != "add" && k.Value != "remove" {
				return fmt.Errorf("line %d: unknown field %q", k.Line, k.Value)
			}
		}
	}
	var v struct {
		Add    []string `yaml:"add"`
		Remove []string `yaml:"remove"`
	}
	if err := n.Decode(&v); err != nil {
		return err
	}
	a.Add, a.Remove = v.Add, v.Remove
	return nil
}

func (f *fileLocks) resolve() (FileLocks, error) {
	out := FileLocks{AutoOff: f.AutoLock.Off, Add: f.AutoLock.Add, Remove: f.AutoLock.Remove}
	if f.Enabled != nil && !*f.Enabled {
		out.Disabled = true
	}
	for _, list := range [][]string{out.Add, out.Remove} {
		for _, p := range list {
			if err := checkPattern(p); err != nil {
				return FileLocks{}, fmt.Errorf("file_locks: auto_lock: %w", err)
			}
		}
	}
	if out.AutoOff && (len(out.Add) > 0 || len(out.Remove) > 0) {
		return FileLocks{}, errors.New("file_locks: auto_lock is either off or add:/remove: lists")
	}
	return out, nil
}

// TeamLocks is a team's side: file locking on, and its auto-lock kinds.
type TeamLocks struct {
	On    bool
	Kinds []string
}

// Locking is what file locks do in a project: the team's settings with the
// project's own on top.
type Locking struct {
	// On: locks exist here (the team turned locking on, the project didn't
	// turn it off).
	On bool `json:"on"`
	// TeamOff: the team hasn't turned locking on (whatever the project says).
	TeamOff bool `json:"teamOff"`
	// ProjectOff: the project's .r3v.yaml turns locks off.
	ProjectOff bool `json:"projectOff"`
	// Auto: the patterns that lock by themselves (empty: manual locks only);
	// Except: those that don't, whatever Auto says.
	Auto   []string `json:"auto"`
	Except []string `json:"except"`
}

// Locking resolves the project's file locks against the team's.
func (p *Profile) Locking(team TeamLocks) Locking {
	return ResolveLocking(team, p.FileLocks)
}

// ResolveLocking resolves a project's file locks against the team's: a
// project only narrows them.
func ResolveLocking(team TeamLocks, f FileLocks) Locking {
	out := Locking{Auto: []string{}, Except: []string{}}
	switch {
	case !team.On:
		out.TeamOff = true
		return out
	case f.Disabled:
		out.ProjectOff = true
		return out
	}
	out.On = true
	if f.AutoOff {
		return out
	}
	for _, id := range team.Kinds {
		for _, k := range LockKinds {
			if k.ID == id {
				out.Auto = append(out.Auto, k.Patterns...)
			}
		}
	}
	out.Auto = append(out.Auto, f.Add...)
	out.Except = append(out.Except, f.Remove...)
	return out
}

// AutoLocks says whether a change to rel (slash separated, relative to the
// project folder) locks it by itself.
func (l Locking) AutoLocks(rel string) bool {
	if !l.On {
		return false
	}
	for _, p := range l.Except {
		if matchPath(p, rel, false) {
			return false
		}
	}
	for _, p := range l.Auto {
		if matchPath(p, rel, false) {
			return true
		}
	}
	return false
}
