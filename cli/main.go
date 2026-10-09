// Package cli is R3V's command line tool, as a library: cmd/r3v runs it,
// and so can a build with extensions (see github.com/nonlabhq/r3v/ext).
package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/nonlabhq/r3v/docs"
	"github.com/nonlabhq/r3v/internal/als"
	"github.com/nonlabhq/r3v/internal/diff"
	"github.com/nonlabhq/r3v/internal/merge"
	"github.com/nonlabhq/r3v/internal/version"
)

const usage = `usage: r3v <command> [args]

everyday (run inside an Ableton project folder):
  save -m MESSAGE                        save a version and share it with the team
  update [--preview]                     get the team's latest versions (or just look)
  status                                 what changed since your last version
  watch                                  keep running: tell you about new versions (never changes
                                         your files)
  log                                    list versions

setup:
  init [--author NAME]                   start tracking this project
  remote <connection-code>               connect this project to team storage (S3-compatible bucket)
  clone <code> <project> [folder]       download a team's project
  teams                                  teams this computer is connected to
  connection-code --endpoint URL --bucket NAME --access-key K --secret-key S [--prefix P] [--name TEAM]
                                         create a code for team storage

branches (advanced):
  branch                                 list branches
  branch new NAME                        start a branch from your current version
  branch delete NAME                     delete a branch from the team (its versions stay)
  branch deleted                         list deleted branches
  branch restore NAME                    bring a deleted branch back where it was
  branch log [NAME]                      who moved the team's branches, when, from which version
  switch NAME                            work on another branch
  merge NAME [--preview]                 merge another branch into yours (or just look)

advanced:
  snapshot -m MESSAGE                    save a version locally only
  checkout <id|HEAD~N|latest> [--force]  go to a version (files and samples); latest goes back
  parked [list]                          changes kept while you're on another branch or version
  parked bring|discard BRANCH[@VERSION]  bring parked changes here, or drop them
  export <id> <folder>                   write a version as a separate project folder
  profile check                          the project's rules (.r3v.yaml) and whether they work
  profile explain <file>...              why a file is tracked or ignored
  profile preset <folder> <preset|none>  which preset applies to a folder (presets: in .r3v.yaml)
  gc                                     free space in .r3v (files the team's storage has,
                                         leftovers no version uses)
  verify [--repair]                      check the history: every version and stored file;
                                         --repair brings back what it can
  backup run [folder] [--team NAME]      back up the whole team (every project and version) into
                                         a folder; only adds. Default: the app's backup folder
  backup status [--team NAME]            this computer's backup, and who else backs up the team
  backup restore [folder] [--run TIME] [--preview]
                                         bring back from a backup what the team's storage lacks
                                         (deleted projects, lost files); never overwrites
  storage-cleanup [--delete]             files in the team's storage no version uses; --delete
                                         deletes those unused for a day (and a week old)

set commands:
  info <set.als>                         tracks, devices, clips, plugins, samples
  diff <a.als> <b.als>                   semantic diff
  merge-sets <base> <ours> <theirs> -o <out>  track-level 3-way merge of files
        [--strategy fail|ours|theirs|both]

  version                                show the R3V version

for programs and AI agents:
  --json                                 status, log, save, update, merge, backup, version: one
                                         JSON object on stdout; errors with fixed codes
  help agents [--snippet]                how AI agents use R3V (or lines for a project's
                                         AGENTS.md)
`

// extraCommands are commands only some builds have (the Nightly channel's),
// and extraUsage their lines in the usage.
var (
	extraCommands = map[string]func(args []string) error{}
	extraUsage    string
)

// Main runs the r3v command line tool with os.Args (extensions register
// what they add first; see github.com/nonlabhq/r3v/ext).
func Main() { os.Exit(Run(os.Args[1:])) }

// Run runs one r3v command and returns its exit status (see output.go).
func Run(args []string) int {
	args, jsonMode = stripJSON(args)
	if len(args) < 1 {
		if jsonMode {
			return fail("", usageError("usage: r3v <command> [args] (r3v help)"))
		}
		fmt.Fprint(os.Stderr, usage+extraUsage)
		return exitUsage
	}
	command, rest := args[0], args[1:]
	if jsonMode && !jsonCommands[command] {
		return fail(command, usageError("--json is not supported by `r3v %s`", command))
	}
	defer releaseHeld()
	var err error
	code := 0
	switch command {
	case "info":
		err = cmdInfo(rest)
	case "diff":
		err = cmdDiff(rest)
	case "merge-sets":
		code, err = cmdMerge(rest)
	case "merge":
		err = cmdMergeBranch(rest)
	case "branch":
		err = cmdBranch(rest)
	case "switch":
		err = cmdSwitch(rest)
	case "init":
		err = cmdInit(rest)
	case "status":
		err = cmdStatus(rest)
	case "snapshot":
		err = cmdSnapshot(rest)
	case "log":
		err = cmdLog(rest)
	case "checkout":
		err = cmdCheckout(rest)
	case "parked":
		err = cmdParked(rest)
	case "export":
		err = cmdExport(rest)
	case "gc":
		err = cmdGC()
	case "verify":
		code, err = cmdVerify(rest)
	case "backup":
		err = cmdBackup(rest)
	case "storage-cleanup":
		err = cmdStorageCleanup(rest)
	case "profile":
		err = cmdProfile(rest)
	case "remote":
		err = cmdRemote(rest)
	case "teams":
		err = cmdTeams(rest)
	case "connection-code":
		err = cmdConnectionCode(rest)
	case "clone":
		err = cmdClone(rest)
	case "watch":
		err = cmdWatch(rest)
	case "save":
		err = cmdSave(rest)
	case "update":
		err = cmdUpdate(rest)
	case "-h", "--help", "help":
		err = cmdHelp(rest)
	case "version", "--version", "-v":
		result("version", map[string]string{"name": version.Name(), "version": version.Full(), "channel": version.Channel,
			"display": version.Display()}, func() { fmt.Println("r3v " + version.Display()) })
	case "path":
		err = cmdPath(rest)
	default:
		if f, ok := extraCommands[command]; ok {
			err = f(rest)
			break
		}
		if jsonMode {
			return fail(command, usageError("unknown command %q", command))
		}
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", command, usage+extraUsage)
		return exitUsage
	}
	if err != nil {
		return fail(command, err)
	}
	return code
}

func cmdInfo(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: r3v info <set.als>")
	}
	s, err := als.Load(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("%s  [%s]  tempo %s  scenes %d  NextPointeeId %d\n",
		filepath.Base(s.Path), s.Creator(), s.Tempo(), s.SceneCount(), s.NextPointeeID())
	fmt.Println("\nTracks:")
	for _, t := range s.Tracks() {
		group := ""
		if t.GroupID() != "-1" {
			group = "  (in group " + t.GroupID() + ")"
		}
		fmt.Printf("  [%3s] %-11s %s%s\n", t.ID(), t.Kind(), t.Name(), group)
		for _, d := range t.DeviceNames() {
			fmt.Printf("          device  %s\n", d)
		}
		for _, c := range t.Clips() {
			fmt.Printf("          clip    %-9s \"%s\" %s %g-%g\n", c.Kind, c.Name, c.Location, c.Start, c.End)
		}
		var auto []string
		for label := range diff.Envelopes(t.Elem) {
			auto = append(auto, label)
		}
		sort.Strings(auto)
		for _, label := range auto {
			fmt.Printf("          auto    %s\n", label)
		}
	}
	fmt.Println("\nPlugins:")
	plugins := s.Plugins()
	if len(plugins) == 0 {
		plugins = []string{"(none)"}
	}
	for _, p := range plugins {
		fmt.Println("  " + p)
	}
	fmt.Println("\nSample references:")
	seen := map[string]bool{}
	for _, r := range s.SampleRefs() {
		key := r.RelativePath + "|" + r.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		name := r.RelativePath
		if name == "" {
			name = r.Path
		}
		pack := ""
		if r.Pack != "" {
			pack = " [" + r.Pack + "]"
		}
		fmt.Printf("  %s%s  (type %s, %s bytes)\n", name, pack, r.RelativePathType, r.FileSize)
	}
	return nil
}

func cmdDiff(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: r3v diff <a.als> <b.als>")
	}
	a, err := als.Load(args[0])
	if err != nil {
		return err
	}
	b, err := als.Load(args[1])
	if err != nil {
		return err
	}
	fmt.Println(diff.Diff(a, b).Render())
	return nil
}

func cmdMerge(args []string) (int, error) {
	fs := flag.NewFlagSet("merge", flag.ContinueOnError)
	out := fs.String("o", "", "output .als path")
	strategy := fs.String("strategy", "fail", "how to resolve tracks changed on both sides: fail|ours|theirs|both")
	// Allow flags after positional args.
	var pos []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return 2, err
		}
		args = fs.Args()
		if len(args) > 0 {
			pos = append(pos, args[0])
			args = args[1:]
		}
	}
	if len(pos) != 3 || *out == "" {
		return 2, fmt.Errorf("usage: r3v merge-sets <base> <ours> <theirs> -o <out> [--strategy ...]")
	}
	var sets [3]*als.LiveSet
	for i, p := range pos {
		s, err := als.Load(p)
		if err != nil {
			return 1, err
		}
		sets[i] = s
	}
	r, err := merge.Merge(sets[0], sets[1], sets[2], *strategy)
	if err != nil {
		return 2, err
	}
	fmt.Println(r.Report())
	if len(r.Conflicts) > 0 && *strategy == "fail" {
		fmt.Fprintln(os.Stderr, "\nmerge aborted: unresolved conflicts (use --strategy ours|theirs|both)")
		return 1, nil
	}
	if err := r.Merged.Save(*out); err != nil {
		return 1, err
	}
	fmt.Printf("\nwrote %s\n", *out)
	return 0, nil
}

// cmdHelp: r3v help [agents [--snippet]].
func cmdHelp(args []string) error {
	switch {
	case len(args) == 0:
		fmt.Print(usage + extraUsage)
	case args[0] == "agents" && len(args) == 1:
		fmt.Print(docs.Agents)
	case args[0] == "agents" && len(args) == 2 && args[1] == "--snippet":
		fmt.Print(docs.AgentsSnippet)
	default:
		return usageError("usage: r3v help [agents [--snippet]]")
	}
	return nil
}
