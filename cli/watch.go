package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/nonlabhq/r3v/internal/teamwatch"
)

func logf(format string, a ...any) {
	fmt.Printf("%s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...))
}

func printEvent(e teamwatch.Event) {
	switch e.Kind {
	case teamwatch.NewVersions:
		for _, m := range e.Versions {
			logf("%s saved a new version: %q", m.Author, m.Message)
		}
		if e.Waiting {
			logf("(these were already waiting)")
		}
		logf("run `r3v update --preview` to see the changes, `r3v update` to get them")
	case teamwatch.Offline:
		logf("cannot reach the team's storage (%s); will keep trying", e.Text)
	case teamwatch.Online:
		logf("team's storage reachable again")
	}
}

func cmdWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	interval := fs.Duration("interval", 0, "how often to check (default: 1m)")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	if _, err := r.Client(); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	fmt.Printf("watching %q (branch %s); Ctrl+C to stop\n", r.Config.Name, r.BranchName())
	fmt.Println("tells you when the team commits a new version; nothing here changes your files")
	if *interval == 0 {
		*interval = r.PollInterval()
	}
	teamwatch.Run(ctx, r.Root, *interval, printEvent)
	fmt.Println("stopped")
	return nil
}
