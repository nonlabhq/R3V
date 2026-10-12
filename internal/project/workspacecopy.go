package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nonlabhq/r3v/internal/store"
)

// A new workspace gets copies of the folders .r3v.yaml names under
// workspaces: copy (build outputs the rules leave out: target/,
// node_modules/), so it needn't build from nothing. Copied file by file
// with the system's copy, a block clone on Dev Drive and ReFS (no room
// taken until a file changes). Links aren't copied: one would point into
// the main copy.

const copyingDir = "copying" // in the workspace's .r3v: a folder being copied

// copyFolders copies the main copy's folders (r) that the rules name into
// the workspace ws. Each goes to .r3v/copying first and is moved into
// place when whole: stopped half-way, the workspace has it whole or not at
// all (and adding again copies it).
func (ws *Repo) copyFolders(r *Repo) (copied int, err error) {
	rules, _ := r.Profile()
	if rules == nil || len(rules.WorkspaceCopy) == 0 {
		return 0, nil
	}
	tmp := filepath.Join(ws.Dir, copyingDir)
	os.RemoveAll(tmp)
	defer os.RemoveAll(tmp)
	for _, rel := range r.foldersToCopy(rules.WorkspaceCopy) {
		dst := ws.Abs(rel)
		if _, err := os.Stat(dst); err == nil {
			continue // there already (a workspace added again)
		}
		if !rules.Ignored(rel, true) {
			continue // tracked: the workspace has its files
		}
		part := filepath.Join(tmp, strings.ReplaceAll(rel, "/", "~"))
		n, err := copyFolderFiles(r.Abs(rel), part)
		if err != nil {
			return copied, err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return copied, err
		}
		if err := os.Rename(part, dst); err != nil {
			return copied, err
		}
		copied += n
	}
	return copied, nil
}

// foldersToCopy are the folders of r the patterns name that exist: "a/b/"
// that one, "**/name/" any folder of that name (not looking inside one).
func (r *Repo) foldersToCopy(patterns []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(rel string) {
		if !seen[strings.ToLower(rel)] {
			seen[strings.ToLower(rel)] = true
			out = append(out, rel)
		}
	}
	for _, p := range patterns {
		p = strings.Trim(strings.TrimSpace(p), "/")
		name, any := strings.CutPrefix(p, "**/")
		if !any {
			if fi, err := os.Stat(r.Abs(p)); err == nil && fi.IsDir() {
				add(p)
			}
			continue
		}
		filepath.WalkDir(r.Root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() || path == r.Root {
				return nil
			}
			rel, _ := filepath.Rel(r.Root, path)
			rel = filepath.ToSlash(rel)
			if d.Name() == metaDir || d.Name() == ".git" {
				return filepath.SkipDir
			}
			if strings.EqualFold(d.Name(), name) {
				add(rel)
				return filepath.SkipDir
			}
			return nil
		})
	}
	return out
}

// copyFolderFiles copies the files of folder src into dst (made), links left out;
// it returns the number of files.
func copyFolderFiles(src, dst string) (int, error) {
	n := 0
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		to := filepath.Join(dst, rel)
		switch {
		case d.Type()&fs.ModeSymlink != 0 || d.Type()&fs.ModeIrregular != 0:
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		case d.IsDir():
			return os.MkdirAll(to, 0o755)
		}
		if err := store.CopyAtomic(path, to); err != nil {
			return err
		}
		n++
		if fileWritten != nil {
			fileWritten(to)
		}
		return nil
	})
	return n, err
}

// WorkspaceEstimate is the room a new workspace of this copy takes, before
// any block cloning: the files of the version it is on, and the build
// folders the rules name (workspaces: copy).
func (r *Repo) WorkspaceEstimate() (files, copies int64) {
	if m, err := r.Header(r.Head()); err == nil {
		files = m.TotalSize
	}
	rules, _ := r.Profile()
	if rules == nil {
		return files, 0
	}
	for _, rel := range r.foldersToCopy(rules.WorkspaceCopy) {
		filepath.WalkDir(r.Abs(rel), func(_ string, d fs.DirEntry, err error) error {
			if err == nil && d.Type().IsRegular() {
				if fi, err := d.Info(); err == nil {
					copies += fi.Size()
				}
			}
			return nil
		})
	}
	return files, copies
}
