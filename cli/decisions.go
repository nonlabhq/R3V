package cli

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nonlabhq/r3v/internal/project"
)

// decisions are what a command that merges is told for the conflicts:
// --strategy for what's left, --resolve and --decisions by conflict key,
// --resolved for a file given as it should be.
type decisions struct {
	strategy *string
	resolve  stringList
	file     *string
	resolved stringList
	// choices: the words the command names the choices with; aliases: its
	// own words for ours and theirs (undo-merge's keep and undo).
	choices string
	aliases map[string]string
}

// decisionFlags adds --resolve, --decisions and --resolved next to a
// command's --strategy.
func decisionFlags(fs *flag.FlagSet, strategy *string) *decisions {
	return decisionFlagsAs(fs, strategy, "ours|theirs|both", nil)
}

// decisionFlagsAs is decisionFlags for a command with words of its own for
// the choices (aliases: each word, and the choice it is).
func decisionFlagsAs(fs *flag.FlagSet, strategy *string, choices string, aliases map[string]string) *decisions {
	d := &decisions{strategy: strategy, choices: choices, aliases: aliases}
	fs.Var(&d.resolve, "resolve", "decide one conflict by its key: <key>="+choices+" (again for more)")
	d.file = fs.String("decisions", "", `decisions in a JSON file, {"<key>": "`+choices+`"} ("-": from stdin)`)
	fs.Var(&d.resolved, "resolved", "give a file as it should be: <path>=<file with its content> (again for more)")
	return d
}

// choice is the merge's word for a choice given in the command's own.
func (d *decisions) choice(c string) string {
	if a, ok := d.aliases[c]; ok {
		return a
	}
	return c
}

// options reads the decisions; a key, choice or path that can't be is a
// usage error.
func (d *decisions) options(r *project.Repo) (project.MergeOptions, error) {
	opts := project.MergeOptions{Strategy: d.choice(*d.strategy)}
	if *d.file != "" {
		var data []byte
		var err error
		if *d.file == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(*d.file)
		}
		if err != nil {
			return opts, usageError("--decisions: %v", err)
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			return opts, usageError(`--decisions: not {"<key>": "<choice>"}: %v`, err)
		}
		for k, c := range m {
			if err := d.decide(&opts, k, c); err != nil {
				return opts, err
			}
		}
	}
	for _, v := range d.resolve {
		i := strings.LastIndex(v, "=")
		if i < 0 {
			return opts, usageError("--resolve %q: want <key>=%s", v, d.choices)
		}
		if err := d.decide(&opts, v[:i], v[i+1:]); err != nil {
			return opts, err
		}
	}
	for _, v := range d.resolved {
		p, file, ok := strings.Cut(v, "=")
		if !ok || p == "" || file == "" {
			return opts, usageError("--resolved %q: want <path>=<file>", v)
		}
		rel, err := projectPath(r, p)
		if err != nil {
			return opts, err
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return opts, usageError("--resolved %s: %v", p, err)
		}
		if opts.Resolved == nil {
			opts.Resolved = map[string][]byte{}
		}
		opts.Resolved[rel] = data
	}
	return opts, nil
}

func (d *decisions) decide(opts *project.MergeOptions, key, given string) error {
	if !validKey(key) {
		return usageError("%q is not a conflict key (file:<path>, text:<path>#<n>, <set>#<key>: see the conflicts' key)", key)
	}
	choice := d.choice(given)
	switch choice {
	case "ours", "theirs", "both":
	default:
		words := strings.Split(d.choices, "|")
		return usageError("%s: %q is not a choice: %s or %s", key, given,
			strings.Join(words[:len(words)-1], ", "), words[len(words)-1])
	}
	if opts.Resolutions == nil {
		opts.Resolutions = map[string]string{}
	}
	opts.Resolutions[key] = choice
	return nil
}

// validKey: a whole file's key, a text file's hunk's, or a set's part's.
func validKey(k string) bool {
	if p, ok := strings.CutPrefix(k, "file:"); ok {
		return p != ""
	}
	if p, ok := strings.CutPrefix(k, "text:"); ok {
		i := strings.LastIndex(p, "#")
		n, err := strconv.Atoi(p[i+1:])
		return i > 0 && err == nil && n > 0
	}
	file, part, ok := strings.Cut(k, "#")
	return ok && file != "" && part != ""
}

// projectPath is a file of the project as conflicts name it (relative to
// its folder, slash separated), from that or a path on disk.
func projectPath(r *project.Repo, p string) (string, error) {
	if filepath.IsAbs(p) {
		rel, err := filepath.Rel(r.Root, p)
		if err != nil {
			return "", usageError("--resolved %s: not in the project", p)
		}
		p = rel
	}
	p = path.Clean(filepath.ToSlash(p))
	if p == "." || p == ".." || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") {
		return "", usageError("--resolved %s: not in the project", p)
	}
	return p, nil
}
