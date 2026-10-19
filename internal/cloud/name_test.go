package cloud

import (
	"errors"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
)

// The person's display name: set on the service (PATCH /v1/me), given back
// by GET /v1/me and so to every team SyncTeams brings in; the service's
// own words when it refuses one; signed out, nothing sent.
func TestSetName(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	f := cloudtest.NewFake(t)
	if err := SetName(f.Service, "Robin Ito"); err != nil {
		t.Fatal(err)
	}
	if got := f.UserName("token"); got != "Robin Ito" {
		t.Errorf("the service has %q", got)
	}
	if f.Requests()["PATCH me"] != 1 {
		t.Errorf("requests %v", f.Requests())
	}
	me, err := SyncTeams(f.Service)
	if err != nil || me.User.Name != "Robin Ito" {
		t.Fatalf("me %+v %v", me, err)
	}
	if err := SetName(f.Service, strings.Repeat("x", 101)); err == nil || !strings.Contains(err.Error(), "1 to 100 characters") {
		t.Errorf("a long name: %v", err)
	}

	t.Setenv("R3V_CLOUD_TOKEN", "nobody")
	if err := SetName(f.Service, "Kai"); !errors.Is(err, remote.ErrSignedOut) {
		t.Errorf("signed out: %v", err)
	}
}
