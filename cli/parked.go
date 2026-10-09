package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/nonlabhq/r3v/internal/project"
)

// parkedJSON is a parked set: uncommitted changes kept on this computer
// while the project is elsewhere (see docs/design/parking.md).
type parkedJSON struct {
	Branch string `json:"branch"`       // the branch's key
	At     string `json:"at,omitempty"` // an older version they were made on
	Base   string `json:"base"`         // the version they were made on
	Files  int    `json:"files"`        // changes
	Bytes  int64  `json:"bytes"`        // new content kept for them
	Since  string `json:"since"`        // when they were parked (RFC 3339)
}

func parkedOf(p *project.Parked) *parkedJSON {
	if p == nil {
		return nil
	}
	return &parkedJSON{Branch: p.Branch, At: p.At, Base: p.Base, Files: p.Files, Bytes: p.Bytes, Since: p.Since}
}

// moveJSON is what switch and checkout did.
type moveJSON struct {
	Branch string `json:"branch"`
	To     string `json:"to"`
	Older  bool   `json:"older"` // on an older version of the branch
	// Parked: the changes left behind; Restored: the place's own changes,
	// back (Merged: with the versions since); Waiting: those couldn't be
	// merged without choosing, still parked (r3v parked bring).
	Parked   *parkedJSON `json:"parked"`
	Restored *parkedJSON `json:"restored"`
	Merged   bool        `json:"merged"`
	Waiting  *parkedJSON `json:"waiting"`
	Relink   []string    `json:"relinked"`
}

func moveOf(r *project.Repo, park *project.ParkOutcome, notes []string) moveJSON {
	out := moveJSON{Branch: r.BranchName(), To: r.Head(), Older: r.OnOlderVersion(), Relink: notes}
	if out.Relink == nil {
		out.Relink = []string{}
	}
	if park != nil {
		out.Parked, out.Restored, out.Merged, out.Waiting = parkedOf(park.Parked), parkedOf(park.Restored), park.Merged, parkedOf(park.Waiting)
	}
	return out
}

func printPark(park *project.ParkOutcome) {
	if park == nil {
		return
	}
	if p := park.Parked; p != nil {
		fmt.Printf("parked %d change(s) on %s: they come back when you return\n", p.Files, placeName(p))
	}
	if p := park.Restored; p != nil {
		with := ""
		if park.Merged {
			with = ", merged with the versions since"
		}
		fmt.Printf("brought back %d parked change(s)%s\n", p.Files, with)
	}
	if p := park.Waiting; p != nil {
		fmt.Printf("%d parked change(s) here need choices to come back: r3v parked bring %s --strategy ...\n", p.Files, placeRef(p))
	}
}

func placeName(p *project.Parked) string {
	if p.At != "" {
		return fmt.Sprintf("%s (version %s)", p.Branch, short(p.At))
	}
	return p.Branch
}

// placeRef is how a parked set is named on the command line: branch, or
// branch@version for one made on an older version.
func placeRef(p *project.Parked) string {
	if p.At != "" {
		return p.Branch + "@" + short(p.At)
	}
	return p.Branch
}

// findParked takes branch or branch@version (a prefix of its id).
func findParked(r *project.Repo, ref string) (*project.Parked, error) {
	branch, at, _ := strings.Cut(ref, "@")
	if key, err := r.ResolveBranch(branch); err == nil {
		branch = key
	}
	for _, p := range r.ParkedSets() {
		if p.Branch == branch && (p.At == at || at != "" && strings.HasPrefix(p.At, at)) {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("nothing is parked on %s", ref)
}

func cmdParked(args []string) error {
	if !project.Parking {
		return errors.New("parked changes are on the Nightly channel")
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "list" {
		sets := []parkedJSON{}
		for _, p := range r.ParkedSets() {
			sets = append(sets, *parkedOf(&p))
		}
		result("parked", map[string]any{"parked": sets}, func() {
			if len(sets) == 0 {
				fmt.Println("nothing parked")
			}
			for _, p := range r.ParkedSets() {
				fmt.Printf("%-24s %d change(s)  since %s\n", placeRef(&p), p.Files, p.Since)
			}
		})
		return nil
	}
	fs := flag.NewFlagSet("parked", flag.ContinueOnError)
	strategy := fs.String("strategy", "fail", "where both sides changed the same thing: fail, ours, theirs, both")
	pos, err := parseArgs(fs, args[1:])
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("usage: r3v parked [list] | bring <branch[@version]> [--strategy S] | discard <branch[@version]>")
	}
	p, err := findParked(r, pos[0])
	if err != nil {
		return err
	}
	switch args[0] {
	case "bring":
		if err := guardLiveAlways(r, false); err != nil {
			return err
		}
		notes, err := r.BringParked(p.Branch, p.At, project.Strategy(*strategy))
		if err != nil {
			return err
		}
		result("parked", moveOf(r, &project.ParkOutcome{Restored: p, Merged: true}, notes), func() {
			fmt.Printf("brought %d parked change(s) here, uncommitted\n", p.Files)
		})
	case "discard":
		if err := r.DiscardParked(p.Branch, p.At); err != nil {
			return err
		}
		result("parked", map[string]any{"discarded": parkedOf(p)}, func() {
			fmt.Printf("discarded %d parked change(s) on %s\n", p.Files, placeName(p))
		})
	default:
		return fmt.Errorf("unknown: r3v parked %s", args[0])
	}
	return nil
}
