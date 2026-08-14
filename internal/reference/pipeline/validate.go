package pipeline

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/archive"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/index"
)

const maximumIndexRecordSize = 16 << 20

// CompatibilityReport records the portable evidence from one validated side.
type CompatibilityReport struct {
	Version         string   `json:"version"`
	Family          string   `json:"family"`
	Side            string   `json:"side"`
	JavaMajor       int      `json:"java_major"`
	JavapMajor      int      `json:"javap_major"`
	Naming          string   `json:"naming"`
	NamedClasses    int      `json:"named_classes"`
	SourceRecords   int      `json:"source_records"`
	SymbolRecords   int      `json:"symbol_records"`
	RequiredClasses []string `json:"required_classes"`
	Passed          bool     `json:"passed"`
}

type validationOptions struct {
	Version      config.Version
	Side         string
	Validation   config.Validation
	NamedJar     string
	SourcesIndex string
	SymbolsIndex string
	ReportPath   string
	JavaMajor    int
	JavapMajor   int
}

func validateOutput(options validationOptions) (CompatibilityReport, error) {
	if err := removeIfPresent(options.ReportPath); err != nil {
		return CompatibilityReport{}, fmt.Errorf("invalidate compatibility report: %w", err)
	}

	classes, err := archive.ListClasses(options.NamedJar)
	if err != nil {
		return CompatibilityReport{}, validationError(options, "read named classes", err)
	}
	namedClasses := make([]string, 0, len(classes))
	for _, class := range classes {
		if strings.HasPrefix(class, "net.minecraft.") {
			namedClasses = append(namedClasses, class)
		}
	}
	if len(namedClasses) == 0 {
		return CompatibilityReport{}, observedError(options, "named classes", 0, 1)
	}

	sourceRecords, minecraftSources, err := scanJSONLines[index.SourceFile](options.SourcesIndex, func(source index.SourceFile) bool {
		return strings.HasPrefix(source.Path, "net/minecraft/")
	})
	if err != nil {
		return CompatibilityReport{}, validationError(options, "read source index", err)
	}
	if minecraftSources == 0 {
		return CompatibilityReport{}, observedError(options, "Minecraft source records", 0, 1)
	}
	if sourceRecords < options.Validation.MinSources {
		return CompatibilityReport{}, observedError(options, "source records", sourceRecords, options.Validation.MinSources)
	}

	symbolRecords, minecraftSymbols, err := scanJSONLines[index.Symbol](options.SymbolsIndex, func(symbol index.Symbol) bool {
		return strings.HasPrefix(symbol.Owner, "net.minecraft.")
	})
	if err != nil {
		return CompatibilityReport{}, validationError(options, "read symbol index", err)
	}
	if minecraftSymbols == 0 {
		return CompatibilityReport{}, observedError(options, "Minecraft symbol records", 0, 1)
	}
	if symbolRecords < options.Validation.MinSymbols {
		return CompatibilityReport{}, observedError(options, "symbol records", symbolRecords, options.Validation.MinSymbols)
	}

	classSegments := make(map[string]struct{}, len(namedClasses))
	for _, class := range namedClasses {
		segment := class[strings.LastIndexByte(class, '.')+1:]
		classSegments[segment] = struct{}{}
	}
	for _, required := range options.Validation.RequiredClasses {
		if _, ok := classSegments[required]; !ok {
			return CompatibilityReport{}, observedError(options, "required class "+required, 0, 1)
		}
	}

	report := CompatibilityReport{
		Version:         options.Version.ID,
		Family:          options.Version.Family,
		Side:            options.Side,
		JavaMajor:       options.JavaMajor,
		JavapMajor:      options.JavapMajor,
		Naming:          options.Version.Naming,
		NamedClasses:    len(namedClasses),
		SourceRecords:   sourceRecords,
		SymbolRecords:   symbolRecords,
		RequiredClasses: append([]string(nil), options.Validation.RequiredClasses...),
		Passed:          true,
	}
	if err := writeJSON(options.ReportPath, report); err != nil {
		return CompatibilityReport{}, validationError(options, "write compatibility report", err)
	}
	return report, nil
}

func scanJSONLines[T any](path string, matches func(T) bool) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = file.Close() }()

	count := 0
	matching := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), maximumIndexRecordSize)
	for scanner.Scan() {
		var record T
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return 0, 0, fmt.Errorf("decode record %d: %w", count+1, err)
		}
		count++
		if matches(record) {
			matching++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}
	return count, matching, nil
}

func observedError(options validationOptions, evidence string, observed, required int) error {
	return fmt.Errorf("version %s side %s %s: observed %d, required %d", options.Version.ID, options.Side, evidence, observed, required)
}

func validationError(options validationOptions, action string, err error) error {
	return fmt.Errorf("version %s side %s %s: %w", options.Version.ID, options.Side, action, err)
}

func removeIfPresent(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
