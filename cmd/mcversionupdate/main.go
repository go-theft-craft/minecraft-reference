// Command mcversionupdate updates the reviewed Minecraft version catalog.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/catalog"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/pipeline"
)

const usageText = `mcversionupdate promotes compatibility evidence into the version catalog.

Usage:
  mcversionupdate accept --config versions.json --reports reference/work/versions --output versions.json

Commands:
  accept  Set validation thresholds from passing compatibility reports

Run "mcversionupdate accept --help" for command options.
`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(arguments []string, stdout, stderr io.Writer) error {
	if len(arguments) == 0 {
		_, _ = io.WriteString(stdout, usageText)
		return nil
	}
	switch arguments[0] {
	case "help", "--help", "-h":
		_, _ = io.WriteString(stdout, usageText)
		return nil
	case "accept":
		return runAccept(arguments[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", arguments[0], usageText)
	}
}

func runAccept(arguments []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("accept", flag.ContinueOnError)
	set.SetOutput(stderr)
	set.Usage = func() {
		_, _ = fmt.Fprintln(stderr, `Usage:
  mcversionupdate accept --config <versions.json> --reports <directory> --output <versions.json>

Options:`)
		set.PrintDefaults()
		_, _ = fmt.Fprintln(stderr, `
Example:
  mcversionupdate accept --config versions.json --reports reference/work/versions --output versions.json`)
	}
	configPath := set.String("config", "", "source versions.json file (required)")
	reportsPath := set.String("reports", "", "directory containing compatibility.json reports (required)")
	outputPath := set.String("output", "", "destination versions.json file (required)")
	if err := set.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if set.NArg() != 0 {
		return fmt.Errorf("accept accepts no positional arguments: %s", strings.Join(set.Args(), " "))
	}
	for _, required := range []struct {
		name  string
		value string
	}{
		{name: "--config", value: *configPath},
		{name: "--reports", value: *reportsPath},
		{name: "--output", value: *outputPath},
	} {
		if strings.TrimSpace(required.value) == "" {
			return fmt.Errorf("%s is required", required.name)
		}
	}

	versions, err := config.ReadVersionFile(*configPath)
	if err != nil {
		return err
	}
	reports, err := readCompatibilityReports(*reportsPath)
	if err != nil {
		return err
	}
	accepted, err := catalog.Accept(versions, reports)
	if err != nil {
		return err
	}
	if err := config.WriteVersionFile(*outputPath, accepted); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "accepted %d version(s) into %s\n", len(accepted), *outputPath)
	return nil
}

func readCompatibilityReports(root string) ([]pipeline.CompatibilityReport, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect reports directory: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("reports directory must not be a symlink: %s", root)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("reports path is not a directory: %s", root)
	}

	reports := make([]pipeline.CompatibilityReport, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || entry.Name() != "compatibility.json" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect report %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open compatibility report %s: %w", path, err)
		}
		var report pipeline.CompatibilityReport
		decoder := json.NewDecoder(file)
		decodeErr := decoder.Decode(&report)
		if decodeErr == nil {
			var trailing any
			if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
				if err == nil {
					decodeErr = errors.New("contains more than one JSON value")
				} else {
					decodeErr = err
				}
			}
		}
		closeErr := file.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode compatibility report %s: %w", path, decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close compatibility report %s: %w", path, closeErr)
		}
		reports = append(reports, report)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk compatibility reports: %w", err)
	}
	return reports, nil
}
