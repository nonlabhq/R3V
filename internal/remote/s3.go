package remote

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"
)

// BucketBackend keeps a team's data in a Bucket (an S3-compatible bucket
// used directly; docs/design/storage-backends.md).
// Layout:
//
//	objects/<ab>/<cdef…>                  file contents
//	projects/<pid>/project.json
//	projects/<pid>/snapshots/<id>.json
//	projects/<pid>/branches/<name>        body = version id; moved with conditional writes
//	projects/<pid>/workspaces/<wsid>.json
//	members/<id>.json, team.json, setups/, backups/, chunked/, gc/
type BucketBackend struct {
	b     Bucket
	actor string // who branch moves are by (SetActor)
	// objs is where contents (objects/, chunked/) go: "" for the team's own
	// folder, projects/<pid>/ when each project keeps its own (ForProject).
	objs string
}

var _ Backend = (*BucketBackend)(nil)

// NewBucketBackend keeps a team's data in b.
func NewBucketBackend(b Bucket) *BucketBackend { return &BucketBackend{b: b} }

// NewS3 connects to bucket at endpoint (e.g. https://<account>.r2.cloudflarestorage.com)
// using path-style requests, keeping the team's data under prefix.
func NewS3(endpoint, bucket, prefix, region, accessKey, secretKey string) (*BucketBackend, error) {
	b, err := newS3Bucket(endpoint, bucket, prefix, region, accessKey, secretKey)
	if err != nil {
		return nil, err
	}
	return NewBucketBackend(b), nil
}

// Bucket is the storage underneath.
func (s *BucketBackend) Bucket() Bucket { return s.b }

// PerProject is implemented by storage that keeps each project's contents
// apart (the hosted service: an upload can't be checked against its name,
// so only a project's own writers may write its contents).
type PerProject interface{ ContentsPerProject() bool }

// ThroughService reports whether b's contents go through a service (a
// hosted team's: each transfer asks it for a URL first, so more of them
// at once keep the line busy).
func ThroughService(b Backend) bool {
	s, ok := b.(*BucketBackend)
	if !ok {
		return false
	}
	pp, ok := s.b.(PerProject)
	return ok && pp.ContentsPerProject()
}

// ForProject is the backend for working on project pid: on storage that
// keeps contents per project, its contents go under projects/<pid>/;
// otherwise it is b itself.
func ForProject(b Backend, pid string) Backend {
	s, ok := b.(*BucketBackend)
	if !ok {
		return b
	}
	if pp, ok := s.b.(PerProject); !ok || !pp.ContentsPerProject() {
		return b
	}
	c := *s
	c.objs = projectDir(pid)
	return &c
}

func (s *BucketBackend) objectKey(hash string) string { return s.objs + objectKey(hash) }

// --- helpers ---

func (s *BucketBackend) get(key string) ([]byte, error) {
	data, _, err := s.b.Get(key)
	return data, err
}

func (s *BucketBackend) put(key string, data []byte) error {
	return s.b.Put(key, bytes.NewReader(data), int64(len(data)), "", "")
}

func (s *BucketBackend) delete(key string) error { return s.b.Delete(key, "") }

func (s *BucketBackend) list(dir string) ([]string, error) { return listKeys(s.b, dir) }

// parallel runs fn over items with bounded concurrency, collecting errors.
func parallel(items []string, fn func(string) error) error { return parallelN(8, items, fn) }

// checks is how many existence checks (small requests) go at once.
const checks = 32

func parallelN(n int, items []string, fn func(string) error) error {
	sem := make(chan struct{}, n)
	var mu sync.Mutex
	var first error
	var wg sync.WaitGroup
	for _, it := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(it string) {
			defer func() { <-sem; wg.Done() }()
			if err := fn(it); err != nil {
				mu.Lock()
				if first == nil {
					first = err
				}
				mu.Unlock()
			}
		}(it)
	}
	wg.Wait()
	return first
}

// readAll reads every key under dir (records: small), skipping ones gone
// meanwhile; fn gets each key and its bytes.
func (s *BucketBackend) readAll(dir string, fn func(key string, data []byte)) error {
	keys, err := s.list(dir)
	if err != nil {
		return err
	}
	var mu sync.Mutex
	return parallel(keys, func(key string) error {
		data, err := s.get(key)
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		mu.Lock()
		fn(key, data)
		mu.Unlock()
		return nil
	})
}

// --- keys ---

func objectKey(hash string) string       { return "objects/" + hash[:2] + "/" + hash[2:] }
func projectDir(pid string) string       { return "projects/" + pid + "/" }
func snapshotKey(pid, id string) string  { return projectDir(pid) + "snapshots/" + id + ".json" }
func branchKey(pid, name string) string  { return projectDir(pid) + "branches/" + name }
func workspaceKey(pid, ws string) string { return projectDir(pid) + "workspaces/" + ws + ".json" }

// memberKey: one object per member, so members never overwrite each other.
func memberKey(id string) string { return "members/" + id + ".json" }
func validHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

func validBranch(name string) bool {
	if name == "" || len(name) > 64 || strings.HasPrefix(name, ".") {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// --- Backend ---

func (s *BucketBackend) Projects() ([]Project, error) {
	dirs, err := s.b.Folders("projects/")
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	out := []Project{}
	err = parallel(dirs, func(dir string) error {
		data, err := s.get(dir + "project.json")
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var p Project
		if json.Unmarshal(data, &p) == nil && p.ID != "" {
			mu.Lock()
			out = append(out, p)
			mu.Unlock()
		}
		return nil
	})
	return out, err
}

func (s *BucketBackend) PutProject(p Project) error {
	if !validHex(p.ID, 32) {
		return errors.New("invalid project id")
	}
	data, _ := json.Marshal(p)
	return s.put(projectDir(p.ID)+"project.json", data)
}

// DeleteProject removes project.json first, so the project leaves the list
// even if deleting the rest is interrupted.
func (s *BucketBackend) DeleteProject(pid string) error {
	if !validHex(pid, 32) {
		return errors.New("invalid project id")
	}
	if err := s.delete(projectDir(pid) + "project.json"); err != nil {
		return err
	}
	keys, err := s.list(projectDir(pid))
	if err != nil {
		return err
	}
	return parallel(keys, s.delete)
}

func (s *BucketBackend) branch(pid, name string) (id, etag string, err error) {
	data, etag, err := s.b.Get(branchKey(pid, name))
	if errors.Is(err, ErrNotFound) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(string(data)), etag, nil
}

// BranchHead reads one branch's head ("" if it doesn't exist): a single GET,
// which storage bills far less than the listing Branches needs.
func (s *BucketBackend) BranchHead(pid, name string) (string, error) {
	if !validBranch(name) {
		return "", fmt.Errorf("invalid branch name %q", name)
	}
	id, _, err := s.branch(pid, name)
	return id, err
}

func (s *BucketBackend) Branches(pid string) (map[string]string, error) {
	keys, err := s.list(projectDir(pid) + "branches/")
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	out := map[string]string{}
	err = parallel(keys, func(key string) error {
		name := path.Base(key)
		id, _, err := s.branch(pid, name)
		if err == nil && id != "" {
			mu.Lock()
			out[name] = id
			mu.Unlock()
		}
		return err
	})
	return out, err
}

// UpdateBranch uses conditional writes: create only if absent, move or
// delete only if the branch still has the etag read.
func (s *BucketBackend) UpdateBranch(pid, name, old, new string) error {
	if !validBranch(name) {
		return fmt.Errorf("invalid branch name %q", name)
	}
	current, etag, err := s.branch(pid, name)
	if err != nil {
		return err
	}
	if current != old {
		return &ErrConflict{Current: current}
	}
	cond := etag
	if old == "" {
		cond = "*"
	}
	key := branchKey(pid, name)
	if new == "" {
		err = s.b.Delete(key, cond)
	} else {
		data := []byte(new + "\n")
		err = s.b.Put(key, bytes.NewReader(data), int64(len(data)), "", cond)
	}
	if errors.Is(err, ErrPrecondition) || (new == "" && errors.Is(err, ErrNotFound)) {
		cur, _, err := s.branch(pid, name)
		if err != nil {
			return err
		}
		return &ErrConflict{Current: cur}
	}
	if err == nil {
		s.logMove(pid, name, old, new)
	}
	return err
}

// SnapshotLister is storage that lists a project's versions (so a long
// history can be asked for all at once).
type SnapshotLister interface {
	SnapshotIDs(pid string) ([]string, error)
}

var _ SnapshotLister = (*BucketBackend)(nil)

func (s *BucketBackend) SnapshotIDs(pid string) ([]string, error) {
	if !validHex(pid, 32) {
		return nil, errors.New("invalid project id")
	}
	keys, err := s.list(projectDir(pid) + "snapshots/")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, k := range keys {
		if id := strings.TrimSuffix(path.Base(k), ".json"); validHex(id, 64) {
			out = append(out, id)
		}
	}
	return out, nil
}

func (s *BucketBackend) MissingSnapshots(pid string, ids []string) ([]string, error) {
	return s.missing(ids, func(id string) string { return snapshotKey(pid, id) })
}

func (s *BucketBackend) missing(names []string, key func(string) string) ([]string, error) {
	var mu sync.Mutex
	missing := []string{}
	err := parallelN(checks, names, func(n string) error {
		if !validHex(n, 64) {
			return fmt.Errorf("invalid id %q", n)
		}
		ok, err := s.b.Exists(key(n))
		if err == nil && !ok {
			mu.Lock()
			missing = append(missing, n)
			mu.Unlock()
		}
		return err
	})
	return missing, err
}

func (s *BucketBackend) PutSnapshot(pid, id string, data []byte) error {
	if !validHex(id, 64) || sha256Hex(data) != id {
		return errors.New("version content does not match its id")
	}
	return s.put(snapshotKey(pid, id), data)
}

func (s *BucketBackend) GetSnapshot(pid, id string) ([]byte, error) {
	if !validHex(id, 64) {
		return nil, ErrNotFound
	}
	data, err := s.get(snapshotKey(pid, id))
	if err != nil {
		return nil, err
	}
	if sha256Hex(data) != id {
		return nil, fmt.Errorf("version %s is corrupt in storage", id[:10])
	}
	return data, nil
}

// MissingObjects asks storage about each object, or, where many share a
// folder (objects/<ab>/), lists the folder: a first share asks about
// thousands of files, and one listing answers for up to 1000 of them.
func (s *BucketBackend) MissingObjects(hashes []string) ([]string, error) {
	shards := map[string][]string{}
	for _, h := range hashes {
		if !validHex(h, 64) {
			return nil, fmt.Errorf("invalid id %q", h)
		}
		shards[h[:2]] = append(shards[h[:2]], h)
	}
	if a, ok := s.b.(ContentsAsker); ok && s.objs != "" {
		return askMissing(a, strings.TrimSuffix(strings.TrimPrefix(s.objs, "projects/"), "/"), dedupeHashes(hashes))
	}
	var mu sync.Mutex
	present := map[string]bool{}
	var ask []string // to ask about one by one
	var listed []string
	for shard, hs := range shards {
		if len(hs) >= listFrom {
			listed = append(listed, shard)
		} else {
			ask = append(ask, hs...)
		}
	}
	err := parallelN(checks, listed, func(shard string) error {
		want := slices.Clone(shards[shard])
		slices.Sort(want)
		// From just before the first wanted object.
		first := s.objectKey(want[0])
		return s.b.List(s.objs+"objects/"+shard+"/", first[:len(first)-1], func(page []Item) bool {
			mu.Lock()
			for _, it := range page {
				present[it.Key] = true
			}
			mu.Unlock()
			// Wanted objects past this page: a few are quicker asked about.
			last := page[len(page)-1].Key
			var rest []string
			for _, h := range want {
				if s.objectKey(h) > last {
					rest = append(rest, h)
				}
			}
			if len(rest) == 0 {
				return false
			}
			if len(rest) < listFrom {
				mu.Lock()
				ask = append(ask, rest...)
				mu.Unlock()
				return false
			}
			return true
		})
	})
	if err != nil {
		return nil, err
	}
	asked, err := s.missing(ask, s.objectKey)
	if err != nil {
		return nil, err
	}
	notAsked := map[string]bool{}
	for _, h := range ask {
		notAsked[h] = true
	}
	missing := asked
	for _, h := range dedupeHashes(hashes) {
		if !notAsked[h] && !present[s.objectKey(h)] {
			missing = append(missing, h)
		}
	}
	return missing, nil
}

// ContentsAsker is storage that says, for many objects at once, which a
// project lacks (the hosted service answers from its index).
type ContentsAsker interface {
	MissingContents(pid string, hashes []string) ([]string, error)
}

// askBatch is how many hashes go in one question.
const askBatch = 1000

func askMissing(a ContentsAsker, pid string, hashes []string) ([]string, error) {
	missing := []string{}
	for i := 0; i < len(hashes); i += askBatch {
		m, err := a.MissingContents(pid, hashes[i:min(i+askBatch, len(hashes))])
		if err != nil {
			return nil, err
		}
		missing = append(missing, m...)
	}
	return missing, nil
}

// listFrom: a folder holding this many of the wanted objects is listed.
const listFrom = 4

func dedupeHashes(hs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, h := range hs {
		if !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	return out
}

// PutObject uploads a blob. Its name is the SHA-256 of its contents, which
// is also the signed payload hash, so storage verifies the upload.
func (s *BucketBackend) PutObject(hash string, r io.Reader) error {
	if !validHex(hash, 64) {
		return fmt.Errorf("invalid object hash %q", hash)
	}
	size := int64(-1)
	switch v := r.(type) {
	case *os.File:
		if fi, err := v.Stat(); err == nil {
			size = fi.Size()
		}
	case interface{ Len() int }:
		size = int64(v.Len())
	case interface{ Size() int64 }: // e.g. a reader reporting progress; -1 if unknown
		size = v.Size()
	}
	if size < 0 {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		r, size = bytes.NewReader(data), int64(len(data))
	}
	return s.PutObjectBody(hash, r, size, hash)
}

// PutObjectBody uploads the bytes stored for the object hash: an encoded
// blob (see package blob) whose own SHA-256 is bodySHA, size bytes long.
func (s *BucketBackend) PutObjectBody(hash string, r io.Reader, size int64, bodySHA string) error {
	if !validHex(hash, 64) || !validHex(bodySHA, 64) {
		return fmt.Errorf("invalid object hash %q", hash)
	}
	return s.b.Put(s.objectKey(hash), r, size, bodySHA, "")
}

func (s *BucketBackend) MarkChunked(hash string) error {
	if !validHex(hash, 64) {
		return fmt.Errorf("invalid object hash %q", hash)
	}
	return s.put(s.objs+chunkedDir+hash, nil)
}

// GetObject reads a file's stored bytes; one a storage cleanup moved to the
// trash comes back from there (see CollectGarbage).
func (s *BucketBackend) GetObject(hash string) (io.ReadCloser, error) {
	if !validHex(hash, 64) {
		return nil, ErrNotFound
	}
	r, err := s.b.Open(s.objectKey(hash))
	if errors.Is(err, ErrNotFound) && s.fromTrash(s.objectKey(hash)) == nil {
		return s.b.Open(s.objectKey(hash))
	}
	return r, err
}

func (s *BucketBackend) PutWorkspace(pid, wsid string, state any) error {
	if !validHex(wsid, 32) {
		return errors.New("invalid workspace id")
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.put(workspaceKey(pid, wsid), data)
}

func (s *BucketBackend) Members() ([]Member, error) {
	out := []Member{}
	err := s.readAll("members/", func(_ string, data []byte) {
		var m Member
		if json.Unmarshal(data, &m) == nil && ValidMemberID(m.ID) {
			out = append(out, m)
		}
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, err
}

func (s *BucketBackend) PutMember(m Member) error {
	if !ValidMemberID(m.ID) {
		return errors.New("invalid member id")
	}
	data, _ := json.Marshal(m)
	return s.put(memberKey(m.ID), data)
}

func (s *BucketBackend) Workspaces(pid string, out any) error {
	docs := []json.RawMessage{}
	if err := s.readAll(projectDir(pid)+"workspaces/", func(_ string, data []byte) {
		if json.Valid(data) {
			docs = append(docs, data)
		}
	}); err != nil {
		return err
	}
	data, _ := json.Marshal(docs)
	return json.Unmarshal(data, out)
}

// Info reads the team name from team.json in the storage folder.
func (s *BucketBackend) Info() (TeamInfo, error) {
	var info TeamInfo
	data, err := s.get("team.json")
	if errors.Is(err, ErrNotFound) {
		return info, nil
	}
	if err != nil {
		return info, err
	}
	// A record that doesn't read is an error, not an empty one: written
	// back empty, it would lose the team's name and features.
	if err := json.Unmarshal(data, &info); err != nil {
		return info, fmt.Errorf("team.json: %w", err)
	}
	return info, nil
}

// SetInfo names the team (written by whoever sets up the storage).
func (s *BucketBackend) SetInfo(info TeamInfo) error {
	data, _ := json.Marshal(info)
	return s.put("team.json", data)
}

// SetupStore keeps what each member's computer has for the team's projects
// (Live versions, plugins, packs: names only), shared by members who chose
// to. A backend an extension adds may not keep them.
type SetupStore interface {
	PutSetup(memberID string, data []byte) error
	DeleteSetup(memberID string) error
	// Setups maps member id to the setup they shared.
	Setups() (map[string][]byte, error)
}

var _ SetupStore = (*BucketBackend)(nil)

const setupsDir = "setups/"

func (s *BucketBackend) PutSetup(memberID string, data []byte) error {
	if !ValidMemberID(memberID) {
		return errors.New("invalid member id")
	}
	return s.put(setupsDir+memberID+".json", data)
}

func (s *BucketBackend) DeleteSetup(memberID string) error {
	if !ValidMemberID(memberID) {
		return errors.New("invalid member id")
	}
	return s.delete(setupsDir + memberID + ".json")
}

func (s *BucketBackend) Setups() (map[string][]byte, error) {
	out := map[string][]byte{}
	err := s.readAll(setupsDir, func(key string, data []byte) {
		if id := strings.TrimSuffix(strings.TrimPrefix(key, setupsDir), ".json"); ValidMemberID(id) {
			out[id] = data
		}
	})
	return out, err
}

// GetPreparer is storage reached through presigned URLs that can ask for
// many at once (the hosted service): asked ahead, the downloads that follow
// don't each ask for their own.
type GetPreparer interface{ PrepareGets(keys []string) }

// Preparer is a backend that can get many downloads ready at once.
type Preparer interface {
	PrepareObjects(hashes []string)
	PrepareSnapshots(pid string, ids []string)
}

var _ Preparer = (*BucketBackend)(nil)

func (s *BucketBackend) PrepareObjects(hashes []string) {
	p, ok := s.b.(GetPreparer)
	if !ok || len(hashes) < 2 {
		return
	}
	keys := make([]string, len(hashes))
	for i, h := range hashes {
		keys[i] = s.objectKey(h)
	}
	p.PrepareGets(keys)
}

func (s *BucketBackend) PrepareSnapshots(pid string, ids []string) {
	p, ok := s.b.(GetPreparer)
	if !ok || len(ids) < 2 {
		return
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = snapshotKey(pid, id)
	}
	p.PrepareGets(keys)
}
