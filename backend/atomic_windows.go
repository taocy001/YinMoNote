//go:build windows

package main

import (
	"errors"
	"syscall"
)

// isTransientRenameError reports whether a rename failure on Windows is likely
// transient — caused by another process briefly holding the file handle
// (antivirus scan, backup agent, etc.).
//
// ERROR_ACCESS_DENIED (5): another process has the file open with incompatible
// share flags and will release it shortly.
// ERROR_SHARING_VIOLATION (32): the classic "file is locked" error from Windows.
//
// Neither error appears on Linux/macOS (POSIX rename is atomic and unaffected
// by other processes holding the file open), so retries are Windows-only.
func isTransientRenameError(err error) bool {
	var e syscall.Errno
	if !errors.As(err, &e) {
		return false
	}
	return e == 5 || e == 32 // ERROR_ACCESS_DENIED || ERROR_SHARING_VIOLATION
}
