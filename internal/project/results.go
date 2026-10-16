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
	// From: the version it was recorded for, when that is another one with
	// the same files (a merge that brought in exactly a lane's head):
	// carried over when read, never stored.
	From string `json:"from,omitempty"`
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
// A version without results of its own that has the same files as one
// with results (see carriedFrom) shows those, From set.
func (r *Repo) Results(version string) map[string]Result {
	out := r.ownResults(version)
	if len(out) > 0 {
		return out
	}
	if from := r.carriedFrom(version); from != "" {
		out = r.ownResults(from)
		for n, res := range out {
			res.From = from
			out[n] = res
		}
	}
	return out
}

// ownResults are the results recorded for the version itself.
func (r *Repo) ownResults(version string) map[string]Result {
	out := map[string]Result{}
	if data, err := os.ReadFile(r.resultsPath(version)); err == nil {
		json.Unmarshal(data, &out)
	}
	return out
}

func (r *Repo) hasOwnResults(version string) bool {
	_, err := os.Stat(r.resultsPath(version))
	return err == nil
}

// carryLevels bounds how far back results are looked for.
const carryLevels = 4

// carriedFrom is the version whose results a version without any of its
// own shows: merging a lane whose head the branch hadn't moved past makes
// a merge version (the branch's line stays its own) with exactly the
// lane's files, and what the checks said about those files holds for it.
// Only merges, only parents with the same files, a few levels at most;
// "" when none.
func (r *Repo) carriedFrom(id string) string {
	for range carryLevels {
		h, err := r.Header(id)
		if err != nil || len(h.Parents) < 2 {
			return ""
		}
		next := ""
		for _, p := range h.Parents {
			if r.sameFiles(id, p) {
				next = p
				break
			}
		}
		if next == "" {
			return ""
		}
		if r.hasOwnResults(next) {
			return next
		}
		id = next
	}
	return ""
}

// sameFiles: versions a and b have the same files (the same top tree, or
// for versions older than trees the same file list) and the same samples
// from outside the project.
func (r *Repo) sameFiles(a, b string) bool {
	ha, err := r.Header(a)
	if err != nil {
		return false
	}
	hb, err := r.Header(b)
	if err != nil {
		return false
	}
	if ha.Tree != "" && hb.Tree != "" {
		if ha.Tree != hb.Tree {
			return false
		}
		ra, err := r.readRecord(a)
		if err != nil {
			return false
		}
		rb, err := r.readRecord(b)
		return err == nil && sameEntries(ra.External, rb.External)
	}
	ma, err := r.Load(a)
	if err != nil {
		return false
	}
	mb, err := r.Load(b)
	return err == nil && sameEntries(ma.Files, mb.Files) && sameEntries(ma.External, mb.External)
}

// sameEntries: the same paths with the same contents, in any order.
func sameEntries(a, b []FileEntry) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]string, len(a))
	for _, e := range a {
		m[e.Path] = e.Hash
	}
	for _, e := range b {
		if h, ok := m[e.Path]; !ok || h != e.Hash {
			return false
		}
	}
	return true
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
	all := r.ownResults(id)
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
// results. One read of the folder, not one per version. Versions among
// shown without results of their own get the ones carried over to them
// (Results).
func (r *Repo) Checked(shown ...string) map[string]string {
	out := map[string]string{}
	entries, err := os.ReadDir(filepath.Join(r.CommonDir(), resultsDir))
	if err != nil {
		return out
	}
	own := map[string]bool{}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || e.IsDir() {
			continue
		}
		own[id] = true
		if state := checkState(r.ownResults(id)); state != "" {
			out[id] = state
		}
	}
	if len(own) == 0 {
		return out
	}
	for _, id := range shown {
		if own[id] {
			continue
		}
		if from := r.carriedFrom(id); out[from] != "" {
			out[id] = out[from]
		}
	}
	return out
}

// checkState sums up a version's results: "pass", "fail", or "" (none, or
// one still running and none failed).
func checkState(results map[string]Result) string {
	state := ""
	for _, res := range results {
		switch {
		case res.State == "fail":
			state = "fail"
		case res.State == "pass" && state == "":
			state = "pass"
		case res.State != "pass" && state != "fail":
			state = "running"
		}
	}
	if state == "running" {
		return ""
	}
	return state
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
