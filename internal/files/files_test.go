package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandGlobsAndDeduplicates(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.zip")
	second := filepath.Join(dir, "second.zip")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	input := strings.Join([]string{filepath.Join(dir, "*.zip"), "", first}, "\n")
	got, err := Expand(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("Expand() = %#v", got)
	}
}

func TestExpandErrorsForNoMatchesAndDirectories(t *testing.T) {
	dir := t.TempDir()
	for _, pattern := range []string{filepath.Join(dir, "missing-*"), dir} {
		if _, err := Expand(pattern); err == nil || !strings.Contains(err.Error(), "matched no files") {
			t.Errorf("Expand(%q) error = %v", pattern, err)
		}
	}
}

func TestExpandEmpty(t *testing.T) {
	got, err := Expand("\n  \n")
	if err != nil || len(got) != 0 {
		t.Fatalf("Expand(empty) = %#v, %v", got, err)
	}
}
