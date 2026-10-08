package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/project"
)

// The rules file is read in a version and now by the parser R3V follows;
// a broken file says why, a missing one says so.
func TestRulesAt(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	t.Cleanup(waitTidy)
	a := NewApp()
	root := newSong(t)
	path := filepath.Join(root, profile.FileName)
	os.WriteFile(path, []byte("presets:\n  ./: ableton\nrules:\n  - ignore: \"Renders/\"\n"), 0o644)
	if _, err := project.Init(root, "yi"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Save(root, "first", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	r, _ := project.Open(root)
	head := r.Head()

	os.WriteFile(path, []byte("requires: \"0.1.0\"\ngitignore: true\npresets:\n  ./: ableton\nrules:\n  - ignore: \"*.wav\"\n  - track: \"Renders/final.wav\"\n"), 0o644)
	then, err := a.RulesAt(root, "", head)
	if err != nil || !then.Exists || then.Error != "" || len(then.Presets) != 1 || then.Presets[0].Preset != "ableton" ||
		len(then.Rules) != 1 || then.Rules[0] != (RuleItem{Kind: "ignore", Pattern: "Renders/"}) || then.Gitignore {
		t.Fatalf("in the version: %+v %v", then, err)
	}
	now, err := a.RulesAt(root, "", "")
	if err != nil || now.Error != "" || now.Requires != "0.1.0" || !now.Gitignore || len(now.Rules) != 2 ||
		now.Rules[1] != (RuleItem{Kind: "track", Pattern: "Renders/final.wav"}) {
		t.Fatalf("now: %+v %v", now, err)
	}

	if len(now.Options) == 0 || now.Options[0].Name == "" {
		t.Fatalf("options: %+v", now.Options)
	}
	// Where it was then.
	os.WriteFile(filepath.Join(root, "old-rules.yaml"), []byte("rules:\n  - ignore: \"x/\"\n"), 0o644)
	if s, err := a.RulesAt(root, "old-rules.yaml", ""); err != nil || len(s.Rules) != 1 || s.Rules[0].Pattern != "x/" {
		t.Fatalf("elsewhere: %+v %v", s, err)
	}
	// No file in a version.
	if s, err := a.RulesAt(root, "", "none"); err != nil || s.Exists {
		t.Fatalf("none: %+v %v", s, err)
	}
	// A broken file: the reason, nothing guessed.
	os.WriteFile(path, []byte("rules:\n  - ignor: \"x\"\n"), 0o644)
	if s, err := a.RulesAt(root, "", ""); err != nil || !s.Exists || !strings.Contains(s.Error, "ignor") || len(s.Rules) != 0 {
		t.Fatalf("broken: %+v %v", s, err)
	}
	// An unknown preset is an error too, as when committing.
	os.WriteFile(path, []byte("presets:\n  ./: nosuchtool\n"), 0o644)
	if s, _ := a.RulesAt(root, "", ""); !strings.Contains(s.Error, "nosuchtool") {
		t.Fatalf("unknown preset: %+v", s)
	}
}
