package manifest

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func hashOf(s string) string { return ID([]byte(s)) }

func TestTreesRoundTrip(t *testing.T) {
	files := []FileEntry{
		{Path: "a.b", Hash: hashOf("1"), Size: 1},
		{Path: "a/c", Hash: hashOf("2"), Size: 2},
		{Path: "Assets/My Scene.unity", Hash: hashOf("3"), Size: 3},
		{Path: "Assets/Art/x.png", Hash: hashOf("4"), Size: 4},
		{Path: "top.txt", Hash: hashOf("5"), Size: 0},
	}
	root, trees, err := BuildTrees(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(trees) != 4 { // top, a, Assets, Assets/Art
		t.Fatalf("%d trees", len(trees))
	}
	got, err := Flatten(root, func(h string) ([]TreeEntry, error) { return ParseTree(h, trees[h]) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]FileEntry(nil), files...)
	sortFiles(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	// Same files, same trees: unchanged folders keep their hash.
	files[0].Hash = hashOf("changed")
	root2, trees2, _ := BuildTrees(files)
	if root2 == root {
		t.Fatal("a change must change the top tree")
	}
	shared := 0
	for h := range trees2 {
		if trees[h] != nil {
			shared++
		}
	}
	if shared != 3 { // a.b is at the top: a/, Assets/ and Assets/Art/ stay
		t.Fatalf("shared trees: %d", shared)
	}
	if _, _, err := BuildTrees([]FileEntry{{Path: "bad\nname", Hash: hashOf("x")}}); err == nil {
		t.Fatal("line breaks in names must fail")
	}
	empty, trees3, _ := BuildTrees(nil)
	if got, err := Flatten(empty, func(h string) ([]TreeEntry, error) { return ParseTree(h, trees3[h]) }, nil); err != nil || len(got) != 0 {
		t.Fatalf("empty project: %v %v", got, err)
	}
}

func sortFiles(fs []FileEntry) {
	for i := range fs {
		for j := i + 1; j < len(fs); j++ {
			if fs[j].Path < fs[i].Path {
				fs[i], fs[j] = fs[j], fs[i]
			}
		}
	}
}

func TestParseTreeRejectsBadInput(t *testing.T) {
	good := EncodeTree([]TreeEntry{{Name: "x", Hash: hashOf("x"), Size: 1}})
	if _, err := ParseTree(hashOf("other"), good); err == nil {
		t.Fatal("wrong hash")
	}
	for _, bad := range []string{"f abc 1 x\n", "f " + hashOf("x") + " -1 x\n", "d " + hashOf("x") + " a/b\n",
		"q " + hashOf("x") + " x\n", "f " + hashOf("x") + " 1 x"} {
		if _, err := ParseTree(hashOf(bad), []byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestRecord(t *testing.T) {
	m := &Manifest{Version: Format, Parents: []string{}, Author: "yi", Time: "2026-01-01T00:00:00Z", Message: "m",
		Branch: "idea", Tree: hashOf("tree"), FileCount: 3, TotalSize: 42}
	data := m.Seal()
	got, err := Parse(m.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tree != m.Tree || got.FileCount != 3 || got.TotalSize != 42 || got.Files != nil || got.Branch != "idea" {
		t.Fatalf("%+v", got)
	}
	// A record must name its tree.
	none := &Manifest{Version: Format, Parents: []string{}}
	if d := none.Seal(); func() error { _, err := Parse(none.ID, d); return err }() == nil {
		t.Fatal("a record without a tree was read")
	}
	// Newer formats are refused.
	d2 := []byte(strings.Replace(string(data), `"version": 2`, `"version": 3`, 1))
	if _, err := Parse(ID(d2), d2); !errors.Is(err, ErrNewerFormat) {
		t.Fatalf("format 3: %v", err)
	}
}

// A version without an actor encodes exactly as before actors were added:
// the ids of every version already made must not change.
func TestNoActorEncodesAsBefore(t *testing.T) {
	const before = "{\n  \"version\": 2,\n  \"parents\": [\n    \"148de9c5a7a44d19e56cd9ae1a554bf67847afb0c58f6e12fa29ac7ddfca9940\"\n  ],\n  \"author\": \"Mia\",\n  \"author_id\": \"m1\",\n  \"time\": \"2026-10-10T08:00:00Z\",\n  \"message\": \"Title\\n\\nDetails\",\n  \"branch\": \"main\",\n  \"files\": 3,\n  \"size\": 42,\n  \"tree\": \"e3b98a4da31a127d4bde6e43033f66ba274cab0eb7eb1c70ec41402bf6273dd8\"\n}\n"
	const id = "c0a5c8dea520cbe24cf953f81d4b289b664d754825ecba81ca56be2f1baee4b6"
	for _, a := range []*Actor{nil, {}} { // an empty actor is none
		m := &Manifest{Version: 2, Parents: []string{hashOf("p")}, Author: "Mia", AuthorID: "m1",
			Time: "2026-10-10T08:00:00Z", Message: "Title\n\nDetails", Branch: "main", FileCount: 3, TotalSize: 42,
			Tree: hashOf("t"), Actor: a}
		if data := m.Seal(); string(data) != before || m.ID != id {
			t.Fatalf("actor %v: encoding changed:\n%s", a, data)
		}
	}
	got, err := Parse(id, []byte(before))
	if err != nil || got.Actor != nil {
		t.Fatalf("%v %+v", err, got.Actor)
	}
}

func TestTitleAndDetails(t *testing.T) {
	msg := Message(" Fix the renderer ", "", "Lights were too bright.\n\nSee issue 4.")
	if msg != "Fix the renderer\n\nLights were too bright.\n\nSee issue 4." {
		t.Fatalf("%q", msg)
	}
	if Title(msg) != "Fix the renderer" || Details(msg) != "Lights were too bright.\n\nSee issue 4." {
		t.Fatalf("%q %q", Title(msg), Details(msg))
	}
	if Title("one line") != "one line" || Details("one line") != "" || Title("a\r\nb") != "a" || Details("a\r\nb") != "b" {
		t.Fatal("one line, or CRLF")
	}
}

func TestActorRoundTrip(t *testing.T) {
	a := &Actor{Kind: ActorAgent, Tool: "claude-code", Model: "opus", Session: "s1"}
	m := &Manifest{Version: Format, Parents: []string{}, Author: "yi", Time: "2026-01-01T00:00:00Z", Message: "m",
		Tree: hashOf("tree"), Actor: a}
	data := m.Seal()
	if !strings.Contains(string(data), `"actor": {`) || !strings.Contains(string(data), `"tool": "claude-code"`) {
		t.Fatalf("%s", data)
	}
	got, err := Parse(m.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Actor == nil || *got.Actor != *a {
		t.Fatalf("%+v", got.Actor)
	}
	// Only what is known is written.
	m.Actor = &Actor{Kind: ActorAgent, Tool: "claude-code"}
	if data := m.Seal(); strings.Contains(string(data), "model") || strings.Contains(string(data), "session") {
		t.Fatalf("%s", data)
	}
}
