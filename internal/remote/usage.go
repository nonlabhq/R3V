package remote

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/nonlabhq/r3v/internal/manifest"
)

// Usage (docs/design/team-settings.md): how much a team on its own storage
// keeps, by project. A project's size is what its versions use (each
// content once, a big file's pieces included); the team's, what any
// project uses, a content several use counted once; Stored, everything in
// the storage's objects/ (unused files waiting for cleanup too). Sizes are
// as stored (compressed). An R3V Cloud team's usage is the service's.

// UsageReport is a team's usage, counted.
type UsageReport struct {
	Projects map[string]int64 `json:"projects"` // by project id
	Used     int64            `json:"used"`     // by any project
	Stored   int64            `json:"stored"`   // all the storage's objects
	Versions int              `json:"versions"`
}

// ErrUsagePerProject: contents kept per project (an R3V Cloud team): its
// service counts them.
var ErrUsagePerProject = errors.New("an R3V Cloud team's usage comes from R3V Cloud")

// Usage counts what the team keeps, walking every version's folder lists
// once (as storage cleanup does).
func (s *BucketBackend) Usage() (*UsageReport, error) {
	if pp, ok := s.b.(PerProject); ok && pp.ContentsPerProject() {
		return nil, ErrUsagePerProject
	}
	rep := &UsageReport{Projects: map[string]int64{}}
	stored, err := listAll(s.b, gcObjectsDir)
	if err != nil {
		return nil, err
	}
	size := make(map[string]int64, len(stored))
	for _, it := range stored {
		size[strings.ReplaceAll(strings.TrimPrefix(it.Key, gcObjectsDir), "/", "")] = it.Size
		rep.Stored += it.Size
	}
	chunked := map[string]bool{}
	if items, err := listAll(s.b, chunkedDir); err == nil {
		for _, it := range items {
			chunked[strings.TrimPrefix(it.Key, chunkedDir)] = true
		}
	}

	// Folder lists and chunk lists, read once for every project.
	var mu sync.Mutex
	trees := map[string][]manifest.TreeEntry{}
	pieces := map[string][]string{}
	readTree := func(h string) ([]manifest.TreeEntry, error) {
		mu.Lock()
		e, ok := trees[h]
		mu.Unlock()
		if ok {
			return e, nil
		}
		data, err := s.get(s.objectKey(h))
		if err != nil {
			return nil, fmt.Errorf("a version's folder list %s can't be read: %w", h[:10], err)
		}
		e, err = manifest.ParseTree(h, data)
		if err != nil {
			return nil, err
		}
		mu.Lock()
		trees[h] = e
		mu.Unlock()
		return e, nil
	}
	piecesOf := func(h string) []string {
		mu.Lock()
		p, ok := pieces[h]
		mu.Unlock()
		if ok {
			return p
		}
		if l, err := s.chunkList(h); err == nil {
			p = l.Hashes()
		}
		mu.Lock()
		pieces[h] = p
		mu.Unlock()
		return p
	}

	dirs, err := s.b.Folders(gcProjectsDir)
	if err != nil {
		return nil, err
	}
	all := map[string]bool{}
	for _, dir := range dirs {
		pid := strings.TrimSuffix(strings.TrimPrefix(dir, gcProjectsDir), "/")
		used := map[string]bool{}
		keys, err := s.list(dir + gcSnapshotsDir)
		if err != nil {
			return nil, err
		}
		var roots []string
		err = parallelN(checks, keys, func(key string) error {
			data, err := s.get(key)
			if errors.Is(err, ErrNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			id := strings.TrimSuffix(key[strings.LastIndex(key, "/")+1:], ".json")
			m, err := manifest.Parse(id, data)
			if err != nil {
				return nil // (a version this R3V can't read: not counted)
			}
			mu.Lock()
			defer mu.Unlock()
			rep.Versions++
			for _, h := range m.Objects() {
				used[h] = true
			}
			if m.Tree != "" {
				roots = append(roots, m.Tree)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		level := roots
		for len(level) > 0 {
			var todo []string
			for _, h := range level {
				if !used[h] {
					used[h] = true
					todo = append(todo, h)
				}
			}
			var next []string
			err := parallelN(checks, todo, func(h string) error {
				entries, err := readTree(h)
				if err != nil {
					return err
				}
				mu.Lock()
				defer mu.Unlock()
				for _, e := range entries {
					if e.Dir {
						next = append(next, e.Hash)
					} else {
						used[e.Hash] = true
					}
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			level = next
		}
		for h := range used {
			if chunked[h] {
				for _, p := range piecesOf(h) {
					used[p] = true
				}
			}
		}
		var n int64
		for h := range used {
			n += size[h]
			all[h] = true
		}
		rep.Projects[pid] = n
	}
	for h := range all {
		rep.Used += size[h]
	}
	return rep, nil
}
