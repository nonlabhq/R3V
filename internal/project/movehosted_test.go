//go:build nightly

package project

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/chunk"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A project moves from a team on its own storage to a hosted team: it
// leaves the first keeping its whole history, joins the second, shares its
// versions; a teammate then gets every version, big files whole.
func TestMoveToHostedTeam(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	own := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a, _ := Init(newProject(t), "yi")
	if err := a.SetRemote(own); err != nil {
		t.Fatal(err)
	}
	big := randomBytes(7, chunk.MinFile+3<<20) // kept as pieces
	os.MkdirAll(filepath.Join(a.Root, "Samples"), 0o755)
	os.WriteFile(filepath.Join(a.Root, "Samples", "stem.wav"), big, 0o644)
	for i, text := range []string{"one\n", "two\n", "three\n"} {
		write(t, a.Root, "Notes/lyrics.txt", lyrics+text)
		if i == 1 {
			big[100] ^= 1 // the big file changes too
			os.WriteFile(filepath.Join(a.Root, "Samples", "stem.wav"), big, 0o644)
		}
		if _, _, err := a.Save("take "+text[:len(text)-1], Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := a.Log()

	// Leaving, every version kept here.
	if err := a.PrepareDetach(true); err != nil {
		t.Fatal(err)
	}
	a.Config.Remote = nil
	if err := a.SaveConfig(); err != nil {
		t.Fatal(err)
	}
	fake.Close() // the old storage is gone: nothing may come from it now

	// Joining the hosted team, and sharing the versions.
	hosted := newHostedFake(t)
	if err := a.SetRemote(hosted); err != nil {
		t.Fatal(err)
	}
	res, err := a.Share(Strategy("fail"))
	if err != nil || res.Action != "published" {
		t.Fatalf("share: %+v %v", res, err)
	}

	b, _, err := Clone(hosted, a.Config.Name, filepath.Join(t.TempDir(), "B", "Song"), "alex")
	if err != nil {
		t.Fatal(err)
	}
	after, err := b.Log()
	if err != nil || len(after) != len(before) {
		t.Fatalf("versions on the hosted team: %d, here before: %d (%v)", len(after), len(before), err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.Root, "Samples", "stem.wav")); !bytes.Equal(got, big) {
		t.Error("the big file isn't whole")
	}
	// The oldest version, its files whole too.
	if _, _, err := b.GoTo(after[len(after)-1].ID, false); err != nil {
		t.Fatalf("going to the first version: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.Root, "Notes", "lyrics.txt")); string(got) != lyrics+"one\n" {
		t.Errorf("the first version's lyrics: %q", got)
	}
}

