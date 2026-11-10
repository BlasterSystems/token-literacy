//go:build windows

package singleinstance

import "os"

// processAlive: on Windows, FindProcess succeeds for any pid; OpenProcess
// probing needs x/sys. The pid lockfile freshness check is best-effort here.
func processAlive(pid int) bool {
	_, err := os.FindProcess(pid)
	return err == nil
}
