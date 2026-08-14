package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var javaVersionPattern = regexp.MustCompile(`(?m)^\s*(?:(?:openjdk|java)\s+version\s+"?)?([0-9]+)(?:\.([0-9]+))?`)

type javaToolchain struct {
	javaPath   string
	javaMajor  int
	javapPath  string
	javapMajor int
}

func preflightJava(ctx context.Context, javaName, javapName string, required int, minecraftVersion string) (javaToolchain, error) {
	javaPath, err := resolveExecutable(javaName, "java")
	if err != nil {
		return javaToolchain{}, err
	}
	javapPath, err := resolveExecutable(javapName, "javap")
	if err != nil {
		return javaToolchain{}, err
	}
	if err := requireSameJDKBin(javaPath, javapPath); err != nil {
		return javaToolchain{}, err
	}

	javaMajor, err := inspectJavaMajor(ctx, "java", javaPath)
	if err != nil {
		return javaToolchain{}, err
	}
	if err := validateJavaMajor("java", javaPath, javaMajor, required, minecraftVersion); err != nil {
		return javaToolchain{}, err
	}
	javapMajor, err := inspectJavaMajor(ctx, "javap", javapPath)
	if err != nil {
		return javaToolchain{}, err
	}
	if err := validateJavaMajor("javap", javapPath, javapMajor, required, minecraftVersion); err != nil {
		return javaToolchain{}, err
	}

	return javaToolchain{
		javaPath:   javaPath,
		javaMajor:  javaMajor,
		javapPath:  javapPath,
		javapMajor: javapMajor,
	}, nil
}

func requireSameJDKBin(javaPath, javapPath string) error {
	javaBin := filepath.Dir(javaPath)
	javapBin := filepath.Dir(javapPath)
	javaBinInfo, err := os.Stat(javaBin)
	if err != nil {
		return fmt.Errorf("inspect java bin directory %s: %w", javaBin, err)
	}
	javapBinInfo, err := os.Stat(javapBin)
	if err != nil {
		return fmt.Errorf("inspect javap bin directory %s: %w", javapBin, err)
	}
	if !os.SameFile(javaBinInfo, javapBinInfo) {
		return fmt.Errorf("java at %s and javap at %s must resolve to the same JDK bin directory", javaPath, javapPath)
	}
	return nil
}

func inspectJavaMajor(ctx context.Context, name, path string) (int, error) {
	command := exec.CommandContext(ctx, path, "-version")
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return 0, fmt.Errorf("check %s version at %s: %w", name, path, err)
		}
		return 0, fmt.Errorf("check %s version at %s: %s: %w", name, path, message, err)
	}
	major, err := parseJavaMajor(string(output))
	if err != nil {
		return 0, fmt.Errorf("check %s version at %s: %w", name, path, err)
	}
	return major, nil
}

func parseJavaMajor(output string) (int, error) {
	match := javaVersionPattern.FindStringSubmatch(output)
	if match == nil {
		return 0, fmt.Errorf("cannot parse Java version from %q", strings.TrimSpace(output))
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("parse Java major version: %w", err)
	}
	if major == 1 && match[2] != "" {
		major, err = strconv.Atoi(match[2])
		if err != nil {
			return 0, fmt.Errorf("parse legacy Java major version: %w", err)
		}
	}
	return major, nil
}

func validateJavaMajor(name, path string, actual, required int, minecraftVersion string) error {
	if actual < required {
		return fmt.Errorf("%s %d at %s is too old; reference preparation for Minecraft %s requires Java %d or newer", name, actual, path, minecraftVersion, required)
	}
	return nil
}
