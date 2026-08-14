package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveReferenceDir(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveReferenceDir(root, "reference/work")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "reference", "work")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveReferenceDirRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{root, filepath.Dir(root), ".."} {
		_, err := ResolveReferenceDir(root, path)
		if !errors.Is(err, ErrUnsafeReferenceDir) {
			t.Errorf("ResolveReferenceDir(%q) error = %v, want ErrUnsafeReferenceDir", path, err)
		}
	}
}

func TestResolveReferenceDirRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveReferenceDir(root, filepath.Join("escape", "work"))
	if !errors.Is(err, ErrUnsafeReferenceDir) {
		t.Fatalf("got %v, want ErrUnsafeReferenceDir", err)
	}
}
