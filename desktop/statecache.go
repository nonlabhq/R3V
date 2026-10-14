package desktop

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

// keepState keeps st for the next start: written when it changed since
// it was last kept (a state is read often, and changes seldom).
func keepState(st *State) {
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	path := stateFile(st.Root)
	sum := sha256.Sum256(data)
	keptStates.Lock()
	defer keptStates.Unlock()
	if keptStates.sums[path] == sum {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0o700)
	if store.WriteAtomic(path, bytes.NewReader(data)) == nil {
		keptStates.sums[path] = sum
	}
}

// keptStates: what each state file holds (its SHA-256), as last written.
var keptStates = struct {
	sync.Mutex
	sums map[string][32]byte
}{sums: map[string][32]byte{}}

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
