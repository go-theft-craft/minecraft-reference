package mapping

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Remap runs pinned SpecialSource when its input fingerprint changed.
func Remap(ctx context.Context, java, tool, input, output, mapping string) error {
	fingerprint, err := remapFingerprint(tool, input, mapping)
	if err != nil {
		return err
	}
	marker := output + ".lock"
	if data, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(data)) == fingerprint {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		return fmt.Errorf("create remap directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(output), ".remap-*.jar")
	if err != nil {
		return fmt.Errorf("create remap output: %w", err)
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close remap output: %w", err)
	}
	defer func() { _ = os.Remove(temporaryPath) }()
	command := exec.CommandContext(
		ctx, java, "-jar", tool,
		"--in-jar", input,
		"--out-jar", temporaryPath,
		"--srg-in", mapping,
	)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("SpecialSource remap: %w", err)
	}
	if err := os.Rename(temporaryPath, output); err != nil {
		return fmt.Errorf("publish remapped jar: %w", err)
	}
	if err := os.WriteFile(marker, []byte(fingerprint+"\n"), 0o600); err != nil {
		return fmt.Errorf("write remap marker: %w", err)
	}
	return nil
}

func remapFingerprint(paths ...string) (string, error) {
	hasher := sha256.New()
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("fingerprint %s: %w", path, err)
		}
		if _, err := io.Copy(hasher, file); err != nil {
			_ = file.Close()
			return "", fmt.Errorf("fingerprint %s: %w", path, err)
		}
		if err := file.Close(); err != nil {
			return "", fmt.Errorf("fingerprint %s: %w", path, err)
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
