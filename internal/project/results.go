package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/store"
)

// Results: what a build, the tests or another check said about a version,
// recorded by whoever ran it (an agent, a person; a CI later), as a
// commit's status is in Git hosts. On this computer (every copy of the
// project: the store's folder), beside the version, never in it: the
// version's id stays, and a result can be recorded again.

const resultsDir = "results"

// Result is one check's latest result for a version.
type Result struct {
	State string `json:"state"` // pass, fail, running
	Note  string `json:"note,omitempty"`
	Time  string `json:"time"`
	Actor string `json:"actor,omitempty"`
	// Changes: files that were changed, not committed, when it was recorded
	// (the result is about those files, not exactly the version).
	Changes int `json:"changes,omitempty"`
}

// ResultStates are the states a result can be in.
var ResultStates = []string{"pass", "fail", "running"}

var resultName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,31}$`)

// ErrBadResult: a check's name or state R3V doesn't take.
var ErrBadResult = errors.New("a result is a check's name (letters, digits, - . _) and pass, fail or running")

func (r *Repo) resultsPath(version string) string {
	return filepath.Join(r.CommonDir(), resultsDir, version+".json")
}

// Results are the checks' latest results for a version (empty when none).
func (r *Repo) Results(version string) map[string]Result {
	out := map[string]Result{}
	if data, err := os.ReadFile(r.resultsPath(version)); err == nil {
		json.Unmarshal(data, &out)
	}
	return out
}

// SetResult records a check's result for a version ("" the one the project
// is on), with the files changed meanwhile counted.
func (r *Repo) SetResult(version, name, state, note string) (string, *Result, error) {
	ok := resultName.MatchString(name)
	known := false
	for _, s := range ResultStates {
		known = known || s == state
	}
	if !ok || !known {
		return "", nil, ErrBadResult
	}
	if version == "" {
		version = r.Head()
	}
	id, err := r.Resolve(version)
	if err != nil {
		return "", nil, err
	}
	res := Result{State: state, Note: note, Time: time.Now().UTC().Format(time.RFC3339)}
	if r.Actor != nil {
		res.Actor = r.Actor.Tool
	}
	if id == r.Head() {
		if changes, err := r.Status(); err == nil {
			res.Changes = len(changes)
		}
	}
	all := r.Results(id)
	all[name] = res
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(filepath.Dir(r.resultsPath(id)), 0o755); err != nil {
		return "", nil, err
	}
	if err := store.WriteAtomic(r.resultsPath(id), bytes.NewReader(append(data, '\n'))); err != nil {
		return "", nil, err
	}
	return id, &res, nil
}

// Checked sums up each version's results, for the history graph: "fail"
// when a check failed, "pass" when every one passed; a version with a
// check still running and none failed is left out, as is one without
// results. One read of the folder, not one per version.
func (r *Repo) Checked() map[string]string {
	out := map[string]string{}
	entries, err := os.ReadDir(filepath.Join(r.CommonDir(), resultsDir))
	if err != nil {
		return out
	}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || e.IsDir() {
			continue
		}
		state := ""
		for _, res := range r.Results(id) {
			switch {
			case res.State == "fail":
				state = "fail"
			case res.State == "pass" && state == "":
				state = "pass"
			case res.State != "pass" && state != "fail":
				state = "running"
			}
		}
		if state == "pass" || state == "fail" {
			out[id] = state
		}
	}
	return out
}

// ResultNames are a results map's checks, in order.
func ResultNames(m map[string]Result) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// pruneResults forgets the results of versions no longer stored here.
func (r *Repo) pruneResults() {
	entries, err := os.ReadDir(filepath.Join(r.CommonDir(), resultsDir))
	if err != nil {
		return
	}
	for _, e := range entries {
		id := e.Name()
		if len(id) > 5 && filepath.Ext(id) == ".json" && !r.HasSnapshot(id[:len(id)-5]) {
			os.Remove(filepath.Join(r.CommonDir(), resultsDir, id))
		}
	}
}
