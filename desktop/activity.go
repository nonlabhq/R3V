package desktop

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/remote"
)

// A team's Activity (docs/design/team-settings.md): what happened, newest
// first, from what its storage keeps: the projects' branch logs (shares,
// merges, branches made and deleted) and the team log (projects added and
// removed, members joining).

// ActivityItem is one thing that happened.
type ActivityItem struct {
	Time string `json:"time"`
	// Kind: share, merge, branch-made, branch-deleted, project-added,
	// project-removed, member-joined.
	Kind        string `json:"kind"`
	Project     string `json:"project"`     // its id ("" for a member joining)
	ProjectName string `json:"projectName"` // as it is now, else as it was
	Branch      string `json:"branch"`      // the branch's name
	Version     string `json:"version"`     // the version the branch went to
	Title       string `json:"title"`       // its title
	By          string `json:"by"`          // the member's id
	ByName      string `json:"byName"`      // their name ("" unknown)
	// Tool and Model: the AI agent that made the version, for its member.
	Tool  string `json:"tool"`
	Model string `json:"model"`
}

// activityMax bounds what one read gives (the newest).
const activityMax = 500

// TeamActivity lists what happened in a team in the last days, newest first.
func (a *App) TeamActivity(teamID string, days int) ([]ActivityItem, error) {
	_, t, err := findTeam(teamID)
	if err != nil {
		return nil, err
	}
	b, err := t.Open()
	if err != nil {
		return nil, err
	}
	s, ok := b.(*remote.BucketBackend)
	if !ok {
		return []ActivityItem{}, nil
	}
	if days <= 0 {
		days = 30
	}
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	ps, err := b.Projects()
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	if ms, err := b.Members(); err == nil {
		for _, m := range ms {
			names[m.ID] = m.Name
		}
	}
	projectName := map[string]string{}
	for _, p := range ps {
		projectName[p.ID] = p.Name
	}
	records, _ := remote.BranchRecordsOf(b)

	var mu sync.Mutex
	out := []ActivityItem{}
	add := func(it ActivityItem) {
		if it.ByName == "" {
			it.ByName = names[it.By]
		}
		mu.Lock()
		out = append(out, it)
		mu.Unlock()
	}

	// The team log.
	events, err := s.TeamLog(since)
	if err != nil && !errors.Is(err, remote.ErrForbidden) {
		return nil, err
	}
	for _, ev := range events {
		it := ActivityItem{Time: ev.Time.UTC().Format(time.RFC3339Nano), Kind: ev.Kind, Project: ev.Project, By: ev.By}
		switch ev.Kind {
		case remote.EventMemberJoined:
			it.By, it.ByName = ev.Member, ev.Name
		default:
			it.ProjectName = projectName[ev.Project]
			if it.ProjectName == "" {
				it.ProjectName = ev.Name
			}
		}
		add(it)
	}

	// Each project's branch log, a few projects at a time.
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, pid := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(pid string) {
			defer func() { <-sem; wg.Done() }()
			moves, err := s.BranchLogSince(pid, since)
			if err != nil || len(moves) == 0 {
				return
			}
			branchName := map[string]string{}
			if records != nil {
				if rs, err := records.BranchRecords(pid); err == nil {
					for k, r := range rs {
						branchName[k] = r.Name
					}
				}
			}
			for _, m := range moves {
				it := ActivityItem{Time: m.Time.UTC().Format(time.RFC3339Nano), Project: pid, ProjectName: projectName[pid],
					Branch: m.Branch, Version: m.To, By: m.By}
				if n := branchName[m.Branch]; n != "" {
					it.Branch = n
				}
				switch {
				case m.To == "":
					it.Kind = "branch-deleted"
				case m.From == "":
					it.Kind = "branch-made"
				default:
					it.Kind = "share"
				}
				if m.To != "" {
					if data, err := b.GetSnapshot(pid, m.To); err == nil {
						if v, err := manifest.Parse(m.To, data); err == nil {
							it.Title = manifest.Title(v.Message)
							if len(v.Parents) > 1 && it.Kind == "share" {
								it.Kind = "merge"
							}
							if v.Actor != nil {
								it.Tool, it.Model = v.Actor.Tool, v.Actor.Model
							}
						}
					}
				}
				add(it)
			}
		}(pid)
	}
	wg.Wait()

	at := func(s string) time.Time { t, _ := time.Parse(time.RFC3339Nano, s); return t }
	sort.SliceStable(out, func(i, j int) bool { return at(out[i].Time).After(at(out[j].Time)) })
	if len(out) > activityMax {
		out = out[:activityMax]
	}
	return out, nil
}
