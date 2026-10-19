// Package cli is R3V's command line tool, as a library: cmd/r3v runs it,
// and so can a build with extensions (see github.com/nonlabhq/r3v/ext).
package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nonlabhq/r3v/docs"
	"github.com/nonlabhq/r3v/internal/als"
	"github.com/nonlabhq/r3v/internal/diff"
	"github.com/nonlabhq/r3v/internal/merge"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/version"
)

const usage = `usage: r3v <command> [args]

AI agents: run "r3v help agents" first (how to work with R3V as an agent{{agent-setup}}),
then make sure the project is on the person's team: "r3v status --json" gives "connected".

everyday (run inside a project folder):
  commit -m TITLE [-m DETAILS] [PATHS]   commit your changes as a version and share it with the team
                                         (PATHS: only the changes in those files and folders)
  update [--preview]                     get the team's latest versions of your branch (or just look);
                                         another branch's come in with merge
  status                                 what changed since your last version
  watch                                  keep running: tell you about new versions (never changes
                                         your files)
  log [BRANCH] [-n N] [--first-parent]   list versions (of your branch, or another branch's);
                                         --first-parent: only the main line, merges with how
                                         many versions each brought in

setup (the first time, in this order):
  login [--service URL]                  sign in to R3V Cloud in the browser (the person does): the
                                         teams they are in appear in r3v teams
  teams                                  teams this computer is connected to
  init [--author NAME] [--preset code|none]
                                         start tracking this project (--preset: a folder R3V
                                         doesn't recognise, as code and documents, or files as they are)
{{team-setup}}  remote <connection-code>               or connect this project to a self-hosted team's storage
                                         (S3-compatible bucket) with its connection code: the person
                                         enters it (it holds the storage keys)
  clone <code> <project> [folder]        download a team's project
{{team-clone}}{{agent-setup-line}}  logout [--service URL]                 sign this computer out of R3V Cloud
  version                                show the R3V version
  connection-code --endpoint URL --bucket NAME --access-key K --secret-key S [--prefix P] [--name TEAM]
                                         create a code for team storage

branches (advanced):
  branch                                 list branches
  branch new NAME [--agent]              start a branch from your current version (--agent: an agent's)
  branch archive NAME                    put a branch away (its versions stay)
  branch archived                        list archived branches
  branch unarchive NAME                  bring an archived branch back where it was
  branch delete NAME                     delete an archived branch for good
  branch log [NAME]                      who moved the team's branches, when, from which version
  switch NAME                            work on another branch
  merge NAME [--preview] [-m MESSAGE]    merge another branch into yours (or just look); without
                                         -m, the merge says what came in (the versions' titles)

advanced:
  commit --local -m MESSAGE              commit on this computer only (your next commit shares it)
  goto <id|HEAD~N|latest> [--force]      go to a version (files and samples); latest goes back
  discard [<path>...] [--preview]        throw your changes away: the files as in the version
                                         you're on; with paths, only those files and folders
                                         (--preview: list them, change nothing; ask the user
                                         first)
  parked [list]                          changes kept while you're on another branch or version
  parked bring|discard BRANCH[@VERSION]  bring parked changes here, or drop them
  export <id> <folder>                   write a version as a separate project folder
  rules [check]                          the project's rules (.r3v.yaml) and whether they work
  rules explain <file>...                why a file is tracked or ignored
  rules preset <folder> <preset|none>    a folder's project type (presets: in .r3v.yaml)
  rules ignore <pattern>                 leave files out: adds the rule ignore: <pattern> to
                                         .r3v.yaml (as the app's "Leave it out")
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
  diff <a.als> <b.als>                   two Live Sets on disk, track by track
  info <set.als>                         a Live Set: tracks, devices, clips, plugins, samples
  merge-sets <base> <ours> <theirs> -o <out>  track-level 3-way merge of Live Sets
        [--strategy fail|ours|theirs|both]

for programs and AI agents:
  --json                                 status, log, save, update, merge, backup, verify, version:
                                         one JSON object on stdout; errors with fixed codes
  --wait SECONDS                         how long a command waits for the app or another command
                                         working on the project (default 10); reading commands
                                         (status, log...) read without waiting
  R3V_CONFIG_DIR=<folder>                keep R3V's own settings (teams joined, their keys) there,
                                         not in %APPDATA%\R3V: for a sandbox that can't write it
  help agents [--snippet]                how AI agents use R3V (or lines for a project's
                                         AGENTS.md)
  help rules                             the project's rules (.r3v.yaml): what is tracked,
                                         presets, patterns
  <command> --help                       a command's lines from this list
`

// setCommandsAt is where the Live Sets' own commands are listed: where
// diff, info and merge-files take any kind of file (Nightly), they aren't
// (the old names still work).
const setCommandsAt = "\nset commands:\n"

// fullUsage is the usage with this build's own sections (extraUsage) among
// the others, before the Live Sets' and the agents' ones, not after them.
func fullUsage() string {
	u := setupUsage().Replace(usage)
	i := strings.Index(u, setCommandsAt)
	if i < 0 {
		return u + extraUsage
	}
	if fileCommands {
		end := strings.Index(u[i+len(setCommandsAt):], "\n\n") + i + len(setCommandsAt) + 1
		return u[:i] + extraUsage + u[end:]
	}
	return u[:i] + extraUsage + u[i:]
}

// setupUsage fills in the usage's first lines and its setup section for
// this build: joining a team by name and setting an agent up are on the
// Nightly channel while they're new, and an agent reads the setup
// section in the order a newcomer needs it.
func setupUsage() *strings.Replacer {
	team := "                                         (to put it on a team in r3v teams: Add project in the R3V app)\n"
	if teamByName {
		team = "  init --team NAME [--preset code|none]  make the folder a project of a team this computer has joined\n" +
			"                                         (r3v teams lists them; no connection code needed)\n" +
			"  remote --team NAME                     connect this project to that team\n"
	}
	clone := ""
	if teamByName {
		clone = "  clone --team NAME <project> [folder]   download a project of a team this computer has joined\n"
	}
	agent, agentLine := ", and R3V_ACTOR", ""
	if _, ok := extraCommands["agent"]; ok {
		agent = ", the one-time \"r3v agent setup\""
		agentLine = "  agent setup [--tool claude-code]       an AI agent, once in its project folder: R3V_ACTOR (its versions\n" +
			"                                         say an agent made them), its hooks, the agents' guide\n"
	}
	return strings.NewReplacer("{{agent-setup}}", agent, "{{team-setup}}", team, "{{team-clone}}", clone,
		"{{agent-setup-line}}", agentLine)
}

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
	readBusy, recovered, unfinished = nil, nil, nil
	var waitErr error
	args, lockWait, waitGiven, waitErr = stripWait(args)
	if waitErr != nil {
		return fail("", waitErr)
	}
	if len(args) < 1 {
		if jsonMode {
			return fail("", usageError("usage: r3v <command> [args] (r3v help)"))
		}
		fmt.Fprint(os.Stderr, fullUsage())
		return exitUsage
	}
	command, rest := args[0], args[1:]
	if jsonMode && !jsonCommands[command] {
		return fail(command, usageError("--json is not supported by `r3v %s`", command))
	}
	if wantsHelp(rest) && command != "help" {
		if text, ok := helpFor(command, rest); ok {
			fmt.Print(text)
			return 0
		}
	}
	project.LockProgram, project.LockCommand = "cli", command
	defer releaseHeld()
	var err error
	code := 0
	switch command {
	case "info":
		err = cmdFileInfo(rest)
	case "diff":
		err = cmdFileDiff(rest)
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
	case "snapshot": // (commit --local, as it was called before 0.1.33)
		err = cmdCommit(append([]string{"--local"}, rest...))
	case "log":
		err = cmdLog(rest)
	case "goto", "checkout": // (checkout: its name before 0.1.34)
		err = cmdGoto(rest)
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
	case "rules", "profile": // (profile: its name before 0.1.40)
		err = cmdRules(rest)
	case "discard":
		err = cmdDiscard(rest)
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
	case "commit", "save": // (save: its name before 0.1.33)
		err = cmdCommit(rest)
	case "update":
		err = cmdUpdate(rest)
	case "-h", "--help", "help":
		err = cmdHelp(rest)
	case "version", "--version", "-v":
		result("version", map[string]string{"name": version.Name(), "version": version.Full(), "channel": version.Channel,
			"display": version.Display()}, func() { fmt.Println("r3v " + version.Display()) })
	case "path":
		err = cmdPath(rest)
	case "hook": // agent tools' hooks (agentlocks.go); not in the usage
		err = cmdHook(rest)
	default:
		if f, ok := extraCommands[command]; ok {
			err = f(rest)
			break
		}
		if jsonMode {
			return fail(command, usageError("unknown command %q", command))
		}
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", command, fullUsage())
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
		fmt.Print(fullUsage())
	case args[0] == "agents" && len(args) == 1:
		fmt.Print(docs.Agents)
	case args[0] == "agents" && len(args) == 2 && args[1] == "--snippet":
		fmt.Print(docs.AgentsSnippet)
	case args[0] == "rules" && len(args) == 1:
		fmt.Print(docs.Rules)
	case len(args) == 1:
		text, _ := helpFor(args[0], nil)
		fmt.Print(text)
	default:
		return usageError("usage: r3v help [agents [--snippet] | rules | <command>]")
	}
	return nil
}
