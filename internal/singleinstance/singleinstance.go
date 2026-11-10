// Package singleinstance guards against a second collector process on the
// same user account: a pid lockfile at ~/.tokenledger/tokenledger.lock.
// A lock held by a dead process is taken over; a live one is reported.
package singleinstance

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Lock is an acquired pid lock.
type Lock struct {
	path    string
	file    *os.File
	release func()
}

// Acquire creates/takes the lock file via O_EXCL. If another live process
// holds it, it returns (nil, pid, nil) so the caller can hand off gracefully.
// A stale lock (dead pid, unparsable, or missing content) is removed and
// retried once; the O_EXCL create closes the two-process TOCTOU window.
func Acquire(path string) (*Lock, int, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, 0, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			pid := os.Getpid()
			if _, werr := f.WriteString(strconv.Itoa(pid) + "\n"); werr != nil {
				f.Close()
				return nil, 0, werr
			}
			l := &Lock{path: path, file: f, release: releaseFunc(path, f, pid)}
			return l, 0, nil
		}
		if !os.IsExist(err) {
			return nil, 0, err
		}
		// Held by someone: live holder wins, a stale one is replaced.
		if pid := HolderPID(path); pid > 0 && processAlive(pid) {
			return nil, pid, nil
		}
		_ = os.Remove(path)
	}
	return nil, 0, fmt.Errorf("could not acquire lock %s", path)
}

func releaseFunc(path string, f *os.File, pid int) func() {
	return func() {
		// Close the handle first: Windows refuses to remove open files.
		f.Close()
		// Only remove the file if it still holds our pid (no takeover race).
		if data, err := os.ReadFile(path); err == nil {
			if cur, err2 := strconv.Atoi(strings.TrimSpace(string(data))); err2 == nil && cur == pid {
				_ = os.Remove(path)
			}
		}
	}
}

// Release removes the lock file (idempotent).
func (l *Lock) Release() {
	if l != nil && l.release != nil {
		l.release()
	}
}

// HolderPID returns the pid recorded in the lock file (0 if none/invalid).
func HolderPID(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

// IsHeldByLiveProcess reports whether another live process owns the lock.
func IsHeldByLiveProcess(path string) (int, bool) {
	pid := HolderPID(path)
	return pid, pid > 0 && processAlive(pid)
}
