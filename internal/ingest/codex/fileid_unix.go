//go:build !windows

package codex

import (
	"os"
	"syscall"
)

// fileID returns the file's inode (0 when unavailable) for rotation detection.
func fileID(fi os.FileInfo) int64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return int64(st.Ino)
	}
	return 0
}
