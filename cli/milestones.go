package cli

import (
	"errors"
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
)

// Milestones (project/milestones.go): a version given a name for the whole
// team ("Sent to the label"), what Git calls a tag; the app shows them on
// the graph.

const milestonesUsage = `
milestones (a version named for the whole team, like a Git tag):
  milestone add NAME [VERSION] [-m NOTE] name a version the team has (default: the one you're on)
  milestone NAME [VERSION] [-m NOTE]     the same, for a name that isn't one of the words here
  milestone rename NAME NEW              rename a milestone
  milestone note NAME NOTE               change its note ("" for none)
  milestone remove NAME                  take a milestone away (the version stays)
  milestones                             the project's milestones, newest first
  milestone [list]                       the same
`

// milestoneWords are milestone's subcommands and words an agent may try
// for one: as the first word they never name a milestone by accident
// ("r3v milestone list" lists). milestone add names a version any of them.
var milestoneWords = map[string]bool{"add": true, "list": true, "ls": true, "rename": true, "note": true,
	"remove": true, "rm": true, "delete": true, "show": true, "help": true}

// milestoneItem is a milestone as --json gives it.
type milestoneItem struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Version string    `json:"version"`
	Note    string    `json:"note,omitempty"`
	By      string    `json:"by,omitempty"` // who named it (a member's id or an agent's)
	Time    time.Time `json:"time"`
}

func milestoneList(r *project.Repo) ([]milestoneItem, error) {
	all, err := r.Milestones()
	if err != nil {
		return nil, err
	}
	out := []milestoneItem{}
	for id, m := range all {
		out = append(out, milestoneItem{ID: id, Name: m.Name, Version: m.Version, Note: m.Note, By: m.By, Time: m.Time})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

// findMilestone is the milestone named name (or whose id starts with it).
func findMilestone(r *project.Repo, name string) (milestoneItem, error) {
	all, err := milestoneList(r)
	if err != nil {
		return milestoneItem{}, err
	}
	var hits []milestoneItem
	for _, m := range all {
		if strings.EqualFold(m.Name, name) {
			hits = append(hits, m)
		}
	}
	if len(hits) == 0 && len(name) >= 6 {
		for _, m := range all {
			if strings.HasPrefix(m.ID, name) {
				hits = append(hits, m)
			}
		}
	}
	switch len(hits) {
	case 0:
		return milestoneItem{}, fmt.Errorf("no milestone %q (r3v milestones lists them)", name)
	case 1:
		return hits[0], nil
	}
	ids := make([]string, len(hits))
	for i, m := range hits {
		ids[i] = m.ID[:8]
	}
	return milestoneItem{}, fmt.Errorf("%d milestones are called %q: name one by its id (%s)", len(hits), name, strings.Join(ids, ", "))
}

func cmdMilestone(args []string) error {
	if len(args) == 0 {
		return listMilestones("milestone", nil)
	}
	explicit := false
	switch strings.ToLower(args[0]) {
	case "list", "ls":
		return listMilestones("milestone", args[1:])
	case "rename", "note", "remove":
		return milestoneChange(strings.ToLower(args[0]), args[1:])
	case "add":
		explicit, args = true, args[1:]
	}
	fs := flag.NewFlagSet("milestone", flag.ContinueOnError)
	note := fs.String("m", "", "a note (what it is, for the team)")
	pos, err := parseArgs(fs, args)
	if err != nil || len(pos) < 1 || len(pos) > 2 {
		return usageError("usage: r3v milestone add NAME [VERSION] [-m NOTE]")
	}
	if word := strings.ToLower(strings.TrimSpace(pos[0])); !explicit && milestoneWords[word] {
		return &cliError{Code: "usage", Exit: exitUsage, Hint: fmt.Sprintf("to name a version so, say it: r3v milestone add %q", pos[0]),
			Err: fmt.Errorf("%q is a word of r3v milestone, not a name (r3v milestones lists them)", pos[0])}
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	ref := "HEAD"
	if len(pos) == 2 {
		ref = pos[1]
	}
	version, err := r.Resolve(ref)
	if err != nil {
		return err
	}
	id, err := r.AddMilestone(version, pos[0], *note)
	if err != nil {
		return milestoneErr(err)
	}
	result("milestone", map[string]any{"id": id, "name": strings.TrimSpace(pos[0]), "version": version}, func() {
		fmt.Printf("milestone %q on version %s, for the whole team\n", strings.TrimSpace(pos[0]), short(version))
	})
	return nil
}

func milestoneChange(op string, args []string) error {
	fs := flag.NewFlagSet("milestone "+op, flag.ContinueOnError)
	pos, err := parseArgs(fs, args)
	want := map[string]int{"rename": 2, "note": 2, "remove": 1}[op]
	if err != nil || len(pos) != want {
		return usageError("usage: r3v milestone %s", map[string]string{"rename": "rename NAME NEW", "note": "note NAME NOTE", "remove": "remove NAME"}[op])
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	m, err := findMilestone(r, pos[0])
	if err != nil {
		return milestoneErr(err)
	}
	switch op {
	case "rename":
		err = r.EditMilestone(m.ID, pos[1], m.Note)
	case "note":
		err = r.EditMilestone(m.ID, m.Name, pos[1])
	case "remove":
		err = r.RemoveMilestone(m.ID)
	}
	if err != nil {
		return milestoneErr(err)
	}
	result("milestone", map[string]any{"id": m.ID, "op": op, "version": m.Version}, func() {
		switch op {
		case "rename":
			fmt.Printf("milestone %q is now %q\n", m.Name, strings.TrimSpace(pos[1]))
		case "note":
			fmt.Printf("milestone %q: note changed\n", m.Name)
		default:
			fmt.Printf("milestone %q removed (version %s stays)\n", m.Name, short(m.Version))
		}
	})
	return nil
}

func cmdMilestones(args []string) error { return listMilestones("milestones", args) }

// listMilestones is r3v milestones (and milestone, milestone list).
func listMilestones(command string, args []string) error {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	pos, err := parseArgs(fs, args)
	if err != nil || len(pos) != 0 {
		return usageError("usage: r3v milestones")
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	all, err := milestoneList(r)
	if err != nil {
		return milestoneErr(err)
	}
	result(command, map[string]any{"milestones": all}, func() {
		if len(all) == 0 {
			fmt.Println("no milestones yet (r3v milestone add NAME names the version you're on)")
			return
		}
		for _, m := range all {
			fmt.Printf("%-24s %s  %s", m.Name, short(m.Version), localTime(m.Time.Format(time.RFC3339)))
			if m.Note != "" {
				fmt.Printf("  %s", m.Note)
			}
			fmt.Println()
		}
	})
	return nil
}

// milestoneErr says what to do when the project or team can't keep them.
func milestoneErr(err error) error {
	if errors.Is(err, project.ErrNoMilestones) || errors.Is(err, project.ErrNoRemote) {
		return fmt.Errorf("%w (milestones are the team's: the project must be shared, on a team that keeps them)", err)
	}
	return err
}
