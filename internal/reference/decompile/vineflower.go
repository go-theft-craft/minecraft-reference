package decompile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// VineflowerArgs returns the fixed non-interactive research option set.
func VineflowerArgs(tool, input, output string, libraries []string) []string {
	arguments := []string{
		"-jar", tool,
		"--folder",
		"--thread-count=1",
		"--log-level=warn",
		"--remove-bridge=false",
		"--remove-synthetic=false",
		"--bytecode-source-mapping=true",
		"--only=net/minecraft/",
	}
	for _, library := range libraries {
		arguments = append(arguments, "--add-external="+library)
	}
	return append(arguments, input, output)
}

// RunVineflower decompiles a jar when its stage fingerprint changed.
func RunVineflower(ctx context.Context, java, tool, input, output string, libraries []string) error {
	arguments := VineflowerArgs(tool, input, output, libraries)
	fingerprintArguments := append(slices.Clone(arguments), "workflow-source-scope-v1")
	fingerprint, err := stageFingerprint(fingerprintArguments, tool, input)
	if err != nil {
		return err
	}
	marker := filepath.Join(output, ".complete")
	if data, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(data)) == fingerprint {
		return nil
	}
	if err := os.RemoveAll(output); err != nil {
		return fmt.Errorf("clear stale source output: %w", err)
	}
	if err := os.MkdirAll(output, 0o750); err != nil {
		return fmt.Errorf("create source output: %w", err)
	}
	command := exec.CommandContext(ctx, java, arguments...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("vineflower decompile: %w", err)
	}
	if err := pruneSources(output); err != nil {
		return err
	}
	if err := os.WriteFile(marker, []byte(fingerprint+"\n"), 0o600); err != nil {
		return fmt.Errorf("write decompile marker: %w", err)
	}
	return nil
}

func pruneSources(output string) error {
	entries, err := os.ReadDir(output)
	if err != nil {
		return fmt.Errorf("read source output: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == "net" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(output, entry.Name())); err != nil {
			return fmt.Errorf("remove non-Minecraft source %s: %w", entry.Name(), err)
		}
	}
	netDir := filepath.Join(output, "net")
	entries, err = os.ReadDir(netDir)
	if err != nil {
		return fmt.Errorf("read net source output: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == "minecraft" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(netDir, entry.Name())); err != nil {
			return fmt.Errorf("remove non-Minecraft net source %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func stageFingerprint(arguments []string, files ...string) (string, error) {
	hasher := sha256.New()
	for _, argument := range arguments {
		_, _ = io.WriteString(hasher, argument)
		_, _ = hasher.Write([]byte{0})
	}
	for _, path := range files {
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
