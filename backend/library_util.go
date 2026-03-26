package main

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// atomicWriteFile writes data to path using a temp-file + rename pattern so that
// a crash mid-write cannot leave the target file in a partially written state.
// The final file is created with the given permission bits.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// extractNoteTitle reads up to the first 512 bytes of a note to extract its title.
// Encrypted notes (ENC1: prefix) return an empty string — the server cannot decrypt
// them and must not attempt to guess a title from ciphertext. The client is responsible
// for supplying the decrypted title through the structure metadata.
func extractNoteTitle(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	// 512 bytes covers ~170 CJK characters (3 bytes each), enough to extract a meaningful
	// title from any note without reading the full file.
	const titlePeekBytes = 512
	buf := make([]byte, titlePeekBytes)
	n, _ := f.Read(buf)
	content := string(buf[:n])
	// The peek may end in the middle of a multibyte UTF-8 sequence. Back off one byte at a
	// time until the string is valid UTF-8. At most 3 iterations needed for any UTF-8 input.
	for !utf8.ValidString(content) && len(content) > 0 {
		content = content[:len(content)-1]
	}
	if strings.HasPrefix(content, "ENC1:") {
		return ""
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return strings.TrimLeft(strings.TrimPrefix(line, "#"), " ")
	}
	return ""
}
