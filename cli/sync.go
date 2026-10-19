package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/handlers"
	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// parseArgs parses flags that may appear before or after positional args.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	if jsonMode {
		fs.SetOutput(io.Discard) // the error says it
	}
	for {
		if err := fs.Parse(args); err != nil {
			return nil, &cliError{Code: "usage", Exit: exitUsage, Err: err}
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func cmdRemote(args []string) error {
	fs := flag.NewFlagSet("remote", flag.ContinueOnError)
	teamName := fs.String("team", "", "a team this computer has joined, by name (r3v teams lists them)")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	if *teamName != "" {
		t, err := joinedTeam(*teamName)
		if err != nil {
			return err
		}
		if err := joinTeam(r, t); err != nil {
			return err
		}
		fmt.Printf("connected to team %q\nnext: r3v commit -m \"message\" to share this project\n", t.Name)
		return nil
	}
	if len(pos) == 0 {
		t, err := r.Team()
		if err != nil {
			return err
		}
		fmt.Printf("team %q at %s\n", refreshTeamName(t.ID), t.Remote.Display())
		return nil
	}
	if err := r.SetRemote(pos[0]); err != nil {
		return err
	}
	t, _ := r.Team()
	fmt.Printf("connected to team %q (%s)\nnext: r3v commit -m \"message\" to share this project\n", t.Name, t.Remote.Display())
	return nil
}

// refreshTeamName picks up the name the team's storage gives now
// (unless the user renamed the team here) and returns the name to show.
func refreshTeamName(id string) string {
	store, err := teams.Load()
	if err != nil {
		return ""
	}
	t := store.Find(id)
	if t == nil {
		return ""
	}
	if b, err := t.Open(); err == nil {
		if info, err := b.Info(); err == nil && teams.SyncTeamName(id, info.Name) {
			return info.Name
		}
	}
	return t.Name
}

func cmdTeams(args []string) error {
	syncHosted()
	store, err := teams.Load()
	if err != nil {
		return err
	}
	if len(store.Teams) == 0 {
		fmt.Println("not connected to any team (use `r3v remote` in a project, or `r3v clone`)")
		return nil
	}
	for i := range store.Teams {
		store.Teams[i].Name = refreshTeamName(store.Teams[i].ID)
	}
	for _, t := range store.Teams {
		mark := "  "
		if t.ID == store.Current {
			mark = "* "
		}
		n := 0
		for k := range store.Projects {
			if strings.HasPrefix(k, t.ID+"/") {
				n++
			}
		}
		note := ""
		if svc, ok := cloud.Hosted(t); ok && !cloud.SignedIn(svc) {
			note = "  signed out: r3v login"
		} else if ok && t.NoAccess {
			note = "  no access: the account signed in isn't in this team (any more)"
		}
		fmt.Printf("%s%-24s %s  (%d project(s) on this computer)%s\n", mark, t.Name, t.Remote.Display(), n, note)
	}
	return nil
}

// syncHosted brings the hosted teams of every service this computer is
// signed in to up to date (teams joined or left since). Offline, or signed
// out, the list stays as it was.
func syncHosted() {
	if !remote.HostedTeams {
		return
	}
	services := map[string]bool{cloud.Service(): true}
	if store, err := teams.Load(); err == nil {
		for _, t := range store.Teams {
			if svc, ok := cloud.Hosted(t); ok {
				services[svc] = true
			}
		}
	}
	for svc := range services {
		if !cloud.SignedIn(svc) {
			continue
		}
		cloud.SyncTeams(svc) // (teams the account isn't in are marked: see cmdTeams)
	}
}

func cmdClone(args []string) error {
	fs := flag.NewFlagSet("clone", flag.ContinueOnError)
	author := fs.String("author", "", "your name on saved versions (default: OS user)")
	teamName := fs.String("team", "", "a team this computer has joined, by name, instead of a connection code")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	var team *teams.Team
	if *teamName != "" {
		if team, err = joinedTeam(*teamName); err != nil {
			return err
		}
		pos = append([]string{""}, pos...) // (no code)
	}
	if len(pos) < 2 {
		return errors.New("usage: r3v clone <connection-code> <project name or id> [folder]")
	}
	dir := ""
	if len(pos) > 2 {
		dir = pos[2]
	}
	var r *project.Repo
	var m *project.Manifest
	if team != nil {
		r, m, err = project.CloneFromTeam(team, pos[1], dir, *author, nil)
	} else {
		r, m, err = project.Clone(pos[0], pos[1], dir, *author)
	}
	if err != nil {
		return err
	}
	if release, err := r.Lock(0); err == nil {
		tidy(r)
		ensureRules(r)
		release()
	}
	fmt.Printf("downloaded %q into %s\n", r.Config.Name, r.Root)
	if m != nil {
		fmt.Printf("  latest version %s  %s (%s)\n", short(m.ID), m.Message, m.Author)
	}
	return nil
}

func strategyFlag(fs *flag.FlagSet) *string {
	return fs.String("strategy", "fail",
		"when you and others changed the same track or file: fail (ask), ours (keep yours), theirs (keep theirs), both (keep both)")
}

// openTool runs the project's running-tool checks (its presets' running:
// Live, the Unity, Unreal and Godot editors, design apps): which one has
// what of the project open ("" when none).
var openTool = func(r *project.Repo) (check, what string) {
	rules, _ := r.Profile()
	return handlers.OpenIn(r.Root, rules.Running())
}

// toolNames: the running-tool checks' programs, for people.
var toolNames = map[string]string{"ableton-live": "Ableton Live", "unity-editor": "the Unity Editor",
	"unreal-editor": "the Unreal Editor", "godot-editor": "the Godot editor", "design-apps": "a design app"}

// openError explains that what a tool has open (check: which tool) must be
// closed first: a Live Set as before (set_open_in_live), any other tool's
// files_open_in_tool.
func openError(check, what, why string) error {
	if check == "ableton-live" {
		return liveOpenError(what, why)
	}
	tool := toolNames[check]
	if tool == "" {
		tool = check
	}
	msg := fmt.Sprintf("%s has this project open", tool)
	if what != "?" && what != "" {
		msg = fmt.Sprintf("%s has %q open", tool, what)
	}
	if why != "" {
		msg = why + ", but " + msg
	}
	return &cliError{Code: "files_open_in_tool", Exit: exitLiveOpen,
		Err:  errors.New(msg + ".\nSave and close it there, then run this again (or use --force if it is not open)"),
		Hint: "ask the user to save and close the project in " + tool + "; use --force only if it is not open"}
}

// liveOpenError explains that set ("?" if unknown) must be closed in Live.
func liveOpenError(set, why string) error {
	what := "a set of this project is open in Ableton Live"
	if set != "?" {
		what = fmt.Sprintf("%q is open in Ableton Live", set)
	}
	if why != "" {
		what = why + ", but " + what
	}
	ce := &cliError{Code: "set_open_in_live", Exit: exitLiveOpen,
		Err:  errors.New(what + ".\nSave and close it in Live, then run this again (or use --force if it is not open)"),
		Hint: "ask the user to save and close the set in Live; use --force only if it is not open"}
	if set != "?" {
		ce.Set = set
	}
	return ce
}

// guardLive refuses to rewrite the project's files while a tool has them
// open (a set in Live, a project in an editor), unless forced.
func guardLive(r *project.Repo, force bool) error {
	if force {
		return nil
	}
	check, what := openTool(r)
	if check == "" {
		return nil
	}
	incoming, err := r.Incoming()
	if err != nil || !incoming {
		return err
	}
	return openError(check, what, "others saved new versions that must be merged into your files")
}

func printMerge(res *project.SyncResult) {
	if res.Restored {
		fmt.Println("the project had been deleted from the team: sharing brought it back, with its history")
	}
	for _, l := range res.MergeLog {
		fmt.Println("  merged " + l)
	}
	for _, n := range res.Relinked {
		fmt.Println("  relinked " + n)
	}
}

// syncJSON is what save, update and merge did.
type syncJSON struct {
	// Action: published (shared), local (saved, no team), nothing-changed,
	// up-to-date, ahead (you have versions the team lacks), fast-forward,
	// merged.
	Action string       `json:"action"`
	Saved  *versionJSON `json:"saved"` // the version saved, if any
	// Base: in a workspace, the branch it was made from when that has
	// versions this branch hasn't (an update doesn't bring them in).
	Base   *baseJSON `json:"base,omitempty"`
	From   string    `json:"from,omitempty"`
	To     string    `json:"to,omitempty"`
	Merged []string  `json:"merged"`   // what a merge took from each side
	Relink []string  `json:"relinked"` // sample paths rewritten for this computer
	// ReopenSets: sets changed under Live: they must be reopened there.
	ReopenSets bool `json:"reopen_sets"`
	// KeptWork: uncommitted changes were kept through an update (merged
	// with the team's versions, still uncommitted).
	KeptWork bool `json:"kept_work,omitempty"`
	// SameFilesAs: the merge version has exactly this version's files (the
	// branch was behind it); its results carry over.
	SameFilesAs string `json:"same_files_as,omitempty"`
	// UnsharedElsewhere (update, merge): the project's other copies here
	// with versions on that branch not shared, which this couldn't see;
	// each also a warning "unshared_versions". (commit has warnings of
	// its own.)
	UnsharedElsewhere []unsharedJSON `json:"unshared_elsewhere,omitempty"`
	Warnings          []warningJSON  `json:"warnings,omitempty"`
	// Restored: the team had deleted the project (it keeps one a while);
	// sharing brought it back.
	Restored bool `json:"restored,omitempty"`
}

// commitJSON is r3v commit's result: Warnings as status has them, of what
// the version took. Shared: the project's versions are in the team's
// storage (false: on this computer only, with --local or in no team; Next
// then says how to connect it). StepEnded: the task the agent began with
// step start and left open, ended by the commit (as step end's ended).
type commitJSON struct {
	syncJSON
	Warnings  []warningJSON      `json:"warnings"`
	Shared    bool               `json:"shared"`
	Next      []nextJSON         `json:"next,omitempty"`
	StepEnded *project.StepEnded `json:"step_ended,omitempty"`
}

// reopenSets: a merge changed Live Sets (between before, the version the
// files were on, and after): only then must they be reopened in Live.
// Other kinds of project have none.
func reopenSets(r *project.Repo, res *project.SyncResult, before, after string) bool {
	return len(res.MergeLog) > 0 && r.SetsChanged(before, after)
}

func syncOf(res *project.SyncResult) syncJSON {
	return syncJSON{Action: res.Action, From: res.From, To: res.To, Merged: nonNil(res.MergeLog),
		Relink: nonNil(res.Relinked), KeptWork: res.KeptWork,
		SameFilesAs: res.SameFilesAs, Restored: res.Restored}
}

// cmdCommit commits the changes as a version and shares it (merging the
// team's versions first); --local, on this computer only. Paths after the
// flags: only the changes in those files and folders, the others left
// uncommitted. ("save" is its name before 0.1.33, still taken.)
func cmdCommit(args []string) error {
	fs := flag.NewFlagSet("commit", flag.ContinueOnError)
	var parts stringList
	fs.Var(&parts, "m", "what changed: the title; again for details (a blank line between, as git)")
	strategy := strategyFlag(fs)
	force := fs.Bool("force", false, "merge even while Ableton Live is running")
	local := fs.Bool("local", false, "on this computer only: shared with the next commit")
	decided := decisionFlags(fs, strategy)
	paths, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	msg := manifest.Message(parts...)
	if manifest.Title(msg) == "" {
		return usageError(`a message is required: r3v commit -m "what changed" [-m "details"]`)
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	opts, err := decided.options(r)
	if err != nil {
		return err
	}
	if len(paths) > 0 {
		if r.Only, err = changesIn(r, paths); err != nil {
			return err
		}
	}
	if r.Config.Remote != nil && !*local {
		if err := guardLive(r, *force); err != nil {
			return err
		}
	}
	// The agent's task left open ends first, so the version lists its
	// steps (the commit failing after, it stays ended: the next lists them).
	var stepEnded *project.StepEnded
	if !stepsHooked(r) {
		if stepEnded, err = r.EndTaskAtCommit(manifest.Title(msg)); err != nil {
			return err
		}
	}
	defer tidy(r)
	var m *project.Manifest
	var res *project.SyncResult
	reopen := false
	if *local { // (a version here only: the next commit shares it)
		m, err = r.Snapshot(msg)
		if errors.Is(err, project.ErrNothingToSnapshot) {
			m, err = nil, nil
		}
		if err == nil {
			err = project.ErrNoRemote // (said as for a project in no team)
		}
	} else {
		before := r.Head()
		if m, res, err = r.Save(msg, opts); err == nil {
			reopen = reopenSets(r, res, before, r.Head())
		}
	}
	var out syncJSON
	switch {
	case errors.Is(err, project.ErrNoRemote):
		out = syncJSON{Action: "nothing-changed", Merged: []string{}, Relink: []string{}}
		if m != nil {
			out.Action = "local"
		}
	case err != nil:
		return err
	default:
		out = syncOf(res)
		out.ReopenSets = reopen
		if out.Action == "up-to-date" && m == nil {
			out.Action = "nothing-changed"
		}
	}
	if m != nil {
		v := versionOf(m, nil)
		out.Saved = &v
	}
	// (after the fact: a commit isn't refused for it; status warned before)
	warned := commitJSON{syncJSON: out, Warnings: []warningJSON{}, Shared: connected(r) && out.Action != "local", StepEnded: stepEnded}
	if m != nil {
		warned.Warnings = builtWarnings(r.BuiltFoldersIn(m))
	}
	if !connected(r) {
		warned.Warnings = append(warned.Warnings, notConnected())
		warned.Next = append(connectSteps(), nextJSON{`r3v commit -m "..."`,
			"once connected, the next commit shares the versions committed here with the team"})
	}
	result("commit", warned, func() {
		defer printWarnings(warned.Warnings, true)
		if stepEnded != nil {
			fmt.Printf("ended the open task (turn %d)%s\n", stepEnded.Turn, endedLine(stepEnded))
		}
		switch out.Action {
		case "local":
			if connected(r) {
				fmt.Printf("committed version %s on this computer: your next commit shares it "+
					"(until then the team's branch, and the project's other copies, don't have it)\n", short(m.ID))
				return
			}
			fmt.Printf("committed version %s on this computer only: it is not shared, this project is not connected to a team\n", short(m.ID))
			printNext("to share it:", warned.Next)
			return
		case "nothing-changed":
			fmt.Println("nothing changed")
			if !connected(r) {
				fmt.Println("(this project is not connected to a team: its versions are on this computer only)")
				printNext("to share them:", warned.Next)
			}
			return
		}
		if m != nil {
			fmt.Printf("committed version %s  %s\n", short(m.ID), manifest.Title(m.Message))
		}
		printMerge(res)
		switch res.Action {
		case "published":
			fmt.Println("shared with the team")
			if out.ReopenSets {
				fmt.Println("others' changes were merged into your files: reopen the set in Live")
			} else if len(res.MergeLog) > 0 {
				fmt.Println("others' changes were merged into your files")
			}
		case "fast-forward":
			fmt.Println("you had nothing new; updated to the team's latest version")
		}
	})
	return nil
}

func cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	preview := fs.Bool("preview", false, "show what the team changed, change nothing")
	strategy := strategyFlag(fs)
	force := fs.Bool("force", false, "update even while Ableton Live is running")
	decided := decisionFlags(fs, strategy)
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	opts, err := decided.options(r)
	if err != nil {
		return err
	}
	if *preview {
		p, err := r.PreviewUpdate()
		if err != nil {
			return err
		}
		out := previewOf(r, p)
		out.UnsharedElsewhere, out.Warnings = unsharedElsewhere(r, r.BranchName())
		result("update", out, func() { printPreview(p, "the team"); printUnshared(out.UnsharedElsewhere) })
		return nil
	}
	if err := guardLive(r, *force); err != nil {
		return err
	}
	defer tidy(r)
	before := r.Head()
	res, err := r.Update(opts)
	if err != nil {
		return err
	}
	out := syncOf(res)
	out.ReopenSets = reopenSets(r, res, before, r.Head())
	out.Base = baseNews(r)
	out.UnsharedElsewhere, out.Warnings = unsharedElsewhere(r, r.BranchName())
	result("update", out, func() {
		defer printUnshared(out.UnsharedElsewhere)
		defer printBaseNews(out.Base)
		switch res.Action {
		case "up-to-date":
			fmt.Println("already up to date")
		case "ahead":
			fmt.Println("you have versions the team does not have yet: r3v commit -m \"...\" to share them")
		case "fast-forward", "merged":
			fmt.Printf("updated to %s\n", short(res.To))
			printMerge(res)
			if res.KeptWork {
				fmt.Println("your uncommitted changes are kept (still uncommitted)")
			}
			if res.Action == "merged" {
				fmt.Println("your versions and the team's were merged; run `r3v commit` to share the result")
			}
		}
	})
	return nil
}

func cmdConnectionCode(args []string) error {
	fs := flag.NewFlagSet("connection-code", flag.ContinueOnError)
	endpoint := fs.String("endpoint", "", "S3 endpoint, e.g. https://<account>.r2.cloudflarestorage.com")
	bucket := fs.String("bucket", "", "bucket name")
	prefix := fs.String("prefix", "r3v", "folder inside the bucket")
	region := fs.String("region", "auto", "region")
	access := fs.String("access-key", "", "access key id (one per team member)")
	secret := fs.String("secret-key", "", "secret access key")
	name := fs.String("name", "", "team name shown to members (stored in the bucket)")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	cfg, err := remote.Storage{Endpoint: *endpoint, Bucket: *bucket, Folder: *prefix, Region: *region,
		AccessKey: *access, SecretKey: *secret}.Config()
	if err != nil {
		return err
	}
	if err := remote.Check(cfg); err != nil {
		return err
	}
	b, err := remote.Open(cfg)
	if err != nil {
		return err
	}
	if *name != "" {
		if err := remote.Rename(b, *name); err != nil {
			return fmt.Errorf("could not save the team name: %w", err)
		}
	}
	info, _ := b.Info()
	if info.Name == "" {
		fmt.Println("tip: add --name \"Team name\" once so members see a name instead of the address")
	}
	fmt.Println("storage OK. Connection code (contains the key: share it privately):")
	fmt.Println()
	fmt.Println(remote.EncodeConnectionCode(cfg))
	return nil
}

// changesIn are the changed files (as the project names them) in the files
// and folders given (relative to the current folder): a rename counts in
// either place, both its paths then.
func changesIn(r *project.Repo, args []string) ([]string, error) {
	changes, err := r.Status()
	if err != nil {
		return nil, err
	}
	if changes, err = changesUnder(r, args, changes); err != nil {
		return nil, err
	}
	var out []string
	for _, c := range changes {
		out = append(out, c.Path)
		if c.From != "" {
			out = append(out, c.From)
		}
	}
	if len(out) == 0 {
		return nil, &cliError{Code: "nothing_to_commit", Exit: exitUsage, Err: fmt.Errorf("nothing changed in %s", strings.Join(args, ", "))}
	}
	return out, nil
}

// changesUnder is the changes in the files or folders args name (paths from
// here, or absolute; a moved file by either of its places), as commit and
// discard take them.
func changesUnder(r *project.Repo, args []string, changes []project.Change) ([]project.Change, error) {
	cwd, _ := os.Getwd()
	var under []string
	for _, a := range args {
		abs := a
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(cwd, a)
		}
		rel, err := filepath.Rel(r.Root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, usageError("%s is not in this project", a)
		}
		under = append(under, filepath.ToSlash(rel))
	}
	in := func(p string) bool {
		for _, u := range under {
			if u == "." || strings.EqualFold(p, u) || strings.HasPrefix(strings.ToLower(p), strings.ToLower(u)+"/") {
				return true
			}
		}
		return false
	}
	var out []project.Change
	for _, c := range changes {
		if in(c.Path) || (c.From != "" && in(c.From)) {
			out = append(out, c)
		}
	}
	return out, nil
}
