// Package catalog promotes validated compatibility evidence into version configuration.
package catalog

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/pipeline"
)

type reportKey struct {
	version string
	side    string
}

// Accept derives conservative validation limits from passing compatibility reports.
func Accept(versions []config.Version, reports []pipeline.CompatibilityReport) ([]config.Version, error) {
	return AcceptAt(versions, reports, time.Now())
}

// AcceptAt derives validation limits and records the UTC verification date.
func AcceptAt(versions []config.Version, reports []pipeline.CompatibilityReport, verifiedAt time.Time) ([]config.Version, error) {
	reportsBySide := make(map[reportKey]pipeline.CompatibilityReport, len(reports))
	for _, report := range reports {
		key := reportKey{version: report.Version, side: report.Side}
		if _, exists := reportsBySide[key]; exists {
			return nil, fmt.Errorf("version %s side %s has duplicate compatibility reports", report.Version, report.Side)
		}
		reportsBySide[key] = report
	}

	accepted := make([]config.Version, 0, len(versions))
	for _, version := range versions {
		updated := cloneVersion(version)
		sides := make([]string, 0, len(version.Sides))
		for side := range version.Sides {
			sides = append(sides, side)
		}
		sort.Strings(sides)
		for _, side := range sides {
			validation := version.Sides[side]
			report, ok := reportsBySide[reportKey{version: version.ID, side: side}]
			if !ok {
				return nil, fmt.Errorf("version %s side %s requires a passing compatibility report", version.ID, side)
			}
			if err := validateReport(version, side, validation, report); err != nil {
				return nil, err
			}
			validation.MinSources = acceptedMinimum(report.SourceRecords)
			validation.MinSymbols = acceptedMinimum(report.SymbolRecords)
			validation.RequiredClasses = append([]string(nil), validation.RequiredClasses...)
			updated.Sides[side] = validation
		}
		updated.VerifiedDate = verifiedAt.UTC().Format(time.DateOnly)
		accepted = append(accepted, updated)
	}
	return accepted, nil
}

func validateReport(version config.Version, side string, validation config.Validation, report pipeline.CompatibilityReport) error {
	label := fmt.Sprintf("version %s side %s compatibility report", version.ID, side)
	if !report.Passed {
		return fmt.Errorf("%s did not pass", label)
	}
	if report.Family != version.Family {
		return fmt.Errorf("%s family %q does not match configured family %q", label, report.Family, version.Family)
	}
	if report.Naming != version.Naming {
		return fmt.Errorf("%s naming %q does not match configured naming %q", label, report.Naming, version.Naming)
	}
	if report.JavaMajor != report.JavapMajor {
		return fmt.Errorf("%s Java major %d does not match javap major %d", label, report.JavaMajor, report.JavapMajor)
	}
	requiredJava := config.EffectiveJavaMajor(version.Java)
	if report.JavaMajor < requiredJava {
		return fmt.Errorf("%s Java major %d is below effective requirement %d", label, report.JavaMajor, requiredJava)
	}
	if report.JavapMajor < requiredJava {
		return fmt.Errorf("%s javap major %d is below effective requirement %d", label, report.JavapMajor, requiredJava)
	}
	if report.NamedClasses < 1 {
		return fmt.Errorf("%s has invalid named class count %d", label, report.NamedClasses)
	}
	if report.SourceRecords < validation.MinSources {
		return fmt.Errorf("%s source records %d are below configured requirement %d", label, report.SourceRecords, validation.MinSources)
	}
	if report.SymbolRecords < validation.MinSymbols {
		return fmt.Errorf("%s symbol records %d are below configured requirement %d", label, report.SymbolRecords, validation.MinSymbols)
	}
	if !slices.Equal(report.RequiredClasses, validation.RequiredClasses) {
		return fmt.Errorf("%s required classes do not match the configured required classes", label)
	}
	return nil
}

func acceptedMinimum(observed int) int {
	minimum := observed/10*9 + observed%10*9/10
	if minimum < 1 {
		return 1
	}
	return minimum
}

func cloneVersion(version config.Version) config.Version {
	cloned := version
	if version.Mapping != nil {
		mapping := *version.Mapping
		cloned.Mapping = &mapping
	}
	cloned.Sides = make(map[string]config.Validation, len(version.Sides))
	for side, validation := range version.Sides {
		validation.RequiredClasses = append([]string(nil), validation.RequiredClasses...)
		cloned.Sides[side] = validation
	}
	return cloned
}
