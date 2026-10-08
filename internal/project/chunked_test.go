package project

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/blob"
	"github.com/nonlabhq/r3v/internal/chunk"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

func randomBytes(seed int64, n int) []byte {
	b := make([]byte, n)
	rand.New(rand.NewSource(seed)).Read(b)
	return b
}

func objectAt(fake *s3test.Server, h string) ([]byte, bool) {
	return fake.Object("team", "r3v/objects/"+h[:2]+"/"+h[2:])
}

// A big file goes up as pieces; after an edit only the changed pieces go up,
// and a teammate downloads only those.
func TestChunkedStorage(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a, _ := Init(newProject(t), "yi")
	if err := a.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	level := randomBytes(1, 24<<20)
	small := randomBytes(2, 1<<20)
	write(t, a.Root, "Maps/Level.umap", string(level))
	write(t, a.Root, "Maps/Small.uasset", string(small))
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatalf("save: %v", err)
	}
	h := chunk.HashOf(level)
	if obj, ok := objectAt(fake, h); !ok || !blob.IsChunkList(obj) {
		t.Fatal("the level should be stored as a chunk list")
	}
	if keys := fake.Keys("team", "r3v/chunked/"); len(keys) != 1 || !strings.HasSuffix(keys[0], h) {
		t.Fatalf("markers: %v", keys)
	}
	if obj, _ := objectAt(fake, chunk.HashOf(small)); !bytes.Equal(obj, small) {
		t.Error("small files stay whole")
	}

	b, _, err := Clone(code, "Song", filepath.Join(t.TempDir(), "B", "Song Project"), "alex")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.Root, "Maps", "Level.umap")); !bytes.Equal(got, level) {
		t.Fatal("level differs after clone")
	}

	// An actor added in the middle of the level.
	at := 11 << 20
	edited := append(append(append([]byte{}, level[:at]...), []byte("a new actor")...), level[at:]...)
	write(t, a.Root, "Maps/Level.umap", string(edited))
	put := fake.PutBytes
	if _, _, err := a.Save("an actor", Strategy("fail")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if sent := fake.PutBytes - put; sent > 2*chunk.Max {
		t.Errorf("the edit sent %d bytes", sent)
	}
	get := fake.GetBytes
	if _, err := b.Update(Strategy("fail")); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.Root, "Maps", "Level.umap")); !bytes.Equal(got, edited) {
		t.Fatal("level differs after update")
	}
	if recv := fake.GetBytes - get; recv > 2*chunk.Max {
		t.Errorf("the update downloaded %d bytes", recv)
	}

	// A piece lost from storage: verify tells, and puts it back.
	l := a.loadChunkList(chunk.HashOf(edited))
	if l == nil {
		t.Fatal("no chunk list kept here")
	}
	lost := l.Pieces[5].Hash
	fake.Delete("team", "r3v/objects/"+lost[:2]+"/"+lost[2:])
	// (Both versions have that piece: the old one's level is not here, the
	// new one's mends both.)
	rep, err := a.Verify(false)
	if err != nil || len(rep.Problems) != 2 || !strings.Contains(rep.Problems[0].Detail, "lacks 1 of its") {
		t.Fatalf("verify: %+v %v", rep, err)
	}
	if rep, err = a.Verify(true); err != nil || len(rep.Problems) != 1 || !rep.Problems[0].Fixed {
		t.Fatalf("repair: %+v %v", rep, err)
	}
	if rep, err = a.Verify(false); err != nil || len(rep.Problems) != 0 {
		t.Fatalf("after repair: %+v %v", rep, err)
	}
	if _, ok := objectAt(fake, lost); !ok {
		t.Error("piece not uploaded again")
	}

	// A third computer, from scratch: all of it, right.
	c, _, err := Clone(code, "Song", filepath.Join(t.TempDir(), "C", "Song Project"), "sam")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(c.Root, "Maps", "Level.umap")); !bytes.Equal(got, edited) {
		t.Fatal("level differs on a new clone")
	}
}

// Storage failing now and then doesn't fail pieces going up or down.
func TestChunkedFlaky(t *testing.T) {
	defer func(w time.Duration) { remote.RetryWait = w }(remote.RetryWait)
	remote.RetryWait = time.Millisecond
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a, _ := Init(newProject(t), "yi")
	if err := a.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	level := append(randomBytes(3, 12<<20), bytes.Repeat([]byte("landscape "), 1<<20)...) // part compresses
	write(t, a.Root, "Maps/Level.umap", string(level))
	fake.Flaky = 5
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatalf("save: %v", err)
	}
	b, _, err := Clone(code, "Song", filepath.Join(t.TempDir(), "B", "Song Project"), "alex")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.Root, "Maps", "Level.umap")); !bytes.Equal(got, level) {
		t.Fatal("level differs")
	}
}

// A big file's pieces are listed while the save reads it (the upload
// doesn't read it once more for them); a list that doesn't match the file
// any more stops the upload rather than sending what isn't the file.
func TestChunkListFromSave(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	cfg := remote.Config{URL: "s3+" + fake.URL + "/team/r3v", AccessKey: "key", SecretKey: "secret"}
	a, _ := Init(newProject(t), "yi")
	if err := a.SetRemote(remote.EncodeConnectionCode(cfg)); err != nil {
		t.Fatal(err)
	}
	level := randomBytes(3, 20<<20)
	write(t, a.Root, "Maps/Level.umap", string(level))
	h := chunk.HashOf(level)
	want, _, err := chunk.ListFile(filepath.Join(a.Root, "Maps", "Level.umap"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _, err := a.hashFile(filepath.Join(a.Root, "Maps", "Level.umap")); err != nil || got != h {
		t.Fatalf("hash %s, %v", got, err)
	}
	if got := a.loadChunkList(h); got == nil || !bytes.Equal(got.Encode(), want.Encode()) {
		t.Fatal("hashing the level didn't keep its chunk list")
	}
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatalf("save: %v", err)
	}

	// The list kept, but the file changed since: the upload says so.
	c, err := remote.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "other")
	os.WriteFile(other, randomBytes(4, 20<<20), 0o644)
	err = a.uploadChunked(c.(remote.BodyStore), c, h, other, -1, a.newTransfer(StageUploading, 1, 0),
		func([]string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Errorf("uploading other bytes under the list: %v", err)
	}
}
