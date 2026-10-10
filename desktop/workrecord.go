package desktop

import (
	"sync"

	"github.com/nonlabhq/r3v/internal/project"
)

// The work this copy has, told to the team in the background (its
// workspace record: see project.NoteWork), once at a time per project.
var noting = struct {
	sync.Mutex
	busy map[string]bool
}{busy: map[string]bool{}}

func (a *App) noteWorkLater(root string, changes int) {
	noting.Lock()
	if noting.busy[root] {
		noting.Unlock()
		return
	}
	noting.busy[root] = true
	noting.Unlock()
	go func() {
		defer func() {
			noting.Lock()
			delete(noting.busy, root)
			noting.Unlock()
		}()
		unlock := a.lock(root)
		defer unlock()
		if r, err := project.Open(root); err == nil {
			r.NoteWork(changes)
		}
	}()
}

// Workspace is another copy of a project, as it last told the team.
type Workspace struct {
	Member   string   `json:"member"` // their name (their id when the team has none)
	MemberID string   `json:"memberId"`
	Branch   string   `json:"branch"`
	Changes  int      `json:"changes"`  // files changed, not committed
	Unshared int      `json:"unshared"` // versions not shared
	Parked   []string `json:"parked"`   // branches with changes parked
	Time     string   `json:"time"`     // RFC 3339
}

// Workspaces lists the other copies of a project (who is working where,
// with what), as they last told the team.
func (a *App) Workspaces(root string) ([]Workspace, error) {
	r, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	ws, err := r.Workspaces()
	if err != nil {
		return nil, err
	}
	names := a.memberNames(r)
	out := []Workspace{}
	for _, w := range ws {
		name := names[w.Member]
		if name == "" {
			name = w.Member
		}
		out = append(out, Workspace{Member: name, MemberID: w.Member, Branch: w.Branch, Changes: w.Changes,
			Unshared: w.Unshared, Parked: nonNil(w.Parked), Time: w.Time.Format("2006-01-02T15:04:05Z07:00")})
	}
	return out, nil
}
