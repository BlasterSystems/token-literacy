//go:build windows

package codex

import "os"

// fileID has no inode equivalent on Windows; 0 keeps the cursor contract.
func fileID(fi os.FileInfo) int64 { return 0 }
