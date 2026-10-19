package agentlock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if dir := os.Getenv("R3V_AGENTLOCK_CHILD"); dir != "" {
		killedChild(dir) // a test's child process, to be killed
		return
	}
	os.Exit(m.Run())
}

var (
	ws1 = Holder{Workspace: "aaaaaaaaaaaa", Branch: "renderer", Session: "s1", Actor: "claude-code"}
	ws2 = Holder{Workspace: "bbbbbbbbbbbb", Branch: "textures", Session: "s2", Actor: "claude-code"}
)

func testList(t *testing.T) (*List, *time.Time) {
	t.Helper()
	l := Open(t.TempDir())
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	l.SetClock(func() time.Time { return now })
	return l, &now
}

func paths(locks []Lock) string {
	var s []string
	for _, l := range locks {
		s = append(s, l.Path)
	}
	return strings.Join(s, ",")
}

func TestAllOrNone(t *testing.T) {
	l, _ := testList(t)
	if _, err := l.Take(ws2, []string{"tex/rock.png"}, KindLock, nil); err != nil {
		t.Fatal(err)
	}
	_, err := l.Take(ws1, []string{"models/ship.glb", "tex/rock.png", "sfx/boom.wav"}, KindLock, nil)
	var refused *Refused
	if !errors.As(err, &refused) || len(refused.Locks) != 1 || refused.Locks[0].Workspace != ws2.Workspace {
		t.Fatalf("refused: %v", err)
	}
	all, _ := l.All()
	if paths(all) != "tex/rock.png" {
		t.Fatalf("a refused set must leave nothing: %s", paths(all))
	}
	// A holder the list doesn't keep (the person's changes) refuses as well.
	extra := []Lock{{Path: "sfx/boom.wav", Kind: KindLock, Workspace: "person", Person: true}}
	if _, err := l.Take(ws1, []string{"models/ship.glb", "sfx/boom.wav"}, KindLock, extra); !errors.As(err, &refused) ||
		!refused.Locks[0].Person {
		t.Fatalf("person: %v", err)
	}
	if all, _ := l.All(); paths(all) != "tex/rock.png" {
		t.Fatalf("after the person's refusal: %s", paths(all))
	}
	res, err := l.Take(ws1, []string{"models/ship.glb", "sfx/boom.wav"}, KindLock, nil)
	if err != nil || len(res.Taken) != 2 {
		t.Fatalf("take: %+v %v", res, err)
	}
	// Again: already ours, nothing new.
	if res, err := l.Take(ws1, []string{"models/ship.glb"}, KindLock, nil); err != nil || len(res.Taken) != 1 {
		t.Fatalf("again: %+v %v", res, err)
	}
	if all, _ := l.All(); len(all) != 3 {
		t.Fatalf("locks: %+v", all)
	}
}

func TestFolderCovers(t *testing.T) {
	l, _ := testList(t)
	if _, err := l.Take(ws1, []string{"Tex/"}, KindLock, nil); err != nil {
		t.Fatal(err)
	}
	var refused *Refused
	if _, err := l.Take(ws2, []string{"tex/sub/rock.png"}, KindLock, nil); !errors.As(err, &refused) {
		t.Fatalf("a folder covers what's under it (any case): %v", err)
	}
	if _, err := l.Take(ws2, []string{"texture.png", "tex2/a.png"}, KindLock, nil); err != nil {
		t.Fatalf("outside the folder: %v", err)
	}
	if _, err := l.Take(ws1, []string{"Tex/a.png"}, KindLock, nil); err != nil {
		t.Fatal(err)
	}
	if all, _ := l.All(); len(all) != 3 {
		t.Fatalf("a file in our own folder is ours already: %+v", all)
	}
	// A folder over another's file is refused too.
	if _, err := l.Take(ws1, []string{"tex2/"}, KindLock, nil); !errors.As(err, &refused) || refused.Locks[0].Path != "tex2/a.png" {
		t.Fatalf("folder over a file: %v", err)
	}
}

func TestStaleTakeover(t *testing.T) {
	l, now := testList(t)
	if _, err := l.Take(ws1, []string{"a.png", "b.png"}, KindLock, nil); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(10 * time.Minute)
	l.Heartbeat(ws1.Workspace) // still there
	*now = now.Add(10 * time.Minute)
	all, _ := l.All()
	if all[0].Stale || !all[0].Heartbeat.Equal(now.Add(-10*time.Minute)) {
		t.Fatalf("heard of 10 minutes ago: %+v", all[0])
	}
	var refused *Refused
	if _, err := l.Take(ws2, []string{"a.png"}, KindLock, nil); !errors.As(err, &refused) {
		t.Fatalf("not stale yet: %v", err)
	}
	*now = now.Add(StaleAfter)
	if all, _ := l.All(); !all[0].Stale || !all[1].Stale {
		t.Fatalf("stale: %+v", all)
	}
	res, err := l.Take(ws2, []string{"a.png"}, KindLock, nil)
	if err != nil || len(res.TakenOver) != 1 || res.TakenOver[0].Workspace != ws1.Workspace {
		t.Fatalf("take over: %+v %v", res, err)
	}
	all, _ = l.All()
	if len(all) != 2 || all[0].Workspace != ws2.Workspace || all[0].Stale || !all[1].Stale {
		t.Fatalf("after: %+v", all)
	}
}

// A holder that's gone before its locks went stale: taken over on
// purpose, the reason and the holder kept.
func TestTakeOver(t *testing.T) {
	l, _ := testList(t)
	if _, err := l.Take(ws1, []string{"a.png", "tex/"}, KindLock, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := l.TakeOver(ws2, []string{"a.png"}, " ", nil); err == nil {
		t.Fatal("a takeover without a reason")
	}
	// The person's changes still win: nothing taken.
	extra := []Lock{{Path: "tex/b.png", Kind: KindLock, Workspace: "person", Person: true}}
	var refused *Refused
	if _, err := l.TakeOver(ws2, []string{"a.png", "tex/b.png"}, "gone", extra); !errors.As(err, &refused) {
		t.Fatalf("the person's: %v", err)
	}
	if all, _ := l.All(); len(all) != 2 || all[0].Workspace != ws1.Workspace {
		t.Fatalf("refused: nothing taken over: %+v", all)
	}
	res, err := l.TakeOver(ws2, []string{"a.png", "tex/b.png"}, "its session ended", nil)
	if err != nil || len(res.TakenOver) != 2 || len(res.Taken) != 2 {
		t.Fatalf("take over: %+v %v", res, err)
	}
	all, _ := l.All()
	if len(all) != 2 || paths(all) != "a.png,tex/b.png" {
		t.Fatalf("after: %+v", all)
	}
	for _, lk := range all {
		if lk.Workspace != ws2.Workspace || lk.Reason != "its session ended" || !strings.Contains(lk.TakenFrom, "renderer") {
			t.Fatalf("recorded: %+v", lk)
		}
	}
	// Nothing in the way: an ordinary lock, no reason kept.
	res, err = l.TakeOver(ws2, []string{"c.png"}, "gone", nil)
	if err != nil || len(res.TakenOver) != 0 || res.Taken[0].Reason != "" {
		t.Fatalf("free: %+v %v", res, err)
	}
}

func TestClaims(t *testing.T) {
	l, _ := testList(t)
	if _, err := l.Take(ws1, []string{"src/renderer/"}, KindClaim, nil); err != nil {
		t.Fatal(err)
	}
	res, err := l.Take(ws2, []string{"src/renderer/main.ts"}, KindLock, nil)
	if err != nil || len(res.Others) != 1 || res.Others[0].Kind != KindClaim {
		t.Fatalf("a claim never refuses, it tells: %+v %v", res, err)
	}
	if _, err := l.Take(ws1, []string{"src/"}, KindClaim, nil); err != nil {
		t.Fatalf("claims over a lock: %v", err)
	}
	res, err = l.Take(ws2, []string{"src/renderer/"}, KindClaim, nil)
	if err != nil || len(res.Others) != 2 {
		t.Fatalf("two claims on one folder: %+v %v", res, err)
	}
	all, _ := l.All()
	if len(all) != 4 {
		t.Fatalf("locks: %+v", all)
	}
}

func TestUnlockBreakRelease(t *testing.T) {
	l, _ := testList(t)
	l.Take(ws1, []string{"a.png", "tex/"}, KindLock, nil)
	l.Take(ws2, []string{"b.png"}, KindLock, nil)
	l.Take(ws2, []string{"src/"}, KindClaim, nil)
	if got, _ := l.Unlock(ws2.Workspace, []string{"a.png"}); len(got) != 0 {
		t.Fatalf("unlock is by holder: %+v", got)
	}
	if got, _ := l.Unlock(ws1.Workspace, []string{"A.png"}); len(got) != 1 {
		t.Fatalf("unlock: %+v", got)
	}
	if got, _ := l.Break("b.png"); len(got) != 1 || got[0].Workspace != ws2.Workspace {
		t.Fatalf("break: %+v", got)
	}
	if got, _ := l.ReleaseBranch(ws2.Branch); paths(got) != "src/" {
		t.Fatalf("release: %+v", got)
	}
	if all, _ := l.All(); paths(all) != "tex/" {
		t.Fatalf("left: %+v", all)
	}
}

func TestClean(t *testing.T) {
	for in, want := range map[string]string{`tex\a.png`: "tex/a.png", "./tex/": "tex/", "../x": "", ".": "", "a//b": "a/b"} {
		if got := Clean(in, false); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
	if Clean("tex", true) != "tex/" {
		t.Error("folder")
	}
}

// A writer stopped half-way leaves a temp file, never half a lock: it's
// not read, and swept later. A write that fails takes back the ones just
// made.
func TestStoppedHalfWay(t *testing.T) {
	l, now := testList(t)
	os.MkdirAll(l.Dir(), 0o755)
	os.WriteFile(filepath.Join(l.Dir(), ".r3v-123"), []byte(`{"path": "a.p`), 0o644)
	if all, err := l.All(); err != nil || len(all) != 0 {
		t.Fatalf("a temp file is no lock: %+v %v", all, err)
	}
	// The second lock's file can't be written: the first is taken back.
	os.MkdirAll(l.file(Lock{Path: "b.png", Kind: KindLock}), 0o755)
	os.WriteFile(filepath.Join(l.file(Lock{Path: "b.png", Kind: KindLock}), "x"), nil, 0o644)
	if _, err := l.Take(ws1, []string{"a.png", "b.png"}, KindLock, nil); err == nil {
		t.Fatal("expected the write to fail")
	}
	if all, _ := l.All(); len(all) != 0 {
		t.Fatalf("all or none: %+v", all)
	}
	*now = now.Add(time.Hour)
	os.Chtimes(filepath.Join(l.Dir(), ".r3v-123"), now.Add(-time.Hour), now.Add(-time.Hour))
	if _, err := l.Take(ws1, []string{"a.png"}, KindLock, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(l.Dir(), ".r3v-123")); !os.IsNotExist(err) {
		t.Fatal("a leftover temp file is swept")
	}
}

// killedChild takes locks as fast as it can until it is killed.
func killedChild(dir string) {
	l := Open(dir)
	for i := 0; ; i++ {
		h := ws1
		if i%2 == 1 {
			h = ws2
		}
		l.Take(h, []string{fmt.Sprintf("f%d.png", i), fmt.Sprintf("g%d.png", i)}, KindLock, nil)
		if i%3 == 0 {
			l.Unlock(h.Workspace, []string{fmt.Sprintf("f%d.png", i)})
		}
	}
}

// Killed at any point, every lock file there is whole.
func TestKilledWhileTaking(t *testing.T) {
	if testing.Short() {
		t.Skip("starts processes")
	}
	for round := 0; round < 4; round++ {
		dir := t.TempDir()
		cmd := exec.Command(os.Args[0], "-test.run=^$")
		cmd.Env = append(os.Environ(), "R3V_AGENTLOCK_CHILD="+dir)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Duration(150+round*70) * time.Millisecond)
		cmd.Process.Kill()
		cmd.Wait()
		entries, _ := os.ReadDir(filepath.Join(dir, DirName))
		n := 0
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), "l-") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, DirName, e.Name()))
			var lk Lock
			if err != nil || json.Unmarshal(data, &lk) != nil || lk.Path == "" {
				t.Fatalf("round %d: half a lock in %s: %q %v", round, e.Name(), data, err)
			}
			n++
		}
		if n == 0 {
			t.Logf("round %d: killed before any lock", round)
		}
		// The list goes on working after.
		l := Open(dir)
		if _, err := l.Take(ws1, []string{"after.png"}, KindLock, nil); err != nil {
			t.Fatalf("round %d: after: %v", round, err)
		}
	}
}
