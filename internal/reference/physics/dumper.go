package physics

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Options selects the reference workspace, version, side, and output path.
type Options struct {
	ReferenceDir string
	Version      string
	Side         string
	Output       string
	Java         string
	Javac        string
}

// dumper is the program to compile and run for one version.
//
// A version gets its own dumper because what the game exposes moves. 1.8.9
// keeps its block registry and its slipperiness field private and needs
// reflection for both; 26.1.2 makes them public and moves gravity and step
// height into attributes. No single program reads both.
type dumper struct {
	// ClassName is the compiled entry point, which is also the source file name
	// javac requires.
	ClassName string
	Source    string
}

var dumpers = map[string]dumper{
	"1.8.9":  {ClassName: "Dump1_8", Source: dump18Source},
	"26.1.2": {ClassName: "Dump26_1", Source: dump261Source},
}

// implementedVersions names what can be dumped, so the error for a version that
// cannot says what can instead of only what cannot.
func implementedVersions() string {
	names := make([]string, 0, len(dumpers))
	for version := range dumpers {
		names = append(names, version)
	}
	sort.Strings(names)

	return strings.Join(names, ", ")
}

// analysisJarPath returns the jar to compile and run against.
//
// Which file that is depends on how the version was named, so it is read from
// the workspace's own compatibility report rather than guessed at. A version
// named from mappings is remapped into named.jar; a version Mojang ships with
// its own names is left alone, and the jar to use is the one the bundler was
// unpacked into. Falling back from one to the other would be worse than
// failing: the obfuscated jar compiles nothing, and a run against the wrong
// file is a confusing compile error rather than a clear missing-workspace one.
func analysisJarPath(referenceDir, version, side string) (string, error) {
	reportPath := filepath.Join(referenceDir, "versions", version, side, "compatibility.json")
	raw, err := os.ReadFile(filepath.Clean(reportPath))
	if err != nil {
		return "", fmt.Errorf("read the prepared workspace's compatibility report: %w", err)
	}

	var report struct {
		Naming string `json:"naming"`
		Passed bool   `json:"passed"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return "", fmt.Errorf("parse %s: %w", reportPath, err)
	}
	if !report.Passed {
		return "", fmt.Errorf("%s %s did not pass validation; re-run mcreference prepare", version, side)
	}

	name := "named.jar"
	if report.Naming == "identity" {
		name = "executable.jar"
	}

	return filepath.Join(referenceDir, "versions", version, side, name), nil
}

// originalJarPath locates the jar exactly as Mojang published it. Its digest is
// the provenance consumers can verify; named.jar is a local remap whose bytes
// depend on the mapping tools and are not reproducible elsewhere.
func originalJarPath(referenceDir, version, side string) string {
	return filepath.Join(referenceDir, "versions", version, side, "original.jar")
}

func librariesPath(referenceDir, version string) string {
	return filepath.Join(referenceDir, "versions", version, "libraries")
}

// Dump compiles and runs the reflective dumper and writes a canonical physics document.
func Dump(ctx context.Context, options Options) error {
	program, supported := dumpers[options.Version]
	if !supported {
		return fmt.Errorf("version %q has no physics dumper; %s are implemented",
			options.Version, implementedVersions())
	}
	if options.Side != "server" {
		return fmt.Errorf("side %q is not supported; use server", options.Side)
	}
	if options.Output == "" {
		return fmt.Errorf("output path is required")
	}

	java := options.Java
	if java == "" {
		java = "java"
	}
	javac := options.Javac
	if javac == "" {
		javac = "javac"
	}

	jar, err := analysisJarPath(options.ReferenceDir, options.Version, options.Side)
	if err != nil {
		return err
	}
	if _, err := os.Stat(jar); err != nil {
		return fmt.Errorf("read the analysis jar: %w", err)
	}

	digest, err := fileDigest(originalJarPath(options.ReferenceDir, options.Version, options.Side))
	if err != nil {
		return fmt.Errorf("read original jar: %w", err)
	}

	staging, err := os.MkdirTemp("", "mcreference-dump-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	sourcePath := filepath.Join(staging, program.ClassName+".java")
	if err := os.WriteFile(sourcePath, []byte(program.Source), 0o600); err != nil {
		return fmt.Errorf("write dumper source: %w", err)
	}

	classpath := buildClasspath(jar, librariesPath(options.ReferenceDir, options.Version))
	if err := runTool(ctx, javac, staging, "-cp", classpath, "-d", staging, sourcePath); err != nil {
		return fmt.Errorf("compile dumper: %w", err)
	}

	var stdout, stderr bytes.Buffer
	command := exec.CommandContext(ctx, java, "-cp", classpath+string(os.PathListSeparator)+staging, program.ClassName)
	command.Dir = staging
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run dumper: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	document, err := ParseDocument(stdout.Bytes())
	if err != nil {
		return fmt.Errorf("decode dumper output: %w", err)
	}
	document.Version = options.Version
	document.Side = options.Side
	document.JarSHA256 = digest

	raw, err := document.MarshalCanonical()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(options.Output), 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(options.Output, raw, 0o600); err != nil {
		return fmt.Errorf("write physics document: %w", err)
	}

	return nil
}

// buildClasspath puts the named jar first, then every library jar below
// libraries/. The layout is Maven-style, so the jars sit at varying depths and
// a fixed-depth glob would miss all of them.
func buildClasspath(jar, libraries string) string {
	entries := []string{jar}

	_ = filepath.WalkDir(libraries, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// An unreadable or missing directory contributes no jars. The
			// compile and run steps report the resulting missing class, which
			// names the real problem better than a walk error would.
			return fs.SkipDir
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".jar") {
			return nil
		}
		entries = append(entries, path)

		return nil
	})

	return strings.Join(entries, string(os.PathListSeparator))
}

func runTool(ctx context.Context, name, workingDirectory string, arguments ...string) error {
	command := exec.CommandContext(ctx, name, arguments...)
	command.Dir = workingDirectory

	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
