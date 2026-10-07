package project

import (
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// lostAnswer moves branches but answers as if someone else had: what a
// write tried again looks like when its first try got through and the
// answer was lost.
type lostAnswer struct{ remote.Backend }

func (b lostAnswer) UpdateBranch(pid, name, old, new string) error {
	if err := b.Backend.UpdateBranch(pid, name, old, new); err != nil {
		return err
	}
	return &remote.ErrConflict{Current: new}
}

func init() {
	remote.Register("lostanswer+", func(cfg remote.Config) (remote.Backend, error) {
		b, err := remote.Open(remote.Config{URL: strings.TrimPrefix(cfg.URL, "lostanswer+")})
		if err != nil {
			return nil, err
		}
		return lostAnswer{b}, nil
	})
}

// A share whose branch move got through, its answer lost, is shared: it
// says so rather than "up to date".
func TestShareWithItsAnswerLost(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	a, _ := Init(newProject(t), "yi")
	if err := a.SetRemote("lostanswer+projtest+" + fake.URL + "/team/r3v"); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "Samples/kick.wav", "v1")
	m, res, err := a.Save("first", MergeOptions{})
	if err != nil || m == nil {
		t.Fatal(m, err)
	}
	if res.Action != "published" || res.To != m.ID {
		t.Errorf("action %q to %q, want published to %s", res.Action, res.To, m.ID)
	}
	c, _ := a.Client()
	if bs, _ := c.Branches(a.Config.ProjectID); bs["main"] != m.ID {
		t.Errorf("branches %v", bs)
	}
}
