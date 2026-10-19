package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/actor"
	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// openRepo opens the project here, kept from other programs (the app) until
// the command ends: waiting up to lockWait for one that has it. A switch
// left halfway is put back first (recoverSwitch).
func openRepo() (*project.Repo, error) {
	r, err := project.Open(".")
	if err != nil {
		return nil, err
	}
	release, err := r.Lock(lockWait)
	if err != nil {
		return nil, err
	}
	held = append(held, release)
	r.Actor = actor.Detect() // (the app never sets it: its versions are a person's)
	recoverSwitch(r)
	ensureRules(r)
	return r, nil
}

// openRead opens the project here for a command that only reads it: as
// openRepo when the lock is free soon, else as it is, writing nothing (as
// the app reads one: desktop openRead). An agent's status or log then
// doesn't fail while another commits; readBusy says who has it.
func openRead() (*project.Repo, error) {
	r, err := project.Open(".")
	if err != nil {
		return nil, err
	}
	wait := readLockWait
	if waitGiven {
		wait = lockWait
	}
	release, err := r.Lock(wait)
	var busy *project.BusyError
	if errors.As(err, &busy) {
		r.ReadOnly = true
		readBusy = &busyJSON{Busy: true, Holder: holderOf(busy.Holder)}
		if !jsonMode {
			who := "another R3V window or command"
			if busy.Holder != nil {
				who = busy.Holder.String()
			}
			fmt.Fprintf(os.Stderr, "note: %s is working on this project: read as it is, nothing kept here\n", who)
		}
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	held = append(held, release)
	r.Actor = actor.Detect()
	recoverSwitch(r) // (with the lock: reading half-written files would mislead)
	ensureRules(r)
	return r, nil
}

// readLockWait is how long a reading command waits for the project's lock
// before reading without it (unless --wait says).
const readLockWait = 300 * time.Millisecond

// defaultLockWait is how long a command waits for the project's lock
// without --wait.
const defaultLockWait = 10 * time.Second

// lockWait is how long this command waits for the project's lock
// (--wait SECONDS); waitGiven: --wait was given.
var (
	lockWait  = defaultLockWait
	waitGiven bool
)

// stripWait takes --wait SECONDS (or --wait=SECONDS) out of args, as
// stripJSON does --json: every command that takes the project's lock
// follows it.
func stripWait(args []string) ([]string, time.Duration, bool, error) {
	var out []string
	wait, given := defaultLockWait, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value, hasValue := strings.Cut(a, "=")
		if name != "--wait" && name != "-wait" {
			out = append(out, a)
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return nil, 0, false, usageError("--wait needs a number of seconds")
			}
			i++
			value = args[i]
		}
		s, err := strconv.ParseFloat(value, 64)
		if err != nil || s < 0 || s > 24*3600 {
			return nil, 0, false, usageError("--wait %q: a number of seconds (0 to 86400)", value)
		}
		wait, given = time.Duration(s*float64(time.Second)), true
	}
	return out, wait, given, nil
}

// busyJSON goes into a reading command's result when it read the project
// without its lock (readBusy): another program was working on it.
type busyJSON struct {
	Busy   bool        `json:"busy"`
	Holder *holderJSON `json:"holder,omitempty"`
}

// readBusy: this command read the project without its lock (openRead).
var readBusy *busyJSON

// held are the locks the command took: released when it ends (Run).
var held []func()

func releaseHeld() {
	for _, release := range held {
		release()
	}
	held = nil
}

// ensureRules writes the project's .r3v.yaml, or adds the presets R3V
// finds to an older one, as the app does when it opens a project.
func ensureRules(r *project.Repo) {
	switch did, err := r.EnsureRules(); {
	case err != nil:
		fmt.Fprintf(os.Stderr, "warning: %s: %v\n", profile.FileName, err)
	case did == "created":
		fmt.Fprintf(os.Stderr, "wrote %s: the presets R3V found (commit it with the project)\n", profile.FileName)
	case did != "":
		fmt.Fprintf(os.Stderr, "%s: added the presets R3V found (commit it with the project)\n", profile.FileName)
	}
}

// printSuggestions tells about projects of tools found in folders the rules
// don't name yet.
func printSuggestions(r *project.Repo) {
	p, err := r.Profile()
	if err != nil {
		return
	}
	for _, s := range p.Suggestions() {
		folder := s.Folder
		if folder == "" {
			folder = "."
		}
		fmt.Printf("found: %s; its rules leave out %s.\n  r3v rules preset %q %s   (or none: not a project)\n",
			s, strings.Join(s.LeftOut, ", "), folder, s.Preset)
	}
}

// tidy keeps .r3v small, as the app does after each operation: of a team
// project, only the sets stay here (the current version's other files are
// in the project folder, older ones in the team's storage); leftovers no
// version refers to go. Best effort: the command's work is done already.
func tidy(r *project.Repo) {
	r.PruneObjects()
	r.GC()
}

func short(id string) string { return id[:min(10, len(id))] }

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	author := fs.String("author", "", "name recorded on snapshots (default: OS user)")
	preset := fs.String("preset", "", "for a folder R3V doesn't recognise: code (text merged line by line, .gitignore followed) or none (every file as it is)")
	teamName := fs.String("team", "", "a team this computer has joined (its name, as r3v teams lists it): the project is that team's")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var team *teams.Team
	if *teamName != "" { // (before anything is made: a name that isn't one leaves the folder as it was)
		t, err := joinedTeam(*teamName)
		if err != nil {
			return err
		}
		team = t
	}
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	if *preset != "" {
		if err := project.ChooseKind(dir, *preset); err != nil {
			return err
		}
	}
	r, err := project.Init(dir, *author)
	if errors.Is(err, project.ErrUnknownKind) {
		return fmt.Errorf("%w; to keep it all the same: r3v init --preset code (code and documents) or --preset none (files as they are)", err)
	}
	if err != nil {
		return err
	}
	ensureRules(r)
	out := initJSON{Folder: r.Root, Project: r.Config.Name, Author: r.Config.Author}
	if team != nil {
		wait := 30 * time.Second
		if waitGiven {
			wait = lockWait
		}
		release, err := r.Lock(wait)
		if err != nil {
			return err
		}
		held = append(held, release)
		if err := joinTeam(r, team); err != nil {
			return err
		}
		out.Team = team.Name
	} else {
		out.Next = connectSteps()
	}
	out.Next = append(out.Next, agentSteps(r.Root)...)
	first := nextJSON{`r3v commit -m "first version"`, "commit the first version: shared with the team once the project is connected, on this computer only until then"}
	if team != nil {
		first.Why = "commit the first version and share it with " + team.Name
	}
	out.Next = append(out.Next, first)
	result("init", out, func() {
		fmt.Printf("initialized r3v project in %s (author %s)\n", r.Root, r.Config.Author)
		if team != nil {
			fmt.Printf("it is %s's project\n", team.Name)
		} else {
			fmt.Println("not connected to a team yet: versions committed now stay on this computer")
		}
		printNext("next:", out.Next)
	})
	return nil
}

// initJSON is `r3v init --json`: the project made, its team ("" for
// none) and what to run next.
type initJSON struct {
	Folder  string     `json:"folder"`
	Project string     `json:"project"`
	Author  string     `json:"author"`
	Team    string     `json:"team"`
	Next    []nextJSON `json:"next"`
}

// statusJSON is `r3v status --json`.
type statusJSON struct {
	Project string `json:"project"`
	Branch  string `json:"branch"`
	Version string `json:"version"` // the version the files are on ("" before the first)
	// OnOlderVersion: checked out an older version (Latest is the newest).
	OnOlderVersion bool      `json:"on_older_version"`
	Latest         string    `json:"latest,omitempty"`
	Team           *teamJSON `json:"team"` // null when not in a team
	// Connected: the project is in a team (false: its versions stay on
	// this computer; Next says how to connect it).
	Connected bool         `json:"connected"`
	Next      []nextJSON   `json:"next,omitempty"`
	Changes   []changeJSON `json:"changes"`
	// InUse: files another program holds (Live writing a Freeze): read once
	// they are free, counted as in the version you are on until then.
	InUse []string `json:"in_use"`
	// Suggestions: projects of tools found in folders the rules don't name.
	Suggestions []suggestionJSON `json:"suggestions"`
	// MainCopy: in a workspace, the folder of the project's main copy.
	MainCopy string `json:"main_copy,omitempty"`
	// Base: in a workspace, the branch it was made from when that has
	// versions this branch hasn't (r3v merge brings them in).
	Base *baseJSON `json:"base,omitempty"`
	// Warnings: what is worth a look before committing (see warningJSON).
	Warnings []warningJSON `json:"warnings"`
	// Unshared: versions committed here the team's branch doesn't have
	// (commit --local); no other copy sees them until shared. Absent
	// without a team, or when it can't be reached.
	Unshared *int `json:"unshared,omitempty"`
}

// warningJSON is a word before committing, never a refusal. Code
// "not_connected": the project is in no team, what is committed stays on
// this computer (Reason says so; the other fields are empty). Code
// "looks_built": a folder new in the changes looks like a build's output
// (named like a folder the rules leave out, or full of built files or
// bytes); committed, it stays in the team's storage for good. Kind says
// why, for programs: "name" (named like Like, a folder the rules leave
// out), "built" (many built files) or "size" (many new bytes); Reason, in
// English. Rule is the line for .r3v.yaml's rules: that leaves it out
// (Pattern, its pattern).
type warningJSON struct {
	Code    string `json:"code"`
	Folder  string `json:"folder"`
	Kind    string `json:"kind"`
	Like    string `json:"like,omitempty"`
	Reason  string `json:"reason"`
	Files   int    `json:"files"`
	Bytes   int64  `json:"bytes"`
	Rule    string `json:"rule"`
	Pattern string `json:"pattern"`
	// Code "unshared_versions" (merge, update: unshared.go): another copy
	// here with versions not shared, which this couldn't see; Reason says
	// so in English.
	Workspace string `json:"workspace,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Count     int    `json:"count,omitempty"`
}

func builtWarnings(found []project.BuiltFolder) []warningJSON {
	out := []warningJSON{}
	for _, b := range found {
		out = append(out, warningJSON{Code: "looks_built", Folder: b.Folder, Kind: b.Kind, Like: b.Like, Reason: b.Reason,
			Files: b.Files, Bytes: b.Bytes, Rule: "ignore: " + strconv.Quote(b.Rule), Pattern: b.Rule})
	}
	return out
}

// printWarnings says what each looks_built warning is about, and the rule
// to add (the others are said where they belong).
func printWarnings(ws []warningJSON, committed bool) {
	for _, w := range ws {
		if w.Code != "looks_built" {
			continue
		}
		files := fmt.Sprintf("%d files", w.Files)
		if w.Files == 1 {
			files = "1 file"
		}
		switch w.Kind {
		case project.BuiltByName:
			fmt.Printf("note: %s/ is named like %s, which the rules leave out (%s, %s)\n", w.Folder, w.Like, files, humanSize(w.Bytes))
		case project.BuiltBySize:
			fmt.Printf("note: %s/ is big (%s) and new (%s)\n", w.Folder, humanSize(w.Bytes), files)
		default:
			fmt.Printf("note: %s/ looks like a build's output (%s; %s, %s)\n", w.Folder, w.Reason, files, humanSize(w.Bytes))
		}
		if committed {
			fmt.Printf("  it is in this version now; to leave it out of the next ones: r3v rules ignore %q  (adds  - %s  to %s)\n", w.Pattern, w.Rule, profile.FileName)
		} else {
			fmt.Printf("  committed, it stays in the team's storage for good; to leave it out: r3v rules ignore %q  (adds  - %s  to %s)\n", w.Pattern, w.Rule, profile.FileName)
		}
	}
}

type baseJSON struct {
	Branch string `json:"branch"` // its name, as r3v merge takes it
	Behind int    `json:"behind"` // its versions this branch hasn't
}

// baseNews is a workspace's base branch when it has versions this branch
// hasn't (nil otherwise).
func baseNews(r *project.Repo) *baseJSON {
	key, n := r.BaseNews()
	if n == 0 {
		return nil
	}
	return &baseJSON{Branch: r.BranchLabel(key), Behind: n}
}

// printBaseNews says what the base branch has, and how to bring it in.
func printBaseNews(b *baseJSON) {
	if b != nil {
		fmt.Printf("%s has %d version(s) this branch hasn't: `r3v merge %s` brings them in (r3v update only gets this branch's)\n",
			b.Branch, b.Behind, b.Branch)
	}
}

type teamJSON struct {
	Reachable bool   `json:"reachable"`
	Incoming  bool   `json:"incoming"` // others saved versions: r3v update
	Error     string `json:"error,omitempty"`
	// Archived: the team archived the project (docs/design/archive.md).
	Archived bool `json:"archived,omitempty"`
	// Section: the team's section the project is in (docs/design/sections.md).
	Section string `json:"section,omitempty"`
}

type suggestionJSON struct {
	Folder  string   `json:"folder"`
	Preset  string   `json:"preset"`
	LeftOut []string `json:"left_out"`
}

func cmdStatus(args []string) error {
	if len(args) > 0 {
		return usageError("usage: r3v status [--json]")
	}
	r, err := openRead()
	if err != nil {
		return err
	}
	out := statusJSON{Project: r.Config.Name, Branch: r.BranchName(), Version: r.Head(),
		OnOlderVersion: r.OnOlderVersion(), Changes: []changeJSON{}, Suggestions: []suggestionJSON{}}
	if out.OnOlderVersion {
		out.Latest = r.Latest()
	}
	if r.IsWorkspace() {
		out.MainCopy = r.MainRoot()
		out.Base = baseNews(r)
	}
	if r.Config.Remote != nil {
		incoming, err := r.Incoming()
		out.Team = &teamJSON{Reachable: err == nil, Incoming: incoming}
		if err != nil {
			out.Team.Error = err.Error()
		} else if c, err := r.Client(); err == nil && remote.ProjectArchive {
			archived, _ := remote.ArchivedProjects(c)
			_, out.Team.Archived = archived[r.Config.ProjectID]
		}
		if out.Team.Error == "" {
			if n, err := r.Unshared(); err == nil {
				out.Unshared = &n
			}
		}
		if c, err := r.Client(); err == nil && out.Team.Error == "" && remote.ProjectSections {
			ss, _ := remote.Sections(c)
			if i := remote.FindSection(ss, remote.SectionOf(ss, r.Config.ProjectID)); i >= 0 {
				out.Team.Section = ss[i].Name
			}
		}
	}
	if p, err := r.Profile(); err == nil {
		for _, s := range p.Suggestions() {
			folder := s.Folder
			if folder == "" {
				folder = "."
			}
			out.Suggestions = append(out.Suggestions, suggestionJSON{Folder: folder, Preset: s.Preset,
				LeftOut: nonNil(s.LeftOut)})
		}
	}
	changes, built, err := r.StatusChecked()
	if err != nil {
		return err
	}
	out.Warnings = builtWarnings(built)
	if out.Connected = connected(r); !out.Connected {
		out.Warnings = append(out.Warnings, notConnected())
		out.Next = connectSteps()
	}
	for _, c := range changes {
		ch := changeJSON{Path: c.Path, Status: c.Status, From: c.From, Edited: c.Edited}
		if c.SetDiff != nil {
			ch.SetChanges, ch.Weight = setLines(c.SetDiff.Render()), c.SetDiff.Weight()
		}
		out.Changes = append(out.Changes, ch)
	}
	out.InUse = nonNil(r.InUse())
	result("status", out, func() { printStatus(r, out); printBaseNews(out.Base) })
	return nil
}

func printStatus(r *project.Repo, s statusJSON) {
	if s.MainCopy != "" {
		fmt.Printf("workspace on branch %s (the project's main copy: %s)\n", r.BranchLabel(s.Branch), s.MainCopy)
	}
	switch {
	case s.OnOlderVersion:
		fmt.Printf("on an older version %s (latest: %s; `r3v goto latest` goes back)\n", short(s.Version), short(s.Latest))
	case s.Version != "":
		fmt.Printf("on version %s\n", short(s.Version))
	default:
		fmt.Println("no snapshots yet")
	}
	switch t := s.Team; {
	case t == nil:
		fmt.Println("not connected to a team: versions committed here stay on this computer")
		defer printNext("to connect it:", s.Next)
	case !t.Reachable:
		fmt.Printf("team: not reachable (%s)\n", t.Error)
	case t.Incoming:
		fmt.Println("team: new versions saved by others (run `r3v update`)")
	default:
		fmt.Println("team: up to date")
	}
	if s.Unshared != nil && *s.Unshared > 0 {
		fmt.Printf("%d version(s) committed here not shared yet: the team and the project's other copies don't see them; `r3v commit` shares them\n", *s.Unshared)
	}
	printSuggestions(r)
	for _, p := range s.InUse {
		fmt.Printf("in use by another program, read once it's free: %s\n", p)
	}
	if len(s.Changes) == 0 {
		fmt.Println("nothing changed")
		return
	}
	defer printWarnings(s.Warnings, false)
	sym := map[string]string{"added": "+", "modified": "~", "deleted": "-", "renamed": "M", "untracked": "o"}
	for _, c := range s.Changes {
		if c.Status == "renamed" {
			edited := ""
			if c.Edited {
				edited = " (and changed)"
			}
			fmt.Printf("M %s -> %s%s\n", c.From, c.Path, edited)
			continue
		}
		fmt.Printf("%s %s\n", sym[c.Status], c.Path)
		for _, line := range c.SetChanges {
			fmt.Println("    " + line)
		}
	}
}

func cmdLog(args []string) error {
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	limit := fs.Int("n", 0, "show only the newest N versions")
	firstParent := fs.Bool("first-parent", false, "only the main line: each merge once, with how many versions it brought in")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	r, err := openRead()
	if err != nil {
		return err
	}
	var log []*project.Manifest
	if len(pos) > 0 { // a branch (its name may be several words)
		log, err = r.BranchVersions(strings.Join(pos, " "))
	} else {
		log, err = r.Log()
	}
	if err != nil {
		return err
	}
	var merged map[string]int
	if *firstParent {
		log, merged = project.FirstParents(log)
	}
	if *limit > 0 && len(log) > *limit {
		log = log[:*limit]
	}
	// Mark where each team branch is (best effort).
	tips := map[string][]string{}
	if r.Config.Remote != nil {
		if branches, err := r.Branches(); err == nil {
			for _, b := range branches {
				tips[b.Head] = append(tips[b.Head], b.Name)
			}
		}
	}
	names := r.MemberNames()
	out := struct {
		Versions []versionJSON `json:"versions"` // newest first
	}{Versions: []versionJSON{}}
	for _, m := range log {
		v := versionOf(m, names)
		v.Branches = tips[m.ID]
		v.MergedCount = merged[m.ID]
		out.Versions = append(out.Versions, v)
	}
	result("log", out, func() {
		if len(log) == 0 {
			fmt.Println("no snapshots yet")
		}
		for i, m := range log {
			mark := ""
			if n := merged[m.ID]; n == 1 {
				mark = "  (+1 version)"
			} else if n > 1 {
				mark = fmt.Sprintf("  (+%d versions)", n)
			}
			if b := out.Versions[i].Branches; len(b) > 0 {
				mark += "  [" + strings.Join(b, ", ") + "]"
			}
			fmt.Printf("%s  %s  %-12s %s%s\n", short(m.ID), when(m), byLine(out.Versions[i].Author, m), manifest.Title(m.Message), mark)
		}
	})
	return nil
}

// cmdDiscard: r3v discard, the app's "Discard changes": your changes
// thrown away, the files as in the version you're on (goto HEAD --force,
// without the Git word).
// With paths, only the changes in those files or folders go (as commit
// takes paths); --preview lists what would go, changing nothing.
func cmdDiscard(args []string) error {
	fs := flag.NewFlagSet("discard", flag.ContinueOnError)
	preview := fs.Bool("preview", false, "only list the changes that would be thrown away")
	paths, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	changes, err := r.Status()
	if err != nil {
		return err
	}
	if len(paths) > 0 {
		if changes, err = changesUnder(r, paths, changes); err != nil {
			return err
		}
		if len(changes) == 0 {
			return &cliError{Code: "nothing_to_discard", Exit: exitUsage, Err: fmt.Errorf("nothing changed in %s", strings.Join(paths, ", "))}
		}
	}
	changes = slices.DeleteFunc(changes, func(c project.Change) bool { return c.Status == "untracked" }) // (not a change: left as it is)
	out := discardJSON{Discarded: []changeJSON{}, Preview: *preview}
	for _, c := range changes {
		out.Discarded = append(out.Discarded, changeJSON{Path: c.Path, Status: c.Status, From: c.From})
	}
	printList := func() {
		for _, c := range out.Discarded {
			fmt.Printf("  %-9s %s\n", c.Status, c.Path)
		}
	}
	if *preview {
		out.moveJSON = moveOf(r, nil, nil)
		result("discard", out, func() {
			if len(changes) == 0 {
				fmt.Println("nothing to discard")
				return
			}
			fmt.Println("would discard (r3v discard without --preview to do it):")
			printList()
		})
		return nil
	}
	if err := guardLiveAlways(r, false); err != nil {
		return err
	}
	defer tidy(r)
	if len(paths) > 0 {
		if err := r.DiscardChanges(changes); err != nil {
			return err
		}
		out.moveJSON = moveOf(r, nil, nil)
		result("discard", out, func() {
			fmt.Printf("discarded (the files are as in version %s):\n", short(r.Head()))
			printList()
		})
		return nil
	}
	m, notes, err := r.GoTo(r.Head(), true)
	if err != nil {
		return err
	}
	out.moveJSON = moveOf(r, nil, notes)
	result("discard", out, func() {
		fmt.Printf("your changes are discarded: the files are as in version %s  %s\n", short(m.ID), m.Message)
	})
	return nil
}

// discardJSON is discard's result: where the project is, and the changes
// thrown away (or, with --preview, that would be).
type discardJSON struct {
	moveJSON
	Discarded []changeJSON `json:"discarded"`
	Preview   bool         `json:"preview,omitempty"`
}

func cmdGoto(args []string) error {
	fs := flag.NewFlagSet("goto", flag.ContinueOnError)
	force := fs.Bool("force", false, "discard uncommitted changes; ignore a running Live")
	var ref string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return err
		}
		args = fs.Args()
		if len(args) > 0 {
			ref, args = args[0], args[1:]
		}
	}
	if ref == "" {
		return errors.New("usage: r3v goto <version|HEAD~N|latest> [--force]")
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	if err := guardLiveAlways(r, *force); err != nil {
		return err
	}
	defer tidy(r)
	m, notes, err := r.GoTo(ref, *force)
	if err != nil {
		return err
	}
	result("goto", moveOf(r, r.Park, notes), func() {
		fmt.Printf("now on version %s  %s\n", short(m.ID), m.Message)
		for _, n := range notes {
			fmt.Println("  relinked " + n)
		}
		printPark(r.Park)
		if r.OnOlderVersion() {
			fmt.Println("this is an older version: `r3v goto latest` goes back, `r3v branch new NAME` continues from here")
		}
	})
	return nil
}

// cmdExport writes a version as a separate project folder.
func cmdExport(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: r3v export <id|HEAD~N> <folder>")
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	m, err := r.Export(args[0], args[1])
	if err != nil {
		return err
	}
	fmt.Printf("exported version %s  %s\n  to %s\n", short(m.ID), m.Message, args[1])
	return nil
}

// cmdGC frees space in .r3v: for a team project, local copies of files
// the team's storage has (sets stay); for any project, objects no version
// uses.
func cmdVerify(args []string) (int, error) {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	repair := fs.Bool("repair", false, "bring back what can be: from the project folder or the team's storage")
	if err := fs.Parse(args); err != nil {
		return 0, err
	}
	r, err := openRepo()
	if err != nil {
		return 0, err
	}
	rep, err := r.Verify(*repair)
	if err != nil {
		return 0, err
	}
	if jsonMode {
		out := verifyOf(rep)
		out.TeamUnreachable = r.Config.Remote != nil && !rep.TeamChecked
		if out.OpenProblems > 0 { // (exit 1, as in text: ok must say so too)
			hint := "show the user error.details.problems; r3v verify --repair brings back what can be"
			if *repair {
				hint = "show the user error.details.problems: what --repair couldn't bring back"
			}
			return 0, &cliError{Code: "verify_problems", Exit: exitError, Details: out, Hint: hint,
				Err: fmt.Errorf("%d problem(s) not repaired: %s", out.OpenProblems, out.Summary)}
		}
		result("verify", out, nil)
		return 0, nil
	}
	open := 0
	for _, p := range rep.Problems {
		what := p.ID[:min(10, len(p.ID))]
		if p.Path != "" {
			what = p.Path
		}
		status := ""
		switch {
		case p.Fixed:
			status = "  -> repaired: " + p.How
		case p.How != "":
			status = "  -> " + p.How
			open++
		default:
			open++
		}
		fmt.Printf("%-12s %s: %s%s\n", p.Kind, what, p.Detail, status)
	}
	if r.Config.Remote != nil && !rep.TeamChecked {
		fmt.Println("(the team's storage couldn't be reached: files only it has weren't checked)")
	}
	fmt.Println(rep.Summary())
	if open > 0 {
		if !*repair {
			fmt.Println("run `r3v verify --repair` to bring back what can be")
		}
		return 1, nil
	}
	return 0, nil
}

// verifyJSON is `r3v verify --json`: what was checked, and where the files
// are (few kept here after sharing is expected: team_only_files).
type verifyJSON struct {
	Versions        int   `json:"versions"`       // every version record on this computer
	InHistory       int   `json:"in_history"`     // of them, this branch's history (as log lists it)
	OtherVersions   int   `json:"other_versions"` // other branches' or workspaces', or R3V's own
	FolderLists     int   `json:"folder_lists"`   // checked
	LocalFiles      int   `json:"local_files"`    // stored on this computer, read again
	LocalBytes      int64 `json:"local_bytes"`
	TeamOnlyFiles   int   `json:"team_only_files"`   // only in the team's storage (downloaded when needed)
	FolderOnlyFiles int   `json:"folder_only_files"` // only in the project folder
	TeamChecked     bool  `json:"team_checked"`
	// TeamUnreachable: in a team whose storage couldn't be asked: files only
	// it has weren't checked.
	TeamUnreachable bool          `json:"team_unreachable"`
	Repaired        bool          `json:"repaired"` // --repair was given
	Problems        []problemJSON `json:"problems"`
	OpenProblems    int           `json:"open_problems"` // not repaired (then exit 1: verify_problems)
	Summary         string        `json:"summary"`
}

type problemJSON struct {
	Kind   string `json:"kind"` // version, folder-list, file
	ID     string `json:"id"`
	Path   string `json:"path,omitempty"`
	Detail string `json:"detail"`
	Fixed  bool   `json:"fixed"`
	How    string `json:"how,omitempty"`
}

func verifyOf(rep *project.VerifyReport) verifyJSON {
	out := verifyJSON{Versions: rep.Versions, InHistory: rep.InHistory, OtherVersions: rep.Versions - rep.InHistory,
		FolderLists: rep.Folders, LocalFiles: rep.Files, LocalBytes: rep.Bytes, TeamOnlyFiles: rep.TeamOnly,
		FolderOnlyFiles: rep.InFolder, TeamChecked: rep.TeamChecked, Repaired: rep.Repaired,
		Problems: []problemJSON{}, Summary: rep.Summary()}
	for _, p := range rep.Problems {
		out.Problems = append(out.Problems, problemJSON{Kind: p.Kind, ID: p.ID, Path: p.Path, Detail: p.Detail, Fixed: p.Fixed, How: p.How})
		if !p.Fixed {
			out.OpenProblems++
		}
	}
	return out
}

func cmdStorageCleanup(args []string) error {
	fs := flag.NewFlagSet("storage-cleanup", flag.ContinueOnError)
	del := fs.Bool("delete", false, "delete the unused files that are due")
	if err := fs.Parse(args); err != nil {
		return err
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	c, err := r.Client()
	if err != nil {
		return err
	}
	s3, ok := c.(*remote.BucketBackend)
	if !ok {
		return errors.New("cleanup works on teams that use S3 or R2 storage")
	}
	rep, err := s3.CollectGarbage(*del)
	if err != nil {
		return err
	}
	mb := func(n int64) string { return fmt.Sprintf("%.1f MB", float64(n)/(1<<20)) }
	fmt.Printf("%d files in storage, used by %d versions\n", rep.Stored, rep.Versions)
	if rep.Deleted > 0 {
		fmt.Printf("deleted %d unused files (%s)\n", rep.Deleted, mb(rep.DeletedBytes))
	}
	switch {
	case rep.Waiting == 0:
		fmt.Println("nothing (else) unused")
	case rep.Due > rep.Deleted:
		fmt.Printf("%d unused files (%s), %d (%s) due: run with --delete\n", rep.Waiting, mb(rep.WaitingBytes),
			rep.Due, mb(rep.DueBytes))
	default:
		fmt.Printf("%d unused files (%s) can be deleted from %s\n", rep.Waiting, mb(rep.WaitingBytes),
			rep.NextCleanup.Local().Format("2006-01-02 15:04"))
	}
	return nil
}

func cmdGC() error {
	r, err := openRepo()
	if err != nil {
		return err
	}
	pruned, err := r.PruneObjects()
	if err != nil {
		return err
	}
	freed, err := r.GC()
	if err != nil {
		return err
	}
	fmt.Printf("%.1f MB kept in the team's storage only, %.1f MB of leftovers removed\n",
		float64(pruned)/(1<<20), float64(freed)/(1<<20))
	return nil
}
