package desktop

import (
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// teamSong is a team's project with one version shared.
func teamSong(t *testing.T, a *App) string {
	t.Helper()
	fake := s3test.New("one")
	t.Cleanup(fake.Close)
	t.Cleanup(waitTidy)
	one, err := a.CreateStorageTeam(remote.Storage{Endpoint: fake.URL, Bucket: "one", AccessKey: "k", SecretKey: "s"}, "One")
	if err != nil {
		t.Fatal(err)
	}
	root := newSong(t)
	if _, err := a.AddProjectToTeam(one.ID, root); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Save(root, "first", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	return root
}
