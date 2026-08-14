package pipeline

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/artifact"
)

func TestCleanRemovesOnlyValidatedChild(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "reference", "work")
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "fixture"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	removed, err := Clean(root, "reference/work")
	if err != nil {
		t.Fatal(err)
	}
	if removed != target {
		t.Fatalf("got %q, want %q", removed, target)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still exists: %v", err)
	}
}

func TestCleanRejectsRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	_, err := Clean(root, root)
	if !errors.Is(err, artifact.ErrUnsafeReferenceDir) {
		t.Fatalf("got %v, want ErrUnsafeReferenceDir", err)
	}
}
