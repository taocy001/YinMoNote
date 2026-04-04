//go:build !windows

package main

// runAsWindowsService is a no-op on non-Windows platforms. It always returns
// false so that main() proceeds with the normal console startup path.
func runAsWindowsService(_ *NoteLibrary, _ string) bool { return false }
