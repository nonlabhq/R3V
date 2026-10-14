package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nonlabhq/r3v/internal/als"
	"github.com/nonlabhq/r3v/internal/audio"
	"github.com/nonlabhq/r3v/internal/project"
)

// r3v info <file>: what a file is, the way its kind tells it (a Live Set
// its tracks, audio its format and length, a picture its size, text its
// lines, any file its size), and, in a project, whether R3V keeps it and
// why. (Stable: a Live Set's only.)

// fileInfoJSON is r3v info --json.
type fileInfoJSON struct {
	Path string `json:"path"`
	// Kind: set, audio, image, text or binary.
	Kind string `json:"kind"`
	Size int64  `json:"size"`
	Set  *struct {
		Creator string `json:"creator"`
		Tempo   string `json:"tempo"`
		Tracks  int    `json:"tracks"`
		Scenes  int    `json:"scenes"`
	} `json:"set,omitempty"`
	Audio *struct {
		Rate     int     `json:"rate"`
		Channels int     `json:"channels"`
		Bits     int     `json:"bits"`
		Seconds  float64 `json:"seconds"`
	} `json:"audio,omitempty"`
	Picture *struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"picture,omitempty"`
	Text *struct {
		Lines      int    `json:"lines"`
		LineEnding string `json:"line_ending"` // lf | crlf | none
	} `json:"text,omitempty"`
	// Tracked: in a project, R3V keeps it (By: the rule that decides).
	Tracked *bool  `json:"tracked,omitempty"`
	By      string `json:"by,omitempty"`
}

func cmdFileInfo(args []string) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	pos, err := parseArgs(fs, args)
	if err != nil || len(pos) != 1 {
		return usageError("usage: r3v info <file>")
	}
	path := pos[0]
	if !fileCommands || (isSetFile(path) && !jsonMode) {
		return cmdInfo(pos) // (a Live Set's tracks, devices, clips, as before)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return usageError("%s is a folder: r3v info takes a file", path)
	}
	in := fileInfoJSON{Path: filepath.ToSlash(path), Kind: "binary", Size: fi.Size()}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	head, _ := io.ReadAll(io.LimitReader(f, maxDiffText+1))
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case isSetFile(path):
		s, err := als.Load(path)
		if err != nil {
			return err
		}
		in.Kind = "set"
		in.Set = &struct {
			Creator string `json:"creator"`
			Tempo   string `json:"tempo"`
			Tracks  int    `json:"tracks"`
			Scenes  int    `json:"scenes"`
		}{s.Creator(), s.Tempo(), len(s.Tracks()), s.SceneCount()}
	case ext == ".wav" || ext == ".aif" || ext == ".aiff":
		f.Seek(0, io.SeekStart)
		read := audio.WAVInfo
		if ext != ".wav" {
			read = audio.AIFFInfo
		}
		if a, err := read(f); err == nil {
			in.Kind = "audio"
			in.Audio = &struct {
				Rate     int     `json:"rate"`
				Channels int     `json:"channels"`
				Bits     int     `json:"bits"`
				Seconds  float64 `json:"seconds"`
			}{a.Rate, a.Channels, a.Bits, a.Seconds}
		}
	default:
		if p := pictureSize(head); p != nil {
			in.Kind = "image"
			in.Picture = &struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			}{p.X, p.Y}
		} else if len(head) <= maxDiffText && isText(head) {
			in.Kind = "text"
			ending := "none"
			if i := bytes.IndexByte(head, '\n'); i > 0 && head[i-1] == '\r' {
				ending = "crlf"
			} else if i >= 0 {
				ending = "lf"
			}
			lines := bytes.Count(head, []byte("\n"))
			if len(head) > 0 && head[len(head)-1] != '\n' {
				lines++
			}
			in.Text = &struct {
				Lines      int    `json:"lines"`
				LineEnding string `json:"line_ending"`
			}{lines, ending}
		}
	}
	// In a project: kept or left out, and why.
	if r, err := project.Open(filepath.Dir(path)); err == nil {
		if abs, err := filepath.Abs(path); err == nil {
			if rel, err := filepath.Rel(r.Root, abs); err == nil && !strings.HasPrefix(rel, "..") {
				if rules, err := r.Profile(); err == nil {
					d := rules.Explain(filepath.ToSlash(rel), false)
					tracked := !d.Ignored
					in.Tracked, in.By = &tracked, d.By
				}
			}
		}
	} else if !errors.Is(err, project.ErrNotRepo) {
		return err
	}
	result("info", in, func() { printFileInfo(in) })
	return nil
}

func printFileInfo(in fileInfoJSON) {
	fmt.Printf("%s  %s, %s\n", in.Path, in.Kind, humanSize(in.Size))
	switch {
	case in.Set != nil:
		fmt.Printf("  Live Set [%s]  tempo %s, %d tracks, %d scenes\n", in.Set.Creator, in.Set.Tempo, in.Set.Tracks, in.Set.Scenes)
	case in.Audio != nil:
		fmt.Printf("  %d Hz, %d channel(s), %d bit, %.2f s\n", in.Audio.Rate, in.Audio.Channels, in.Audio.Bits, in.Audio.Seconds)
	case in.Picture != nil:
		fmt.Printf("  %d x %d\n", in.Picture.Width, in.Picture.Height)
	case in.Text != nil:
		fmt.Printf("  %d lines (%s)\n", in.Text.Lines, in.Text.LineEnding)
	}
	if in.Tracked != nil {
		state := "tracked"
		if !*in.Tracked {
			state = "ignored"
		}
		fmt.Printf("  %s (%s)\n", state, in.By)
	}
}
