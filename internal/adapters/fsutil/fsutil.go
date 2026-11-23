// Package fsutil holds the filesystem primitives shared by the local
// adapters fsobject and fsdocs: atomic writes, exclusive creation, an
// advisory lock, and mapping of filesystem errors to the typed data errors.
// It is local-only and must never be imported by a gcp build.
//
// It does not model fsync ordering across files, network filesystems, or
// Windows; advisory locks (flock) need a local POSIX filesystem.
package fsutil

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

const (
	dirMode  = 0o755
	fileMode = 0o644
	// TempPrefix starts the name of every in-flight temporary file.
	TempPrefix = ".tmp-"
)

// OpenRoot validates root, creates it when missing, and returns it as an
// absolute path. It touches no other path.
func OpenRoot(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("data root is empty: %w", data.ErrInvalid)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("data root %q: %w", root, data.ErrInvalid)
	}
	if err := os.MkdirAll(abs, dirMode); err != nil {
		return "", MapErr("open data root", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", MapErr("open data root", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("data root %q is not a directory: %w", root, data.ErrInvalid)
	}
	return abs, nil
}

// MapErr maps a filesystem error to a typed data error. Context errors pass
// through wrapped; a missing file is ErrNotFound; everything else is
// ErrUnavailable.
func MapErr(op string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, data.ErrNotFound), errors.Is(err, data.ErrConflict),
		errors.Is(err, data.ErrInvalid), errors.Is(err, data.ErrUnavailable),
		errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s: %w", op, err)
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s: %w", op, data.ErrNotFound)
	default:
		return fmt.Errorf("%s: %w: %w", op, data.ErrUnavailable, err)
	}
}

// CleanRel validates a slash-separated relative path: no empty, ".", or ".."
// segments, no backslash or NUL, and no segment that starts with TempPrefix.
// It returns ErrInvalid otherwise.
func CleanRel(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00") {
		return fmt.Errorf("path %q: %w", p, data.ErrInvalid)
	}
	for seg := range strings.SplitSeq(p, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.HasPrefix(seg, TempPrefix) {
			return fmt.Errorf("path %q: %w", p, data.ErrInvalid)
		}
	}
	return nil
}

// writeTemp writes body to a new synced temporary file in dir (created when
// missing) and returns its path.
func writeTemp(dir string, body []byte) (string, error) {
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return "", err //nolint:wrapcheck // callers wrap with MapErr
	}
	f, err := os.CreateTemp(dir, TempPrefix+"*")
	if err != nil {
		return "", err //nolint:wrapcheck // callers wrap with MapErr
	}
	name := f.Name()
	if _, err = f.Write(body); err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(name, fileMode)
	}
	if err != nil {
		_ = os.Remove(name)
		return "", err //nolint:wrapcheck // callers wrap with MapErr
	}
	return name, nil
}

// WriteOnce creates path with body, atomically and exclusively: readers see
// the whole file or none, and when several writers race exactly one creates
// it. created is false when the path already exists; the file is untouched.
func WriteOnce(path string, body []byte) (created bool, err error) {
	tmp, err := writeTemp(filepath.Dir(path), body)
	if err != nil {
		return false, MapErr("write "+path, err)
	}
	defer func() { _ = os.Remove(tmp) }()
	if err = os.Link(tmp, path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, nil
		}
		return false, MapErr("write "+path, err)
	}
	syncDir(filepath.Dir(path))
	return true, nil
}

// Replace writes body to path by temp file, fsync, and atomic rename.
func Replace(path string, body []byte) error {
	tmp, err := writeTemp(filepath.Dir(path), body)
	if err != nil {
		return MapErr("write "+path, err)
	}
	if err = os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return MapErr("write "+path, err)
	}
	syncDir(filepath.Dir(path))
	return nil
}

// syncDir flushes a directory entry; failures are ignored because not every
// filesystem supports it and the data file itself is already synced.
func syncDir(dir string) {
	d, err := os.Open(dir) //nolint:gosec // dir is built from the validated data root
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}

// Lock takes an exclusive advisory lock on the file at path (created when
// missing) and returns the function that releases it. The lock excludes other
// goroutines and other processes.
func Lock(path string) (unlock func(), err error) {
	if err = os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return nil, MapErr("lock "+path, err)
	}
	//nolint:gosec // path is built from the validated data root
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, fileMode)
	if err != nil {
		return nil, MapErr("lock "+path, err)
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, MapErr("lock "+path, err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
