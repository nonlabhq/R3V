package cli

import (
	"encoding/json"
	"fmt"

	"github.com/nonlabhq/r3v/internal/project"
)

// unsharedJSON is another copy of the project on this computer with
// versions not shared (commit --local): a merge or update here can't see
// them, so "up-to-date" doesn't mean they are in.
type unsharedJSON struct {
	Workspace string `json:"workspace"` // its folder
	MainCopy  bool   `json:"main_copy,omitempty"`
	Branch    string `json:"branch"` // its name, as merge takes it
	Count     int    `json:"count"`
}

// unsharedElsewhere is the copies here on branch (a key) with versions not
// shared, and a warning for each (nil, nil: none, or can't tell).
func unsharedElsewhere(r *project.Repo, branch string) ([]unsharedJSON, []warningJSON) {
	copies, err := r.UnsharedElsewhere(branch)
	if err != nil || len(copies) == 0 {
		return nil, nil
	}
	var out []unsharedJSON
	var warns []warningJSON
	for _, c := range copies {
		u := unsharedJSON{Workspace: c.Folder, MainCopy: c.Main, Branch: r.BranchLabel(c.Branch), Count: c.Count}
		out = append(out, u)
		warns = append(warns, warningJSON{Code: "unshared_versions", Reason: unsharedLine(u),
			Workspace: u.Workspace, Branch: u.Branch, Count: u.Count})
	}
	return out, warns
}

func unsharedLine(u unsharedJSON) string {
	where := "the workspace " + u.Workspace
	if u.MainCopy {
		where = "the project's main copy (" + u.Workspace + ")"
	}
	return fmt.Sprintf("%s has %d version(s) on %s not shared yet (committed --local): they aren't on the team's branch, "+
		"so they weren't seen here; `r3v commit` there shares them", where, u.Count, u.Branch)
}

func printUnshared(us []unsharedJSON) {
	for _, u := range us {
		fmt.Println("note: " + unsharedLine(u))
	}
}

// MarshalJSON keeps the other warnings' fields as they always were; an
// unshared_versions warning carries only its own.
func (w warningJSON) MarshalJSON() ([]byte, error) {
	type plain warningJSON
	if w.Code != "unshared_versions" {
		return json.Marshal(plain(w))
	}
	return json.Marshal(struct {
		Code      string `json:"code"`
		Reason    string `json:"reason"`
		Workspace string `json:"workspace,omitempty"`
		Branch    string `json:"branch,omitempty"`
		Count     int    `json:"count,omitempty"`
	}{w.Code, w.Reason, w.Workspace, w.Branch, w.Count})
}
