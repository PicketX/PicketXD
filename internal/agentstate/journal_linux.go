// SPDX-License-Identifier: GPL-3.0-or-later
package agentstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const maxCacheBytes = 6 << 20

type cacheRecord struct {
	Version      int       `json:"version"`
	ProviderType string    `json:"providerType"`
	ProviderID   string    `json:"providerID"`
	AgentRef     string    `json:"agentRef"`
	RuntimeKey   string    `json:"runtimeKey"`
	BootID       string    `json:"bootID"`
	Generation   uint64    `json:"generation"`
	AcceptedAt   time.Time `json:"acceptedAt"`
	PolicyHash   string    `json:"policyHash"`
	Policy       []byte    `json:"policy"`
}
type commitError struct {
	Published bool
	Err       error
}

func (e *commitError) Error() string { return e.Err.Error() }
func (e *commitError) Unwrap() error { return e.Err }
func privateFile(info os.FileInfo, dir bool) error {
	if info.IsDir() != dir || !dir && !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("state requires private regular files and directory: %s mode=%v", info.Name(), info.Mode())
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("state must be owned by the current user")
	}
	return nil
}
func lockJournal(path string) (*os.File, error) {
	dir := filepath.Dir(path)
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	info, e := os.Lstat(dir)
	if e != nil {
		return nil, e
	}
	if e = privateFile(info, true); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, e
	}
	info, e = f.Stat()
	if e == nil {
		e = privateFile(info, false)
	}
	if e == nil {
		e = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	}
	if e != nil {
		f.Close()
		return nil, fmt.Errorf("lock static state: %w", e)
	}
	return f, nil
}
func unlockJournal(f *os.File) error {
	if f == nil {
		return nil
	}
	return errors.Join(unix.Flock(int(f.Fd()), unix.LOCK_UN), f.Close())
}
func readJournal(path string) (cacheRecord, bool, error) {
	var r cacheRecord
	f, e := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if os.IsNotExist(e) {
		return r, false, nil
	}
	if e != nil {
		return r, false, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return r, false, e
	}
	if e = privateFile(info, false); e != nil {
		return r, false, e
	}
	data, e := io.ReadAll(io.LimitReader(f, maxCacheBytes+1))
	if e != nil {
		return r, false, e
	}
	if len(data) > maxCacheBytes {
		return r, false, fmt.Errorf("cache exceeds size bound")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e = d.Decode(&r); e != nil {
		return r, false, e
	}
	canonical, e := json.Marshal(r)
	if e != nil {
		return r, false, e
	}
	// The private journal writer always publishes canonical JSON. This also
	// rejects field aliases, duplicates, nulls, trailing values and corrupt UTF-8.
	if !bytes.Equal(bytes.TrimSpace(data), canonical) {
		return r, false, fmt.Errorf("cache is not canonical")
	}
	return r, true, nil
}
func writeJournal(path string, r cacheRecord) error {
	data, e := json.Marshal(r)
	if e != nil {
		return &commitError{Err: e}
	}
	if len(data) > maxCacheBytes {
		return &commitError{Err: fmt.Errorf("cache exceeds size bound")}
	}
	dir := filepath.Dir(path)
	f, e := os.CreateTemp(dir, ".picketx-state-*")
	if e != nil {
		return &commitError{Err: e}
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(data); e == nil {
		e = f.Sync()
	}
	e = errors.Join(e, f.Close())
	if e != nil {
		return &commitError{Err: e}
	}
	if e = os.Rename(tmp, path); e != nil {
		return &commitError{Err: e}
	}
	d, e := os.Open(dir)
	if e != nil {
		return &commitError{Published: true, Err: e}
	}
	e = errors.Join(d.Sync(), d.Close())
	if e != nil {
		return &commitError{Published: true, Err: e}
	}
	return nil
}
