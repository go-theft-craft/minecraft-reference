// Package decompile extracts executable jars and invokes the pinned decompiler.
package decompile

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/archive"
)

const versionsList = "META-INF/versions.list"

// ExecutableServer extracts and verifies the inner jar from a bundled server.
func ExecutableServer(downloaded, destination string) (string, error) {
	bundled, err := archive.HasFile(downloaded, versionsList)
	if err != nil {
		return "", err
	}
	if !bundled {
		return downloaded, nil
	}
	data, err := archive.ReadFile(downloaded, versionsList)
	if err != nil {
		return "", err
	}
	entry, digest, err := parseVersionList(data)
	if err != nil {
		return "", err
	}
	if err := archive.ExtractFile(downloaded, entry, destination); err != nil {
		return "", err
	}
	if err := verifySHA256(destination, digest); err != nil {
		_ = os.Remove(destination)
		return "", err
	}
	return destination, nil
}

func parseVersionList(data []byte) (string, string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var selectedPath, selectedDigest string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			return "", "", fmt.Errorf("invalid versions.list line %q", line)
		}
		if selectedPath != "" {
			return "", "", errors.New("versions.list contains more than one executable server")
		}
		selectedDigest = parts[0]
		selectedPath = "META-INF/versions/" + parts[2]
	}
	if err := scanner.Err(); err != nil {
		return "", "", fmt.Errorf("scan versions.list: %w", err)
	}
	if selectedPath == "" || len(selectedDigest) != sha256.Size*2 {
		return "", "", errors.New("versions.list has no valid executable server")
	}
	return selectedPath, selectedDigest, nil
}

func verifySHA256(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("bundled server SHA-256 mismatch: got %s, want %s", actual, expected)
	}
	return nil
}
