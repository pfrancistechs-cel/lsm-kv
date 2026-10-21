package wal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWALAppendAndRecover(t *testing.T) {
	dir, err := os.MkdirTemp("", "wal-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	walPath := filepath.Join(dir, "test.wal")
	w, err := OpenWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}

	err = w.Append([]byte("alpha"), []byte("val1"), false)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	entries, err := RecoverWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}
