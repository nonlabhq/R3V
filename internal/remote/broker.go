package remote

// brokerBucket reaches a team kept by the hosted service (R3V-Cloud): the
// keys that change go through the service, which keeps them and checks who
// may do what; contents go straight to and from storage through presigned
// URLs the service hands out, and are kept per project (an upload can't be
// checked against its name, so only a project's own writers may write its
// contents).
//
// Address: r3v-cloud+https://<host>/v1/teams/<team>; the session comes from
// SessionToken. Only the Nightly build opens such addresses
// (broker_nightly.go).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// SessionToken finds the session for a service (https://<host>): signing
// in (package cloud) keeps it in the system's credential store and sets
// this. R3V_CLOUD_TOKEN stands in for it in tests.
var SessionToken = func(service string) string { return os.Getenv("R3V_CLOUD_TOKEN") }

// HostedTeams: hosted teams are on in this build (the Nightly channel, see
// broker_nightly.go). Off, a hosted team's address is no team's: nothing
// reaches the service (sign-in, the account's teams, live notices) even with
// the settings and the session Nightly left (both channels share them).
var HostedTeams = false

// BrokerService is the service a hosted team's address is at (https://<host>);
// false in a build without hosted teams.
func BrokerService(address string) (string, bool) {
	rest, ok := strings.CutPrefix(address, "r3v-cloud+")
	if !ok || !HostedTeams {
		return "", false
	}
	u, err := url.Parse(rest)
	if err != nil || u.Host == "" {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}

// withoutAddress keeps a presigned address out of an error: its signature
// is a key to the storage for a while, and errors are shown and logged.
// (Still a *url.Error: whether to try again is told the same way.)
func withoutAddress(err error) error {
	if ue, ok := err.(*url.Error); ok {
		c := *ue
		c.URL = "(storage address)"
		return &c
	}
	return err
}

// openBroker opens a hosted team's address.
func openBroker(cfg Config) (Backend, error) {
	service, _ := BrokerService(cfg.URL)
	b, err := NewBroker(cfg.URL[len("r3v-cloud+"):], firstNonEmpty(cfg.SecretKey, SessionToken(service)))
	if err != nil {
		return nil, err
	}
	return NewBucketBackend(b), nil
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

var (
	// ErrForbidden: the service doesn't let this person do that.
	ErrForbidden = errors.New("not allowed for you in this team")
	// ErrSignedOut: the session ended (or never was): sign in again.
	ErrSignedOut = errors.New("signed out of R3V-Cloud: sign in again")
)

type brokerBucket struct {
	base  string // https://<host>/v1/teams/<team>
	token string
	http  *http.Client // presigned transfers (long; watched for stalls instead)
	api   *http.Client // the service's own calls (short, with a timeout)
	// Transfers: how many tries, the first wait between them, and how long
	// a transfer may go without progress.
	tries   int
	backoff time.Duration
	stall   time.Duration

	// Download URLs asked for ahead, many in one request (PrepareGets):
	// each used once, by the first try of its download.
	prepMu   sync.Mutex
	prepared map[string]preparedURL
	// Upload URLs asked for ahead (PreparePuts), each for the size and
	// SHA-256 it was signed for.
	preparedPuts map[string]preparedURL
}

type preparedURL struct {
	url   string
	until time.Time
	sum   string // an upload's SHA-256 (base64) and size
	size  int64
}

// PrepareGets asks for the download URLs of contents keys, up to 1,000 a
// request, so the downloads that follow don't each ask for their own (a
// request to the service apiece). Best effort: a key not prepared asks
// for its URL as before.
func (b *brokerBucket) PrepareGets(keys []string) {
	byPID := map[string][]string{}
	for _, k := range keys {
		if pid, _, ok := contents(k); ok {
			byPID[pid] = append(byPID[pid], k)
		}
	}
	for pid, ks := range byPID {
		for len(ks) > 0 {
			n := min(len(ks), 1000)
			batch := ks[:n]
			ks = ks[n:]
			rels := make([]string, len(batch))
			for i, k := range batch {
				_, rels[i], _ = contents(k)
			}
			var out struct{ Get []string }
			if b.json("/projects/"+pid+"/urls", map[string]any{"get": rels}, &out) != nil || len(out.Get) != len(batch) {
				continue
			}
			// (valid 15 minutes: used within 12)
			until := time.Now().Add(12 * time.Minute)
			b.prepMu.Lock()
			if b.prepared == nil {
				b.prepared = map[string]preparedURL{}
			}
			for i, k := range batch {
				b.prepared[k] = preparedURL{url: out.Get[i], until: until}
			}
			b.prepMu.Unlock()
		}
	}
}

// PreparePuts asks for the upload URLs of contents, up to 1,000 a request,
// so the uploads that follow don't each ask for their own. Best effort, as
// PrepareGets.
func (b *brokerBucket) PreparePuts(items []PutPrep) {
	byPID := map[string][]PutPrep{}
	for _, it := range items {
		if pid, _, ok := contents(it.Key); ok {
			byPID[pid] = append(byPID[pid], it)
		}
	}
	for pid, its := range byPID {
		for len(its) > 0 {
			n := min(len(its), 1000)
			batch := its[:n]
			its = its[n:]
			put := make([]putItem, 0, len(batch))
			for _, it := range batch {
				raw, err := hex.DecodeString(it.SHA256)
				if err != nil || len(raw) != 32 {
					return
				}
				_, rel, _ := contents(it.Key)
				put = append(put, putItem{rel, it.Size, base64.StdEncoding.EncodeToString(raw)})
			}
			var out struct{ Put []string }
			if b.json("/projects/"+pid+"/urls", map[string]any{"put": put}, &out) != nil || len(out.Put) != len(batch) {
				continue
			}
			until := time.Now().Add(12 * time.Minute)
			b.prepMu.Lock()
			if b.preparedPuts == nil {
				b.preparedPuts = map[string]preparedURL{}
			}
			for i, it := range batch {
				b.preparedPuts[it.Key] = preparedURL{out.Put[i], until, put[i].SHA256, it.Size}
			}
			b.prepMu.Unlock()
		}
	}
}

// preparedPut takes key's prepared upload URL, if there is one still good
// for these bytes.
func (b *brokerBucket) preparedPut(key, sum string, size int64) string {
	b.prepMu.Lock()
	defer b.prepMu.Unlock()
	p, ok := b.preparedPuts[key]
	delete(b.preparedPuts, key)
	if !ok || time.Now().After(p.until) || p.sum != sum || p.size != size {
		return ""
	}
	return p.url
}

// preparedGet takes key's prepared URL, if there is one still good.
func (b *brokerBucket) preparedGet(key string) string {
	b.prepMu.Lock()
	defer b.prepMu.Unlock()
	p, ok := b.prepared[key]
	delete(b.prepared, key)
	if !ok || time.Now().After(p.until) {
		return ""
	}
	return p.url
}

// NewBroker reaches the team at base (https://<host>/v1/teams/<team>) as
// the person whose session token is token.
func NewBroker(base, token string) (Bucket, error) {
	if token == "" {
		return nil, ErrSignedOut
	}
	return &brokerBucket{
		base: strings.TrimRight(base, "/"), token: token, http: &http.Client{}, api: &http.Client{Timeout: time.Minute},
		tries: 4, backoff: time.Second, stall: 20 * time.Second,
	}, nil
}

// ContentsPerProject: the service keeps each project's contents apart.
func (b *brokerBucket) ContentsPerProject() bool { return true }

// KeepsLooks: the service keeps members' and projects' looks in their
// records, and members' pictures (LooksKeeper).
func (b *brokerBucket) KeepsLooks() bool { return true }

// KeepsBranchRecords: the service keeps projects' branch records
// (branchinfo/) and milestones, and tells of their changes.
func (b *brokerBucket) KeepsBranchRecords() bool { return true }

var brokerContents = regexp.MustCompile(`^projects/([0-9a-f]{32})/((?:objects|chunked|snapshots)/.+)$`)

// contents splits a key kept in storage (handed out as URLs) into its
// project and its key there; ok is false for the keys the service keeps.
func contents(key string) (pid, rel string, ok bool) {
	m := brokerContents.FindStringSubmatch(key)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// --- the service's own calls -------------------------------------------------

// call sends a request to the service, tried again when the service or the
// network fails. Writes of small keys are tried again too, as storage
// teams' are: a conditional write that got through but whose answer was
// lost comes back as a conflict, which the caller looks into (failing at
// once left a commit unshared over one dropped connection).
func (b *brokerBucket) call(method, path string, body []byte, hdr map[string]string) (*http.Response, error) {
	var last error
	for try := 0; try < b.tries; try++ {
		if try > 0 {
			time.Sleep(b.backoff << (try - 1))
		}
		var r io.Reader
		if body != nil {
			r = bytes.NewReader(body)
		}
		req, err := http.NewRequest(method, b.base+path, r)
		if err != nil {
			return nil, err
		}
		req.Header.Set("authorization", "Bearer "+b.token)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := b.api.Do(req)
		if err != nil {
			last = err
			continue
		}
		switch {
		case resp.StatusCode == 401:
			resp.Body.Close()
			return nil, ErrSignedOut
		case resp.StatusCode == 403:
			resp.Body.Close()
			return nil, fmt.Errorf("%w (%s %s)", ErrForbidden, method, path)
		case resp.StatusCode >= 500 || resp.StatusCode == 429:
			last = failed(resp, method+" "+path)
			continue
		}
		return resp, nil
	}
	return nil, last
}

func (b *brokerBucket) json(path string, in, out any) error {
	data, _ := json.Marshal(in)
	resp, err := b.call("POST", path, data, map[string]string{"content-type": "application/json"})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return ErrNotFound
	}
	if resp.StatusCode != 200 {
		return failed(resp, path)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func keyPath(key string) string { return "/keys/" + url.PathEscape(key) }

func failed(resp *http.Response, what string) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	text := strings.TrimSpace(string(data))
	// Storage's errors (XML) say just their code: the rest echoes the
	// request signed, which isn't for showing or logging.
	if strings.HasPrefix(text, "<") {
		text = ""
		if m := xmlCode.FindStringSubmatch(string(data)); m != nil {
			text = m[1]
		}
	}
	return fmt.Errorf("%s: %d %s", what, resp.StatusCode, text)
}

var xmlCode = regexp.MustCompile(`<Code>([A-Za-z0-9.]+)</Code>`)

type putItem struct {
	Key    string `json:"key"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// urls asks for one presigned URL (an upload when put is set).
func (b *brokerBucket) urls(pid string, put *putItem, get string) (string, error) {
	var req struct {
		Put []putItem `json:"put,omitempty"`
		Get []string  `json:"get,omitempty"`
	}
	if put != nil {
		req.Put = []putItem{*put}
	} else {
		req.Get = []string{get}
	}
	var out struct{ Put, Get []string }
	if err := b.json("/projects/"+pid+"/urls", req, &out); err != nil {
		return "", err
	}
	if put != nil && len(out.Put) == 1 {
		return out.Put[0], nil
	}
	if put == nil && len(out.Get) == 1 {
		return out.Get[0], nil
	}
	return "", errors.New("urls: an unexpected answer")
}

// MissingContents asks the service which of up to 1,000 of a project's
// objects it lacks (ContentsAsker).
func (b *brokerBucket) MissingContents(pid string, hashes []string) ([]string, error) {
	var out struct{ Missing []string }
	if err := b.json("/projects/"+pid+"/missing", map[string]any{"hashes": hashes}, &out); err != nil {
		return nil, err
	}
	return out.Missing, nil
}

// --- transfers straight to storage --------------------------------------------

// transfer sends a presigned request (a fresh URL each try), retrying 5xx,
// 429 and failures with backoff, and giving up on a try that makes no
// progress for b.stall. body is nil or rewindable. The answer's body is
// watched too, until the caller closes it.
func (b *brokerBucket) transfer(method string, newURL func() (string, error), body io.ReadSeeker, size int64, hdr map[string]string) (*http.Response, error) {
	var last error
	for try := 0; try < b.tries; try++ {
		if try > 0 {
			time.Sleep(b.backoff << (try - 1))
		}
		u, err := newURL()
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithCancel(context.Background())
		dog := time.AfterFunc(b.stall, cancel)
		var r io.Reader
		if body != nil {
			if _, err := body.Seek(0, io.SeekStart); err != nil {
				dog.Stop()
				cancel()
				return nil, err
			}
			r = &progress{body, dog, b.stall}
		}
		req, err := http.NewRequestWithContext(ctx, method, u, r)
		if err != nil {
			dog.Stop()
			cancel()
			return nil, err
		}
		if body != nil {
			req.ContentLength = size
			if size == 0 {
				// (an empty body of unknown length would go chunked, without
				// the content-length the URL is signed for)
				req.Body = http.NoBody
			}
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := b.http.Do(req)
		if err != nil {
			dog.Stop()
			if ctx.Err() != nil {
				err = fmt.Errorf("no progress for %v", b.stall)
			}
			cancel()
			last = fmt.Errorf("%s: %w", method, withoutAddress(err))
			continue
		}
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			dog.Stop()
			last = failed(resp, method)
			cancel()
			continue
		}
		dog.Reset(b.stall)
		resp.Body = &watched{resp.Body, dog, b.stall, cancel}
		return resp, nil
	}
	return nil, last
}

// progress pushes the stall timer back whenever bytes move.
type progress struct {
	r     io.Reader
	dog   *time.Timer
	stall time.Duration
}

func (p *progress) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	if n > 0 {
		p.dog.Reset(p.stall)
	}
	return n, err
}

type watched struct {
	io.ReadCloser
	dog    *time.Timer
	stall  time.Duration
	cancel context.CancelFunc
}

func (w *watched) Read(buf []byte) (int, error) {
	n, err := w.ReadCloser.Read(buf)
	if n > 0 {
		w.dog.Reset(w.stall)
	}
	return n, err
}

func (w *watched) Close() error {
	w.dog.Stop()
	defer w.cancel()
	return w.ReadCloser.Close()
}

// --- Bucket -------------------------------------------------------------------------

func (b *brokerBucket) open(key string) (*http.Response, error) {
	if pid, rel, ok := contents(key); ok {
		first := b.preparedGet(key)
		resp, err := b.transfer("GET", func() (string, error) {
			if u := first; u != "" {
				first = ""
				return u, nil
			}
			return b.urls(pid, nil, rel)
		}, nil, 0, nil)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 404 {
			resp.Body.Close()
			return nil, ErrNotFound
		}
		if resp.StatusCode != 200 {
			return nil, failed(resp, "download "+key)
		}
		return resp, nil
	}
	resp, err := b.call("GET", keyPath(key), nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		resp.Body.Close()
		return nil, ErrNotFound
	}
	if resp.StatusCode != 200 {
		return nil, failed(resp, "get "+key)
	}
	return resp, nil
}

func (b *brokerBucket) Get(key string) ([]byte, string, error) {
	resp, err := b.open(key)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.Header.Get("etag"), err
}

func (b *brokerBucket) Open(key string) (io.ReadCloser, error) {
	resp, err := b.open(key)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (b *brokerBucket) Exists(key string) (bool, error) {
	resp, err := b.call("HEAD", keyPath(key), nil, nil)
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	switch resp.StatusCode {
	case 200:
		return true, nil
	case 404:
		return false, nil
	}
	return false, fmt.Errorf("exists %s: %d", key, resp.StatusCode)
}

// KeepsLocks: the service keeps the team's file locks.
func (b *brokerBucket) KeepsLocks() bool { return true }

var brokerBranch = regexp.MustCompile(`^projects/[0-9a-f]{32}/branches/[^/]+$`)

// branchBody is a branch move as the service takes it: the head, and the
// paths the move's new versions change (checked against file locks).
const branchBody = "application/vnd.r3v.branch+json"

// PutBranch moves a branch, saying which paths its new versions change.
// Always this form, locks on or off (the service refuses the plain one
// once a team's locks are on).
func (b *brokerBucket) PutBranch(key, head string, changed []string, cond string) error {
	if changed == nil {
		changed = []string{}
	}
	data, _ := json.Marshal(map[string]any{"head": head, "changed": changed})
	return b.putKey(key, data, cond, branchBody)
}

// putKey writes a key the service keeps; cond as in Put.
func (b *brokerBucket) putKey(key string, data []byte, cond, ctype string) error {
	hdr := map[string]string{}
	if ctype != "" {
		hdr["content-type"] = ctype
	}
	switch cond {
	case "":
	case "*":
		hdr["if-none-match"] = "*"
	default:
		hdr["if-match"] = cond
	}
	resp, err := b.call("PUT", keyPath(key), data, hdr)
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case 200:
		resp.Body.Close()
		return nil
	case 412:
		resp.Body.Close()
		return ErrPrecondition
	case 404:
		resp.Body.Close()
		return ErrNotFound
	case 409:
		if ctype == branchBody {
			return refused(resp, "put "+key)
		}
	}
	return failed(resp, "put "+key)
}

// refused reads a branch move's 409: someone else holds a path the move
// changes (*ErrLocked), or the team needs a newer R3V.
func refused(resp *http.Response, what string) error {
	defer resp.Body.Close()
	var e struct {
		Error struct {
			Code    string
			Message string
			Locks   []LockHolder
		}
	}
	json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&e)
	switch e.Error.Code {
	case "locked":
		return &ErrLocked{Locks: e.Error.Locks}
	case "update_r3v":
		return ErrUpdateR3V
	}
	return fmt.Errorf("%s: 409 %s", what, e.Error.Message)
}

func (b *brokerBucket) Put(key string, r io.Reader, size int64, sum, cond string) error {
	pid, rel, isContents := contents(key)
	if !isContents {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		if brokerBranch.MatchString(key) {
			return b.PutBranch(key, strings.TrimSpace(string(data)), nil, cond)
		}
		return b.putKey(key, data, cond, "")
	}
	// Contents are written once, with their SHA-256 (the URL demands both),
	// from something that can be read again for a retry.
	rs, ok := r.(io.ReadSeeker)
	if !ok || sum == "" {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		if sum == "" {
			s := sha256.Sum256(data)
			sum = hex.EncodeToString(s[:])
		}
		rs, size = bytes.NewReader(data), int64(len(data))
	}
	raw, err := hex.DecodeString(sum)
	if err != nil || len(raw) != 32 {
		return fmt.Errorf("put %s: invalid SHA-256", key)
	}
	b64 := base64.StdEncoding.EncodeToString(raw)
	item := &putItem{rel, size, b64}
	first := b.preparedPut(key, b64, size)
	resp, err := b.transfer("PUT", func() (string, error) {
		if u := first; u != "" {
			first = ""
			return u, nil
		}
		return b.urls(pid, item, "")
	}, rs, size,
		map[string]string{"if-none-match": "*", "x-amz-checksum-sha256": b64})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200:
		return nil
	case 412: // there already (or an earlier try got through): named by its hash
		if cond == "*" {
			return ErrPrecondition
		}
		return nil
	}
	return failed(resp, "upload "+key)
}

// Copy is for storage cleanup's trash, which for hosted teams goes project
// by project through the service's cleanup calls (to come).
func (b *brokerBucket) Copy(src, dst string) error {
	return errors.New("copying within R3V-Cloud's storage isn't supported yet")
}

func (b *brokerBucket) Delete(key, cond string) error {
	hdr := map[string]string{}
	if cond != "" {
		hdr["if-match"] = cond
	}
	resp, err := b.call("DELETE", keyPath(key), nil, hdr)
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case 200:
		resp.Body.Close()
		return nil
	case 412:
		resp.Body.Close()
		return ErrPrecondition
	case 404:
		resp.Body.Close()
		return ErrNotFound
	}
	return failed(resp, "delete "+key)
}

func (b *brokerBucket) List(prefix, startAfter string, page func([]Item) bool) error {
	after := startAfter
	for {
		q := url.Values{"prefix": {prefix}, "after": {after}}
		resp, err := b.call("GET", "/keys?"+q.Encode(), nil, nil)
		if err != nil {
			return err
		}
		if resp.StatusCode == 404 {
			resp.Body.Close()
			return ErrNotFound
		}
		if resp.StatusCode != 200 {
			return failed(resp, "list "+prefix)
		}
		var out struct {
			Items []struct {
				Key      string `json:"key"`
				Size     int64  `json:"size"`
				Modified string `json:"modified"`
			} `json:"items"`
			More bool `json:"more"`
		}
		err = json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if len(out.Items) == 0 {
			return nil
		}
		items := make([]Item, len(out.Items))
		for i, it := range out.Items {
			t, _ := time.Parse(time.RFC3339, it.Modified)
			items[i] = Item{Key: it.Key, Size: it.Size, Modified: t}
		}
		if !page(items) || !out.More {
			return nil
		}
		after = items[len(items)-1].Key
	}
}

func (b *brokerBucket) Folders(dir string) ([]string, error) {
	resp, err := b.call("GET", "/folders?"+url.Values{"dir": {dir}}.Encode(), nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, failed(resp, "folders "+dir)
	}
	defer resp.Body.Close()
	var out []string
	return out, json.NewDecoder(resp.Body).Decode(&out)
}
