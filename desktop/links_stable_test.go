//go:build !nightly

package desktop

import "testing"

// Stable opens no r3v:// links.
func TestNoLinksInStable(t *testing.T) {
	a := NewApp()
	if a.handLinks([]string{"r3v://team/" + "0123456789abcdef0123456789abcdef"}) || a.TakeLink() != nil {
		t.Fatal("a link taken in Stable")
	}
}
