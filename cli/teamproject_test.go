package cli

import (
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// teamProject is a project committed and shared on a team's storage.
func teamProject(t *testing.T) string {
	t.Helper()
	fake := s3test.New("team")
	t.Cleanup(fake.Close)
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "src/main.ts": "export {};\n"})
	ra, err := project.Init(a, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if err := ra.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	if exit, rep := run(t, a, "commit", "-m", "first"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	return a
}
