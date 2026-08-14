// Package index builds searchable JVM and source indexes.
package index

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/archive"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/mapping"
)

const javapBatchSize = 40

// Symbol is one exact JVM member identity from javap output.
type Symbol struct {
	Version     string `json:"version"`
	Side        string `json:"side"`
	Owner       string `json:"owner"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Descriptor  string `json:"descriptor"`
	Declaration string `json:"declaration"`
}

// ValidateSymbol verifies one symbol record against its generating run.
func ValidateSymbol(symbol Symbol, version, side string) error {
	if symbol.Version != version {
		return fmt.Errorf("version %q does not match %q", symbol.Version, version)
	}
	if symbol.Side != side {
		return fmt.Errorf("side %q does not match %q", symbol.Side, side)
	}
	if !validOwner(symbol.Owner) {
		return fmt.Errorf("owner %q is not a valid class", symbol.Owner)
	}

	methodDescriptor := false
	switch symbol.Kind {
	case "field":
		if !validMemberName(symbol.Name) {
			return fmt.Errorf("field name %q is invalid", symbol.Name)
		}
	case "method":
		methodDescriptor = true
		if !validMemberName(symbol.Name) {
			return fmt.Errorf("method name %q is invalid", symbol.Name)
		}
	case "constructor":
		methodDescriptor = true
		if symbol.Name != "<init>" {
			return fmt.Errorf("constructor name %q is invalid", symbol.Name)
		}
	case "initializer":
		if symbol.Name != "<clinit>" {
			return fmt.Errorf("initializer name %q is invalid", symbol.Name)
		}
		if symbol.Descriptor != "()V" {
			return fmt.Errorf("initializer descriptor %q is invalid", symbol.Descriptor)
		}
		return nil
	default:
		return fmt.Errorf("kind %q is invalid", symbol.Kind)
	}
	if err := mapping.ValidateDescriptor(symbol.Descriptor, methodDescriptor); err != nil {
		return fmt.Errorf("descriptor %q is invalid: %w", symbol.Descriptor, err)
	}
	if symbol.Kind == "constructor" && !strings.HasSuffix(symbol.Descriptor, ")V") {
		return fmt.Errorf("constructor descriptor %q must return void", symbol.Descriptor)
	}
	return nil
}

func validOwner(owner string) bool {
	if owner == "" || strings.ContainsAny(owner, "/;[():") {
		return false
	}
	for segment := range strings.SplitSeq(owner, ".") {
		if segment == "" || strings.IndexFunc(segment, unicode.IsSpace) >= 0 {
			return false
		}
	}
	return true
}

func validMemberName(name string) bool {
	return name != "" && !strings.ContainsAny(name, ".;/[()<>") && strings.IndexFunc(name, unicode.IsSpace) < 0
}

// GenerateJavap writes deterministic JSON Lines records for every jar member.
func GenerateJavap(ctx context.Context, javap, jar, output, version, side string) error {
	classes, err := archive.ListClasses(jar)
	if err != nil {
		return err
	}
	classes = minecraftClasses(classes)
	sort.Strings(classes)
	if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		return fmt.Errorf("create index directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(output), ".symbols-*")
	if err != nil {
		return fmt.Errorf("create symbol index: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	encoder := json.NewEncoder(temporary)
	for start := 0; start < len(classes); start += javapBatchSize {
		end := min(start+javapBatchSize, len(classes))
		arguments := append([]string{"-p", "-s", "-classpath", jar}, classes[start:end]...)
		command := exec.CommandContext(ctx, javap, arguments...)
		data, err := command.Output()
		if err != nil {
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				return fmt.Errorf("javap failed: %s: %w", strings.TrimSpace(string(exitError.Stderr)), err)
			}
			return fmt.Errorf("run javap: %w", err)
		}
		symbols, err := ParseJavap(string(data), version, side)
		if err != nil {
			return fmt.Errorf("parse javap batch %s through %s: %w", classes[start], classes[end-1], err)
		}
		for _, symbol := range symbols {
			if err := encoder.Encode(symbol); err != nil {
				return fmt.Errorf("write symbol index: %w", err)
			}
		}
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close symbol index: %w", err)
	}
	if err := os.Rename(temporaryPath, output); err != nil {
		return fmt.Errorf("publish symbol index: %w", err)
	}
	return nil
}

func minecraftClasses(classes []string) []string {
	result := make([]string, 0, len(classes))
	for _, class := range classes {
		if strings.HasPrefix(class, "net.minecraft.") {
			result = append(result, class)
		}
	}
	return result
}

// ParseJavap converts javap declarations and descriptors into symbol records.
func ParseJavap(data, version, side string) ([]Symbol, error) {
	scanner := bufio.NewScanner(strings.NewReader(data))
	var owner, declaration string
	result := make([]Symbol, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasSuffix(line, "{") && isTypeDeclaration(line) {
			owner = declaredOwner(line)
			continue
		}
		if strings.HasPrefix(line, "descriptor:") {
			if owner == "" || declaration == "" {
				return nil, fmt.Errorf("descriptor without owner or declaration: %q", line)
			}
			descriptor := strings.TrimSpace(strings.TrimPrefix(line, "descriptor:"))
			name, kind := declaredMember(owner, declaration)
			result = append(result, Symbol{Version: version, Side: side, Owner: owner, Kind: kind, Name: name, Descriptor: descriptor, Declaration: declaration})
			declaration = ""
			continue
		}
		if strings.HasSuffix(line, ";") && owner != "" && !strings.HasPrefix(line, "Compiled from") {
			declaration = line
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func isTypeDeclaration(line string) bool {
	for _, keyword := range []string{"class", "interface", "enum"} {
		if strings.HasPrefix(line, keyword+" ") || strings.Contains(line, " "+keyword+" ") {
			return true
		}
	}
	return false
}

func declaredOwner(line string) string {
	line = strings.TrimSuffix(line, " {")
	fields := strings.Fields(line)
	for i, field := range fields {
		if field == "class" || field == "interface" || field == "enum" {
			if i+1 < len(fields) {
				return strings.TrimSuffix(fields[i+1], "<")
			}
		}
	}
	return ""
}

func declaredMember(owner, declaration string) (string, string) {
	declaration = strings.TrimSuffix(declaration, ";")
	if declaration == "static {}" {
		return "<clinit>", "initializer"
	}
	if open := strings.IndexByte(declaration, '('); open >= 0 {
		prefix := declaration[:open]
		name := prefix[strings.LastIndexByte(prefix, ' ')+1:]
		if name == owner || strings.HasSuffix(owner, "."+name) {
			return "<init>", "constructor"
		}
		return name, "method"
	}
	fields := strings.Fields(declaration)
	if len(fields) == 0 {
		return "", "field"
	}
	return fields[len(fields)-1], "field"
}
