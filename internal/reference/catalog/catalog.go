package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/artifact"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
)

// Candidate describes one new or replaced stable family representative.
type Candidate struct {
	Family string          `json:"family"`
	Old    *config.Version `json:"old,omitempty"`
	New    config.Version  `json:"new"`
}

// CandidateFile is a complete proposed version configuration with its changes.
type CandidateFile struct {
	Versions   []config.Version `json:"versions"`
	Candidates []Candidate      `json:"candidates"`

	hasCandidates bool
}

// HasCandidatesField reports whether the source file declared a candidate list.
func (file CandidateFile) HasCandidatesField() bool {
	return file.hasCandidates
}

// Discover selects changed stable family representatives and resolves only their metadata.
func Discover(
	ctx context.Context,
	releases []artifact.Release,
	current []config.Version,
	resolve func(context.Context, artifact.Release) (artifact.VersionMetadata, error),
) ([]Candidate, error) {
	currentByFamily := make(map[string]config.Version, len(current))
	for _, version := range current {
		currentByFamily[version.Family] = cloneVersion(version)
	}

	selected := make(map[string]artifact.Release)
	for _, release := range releases {
		family, ok := stableFamily(release)
		if !ok {
			continue
		}
		previous, exists := selected[family]
		if !exists || release.ReleaseTime.After(previous.ReleaseTime) ||
			release.ReleaseTime.Equal(previous.ReleaseTime) && compareVersionIDs(release.ID, previous.ID) > 0 {
			selected[family] = release
		}
	}

	families := make([]string, 0, len(selected))
	for family, release := range selected {
		old, exists := currentByFamily[family]
		if exists && old.ID == release.ID {
			continue
		}
		families = append(families, family)
	}
	sort.Slice(families, func(i, j int) bool { return compareNumericParts(families[i], families[j]) < 0 })

	candidates := make([]Candidate, 0, len(families))
	for _, family := range families {
		release := selected[family]
		metadata, err := resolve(ctx, release)
		if err != nil {
			return nil, fmt.Errorf("resolve metadata for %s: %w", release.ID, err)
		}
		if metadata.ID != release.ID {
			return nil, fmt.Errorf("metadata id mismatch for %s: got %q", release.ID, metadata.ID)
		}
		if metadata.JavaVersion.MajorVersion < 1 {
			return nil, fmt.Errorf("version %s metadata has invalid Java major %d", release.ID, metadata.JavaVersion.MajorVersion)
		}

		old, replacing := currentByFamily[family]
		version, err := candidateVersion(family, metadata, old, replacing)
		if err != nil {
			return nil, err
		}
		candidate := Candidate{Family: family, New: version}
		if replacing {
			oldCopy := cloneVersion(old)
			candidate.Old = &oldCopy
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func candidateVersion(family string, metadata artifact.VersionMetadata, old config.Version, replacing bool) (config.Version, error) {
	sides := make(map[string]config.Validation, 2)
	allMappings := true
	for _, side := range []string{"client", "server"} {
		if _, ok := metadata.Downloads[side]; !ok {
			continue
		}
		validation, exists := old.Sides[side]
		if !replacing || !exists {
			requiredClass := "Minecraft"
			if side == "server" {
				requiredClass = "MinecraftServer"
			}
			validation = config.Validation{MinSources: 1, MinSymbols: 1, RequiredClasses: []string{requiredClass}}
		}
		validation.RequiredClasses = append([]string(nil), validation.RequiredClasses...)
		sides[side] = validation
		if _, ok := metadata.MappingDownload(side); !ok {
			allMappings = false
		}
	}
	if len(sides) == 0 {
		return config.Version{}, fmt.Errorf("version %s metadata has no client or server download", metadata.ID)
	}
	naming := "identity"
	if allMappings {
		naming = "mojang"
	}
	return config.Version{
		ID:     metadata.ID,
		Family: family,
		Java:   metadata.JavaVersion.MajorVersion,
		Naming: naming,
		Sides:  sides,
	}, nil
}

func stableFamily(release artifact.Release) (string, bool) {
	if release.Type != "release" {
		return "", false
	}
	parts := strings.Split(release.ID, ".")
	if len(parts) < 2 {
		return "", false
	}
	values := make([]int, len(parts))
	for index, part := range parts {
		if part == "" {
			return "", false
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 || strconv.Itoa(value) != part {
			return "", false
		}
		values[index] = value
	}
	if values[0] < 1 {
		return "", false
	}
	return fmt.Sprintf("%d.%d", values[0], values[1]), true
}

func compareVersionIDs(left, right string) int {
	return compareNumericParts(left, right)
}

func compareNumericParts(left, right string) int {
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	length := len(leftParts)
	if len(rightParts) > length {
		length = len(rightParts)
	}
	for index := 0; index < length; index++ {
		leftValue, rightValue := 0, 0
		if index < len(leftParts) {
			leftValue, _ = strconv.Atoi(leftParts[index])
		}
		if index < len(rightParts) {
			rightValue, _ = strconv.Atoi(rightParts[index])
		}
		if leftValue < rightValue {
			return -1
		}
		if leftValue > rightValue {
			return 1
		}
	}
	return 0
}

// ApplyCandidates returns the complete proposed configuration.
func ApplyCandidates(current []config.Version, candidates []Candidate) []config.Version {
	byFamily := make(map[string]config.Version, len(current)+len(candidates))
	for _, version := range current {
		byFamily[version.Family] = cloneVersion(version)
	}
	for _, candidate := range candidates {
		byFamily[candidate.Family] = cloneVersion(candidate.New)
	}
	versions := make([]config.Version, 0, len(byFamily))
	for _, version := range byFamily {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool {
		comparison := compareNumericParts(versions[i].Family, versions[j].Family)
		if comparison != 0 {
			return comparison < 0
		}
		return compareVersionIDs(versions[i].ID, versions[j].ID) < 0
	})
	return versions
}

// ReadCandidateFile reads either a plain versions file or a discovery candidate file.
func ReadCandidateFile(path string) (CandidateFile, error) {
	versions, err := config.ReadVersionFile(path)
	if err != nil {
		return CandidateFile{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return CandidateFile{}, fmt.Errorf("read %s: %w", path, err)
	}
	var raw struct {
		Candidates *[]Candidate `json:"candidates"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return CandidateFile{}, fmt.Errorf("decode %s: %w", path, err)
	}
	file := CandidateFile{Versions: versions}
	if raw.Candidates == nil {
		return file, nil
	}
	file.hasCandidates = true
	file.Candidates = append([]Candidate(nil), (*raw.Candidates)...)
	if file.Candidates == nil {
		file.Candidates = []Candidate{}
	}
	if err := validateCandidates(file); err != nil {
		return CandidateFile{}, fmt.Errorf("validate %s: %w", path, err)
	}
	return file, nil
}

// WriteCandidateFile atomically writes a proposed version configuration and candidate list.
func WriteCandidateFile(path string, versions []config.Version, candidates []Candidate) error {
	directory := filepath.Dir(path)
	validation, err := os.CreateTemp(directory, ".candidate-validation-*.json")
	if err != nil {
		return fmt.Errorf("create candidate validation file: %w", err)
	}
	validationPath := validation.Name()
	if err := validation.Close(); err != nil {
		_ = os.Remove(validationPath)
		return fmt.Errorf("close candidate validation file: %w", err)
	}
	defer func() { _ = os.Remove(validationPath) }()
	if err := config.WriteVersionFile(validationPath, versions); err != nil {
		return err
	}
	validatedVersions, err := config.ReadVersionFile(validationPath)
	if err != nil {
		return err
	}

	file := CandidateFile{
		Versions:      validatedVersions,
		Candidates:    append([]Candidate(nil), candidates...),
		hasCandidates: true,
	}
	if file.Candidates == nil {
		file.Candidates = []Candidate{}
	}
	if err := validateCandidates(file); err != nil {
		return err
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	data = append(data, '\n')
	return writeAtomic(path, data)
}

func validateCandidates(file CandidateFile) error {
	versions := make(map[string]config.Version, len(file.Versions))
	for _, version := range file.Versions {
		versions[version.Family] = version
	}
	seen := make(map[string]struct{}, len(file.Candidates))
	for _, candidate := range file.Candidates {
		if candidate.Family == "" || candidate.New.Family != candidate.Family {
			return fmt.Errorf("candidate family %q does not match new version family %q", candidate.Family, candidate.New.Family)
		}
		if _, exists := seen[candidate.Family]; exists {
			return fmt.Errorf("candidate family %q is duplicated", candidate.Family)
		}
		seen[candidate.Family] = struct{}{}
		version, exists := versions[candidate.Family]
		if !exists || !reflect.DeepEqual(version, candidate.New) {
			return fmt.Errorf("candidate %s new version %s is absent from proposed versions", candidate.Family, candidate.New.ID)
		}
		if candidate.Old != nil && candidate.Old.Family != candidate.Family {
			return fmt.Errorf("candidate family %q does not match old version family %q", candidate.Family, candidate.Old.Family)
		}
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".candidate-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary candidate file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("set temporary candidate file permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write temporary candidate file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary candidate file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary candidate file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
