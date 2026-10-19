package cli

import (
	"time"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
)

// r3v branch --json: the team's branches as their records hold them.

// branchItem is a branch as r3v branch --json lists it.
type branchItem struct {
	Key     string `json:"key"`
	Name    string `json:"name"` // what people call it (the key when it has no other name)
	Head    string `json:"head,omitempty"`
	Current bool   `json:"current,omitempty"`
	// Kind: "agent" for an agent's branch, "" a person's (or a kind a
	// newer R3V knows); Owner the member an agent works for.
	Kind      string `json:"kind,omitempty"`
	Owner     string `json:"owner,omitempty"`
	OwnerName string `json:"owner_name,omitempty"`
	Color     string `json:"color,omitempty"`  // a palette number ("b3"); "" for the app's pick
	Parent    string `json:"parent,omitempty"` // the branch it was made from
	From      string `json:"from,omitempty"`   // the version it started at
	// Latest is its latest version (none: not on this computer, or no
	// versions yet).
	Latest *versionJSON `json:"latest,omitempty"`
}

func branchItems(branches []project.BranchInfo, recs map[string]remote.BranchRecord, names map[string]string) []branchItem {
	out := []branchItem{}
	for _, b := range branches {
		rec := recs[b.Name]
		it := branchItem{Key: b.Name, Name: branchLabel(b.Name, rec), Head: b.Head, Current: b.Current,
			Kind: rec.Kind, Owner: rec.Owner, OwnerName: names[rec.Owner], Color: rec.Color, Parent: rec.Parent, From: rec.From}
		if b.Latest != nil {
			v := versionOf(b.Latest, names)
			it.Latest = &v
		}
		out = append(out, it)
	}
	return out
}

// branchLabel is what people call a branch: its name, else its key.
func branchLabel(key string, rec remote.BranchRecord) string {
	if rec.Name != "" {
		return rec.Name
	}
	return key
}

// archivedItem is an archived branch as r3v branch archived --json lists it.
type archivedItem struct {
	Key    string    `json:"key"`
	Name   string    `json:"name"`
	Head   string    `json:"head"` // its latest version when archived
	By     string    `json:"by,omitempty"`
	ByName string    `json:"by_name,omitempty"`
	Time   time.Time `json:"time"`
}

// branchMoveItem is a move of r3v branch log --json (oldest first).
type branchMoveItem struct {
	Branch string    `json:"branch"`
	From   string    `json:"from,omitempty"` // "" when it was made
	To     string    `json:"to,omitempty"`   // "" when it was deleted
	By     string    `json:"by,omitempty"`
	ByName string    `json:"by_name,omitempty"`
	Time   time.Time `json:"time"`
}
