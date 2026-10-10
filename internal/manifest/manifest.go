// Package manifest defines the version record (snapshot manifest) kept in a
// project and in the team's storage.
//
// A version record stores the project folder as trees, one per folder (see
// tree.go and docs/design/tree-manifests.md), and names the top tree. In
// memory a manifest always has the flat list of files.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

type FileEntry struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

// Format is the version record format this code writes (2: a version
// keeps its branch); it reads every format up to it.
const Format = 2

// ErrNewerFormat: the version was made by a newer R3V.
var ErrNewerFormat = errors.New("this version was saved by a newer R3V: update R3V to open it")

// Manifest describes one snapshot. Its id is the SHA-256 of its encoding.
type Manifest struct {
	Version int      `json:"version"`
	Parents []string `json:"parents"`
	Author  string   `json:"author"` // the name at the time (older versions: the only record)
	// AuthorID is the team member's id; their current name comes from the
	// team's member list. Empty when no member was chosen.
	AuthorID string `json:"author_id,omitempty"`
	Time     string `json:"time"`
	Message  string `json:"message"`
	// Branch is the key of the branch it was made on (format 2 on): which
	// line of work it is, for good. Empty in older versions.
	Branch string `json:"branch,omitempty"`
	// Files inside the project folder. Records hold Tree instead: Files is
	// filled from the trees (nil in a header).
	Files []FileEntry `json:"files"`
	// Tree is the project folder's tree.
	Tree string `json:"-"`
	// FileCount and TotalSize: how many files, how many bytes (known
	// without reading the trees).
	FileCount int   `json:"-"`
	TotalSize int64 `json:"-"`
	// External are samples referenced by a set from outside the project,
	// keyed by their original absolute path (slash separated).
	External []FileEntry `json:"external,omitempty"`
	// Packs are Live packs whose samples are referenced (not stored).
	Packs []string `json:"packs,omitempty"`
	// Missing are referenced samples that were not found when snapshotting.
	Missing []string `json:"missing,omitempty"`

	ID string `json:"-"`
}

// record is a version record as stored.
type record struct {
	Version  int         `json:"version"`
	Parents  []string    `json:"parents"`
	Author   string      `json:"author"`
	AuthorID string      `json:"author_id,omitempty"`
	Time     string      `json:"time"`
	Message  string      `json:"message"`
	Branch   string      `json:"branch,omitempty"`
	Files    int         `json:"files"`
	Size     int64       `json:"size"`
	Tree     string      `json:"tree"`
	External []FileEntry `json:"external,omitempty"`
	Packs    []string    `json:"packs,omitempty"`
	Missing  []string    `json:"missing,omitempty"`
}

// Encode returns the canonical bytes whose hash is the snapshot id (Tree
// set: see Repo.save).
func (m *Manifest) Encode() []byte {
	data, _ := json.MarshalIndent(record{Version: m.Version, Parents: m.Parents, Author: m.Author,
		AuthorID: m.AuthorID, Time: m.Time, Message: m.Message, Branch: m.Branch, Files: m.FileCount, Size: m.TotalSize,
		Tree: m.Tree, External: m.External, Packs: m.Packs, Missing: m.Missing}, "", "  ")
	return append(data, '\n')
}

// Seal computes and sets the id; call after the manifest is complete.
func (m *Manifest) Seal() []byte {
	data := m.Encode()
	m.ID = ID(data)
	return data
}

func ID(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Parse decodes a manifest and checks it against its id. It comes without
// Files (read its trees for them).
func Parse(id string, data []byte) (*Manifest, error) {
	if got := ID(data); got != id {
		return nil, fmt.Errorf("manifest %s: content hash is %s", short(id), short(got))
	}
	var r record
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("manifest %s: %w", short(id), err)
	}
	if r.Version > Format {
		return nil, fmt.Errorf("version %s: %w", short(id), ErrNewerFormat)
	}
	if r.Version < 1 || !validHash(r.Tree) {
		return nil, fmt.Errorf("manifest %s: not a version record (tree %q)", short(id), r.Tree)
	}
	m := &Manifest{Version: r.Version, Parents: r.Parents, Author: r.Author, AuthorID: r.AuthorID,
		Time: r.Time, Message: r.Message, Branch: r.Branch, Tree: r.Tree, FileCount: r.Files, TotalSize: r.Size,
		External: r.External, Packs: r.Packs, Missing: r.Missing}
	m.ID = id
	return m, nil
}

func (m *Manifest) FileMap() map[string]FileEntry {
	out := make(map[string]FileEntry, len(m.Files))
	for _, f := range m.Files {
		out[f.Path] = f
	}
	return out
}

// Objects lists every blob hash the snapshot needs (not its trees).
func (m *Manifest) Objects() []string {
	var out []string
	for _, f := range m.Files {
		out = append(out, f.Hash)
	}
	for _, f := range m.External {
		out = append(out, f.Hash)
	}
	return out
}

func validHash(h string) bool {
	if len(h) != 64 {
		return false
	}
	_, err := hex.DecodeString(h)
	return err == nil
}

func short(id string) string { return id[:min(10, len(id))] }
