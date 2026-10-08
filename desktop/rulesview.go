package desktop

import (
	"github.com/nonlabhq/r3v/internal/profile"
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
	return rulesState(data, r.Root), nil
}

// rulesState reads a .r3v.yaml's contents (nil: no file).
func rulesState(data []byte, root string) *RulesState {
	out := &RulesState{Exists: data != nil, Presets: []profile.PresetEntry{}, Rules: []RuleItem{}, Options: presetOptions()}
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
	out.Presets = profile.PresetEntries(string(data))
	out.Rules = ruleItems(rules)
	return out
}
