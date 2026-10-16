package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nonlabhq/r3v/internal/profile"
)

// cmdRules: `r3v rules [check]`, `r3v rules explain <file>...`, `r3v
// rules preset <folder> <preset|none>` and `r3v rules ignore <pattern>`
// (profile: their name before 0.1.40).
func cmdRules(args []string) error {
	if len(args) == 0 {
		args = []string{"check"}
	}
	if jsonMode && args[0] != "ignore" {
		return usageError("--json is supported by `r3v rules ignore` only")
	}
	open := openRepo
	if args[0] == "check" || args[0] == "explain" { // (only read)
		open = openRead
	}
	r, err := open()
	if err != nil {
		return err
	}
	rules, loadErr := r.Profile()
	switch args[0] {
	case "check":
		if rules.FromFile {
			fmt.Printf("rules: %s\n", profile.FileName)
		} else {
			fmt.Printf("rules: detected (no %s)\n", profile.FileName)
		}
		for _, a := range rules.Applied() {
			folder := a.Folder
			if folder == "" {
				folder = "(project folder)"
			}
			fmt.Printf("  %s: %s\n", folder, a.Preset)
		}
		for i, ru := range rules.Rules {
			if ru.Ignore != "" {
				fmt.Printf("  rule %d: ignore %q\n", i+1, ru.Ignore)
			} else {
				fmt.Printf("  rule %d: track %q\n", i+1, ru.Track)
			}
		}
		for i, h := range rules.Handlers {
			fmt.Printf("  handler %d: %s merge for %s\n", i+1, h.Merge, strings.Join(h.Files, ", "))
		}
		if err := r.CheckRules(); err != nil {
			return err
		}
		fmt.Println("ok")
		return loadErr
	case "explain":
		if len(args) < 2 {
			return errors.New("usage: r3v rules explain <file>...")
		}
		cwd, _ := os.Getwd()
		for _, arg := range args[1:] {
			abs := arg
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(cwd, arg)
			}
			rel, err := filepath.Rel(r.Root, abs)
			if err != nil || strings.HasPrefix(rel, "..") {
				return fmt.Errorf("%s is not in this project", arg)
			}
			rel = filepath.ToSlash(rel)
			fi, statErr := os.Stat(abs)
			isDir := statErr == nil && fi.IsDir()
			d := rules.Explain(rel, isDir)
			state := "tracked"
			if d.Ignored {
				state = "ignored"
			}
			fmt.Printf("%s: %s (%s)", rel, state, d.By)
			if !isDir {
				fmt.Printf(", kind %s", rules.Kind(rel))
				if h := rules.Handler(rel); h.Merge != "" || h.Samples != "" {
					fmt.Printf(", handled by %s", strings.Trim(h.Merge+" "+h.Samples, " "))
				}
			}
			fmt.Println()
		}
		return loadErr
	case "preset":
		if len(args) != 3 {
			return errors.New("usage: r3v rules preset <folder> <preset|none>")
		}
		folder := filepath.ToSlash(filepath.Clean(args[1]))
		if err := r.SetPreset(folder, args[2]); err != nil {
			return err
		}
		fmt.Printf("%s: %s now uses %s (commit it with the project)\n", profile.FileName, args[1], args[2])
		return nil
	case "ignore":
		if len(args) != 2 {
			return usageError("usage: r3v rules ignore <pattern>")
		}
		added, err := r.AddIgnoreRule(args[1])
		if err != nil {
			return err
		}
		out := rulesIgnoreJSON{Rule: "ignore: " + strconv.Quote(args[1]), Added: added}
		result("rules", out, func() {
			if added {
				fmt.Printf("%s: added the rule  - %s  (commit it with the project)\n", profile.FileName, out.Rule)
			} else {
				fmt.Printf("%s: the rule  - %s  is there already\n", profile.FileName, out.Rule)
			}
		})
		return nil
	}
	return fmt.Errorf("unknown rules command %q (check, explain, preset, ignore)", args[0])
}

// rulesIgnoreJSON is `r3v rules ignore`'s result: Rule, the line under
// rules: in .r3v.yaml; Added false when it was the last rule already.
type rulesIgnoreJSON struct {
	Rule  string `json:"rule"`
	Added bool   `json:"added"`
}
