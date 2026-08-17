package blocks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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

func namedJarPath(referenceDir, version, side string) string {
	return filepath.Join(referenceDir, "versions", version, side, "named.jar")
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

// Dump compiles and runs the reflective dumper and writes a canonical blocks
// document.
func Dump(ctx context.Context, options Options) error {
	if options.Version != "1.8.9" {
		return fmt.Errorf("version %q has no block dumper; only 1.8.9 is implemented", options.Version)
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

	jar := namedJarPath(options.ReferenceDir, options.Version, options.Side)
	if _, err := os.Stat(jar); err != nil {
		return fmt.Errorf("read named jar: %w", err)
	}

	digest, err := fileDigest(originalJarPath(options.ReferenceDir, options.Version, options.Side))
	if err != nil {
		return fmt.Errorf("read original jar: %w", err)
	}

	staging, err := os.MkdirTemp("", "mcreference-blocks-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	sourcePath := filepath.Join(staging, "DumpBlocks1_8.java")
	if err := os.WriteFile(sourcePath, []byte(dumpBlocks18Source), 0o600); err != nil {
		return fmt.Errorf("write dumper source: %w", err)
	}

	classpath := buildClasspath(jar, librariesPath(options.ReferenceDir, options.Version))
	if err := runTool(ctx, javac, staging, "-cp", classpath, "-d", staging, sourcePath); err != nil {
		return fmt.Errorf("compile dumper: %w", err)
	}

	var stdout, stderr bytes.Buffer
	command := exec.CommandContext(ctx, java,
		"-cp", classpath+string(os.PathListSeparator)+staging, "DumpBlocks1_8")
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
	if len(document.Blocks) == 0 {
		// A dumper that runs, exits zero, and describes nothing is the failure
		// worth naming: the output parses, the file writes, and every consumer
		// then reads a world with no solid blocks in it.
		return fmt.Errorf("dumper produced no blocks")
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
		return fmt.Errorf("write blocks document: %w", err)
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
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(digest.Sum(nil)), nil
}
