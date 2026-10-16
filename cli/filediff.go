package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/gif" // (sizes of the pictures diff compares)
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/nonlabhq/r3v/internal/als"
	"github.com/nonlabhq/r3v/internal/diff"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/textdiff"
)

// r3v diff: what changed in each file, the way each kind of file says it
// (a Live Set track by track, text line by line, a picture its size, any
// other file its size): your changes against the version you're on, a
// version's own changes, or any two versions. Two .als files on disk are
// still compared as before (r3v diff a.als b.als).

// fileDiffJSON is one file in r3v diff --json.
type fileDiffJSON struct {
	Path   string `json:"path"`
	Status string `json:"status"` // added | modified | deleted | renamed
	From   string `json:"from,omitempty"`
	// Kind: set (SetChanges), text (Added, Removed, Hunks), image (sizes
	// and Width/Height), binary (sizes).
	Kind       string          `json:"kind"`
	SetChanges []string        `json:"set_changes,omitempty"`
	Added      int             `json:"added,omitempty"`
	Removed    int             `json:"removed,omitempty"`
	Hunks      []textdiff.Hunk `json:"hunks,omitempty"`
	Truncated  bool            `json:"truncated,omitempty"` // more lines changed than shown
	TooBig     bool            `json:"too_big,omitempty"`   // a text too big to compare
	SizeOld    int64           `json:"size_old"`
	SizeNew    int64           `json:"size_new"`
	Picture    *pictureSizes   `json:"picture,omitempty"`
}

type pictureSizes struct {
	OldWidth  int `json:"old_width"`
	OldHeight int `json:"old_height"`
	NewWidth  int `json:"new_width"`
	NewHeight int `json:"new_height"`
}

// fileCommands: diff takes the project's files, any kind (Nightly while new;
// Stable compares two sets on disk only).
var fileCommands = false

const fileDiffsUsage = `
files (each kind the way it reads: text line by line, a Live Set track by track,
a picture its size, any other file its size; Nightly):
  diff [PATHS]                           what changed in each file since the version you're on
  diff --version V [PATHS]               what a version changed
  diff --from A --to B [PATHS]           what changed between two versions
  diff --files <a> <b>                   two files on disk, any kind (two Live Sets: track by track)
  info <file>                            what a file is (a set's tracks, audio's format, a picture's
                                         size, text's lines) and, in a project, whether R3V keeps it
  merge-files <base> <ours> <theirs> -o <out> [--strategy fail|ours|theirs|both]
                                         a 3-way merge of three files, each kind the way it merges
                                         (merge-sets: the same, for Live Sets)
`

const (
	maxDiffText  = 8 << 20 // bytes a side of a text compared
	maxDiffLines = 2000    // lines shown per file
)

func cmdFileDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	version := fs.String("version", "", "what a version changed (against the version before it)")
	from := fs.String("from", "", "compare from this version...")
	to := fs.String("to", "", "...to this one (default: your files now)")
	files := fs.Bool("files", false, "compare two files on disk (any kind; in a project or not)")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	// Two Live Sets on disk: compared as before.
	if !*files && *version == "" && *from == "" && *to == "" && len(pos) == 2 && isSetFile(pos[0]) && isSetFile(pos[1]) {
		if _, err := os.Stat(pos[0]); err == nil {
			return diffSetFiles(pos[0], pos[1])
		}
	}
	if *files {
		if !fileCommands {
			return usageError("diff --files needs R3V's Nightly channel")
		}
		if len(pos) != 2 || *version != "" || *from != "" || *to != "" {
			return usageError("usage: r3v diff --files <a> <b>")
		}
		d, err := diffDiskFiles(pos[0], pos[1])
		if err != nil {
			return err
		}
		result("diff", map[string]any{"files": []fileDiffJSON{d}}, func() { printFileDiffs([]fileDiffJSON{d}) })
		return nil
	}
	if !fileCommands {
		return usageError("usage: r3v diff <a.als> <b.als> (a project's files: R3V's Nightly channel)")
	}
	r, err := openRead()
	if err != nil {
		return err
	}
	// (old, new): the versions compared; "" for your files now.
	var old, cur string
	var changes []project.FileChange
	switch {
	case *version != "":
		if *from != "" || *to != "" {
			return usageError("--version or --from/--to, not both")
		}
		id, err := r.Resolve(*version)
		if err != nil {
			return err
		}
		m, err := r.Load(id)
		if err != nil {
			return err
		}
		if len(m.Parents) > 0 {
			old = m.Parents[0]
		}
		cur = id
		if changes, err = r.VersionChanges(id); err != nil {
			return err
		}
	case *from != "" && *to != "":
		if old, err = r.Resolve(*from); err != nil {
			return err
		}
		if cur, err = r.Resolve(*to); err != nil {
			return err
		}
		if changes, err = r.ChangesBetween(old, cur); err != nil {
			return err
		}
	case *from != "" || *to != "":
		return usageError("--from and --to go together (two versions); your changes: r3v diff")
	default:
		old = r.Head()
		st, err := r.Status()
		if err != nil {
			return err
		}
		for _, c := range st {
			if c.Status == "untracked" {
				continue
			}
			changes = append(changes, project.FileChange{Path: c.Path, Status: c.Status, From: c.From, Edited: c.Edited, SetDiff: c.SetDiff})
		}
	}
	var out []fileDiffJSON
	for _, c := range changes {
		if !underAny(c.Path, pos) && !(c.From != "" && underAny(c.From, pos)) {
			continue
		}
		out = append(out, compareFile(r, c, old, cur))
	}
	if out == nil {
		out = []fileDiffJSON{}
	}
	result("diff", map[string]any{"from": old, "to": cur, "files": out}, func() { printFileDiffs(out) })
	return nil
}

// underAny: path is one of paths, or in one of them (a folder); every
// path when there are none.
func underAny(path string, paths []string) bool {
	if len(paths) == 0 {
		return true
	}
	for _, p := range paths {
		p = strings.TrimSuffix(filepath.ToSlash(filepath.Clean(p)), "/")
		if p == "." || path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

func isSetFile(p string) bool { return strings.EqualFold(filepath.Ext(p), ".als") }

// compareFile says how file c changed from version old to cur ("" for the
// files now; old "" for none).
func compareFile(r *project.Repo, c project.FileChange, old, cur string) fileDiffJSON {
	d := fileDiffJSON{Path: c.Path, Status: c.Status, From: c.From, Kind: "binary"}
	oldPath := c.Path
	if c.From != "" {
		oldPath = c.From
	}
	var a, b []byte
	var aErr, bErr error
	if c.Status != "added" && old != "" {
		a, d.SizeOld, aErr = readSide(r, oldPath, old)
	}
	if c.Status != "deleted" {
		b, d.SizeNew, bErr = readSide(r, c.Path, cur)
	}
	describe(&d, c.Path, a, b, aErr == nil && bErr == nil, func() *diff.SetDiff { return c.SetDiff })
	return d
}

// describe fills in how a file changed from a to b, the way its kind reads
// (read: both sides were read whole enough to compare; setDiff: a Live
// Set's changes, when it is one).
func describe(d *fileDiffJSON, path string, a, b []byte, read bool, setDiff func() *diff.SetDiff) {
	switch {
	case isSetFile(path):
		d.Kind = "set"
		if sd := setDiff(); sd != nil {
			d.SetChanges = setLines(sd.Render())
		}
	case read && isText(a) && isText(b) && (len(a) > 0 || len(b) > 0):
		d.Kind = "text"
		if len(a) > maxDiffText || len(b) > maxDiffText {
			d.TooBig = true
			break
		}
		shown := 0
		for _, h := range textdiff.Hunks(textdiff.Split(string(a)), textdiff.Split(string(b)), 3) {
			for _, l := range h.Lines {
				switch l.Kind {
				case "add":
					d.Added++
				case "del":
					d.Removed++
				}
			}
			if shown+len(h.Lines) > maxDiffLines {
				d.Truncated = true
				continue
			}
			shown += len(h.Lines)
			d.Hunks = append(d.Hunks, h)
		}
	default:
		if pa, pb := pictureSize(a), pictureSize(b); pa != nil || pb != nil {
			d.Kind = "image"
			d.Picture = &pictureSizes{}
			if pa != nil {
				d.Picture.OldWidth, d.Picture.OldHeight = pa.X, pa.Y
			}
			if pb != nil {
				d.Picture.NewWidth, d.Picture.NewHeight = pb.X, pb.Y
			}
		}
	}
}

// readSide reads path in version (a version id, "" for the folder now): at
// most maxDiffText+1 bytes, and its size.
func readSide(r *project.Repo, path, version string) ([]byte, int64, error) {
	f, err := r.OpenFile(path, version)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	size, _ := f.Seek(0, io.SeekEnd)
	f.Seek(0, io.SeekStart)
	data, err := io.ReadAll(io.LimitReader(f, maxDiffText+1))
	return data, size, err
}

func isText(b []byte) bool { return bytes.IndexByte(b, 0) < 0 && utf8.Valid(b) }

func pictureSize(b []byte) *image.Point {
	if len(b) == 0 {
		return nil
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	return &image.Point{X: cfg.Width, Y: cfg.Height}
}

func printFileDiffs(files []fileDiffJSON) {
	if len(files) == 0 {
		fmt.Println("no changes")
		return
	}
	for i, d := range files {
		if i > 0 {
			fmt.Println()
		}
		name := d.Path
		if d.From != "" {
			name = d.From + " -> " + d.Path
		}
		switch d.Kind {
		case "text":
			fmt.Printf("%s %s  +%d -%d\n", d.Status, name, d.Added, d.Removed)
			if d.TooBig {
				fmt.Println("  (too big to compare line by line)")
			}
			for _, h := range d.Hunks {
				fmt.Println("@@")
				for _, l := range h.Lines {
					mark := " "
					switch l.Kind {
					case "add":
						mark = "+"
					case "del":
						mark = "-"
					}
					fmt.Println(mark + l.Text)
				}
			}
			if d.Truncated {
				fmt.Println("  (more changes not shown)")
			}
		case "set":
			fmt.Printf("%s %s  (Live Set)\n", d.Status, name)
			for _, l := range d.SetChanges {
				fmt.Println("  " + l)
			}
		case "image":
			p := d.Picture
			fmt.Printf("%s %s  (picture) %dx%d -> %dx%d, %s -> %s\n", d.Status, name, p.OldWidth, p.OldHeight, p.NewWidth, p.NewHeight,
				humanSize(d.SizeOld), humanSize(d.SizeNew))
		default:
			fmt.Printf("%s %s  %s -> %s\n", d.Status, name, humanSize(d.SizeOld), humanSize(d.SizeNew))
		}
	}
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// diffDiskFiles compares two files on disk, a with b (diff --files).
func diffDiskFiles(a, b string) (fileDiffJSON, error) {
	read := func(p string) ([]byte, int64, error) {
		f, err := os.Open(p)
		if err != nil {
			return nil, 0, err
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			return nil, 0, err
		}
		if fi.IsDir() {
			return nil, 0, fmt.Errorf("%s is a folder: diff --files takes two files", p)
		}
		data, err := io.ReadAll(io.LimitReader(f, maxDiffText+1))
		return data, fi.Size(), err
	}
	d := fileDiffJSON{Path: filepath.ToSlash(b), From: filepath.ToSlash(a), Status: "modified", Kind: "binary"}
	da, sa, err := read(a)
	if err != nil {
		return d, err
	}
	db, sb, err := read(b)
	if err != nil {
		return d, err
	}
	d.SizeOld, d.SizeNew = sa, sb
	if bytes.Equal(da, db) && sa == sb {
		d.Status = "same"
	}
	var setErr error
	describe(&d, b, da, db, true, func() *diff.SetDiff {
		if !isSetFile(a) {
			return nil
		}
		sa, err := als.Load(a)
		if err != nil {
			setErr = err
			return nil
		}
		sb, err := als.Load(b)
		if err != nil {
			setErr = err
			return nil
		}
		return diff.Diff(sa, sb)
	})
	return d, setErr
}

// diffSetFiles compares two Live Sets on disk (r3v diff a.als b.als).
func diffSetFiles(a, b string) error {
	sa, err := als.Load(a)
	if err != nil {
		return err
	}
	sb, err := als.Load(b)
	if err != nil {
		return err
	}
	fmt.Println(diff.Diff(sa, sb).Render())
	return nil
}
