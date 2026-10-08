package project

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/nonlabhq/r3v/internal/chunk"
	"github.com/nonlabhq/r3v/internal/remote"
)

// Stages reported while saving or downloading a project.
const (
	StageScanning    = "scanning"    // looking for changed files
	StageStoring     = "storing"     // copying changed files into the history
	StageChecking    = "checking"    // asking the team's storage which files it has
	StageUploading   = "uploading"   // sending files to the team
	StageDownloading = "downloading" // fetching files from the team
	StageExporting   = "exporting"   // writing a version to another folder
	// StageFinishing: the files are up; the version's folder lists, the
	// version and the branch follow (Done of Total steps).
	StageFinishing = "finishing"
	// StageHistory: getting the team's versions (Done so far; Total 0).
	StageHistory = "history"
	// StagePlacing: putting a version's files in the project folder.
	StagePlacing = "placing"
)

// Progress describes a long-running step: Done of Total items (Total is 0
// when unknown) and, for transfers, Bytes of TotalBytes.
type Progress struct {
	Stage             string
	Done, Total       int
	Bytes, TotalBytes int64
}

// ErrCancelled: the user stopped a commit before it was shared.
var ErrCancelled = fmt.Errorf("cancelled (%w)", remote.ErrStopped)

// stopped says whether the step under way is to stop (Cancel, stop).
func (r *Repo) stopped() error {
	if r.Cancel != nil {
		if err := r.Cancel(); err != nil {
			return err
		}
	}
	if r.stop != nil {
		return r.stop()
	}
	return nil
}

// stopReader reads rd until the step is to stop.
type stopReader struct {
	r  *Repo
	rd io.Reader
}

func (s stopReader) Read(p []byte) (int, error) {
	if err := s.r.stopped(); err != nil {
		return 0, err
	}
	return s.rd.Read(p)
}

// hashFile hashes a file of the project, stopping when asked.
func (r *Repo) hashFile(abs string) (string, int64, error) {
	f, err := os.Open(abs)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	var w io.Writer = h
	var l *lister
	if fi, err := f.Stat(); err == nil && fi.Size() >= chunk.MinFile {
		l = newLister()
		w = io.MultiWriter(h, l)
	}
	n, err := io.Copy(w, stopReader{r, f})
	sum := hex.EncodeToString(h.Sum(nil))
	if l != nil {
		l.keep(r, sum, err)
	}
	if err != nil {
		return "", 0, err
	}
	return sum, n, nil
}

// storeFile copies a file of the project into the store, stopping when
// asked.
func (r *Repo) storeFile(abs string) (string, int64, error) {
	f, err := os.Open(abs)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	return r.Store.Put(stopReader{r, f})
}

// report calls r.OnProgress when set.
func (r *Repo) report(stage string, done, total int) {
	if r.OnProgress != nil {
		r.OnProgress(Progress{Stage: stage, Done: done, Total: total})
	}
}

// transfer tracks the bytes of one upload or download of several files.
// transfer counts an upload or download; files go several at a time, so
// it is safe for concurrent use (and reports one at a time).
type transfer struct {
	r                 *Repo
	stage             string
	mu                sync.Mutex
	done, total       int
	bytes, totalBytes int64
}

func (r *Repo) newTransfer(stage string, files int, totalBytes int64) *transfer {
	return &transfer{r: r, stage: stage, total: files, totalBytes: totalBytes}
}

func (t *transfer) report() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reportLocked()
}

func (t *transfer) reportLocked() {
	if t.r.OnProgress != nil {
		t.r.OnProgress(Progress{Stage: t.stage, Done: t.done, Total: t.total, Bytes: t.bytes, TotalBytes: t.totalBytes})
	}
}

// fileDone counts a finished file.
func (t *transfer) fileDone() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.done++
	t.reportLocked()
}

// shrink takes n bytes off the total: a file goes up compressed.
func (t *transfer) shrink(n int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalBytes -= n
	t.reportLocked()
}

// count adds n bytes moved (or found here instead).
func (t *transfer) count(n int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bytes += n
	t.reportLocked()
}

// reader counts what passes through rd. Size lets uploads stream a file of
// known size instead of reading it into memory first.
func (t *transfer) reader(rd io.Reader, size int64) *countingReader {
	return &countingReader{rd: rd, size: size, t: t}
}

type countingReader struct {
	rd   io.Reader
	size int64
	t    *transfer
	read int64
}

// undo takes back what was counted: the transfer is tried again.
func (c *countingReader) undo() {
	c.t.mu.Lock()
	c.t.bytes -= c.read
	c.read = 0
	c.t.reportLocked()
	c.t.mu.Unlock()
}

func (c *countingReader) Read(p []byte) (int, error) {
	if err := c.t.r.stopped(); err != nil {
		return 0, err
	}
	n, err := c.rd.Read(p)
	if n > 0 {
		c.t.mu.Lock()
		c.read += int64(n)
		c.t.bytes += int64(n)
		c.t.reportLocked()
		c.t.mu.Unlock()
	}
	return n, err
}

func (c *countingReader) Size() int64 { return c.size }

// knowSizes remembers the sizes of m's files, so downloads can tell how much
// is left.
func (r *Repo) knowSizes(ms ...*Manifest) {
	if r.sizes == nil {
		r.sizes = map[string]int64{}
	}
	for _, m := range ms {
		if m == nil {
			continue
		}
		for _, f := range m.Files {
			r.sizes[f.Hash] = f.Size
		}
		for _, f := range m.External {
			r.sizes[f.Hash] = f.Size
		}
	}
}

// SetsSignature changes whenever a set in the project root is saved or the
// workspace moves to another version. It only stats files, so it is cheap
// enough to poll every second.
func (r *Repo) SetsSignature() string {
	sets, _ := filepath.Glob(filepath.Join(r.Root, "*.als"))
	sort.Strings(sets)
	var b strings.Builder
	b.WriteString(r.Head())
	for _, s := range sets {
		if fi, err := os.Stat(s); err == nil {
			fmt.Fprintf(&b, "|%s:%d:%d", filepath.Base(s), fi.Size(), fi.ModTime().UnixNano())
		}
	}
	return b.String()
}
