// Command mcversionupdate updates the reviewed Minecraft version catalog.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/artifact"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/catalog"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/pipeline"
)

const usageText = `mcversionupdate promotes compatibility evidence into the version catalog.

Usage:
  mcversionupdate discover --output candidate.json
  mcversionupdate matrix --config candidate.json
  mcversionupdate accept --config versions.json --reports reference/work/versions --output versions.json
  mcversionupdate readme --config versions.json --file README.md --write

Commands:
  discover  Find new stable family representatives
  matrix    Print a GitHub Actions compatibility matrix
  accept    Set validation thresholds from passing compatibility reports
  readme    Write or check the generated supported-versions table

Run "mcversionupdate <command> --help" for command options.
`

type runDependencies struct {
	HTTPClient *http.Client
	Root       string
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: get current directory: %v\n", err)
		os.Exit(1)
	}
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, runDependencies{Root: root}); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, stdout, stderr io.Writer, dependencies runDependencies) error {
	if len(arguments) == 0 {
		_, _ = io.WriteString(stdout, usageText)
		return nil
	}
	switch arguments[0] {
	case "help", "--help", "-h":
		_, _ = io.WriteString(stdout, usageText)
		return nil
	case "discover":
		return runDiscover(ctx, arguments[1:], stdout, stderr, dependencies)
	case "matrix":
		return runMatrix(arguments[1:], stdout, stderr, dependencies)
	case "accept":
		return runAccept(arguments[1:], stdout, stderr, dependencies)
	case "readme":
		return runREADME(arguments[1:], stdout, stderr, dependencies)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", arguments[0], usageText)
	}
}

func runDiscover(ctx context.Context, arguments []string, stdout, stderr io.Writer, dependencies runDependencies) error {
	set := flag.NewFlagSet("discover", flag.ContinueOnError)
	set.SetOutput(stderr)
	set.Usage = func() {
		_, _ = fmt.Fprintln(stderr, `Usage:
  mcversionupdate discover --output <candidate.json>

Options:`)
		set.PrintDefaults()
	}
	outputPath := set.String("output", "", "destination candidate.json file (required)")
	if err := parseFlags(set, arguments, "discover"); err != nil {
		if errors.Is(err, errHelpRequested) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*outputPath) == "" {
		return errors.New("--output is required")
	}

	configured, err := config.LoadVersions("")
	if err != nil {
		return err
	}
	current := make([]config.Version, 0, len(configured))
	for _, version := range configured {
		current = append(current, version)
	}
	current = catalog.ApplyCandidates(current, nil)

	resolver := artifact.Resolver{Client: dependencies.HTTPClient}
	releases, err := resolver.ListReleases(ctx)
	if err != nil {
		return err
	}
	root := dependencies.root()
	temporaryDirectory, err := os.MkdirTemp(root, ".mcversionupdate-*")
	if err != nil {
		return fmt.Errorf("create metadata workspace: %w", err)
	}
	defer func() { _ = os.RemoveAll(temporaryDirectory) }()
	downloadIndex := 0
	candidates, err := catalog.Discover(ctx, releases, current, func(ctx context.Context, release artifact.Release) (artifact.VersionMetadata, error) {
		destination := filepath.Join(temporaryDirectory, fmt.Sprintf("metadata-%d.json", downloadIndex))
		downloadIndex++
		_, err := (artifact.Downloader{Client: dependencies.HTTPClient}).Download(ctx, artifact.DownloadSpec{
			URL:  release.URL,
			SHA1: release.SHA1,
		}, destination)
		if err != nil {
			return artifact.VersionMetadata{}, err
		}
		data, err := os.ReadFile(destination)
		if err != nil {
			return artifact.VersionMetadata{}, fmt.Errorf("read downloaded version metadata: %w", err)
		}
		return resolver.DecodeVersion(data, release.ID)
	})
	if err != nil {
		return err
	}
	versions := catalog.ApplyCandidates(current, candidates)
	output := dependencies.path(*outputPath)
	if err := catalog.WriteCandidateFile(output, versions, candidates); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "discovered %d candidate(s) into %s\n", len(candidates), *outputPath)
	return nil
}

func runMatrix(arguments []string, stdout, stderr io.Writer, dependencies runDependencies) error {
	set := flag.NewFlagSet("matrix", flag.ContinueOnError)
	set.SetOutput(stderr)
	set.Usage = func() {
		_, _ = fmt.Fprintln(stderr, `Usage:
  mcversionupdate matrix --config <versions-or-candidate.json>

Options:`)
		set.PrintDefaults()
	}
	configPath := set.String("config", "", "versions or candidate configuration file (required)")
	if err := parseFlags(set, arguments, "matrix"); err != nil {
		if errors.Is(err, errHelpRequested) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*configPath) == "" {
		return errors.New("--config is required")
	}
	file, err := catalog.ReadCandidateFile(dependencies.path(*configPath))
	if err != nil {
		return err
	}
	versions := file.Versions
	if file.HasCandidatesField() {
		versions = make([]config.Version, 0, len(file.Candidates))
		for _, candidate := range file.Candidates {
			versions = append(versions, candidate.New)
		}
	}
	versions = catalog.ApplyCandidates(versions, nil)

	type matrixEntry struct {
		Version string `json:"version"`
		Family  string `json:"family"`
		Side    string `json:"side"`
		Java    int    `json:"java"`
	}
	matrix := struct {
		Include []matrixEntry `json:"include"`
	}{Include: make([]matrixEntry, 0)}
	for _, version := range versions {
		for _, side := range []string{"client", "server"} {
			if version.SupportsSide(side) {
				matrix.Include = append(matrix.Include, matrixEntry{
					Version: version.ID,
					Family:  version.Family,
					Side:    side,
					Java:    version.Java,
				})
			}
		}
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(matrix); err != nil {
		return fmt.Errorf("encode compatibility matrix: %w", err)
	}
	return nil
}

func runAccept(arguments []string, stdout, stderr io.Writer, dependencies runDependencies) error {
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
	if err := parseFlags(set, arguments, "accept"); err != nil {
		if errors.Is(err, errHelpRequested) {
			return nil
		}
		return err
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

	file, err := catalog.ReadCandidateFile(dependencies.path(*configPath))
	if err != nil {
		return err
	}
	versions := file.Versions
	if file.HasCandidatesField() {
		versions = make([]config.Version, 0, len(file.Candidates))
		for _, candidate := range file.Candidates {
			versions = append(versions, candidate.New)
		}
	}
	reports, err := readCompatibilityReports(dependencies.path(*reportsPath))
	if err != nil {
		return err
	}
	accepted, err := catalog.Accept(versions, reports)
	if err != nil {
		return err
	}
	outputVersions := accepted
	if file.HasCandidatesField() {
		acceptedCandidates := make([]catalog.Candidate, 0, len(accepted))
		for _, version := range accepted {
			acceptedCandidates = append(acceptedCandidates, catalog.Candidate{Family: version.Family, New: version})
		}
		outputVersions = catalog.ApplyCandidates(file.Versions, acceptedCandidates)
	}
	if err := config.WriteVersionFile(dependencies.path(*outputPath), outputVersions); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "accepted %d version(s) into %s\n", len(accepted), *outputPath)
	return nil
}

func runREADME(arguments []string, stdout, stderr io.Writer, dependencies runDependencies) error {
	set := flag.NewFlagSet("readme", flag.ContinueOnError)
	set.SetOutput(stderr)
	set.Usage = func() {
		_, _ = fmt.Fprintln(stderr, `Usage:
  mcversionupdate readme --config <versions.json> --file <README.md> (--write | --check)

Options:`)
		set.PrintDefaults()
	}
	configPath := set.String("config", "", "versions configuration file (required)")
	readmePath := set.String("file", "", "README file (required)")
	write := set.Bool("write", false, "write the generated table")
	check := set.Bool("check", false, "fail if the generated table is out of date")
	if err := parseFlags(set, arguments, "readme"); err != nil {
		if errors.Is(err, errHelpRequested) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*configPath) == "" {
		return errors.New("--config is required")
	}
	if strings.TrimSpace(*readmePath) == "" {
		return errors.New("--file is required")
	}
	if *write == *check {
		return errors.New("exactly one of --write or --check is required")
	}
	versions, err := config.ReadVersionFile(dependencies.path(*configPath))
	if err != nil {
		return err
	}
	path := dependencies.path(*readmePath)
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", *readmePath, err)
	}
	if *check {
		if err := catalog.CheckREADME(source, versions); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "%s supported-versions table is current\n", *readmePath)
		return nil
	}
	updated, err := catalog.UpdateREADME(source, versions)
	if err != nil {
		return err
	}
	if err := writeREADME(path, updated); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "updated %s supported-versions table\n", *readmePath)
	return nil
}

func parseFlags(set *flag.FlagSet, arguments []string, command string) error {
	if err := set.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return errHelpRequested
		}
		return err
	}
	if set.NArg() != 0 {
		return fmt.Errorf("%s accepts no positional arguments: %s", command, strings.Join(set.Args(), " "))
	}
	return nil
}

var errHelpRequested = errors.New("help requested")

func (dependencies runDependencies) root() string {
	if dependencies.Root == "" {
		return "."
	}
	return dependencies.Root
}

func (dependencies runDependencies) path(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dependencies.root(), path)
}

func writeREADME(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect README: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".readme-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary README: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		return fmt.Errorf("set temporary README permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write temporary README: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary README: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary README: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace README: %w", err)
	}
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
