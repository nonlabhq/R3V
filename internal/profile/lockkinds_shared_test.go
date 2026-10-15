package profile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The kinds of file a team auto-locks, as the shared UI has them
// (desktop/frontend/src/shared/lockKinds.json: R3V Cloud's web app reads
// it too): the same as LockKinds, in the same order. R3V_UPDATE_SHARED=1
// writes it.
func TestLockKindsShared(t *testing.T) {
	path := filepath.Join("..", "..", "desktop", "frontend", "src", "shared", "lockKinds.json")
	want, _ := json.MarshalIndent(LockKinds, "", "  ")
	want = append(want, '\n')
	if os.Getenv("R3V_UPDATE_SHARED") != "" {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")), want) {
		t.Errorf("%s isn't profile.LockKinds: run this test with R3V_UPDATE_SHARED=1", path)
	}
}
