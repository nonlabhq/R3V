package textmerge

import (
	"strings"
	"testing"
)

// Both sides add the same function, in different places: merged it would
// be there twice, so each place is a hunk to decide (keep it there or not).
// The same addition in the same place is taken once; a few short lines
// alike (braces) are no matter.
func TestAddedOnBothSides(t *testing.T) {
	base := "import x\n\nfunction a() {}\n\nfunction z() {}\n"
	fn := "function b() {\n  const v = 2\n  return v\n}\n\n"
	ours := strings.Replace(base, "function a()", fn+"function a()", 1)
	theirs := strings.Replace(base, "function z()", fn+"function z()", 1)
	res := Hunks([]byte(base), []byte(ours), []byte(theirs))
	if len(res.Hunks()) != 2 {
		t.Fatalf("b() added on both sides, apart: %d hunks", len(res.Hunks()))
	}
	if merged, _, _ := res.Resolve([]string{"ours", "ours"}); strings.Count(string(merged), "function b()") != 1 ||
		!strings.HasPrefix(string(merged), "import x\n\nfunction b()") {
		t.Fatalf("kept where ours put it:\n%s", merged)
	}
	merged, clean, _ := Merge([]byte(base), []byte(ours), []byte(ours))
	if !clean || strings.Count(string(merged), "function b()") != 1 {
		t.Fatalf("the same place: %v\n%s", clean, merged)
	}
	// Short lines alike on both sides, apart: merged as before.
	o := strings.Replace(base, "function a() {}", "function a() {\n}", 1)
	th := strings.Replace(base, "function z() {}", "function z() {\n}", 1)
	if merged, clean, _ := Merge([]byte(base), []byte(o), []byte(th)); !clean || !strings.Contains(string(merged), "a() {\n}") ||
		!strings.Contains(string(merged), "z() {\n}") {
		t.Fatalf("short lines: %v\n%s", clean, merged)
	}
}
