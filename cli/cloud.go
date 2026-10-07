package cli

import (
	"context"
	"flag"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/nonlabhq/r3v/internal/cloud"
)

// Signing in to R3V-Cloud, the hosted service (the Nightly channel adds
// these commands: cloud_nightly.go).

func cmdLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	service := fs.String("service", cloud.Service(), "the service's address")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	svc := strings.TrimRight(*service, "/")
	me, err := cloud.SignIn(context.Background(), svc, func(addr string) error {
		fmt.Printf("sign in in your browser; if it didn't open, open:\n  %s\n", addr)
		openBrowser(addr)
		return nil
	})
	if err != nil {
		return err
	}
	if _, err := cloud.SyncTeams(svc); err != nil {
		return err
	}
	fmt.Printf("signed in as %s\n", me.User.Email)
	if len(me.Teams) == 0 {
		fmt.Println("you're in no team yet: create one in the app, or open an invitation")
	}
	for _, t := range me.Teams {
		fmt.Printf("  %-24s %s\n", t.Name, t.Role)
	}
	return nil
}

func cmdLogout(args []string) error {
	fs := flag.NewFlagSet("logout", flag.ContinueOnError)
	service := fs.String("service", cloud.Service(), "the service's address")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	if err := cloud.SignOut(strings.TrimRight(*service, "/")); err != nil {
		return err
	}
	fmt.Println("signed out; your hosted teams stay listed until you sign in again")
	return nil
}

// openBrowser shows an address in the default browser (best effort: the
// address is printed too).
func openBrowser(addr string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", addr)
	case "darwin":
		cmd = exec.Command("open", addr)
	default:
		cmd = exec.Command("xdg-open", addr)
	}
	cmd.Start()
}
