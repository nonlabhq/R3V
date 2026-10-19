package desktop

import (
	"strings"
	"sync"
	"time"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/teams"
)

// A hosted team's keys changing (live notices): what shows them is read
// again at once instead of at the next poll. Only read: a teammate's
// version is still told of and asked about as ever, never applied.

// TeamKeysEvent ("team-keys") tells the frontend which of a team's keys
// changed and what to read again.
type TeamKeysEvent struct {
	Team string   `json:"team"` // the team's id
	Keys []string `json:"keys"` // "" for anything (after a reconnect)
	// Overview: the team's look, projects or archived ones (Overview).
	Overview bool `json:"overview"`
	// Activity: the team's Activity (its log, a project's branch moves).
	Activity bool `json:"activity"`
}

// liveKeysWait gathers the notices of a moment into one read: a share
// moves a branch, writes its log and a workspace, all at once.
var liveKeysWait = 300 * time.Millisecond

type liveKeys struct {
	sync.Mutex
	pending map[string]*keysBatch // by team id
}

type keysBatch struct {
	ev    TeamKeysEvent
	roots map[string]bool // projects here whose page reads again
}

// onKey follows a key a hosted team's service says changed.
func (a *App) onKey(service, team string, k cloud.KeyNotice) {
	store, err := teams.Load()
	if err != nil {
		return
	}
	t := store.FindByURL(cloud.TeamAddress(service, team))
	if t == nil {
		return
	}
	overview, activity := false, false
	roots := map[string]bool{}
	rest, isProject := strings.CutPrefix(k.Key, "projects/"+k.Project+"/")
	switch {
	case k.Key == "":
		overview, activity = true, true
		for key, root := range store.Projects {
			if strings.HasPrefix(key, t.ID+"/") {
				roots[root] = true
			}
		}
		looksCache.forget(t.Remote.URL)
	case k.Project != "" && isProject:
		dir, _, _ := strings.Cut(rest, "/")
		switch dir {
		case "branches", "workspaces":
			if root, ok := store.Projects[t.ID+"/"+k.Project]; ok {
				roots[root] = true
			}
		}
		activity = dir == "branches" || dir == "branchlog"
	case strings.HasPrefix(k.Key, "teamlog/"):
		activity = true
	case k.Key == "team.json", k.Key == "archived.json", k.Key == "sections.json",
		strings.HasPrefix(k.Key, "pictures/team/"), strings.HasPrefix(k.Key, "pictures/projects/"):
		overview = true
	case strings.HasPrefix(k.Key, "members/"), strings.HasPrefix(k.Key, "pictures/members/"):
		looksCache.forget(t.Remote.URL)
	}
	if !overview && !activity && len(roots) == 0 {
		return
	}
	a.liveKeys.Lock()
	defer a.liveKeys.Unlock()
	if a.liveKeys.pending == nil {
		a.liveKeys.pending = map[string]*keysBatch{}
	}
	b := a.liveKeys.pending[t.ID]
	if b == nil {
		b = &keysBatch{ev: TeamKeysEvent{Team: t.ID, Keys: []string{}}, roots: map[string]bool{}}
		a.liveKeys.pending[t.ID] = b
		time.AfterFunc(liveKeysWait, func() { a.flushKeys(t.ID) })
	}
	b.ev.Keys = append(b.ev.Keys, k.Key)
	b.ev.Overview = b.ev.Overview || overview
	b.ev.Activity = b.ev.Activity || activity
	for r := range roots {
		b.roots[r] = true
	}
}

func (a *App) flushKeys(teamID string) {
	a.liveKeys.Lock()
	b := a.liveKeys.pending[teamID]
	delete(a.liveKeys.pending, teamID)
	a.liveKeys.Unlock()
	if b == nil || a.emit == nil {
		return
	}
	if b.ev.Overview || b.ev.Activity {
		a.emit("team-keys", b.ev)
	}
	for root := range b.roots {
		a.emit("team-watch", WatchEvent{Root: root, Kind: "record", Labels: []string{}, Versions: []Version{}})
	}
}

// watchHostedTeams listens to every hosted team this computer is in, a
// project of it here or not (the team's own keys: its home and lists).
func (a *App) watchHostedTeams() {
	store, err := teams.Load()
	if err != nil {
		return
	}
	want := map[string]bool{}
	for _, t := range store.Teams {
		if _, ok := cloud.Hosted(t); ok && !t.NoAccess {
			want[t.Remote.URL] = true
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.teamLive == nil {
		a.teamLive = map[string]func(){}
	}
	for url, stop := range a.teamLive {
		if !want[url] {
			stop()
			delete(a.teamLive, url)
		}
	}
	for url := range want {
		if _, ok := a.teamLive[url]; !ok {
			a.teamLive[url] = a.live.WatchTeam(url)
		}
	}
}
