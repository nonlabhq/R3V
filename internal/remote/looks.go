package remote

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg" // pictures are PNG or JPEG
	_ "image/png"
	"slices"
	"strings"
)

// Looks: how members and projects show in the team, so teammates see
// the same: a member's colour (for their initial) or picture, a project's
// icon and colour. Colours and icons are names from the app's own sets
// (the palette in tokens.css, the icons in the app); a name a build doesn't
// know shows as its own pick.
//
// A member's picture is stored at pictures/members/<member id>/<its
// SHA-256>, written whole (storage checks the upload against the sum: one
// cut off is never there), then the member's record names it, then that
// member's other pictures go. Stopped half-way, the record still names a
// picture that is there; a picture left over goes at the member's next
// change. Storage cleanup (gc.go) only looks under objects/ and chunked/,
// never under pictures/. (A project's own picture would go under
// pictures/projects/<project id>/, named by its record's picture, the same
// way.)
//
// Looks change what a team stores, so they are a team feature, on in
// Nightly builds only (looks_nightly.go); the first change turns it on.

// FeatureLooks is the team feature for looks.
const FeatureLooks = "looks"

// Looks: looks are on in this build (the Nightly channel). Off, nothing
// reads or writes them.
var Looks = false

const (
	// MaxPictureSize is the most bytes a stored picture has.
	MaxPictureSize = 64 << 10
	// MaxPictureSide is the most pixels on a side (pictures are square;
	// the app makes them 128).
	MaxPictureSide = 256

	memberPicturesDir = "pictures/members/"
)

var (
	// ErrLooksNotInBuild: this build has no looks (Stable).
	ErrLooksNotInBuild = errors.New("pictures, icons and colours need R3V's Nightly channel")
	// ErrNoLooks: the team's storage can't keep looks yet (a hosted team).
	ErrNoLooks = errors.New("this team can't keep pictures, icons and colours yet")
	// ErrBadPicture: not a small square PNG or JPEG.
	ErrBadPicture = errors.New("a picture must be a square PNG or JPEG of at most 256 pixels and 64 KB")
)

// PictureStore is storage that keeps member pictures.
type PictureStore interface {
	// PutPicture stores a checked picture of member id; it returns the
	// picture's SHA-256 (hex), what the member's record names.
	PutPicture(id string, data []byte) (string, error)
	// GetPicture reads member id's picture sum, checked against its sum.
	GetPicture(id, sum string) ([]byte, error)
	// PrunePictures deletes member id's pictures other than keep.
	PrunePictures(id, keep string) error
}

var _ PictureStore = (*BucketBackend)(nil)

// CheckPicture fails with ErrBadPicture unless data is a small square PNG or
// JPEG.
func CheckPicture(data []byte) error {
	if len(data) == 0 || len(data) > MaxPictureSize {
		return ErrBadPicture
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return ErrBadPicture
	}
	if cfg.Width != cfg.Height || cfg.Width < 1 || cfg.Width > MaxPictureSide {
		return ErrBadPicture
	}
	return nil
}

// ValidLookName reports whether c can name a colour or an icon: a short
// name of a-z, 0-9 and "-" ("" for none, the app's own pick).
func ValidLookName(c string) bool {
	if len(c) > 32 {
		return false
	}
	for _, r := range c {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func pictureKey(id, sum string) string { return memberPicturesDir + id + "/" + sum }

func (s *BucketBackend) PutPicture(id string, data []byte) (string, error) {
	if !Looks {
		return "", ErrLooksNotInBuild
	}
	if !ValidMemberID(id) {
		return "", errors.New("invalid member id")
	}
	if err := CheckPicture(data); err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	sum := hex.EncodeToString(h[:])
	if err := s.b.Put(pictureKey(id, sum), bytes.NewReader(data), int64(len(data)), sum, ""); err != nil {
		return "", err
	}
	return sum, nil
}

func (s *BucketBackend) GetPicture(id, sum string) ([]byte, error) {
	if !Looks {
		return nil, ErrLooksNotInBuild
	}
	if !ValidMemberID(id) || !validHex(sum, 64) {
		return nil, ErrNotFound
	}
	data, err := s.get(pictureKey(id, sum))
	if err != nil {
		return nil, err
	}
	if h := sha256.Sum256(data); hex.EncodeToString(h[:]) != sum || CheckPicture(data) != nil {
		return nil, ErrBadPicture
	}
	return data, nil
}

func (s *BucketBackend) PrunePictures(id, keep string) error {
	if !Looks {
		return ErrLooksNotInBuild
	}
	if !ValidMemberID(id) {
		return errors.New("invalid member id")
	}
	keys, err := s.list(memberPicturesDir + id + "/")
	if err != nil {
		return err
	}
	for _, k := range keys {
		if keep != "" && strings.HasSuffix(k, "/"+keep) {
			continue
		}
		if err := s.delete(k); err != nil {
			return err
		}
	}
	return nil
}

// SetMemberLook sets how member id shows in the team: color (a palette
// name, "" for the app's pick) and picture (nil for none, the initial on
// the colour). It turns member pictures on for the team first.
func SetMemberLook(b Backend, id, color string, picture []byte) error {
	if !Looks {
		return ErrLooksNotInBuild
	}
	// A hosted team's records go through its service, which keeps only
	// what it knows: looks wait for it to keep pictures.
	ps, ok := b.(PictureStore)
	if !ok {
		return ErrNoLooks
	}
	if !ValidLookName(color) {
		return errors.New("unknown colour")
	}
	ms, err := b.Members()
	if err != nil {
		return err // never write the record back without what it holds
	}
	i := slices.IndexFunc(ms, func(m Member) bool { return m.ID == id })
	if i < 0 {
		return errors.New("not a member of this team")
	}
	// Before anything is written: from here on, a R3V without pictures
	// stops before working with the team.
	if err := EnableFeature(b, FeatureLooks); err != nil {
		return err
	}
	sum := ""
	if picture != nil {
		if sum, err = ps.PutPicture(id, picture); err != nil {
			return err
		}
	}
	m := ms[i]
	m.Color, m.Picture = color, sum
	if err := b.PutMember(m); err != nil {
		return err
	}
	return ps.PrunePictures(id, sum)
}

// SetProjectLook sets how project pid shows in the team: icon and color
// (names, "" for the app's pick). It turns looks on for the team first.
func SetProjectLook(b Backend, pid, icon, color string) error {
	if !Looks {
		return ErrLooksNotInBuild
	}
	if _, ok := b.(PictureStore); !ok {
		return ErrNoLooks
	}
	if !ValidLookName(icon) || !ValidLookName(color) {
		return errors.New("unknown icon or colour")
	}
	ps, err := b.Projects()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(ps, func(p Project) bool { return p.ID == pid })
	if i < 0 {
		return ErrNotFound
	}
	if err := EnableFeature(b, FeatureLooks); err != nil {
		return err
	}
	p := ps[i]
	p.Icon, p.Color = icon, color
	return b.PutProject(p)
}
