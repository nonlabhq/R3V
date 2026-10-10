package remote

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/nonlabhq/r3v/internal/jsonx"
)

// Branch records: how a branch shows, apart from where it is. A branch's
// key (projects/<pid>/branches/<key>) stays what storage and every R3V can
// read; its record (projects/<pid>/branchinfo/<key>.json) gives it a name
// people choose (any language, spaces, capitals) and a colour (a palette
// number). Renaming changes the record only: nothing moves, a teammate on
// the branch keeps working. An R3V without records shows the key.
//
// A new branch's record is written before the branch: stopped in between,
// a record no branch has is left, never shown (a branch made again with
// that key writes it anew).
//
// Records only add to what a team stores (every R3V ignores the folder;
// storage cleanup looks only under objects/ and chunked/), so they are not
// a team feature. They are built in Nightly only (branches_nightly.go).

// BranchRecords: this build keeps branch records (the Nightly channel).
var BranchRecords = false

// ErrNoBranchRecords: the team's storage can't keep branch records yet.
var ErrNoBranchRecords = errors.New("this team can't keep branch names and colours yet")

// MaxBranchName is the most characters (not bytes) of a branch's name.
const MaxBranchName = 64

// BranchRecord is how a branch shows.
type BranchRecord struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"` // a palette number ("b3"), "" for the app's pick
	// Parent is the key of the branch it was made from, From the version it
	// started at (docs/design/branch-tree.md); "" for main and older branches.
	Parent string `json:"parent,omitempty"`
	From   string `json:"from,omitempty"`
	// Removed: archived, then deleted for good (listed no more).
	Removed bool `json:"removed,omitempty"`
	// Deleted: where the branch was when it was deleted, written before
	// the delete (the branch log is written after, as well as it can be),
	// so a branch deleted can always come back.
	Deleted *BranchDeleted `json:"deleted,omitempty"`
	// Extra: fields a newer R3V wrote, kept when this one rewrites the record.
	Extra jsonx.Extra `json:"-"`
}

func (v *BranchRecord) UnmarshalJSON(b []byte) error {
	type plain BranchRecord
	return jsonx.Decode(b, (*plain)(v), &v.Extra)
}

func (v BranchRecord) MarshalJSON() ([]byte, error) {
	type plain BranchRecord
	return jsonx.Encode(plain(v), v.Extra)
}

// BranchDeleted is a branch's deletion.
type BranchDeleted struct {
	Head string    `json:"head"`
	By   string    `json:"by,omitempty"` // member id
	Time time.Time `json:"time"`
}

// ActorOf names who b's writes are by (a member id, "" unknown).
func ActorOf(b Backend) string {
	if s, ok := b.(*BucketBackend); ok {
		return s.actor
	}
	return ""
}

// BranchRecordStore is storage that keeps branch records.
type BranchRecordStore interface {
	// BranchRecords maps project pid's branch keys to their records.
	BranchRecords(pid string) (map[string]BranchRecord, error)
	// PutBranchRecord writes the record of branch key.
	PutBranchRecord(pid, key string, r BranchRecord) error
}

// BranchRecordKeeper is storage keeping contents per project that keeps
// branch records all the same (a hosted team's service that knows them).
type BranchRecordKeeper interface{ KeepsBranchRecords() bool }

// BranchRecordsOf returns b's branch record store, if its team keeps them.
func BranchRecordsOf(b Backend) (BranchRecordStore, bool) {
	if !BranchRecords {
		return nil, false
	}
	s, ok := b.(*BucketBackend)
	if !ok {
		return nil, false
	}
	if pp, ok := s.b.(PerProject); ok && pp.ContentsPerProject() {
		if k, ok := s.b.(BranchRecordKeeper); !ok || !k.KeepsBranchRecords() {
			return nil, false
		}
	}
	return s, true
}

func branchRecordKey(pid, key string) string {
	return projectDir(pid) + "branchinfo/" + key + ".json"
}

func (s *BucketBackend) BranchRecords(pid string) (map[string]BranchRecord, error) {
	if !validHex(pid, 32) {
		return nil, errors.New("invalid project id")
	}
	keys, err := s.list(projectDir(pid) + "branchinfo/")
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	out := map[string]BranchRecord{}
	err = parallel(keys, func(k string) error {
		name := strings.TrimSuffix(path.Base(k), ".json")
		if !validBranch(name) {
			return nil
		}
		data, err := s.get(k)
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var r BranchRecord
		if json.Unmarshal(data, &r) != nil {
			return nil // one that can't be read shows as its key
		}
		mu.Lock()
		out[name] = r
		mu.Unlock()
		return nil
	})
	return out, err
}

func (s *BucketBackend) PutBranchRecord(pid, key string, r BranchRecord) error {
	if !BranchRecords {
		return errors.New("branch names and colours need R3V's Nightly channel")
	}
	if !validHex(pid, 32) || !validBranch(key) {
		return errors.New("invalid branch")
	}
	data, _ := json.Marshal(r)
	return s.put(branchRecordKey(pid, key), data)
}

// CleanBranchName makes name one way of writing it (Unicode NFC, no space
// around it) and checks it: 1 to MaxBranchName characters, none a control.
func CleanBranchName(name string) (string, error) {
	name = strings.TrimSpace(norm.NFC.String(name))
	if name == "" {
		return "", errors.New("a branch needs a name")
	}
	if utf8.RuneCountInString(name) > MaxBranchName {
		return "", errors.New("a branch name has at most 64 characters")
	}
	if strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", errors.New("a branch name can't have line breaks or control characters")
	}
	return name, nil
}

// SameBranchName reports whether a and b name the same branch (ignoring
// case, in any language).
func SameBranchName(a, b string) bool {
	return strings.EqualFold(norm.NFC.String(a), norm.NFC.String(b))
}

// BranchKeyFor picks the key of a new branch named name: its name, when
// storage can take it as it is, else its Latin letters and digits
// ("Mia's Verse 2" → "mias-verse-2"), else something random ("b-3f9a1c2e");
// never one in taken.
func BranchKeyFor(name string, taken func(key string) bool) string {
	if validBranch(name) && !taken(name) {
		return name
	}
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(norm.NFKD.String(name)) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.' || r == '/':
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	key := strings.TrimRight(b.String(), "-")
	if len(key) > 48 {
		key = strings.TrimRight(key[:48], "-")
	}
	if len(key) >= 2 && validBranch(key) && !taken(key) {
		return key
	}
	for {
		var r [4]byte
		rand.Read(r[:])
		if key := "b-" + hex.EncodeToString(r[:]); !taken(key) {
			return key
		}
	}
}
