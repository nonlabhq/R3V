package desktop

import (
	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
)

// RulesState is a project's .r3v.yaml in one version, read by the same
// parser R3V follows: what the file viewer shows in plain words, and
// compares rule by rule.
type RulesState struct {
	Exists    bool                  `json:"exists"`    // false: no rules file in that version
	Error     string                `json:"error"`     // it can't be read: shown as text instead
	Requires  string                `json:"requires"`  // the oldest R3V that understands it
	Gitignore bool                  `json:"gitignore"` // follows the project's .gitignore files
	Presets   []profile.PresetEntry `json:"presets"`
	Rules     []RuleItem            `json:"rules"`
	Options   []PresetOption        `json:"options"` // the presets R3V knows, with what they leave out
	// FileLocks: what the file says about file locks; TeamLocks: the
	// team's own (nil in a build without locks): together, what locks do.
	FileLocks profile.FileLocks `json:"fileLocks"`
	TeamLocks *TeamLockSide     `json:"teamLocks"`
}

// TeamLockSide is a team's file locks, for reading a project's rules.
type TeamLockSide struct {
	On    bool     `json:"on"`
	Kinds []string `json:"kinds"`
}

// teamLockSide reads the switch of the project's team (off for a team on
// its own storage; nil when this build has no locks).
func teamLockSide(r *project.Repo) *TeamLockSide {
	if !remote.FileLocks {
		return nil
	}
	out := &TeamLockSide{Kinds: []string{}}
	if r.Config.Remote == nil {
		return out
	}
	if t, err := r.Team(); err == nil {
		if s, err := teamLockSettings(t, false); err == nil || s.On {
			out.On, out.Kinds = s.On, s.Kinds
		}
	}
	return out
}

// RulesAt reads a project's rules file in a version (a version id, "" for
// the project folder now, "none" for no file); file is where it was then
// ("" for .r3v.yaml).
func (a *App) RulesAt(root, file, version string) (*RulesState, error) {
	r, unlock, err := a.open(root)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if file == "" {
		file = profile.FileName
	}
	data, err := readText(r, file, version)
	if err != nil {
		return nil, err
	}
	out := rulesState(data, r.Root)
	out.TeamLocks = teamLockSide(r)
	return out, nil
}

// rulesState reads a .r3v.yaml's contents (nil: no file).
func rulesState(data []byte, root string) *RulesState {
	out := &RulesState{Exists: data != nil, Presets: []profile.PresetEntry{}, Rules: []RuleItem{}, Options: presetOptions(),
		FileLocks: profile.FileLocks{Add: []string{}, Remove: []string{}}}
	if data == nil {
		return out
	}
	if len(data) > maxTextDiff || !isText(data) {
		out.Error = "not a text file"
		return out
	}
	rules, err := profile.Parse(data, root)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Requires, out.Gitignore = rules.Requires, rules.OwnGitignore
	out.FileLocks.Disabled, out.FileLocks.AutoOff = rules.FileLocks.Disabled, rules.FileLocks.AutoOff
	out.FileLocks.Add = append(out.FileLocks.Add, rules.FileLocks.Add...)
	out.FileLocks.Remove = append(out.FileLocks.Remove, rules.FileLocks.Remove...)
	out.Presets = profile.PresetEntries(string(data))
	out.Rules = ruleItems(rules)
	return out
}
