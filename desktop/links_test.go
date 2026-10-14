package desktop

import "testing"

// The r3v:// links R3V Cloud's web app gives: a team, a project, a version.
func TestParseLink(t *testing.T) {
	team, pid, vid := "0123456789abcdef0123456789abcdef", "fedcba9876543210fedcba9876543210", "aa"+"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd"
	for in, want := range map[string]Link{
		"r3v://team/" + team: {Team: team},
		"R3V://TEAM/" + "0123456789ABCDEF0123456789ABCDEF": {Team: team},
		"r3v://project/" + pid + "/":                       {Project: pid},
		"r3v://project/" + pid + "/version/" + vid[:64]:    {Project: pid, Version: vid[:64]},
	} {
		if got, ok := ParseLink(in); !ok || got != want {
			t.Errorf("%s: %+v %v", in, got, ok)
		}
	}
	for _, in := range []string{"https://r3v.so/team/ab", "r3v://team/", "r3v://team/zz", "r3v://project/" + pid + "/files",
		"r3v://user/ab", "--background", "r3v://team/" + team + "/" + team} {
		if got, ok := ParseLink(in); ok {
			t.Errorf("%s: taken as %+v", in, got)
		}
	}
}
