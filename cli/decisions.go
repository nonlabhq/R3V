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
}

// decisionFlags adds --resolve, --decisions and --resolved next to a
// command's --strategy.
func decisionFlags(fs *flag.FlagSet, strategy *string) *decisions {
	d := &decisions{strategy: strategy}
	fs.Var(&d.resolve, "resolve", "decide one conflict by its key: <key>=ours|theirs|both (again for more)")
	d.file = fs.String("decisions", "", `decisions in a JSON file, {"<key>": "ours|theirs|both"} ("-": from stdin)`)
	fs.Var(&d.resolved, "resolved", "give a file as it should be: <path>=<file with its content> (again for more)")
	return d
}

// options reads the decisions; a key, choice or path that can't be is a
// usage error.
func (d *decisions) options(r *project.Repo) (project.MergeOptions, error) {
	opts := project.MergeOptions{Strategy: *d.strategy}
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
			if err := decide(&opts, k, c); err != nil {
				return opts, err
			}
		}
	}
	for _, v := range d.resolve {
		i := strings.LastIndex(v, "=")
		if i < 0 {
			return opts, usageError("--resolve %q: want <key>=ours|theirs|both", v)
		}
		if err := decide(&opts, v[:i], v[i+1:]); err != nil {
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

func decide(opts *project.MergeOptions, key, choice string) error {
	if !validKey(key) {
		return usageError("%q is not a conflict key (file:<path>, text:<path>#<n>, <set>#<key>: see the conflicts' key)", key)
	}
	switch choice {
	case "ours", "theirs", "both":
	default:
		return usageError("%s: %q is not a choice: ours, theirs or both", key, choice)
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
