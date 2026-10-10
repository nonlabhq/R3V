package desktop

import (
	"errors"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/agentlock"
	"github.com/nonlabhq/r3v/internal/project"
)

// Agent locks (docs/design/agent-workspaces.md, "Locks"; Nightly while
// project.AgentLocks is): the workspaces of a project on this computer
// take turns on files that can't be merged. The app shows them with the
// team's locks (one padlock: you, a member, an agent's workspace) and
// breaks any: people always win.

// agentLockItems is the agent locks of the project at root. The files you
// changed and haven't committed count as yours only while agents are at
// work (some lock or claim is in the list): without them, a padlock on
// every changed file would say nothing.
func agentLockItems(root string) []LockItem {
	if !project.AgentLocks {
		return nil
	}
	r, err := project.Open(root)
	if err != nil {
		return nil
	}
	stored, err := r.AgentLockList().All()
	if err != nil || len(stored) == 0 {
		return nil
	}
	locks, err := r.AllAgentLocks()
	if err != nil {
		locks = stored
	}
	out := []LockItem{}
	for _, l := range locks {
		if l.Kind != agentlock.KindLock {
			continue // (claims warn agents, they hold nothing)
		}
		out = append(out, agentLockItem(r, l))
	}
	return out
}

func agentLockItem(r *project.Repo, l agentlock.Lock) LockItem {
	it := LockItem{Path: l.Path, Prefix: strings.HasSuffix(l.Path, "/"), Agent: true,
		AgentHolder: agentHolder(l)}
	if !l.Since.IsZero() {
		it.Since = l.Since.UTC().Format(time.RFC3339)
	}
	// Yours: your changes, or a lock you took yourself (no agent) in this copy.
	it.Mine = l.Person || (l.Actor == "" && l.Workspace != "" && l.Workspace == r.Config.WorkspaceID)
	return it
}

func agentHolder(l agentlock.Lock) AgentHolder {
	return AgentHolder{Tool: l.Actor, Branch: l.Branch, Person: l.Person, Stale: l.Stale}
}

// AgentLocks reads a project's agent locks alone: agents take and free
// them without telling the app, so the page asks again now and then, which
// mustn't ask the team for its locks each time (ProjectLocks does).
func (a *App) AgentLocks(root string) []LockItem {
	if !knownProject(root) {
		return []LockItem{}
	}
	return nonNil(agentLockItems(root))
}

// BreakAgentLock takes back whoever's agent lock is on path (a folder's
// ending in "/"): the person's call, any time.
func (a *App) BreakAgentLock(root, path string) error {
	if !project.AgentLocks {
		return errors.New("agent locks are not in this build")
	}
	r, err := project.Open(root)
	if err != nil {
		return err
	}
	if _, err := r.BreakAgentLock(path); err != nil {
		return err
	}
	a.emitLocks(root)
	return nil
}

// agentLockedResult is a commit refused because another workspace on this
// computer holds files it changes: nothing was committed.
func agentLockedResult(r *project.Repo, err error) (*Result, bool) {
	var l *project.ErrLockedByAgent
	if !errors.As(err, &l) {
		return nil, false
	}
	out := syncResult(nil)
	out.Action = "agent-locked"
	out.Locks = []HeldLock{}
	for _, h := range l.Locks {
		out.Locks = append(out.Locks, HeldLock{Path: h.Path, AgentHolder: agentHolder(h)})
	}
	return out, true
}
