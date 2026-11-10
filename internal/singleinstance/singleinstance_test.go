package singleinstance

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAcquireAndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokenledger.lock")
	lock, holder, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if lock == nil || holder != 0 {
		t.Fatalf("lock = %v holder = %d", lock, holder)
	}
	if pid, held := IsHeldByLiveProcess(path); !held || pid != os.Getpid() {
		t.Fatalf("held = %v pid = %d", held, pid)
	}
	lock.Release()
	if _, held := IsHeldByLiveProcess(path); held {
		t.Fatal("lock still held after release")
	}
	lock.Release() // idempotent
}

func TestSecondAcquireReportsHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokenledger.lock")
	first, _, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	second, holder, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if second != nil {
		t.Fatal("second acquire must not take the lock")
	}
	if holder != os.Getpid() {
		t.Fatalf("holder = %d, want %d", holder, os.Getpid())
	}
	first.Release()
}

func TestStaleLockTakenOver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokenledger.lock")
	if err := os.WriteFile(path, []byte(strconv.Itoa(999999)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, holder, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if lock == nil || holder != 0 {
		t.Fatalf("stale lock not taken over: lock=%v holder=%d", lock, holder)
	}
	lock.Release()
}

func TestHolderPIDInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokenledger.lock")
	if err := os.WriteFile(path, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, held := IsHeldByLiveProcess(path); held {
		t.Fatal("garbage lock reported as held")
	}
}
