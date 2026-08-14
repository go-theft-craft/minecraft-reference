package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
)

const (
	readmeBeginMarker = "<!-- BEGIN GENERATED SUPPORTED VERSIONS -->"
	readmeEndMarker   = "<!-- END GENERATED SUPPORTED VERSIONS -->"
)

// UpdateREADME replaces only the generated supported-version table.
func UpdateREADME(source []byte, versions []config.Version) ([]byte, error) {
	text := string(source)
	if strings.Count(text, readmeBeginMarker) != 1 || strings.Count(text, readmeEndMarker) != 1 {
		return nil, errors.New("README must contain exactly one generated supported-versions marker pair")
	}
	begin := strings.Index(text, readmeBeginMarker)
	end := strings.Index(text, readmeEndMarker)
	if end < begin {
		return nil, errors.New("README generated supported-versions markers are out of order")
	}

	var table strings.Builder
	table.WriteString("\n| Family | Tested release | Released | Verified | Minimum JDK | Mapping source | Tested sides |\n")
	table.WriteString("| --- | --- | --- | --- | ---: | --- | --- |\n")
	sorted := append([]config.Version(nil), versions...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ReleaseDate != sorted[j].ReleaseDate {
			return sorted[i].ReleaseDate > sorted[j].ReleaseDate
		}
		comparison := compareNumericParts(sorted[i].Family, sorted[j].Family)
		if comparison != 0 {
			return comparison > 0
		}
		return compareVersionIDs(sorted[i].ID, sorted[j].ID) > 0
	})
	for _, version := range sorted {
		mapping, err := readableMapping(version.Naming)
		if err != nil {
			return nil, fmt.Errorf("version %s: %w", version.ID, err)
		}
		sides := make([]string, 0, len(version.Sides))
		for _, side := range []string{"client", "server"} {
			if version.SupportsSide(side) {
				sides = append(sides, side)
			}
		}
		if len(sides) == 0 {
			return nil, fmt.Errorf("version %s has no readable tested sides", version.ID)
		}
		_, _ = fmt.Fprintf(&table, "| %s | `%s` | %s | %s | %d | %s | %s |\n", version.Family, version.ID, readableDate(version.ReleaseDate), readableDate(version.VerifiedDate), config.EffectiveJavaMajor(version.Java), mapping, strings.Join(sides, " and "))
	}

	updated := make([]byte, 0, len(source)+table.Len())
	updated = append(updated, text[:begin+len(readmeBeginMarker)]...)
	updated = append(updated, table.String()...)
	updated = append(updated, text[end:]...)
	return updated, nil
}

func readableDate(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

// CheckREADME reports whether the generated table matches the versions.
func CheckREADME(source []byte, versions []config.Version) error {
	updated, err := UpdateREADME(source, versions)
	if err != nil {
		return err
	}
	if !bytes.Equal(source, updated) {
		return errors.New("README supported-versions table is out of date")
	}
	return nil
}

func readableMapping(naming string) (string, error) {
	switch naming {
	case "mcp":
		return "Pinned MCP mappings", nil
	case "mojang":
		return "Mojang client and server mappings", nil
	case "identity":
		return "Names distributed with the game", nil
	default:
		return "", fmt.Errorf("unsupported naming strategy %q", naming)
	}
}
