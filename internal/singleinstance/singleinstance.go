// Package singleinstance guards against a second collector process on the
// same user account: a pid lockfile at ~/.tokenledger/tokenledger.lock.
// A lock held by a dead process is taken over; a live one is reported.
package singleinstance

import (
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

// Acquire creates/takes the lock file. If another live process holds it,
// it returns (nil, pid, nil) so the caller can hand off gracefully.
func Acquire(path string) (*Lock, int, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, 0, err
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if pid, perr := strconv.Atoi(strings.TrimSpace(string(data))); perr == nil && pid > 0 && processAlive(pid) {
			return nil, pid, nil
		}
		// Stale lock: take over.
		_ = os.Remove(path)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, 0, err
	}
	pid := os.Getpid()
	if _, err := f.WriteString(strconv.Itoa(pid) + "\n"); err != nil {
		f.Close()
		return nil, 0, err
	}
	l := &Lock{path: path, file: f, release: func() {
		// Close the handle first: Windows refuses to remove open files.
		f.Close()
		// Only remove the file if it still holds our pid (no takeover race).
		if data, err := os.ReadFile(path); err == nil {
			if cur, err2 := strconv.Atoi(strings.TrimSpace(string(data))); err2 == nil && cur == pid {
				_ = os.Remove(path)
			}
		}
	}}
	return l, 0, nil
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
