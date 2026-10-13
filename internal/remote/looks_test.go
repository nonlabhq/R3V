package remote_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/membucket"
)

const (
	alice = "0123456789abcdef0123456789abcdef"
	song  = "00112233445566778899aabbccddeeff"
)

// picture is a side×side PNG of one colour.
func picture(t *testing.T, side int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	for i := range img.Pix {
		img.Pix[i] = 0
	}
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withLooks turns member pictures on for a test (as Nightly does).
func withLooks(t *testing.T) {
	t.Helper()
	was := remote.Looks
	remote.Looks = true
	t.Cleanup(func() { remote.Looks = was })
}

// failing is a bucket that stops at a write: the first Put or Delete whose
// key starts with at fails (a Put after sending half its bytes).
type failing struct {
	*membucket.Bucket
	at string
	// after, when set, runs once after a Put whose key starts with
	// afterAt (another computer writing meanwhile).
	after   func()
	afterAt string
}

func (f *failing) Put(key string, r io.Reader, size int64, sum, cond string) error {
	if f.after != nil && strings.HasPrefix(key, f.afterAt) {
		after := f.after
		f.after = nil
		defer after()
	}
	if f.at != "" && strings.HasPrefix(key, f.at) {
		half, _ := io.ReadAll(io.LimitReader(r, size/2))
		f.Bucket.Put(key, bytes.NewReader(half), size, sum, cond) // refused: too short
		return errors.New("connection lost")
	}
	return f.Bucket.Put(key, r, size, sum, cond)
}

func (f *failing) Delete(key, cond string) error {
	if f.at != "" && strings.HasPrefix(key, f.at) {
		return errors.New("connection lost")
	}
	return f.Bucket.Delete(key, cond)
}

func newTeam(t *testing.T) (*failing, *remote.BucketBackend) {
	t.Helper()
	f := &failing{Bucket: membucket.New()}
	b := remote.NewBucketBackend(f)
	if err := b.PutMember(remote.Member{ID: alice, Name: "Alice"}); err != nil {
		t.Fatal(err)
	}
	return f, b
}

func member(t *testing.T, b remote.Backend) remote.Member {
	t.Helper()
	ms, err := b.Members()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range ms {
		if m.ID == alice {
			return m
		}
	}
	t.Fatal("no Alice")
	return remote.Member{}
}

func pictureKeys(t *testing.T, f *failing) []string {
	t.Helper()
	var keys []string
	f.List("pictures/", "", func(items []remote.Item) bool {
		for _, it := range items {
			keys = append(keys, it.Key)
		}
		return true
	})
	return keys
}

func TestMemberLook(t *testing.T) {
	withLooks(t)
	f, b := newTeam(t)
	red := picture(t, 128, color.RGBA{255, 0, 0, 255})
	if err := remote.SetMemberLook(b, alice, "3", red); err != nil {
		t.Fatal(err)
	}
	m := member(t, b)
	if m.Name != "Alice" || m.Color != "3" || m.Picture == "" {
		t.Fatalf("record: %+v", m)
	}
	got, err := b.GetPicture(alice, m.Picture)
	if err != nil || !bytes.Equal(got, red) {
		t.Fatalf("GetPicture: %v", err)
	}
	// Not a team feature: an R3V without looks keeps working with the team.
	if info, _ := b.Info(); len(info.Features) != 0 {
		t.Errorf("the team's features: %v", info.Features)
	}

	// A new picture replaces the old one; none removes it.
	blue := picture(t, 128, color.RGBA{0, 0, 255, 255})
	if err := remote.SetMemberLook(b, alice, "3", blue); err != nil {
		t.Fatal(err)
	}
	if keys := pictureKeys(t, f); len(keys) != 1 || !strings.HasSuffix(keys[0], member(t, b).Picture) {
		t.Errorf("pictures after a change: %v", keys)
	}
	if err := remote.SetMemberLook(b, alice, "1", nil); err != nil {
		t.Fatal(err)
	}
	if m := member(t, b); m.Picture != "" || m.Color != "1" {
		t.Errorf("record after removing: %+v", m)
	}
	if keys := pictureKeys(t, f); len(keys) != 0 {
		t.Errorf("pictures left: %v", keys)
	}

	// Renaming keeps the look (a Stable R3V renames too).
	remote.SetMemberLook(b, alice, "2", red)
	if err := remote.RenameMember(b, alice, "Al"); err != nil {
		t.Fatal(err)
	}
	if m := member(t, b); m.Name != "Al" || m.Color != "2" || m.Picture == "" {
		t.Errorf("record after renaming: %+v", m)
	}
}

func TestMemberLookRefuses(t *testing.T) {
	withLooks(t)
	f, b := newTeam(t)
	for name, data := range map[string][]byte{
		"not square": func() []byte {
			var buf bytes.Buffer
			png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 128, 64)))
			return buf.Bytes()
		}(),
		"too big a side": picture(t, 300, color.White),
		"too many bytes": append(picture(t, 16, color.White), make([]byte, remote.MaxPictureSize)...),
		"not a picture":  []byte("GIF89a, or something"),
		"empty":          {},
	} {
		if err := remote.SetMemberLook(b, alice, "", data); !errors.Is(err, remote.ErrBadPicture) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if keys := pictureKeys(t, f); len(keys) != 0 {
		t.Errorf("stored: %v", keys)
	}
	if err := remote.SetMemberLook(b, alice, "Red!", nil); err == nil {
		t.Error("a colour that isn't a palette name was taken")
	}
	if err := remote.SetMemberLook(b, "fedcba9876543210fedcba9876543210", "", nil); err == nil {
		t.Error("someone not in the team got a look")
	}
	// A picture altered in storage isn't shown.
	remote.SetMemberLook(b, alice, "", picture(t, 64, color.White))
	m := member(t, b)
	f.Bucket.Put("pictures/members/"+alice+"/"+m.Picture, bytes.NewReader([]byte("x")), 1, "", "")
	if _, err := b.GetPicture(alice, m.Picture); err == nil {
		t.Error("an altered picture was read")
	}
}

// Stopped at each write, the record names a picture that is there (the
// old one or the new one), and what was left over goes at the next change.
func TestMemberLookInterrupted(t *testing.T) {
	withLooks(t)
	old := picture(t, 128, color.RGBA{255, 0, 0, 255})
	next := picture(t, 128, color.RGBA{0, 255, 0, 255})
	for _, at := range []string{"pictures/", "members/", "pictures/members/" + alice + "/"} {
		f, b := newTeam(t)
		if err := remote.SetMemberLook(b, alice, "1", old); err != nil {
			t.Fatal(err)
		}
		f.at = at
		if at == "pictures/members/"+alice+"/" { // the cleanup after the record
			f.at = "pictures/members/" + alice + "/" + member(t, b).Picture
		}
		if err := remote.SetMemberLook(b, alice, "2", next); err == nil {
			t.Fatalf("%s: no failure", at)
		}
		f.at = ""
		m := member(t, b)
		got, err := b.GetPicture(alice, m.Picture)
		if err != nil || !(bytes.Equal(got, old) || bytes.Equal(got, next)) {
			t.Errorf("stopped at %s: the record's picture: %v", at, err)
		}
		if err := remote.SetMemberLook(b, alice, "2", next); err != nil {
			t.Fatal(err)
		}
		if keys := pictureKeys(t, f); len(keys) != 1 {
			t.Errorf("stopped at %s, then changed again: %v", at, keys)
		}
	}
}

// The same member changing their picture on two computers at once: the
// cleanup after one change keeps the picture the other one's record names.
func TestMemberLookOnTwoComputers(t *testing.T) {
	withLooks(t)
	f, b := newTeam(t)
	mine := picture(t, 128, color.RGBA{255, 0, 0, 255})
	theirs := picture(t, 128, color.RGBA{0, 0, 255, 255})
	f.afterAt = "members/"
	f.after = func() { // the other computer, right after this one's record
		if err := remote.SetMemberLook(b, alice, "2", theirs); err != nil {
			t.Error(err)
		}
	}
	if err := remote.SetMemberLook(b, alice, "1", mine); err != nil {
		t.Fatal(err)
	}
	m := member(t, b)
	if got, err := b.GetPicture(alice, m.Picture); err != nil || !bytes.Equal(got, theirs) {
		t.Errorf("the record's picture: %v", err)
	}
}

func TestCleanupKeepsPictures(t *testing.T) {
	withLooks(t)
	f, b := newTeam(t)
	if err := remote.SetMemberLook(b, alice, "", picture(t, 128, color.White)); err != nil {
		t.Fatal(err)
	}
	before := pictureKeys(t, f)
	for range 2 { // marked, then deleted
		if _, err := b.CollectGarbage(true); err != nil {
			t.Fatal(err)
		}
	}
	if after := pictureKeys(t, f); !slices.Equal(before, after) {
		t.Errorf("cleanup: %v, then %v", before, after)
	}
}

// A hosted team's storage keeps no pictures: the app shows the initial.
func TestNoPicturesWithoutStore(t *testing.T) {
	withLooks(t)
	f, _ := newTeam(t)
	b := remote.NewBucketBackend(perProject{f})
	if err := remote.SetMemberLook(b, alice, "", nil); !errors.Is(err, remote.ErrNoLooks) {
		t.Errorf("SetMemberLook: %v", err)
	}
	if err := remote.SetMemberLook(struct{ remote.Backend }{b}, alice, "", nil); !errors.Is(err, remote.ErrNoLooks) {
		t.Errorf("SetMemberLook, another backend: %v", err)
	}
}

func TestProjectLook(t *testing.T) {
	withLooks(t)
	_, b := newTeam(t)
	b.PutProject(remote.Project{ID: song, Name: "Song"})
	if err := remote.SetProjectLook(b, song, "drum", "b3"); err != nil {
		t.Fatal(err)
	}
	if err := remote.RenameProject(b, song, "Song 2"); err != nil {
		t.Fatal(err)
	}
	ps, _ := b.Projects()
	if len(ps) != 1 || ps[0].Name != "Song 2" || ps[0].Icon != "drum" || ps[0].Color != "b3" {
		t.Errorf("record: %+v", ps)
	}
	if info, _ := b.Info(); len(info.Features) != 0 {
		t.Errorf("the team's features: %v", info.Features)
	}
	family := "e-1f468-1f3fb-200d-1f469-1f3fb-200d-1f467-1f3fb" // 👨🏻‍👩🏻‍👧🏻
	if err := remote.SetProjectLook(b, song, family, ""); err != nil {
		t.Errorf("an emoji icon: %v", err)
	}
	if err := remote.SetProjectLook(b, song, "e-"+strings.Repeat("1f3b5-", 11), ""); err == nil {
		t.Error("an icon name longer than 64 was taken")
	}
	if err := remote.SetProjectLook(b, song, "<svg>", ""); err == nil {
		t.Error("an icon that isn't a name was taken")
	}
	if err := remote.SetProjectLook(b, "fedcba9876543210fedcba9876543210", "drum", ""); !errors.Is(err, remote.ErrNotFound) {
		t.Errorf("a project not in the team: %v", err)
	}
	f, _ := newTeam(t)
	if err := remote.SetProjectLook(remote.NewBucketBackend(perProject{f}), song, "drum", ""); !errors.Is(err, remote.ErrNoLooks) {
		t.Errorf("a team that can't keep looks: %v", err)
	}
}

// A project's and the team's own pictures, kept as a member's: the record
// names a picture that is there wherever it stops, the old ones go.
func TestProjectAndTeamPictures(t *testing.T) {
	withLooks(t)
	red := picture(t, 128, color.RGBA{255, 0, 0, 255})
	blue := picture(t, 128, color.RGBA{0, 0, 255, 255})
	project := func(b remote.Backend) remote.Project {
		ps, _ := b.Projects()
		return ps[0]
	}
	cases := []struct {
		name string
		dir  string
		set  func(b remote.Backend, pic []byte) error
		sum  func(b remote.Backend) string
	}{
		{"project", "pictures/projects/" + song + "/", func(b remote.Backend, pic []byte) error { return remote.SetProjectPicture(b, song, pic) },
			func(b remote.Backend) string { return project(b).Picture }},
		{"team", "pictures/team/", func(b remote.Backend, pic []byte) error { return remote.SetTeamPicture(b, pic) },
			func(b remote.Backend) string { info, _ := b.Info(); return info.Picture }},
	}
	for _, c := range cases {
		f, b := newTeam(t)
		b.PutProject(remote.Project{ID: song, Name: "Song", Icon: "drum"})
		b.SetInfo(remote.TeamInfo{Name: "Band", Icon: "mic"})
		if err := c.set(b, red); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got, err := b.GetPictureIn(c.dir, c.sum(b))
		if err != nil || !bytes.Equal(got, red) {
			t.Fatalf("%s: GetPictureIn: %v", c.name, err)
		}
		if p := project(b); p.Name != "Song" || p.Icon != "drum" {
			t.Errorf("%s: the project's record: %+v", c.name, p)
		}
		if info, _ := b.Info(); info.Name != "Band" || info.Icon != "mic" || len(info.Features) != 0 {
			t.Errorf("%s: the team's record: %+v", c.name, info)
		}
		// Stopped at each step: the record names a picture that is there.
		for _, at := range []string{c.dir, "projects/", "team.json", c.dir + c.sum(b)} {
			f.at = at
			c.set(b, blue)
			f.at = ""
			if sum := c.sum(b); sum != "" {
				if _, err := b.GetPictureIn(c.dir, sum); err != nil {
					t.Errorf("%s: stopped at %s: %v", c.name, at, err)
				}
			}
		}
		if err := c.set(b, blue); err != nil {
			t.Fatal(err)
		}
		if keys := pictureKeys(t, f); len(keys) != 1 || !strings.HasSuffix(keys[0], c.sum(b)) {
			t.Errorf("%s: pictures after a change: %v", c.name, keys)
		}
		if err := c.set(b, nil); err != nil {
			t.Fatal(err)
		}
		if c.sum(b) != "" || len(pictureKeys(t, f)) != 0 {
			t.Errorf("%s: after removing: %q %v", c.name, c.sum(b), pictureKeys(t, f))
		}
	}
	_, b := newTeam(t)
	if err := remote.SetProjectPicture(b, song, red); !errors.Is(err, remote.ErrNotFound) {
		t.Errorf("a project not in the team: %v", err)
	}
	if err := remote.SetProjectPicture(b, song, []byte("not a picture")); err == nil {
		t.Error("not a picture: taken")
	}
	if _, err := b.GetPictureIn(remote.PicturesOf("project", "../x"), "ab"); !errors.Is(err, remote.ErrNotFound) {
		t.Errorf("a folder that isn't one: %v", err)
	}
}

// The team's icon and colour, in team.json; a rename keeps them.
func TestTeamLook(t *testing.T) {
	withLooks(t)
	_, b := newTeam(t)
	b.SetInfo(remote.TeamInfo{Name: "Band"})
	if err := remote.SetTeamLook(b, "e-1f3b8", "b4"); err != nil {
		t.Fatal(err)
	}
	if err := remote.Rename(b, "The Band"); err != nil {
		t.Fatal(err)
	}
	if info, _ := b.Info(); info.Name != "The Band" || info.Icon != "e-1f3b8" || info.Color != "b4" {
		t.Errorf("record: %+v", info)
	}
	if err := remote.SetTeamLook(b, "<svg>", ""); err == nil {
		t.Error("an icon that isn't a name was taken")
	}
	f, _ := newTeam(t)
	if err := remote.SetTeamLook(remote.NewBucketBackend(perProject{f}), "mic", ""); !errors.Is(err, remote.ErrNoLooks) {
		t.Errorf("a team that can't keep looks: %v", err)
	}
}
