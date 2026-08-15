// Command mcreference manages local vanilla Minecraft reference artifacts.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/physics"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/pipeline"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const usageText = `mcreference prepares a local vanilla Minecraft reference workspace.

Usage:
  mcreference prepare --versions <ids> [options]
  mcreference dump --versions <id> [--side server] --output <path>
  mcreference clean --reference-dir <path> --yes
  mcreference version

Commands:
  prepare  Download, name, decompile, and index client or server jars
  dump     Extract physics constants from a prepared reference workspace
  clean    Remove one validated reference workspace
  version  Print build information

Run "mcreference <command> --help" for command options and examples.
`

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, stdout, stderr io.Writer) error {
	if len(arguments) == 0 {
		_, _ = io.WriteString(stdout, usageText)
		return nil
	}
	switch arguments[0] {
	case "help", "--help", "-h":
		_, _ = io.WriteString(stdout, usageText)
		return nil
	case "prepare":
		return runPrepare(ctx, arguments[1:], stdout, stderr)
	case "dump":
		return runDump(ctx, arguments[1:], stdout, stderr)
	case "clean":
		return runClean(arguments[1:], stdout, stderr)
	case "version", "--version":
		_, _ = fmt.Fprintf(stdout, "mcreference %s (%s, %s)\n", version, commit, date)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", arguments[0], usageText)
	}
}

func runPrepare(ctx context.Context, arguments []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("prepare", flag.ContinueOnError)
	set.SetOutput(stderr)
	set.Usage = func() {
		_, _ = fmt.Fprintln(stderr, `Usage:
  mcreference prepare --versions <ids> [--sides <sides>] [--workspace <path>] [--reference-dir <path>]

Options:`)
		set.PrintDefaults()
		_, _ = fmt.Fprintln(stderr, `
Examples:
  mcreference prepare --versions 1.8.9
  mcreference prepare --versions 1.8.9,26.1.2 --sides client,server --workspace . --reference-dir reference/work`)
	}
	versions := set.String("versions", "", "comma-separated Minecraft versions (required)")
	sides := set.String("sides", "client,server", "comma-separated sides: client, server")
	workspace := set.String("workspace", "", "workspace root (default: current directory)")
	referenceDir := set.String("reference-dir", "reference/work", "output directory below the workspace")
	configDir := set.String("config-dir", "", "directory with versions.json and tools.json (default: embedded configuration)")
	java := set.String("java", "java", "Java executable")
	javap := set.String("javap", "javap", "javap executable")
	if err := set.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if set.NArg() != 0 {
		return fmt.Errorf("prepare accepts no positional arguments: %s", strings.Join(set.Args(), " "))
	}
	if strings.TrimSpace(*versions) == "" {
		return errors.New("--versions is required; example: mcreference prepare --versions 1.8.9,26.1.2")
	}
	root, err := workspaceRoot(*workspace)
	if err != nil {
		return err
	}
	return pipeline.Prepare(ctx, pipeline.Options{
		WorkspaceRoot: root,
		ConfigDir:     *configDir,
		ReferenceDir:  *referenceDir,
		Versions:      splitCSV(*versions),
		Sides:         splitCSV(*sides),
		Java:          *java,
		Javap:         *javap,
		Progress: func(message string) {
			_, _ = fmt.Fprintln(stdout, message)
		},
	})
}

func runClean(arguments []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("clean", flag.ContinueOnError)
	set.SetOutput(stderr)
	set.Usage = func() {
		_, _ = fmt.Fprintln(stderr, `Usage:
  mcreference clean [--workspace <path>] --reference-dir <path> --yes

Options:`)
		set.PrintDefaults()
		_, _ = fmt.Fprintln(stderr, `
Example:
  mcreference clean --workspace . --reference-dir reference/work --yes`)
	}
	workspace := set.String("workspace", "", "workspace root (default: current directory)")
	referenceDir := set.String("reference-dir", "reference/work", "output directory below the workspace")
	yes := set.Bool("yes", false, "confirm deletion of the validated directory")
	if err := set.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if set.NArg() != 0 {
		return fmt.Errorf("clean accepts no positional arguments: %s", strings.Join(set.Args(), " "))
	}
	if !*yes {
		return errors.New("clean requires --yes; no files were removed")
	}
	root, err := workspaceRoot(*workspace)
	if err != nil {
		return err
	}
	removed, err := pipeline.Clean(root, *referenceDir)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "removed %s\n", removed)
	return nil
}

func runDump(ctx context.Context, arguments []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("dump", flag.ContinueOnError)
	set.SetOutput(stderr)
	set.Usage = func() {
		_, _ = fmt.Fprintln(stderr, `Usage:
  mcreference dump --versions <id> [--side server] [--reference-dir <path>] --output <path>

Options:`)
		set.PrintDefaults()
		_, _ = fmt.Fprintln(stderr, `
Examples:
  mcreference dump --versions 1.8.9 --output physics.json`)
	}
	versions := set.String("versions", "", "one prepared Minecraft version (required)")
	side := set.String("side", "server", "prepared side to read")
	workspace := set.String("workspace", "", "workspace root (default: current directory)")
	referenceDir := set.String("reference-dir", "reference/work", "prepared directory below the workspace")
	output := set.String("output", "", "physics document output path (required)")
	java := set.String("java", "java", "Java executable")
	javac := set.String("javac", "javac", "Java compiler executable")
	if err := set.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return err
	}
	if set.NArg() != 0 {
		return fmt.Errorf("dump accepts no positional arguments: %s", strings.Join(set.Args(), " "))
	}
	if strings.TrimSpace(*versions) == "" {
		return errors.New("--versions is required; example: mcreference dump --versions 1.8.9 --output physics.json")
	}
	if strings.TrimSpace(*output) == "" {
		return errors.New("--output is required; example: mcreference dump --versions 1.8.9 --output physics.json")
	}

	selected := splitCSV(*versions)
	if len(selected) != 1 {
		return fmt.Errorf("dump accepts exactly one version, got %d", len(selected))
	}

	root, err := workspaceRoot(*workspace)
	if err != nil {
		return err
	}
	if err := physics.Dump(ctx, physics.Options{
		ReferenceDir: filepath.Join(root, *referenceDir),
		Version:      selected[0],
		Side:         *side,
		Output:       *output,
		Java:         *java,
		Javac:        *javac,
	}); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "wrote %s\n", *output)

	return nil
}

func workspaceRoot(requested string) (string, error) {
	if requested == "" {
		current, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get current directory: %w", err)
		}
		requested = current
	}
	root, err := filepath.Abs(requested)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("inspect workspace root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace root is not a directory: %s", root)
	}
	return root, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
