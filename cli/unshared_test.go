package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// status counts the versions committed here the team's branch lacks
// (commit --local), until a commit shares them.
func TestStatusCountsUnshared(t *testing.T) {
	a := teamProject(t)
	unshared := func(want int) {
		t.Helper()
		exit, rep := run(t, a, "status")
		var st statusJSON
		json.Unmarshal(rep.Result, &st)
		if exit != 0 || st.Unshared == nil || *st.Unshared != want {
			t.Fatalf("status: want unshared %d: %s", want, rep.Result)
		}
	}
	unshared(0)
	for i, body := range []string{"one", "two"} {
		os.WriteFile(filepath.Join(a, "src", "main.ts"), []byte(body), 0o644)
		if exit, rep := run(t, a, "commit", "--local", "-m", body); exit != 0 {
			t.Fatalf("commit --local: %+v", rep.Error)
		}
		unshared(i + 1)
	}
	if exit, rep := run(t, a, "commit", "-m", "share"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	unshared(0)
}
