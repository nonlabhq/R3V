package actor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/store"
)

// SessionsDir holds, in a copy's own .r3v, the model each agent session
// runs, as the tool's hooks tell it (Claude Code's SessionStart and
// PostModelSwitch: no variable says the model). This copy's only, never
// tracked, never shared; one small file per session, so sessions starting
// at once don't race.
const SessionsDir = "agent-sessions"

// sessionKept: older records are dropped when one is written.
const sessionKept = 30 * 24 * time.Hour

// sessionRecord is one session's file.
type sessionRecord struct {
	Tool    string    `json:"tool"`
	Model   string    `json:"model"`
	Updated time.Time `json:"updated"`
}

// sessionID is what a session id may be to name a file.
var sessionID = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]{0,127}$`)

func sessionPath(dir, session string) string {
	if dir == "" || !sessionID.MatchString(session) {
		return ""
	}
	return filepath.Join(dir, SessionsDir, session+".json")
}

// RecordModel keeps the model the tool's session runs, in the .r3v
// folder dir. Records older than a month go.
func RecordModel(dir, tool, session, model string) error {
	path := sessionPath(dir, session)
	tool, model = clean(tool), clean(model)
	if path == "" || tool == "" || model == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, _ := json.Marshal(sessionRecord{Tool: tool, Model: model, Updated: time.Now().UTC()})
	if err := store.WriteAtomic(path, strings.NewReader(string(data)+"\n")); err != nil {
		return err
	}
	if entries, err := os.ReadDir(filepath.Dir(path)); err == nil {
		for _, e := range entries {
			if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > sessionKept {
				os.Remove(filepath.Join(filepath.Dir(path), e.Name()))
			}
		}
	}
	return nil
}

// SessionModel is the model recorded for the tool's session in the .r3v
// folder dir, "" when none is.
func SessionModel(dir, tool, session string) string {
	path := sessionPath(dir, session)
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var rec sessionRecord
	if json.Unmarshal(data, &rec) != nil || rec.Tool != tool {
		return ""
	}
	return clean(rec.Model)
}
