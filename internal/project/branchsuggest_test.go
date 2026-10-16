package project

import "testing"

func TestClosestName(t *testing.T) {
	names := map[string]string{"main": "main", "lane-design": "lane-design", "lane-render": "lane-render",
		"mias-verse": "Mia's verse", "Mia's verse": "Mia's verse"}
	for _, c := range []struct{ name, want string }{
		{"lane/design", "lane-design"}, // separators and case: near-equal
		{"Lane_Design", "lane-design"},
		{"lane-desing", "lane-design"}, // a typo or two
		{"mian", "main"},
		{"mia's verse", "Mia's verse"},
		{"mias verse", "Mia's verse"},
		{"drums", ""}, // nothing near
		{"x", ""},
	} {
		if got := closestName(c.name, names); got != c.want {
			t.Errorf("closestName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
