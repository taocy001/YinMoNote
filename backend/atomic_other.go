//go:build !windows

package main

// isTransientRenameError always returns false on non-Windows platforms.
// POSIX rename(2) is atomic and unaffected by other processes holding the
// target file open, so transient rename failures do not occur.
func isTransientRenameError(_ error) bool { return false }
