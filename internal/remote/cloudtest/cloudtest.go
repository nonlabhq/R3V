// Package cloudtest is a fake R3V-Cloud for tests: the small keys (with
// conditional writes), folders, the presigned URLs it hands out and the
// storage they point at, which objects a project lacks, and file locks
// (branch moves carrying the paths they change, checked against them).
package cloudtest

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
	"sync/atomic"
	"testing"
	"time"
)

// Owner is the member the default session ("token") is.
const Owner = "0000000000000000000000000000000a"

// Fake is a fake hosted team.
type Fake struct {
	// Address is the team's address (to connect to).
	Address string
	// Service is the service's address (https://<host>).
	Service string

	mu      sync.Mutex
	members map[string]member // by session token
	locks   map[string]map[string]*lock
	// BranchBodies counts branch moves by their form ("branch", "plain").
	BranchBodies map[string]int
	// Down: the service can't be reached (connections dropped).
	Down atomic.Bool
}

type member struct {
	id    string
	admin bool
}

type lock struct {
	Path      string `json:"path"`
	Prefix    bool   `json:"prefix"`
	MemberID  string `json:"memberId"`
	Workspace string `json:"workspace"`
	Since     int64  `json:"since"`
}

// New starts a fake hosted team (closed with the test) and returns its
// address (to connect to), the session token set in the environment.
func New(t testing.TB) string { return NewFake(t).Address }

// NewFake starts a fake hosted team; the session in the environment is the
// owner's ("token").
func NewFake(t testing.TB) *Fake {
	t.Helper()
	f := &Fake{members: map[string]member{"token": {Owner, true}}, locks: map[string]map[string]*lock{},
		BranchBodies: map[string]int{}}
	kv := map[string][]byte{}
	tags := map[string]int{}
	n := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f.Down.Load() {
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					conn.Close()
					return
				}
			}
		}
		ep := r.URL.EscapedPath()
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		defer f.mu.Unlock()
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
		me, ok := f.members[strings.TrimPrefix(r.Header.Get("authorization"), "Bearer ")]
		if !ok {
			w.WriteHeader(401)
			return
		}
		if ep == "/v1/me" {
			role := "member"
			if me.admin {
				role = "owner"
			}
			json.NewEncoder(w).Encode(map[string]any{"user": map[string]string{"id": "u" + me.id, "email": me.id + "@example.test"},
				"teams": []map[string]string{{"id": "t", "name": "Team", "role": role, "memberId": me.id}}})
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
				if !there || !listed(kv, k) {
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
				if pid, ok := branchKey(k); ok {
					var changed []string
					if body, changed, ok = f.branchMove(w, r, kv, body, pid, me); !ok {
						return
					}
					defer f.freeShared(pid, me.id, changed)
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
		case strings.Contains(rest, "/locks"):
			f.locksCall(w, r, kv, rest, body, me)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("R3V_CLOUD_TOKEN", "token")
	f.Service = srv.URL
	f.Address = "r3v-cloud+" + srv.URL + "/v1/teams/t"
	return f
}

// AddMember lets a session (token) in as a member (id: 32 hex
// characters); admins may break locks.
func (f *Fake) AddMember(token, id string, admin bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.members[token] = member{id, admin}
}

// Locks lists a project's locks: path -> member id.
func (f *Fake) Locks(pid string) map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]string{}
	for p, l := range f.locks[pid] {
		out[p] = l.MemberID
	}
	return out
}

func branchKey(k string) (pid string, ok bool) {
	parts := strings.Split(k, "/")
	if len(parts) == 4 && parts[0] == "projects" && parts[2] == "branches" {
		return parts[1], true
	}
	return "", false
}

func locksOn(kv map[string][]byte) bool {
	var info struct{ Locks struct{ On bool } }
	json.Unmarshal(kv["team.json"], &info)
	return info.Locks.On
}

func refuse(w http.ResponseWriter, code, message string, extra map[string]any) {
	e := map[string]any{"code": code, "message": message}
	for k, v := range extra {
		e[k] = v
	}
	w.WriteHeader(409)
	json.NewEncoder(w).Encode(map[string]any{"error": e})
}

// branchMove reads a branch move's body (stored as "<head>\n") and checks
// it against the locks; false when refused (answered).
func (f *Fake) branchMove(w http.ResponseWriter, r *http.Request, kv map[string][]byte, body []byte, pid string,
	me member) ([]byte, []string, bool) {
	if r.Header.Get("content-type") != "application/vnd.r3v.branch+json" {
		f.BranchBodies["plain"]++
		if locksOn(kv) {
			refuse(w, "update_r3v", "this team uses file locks: update R3V", nil)
			return nil, nil, false
		}
		return body, nil, true
	}
	f.BranchBodies["branch"]++
	var mv struct {
		Head    string    `json:"head"`
		Changed *[]string `json:"changed"`
	}
	if json.Unmarshal(body, &mv) != nil || mv.Head == "" || mv.Changed == nil {
		w.WriteHeader(400)
		return nil, nil, false
	}
	if locksOn(kv) {
		var held []map[string]string
		for _, p := range *mv.Changed {
			for _, l := range f.locks[pid] {
				if l.MemberID != me.id && covers(l.Path, p) {
					held = append(held, map[string]string{"path": p, "memberId": l.MemberID})
					break
				}
			}
		}
		if len(held) > 0 {
			refuse(w, "locked", "someone else is editing these files", map[string]any{"locks": held[:min(len(held), 100)]})
			return nil, nil, false
		}
	}
	return []byte(mv.Head + "\n"), *mv.Changed, true
}

// freeShared frees the sharer's own file locks on the paths a share changed
// (folder locks stay).
func (f *Fake) freeShared(pid, memberID string, changed []string) {
	for _, p := range changed {
		if l := f.locks[pid][p]; l != nil && !l.Prefix && l.MemberID == memberID {
			delete(f.locks[pid], p)
		}
	}
}

func covers(lockPath, p string) bool {
	if strings.HasSuffix(lockPath, "/") {
		return strings.HasPrefix(p, lockPath)
	}
	return lockPath == p
}

// locksCall answers …/projects/<pid>/locks[/heartbeat | /<path>].
func (f *Fake) locksCall(w http.ResponseWriter, r *http.Request, kv map[string][]byte, rest string, body []byte, me member) {
	tail := strings.TrimPrefix(rest, "/projects/")
	pid, tail, _ := strings.Cut(tail, "/locks")
	if f.locks[pid] == nil {
		f.locks[pid] = map[string]*lock{}
	}
	locks := f.locks[pid]
	switch {
	case r.Method == "GET" && tail == "":
		items := []*lock{}
		for _, l := range locks {
			items = append(items, l)
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
		json.NewEncoder(w).Encode(map[string]any{"items": items})
	case r.Method == "POST" && tail == "/heartbeat":
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	case r.Method == "POST" && tail == "":
		var req struct {
			Lock, Unlock []string
			Workspace    string
		}
		json.Unmarshal(body, &req)
		if len(req.Lock) > 0 && !locksOn(kv) {
			refuse(w, "locks_off", "file locking is off for this team", nil)
			return
		}
		for _, p := range req.Unlock {
			if l := locks[p]; l != nil && l.MemberID == me.id {
				delete(locks, p)
			}
		}
		out := struct {
			Locked  []string            `json:"locked"`
			Refused []map[string]string `json:"refused"`
		}{[]string{}, []map[string]string{}}
		for _, p := range req.Lock {
			by := ""
			for _, l := range locks {
				if l.MemberID == me.id {
					continue
				}
				if covers(l.Path, p) || strings.HasSuffix(p, "/") && strings.HasPrefix(l.Path, p) {
					by = l.MemberID
					break
				}
			}
			if by != "" {
				out.Refused = append(out.Refused, map[string]string{"path": p, "memberId": by})
				continue
			}
			if l := locks[p]; l != nil {
				l.Workspace = req.Workspace // (the member's own: moves to where it was taken last)
			} else {
				locks[p] = &lock{Path: p, Prefix: strings.HasSuffix(p, "/"), MemberID: me.id, Workspace: req.Workspace,
					Since: time.Now().UnixMilli()}
			}
			out.Locked = append(out.Locked, p)
		}
		json.NewEncoder(w).Encode(out)
	case r.Method == "DELETE" && strings.HasPrefix(tail, "/"):
		if !me.admin {
			w.WriteHeader(403)
			return
		}
		p, _ := url.PathUnescape(strings.TrimPrefix(tail, "/"))
		delete(locks, p)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	default:
		w.WriteHeader(404)
	}
}

// listed: like the service, a project's contents (objects, big files'
// lists, versions) are asked about only once its project.json is there.
func listed(kv map[string][]byte, key string) bool {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) < 3 || parts[0] != "projects" {
		return true
	}
	switch strings.SplitN(parts[2], "/", 2)[0] {
	case "objects", "chunked", "snapshots":
		_, ok := kv["projects/"+parts[1]+"/project.json"]
		return ok
	}
	return true
}
