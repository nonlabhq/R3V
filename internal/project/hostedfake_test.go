//go:build nightly

package project

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// newHostedFake stands in for R3V-Cloud for tests in this package: its
// small keys (/keys, with conditional writes), folders, the presigned URLs
// it hands out and the storage they point at (/r2/), and /missing. It
// returns the team's address (to SetRemote) once the session token is set.
func newHostedFake(t *testing.T) string {
	t.Helper()
	var mu sync.Mutex
	kv := map[string][]byte{}
	tags := map[string]int{}
	n := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ep := r.URL.EscapedPath()
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		if rest, ok := strings.CutPrefix(ep, "/r2/"); ok {
			key, _ := url.PathUnescape(rest)
			if r.Method == "PUT" {
				if _, there := kv[key]; there && r.Header.Get("if-none-match") == "*" {
					w.WriteHeader(412)
					return
				}
				kv[key] = body
				return
			}
			data, there := kv[key]
			if !there {
				w.WriteHeader(404)
				return
			}
			w.Write(data)
			return
		}
		rest, ok := strings.CutPrefix(ep, "/v1/teams/t")
		if !ok {
			w.WriteHeader(404)
			return
		}
		switch {
		case strings.HasPrefix(rest, "/keys/"):
			k, _ := url.PathUnescape(strings.TrimPrefix(rest, "/keys/"))
			data, there := kv[k]
			tag := fmt.Sprintf(`"%d"`, tags[k])
			switch r.Method {
			case "GET", "HEAD":
				if !there {
					w.WriteHeader(404)
					return
				}
				w.Header().Set("etag", tag)
				if r.Method == "GET" {
					w.Write(data)
				}
			case "PUT":
				if c := r.Header.Get("if-none-match"); c == "*" && there {
					w.WriteHeader(412)
					return
				}
				if c := r.Header.Get("if-match"); c != "" && (!there || c != tag) {
					w.WriteHeader(412)
					return
				}
				kv[k] = body
				n++
				tags[k] = n
			case "DELETE":
				if !there {
					w.WriteHeader(404)
					return
				}
				delete(kv, k)
			}
		case rest == "/keys":
			prefix, after := r.URL.Query().Get("prefix"), r.URL.Query().Get("after")
			var keys []string
			for k := range kv {
				if strings.HasPrefix(k, prefix) && k > after {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			type item struct {
				Key      string `json:"key"`
				Size     int64  `json:"size"`
				Modified string `json:"modified"`
			}
			out := struct {
				Items []item `json:"items"`
				More  bool   `json:"more"`
			}{Items: []item{}}
			for _, k := range keys {
				out.Items = append(out.Items, item{k, int64(len(kv[k])), time.Now().UTC().Format(time.RFC3339)})
			}
			json.NewEncoder(w).Encode(out)
		case rest == "/folders":
			dir := r.URL.Query().Get("dir")
			seen := map[string]bool{}
			out := []string{}
			for k := range kv {
				if after, ok := strings.CutPrefix(k, dir); ok {
					if i := strings.Index(after, "/"); i > 0 && !seen[after[:i]] {
						seen[after[:i]] = true
						out = append(out, dir+after[:i+1])
					}
				}
			}
			json.NewEncoder(w).Encode(out)
		case strings.HasSuffix(rest, "/urls"):
			pid := strings.TrimSuffix(strings.TrimPrefix(rest, "/projects/"), "/urls")
			var req struct {
				Put []struct{ Key string }
				Get []string
			}
			json.Unmarshal(body, &req)
			out := map[string][]string{"put": {}, "get": {}}
			for _, p := range req.Put {
				out["put"] = append(out["put"], srv.URL+"/r2/"+url.PathEscape("projects/"+pid+"/"+p.Key))
			}
			for _, g := range req.Get {
				out["get"] = append(out["get"], srv.URL+"/r2/"+url.PathEscape("projects/"+pid+"/"+g))
			}
			json.NewEncoder(w).Encode(out)
		case strings.HasSuffix(rest, "/missing"):
			pid := strings.TrimSuffix(strings.TrimPrefix(rest, "/projects/"), "/missing")
			var req struct{ Hashes []string }
			json.Unmarshal(body, &req)
			miss := []string{}
			for _, h := range req.Hashes {
				if _, there := kv["projects/"+pid+"/objects/"+h[:2]+"/"+h[2:]]; !there {
					miss = append(miss, h)
				}
			}
			json.NewEncoder(w).Encode(map[string][]string{"missing": miss})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("R3V_CLOUD_TOKEN", "token")
	return "r3v-cloud+" + srv.URL + "/v1/teams/t"
}
