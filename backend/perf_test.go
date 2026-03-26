package main

import (
	"fmt"
	"os"
	"testing"
)

func BenchmarkLoadNotes(b *testing.B) {
	// Setup 1000 empty files
	dataDir := "./data_perf"
	os.MkdirAll(dataDir, 0755)
	for i := 0; i < 1000; i++ {
		os.WriteFile(fmt.Sprintf("%s/20260318perf%016d.md", dataDir, i), []byte("test"), 0644)
	}
	defer os.RemoveAll(dataDir)

	lib, _ := NewNoteLibrary(dataDir, "assets", dataDir+"/config.json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Benchmarking the structure loading logic
		lib.GetStructure()
	}
}
