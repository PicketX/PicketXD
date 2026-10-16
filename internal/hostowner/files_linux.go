// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// Accepted 256-byte identities may expand sixfold in canonical JSON.
// This covers all 4096 entries even at the maximum escaped identity size.
const maxRecordBytes = 32 << 20

type publicationError struct {
	Published bool
	Err       error
}

func (e *publicationError) Error() string { return e.Err.Error() }
func (e *publicationError) Unwrap() error { return e.Err }

type journal struct {
	dir        *os.File
	name, path string
}

func namespaceSocket() (func() error, error) {
	fd, err := unix.Socket(unix.AF_UNIX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	if err = unix.Bind(fd, &unix.SockaddrUnix{Name: "@picketx.hostowner.v1"}); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("namespace singleton socket: %w", err)
	}
	return func() error { return unix.Close(fd) }, nil
}
func private(info os.FileInfo, dir bool) error {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != uint32(os.Geteuid()) || info.IsDir() != dir || (!dir && (!info.Mode().IsRegular() || st.Nlink != 1)) || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("ownership state requires current-owner private regular files/directory")
	}
	return nil
}

// Every lookup/create is relative to a held parent descriptor. No Lstat-then-
// pathname traversal can redirect creation, lock, read, rename or fsync.
func walkDirectory(path string, mkdir func(int, string, uint32) error) (out *os.File, err error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("absolute directory required")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	current := os.NewFile(uintptr(fd), "/")
	defer func() {
		if current != nil {
			current.Close()
		}
	}()
	for _, part := range strings.Split(strings.TrimPrefix(filepath.Clean(path), "/"), "/") {
		if part == "" {
			continue
		}
		child, openErr := unix.Openat(int(current.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if errors.Is(openErr, unix.ENOENT) {
			if err = mkdir(int(current.Fd()), part, 0700); err != nil && !errors.Is(err, unix.EEXIST) {
				return nil, err
			}
			child, openErr = unix.Openat(int(current.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		}
		if openErr != nil {
			return nil, fmt.Errorf("open ownership directory component %s: %w", part, openErr)
		}
		next := os.NewFile(uintptr(child), part)
		info, statErr := next.Stat()
		if statErr != nil {
			next.Close()
			return nil, statErr
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || !info.IsDir() || (st.Uid != 0 && st.Uid != uint32(os.Geteuid())) || (info.Mode().Perm()&0022 != 0 && info.Mode()&os.ModeSticky == 0) {
			next.Close()
			return nil, fmt.Errorf("unsafe ownership directory component %s", part)
		}
		if err = errors.Join(next.Sync(), current.Sync()); err != nil {
			next.Close()
			return nil, err
		}
		if err = current.Close(); err != nil {
			next.Close()
			current = nil
			return nil, err
		}
		current = next
	}
	info, err := current.Stat()
	if err != nil {
		return nil, err
	}
	if err = private(info, true); err != nil {
		return nil, err
	}
	out = current
	current = nil
	return out, nil
}
func openAt(dir *os.File, name string, flags int) (*os.File, error) {
	fd, err := unix.Openat(int(dir.Fd()), name, flags|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}
func lockAt(dir *os.File, name string) (func() error, error) {
	f, err := openAt(dir, name, unix.O_CREAT|unix.O_RDWR)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err == nil {
		err = private(info, false)
	}
	if err == nil {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("ownership singleton lock: %w", err)
	}
	return func() error { return errors.Join(unix.Flock(int(f.Fd()), unix.LOCK_UN), f.Close()) }, nil
}
func acquireLocks(state, root string, ns namespace) (releases []func() error, j journal, err error) {
	defer func() {
		if err != nil {
			for i := len(releases) - 1; i >= 0; i-- {
				err = errors.Join(err, releases[i]())
			}
			releases = nil
		}
	}()
	lockDir, err := walkDirectory(root, unix.Mkdirat)
	if err != nil {
		return nil, j, err
	}
	releases = append(releases, lockDir.Close)
	stateDir, err := walkDirectory(filepath.Dir(state), unix.Mkdirat)
	if err != nil {
		return releases, j, err
	}
	releases = append(releases, stateDir.Close)
	j = journal{dir: stateDir, name: filepath.Base(state), path: state}
	data, _ := json.Marshal(ns)
	hash := sha256.Sum256(data)
	release, err := lockAt(lockDir, fmt.Sprintf("netns-%x.lock", hash))
	if err != nil {
		return releases, j, err
	}
	releases = append(releases, release)
	release, err = lockAt(stateDir, j.name+".lock")
	if err != nil {
		return releases, j, err
	}
	releases = append(releases, release)
	return releases, j, nil
}
func (j journal) read() (record, bool, error) {
	var r record
	f, err := openAt(j.dir, j.name, unix.O_RDONLY)
	if errors.Is(err, unix.ENOENT) {
		return r, false, nil
	}
	if err != nil {
		return r, false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err == nil {
		err = private(info, false)
	}
	if err != nil {
		return r, false, err
	}
	data, err := io.ReadAll(io.LimitReader(f, maxRecordBytes+1))
	if err != nil {
		return r, false, err
	}
	if len(data) > maxRecordBytes {
		return r, false, fmt.Errorf("ownership journal exceeds bound")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&r); err != nil {
		return r, false, err
	}
	canonical, err := json.Marshal(r)
	if err != nil {
		return r, false, err
	}
	if !bytes.Equal(bytes.TrimSpace(data), canonical) {
		return r, false, fmt.Errorf("ownership journal is not canonical")
	}
	if err = validRecord(r); err != nil {
		return r, false, err
	}
	return r, true, nil
}
func readRecord(path string) (record, bool, error) {
	dir, err := walkDirectory(filepath.Dir(path), unix.Mkdirat)
	if err != nil {
		return record{}, false, err
	}
	defer dir.Close()
	return (journal{dir: dir, name: filepath.Base(path), path: path}).read()
}
func writeRecord(path string, r record) error { return writeRecordWithSync(path, r, nil) }
func writeRecordWithSync(path string, r record, sync func(string) error) error {
	dir, err := walkDirectory(filepath.Dir(path), unix.Mkdirat)
	if err != nil {
		return &publicationError{Err: err}
	}
	defer dir.Close()
	return (journal{dir: dir, name: filepath.Base(path), path: path}).write(r, sync)
}
func (j journal) write(r record, sync func(string) error) error {
	if err := validRecord(r); err != nil {
		return &publicationError{Err: err}
	}
	data, err := json.Marshal(r)
	if err != nil {
		return &publicationError{Err: err}
	}
	if len(data) > maxRecordBytes {
		return &publicationError{Err: fmt.Errorf("ownership journal exceeds bound")}
	}
	id, err := randomID()
	if err != nil {
		return &publicationError{Err: err}
	}
	name := ".picketx-owner-" + id
	f, err := openAt(j.dir, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL)
	if err != nil {
		return &publicationError{Err: err}
	}
	defer unix.Unlinkat(int(j.dir.Fd()), name, 0)
	n, err := f.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return &publicationError{Err: err}
	}
	if err = unix.Renameat(int(j.dir.Fd()), name, int(j.dir.Fd()), j.name); err != nil {
		return &publicationError{Err: err}
	}
	if sync != nil {
		err = sync(filepath.Dir(j.path))
	} else {
		err = j.dir.Sync()
	}
	if err != nil {
		return &publicationError{Published: true, Err: err}
	}
	return nil
}
