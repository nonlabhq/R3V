package remote

import (
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Reading many small keys at once (R3V-Cloud's docs/api.md, "Reading many
// small keys"): a project's branch records, milestones, members, the
// projects' records come in one request or a few instead of a listing and
// a GET apiece (each a request the service bills and answers on its own).

// ValueReader is storage that reads many small keys in a request or a few.
// Either way the answer is the same as listing and reading them one by one
// (an older service: it reads them so).
type ValueReader interface {
	// ListValues calls fn with each key under prefix after startAfter, in
	// key order, and its bytes.
	ListValues(prefix, startAfter string, fn func(key string, data []byte)) error
	// ReadKeys reads keys; those absent (or not the caller's to read) are
	// left out.
	ReadKeys(keys []string) (map[string][]byte, error)
}

var _ ValueReader = (*brokerBucket)(nil)

// readBatch is how many keys one keys/read asks for (the service's most).
const readBatch = 1000

// noBulkRead: services found answering keys/read with 404 (older ones), by
// address, and when: not asked again for a while (each Open is a new
// bucket; asking every time would be a wasted request apiece).
var noBulkRead = struct {
	sync.Mutex
	since map[string]time.Time
}{since: map[string]time.Time{}}

const noBulkReadFor = time.Hour

func (b *brokerBucket) bulkReads() bool {
	noBulkRead.Lock()
	defer noBulkRead.Unlock()
	at, ok := noBulkRead.since[b.base]
	return !ok || time.Since(at) > noBulkReadFor
}

func (b *brokerBucket) ReadKeys(keys []string) (map[string][]byte, error) {
	out := map[string][]byte{}
	var one []string // read one by one
	if !b.bulkReads() {
		one = keys
		keys = nil
	}
	for len(keys) > 0 {
		n := min(len(keys), readBatch)
		batch := keys[:n]
		keys = keys[n:]
		var resp struct {
			Items []struct {
				Key     string  `json:"key"`
				Value   *string `json:"value"`
				Missing bool    `json:"missing"`
				Later   bool    `json:"later"`
			} `json:"items"`
		}
		err := b.json("/keys/read", map[string]any{"keys": batch}, &resp)
		if errors.Is(err, ErrNotFound) {
			noBulkRead.Lock()
			noBulkRead.since[b.base] = time.Now()
			noBulkRead.Unlock()
			one = append(one, batch...)
			one = append(one, keys...)
			break
		}
		if err != nil {
			return nil, err
		}
		answered := map[string]bool{}
		var later []string
		for _, it := range resp.Items {
			switch {
			case it.Value != nil:
				out[it.Key] = []byte(*it.Value)
			case it.Later:
				later = append(later, it.Key)
			case !it.Missing:
				continue // (an answer this build doesn't know: read it alone)
			}
			answered[it.Key] = true
		}
		for _, k := range batch {
			if !answered[k] {
				one = append(one, k)
			}
		}
		if len(later) == len(batch) {
			// Nothing came back but "later": read them one by one rather
			// than ask the same again.
			one = append(one, later...)
		} else {
			keys = append(later, keys...)
		}
	}
	if len(one) > 0 {
		var mu sync.Mutex
		err := parallel(one, func(k string) error {
			data, _, err := b.Get(k)
			if errors.Is(err, ErrNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			mu.Lock()
			out[k] = data
			mu.Unlock()
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (b *brokerBucket) ListValues(prefix, startAfter string, fn func(key string, data []byte)) error {
	after := startAfter
	for {
		q := url.Values{"prefix": {prefix}, "after": {after}, "values": {"1"}}
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
		var page struct {
			Items []struct {
				Key    string  `json:"key"`
				Value  *string `json:"value"`
				Binary bool    `json:"binary"`
			} `json:"items"`
			More bool `json:"more"`
		}
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if len(page.Items) == 0 {
			return nil
		}
		// Those without a value (pictures; everything, from an older
		// service, which leaves values out) are read one by one.
		got := map[string][]byte{}
		var one []string
		for _, it := range page.Items {
			if it.Value != nil && !it.Binary {
				got[it.Key] = []byte(*it.Value)
			} else {
				one = append(one, it.Key)
			}
		}
		var mu sync.Mutex
		if err := parallel(one, func(k string) error {
			data, _, err := b.Get(k)
			if errors.Is(err, ErrNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			mu.Lock()
			got[k] = data
			mu.Unlock()
			return nil
		}); err != nil {
			return err
		}
		for _, it := range page.Items {
			if data, ok := got[it.Key]; ok {
				fn(it.Key, data)
			}
		}
		if !page.More {
			return nil
		}
		after = page.Items[len(page.Items)-1].Key
	}
}

// readKeys reads small keys (records), those absent left out: in a request
// or a few where storage can (ValueReader), else a GET apiece.
func (s *BucketBackend) readKeys(keys []string) (map[string][]byte, error) {
	if vr, ok := s.b.(ValueReader); ok {
		return vr.ReadKeys(keys)
	}
	var mu sync.Mutex
	out := map[string][]byte{}
	err := parallel(keys, func(k string) error {
		data, err := s.get(k)
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		mu.Lock()
		out[k] = data
		mu.Unlock()
		return nil
	})
	return out, err
}

// readDir reads every small key under dir after startAfter (records), as
// readKeys does.
func (s *BucketBackend) readDir(dir, startAfter string) (map[string][]byte, error) {
	if vr, ok := s.b.(ValueReader); ok {
		out := map[string][]byte{}
		err := vr.ListValues(dir, startAfter, func(k string, data []byte) { out[k] = data })
		return out, err
	}
	var keys []string
	err := s.b.List(dir, startAfter, func(page []Item) bool {
		for _, it := range page {
			keys = append(keys, it.Key)
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return s.readKeys(keys)
}

// sortedKeys is m's keys in order (records read at once, handed on in the
// order a listing has them).
func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// projectRecordKeys: the project.json of each project folder ("projects/<pid>/").
func projectRecordKeys(dirs []string) []string {
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if strings.HasSuffix(d, "/") {
			out = append(out, d+"project.json")
		}
	}
	return out
}
