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
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	resp.Body.Close()
	return fmt.Errorf("%s: %d %s", what, resp.StatusCode, strings.TrimSpace(string(data)))
}

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
		resp, err := b.transfer("GET", func() (string, error) { return b.urls(pid, nil, rel) }, nil, 0, nil)
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

func (b *brokerBucket) Put(key string, r io.Reader, size int64, sum, cond string) error {
	pid, rel, isContents := contents(key)
	if !isContents {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		hdr := map[string]string{}
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
		}
		return failed(resp, "put "+key)
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
	resp, err := b.transfer("PUT", func() (string, error) { return b.urls(pid, item, "") }, rs, size,
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
