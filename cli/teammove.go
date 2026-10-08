package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/teammove"
)

// cmdMoveTeam moves the team of the project here to R3V Cloud (package
// teammove): the service copies the team's history with a read-only key,
// then this computer finishes; its projects follow.
func cmdMoveTeam(args []string) error {
	fs := flag.NewFlagSet("move-team", flag.ContinueOnError)
	to := fs.String("to", "", "the hosted team's address (r3v-cloud+https://…/v1/teams/<id>)")
	keyID := fs.String("key-id", "", "a read-only key's id for the team's bucket (or R3V_MOVE_KEY_ID)")
	secret := fs.String("secret", "", "its secret (or R3V_MOVE_SECRET)")
	region := fs.String("region", "", `the bucket's region ("" for R2)`)
	noFinish := fs.Bool("no-finish", false, "start the copy only (finish later with --finish)")
	finishOnly := fs.Bool("finish", false, "finish a move started before")
	var only stringList
	fs.Var(&only, "project", "a project to move (an id; again for more; none: all)")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	from, err := r.Team()
	if err != nil {
		return err
	}
	if !*finishOnly {
		if *keyID == "" {
			*keyID = os.Getenv("R3V_MOVE_KEY_ID")
		}
		if *to == "" || *keyID == "" {
			return errors.New("usage: r3v move-team --to ADDRESS --key-id ID --secret SECRET [--region R] [--project ID]... [--no-finish]")
		}
		if *secret == "" {
			*secret = os.Getenv("R3V_MOVE_SECRET") // (kept off the command line others can see)
		}
		dest, err := project.Connect(*to)
		if err != nil {
			return err
		}
		est, err := teammove.Plan(from.ID)
		if err != nil {
			return err
		}
		ids := []string(only)
		if len(ids) == 0 {
			for _, p := range est.Projects {
				ids = append(ids, p.ID)
			}
		}
		fmt.Printf("moving %d projects of %s to %s (%s there; the storage holds %s now)\n",
			len(ids), from.Name, dest.Name, size(est.Hosted), size(est.Storage))
		if err := teammove.Start(teammove.Cloud, from.ID, dest.ID, ids, *keyID, *secret, *region); err != nil {
			return err
		}
		if err := teammove.Copy(teammove.Cloud, from.ID); err != nil {
			teammove.Recorded(from.ID, err)
			return err
		}
		fmt.Println("R3V Cloud is copying (the team works on meanwhile)…")
		last := ""
		for {
			st, err := teammove.Status(teammove.Cloud, from.ID)
			if err != nil {
				return err
			}
			if st.Error != "" {
				return errors.New(st.Error)
			}
			line := fmt.Sprintf("  %d of %d files, %s of %s", st.ItemsDone, st.Items, size(st.BytesDone), size(st.Bytes))
			if line != last {
				fmt.Println(line)
				last = line
			}
			if st.Copied {
				break
			}
			time.Sleep(teammove.Every)
		}
		if *noFinish {
			fmt.Println("copied: finish with `r3v move-team --finish` (sharing to the old team stops for a few minutes)")
			return nil
		}
	}
	fmt.Println("finishing: the old team takes no shares meanwhile…")
	roots, err := teammove.Finish(teammove.Cloud, from.ID, func(s *cloud.MoveStatus) {})
	teammove.Recorded(from.ID, err)
	if err != nil {
		return err
	}
	fmt.Printf("moved: %d project folders here belong to the hosted team now; the old storage is kept as it was\n", len(roots))
	return nil
}

// stringList is a flag given several times.
type stringList []string

func (l *stringList) String() string     { return fmt.Sprint(*l) }
func (l *stringList) Set(v string) error { *l = append(*l, v); return nil }

func size(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d KB", n>>10)
}
