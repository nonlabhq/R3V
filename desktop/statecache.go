package desktop

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/nonlabhq/r3v/internal/store"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Each project's state as R3V last read it, kept on this computer, so the
// page opens at once after R3V starts (LastState) while it's read again
// (State: reading it can wait for an agent's commit, the team, a big
// folder).

func stateFile(root string) string {
	h := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(root))))
	return filepath.Join(teams.Dir(), "states", hex.EncodeToString(h[:12])+".json")
}

// keepState keeps st for the next start (in the background: never in the
// way of the page).
func keepState(st *State) {
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	go func() {
		path := stateFile(st.Root)
		os.MkdirAll(filepath.Dir(path), 0o700)
		store.WriteAtomic(path, bytes.NewReader(data))
	}()
}

// LastState is the project's state as R3V last read it (Stale), nil when it
// never did: shown until State answers.
func (a *App) LastState(root string) (*State, error) {
	data, err := os.ReadFile(stateFile(root))
	if err != nil {
		return nil, nil
	}
	var st State
	if json.Unmarshal(data, &st) != nil || !strings.EqualFold(filepath.Clean(st.Root), filepath.Clean(root)) {
		return nil, nil
	}
	st.Stale, st.Busy = true, false
	return &st, nil
}
