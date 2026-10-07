package remote

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeCloud stands in for the service (its /v1/teams/t/… calls) and for
// storage (/r2/…, where the URLs it hands out point).
type fakeCloud struct {
	*httptest.Server
	mu       sync.Mutex
	stored   map[string][]byte
	urlCalls atomic.Int32
	putTries atomic.Int32
	keyPuts  atomic.Int32
	asked    [][]string
	// storage answers a try (1, 2, …) of a PUT: true when it handled it.
	storage func(try int32, w http.ResponseWriter, r *http.Request) bool
	keyPut  func(w http.ResponseWriter)
}

const testPID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func newFakeCloud(t *testing.T) *fakeCloud {
	f := &fakeCloud{stored: map[string][]byte{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/teams/t/projects/"+testPID+"/urls", func(w http.ResponseWriter, r *http.Request) {
		f.urlCalls.Add(1)
		var req struct {
			Put []putItem
			Get []string
		}
		json.NewDecoder(r.Body).Decode(&req)
		out := map[string][]string{"put": {}, "get": {}}
		for _, p := range req.Put {
			out["put"] = append(out["put"], f.URL+"/r2/"+p.Key)
		}
		for _, g := range req.Get {
			out["get"] = append(out["get"], f.URL+"/r2/"+g)
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/v1/teams/t/projects/"+testPID+"/missing", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Hashes []string }
		json.NewDecoder(r.Body).Decode(&req)
		f.mu.Lock()
		f.asked = append(f.asked, req.Hashes)
		f.mu.Unlock()
		json.NewEncoder(w).Encode(map[string][]string{"missing": req.Hashes})
	})
	mux.HandleFunc("/v1/teams/t/keys/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			f.keyPuts.Add(1)
			if f.keyPut != nil {
				f.keyPut(w)
				return
			}
		}
		w.WriteHeader(200)
	})
	mux.HandleFunc("/r2/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/r2/")
		if r.Method == "PUT" {
			try := f.putTries.Add(1)
			if f.storage != nil && f.storage(try, w, r) {
				return
			}
			data, _ := io.ReadAll(r.Body)
			f.mu.Lock()
			f.stored[key] = data
			f.mu.Unlock()
			return
		}
		f.mu.Lock()
		data, ok := f.stored[key]
		f.mu.Unlock()
		if !ok {
			w.WriteHeader(404)
			return
		}
		w.Write(data)
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeCloud) bucket(t *testing.T) *brokerBucket {
	b, err := NewBroker(f.URL+"/v1/teams/t", "token")
	if err != nil {
		t.Fatal(err)
	}
	bb := b.(*brokerBucket)
	bb.backoff, bb.stall = time.Millisecond, 300*time.Millisecond
	return bb
}

func object(s string) (key string, data []byte, sum string) {
	data = []byte(s)
	h := sha256.Sum256(data)
	sum = hex.EncodeToString(h[:])
	return "projects/" + testPID + "/objects/" + sum[:2] + "/" + sum[2:], data, sum
}

func TestBrokerUploadRetriesWithFreshURLs(t *testing.T) {
	f := newFakeCloud(t)
	f.storage = func(try int32, w http.ResponseWriter, r *http.Request) bool {
		if try < 3 { // storage fails twice
			io.Copy(io.Discard, r.Body)
			w.WriteHeader(503)
			return true
		}
		return false
	}
	key, data, sum := object("the take")
	if err := f.bucket(t).Put(key, bytes.NewReader(data), int64(len(data)), sum, ""); err != nil {
		t.Fatal(err)
	}
	if f.putTries.Load() != 3 || f.urlCalls.Load() != 3 {
		t.Errorf("tries %d, URLs asked %d; want 3 and 3", f.putTries.Load(), f.urlCalls.Load())
	}
	if got := f.stored[strings.TrimPrefix(key, "projects/"+testPID+"/")]; !bytes.Equal(got, data) {
		t.Errorf("stored %q, want the whole body again on the try that worked", got)
	}
}

func TestBrokerUploadGivesUpOnAStall(t *testing.T) {
	f := newFakeCloud(t)
	f.storage = func(try int32, w http.ResponseWriter, r *http.Request) bool {
		if try == 1 { // reads nothing and answers nothing for a long while
			time.Sleep(3 * time.Second)
			return true
		}
		return false
	}
	key, data, sum := object("stuck once")
	start := time.Now()
	if err := f.bucket(t).Put(key, bytes.NewReader(data), int64(len(data)), sum, ""); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v: the stalled try wasn't given up", d)
	}
	if f.putTries.Load() != 2 {
		t.Errorf("tries %d, want 2", f.putTries.Load())
	}
}

func TestBrokerUploadThereAlready(t *testing.T) {
	f := newFakeCloud(t)
	f.storage = func(_ int32, w http.ResponseWriter, r *http.Request) bool {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(412)
		return true
	}
	key, data, sum := object("kept before")
	b := f.bucket(t)
	if err := b.Put(key, bytes.NewReader(data), int64(len(data)), sum, ""); err != nil {
		t.Errorf("an object there already: %v, want success", err)
	}
	if err := b.Put(key, bytes.NewReader(data), int64(len(data)), sum, "*"); !errors.Is(err, ErrPrecondition) {
		t.Errorf("only-if-absent on an object there: %v, want ErrPrecondition", err)
	}
}

// A write the service failed is tried again: one dropped answer mustn't
// leave a commit unshared.
func TestBrokerWritesOfKeysAreRepeated(t *testing.T) {
	f := newFakeCloud(t)
	f.keyPut = func(w http.ResponseWriter) {
		if f.keyPuts.Load() < 3 {
			w.WriteHeader(503)
		}
	}
	err := f.bucket(t).Put("projects/"+testPID+"/branches/main", strings.NewReader("v"), 1, "", "*")
	if err != nil || f.keyPuts.Load() != 3 {
		t.Errorf("err %v after %d tries; want it to work at the third", err, f.keyPuts.Load())
	}
}

func TestBrokerSessionAndAccessErrors(t *testing.T) {
	for _, c := range []struct {
		status int
		want   error
	}{{401, ErrSignedOut}, {403, ErrForbidden}} {
		f := newFakeCloud(t)
		f.keyPut = func(w http.ResponseWriter) { w.WriteHeader(c.status) }
		err := f.bucket(t).Put("team.json", strings.NewReader("{}"), 2, "", "")
		if !errors.Is(err, c.want) {
			t.Errorf("%d: %v, want %v", c.status, err, c.want)
		}
	}
}

func TestBrokerAsksAboutContentsInBatches(t *testing.T) {
	f := newFakeCloud(t)
	backend := ForProject(NewBucketBackend(f.bucket(t)), testPID)
	var hashes []string
	for i := 0; i < 2500; i++ {
		hashes = append(hashes, fmt.Sprintf("%064x", i))
	}
	hashes = append(hashes, hashes[0]) // asked twice
	missing, err := backend.MissingObjects(hashes)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 2500 {
		t.Errorf("missing %d, want 2500", len(missing))
	}
	if len(f.asked) != 3 || len(f.asked[0]) != 1000 || len(f.asked[2]) != 500 {
		sizes := []int{}
		for _, a := range f.asked {
			sizes = append(sizes, len(a))
		}
		t.Errorf("questions of %v hashes, want 1000, 1000, 500", sizes)
	}
}

func TestBrokerDownloadGivesUpOnAStall(t *testing.T) {
	b := newFakeCloud(t).bucket(t)
	// Storage sends the headers and a few bytes, then nothing.
	stalled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-length", "1000")
		w.Write([]byte("a l"))
		w.(http.Flusher).Flush()
		time.Sleep(3 * time.Second)
	}))
	defer stalled.Close()
	resp, err := b.transfer("GET", func() (string, error) { return stalled.URL, nil }, nil, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err == nil || time.Since(start) > 2*time.Second {
		t.Errorf("a stalled download: %v after %v; want an error within the stall time", err, time.Since(start))
	}
}

func TestBrokerReadsAreRetried(t *testing.T) {
	var tries atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tries.Add(1) < 3 {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("etag", `"e1"`)
		w.Write([]byte(`{"name":"Band"}`))
	}))
	defer srv.Close()
	bb, _ := NewBroker(srv.URL+"/v1/teams/t", "token")
	b := bb.(*brokerBucket)
	b.backoff = time.Millisecond
	data, etag, err := b.Get("team.json")
	if err != nil || string(data) != `{"name":"Band"}` || etag != `"e1"` || tries.Load() != 3 {
		t.Errorf("Get = %q %q %v after %d tries", data, etag, err, tries.Load())
	}
}

func TestBrokerUploadOfUnknownSum(t *testing.T) {
	f := newFakeCloud(t)
	var header string
	f.storage = func(_ int32, w http.ResponseWriter, r *http.Request) bool {
		header = r.Header.Get("x-amz-checksum-sha256")
		return false
	}
	// A version record: written without its sum, from a plain reader.
	data := []byte(`{"version":1}`)
	s := sha256.Sum256(data)
	id := hex.EncodeToString(s[:])
	key := "projects/" + testPID + "/snapshots/" + id + ".json"
	if err := f.bucket(t).Put(key, io.MultiReader(bytes.NewReader(data)), -1, "", ""); err != nil {
		t.Fatal(err)
	}
	if got := f.stored["snapshots/"+id+".json"]; !bytes.Equal(got, data) {
		t.Errorf("stored %q", got)
	}
	if header != base64.StdEncoding.EncodeToString(s[:]) {
		t.Errorf("checksum sent %q, want the record's own", header)
	}
}

func TestBrokerDownloadRoundTrip(t *testing.T) {
	f := newFakeCloud(t)
	b := f.bucket(t)
	key, data, sum := object("bounce")
	if err := b.Put(key, bytes.NewReader(data), int64(len(data)), sum, ""); err != nil {
		t.Fatal(err)
	}
	got, _, err := b.Get(key)
	if err != nil || !bytes.Equal(got, data) {
		t.Errorf("Get = %q, %v", got, err)
	}
	other, _, _ := object("never stored")
	if _, _, err := b.Get(other); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get of an unknown object: %v, want ErrNotFound", err)
	}
}

// An empty file goes up with "content-length: 0", as its URL is signed for:
// sent chunked (no length), storage refuses the signature.
func TestBrokerUploadOfAnEmptyFile(t *testing.T) {
	f := newFakeCloud(t)
	f.storage = func(_ int32, w http.ResponseWriter, r *http.Request) bool {
		if r.ContentLength != 0 || len(r.TransferEncoding) != 0 || r.Header.Get("content-length") == "" && r.ContentLength != 0 {
			w.WriteHeader(403)
			w.Write([]byte(`<?xml version="1.0"?><Error><Code>SignatureDoesNotMatch</Code><StringToSign>X-Amz-Credential=AKIA</StringToSign></Error>`))
			return true
		}
		return false
	}
	key := "projects/" + testPID + "/chunked/" + strings.Repeat("ab", 32)
	if err := f.bucket(t).Put(key, bytes.NewReader(nil), 0, "", ""); err != nil {
		t.Fatal(err)
	}
}

// Storage's XML errors are cut to their code: the rest echoes the signed
// request.
func TestBrokerStorageErrorsSayTheirCode(t *testing.T) {
	f := newFakeCloud(t)
	f.storage = func(_ int32, w http.ResponseWriter, _ *http.Request) bool {
		w.WriteHeader(403)
		w.Write([]byte(`<?xml version="1.0"?><Error><Code>SignatureDoesNotMatch</Code><CanonicalRequest>X-Amz-Credential=AKIA</CanonicalRequest></Error>`))
		return true
	}
	key, data, sum := object("x")
	err := f.bucket(t).Put(key, bytes.NewReader(data), int64(len(data)), sum, "")
	if err == nil || !strings.Contains(err.Error(), "403 SignatureDoesNotMatch") || strings.Contains(err.Error(), "AKIA") {
		t.Errorf("error %v", err)
	}
}
