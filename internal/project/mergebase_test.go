package project

import (
	"fmt"
	"testing"
)

// The base of a merge is the nearest common ancestor: one that no other
// common ancestor descends from. A lane that merged main in, merged into
// main again, has main's version it took in as its base (not the older
// version of the lane main merged before): else what main changed since
// looks added on both sides (a function twice), or undone.
func TestMergeBaseAfterMergingBothWays(t *testing.T) {
	r, _ := Init(newProject(t), "yi")
	n := 0
	mk := func(parents ...string) string {
		n++
		m := &Manifest{Parents: parents, Author: "yi", Time: fmt.Sprintf("2026-10-12T10:%02d:00Z", n), Message: fmt.Sprint("v", n),
			Files: []FileEntry{}}
		if err := r.save(m); err != nil {
			t.Fatal(err)
		}
		return m.ID
	}
	m1 := mk()
	l1 := mk(m1)     // the lane
	m2 := mk(m1, l1) // main merges the lane
	m3 := mk(m2)     // main goes on
	l2 := mk(l1, m3) // the lane merges main in
	l3 := mk(l2)     // the lane goes on
	for _, c := range []struct{ a, b, want string }{
		{m3, l3, m3}, {l3, m3, m3}, // main merges the lane again; or the lane main
		{m2, l1, l1}, {m1, l1, m1}, {l3, l3, l3},
	} {
		if got, err := r.mergeBase(c.a, c.b); err != nil || got != c.want {
			t.Errorf("base of %s and %s: %s, want %s (%v)", c.a[:6], c.b[:6], short(got), short(c.want), err)
		}
	}
	// Criss-cross: two bases neither descends from; the newer is taken.
	x1 := mk(m1)
	y1 := mk(m1)
	x2 := mk(x1, y1)
	y2 := mk(y1, x1)
	if got, _ := r.mergeBase(x2, y2); got != y1 {
		t.Errorf("criss-cross: %s, want the newer of the two (%s)", short(got), short(y1))
	}
}

// The lanes' story: a lane merged into main, main went on, the lane merged
// main in and went on; main merges the lane again. Main's later changes are
// neither added twice (a function) nor undone (a schema the lane never
// touched since it took main in).
func TestMergeLaneAgainAfterItTookMainIn(t *testing.T) {
	a, _ := codeTeam(t) // (.ts and .json merged as text)
	save := func(msg string) {
		t.Helper()
		if _, _, err := a.Save(msg, Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	}
	sw := func(name string) {
		t.Helper()
		if _, err := a.SwitchBranch(name, false); err != nil {
			t.Fatal(err)
		}
	}
	merge := func(name string) {
		t.Helper()
		if _, err := a.MergeBranch(name, "", Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	}
	write(t, a.Root, "src/api.ts", "export function a() {}\n")
	write(t, a.Root, "schema.json", "{\n  \"v\": 1\n}\n")
	save("main: api, schema")
	if err := a.CreateBranch("lane"); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "src/lane.ts", "export const lane = 1\n")
	save("lane: its file")
	sw("main")
	merge("lane")
	write(t, a.Root, "src/api.ts", "export function a() {}\n\nexport function b() {\n  return 2\n}\n")
	write(t, a.Root, "schema.json", "{\n  \"v\": 2\n}\n")
	save("main: b(), schema v2")
	sw("lane")
	merge("main") // the lane takes main in
	write(t, a.Root, "src/lane.ts", "export const lane = 2\n")
	save("lane: goes on")
	sw("main")
	write(t, a.Root, "schema.json", "{\n  \"v\": 3\n}\n")
	save("main: schema v3")
	merge("lane")
	if got := readText(t, a, "src/api.ts"); got != "export function a() {}\n\nexport function b() {\n  return 2\n}\n" {
		t.Errorf("api.ts (b() twice?):\n%s", got)
	}
	if got := readText(t, a, "schema.json"); got != "{\n  \"v\": 3\n}\n" {
		t.Errorf("schema.json (undone?):\n%s", got)
	}
	if got := readText(t, a, "src/lane.ts"); got != "export const lane = 2\n" {
		t.Errorf("lane.ts:\n%s", got)
	}
}
