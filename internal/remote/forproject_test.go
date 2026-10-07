package remote_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"slices"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/membucket"
)

type perProject struct{ remote.Bucket }

func (perProject) ContentsPerProject() bool { return true }

func keys(t *testing.T, b remote.Bucket, prefix string) []string {
	t.Helper()
	var out []string
	if err := b.List(prefix, "", func(page []remote.Item) bool {
		for _, it := range page {
			out = append(out, it.Key)
		}
		return true
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestForProjectLeavesStorageTeamsAlone(t *testing.T) {
	b := remote.NewBucketBackend(membucket.New())
	if remote.ForProject(b, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") != remote.Backend(b) {
		t.Error("a storage team's backend should be used as it is")
	}
}

func TestForProjectKeepsContentsInTheProject(t *testing.T) {
	mb := membucket.New()
	const pid = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	team := remote.NewBucketBackend(perProject{mb})
	b := remote.ForProject(team, pid)

	data := []byte("a kick")
	sum := sha256.Sum256(data)
	h := hex.EncodeToString(sum[:])
	if err := b.PutObject(h, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	if err := b.(remote.BodyStore).MarkChunked(h); err != nil {
		t.Fatal(err)
	}
	if got := keys(t, mb, "projects/"+pid+"/"); len(got) != 2 ||
		got[0] != "projects/"+pid+"/chunked/"+h || got[1] != "projects/"+pid+"/objects/"+h[:2]+"/"+h[2:] {
		t.Errorf("project keys: %v", got)
	}
	for _, shared := range []string{"objects/", "chunked/"} {
		if got := keys(t, mb, shared); len(got) != 0 {
			t.Errorf("%s: %v", shared, got)
		}
	}
	r, err := b.GetObject(h)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(r)
	r.Close()
	if !bytes.Equal(got, data) {
		t.Errorf("GetObject = %q", got)
	}
	// Seen from the team's folder, it isn't there.
	if _, err := team.GetObject(h); err == nil {
		t.Error("the team's own folder shouldn't have the project's object")
	}

	// "Missing" in the project, both ways it asks: one by one, and by
	// listing a folder where many are wanted (here 6 in folder "ab").
	var want []string
	for i := 0; i < 6; i++ {
		want = append(want, fmt.Sprintf("ab%062x", i))
	}
	for _, w := range want[:3] {
		if err := mb.Put("projects/"+pid+"/objects/"+w[:2]+"/"+w[2:], bytes.NewReader(nil), 0, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	missing, err := b.MissingObjects(append(want, h))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(missing)
	if !slices.Equal(missing, want[3:]) {
		t.Errorf("missing %v, want the last three of folder ab", missing)
	}
}
