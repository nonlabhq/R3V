package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// A team's Usage (docs/design/team-settings.md): on a team's own storage,
// R3V's count, made on request and kept on this computer with its time; on
// R3V Cloud, the service's numbers and the plan's.

// ProjectBytes is a project's part. Removed: the project is gone, its
// files not yet.
type ProjectBytes struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Bytes   int64  `json:"bytes"`
	Removed bool   `json:"removed"`
}

// TeamUsageView is a team's Usage tab.
type TeamUsageView struct {
	Hosted bool `json:"hosted"`
	// Counted: when R3V last counted ("" never); Used: what projects use,
	// Stored: all the storage holds (unused files too).
	Counted  string         `json:"counted"`
	Used     int64          `json:"used"`
	Stored   int64          `json:"stored"`
	Versions int            `json:"versions"`
	Projects []ProjectBytes `json:"projects"`
	// R3V Cloud: the plan's quota (0 none) and its name.
	Quota int64  `json:"quota"`
	Plan  string `json:"plan"`
}

type keptUsage struct {
	At     time.Time           `json:"at"`
	Report *remote.UsageReport `json:"report"`
}

func usageFile(teamID string) string { return filepath.Join(teams.Dir(), "usage", teamID+".json") }

// TeamUsage reads a team's usage: the last count (own storage) or the
// service's (R3V Cloud).
func (a *App) TeamUsage(teamID string) (*TeamUsageView, error) {
	_, t, err := findTeam(teamID)
	if err != nil {
		return nil, err
	}
	out := &TeamUsageView{Projects: []ProjectBytes{}}
	names := map[string]string{}
	if b, err := t.Open(); err == nil {
		if ps, err := b.Projects(); err == nil {
			for _, p := range ps {
				names[p.ID] = p.Name
			}
		}
	}
	add := func(id, name string, n int64, removed bool) {
		if names[id] != "" {
			name = names[id]
		}
		if name == "" {
			name, removed = id[:min(8, len(id))], true // (a project removed, its files not cleaned up yet)
		}
		out.Projects = append(out.Projects, ProjectBytes{ID: id, Name: name, Bytes: n, Removed: removed})
	}
	if svc, ok := cloud.Hosted(*t); ok {
		out.Hosted = true
		u, err := cloud.TeamUsage(svc, cloud.TeamID(t.Remote.URL))
		if err != nil {
			return nil, err
		}
		out.Used, out.Stored, out.Quota, out.Plan = u.Used, u.Used, u.Quota, u.Plan
		out.Counted = time.Now().UTC().Format(time.RFC3339)
		for _, p := range u.Projects {
			add(p.ID, p.Name, p.Bytes, p.DeletedAt != "")
		}
	} else {
		var k keptUsage
		if data, err := os.ReadFile(usageFile(teamID)); err == nil && json.Unmarshal(data, &k) == nil && k.Report != nil {
			out.Counted = k.At.UTC().Format(time.RFC3339)
			out.Used, out.Stored, out.Versions = k.Report.Used, k.Report.Stored, k.Report.Versions
			for id, n := range k.Report.Projects {
				add(id, "", n, false)
			}
		}
	}
	sort.Slice(out.Projects, func(i, j int) bool { return out.Projects[i].Bytes > out.Projects[j].Bytes })
	return out, nil
}

// CountTeamUsage counts what a team on its own storage keeps (every
// version's folder lists read once), keeps the count, and gives the view.
func (a *App) CountTeamUsage(teamID string) (*TeamUsageView, error) {
	_, t, err := findTeam(teamID)
	if err != nil {
		return nil, err
	}
	if _, ok := cloud.Hosted(*t); ok {
		return a.TeamUsage(teamID)
	}
	b, err := t.Open()
	if err != nil {
		return nil, err
	}
	s, ok := b.(*remote.BucketBackend)
	if !ok {
		return a.TeamUsage(teamID)
	}
	rep, err := s.Usage()
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(keptUsage{At: time.Now(), Report: rep})
	os.MkdirAll(filepath.Dir(usageFile(teamID)), 0o700)
	if err := os.WriteFile(usageFile(teamID), data, 0o600); err != nil {
		return nil, err
	}
	return a.TeamUsage(teamID)
}
