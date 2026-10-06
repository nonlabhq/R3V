package project

import (
	"os"
	"testing"
)

// Tests must not touch the user's real team store.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "r3v-teams-")
	if err != nil {
		panic(err)
	}
	os.Setenv("R3V_CONFIG_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
