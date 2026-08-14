// Package config loads the tracked reference workflow configuration.
package config

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

//go:embed defaults/*.json
var defaults embed.FS

// ErrUnsupportedVersion means the requested version has no reviewed strategy.
var ErrUnsupportedVersion = errors.New("unsupported Minecraft version")

var sha256Pattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// Version configures Java and naming behavior for one supported release.
type Version struct {
	ID     string `json:"id"`
	Java   int    `json:"java"`
	Naming string `json:"naming"`
}

// Tool is a pinned external workflow artifact.
type Tool struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type versionsFile struct {
	Versions []Version `json:"versions"`
}

type toolsFile struct {
	Tools []Tool `json:"tools"`
}

// LoadVersions reads supported versions from configDir or the embedded defaults.
func LoadVersions(configDir string) (map[string]Version, error) {
	var value versionsFile
	if err := load(configDir, "versions.json", &value); err != nil {
		return nil, err
	}

	result := make(map[string]Version, len(value.Versions))
	for _, version := range value.Versions {
		if version.ID == "" || version.Naming == "" {
			return nil, errors.New("version configuration contains an empty id or naming strategy")
		}
		if _, exists := result[version.ID]; exists {
			return nil, fmt.Errorf("duplicate configured version %q", version.ID)
		}
		result[version.ID] = version
	}
	return result, nil
}

// LoadTools reads pinned tools from configDir or the embedded defaults.
func LoadTools(configDir string) (map[string]Tool, error) {
	var value toolsFile
	if err := load(configDir, "tools.json", &value); err != nil {
		return nil, err
	}

	result := make(map[string]Tool, len(value.Tools))
	for _, tool := range value.Tools {
		if tool.ID == "" || tool.URL == "" || tool.SHA256 == "" {
			return nil, errors.New("tool configuration contains an empty required field")
		}
		if !sha256Pattern.MatchString(tool.SHA256) {
			return nil, fmt.Errorf("tool %q has an invalid SHA-256", tool.ID)
		}
		if _, exists := result[tool.ID]; exists {
			return nil, fmt.Errorf("duplicate configured tool %q", tool.ID)
		}
		result[tool.ID] = tool
	}
	return result, nil
}

// RequireVersion returns one configured version or ErrUnsupportedVersion.
func RequireVersion(versions map[string]Version, id string) (Version, error) {
	version, ok := versions[id]
	if !ok {
		return Version{}, fmt.Errorf("%w: %s", ErrUnsupportedVersion, id)
	}
	return version, nil
}

func load(configDir, name string, value any) error {
	path := filepath.Join(configDir, name)
	var (
		data []byte
		err  error
	)
	if configDir == "" {
		path = filepath.ToSlash(filepath.Join("defaults", name))
		data, err = defaults.ReadFile(path)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
