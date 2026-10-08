package profile

import (
	"reflect"
	"strings"
	"testing"
)

func TestFileLocksParse(t *testing.T) {
	cases := []struct {
		name, yaml string
		want       FileLocks
		err        string
	}{
		{"none", "rules: []\n", FileLocks{}, ""},
		{"off", "file_locks:\n  enabled: false\n", FileLocks{Disabled: true}, ""},
		{"on says nothing", "file_locks:\n  enabled: true\n", FileLocks{}, ""},
		{"manual only", "file_locks:\n  auto_lock: off\n", FileLocks{AutoOff: true}, ""},
		{"manual only (false)", "file_locks:\n  auto_lock: false\n", FileLocks{AutoOff: true}, ""},
		{"add and remove", "file_locks:\n  auto_lock:\n    add: [\"Content/Maps/**\"]\n    remove: [\"*.uasset\"]\n",
			FileLocks{Add: []string{"Content/Maps/**"}, Remove: []string{"*.uasset"}}, ""},
		{"bad scalar", "file_locks:\n  auto_lock: sometimes\n", FileLocks{}, "auto_lock is off"},
		{"bad pattern", "file_locks:\n  auto_lock:\n    add: [\"[\"]\n", FileLocks{}, "bad pattern"},
		{"misspelt", "file_locks:\n  enable: false\n", FileLocks{}, "unknown field"},
		{"misspelt list", "file_locks:\n  auto_lock:\n    adds: [\"*.x\"]\n", FileLocks{}, "adds"},
	}
	for _, c := range cases {
		p, err := Parse([]byte(c.yaml), t.TempDir())
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%s: error %v, want %q", c.name, err, c.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !reflect.DeepEqual(p.FileLocks, c.want) {
			t.Errorf("%s: %+v, want %+v", c.name, p.FileLocks, c.want)
		}
	}
}

func TestLockingTeamTimesProject(t *testing.T) {
	team := TeamLocks{On: true, Kinds: []string{"unreal", "blender"}}
	cases := []struct {
		name    string
		team    TeamLocks
		project FileLocks
		on      bool
		teamOff bool
		projOff bool
		auto    map[string]bool
	}{
		{"team off", TeamLocks{Kinds: []string{"unreal"}}, FileLocks{}, false, true, false,
			map[string]bool{"Content/Maps/Harbor.umap": false}},
		{"team off, project can't turn on", TeamLocks{}, FileLocks{Add: []string{"*.png"}}, false, true, false,
			map[string]bool{"a.png": false}},
		{"team on", team, FileLocks{}, true, false, false,
			map[string]bool{"Content/Maps/Harbor.umap": true, "Art/ship.blend": true, "Song.als": false, "x.psd": false}},
		{"team on, manual only", TeamLocks{On: true, Kinds: []string{}}, FileLocks{}, true, false, false,
			map[string]bool{"Content/Maps/Harbor.umap": false}},
		{"project off", team, FileLocks{Disabled: true}, false, false, true,
			map[string]bool{"Content/Maps/Harbor.umap": false}},
		{"project manual only", team, FileLocks{AutoOff: true}, true, false, false,
			map[string]bool{"Content/Maps/Harbor.umap": false}},
		{"project adds and removes", team, FileLocks{Add: []string{"Content/Maps/**"}, Remove: []string{"*.uasset", "Content/Dev/"}}, true, false, false,
			map[string]bool{"Content/Maps/Harbor.umap": true, "Content/Maps/notes.txt": true, "Content/Hero.uasset": false,
				"Content/Dev/Test.umap": false, "Art/ship.blend": true}},
	}
	for _, c := range cases {
		l := ResolveLocking(c.team, c.project)
		if l.On != c.on || l.TeamOff != c.teamOff || l.ProjectOff != c.projOff {
			t.Errorf("%s: %+v", c.name, l)
		}
		for p, want := range c.auto {
			if got := l.AutoLocks(p); got != want {
				t.Errorf("%s: AutoLocks(%s) = %v", c.name, p, got)
			}
		}
	}
}

func TestDefaultLockKinds(t *testing.T) {
	want := []string{"unreal", "unity", "godot", "blender", "source-art"}
	if got := DefaultLockKinds(); !reflect.DeepEqual(got, want) {
		t.Errorf("defaults: %v", got)
	}
	l := ResolveLocking(TeamLocks{On: true, Kinds: DefaultLockKinds()}, FileLocks{})
	for p, want := range map[string]bool{"a.umap": true, "b.prefab": true, "c.scn": true, "d.tscn": false, "e.psd": true,
		"f.als": false, "g.wav": false, "h.png": false} {
		if l.AutoLocks(p) != want {
			t.Errorf("%s: %v", p, !want)
		}
	}
}
