package formats

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/blob"
	"github.com/nonlabhq/r3v/internal/chunk"
	"github.com/nonlabhq/r3v/internal/manifest"
)

var update = flag.Bool("update", false, "write testdata/formats again")

const dir = "../../testdata/formats"

// cases.json: every file of the corpus and what a reader gets from it.
type corpus struct {
	About   string       `json:"about"`
	Records []recordCase `json:"records"`
	Trees   []treeCase   `json:"trees"`
	Blobs   []blobCase   `json:"blobs"`
}

type recordCase struct {
	Name string `json:"name"`
	File string `json:"file"` // the record's bytes
	ID   string `json:"id"`   // what it is stored as (its name)
	// OK: R3V reads it (Expect); else Error says why not.
	OK     bool          `json:"ok"`
	Error  string        `json:"error,omitempty"` // hash | newer | not_a_record
	Expect *recordExpect `json:"expect,omitempty"`
}

type recordExpect struct {
	Version  int                  `json:"version"`
	Parents  []string             `json:"parents"`
	Author   string               `json:"author"`
	AuthorID string               `json:"author_id"`
	Time     string               `json:"time"`
	Title    string               `json:"title"`   // the message's first line
	Details  string               `json:"details"` // after the blank line
	Branch   string               `json:"branch"`
	Actor    *manifest.Actor      `json:"actor"`
	Files    int                  `json:"files"`
	Size     int64                `json:"size"`
	Tree     string               `json:"tree"`
	External []manifest.FileEntry `json:"external"`
}

type treeCase struct {
	Name    string      `json:"name"`
	File    string      `json:"file"`
	Hash    string      `json:"hash"` // its name: the SHA-256 of its bytes
	OK      bool        `json:"ok"`
	Error   string      `json:"error,omitempty"` // hash | bad_line
	Entries []treeEntry `json:"entries,omitempty"`
}

type treeEntry struct {
	Kind string `json:"kind"` // d (a folder: hash is its tree) | f (a file)
	Name string `json:"name"`
	Hash string `json:"hash"`
	Size int64  `json:"size"` // files only
}

type blobCase struct {
	Name string `json:"name"`
	File string `json:"file"` // as stored
	Hash string `json:"hash"` // the file's: the SHA-256 of Contents
	// Mode: plain (no header) | z | s | c (a chunk list: Pieces, each a blob
	// of its own, give the file in order).
	Mode     string   `json:"mode"`
	OK       bool     `json:"ok"`
	Error    string   `json:"error,omitempty"`    // newer | hash
	Contents string   `json:"contents,omitempty"` // the file's bytes
	Pieces   []string `json:"pieces,omitempty"`   // their blobs' files, in order
}

func TestCorpus(t *testing.T) {
	if *update {
		write(t)
	}
	data, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c corpus
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Records) == 0 || len(c.Trees) == 0 || len(c.Blobs) == 0 {
		t.Fatal("an empty corpus")
	}
	for _, rc := range c.Records {
		checkRecord(t, rc)
	}
	for _, tc := range c.Trees {
		checkTree(t, tc)
	}
	for _, bc := range c.Blobs {
		checkBlob(t, bc)
	}
}

func read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func checkRecord(t *testing.T, rc recordCase) {
	m, err := manifest.Parse(rc.ID, read(t, rc.File))
	if !rc.OK {
		want := map[string]func(error) bool{
			"newer":        func(e error) bool { return errors.Is(e, manifest.ErrNewerFormat) },
			"hash":         func(e error) bool { return e != nil && strings.Contains(e.Error(), "content hash") },
			"not_a_record": func(e error) bool { return e != nil && strings.Contains(e.Error(), "not a version record") },
		}[rc.Error]
		if want == nil || !want(err) {
			t.Errorf("record %s: want %s, got %v", rc.Name, rc.Error, err)
		}
		return
	}
	if err != nil {
		t.Fatalf("record %s: %v", rc.Name, err)
	}
	e := rc.Expect
	title, details := manifest.Title(m.Message), manifest.Details(m.Message)
	if m.Version != e.Version || strings.Join(m.Parents, ",") != strings.Join(e.Parents, ",") || m.Author != e.Author ||
		m.AuthorID != e.AuthorID || m.Time != e.Time || title != e.Title || details != e.Details || m.Branch != e.Branch ||
		m.FileCount != e.Files || m.TotalSize != e.Size || m.Tree != e.Tree || len(m.External) != len(e.External) ||
		fmt.Sprint(m.Actor) != fmt.Sprint(e.Actor) {
		t.Errorf("record %s: read %+v, the corpus says %+v", rc.Name, m, e)
	}
}

func checkTree(t *testing.T, tc treeCase) {
	entries, err := manifest.ParseTree(tc.Hash, read(t, tc.File))
	if !tc.OK {
		if err == nil || (tc.Error == "hash") != strings.Contains(err.Error(), "content hash") {
			t.Errorf("tree %s: want %s, got %v", tc.Name, tc.Error, err)
		}
		return
	}
	if err != nil {
		t.Fatalf("tree %s: %v", tc.Name, err)
	}
	if len(entries) != len(tc.Entries) {
		t.Fatalf("tree %s: %d entries, the corpus says %d", tc.Name, len(entries), len(tc.Entries))
	}
	for i, e := range entries {
		kind := "f"
		if e.Dir {
			kind = "d"
		}
		w := tc.Entries[i]
		if kind != w.Kind || e.Name != w.Name || e.Hash != w.Hash || e.Size != w.Size {
			t.Errorf("tree %s entry %d: %+v, the corpus says %+v", tc.Name, i, e, w)
		}
	}
}

func checkBlob(t *testing.T, bc blobCase) {
	got, err := readBlob(t, bc)
	if !bc.OK {
		if err == nil || (bc.Error == "newer") != errors.Is(err, blob.ErrNewerFormat) {
			t.Errorf("blob %s: want %s, got %v", bc.Name, bc.Error, err)
		}
		return
	}
	if err != nil {
		t.Fatalf("blob %s: %v", bc.Name, err)
	}
	want := read(t, bc.Contents)
	if !bytes.Equal(got, want) || hashOf(got) != bc.Hash {
		t.Errorf("blob %s: read %d bytes (%s), the corpus says %d (%s)", bc.Name, len(got), hashOf(got)[:8], len(want), bc.Hash[:8])
	}
}

// readBlob reads a stored file as R3V does: its blob, and a chunk list's
// pieces in order, each checked against its hash.
func readBlob(t *testing.T, bc blobCase) ([]byte, error) {
	rc, list, err := blob.Open(bytes.NewReader(read(t, bc.File)))
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || !list {
		return data, err
	}
	l, err := chunk.Parse(data)
	if err != nil {
		return nil, err
	}
	if len(l.Pieces) != len(bc.Pieces) {
		return nil, fmt.Errorf("%d pieces, the corpus lists %d", len(l.Pieces), len(bc.Pieces))
	}
	var out []byte
	for i, p := range l.Pieces {
		piece, err := blob.DecodeBytes(read(t, bc.Pieces[i]))
		if err != nil {
			return nil, err
		}
		if hashOf(piece) != p.Hash || int64(len(piece)) != p.Size {
			return nil, fmt.Errorf("piece %d isn't what the list says", i)
		}
		out = append(out, piece...)
	}
	return out, nil
}

func hashOf(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// --- writing the corpus (-update) ---

func write(t *testing.T) {
	os.RemoveAll(dir)
	put := func(name string, data []byte) string {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return name
	}
	c := corpus{About: "R3V's stored formats, and what R3V reads from each (docs/design/formats.md); " +
		"written by internal/formats (go test ./internal/formats -run Corpus -update), read by its test and by readers outside R3V."}

	// Trees: a folder with a file and a subfolder; an empty one; names with spaces and non-ASCII.
	sub := []manifest.TreeEntry{{Name: "kick.wav", Hash: hashOf([]byte("kick")), Size: 4}}
	subData := manifest.EncodeTree(sub)
	top := []manifest.TreeEntry{
		{Name: "Samples", Dir: true, Hash: manifest.ID(subData)},
		{Name: "Song.als", Hash: hashOf([]byte("song")), Size: 4},
		{Name: "notes – v2 ü.md", Hash: hashOf([]byte("notes")), Size: 5},
	}
	topData := manifest.EncodeTree(top)
	addTree := func(name string, data []byte, entries []manifest.TreeEntry) {
		h := manifest.ID(data)
		tc := treeCase{Name: name, File: put("trees/"+h, data), Hash: h, OK: true, Entries: []treeEntry{}}
		for _, e := range entries {
			kind := "f"
			if e.Dir {
				kind = "d"
			}
			tc.Entries = append(tc.Entries, treeEntry{Kind: kind, Name: e.Name, Hash: e.Hash, Size: e.Size})
		}
		c.Trees = append(c.Trees, tc)
	}
	sorted, err := manifest.ParseTree(manifest.ID(topData), topData) // (as stored: sorted by name)
	if err != nil {
		t.Fatal(err)
	}
	addTree("project folder", topData, sorted)
	addTree("a subfolder", subData, sub)
	addTree("an empty folder", nil, nil)
	bad := []byte("f " + hashOf([]byte("x")) + " notasize name\n")
	c.Trees = append(c.Trees, treeCase{Name: "a bad line", File: put("trees/bad-line", bad), Hash: manifest.ID(bad), Error: "bad_line"})
	c.Trees = append(c.Trees, treeCase{Name: "named by another hash", File: put("trees/wrong-hash", topData), Hash: hashOf([]byte("other")), Error: "hash"})

	// Records: a first version, a merge an agent made, an older one (format 1, no branch).
	tree := manifest.ID(topData)
	addRecord := func(name string, m *manifest.Manifest) {
		data := m.Seal()
		p, err := manifest.Parse(m.ID, data)
		if err != nil {
			t.Fatal(err)
		}
		c.Records = append(c.Records, recordCase{Name: name, File: put("records/"+m.ID+".json", data), ID: m.ID, OK: true,
			Expect: &recordExpect{Version: p.Version, Parents: nonNil(p.Parents), Author: p.Author, AuthorID: p.AuthorID, Time: p.Time,
				Title: manifest.Title(p.Message), Details: manifest.Details(p.Message), Branch: p.Branch, Actor: p.Actor,
				Files: p.FileCount, Size: p.TotalSize, Tree: p.Tree, External: nonNilFiles(p.External)}})
	}
	first := &manifest.Manifest{Version: manifest.Format, Author: "Yi", AuthorID: "0123456789abcdef0123456789abcdef",
		Time: "2026-10-12T09:22:14Z", Message: "First version", Branch: "main", Tree: tree, FileCount: 3, TotalSize: 13}
	addRecord("a first version", first)
	addRecord("a merge an agent made", &manifest.Manifest{Version: manifest.Format, Parents: []string{first.ID, hashOf([]byte("lane"))},
		Author: "Yi", Time: "2026-10-13T01:02:03Z", Message: "Merge branch lane/render\n\nLights were too bright.", Branch: "main",
		Actor: &manifest.Actor{Kind: "agent", Tool: "claude-code", Model: "opus"}, Tree: tree, FileCount: 3, TotalSize: 13,
		External: []manifest.FileEntry{{Path: "C:/Samples/Pack/hat.wav", Hash: hashOf([]byte("hat")), Size: 3}}})
	addRecord("an older version (format 1: no branch)", &manifest.Manifest{Version: 1, Parents: []string{first.ID},
		Author: "Mia", Time: "2026-09-30T12:00:00Z", Message: "Bass line", Tree: tree, FileCount: 3, TotalSize: 13})
	newer := []byte(strings.Replace(string(first.Encode()), `"version": 2`, `"version": 3`, 1))
	c.Records = append(c.Records, recordCase{Name: "made by a newer R3V", File: put("records/newer.json", newer), ID: manifest.ID(newer), Error: "newer"})
	notOne := []byte("{\n  \"version\": 2,\n  \"tree\": \"\"\n}\n")
	c.Records = append(c.Records, recordCase{Name: "not a version record", File: put("records/not-a-record.json", notOne), ID: manifest.ID(notOne), Error: "not_a_record"})
	c.Records = append(c.Records, recordCase{Name: "named by another hash", File: put("records/wrong-hash.json", first.Encode()), ID: hashOf([]byte("other")), Error: "hash"})

	// Blobs: as they are; zstd; stored behind the header (a file that starts
	// like one); a chunk list with its pieces; an unknown mode.
	addBlob := func(name, mode string, contents, stored []byte, pieces []string) {
		h := hashOf(contents)
		c.Blobs = append(c.Blobs, blobCase{Name: name, File: put("blobs/"+mode+"-"+h, stored), Hash: h, Mode: mode, OK: true,
			Contents: put("contents/"+h, contents), Pieces: pieces})
	}
	plain := []byte("kick\nsnare\n")
	addBlob("a small file, as it is", "plain", plain, blob.EncodeBytes(plain), nil)
	text := bytes.Repeat([]byte("the same line, many times\n"), 400)
	z := blob.EncodeBytes(text)
	if !bytes.HasPrefix(z, []byte(blob.Magic+"z")) {
		t.Fatal("the text wasn't compressed")
	}
	addBlob("a file that compresses (zstd)", "z", text, z, nil)
	odd := append([]byte(blob.Magic), []byte("starts like a header")...)
	s := blob.EncodeBytes(odd)
	if !bytes.HasPrefix(s, []byte(blob.Magic+"s")) {
		t.Fatal("not stored behind the header")
	}
	addBlob("a file that starts like a header", "s", odd, s, nil)
	pieceA, pieceB := bytes.Repeat([]byte("A"), 700), []byte("tail of the file")
	l := chunk.List{Pieces: []chunk.Piece{{Hash: hashOf(pieceA), Size: int64(len(pieceA))}, {Hash: hashOf(pieceB), Size: int64(len(pieceB))}}}
	pa := put("pieces/"+hashOf(pieceA), blob.EncodeBytes(pieceA))
	pb := put("pieces/"+hashOf(pieceB), blob.EncodeBytes(pieceB))
	addBlob("a big file as pieces (a chunk list)", "c", append(append([]byte{}, pieceA...), pieceB...), blob.ChunkList(l.Encode()), []string{pa, pb})
	unknown := append([]byte(blob.Magic), 'q', 1, 2, 3)
	c.Blobs = append(c.Blobs, blobCase{Name: "made by a newer R3V (a mode R3V doesn't know)", File: put("blobs/newer-mode", unknown), Mode: "q", Error: "newer"})

	out, _ := json.MarshalIndent(c, "", "  ")
	put("cases.json", append(out, '\n'))
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilFiles(s []manifest.FileEntry) []manifest.FileEntry {
	if s == nil {
		return []manifest.FileEntry{}
	}
	return s
}
