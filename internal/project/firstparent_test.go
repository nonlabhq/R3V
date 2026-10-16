package project

import (
	"strings"
	"testing"
)

// The main line follows first parents; each merge on it counts what it
// brought in, once.
func TestFirstParents(t *testing.T) {
	v := func(id string, parents ...string) *Manifest { return &Manifest{ID: id, Parents: parents} }
	// main: r - m1 - m2 - x - m3; lane: r - l1 - l2 (m1), l3 (m2: l1..l3,
	// but l1, l2 already in), and k (m3) on top of m1.
	log := []*Manifest{
		v("m3", "x", "k"), v("k", "m1"), v("x", "m2"), v("m2", "m1", "l3"), v("l3", "l2"),
		v("m1", "r", "l2"), v("l2", "l1"), v("l1", "r"), v("r"),
	}
	line, merged := FirstParents(log)
	var ids []string
	for _, m := range line {
		ids = append(ids, m.ID)
	}
	if strings.Join(ids, " ") != "m3 x m2 m1 r" {
		t.Fatalf("line: %v", ids)
	}
	if merged["m1"] != 2 || merged["m2"] != 1 || merged["m3"] != 1 || len(merged) != 3 {
		t.Fatalf("merged: %v", merged)
	}
}
