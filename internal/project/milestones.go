package project

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nonlabhq/r3v/internal/remote"
)

// Milestones (remote.Milestone): versions given a name for the whole team.

// ErrNoMilestones: the team keeps no milestones (Stable, or a service that
// doesn't yet).
var ErrNoMilestones = errors.New("this team can't keep milestones yet")

func (r *Repo) milestones() (remote.MilestoneStore, remote.Backend, error) {
	c, err := r.Client()
	if err != nil {
		return nil, nil, err
	}
	store, ok := remote.MilestonesOf(c)
	if !ok {
		return nil, nil, ErrNoMilestones
	}
	return store, c, nil
}

// Milestones maps the team's milestone ids to milestones (none on a team
// that keeps none).
func (r *Repo) Milestones() (map[string]remote.Milestone, error) {
	store, _, err := r.milestones()
	if errors.Is(err, ErrNoMilestones) {
		return map[string]remote.Milestone{}, nil
	}
	if err != nil {
		return nil, err
	}
	return store.Milestones(r.Config.ProjectID)
}

func cleanMilestone(name, note string) (string, string, error) {
	name, err := remote.CleanBranchName(name) // the same rules as a branch's name
	if err != nil {
		return "", "", errors.New(strings.Replace(err.Error(), "branch", "milestone", 1))
	}
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > remote.MaxMilestoneNote {
		return "", "", errors.New("a milestone's note has at most 1000 characters")
	}
	return name, note, nil
}

// AddMilestone names version for the whole team; it returns the
// milestone's id.
func (r *Repo) AddMilestone(version, name, note string) (string, error) {
	store, c, err := r.milestones()
	if err != nil {
		return "", err
	}
	if name, note, err = cleanMilestone(name, note); err != nil {
		return "", err
	}
	if !r.HasSnapshot(version) {
		if err := r.fetchSnapshots(c, version); err != nil || !r.HasSnapshot(version) {
			return "", errors.New("that version isn't on the team")
		}
	}
	b := make([]byte, 16)
	rand.Read(b)
	id := hex.EncodeToString(b)
	m := remote.Milestone{Version: version, Name: name, Note: note, By: remote.ActorOf(c), Time: time.Now().UTC()}
	return id, store.PutMilestone(r.Config.ProjectID, id, m)
}

// EditMilestone renames milestone id and changes its note.
func (r *Repo) EditMilestone(id, name, note string) error {
	store, _, err := r.milestones()
	if err != nil {
		return err
	}
	if name, note, err = cleanMilestone(name, note); err != nil {
		return err
	}
	all, err := store.Milestones(r.Config.ProjectID)
	if err != nil {
		return err // never write the record back without what it holds
	}
	m, ok := all[id]
	if !ok {
		return errors.New("that milestone was removed meanwhile")
	}
	m.Name, m.Note = name, note
	return store.PutMilestone(r.Config.ProjectID, id, m)
}

// RemoveMilestone takes milestone id away (the version stays).
func (r *Repo) RemoveMilestone(id string) error {
	store, _, err := r.milestones()
	if err != nil {
		return err
	}
	return store.DeleteMilestone(r.Config.ProjectID, id)
}
