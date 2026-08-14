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
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed defaults/*.json
var defaults embed.FS

// ErrUnsupportedVersion means the requested version has no reviewed strategy.
var ErrUnsupportedVersion = errors.New("unsupported Minecraft version")

// MinimumToolchainJavaMajor is required to run the pinned Vineflower release.
const MinimumToolchainJavaMajor = 17

// EffectiveJavaMajor combines Minecraft's declared requirement with the
// reference toolchain's runtime requirement.
func EffectiveJavaMajor(minecraftMajor int) int {
	if minecraftMajor < MinimumToolchainJavaMajor {
		return MinimumToolchainJavaMajor
	}
	return minecraftMajor
}

var sha256Pattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// Mapping configures the mapping artifacts used for one version family.
type Mapping struct {
	Format    string `json:"format"`
	Tool      string `json:"tool,omitempty"`
	SRGTool   string `json:"srg_tool,omitempty"`
	NamesTool string `json:"names_tool,omitempty"`
}

// Validation contains the minimum evidence required for one version side.
type Validation struct {
	MinSources      int      `json:"min_sources"`
	MinSymbols      int      `json:"min_symbols"`
	RequiredClasses []string `json:"required_classes"`
}

// Version configures one supported release and its validation evidence.
type Version struct {
	ID           string                `json:"id"`
	Family       string                `json:"family"`
	Java         int                   `json:"java"`
	ReleaseDate  string                `json:"release_date,omitempty"`
	VerifiedDate string                `json:"verified_date,omitempty"`
	Naming       string                `json:"naming"`
	Mapping      *Mapping              `json:"mapping,omitempty"`
	Sides        map[string]Validation `json:"sides"`
}

// SupportsSide reports whether reference evidence is configured for side.
func (v Version) SupportsSide(side string) bool {
	_, ok := v.Sides[side]
	return ok
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

// ReadVersionFile reads and validates a standalone versions.json file.
func ReadVersionFile(path string) ([]Version, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var value versionsFile
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if err := validateVersions(value.Versions); err != nil {
		return nil, err
	}
	return value.Versions, nil
}

// WriteVersionFile validates and atomically writes a standalone versions.json file.
func WriteVersionFile(path string, versions []Version) error {
	if err := validateVersions(versions); err != nil {
		return err
	}
	sorted := append([]Version(nil), versions...)
	sort.SliceStable(sorted, func(i, j int) bool {
		comparison := compareFamilies(sorted[i].Family, sorted[j].Family)
		if comparison != 0 {
			return comparison < 0
		}
		return sorted[i].ID < sorted[j].ID
	})

	data, err := json.MarshalIndent(versionsFile{Versions: sorted}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	data = append(data, '\n')

	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".versions-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary versions file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("set temporary versions file permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write temporary versions file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary versions file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary versions file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

// LoadVersions reads supported versions from configDir or the embedded defaults.
func LoadVersions(configDir string) (map[string]Version, error) {
	var value versionsFile
	if err := load(configDir, "versions.json", &value); err != nil {
		return nil, err
	}
	if err := validateVersions(value.Versions); err != nil {
		return nil, err
	}

	result := make(map[string]Version, len(value.Versions))
	for _, version := range value.Versions {
		result[version.ID] = version
	}
	return result, nil
}

func validateVersions(versions []Version) error {
	ids := make(map[string]struct{}, len(versions))
	families := make(map[string]string, len(versions))
	for _, version := range versions {
		label := version.ID
		if label == "" {
			label = "<empty>"
		}
		if version.ID == "" {
			return fmt.Errorf("version %q field id must not be empty", label)
		}
		if version.Java < 1 {
			return fmt.Errorf("version %q field java has invalid Java requirement %d", version.ID, version.Java)
		}
		if err := validateDate(version.ID, "release_date", version.ReleaseDate); err != nil {
			return err
		}
		if err := validateDate(version.ID, "verified_date", version.VerifiedDate); err != nil {
			return err
		}
		if version.Family == "" {
			return fmt.Errorf("version %q field family must not be empty", version.ID)
		}
		if _, err := familyParts(version.Family); err != nil {
			return fmt.Errorf("version %q field family is invalid: %w", version.ID, err)
		}
		if previous, exists := families[version.Family]; exists {
			return fmt.Errorf("version %q field family duplicates version %q (%s)", version.ID, previous, version.Family)
		}
		families[version.Family] = version.ID
		if _, exists := ids[version.ID]; exists {
			return fmt.Errorf("version %q field id is duplicated", version.ID)
		}
		ids[version.ID] = struct{}{}

		switch version.Naming {
		case "mcp":
			if err := validateMCPMapping(version); err != nil {
				return err
			}
		case "mojang", "identity":
		default:
			return fmt.Errorf("version %q field naming has unsupported strategy %q", version.ID, version.Naming)
		}
		if len(version.Sides) == 0 {
			return fmt.Errorf("version %q field sides must contain at least one side", version.ID)
		}
		for side, validation := range version.Sides {
			if side != "client" && side != "server" {
				return fmt.Errorf("version %q field sides.%s is unsupported", version.ID, side)
			}
			if validation.MinSources < 1 {
				return fmt.Errorf("version %q field sides.%s.min_sources must be positive (got %d)", version.ID, side, validation.MinSources)
			}
			if validation.MinSymbols < 1 {
				return fmt.Errorf("version %q field sides.%s.min_symbols must be positive (got %d)", version.ID, side, validation.MinSymbols)
			}
		}
	}
	return nil
}

func validateDate(versionID, field, value string) error {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil || parsed.Format(time.DateOnly) != value {
		return fmt.Errorf("version %q field %s must use YYYY-MM-DD", versionID, field)
	}
	return nil
}

func validateMCPMapping(version Version) error {
	if version.Mapping == nil {
		return fmt.Errorf("version %q field mapping must be provided for mcp naming", version.ID)
	}
	if version.Mapping.Format == "" {
		return fmt.Errorf("version %q field mapping.format must not be empty", version.ID)
	}
	switch version.Mapping.Format {
	case "tiny-v1":
		if version.Mapping.Tool == "" {
			return fmt.Errorf("version %q field mapping.tool is required for tiny-v1", version.ID)
		}
	case "srg-csv":
		if version.Mapping.SRGTool == "" {
			return fmt.Errorf("version %q field mapping.srg_tool is required for srg-csv", version.ID)
		}
		if version.Mapping.NamesTool == "" {
			return fmt.Errorf("version %q field mapping.names_tool is required for srg-csv", version.ID)
		}
	default:
		return fmt.Errorf("version %q field mapping.format has unsupported format %q", version.ID, version.Mapping.Format)
	}
	return nil
}

func familyParts(family string) ([]int, error) {
	parts := strings.Split(family, ".")
	if len(parts) != 2 {
		return nil, errors.New("must contain exactly two numeric components")
	}
	result := make([]int, len(parts))
	for index, part := range parts {
		if part == "" {
			return nil, errors.New("must contain only numeric components")
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return nil, errors.New("must contain only numeric components")
		}
		result[index] = value
	}
	return result, nil
}

func compareFamilies(left, right string) int {
	leftParts, _ := familyParts(left)
	rightParts, _ := familyParts(right)
	for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
		if leftParts[index] < rightParts[index] {
			return -1
		}
		if leftParts[index] > rightParts[index] {
			return 1
		}
	}
	switch {
	case len(leftParts) < len(rightParts):
		return -1
	case len(leftParts) > len(rightParts):
		return 1
	default:
		return 0
	}
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
