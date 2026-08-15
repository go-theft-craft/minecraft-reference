package physics

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

func librariesPath(referenceDir, version string) string {
	return filepath.Join(referenceDir, "versions", version, "libraries")
}

// Dump compiles and runs the reflective dumper and writes a canonical physics document.
func Dump(ctx context.Context, options Options) error {
	if options.Version != "1.8.9" {
		return fmt.Errorf("version %q has no dumper; only 1.8.9 is implemented", options.Version)
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
	digest, err := fileDigest(jar)
	if err != nil {
		return fmt.Errorf("read named jar: %w", err)
	}

	staging, err := os.MkdirTemp("", "mcreference-dump-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	sourcePath := filepath.Join(staging, "Dump1_8.java")
	if err := os.WriteFile(sourcePath, []byte(dump18Source), 0o600); err != nil {
		return fmt.Errorf("write dumper source: %w", err)
	}

	classpath := buildClasspath(jar, librariesPath(options.ReferenceDir, options.Version))
	if err := runTool(ctx, javac, staging, "-cp", classpath, "-d", staging, sourcePath); err != nil {
		return fmt.Errorf("compile dumper: %w", err)
	}

	var stdout, stderr bytes.Buffer
	command := exec.CommandContext(ctx, java, "-cp", classpath+string(os.PathListSeparator)+staging, "Dump1_8")
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
