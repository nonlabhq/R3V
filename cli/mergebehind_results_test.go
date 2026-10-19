package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// A result recorded for a branch head shows on the merge version made of
// exactly its files, with where it was recorded.
func TestResultsCarryOverToSameFilesMerge(t *testing.T) {
	a, lane := behindLane(t)
	if exit, rep := run(t, a, "result", "tests", "pass", "-m", "all passed", "--version", lane); exit != 0 {
		t.Fatalf("result: %+v", rep.Error)
	}
	if exit, rep := run(t, a, "merge", "vfx"); exit != 0 {
		t.Fatalf("merge: %+v", rep.Error)
	}
	exit, rep := run(t, a, "results")
	var out struct {
		Version string
		Results map[string]struct{ State, From string }
	}
	json.Unmarshal(rep.Result, &out)
	if exit != 0 || out.Version == lane || out.Results["tests"].State != "pass" || out.Results["tests"].From != lane {
		t.Fatalf("results: %s", rep.Result)
	}
	if exit, text := runText(t, a, "results"); exit != 0 || !strings.Contains(text, "recorded for "+lane[:10]) {
		t.Fatalf("results text: %s", text)
	}
}
