package project

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nonlabhq/r3v/internal/blob"
	"github.com/nonlabhq/r3v/internal/chunk"
	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Planning a team's move to R3V Cloud (docs/design/moving.md): what of a
// team's own storage each project needs, as keys for the service to copy
// (it copies; it doesn't read R3V's formats). The walk is MoveToTeam's,
// on a team this computer may not have the project of: a scratch project
// keeps the versions and folder lists it reads (small), then goes.

// MoveItem is a key to copy: Src where the team's storage keeps it
// (relative to its folder), Dst where the hosted team keeps it (relative to
// the project), Size its bytes as stored.
type MoveItem struct {
	Src, Dst string
	Size     int64
}

// MovePlan is what a project needs moved, in an order safe to copy in: a
// big file's pieces before its mark and its list, contents before
// versions.
type MovePlan struct {
	Project  remote.Project
	Versions int
	Items    []MoveItem
	Bytes    int64 // what the hosted team will keep for it
}

// TeamSizes is what a planner needs to know of a team's storage: the size
// of every file's contents (one listing for all projects) and which files
// are kept as pieces.
type TeamSizes struct {
	Objects map[string]int64 // by key (objects/ab/…)
	Chunked map[string]bool  // by file hash
	Total   int64            // the whole pool, as the team's storage holds it
}

// SizesOf lists a team's storage once, for planning all its projects.
func SizesOf(t *teams.Team) (*TeamSizes, error) {
	b, err := t.Open()
	if err != nil {
		return nil, err
	}
	s, ok := b.(*remote.BucketBackend)
	if !ok {
		return nil, errors.New("this team's storage can't be listed")
	}
	if remote.ThroughService(b) {
		return nil, errors.New("this team is on R3V Cloud already")
	}
	objs, err := s.KeySizes("objects/")
	if err != nil {
		return nil, err
	}
	marks, err := s.KeySizes("chunked/")
	if err != nil {
		return nil, err
	}
	out := &TeamSizes{Objects: objs, Chunked: map[string]bool{}}
	for _, n := range objs {
		out.Total += n
	}
	for k := range marks {
		out.Chunked[strings.TrimPrefix(k, "chunked/")] = true
	}
	return out, nil
}

// PlanMove plans project p of team t (sizes from SizesOf).
func PlanMove(t *teams.Team, p remote.Project, sizes *TeamSizes) (*MovePlan, error) {
	dir, err := os.MkdirTemp("", "r3v-plan-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	// (a scratch folder R3V takes for a project: no preset, no files)
	if err := os.WriteFile(filepath.Join(dir, ".r3v.yaml"), []byte("presets:\n  ./: none\n"), 0o644); err != nil {
		return nil, err
	}
	r, err := Init(dir, "")
	if err != nil {
		return nil, err
	}
	r.Config.ProjectID, r.Config.Name = p.ID, p.Name
	r.Config.Remote = &RemoteConfig{URL: t.Remote.URL}
	c, err := r.Client()
	if err != nil {
		return nil, err
	}
	heads, err := c.Branches(p.ID)
	if err != nil && !errors.Is(err, remote.ErrNotFound) {
		return nil, err
	}
	tips := map[string]bool{}
	for _, h := range heads {
		tips[h] = true
	}
	if gone, err := r.ArchivedBranches(); err == nil {
		for _, g := range gone {
			tips[g.Head] = true
		}
	}
	if marks, err := r.Milestones(); err == nil {
		for _, m := range marks {
			tips[m.Version] = true
		}
	}
	versions := map[string]bool{}
	for h := range tips {
		if h == "" {
			continue
		}
		if err := r.fetchSnapshots(c, h); err != nil {
			return nil, err
		}
		anc, err := r.ancestors(h)
		if err != nil {
			return nil, err
		}
		for v := range anc {
			versions[v] = true
		}
	}
	var ids, files, roots []string
	for v := range versions {
		ids = append(ids, v)
		m, err := r.Load(v)
		if err != nil {
			return nil, err
		}
		files = append(files, m.Objects()...)
		if m.Tree != "" {
			roots = append(roots, m.Tree)
		}
	}
	if err := r.walkTrees(roots, func(h string, _ []manifest.TreeEntry) error {
		files = append(files, h)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(ids)
	files = dedupe(files)
	sort.Strings(files)

	plan := &MovePlan{Project: p, Versions: len(ids)}
	add := func(src, dst string, size int64) {
		plan.Items = append(plan.Items, MoveItem{src, dst, size})
		plan.Bytes += size
	}
	object := func(h string) error {
		k := remote.ObjectKey(h)
		n, ok := sizes.Objects[k]
		if !ok {
			return fmt.Errorf("%s isn't in the team's storage", short(h))
		}
		add(k, k, n)
		return nil
	}
	seen := map[string]bool{}
	for _, h := range files {
		if seen[h] {
			continue
		}
		seen[h] = true
		if sizes.Chunked[h] {
			// The pieces, then the mark, then the list.
			pieces, err := piecesOf(c, h)
			if err != nil {
				return nil, err
			}
			for _, ph := range pieces {
				if !seen[ph] {
					seen[ph] = true
					if err := object(ph); err != nil {
						return nil, err
					}
				}
			}
			add(remote.ChunkedKey(h), remote.ChunkedKey(h), 0)
		}
		if err := object(h); err != nil {
			return nil, err
		}
	}
	for _, id := range ids {
		data, err := os.ReadFile(r.snapshotPath(id))
		if err != nil {
			return nil, err
		}
		add(remote.SnapshotKey(p.ID, id), "snapshots/"+id+".json", int64(len(data)))
	}
	return plan, nil
}

// piecesOf reads big file h's chunk list from c: its pieces.
func piecesOf(c remote.Backend, h string) ([]string, error) {
	rc, err := c.GetObject(h)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", short(h), err)
	}
	defer rc.Close()
	lr, list, err := blob.Open(rc)
	if err != nil {
		return nil, err
	}
	defer lr.Close()
	if !list {
		return nil, nil // (marked, but kept whole: nothing more to copy)
	}
	text, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	l, err := chunk.Parse(text)
	if err != nil {
		return nil, err
	}
	return dedupe(l.Hashes()), nil
}

// MoveEstimate is what moving a team to R3V Cloud means: per project, what
// the hosted team will keep (each project's contents apart, so a file two
// projects use counts twice), against what the team's storage holds now.
type MoveEstimate struct {
	Plans   []*MovePlan
	Hosted  int64 // all the plans' bytes
	Storage int64 // the team's storage now (contents)
	People  int
}

// EstimateMove plans every project of team t.
func EstimateMove(t *teams.Team) (*MoveEstimate, error) {
	b, err := t.Open()
	if err != nil {
		return nil, err
	}
	sizes, err := SizesOf(t)
	if err != nil {
		return nil, err
	}
	ps, err := b.Projects()
	if err != nil {
		return nil, err
	}
	ms, err := b.Members()
	if err != nil {
		return nil, err
	}
	out := &MoveEstimate{Storage: sizes.Total, People: len(ms)}
	sort.Slice(ps, func(i, j int) bool { return ps[i].Name < ps[j].Name })
	for _, p := range ps {
		plan, err := PlanMove(t, p, sizes)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Name, err)
		}
		out.Plans = append(out.Plans, plan)
		out.Hosted += plan.Bytes
	}
	return out, nil
}

// ErrNotCopied: the service hasn't copied what a project's branches need.
var ErrNotCopied = errors.New("not all of the project is on R3V Cloud yet")

// FinishMove is a team move's last step for project p (docs/design/
// moving.md), once the service copied its plan: its branch names and
// colours and milestones, then its branches, then its record, on hosted
// team to. Safe to do again.
func FinishMove(from, to *teams.Team, p remote.Project) error {
	fb, err := from.Open()
	if err != nil {
		return err
	}
	tb, err := to.Open()
	if err != nil {
		return err
	}
	c, d := remote.ForProject(fb, p.ID), remote.ForProject(tb, p.ID)
	heads, err := c.Branches(p.ID)
	if err != nil && !errors.Is(err, remote.ErrNotFound) {
		return err
	}
	var tips []string
	for _, h := range heads {
		tips = append(tips, h)
	}
	// Listed first: R3V Cloud answers about a project's contents only once
	// it is (with no branches yet, it shows no versions).
	if err := d.PutProject(p); err != nil {
		return err
	}
	if missing, err := d.MissingSnapshots(p.ID, dedupe(tips)); err != nil {
		return err
	} else if len(missing) > 0 {
		return ErrNotCopied
	}
	if err := copyRecords(c, d, p.ID); err != nil {
		return err
	}
	return copyBranches(d, p.ID, heads, nil)
}
