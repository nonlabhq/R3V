package project

import (
	"bytes"
	"net/url"
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

// perProjectBucket is storage that keeps each project's contents apart, as
// the hosted service does (remote.ForProject).
type perProjectBucket struct{ remote.Bucket }

func (perProjectBucket) ContentsPerProject() bool { return true }

// Addresses "projtest+http://<fake>/team/r3v" open the fake storage that way.
func init() {
	remote.Register("projtest+", func(cfg remote.Config) (remote.Backend, error) {
		u, err := url.Parse(strings.TrimPrefix(cfg.URL, "projtest+"))
		if err != nil {
			return nil, err
		}
		bucket, prefix, _ := strings.Cut(strings.Trim(u.Path, "/"), "/")
		b, err := remote.NewS3(u.Scheme+"://"+u.Host, bucket, prefix, "auto", "key", "secret")
		if err != nil {
			return nil, err
		}
		return remote.NewBucketBackend(perProjectBucket{b.Bucket()}), nil
	})
}

// Everything a project stores goes under its own folder when storage keeps
// contents per project: saves (whole files, chunked files and their
// markers, trees), files sent ahead, clones and updates, verify's repairs.
// Nothing lands in the team's shared folders.
func TestContentsPerProject(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	addr := "projtest+" + fake.URL + "/team/r3v"
	a, _ := Init(newProject(t), "yi")
	if err := a.SetRemote(addr); err != nil {
		t.Fatal(err)
	}
	pid := a.Config.ProjectID
	in := func(h string) string { return "r3v/projects/" + pid + "/objects/" + h[:2] + "/" + h[2:] }
	shared := func() {
		t.Helper()
		if k := fake.Keys("team", "r3v/objects/"); len(k) != 0 {
			t.Fatalf("contents in the team's shared folder: %v", k)
		}
		if k := fake.Keys("team", "r3v/chunked/"); len(k) != 0 {
			t.Fatalf("markers in the team's shared folder: %v", k)
		}
	}

	level := randomBytes(3, 24<<20)
	small := randomBytes(4, 1<<20)
	write(t, a.Root, "Maps/Level.umap", string(level))
	write(t, a.Root, "Maps/Small.uasset", string(small))
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatalf("save: %v", err)
	}
	shared()
	h := chunk.HashOf(level)
	if obj, ok := fake.Object("team", in(h)); !ok || !blob.IsChunkList(obj) {
		t.Fatal("the level's chunk list isn't in the project's folder")
	}
	if _, ok := fake.Object("team", "r3v/projects/"+pid+"/chunked/"+h); !ok {
		t.Fatal("the level's marker isn't in the project's folder")
	}
	if obj, _ := fake.Object("team", in(chunk.HashOf(small))); !bytes.Equal(obj, small) {
		t.Error("the small file isn't in the project's folder")
	}

	// A big file sent ahead of the commit.
	defer func(m int64, s time.Duration) { PreuploadMin, PreuploadStable = m, s }(PreuploadMin, PreuploadStable)
	PreuploadMin, PreuploadStable = 1000, time.Minute
	take := strings.Repeat("frame ", 2000)
	write(t, a.Root, "Video/take.mov", take)
	cands, err := a.PreuploadCandidates(time.Now().Add(2 * time.Minute))
	if err != nil || len(cands) != 1 {
		t.Fatalf("candidates: %+v %v", cands, err)
	}
	c, _ := a.Client()
	if err := a.Preupload(c, cands[0], nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := fake.Object("team", in(cands[0].Hash)); !ok {
		t.Error("the file sent ahead isn't in the project's folder")
	}
	shared()

	// A teammate clones, then updates to an edit.
	b, _, err := Clone(addr, "Song", filepath.Join(t.TempDir(), "B", "Song Project"), "alex")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.Root, "Maps", "Level.umap")); !bytes.Equal(got, level) {
		t.Fatal("level differs after clone")
	}
	at := 7 << 20
	edited := append(append(append([]byte{}, level[:at]...), []byte("a new actor")...), level[at:]...)
	write(t, a.Root, "Maps/Level.umap", string(edited))
	if _, _, err := a.Save("an actor", Strategy("fail")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := b.Update(Strategy("fail")); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.Root, "Maps", "Level.umap")); !bytes.Equal(got, edited) {
		t.Fatal("level differs after update")
	}
	if got, _ := os.ReadFile(filepath.Join(b.Root, "Video", "take.mov")); string(got) != take {
		t.Fatal("the file sent ahead differs after update")
	}

	// A piece lost from the project's folder: verify tells, and puts it back
	// there.
	l := a.loadChunkList(chunk.HashOf(edited))
	if l == nil {
		t.Fatal("no chunk list kept here")
	}
	lost := l.Pieces[3].Hash
	fake.Delete("team", in(lost))
	if rep, err := a.Verify(true); err != nil || len(rep.Problems) == 0 || !rep.Problems[0].Fixed {
		t.Fatalf("repair: %+v %v", rep, err)
	}
	if _, ok := fake.Object("team", in(lost)); !ok {
		t.Error("the piece wasn't put back in the project's folder")
	}
	shared()

	// Another project of the team keeps its own copy of the same file.
	d, _ := Init(newProject(t), "sam")
	if err := d.SetRemote(addr); err != nil {
		t.Fatal(err)
	}
	write(t, d.Root, "Maps/Small.uasset", string(small))
	if _, _, err := d.Save("first", Strategy("fail")); err != nil {
		t.Fatalf("save: %v", err)
	}
	other := "r3v/projects/" + d.Config.ProjectID + "/objects/"
	hs := chunk.HashOf(small)
	if _, ok := fake.Object("team", other+hs[:2]+"/"+hs[2:]); !ok {
		t.Error("the second project doesn't keep its own copy")
	}
	shared()
}
