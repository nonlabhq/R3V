package desktop

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nonlabhq/r3v/internal/project"
)

// The Files tab changes the project folder itself: renaming a file or
// folder, and copying files dropped from Explorer into it. Both check first
// and fail rather than overwrite anything or guess: names are checked, R3V's
// own folder (.r3v) is left alone, nothing already there is replaced, and
// nothing is renamed while the project's tool (Live) has the project open.

// RenameFile renames a file or folder of the project (rel, relative to
// root) to name, in the same folder, and returns its new relative path.
func (a *App) RenameFile(root, rel, name string) (string, error) {
	if !knownProject(root) {
		return "", errors.New("unknown project")
	}
	r, unlock, err := a.open(root)
	if err != nil {
		return "", err
	}
	defer unlock()
	return renameInProject(r, rel, name)
}

func renameInProject(r *project.Repo, rel, name string) (string, error) {
	rel = strings.Trim(filepath.ToSlash(rel), "/")
	if !insideProject(rel) {
		return "", errors.New("invalid path")
	}
	name = strings.TrimSpace(name)
	if err := checkName(name); err != nil {
		return "", err
	}
	src := r.Abs(rel)
	fi, err := os.Lstat(src)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%s isn't there any more", path.Base(rel))
		}
		return "", err
	}
	to := name
	if d := path.Dir(rel); d != "." {
		to = d + "/" + name
	}
	if to == rel {
		return rel, nil
	}
	dst := r.Abs(to)
	if err := free(src, dst, name); err != nil {
		return "", err
	}
	// Live (or the project's tool) keeps sets and samples open by path:
	// renaming one under it could lose them. Plain other files are fine.
	if fi.IsDir() || fileKind(r, rel) != "other" {
		if set := toolOpen(r); set != "" {
			if set == "?" {
				return "", errors.New("Live is running: close the project's set in Live first, then rename")
			}
			return "", fmt.Errorf("%s is open in Live: close it first, then rename", path.Base(set))
		}
	}
	// A Unity asset's .meta goes with it, or Unity loses what refers to it.
	meta := !fi.IsDir() && !strings.HasSuffix(strings.ToLower(rel), ".meta")
	if meta {
		if _, err := os.Lstat(src + ".meta"); err != nil {
			meta = false
		} else if err := free(src+".meta", dst+".meta", name+".meta"); err != nil {
			return "", err
		}
	}
	if err := renameNoReplace(src, dst); err != nil {
		return "", renameError(rel, err)
	}
	if meta {
		if err := renameNoReplace(src+".meta", dst+".meta"); err != nil {
			// Put the file back rather than leave it apart from its .meta.
			if back := renameNoReplace(dst, src); back != nil {
				return "", fmt.Errorf("renamed %s, but not its .meta file (%v): rename %s.meta by hand", path.Base(rel), err, name)
			}
			return "", renameError(rel+".meta", err)
		}
	}
	return to, nil
}

// free fails when something other than src is at dst (renaming a file to
// another case of its name is fine: it is the same file).
func free(src, dst, name string) error {
	di, err := os.Lstat(dst)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if si, err := os.Lstat(src); err == nil && os.SameFile(si, di) {
		return nil
	}
	return fmt.Errorf("there is already a file or folder named %s here", name)
}

// insideProject: a relative path in the project folder, not R3V's own.
func insideProject(rel string) bool {
	if !safeRel(rel) || rel == "." {
		return false
	}
	first, _, _ := strings.Cut(rel, "/")
	return !strings.EqualFold(first, ".r3v")
}

// checkName refuses names Windows can't have or that would leave the folder.
func checkName(name string) error {
	switch {
	case name == "":
		return errors.New("type a name")
	case name == "." || name == "..":
		return errors.New("that name can't be used")
	case strings.ContainsAny(name, `/\`):
		return errors.New("a name can't have / or \\ in it")
	case strings.ContainsAny(name, `<>:"|?*`) || strings.ContainsFunc(name, func(r rune) bool { return r < 32 }):
		return errors.New(`a name can't have any of < > : " | ? * in it`)
	case strings.HasSuffix(name, ".") || strings.HasSuffix(name, " "):
		return errors.New("a name can't end with a dot or a space")
	case strings.EqualFold(name, ".r3v"):
		return errors.New("that name is R3V's own")
	}
	base, _, _ := strings.Cut(strings.ToUpper(name), ".")
	switch base {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return fmt.Errorf("Windows keeps the name %s for itself", name)
	}
	return nil
}

// renameError says plainly why a rename failed.
func renameError(rel string, err error) error {
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("there is already a file or folder named %s here", path.Base(rel))
	}
	if inUseErr(err) {
		return fmt.Errorf("%s is open in another program: close it there, then try again", path.Base(rel))
	}
	return err
}

// DropResult tells what dropping files into the project did.
type DropResult struct {
	Copied  []string `json:"copied"`  // the new files and folders (relative paths)
	Clashes []string `json:"clashes"` // names already in the folder: nothing was copied
}

// CopyIntoProject copies files and folders (absolute paths, e.g. dropped
// from Explorer) into dir, a folder of the project ("" its top). When a name
// is already there nothing is copied and Clashes lists them.
func (a *App) CopyIntoProject(root, dir string, sources []string) (*DropResult, error) {
	if !knownProject(root) {
		return nil, errors.New("unknown project")
	}
	r, unlock, err := a.open(root)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return copyInto(r, dir, sources)
}

func copyInto(r *project.Repo, dir string, sources []string) (*DropResult, error) {
	dir = strings.Trim(filepath.ToSlash(dir), "/")
	if dir != "" && !insideProject(dir) {
		return nil, errors.New("invalid folder")
	}
	target := r.Root
	if dir != "" {
		target = r.Abs(dir)
	}
	if fi, err := os.Stat(target); err != nil || !fi.IsDir() {
		return nil, errors.New("that folder isn't there any more")
	}
	out := &DropResult{Copied: []string{}, Clashes: []string{}}
	seen := map[string]bool{}
	for _, s := range sources {
		s = filepath.Clean(s)
		if !filepath.IsAbs(s) {
			return nil, errors.New("invalid path")
		}
		fi, err := os.Lstat(s)
		if err != nil {
			return nil, fmt.Errorf("%s isn't there", filepath.Base(s))
		}
		if fi.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s is a link: copy what it points to instead", filepath.Base(s))
		}
		if fi.IsDir() && within(s, target) {
			return nil, fmt.Errorf("%s can't be copied into itself", filepath.Base(s))
		}
		if within(r.Dir, s) {
			return nil, errors.New("R3V's own files (.r3v) can't be copied")
		}
		name := filepath.Base(s)
		key := strings.ToLower(name)
		if _, err := os.Lstat(filepath.Join(target, name)); err == nil || seen[key] {
			out.Clashes = append(out.Clashes, name)
		}
		seen[key] = true
	}
	if len(out.Clashes) > 0 {
		return out, nil
	}
	// Copied aside in R3V's folder first (same drive, tidied when left by a
	// crash), then moved in whole: a file is there complete or not at all.
	tmpRoot := filepath.Join(r.Dir, "objects", "tmp")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(tmpRoot, "drop-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	for _, s := range sources {
		name := filepath.Base(filepath.Clean(s))
		if err := copyTree(filepath.Clean(s), filepath.Join(stage, name)); err != nil {
			return out, fmt.Errorf("copying %s: %w", name, err)
		}
	}
	for _, s := range sources {
		name := filepath.Base(filepath.Clean(s))
		if err := renameNoReplace(filepath.Join(stage, name), filepath.Join(target, name)); err != nil {
			return out, renameError(name, err)
		}
		rel := name
		if dir != "" {
			rel = dir + "/" + name
		}
		out.Copied = append(out.Copied, rel)
	}
	return out, nil
}

// within: p is dir or inside it.
func within(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// copyTree copies a file, or a folder and what is in it (links are left out).
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		to := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(to, 0o755)
		case d.Type()&fs.ModeSymlink != 0 || !d.Type().IsRegular():
			return nil
		}
		return copyFile(p, to)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, fi.ModTime(), fi.ModTime())
}
