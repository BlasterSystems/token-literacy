//go:build !windows

package singleinstance

import (
	"os"
	"syscall"
)

// processAlive probes pid liveness with signal 0.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
