package project

import (
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// cancelTeam is a project shared with a team, with a big file (in pieces)
// to commit; cancelAt makes r.Cancel stop the next commit once a progress
// report says so.
func cancelTeam(t *testing.T) (r *Repo, fake *s3test.Server, big []byte) {
	t.Helper()
	fake = s3test.New("team")
	t.Cleanup(fake.Close)
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	r, _ = Init(newProject(t), "yi")
	if err := r.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Save("v1", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	big = make([]byte, 20<<20)
	rand.New(rand.NewSource(2)).Read(big)
	write(t, r.Root, "Video/take.mov", string(big))
	return r, fake, big
}

func cancelAt(r *Repo, when func(Progress) bool) {
	var stop atomic.Bool
	r.OnProgress = func(p Progress) {
		if when(p) {
			stop.Store(true)
		}
	}
	r.Cancel = func() error {
		if stop.Load() {
			return ErrCancelled
		}
		return nil
	}
}

func teamHead(t *testing.T, r *Repo) string {
	t.Helper()
	c, err := r.Client()
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.Branches(r.Config.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	return b[r.BranchName()]
}

// A commit cancelled while its files are copied into the history leaves
// nothing: no version, the changes as they were.
func TestCancelWhileStoring(t *testing.T) {
	r, _, _ := cancelTeam(t)
	head, shared := r.Head(), teamHead(t, r)
	cancelAt(r, func(p Progress) bool { return p.Stage == StageStoring })
	m, _, err := r.Save("oops", Strategy("fail"))
	if !errors.Is(err, ErrCancelled) || m != nil {
		t.Fatalf("got %v, %v", m, err)
	}
	if r.Head() != head || teamHead(t, r) != shared {
		t.Error("a version was made")
	}
	r.Cancel = nil
	if ch, _ := r.Status(); len(ch) != 1 || ch[0].Path != "Video/take.mov" {
		t.Errorf("changes: %+v", ch)
	}
}

// A commit cancelled while it uploads is taken back: the version is gone
// here and the team never saw it, the files are as they were, and the next
// commit sends only what didn't go up.
func TestCancelWhileUploading(t *testing.T) {
	r, fake, big := cancelTeam(t)
	head, shared := r.Head(), teamHead(t, r)
	cancelAt(r, func(p Progress) bool { return p.Stage == StageUploading && p.Bytes > 6<<20 })
	before := fake.PutBytes
	m, _, err := r.Save("oops", Strategy("fail"))
	if !errors.Is(err, ErrCancelled) || m != nil {
		t.Fatalf("got %v, %v", m, err)
	}
	if r.Head() != head || teamHead(t, r) != shared {
		t.Fatal("the version stayed")
	}
	r.Cancel = nil
	if ch, _ := r.Status(); len(ch) != 1 || ch[0].Path != "Video/take.mov" {
		t.Errorf("changes: %+v", ch)
	}
	if b, _ := os.ReadFile(filepath.Join(r.Root, "Video", "take.mov")); string(b) != string(big) {
		t.Error("the file changed")
	}
	first := fake.PutBytes - before
	if first <= 0 || first >= int64(len(big)) {
		t.Fatalf("sent %d of %d before stopping", first, len(big))
	}

	m, res, err := r.Save("the take", Strategy("fail"))
	if err != nil || m == nil || res.Action != "published" || teamHead(t, r) != m.ID {
		t.Fatalf("commit again: %v %+v %v", m, res, err)
	}
	if again := fake.PutBytes - before - first; again >= int64(len(big)) {
		t.Errorf("sent all %d MB again", again>>20)
	}
}

// Once the team has the version it's too late: cancelling changes nothing.
func TestCancelAfterShared(t *testing.T) {
	r, _, _ := cancelTeam(t)
	r.Cancel = func() error { return nil }
	m, res, err := r.Save("the take", Strategy("fail"))
	if err != nil || res.Action != "published" {
		t.Fatal(err)
	}
	r.Cancel = func() error { return ErrCancelled }
	if _, err := r.Share(Strategy("fail")); err != nil || r.Head() != m.ID || teamHead(t, r) != m.ID {
		t.Errorf("after: %v", err)
	}
}
