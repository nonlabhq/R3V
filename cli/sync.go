package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/livecheck"
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
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	r, err := openRepo()
	if err != nil {
		return err
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
	fmt.Printf("connected to team %q (%s)\nnext: r3v save -m \"message\" to share this project\n", t.Name, t.Remote.Display())
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
		}
		fmt.Printf("%s%-24s %s  (%d project(s) on this computer)%s\n", mark, t.Name, t.Remote.Display(), n, note)
	}
	return nil
}

// syncHosted brings the hosted teams of every service this computer is
// signed in to up to date (teams joined or left since). Offline, or signed
// out, the list stays as it was.
func syncHosted() {
	services := map[string]bool{cloud.Service(): true}
	if store, err := teams.Load(); err == nil {
		for _, t := range store.Teams {
			if svc, ok := cloud.Hosted(t); ok {
				services[svc] = true
			}
		}
	}
	for svc := range services {
		if cloud.SignedIn(svc) {
			cloud.SyncTeams(svc)
		}
	}
}

func cmdClone(args []string) error {
	fs := flag.NewFlagSet("clone", flag.ContinueOnError)
	author := fs.String("author", "", "your name on saved versions (default: OS user)")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(pos) < 2 {
		return errors.New("usage: r3v clone <connection-code> <project name or id> [folder]")
	}
	dir := ""
	if len(pos) > 2 {
		dir = pos[2]
	}
	r, m, err := project.Clone(pos[0], pos[1], dir, *author)
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

var openSet = livecheck.OpenSet

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

// guardLive refuses to rewrite sets while one is open in Live, unless forced.
func guardLive(r *project.Repo, force bool) error {
	set := ""
	if !force {
		set = openSet(r.Root)
	}
	if set == "" {
		return nil
	}
	incoming, err := r.Incoming()
	if err != nil || !incoming {
		return err
	}
	return liveOpenError(set, "others saved new versions that must be merged into your files")
}

func printMerge(res *project.SyncResult) {
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
	From   string       `json:"from,omitempty"`
	To     string       `json:"to,omitempty"`
	Merged []string     `json:"merged"`   // what a merge took from each side
	Relink []string     `json:"relinked"` // sample paths rewritten for this computer
	// ReopenSets: sets changed under Live: they must be reopened there.
	ReopenSets bool `json:"reopen_sets"`
	// KeptWork: uncommitted changes were kept through an update (merged
	// with the team's versions, still uncommitted).
	KeptWork bool `json:"kept_work,omitempty"`
}

func syncOf(res *project.SyncResult) syncJSON {
	return syncJSON{Action: res.Action, From: res.From, To: res.To, Merged: nonNil(res.MergeLog),
		Relink: nonNil(res.Relinked), ReopenSets: len(res.MergeLog) > 0, KeptWork: res.KeptWork}
}

func cmdSave(args []string) error {
	fs := flag.NewFlagSet("save", flag.ContinueOnError)
	msg := fs.String("m", "", "what changed")
	strategy := strategyFlag(fs)
	force := fs.Bool("force", false, "merge even while Ableton Live is running")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	if *msg == "" {
		return usageError(`a message is required: r3v save -m "what changed"`)
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	if r.Config.Remote != nil {
		if err := guardLive(r, *force); err != nil {
			return err
		}
	}
	defer tidy(r)
	m, res, err := r.Save(*msg, project.Strategy(*strategy))
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
		if out.Action == "up-to-date" && m == nil {
			out.Action = "nothing-changed"
		}
	}
	if m != nil {
		v := versionOf(m, nil)
		out.Saved = &v
	}
	result("save", out, func() {
		switch out.Action {
		case "local":
			fmt.Printf("saved version %s locally (not connected to a team; see `r3v remote`)\n", short(m.ID))
			return
		case "nothing-changed":
			fmt.Println("nothing changed")
			return
		}
		if m != nil {
			fmt.Printf("saved version %s  %s\n", short(m.ID), m.Message)
		}
		printMerge(res)
		switch res.Action {
		case "published":
			fmt.Println("shared with the team")
			if len(res.MergeLog) > 0 {
				fmt.Println("others' changes were merged into your files: reopen the set in Live")
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
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	if *preview {
		p, err := r.PreviewUpdate()
		if err != nil {
			return err
		}
		result("update", previewOf(r, p), func() { printPreview(p, "the team") })
		return nil
	}
	if err := guardLive(r, *force); err != nil {
		return err
	}
	defer tidy(r)
	res, err := r.Update(project.Strategy(*strategy))
	if err != nil {
		return err
	}
	result("update", syncOf(res), func() {
		switch res.Action {
		case "up-to-date":
			fmt.Println("already up to date")
		case "ahead":
			fmt.Println("you have versions the team does not have yet: r3v save -m \"...\" to share them")
		case "fast-forward", "merged":
			fmt.Printf("updated to %s\n", short(res.To))
			printMerge(res)
			if res.KeptWork {
				fmt.Println("your uncommitted changes are kept (still uncommitted)")
			}
			if res.Action == "merged" {
				fmt.Println("your versions and the team's were merged; run `r3v save` to share the result")
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
