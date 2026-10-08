package remote_test

import (
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/membucket"
)

func withBranchRecords(t *testing.T) {
	t.Helper()
	was := remote.BranchRecords
	remote.BranchRecords = true
	t.Cleanup(func() { remote.BranchRecords = was })
}

func TestCleanBranchName(t *testing.T) {
	for in, want := range map[string]string{
		"  Mia's verse  ": "Mia's verse",
		"主歌 Demo":         "主歌 Demo",
		"éte":            "éte", // one way of writing é
		"ドラム 🥁":           "ドラム 🥁",
	} {
		if got, err := remote.CleanBranchName(in); err != nil || got != want {
			t.Errorf("%q: %q %v, want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "   ", "a\nb", "tab\there", strings.Repeat("字", 65)} {
		if _, err := remote.CleanBranchName(bad); err == nil {
			t.Errorf("%q was taken", bad)
		}
	}
	if _, err := remote.CleanBranchName(strings.Repeat("字", 64)); err != nil {
		t.Errorf("64 characters: %v", err)
	}
	if !remote.SameBranchName("Demo", "dEMO") || !remote.SameBranchName("ÉTÉ", "été") || remote.SameBranchName("Demo", "Demo 2") {
		t.Error("SameBranchName")
	}
}

func TestBranchKeyFor(t *testing.T) {
	none := func(string) bool { return false }
	for name, want := range map[string]string{
		"verse-2":         "verse-2",
		"Mia's Verse 2":   "mias-verse-2",
		"Été chanté":      "ete-chante",
		"  -- Big Mix --": "big-mix",
	} {
		if got := remote.BranchKeyFor(name, none); got != want {
			t.Errorf("%q: %q, want %q", name, got, want)
		}
	}
	for _, name := range []string{"主歌", "ドラム", "🥁"} {
		if got := remote.BranchKeyFor(name, none); !strings.HasPrefix(got, "b-") || len(got) != 10 {
			t.Errorf("%q: %q", name, got)
		}
	}
	taken := func(k string) bool { return k == "verse" || k == "big-mix" }
	if got := remote.BranchKeyFor("verse", taken); !strings.HasPrefix(got, "b-") {
		t.Errorf("a taken key: %q", got)
	}
}

func TestBranchRecords(t *testing.T) {
	withBranchRecords(t)
	b := remote.NewBucketBackend(membucket.New())
	store, ok := remote.BranchRecordsOf(b)
	if !ok {
		t.Fatal("storage keeps no branch records")
	}
	if err := store.PutBranchRecord(song, "b-1234abcd", remote.BranchRecord{Name: "主歌", Color: "b3"}); err != nil {
		t.Fatal(err)
	}
	recs, err := store.BranchRecords(song)
	if err != nil || recs["b-1234abcd"].Name != "主歌" || recs["b-1234abcd"].Color != "b3" {
		t.Fatalf("%+v %v", recs, err)
	}
	if err := store.PutBranchRecord(song, "../x", remote.BranchRecord{Name: "x"}); err == nil {
		t.Error("a key storage can't take")
	}
	// A hosted team's service doesn't keep them (yet).
	if _, ok := remote.BranchRecordsOf(remote.NewBucketBackend(perProject{membucket.New()})); ok {
		t.Error("per-project storage keeps branch records")
	}
	remote.BranchRecords = false
	if _, ok := remote.BranchRecordsOf(b); ok {
		t.Error("records without the channel")
	}
	if err := store.PutBranchRecord(song, "verse", remote.BranchRecord{Name: "Verse"}); err == nil {
		t.Error("written without the channel")
	}
}

// Storage reached directly has nothing to get ready.
func TestPrepareUploadsDirect(t *testing.T) {
	h := strings.Repeat("a", 64)
	if remote.NewBucketBackend(membucket.New()).PrepareUploads([]remote.Body{{Hash: h, SHA256: h, Size: 1}}, nil) {
		t.Error("a bucket reached directly prepares uploads")
	}
}
