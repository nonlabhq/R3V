package desktop

import (
	"os"
	"testing"

	"github.com/nonlabhq/r3v/internal/actor"
)

// The app is a person, whoever runs the tests (an agent's commits take
// agent locks; tests that want an agent say so).
func TestMain(m *testing.M) {
	os.Setenv(actor.EnvTool, actor.None)
	os.Exit(m.Run())
}
