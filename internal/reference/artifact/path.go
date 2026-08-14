package artifact

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsafeReferenceDir means the requested output could escape its workspace.
var ErrUnsafeReferenceDir = errors.New("unsafe reference directory")

// ResolveReferenceDir resolves a workspace-local child path through symlinks.
func ResolveReferenceDir(workspaceRoot, requested string) (string, error) {
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root symlinks: %w", err)
	}

	target := requested
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve reference directory: %w", err)
	}
	target = filepath.Clean(target)

	resolvedParent, suffix, err := existingAncestor(target)
	if err != nil {
		return "", err
	}
	resolvedParent, err = filepath.EvalSymlinks(resolvedParent)
	if err != nil {
		return "", fmt.Errorf("resolve reference directory symlinks: %w", err)
	}
	target = filepath.Join(append([]string{resolvedParent}, suffix...)...)

	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", fmt.Errorf("compare reference directory: %w", err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: %s must be a child of %s", ErrUnsafeReferenceDir, target, root)
	}
	return target, nil
}

func existingAncestor(path string) (string, []string, error) {
	current := path
	var suffix []string
	for {
		_, err := os.Lstat(current)
		if err == nil {
			return current, suffix, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", nil, fmt.Errorf("inspect %s: %w", current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", nil, fmt.Errorf("%w: no existing parent for %s", ErrUnsafeReferenceDir, path)
		}
		suffix = append([]string{filepath.Base(current)}, suffix...)
		current = parent
	}
}
