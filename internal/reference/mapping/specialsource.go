package mapping

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// WriteSRG writes a mapping in deterministic SpecialSource SRG order.
func WriteSRG(output io.Writer, value Mapping) error {
	classes := append([]Class(nil), value.Classes...)
	fields := append([]Field(nil), value.Fields...)
	methods := append([]Method(nil), value.Methods...)
	sort.Slice(classes, func(i, j int) bool {
		if classes[i].Source != classes[j].Source {
			return classes[i].Source < classes[j].Source
		}
		return classes[i].Target < classes[j].Target
	})
	sort.Slice(fields, func(i, j int) bool {
		return lessMember(fields[i].Owner, fields[i].Source, fields[i].Descriptor, fields[j].Owner, fields[j].Source, fields[j].Descriptor)
	})
	sort.Slice(methods, func(i, j int) bool {
		return lessMember(methods[i].Owner, methods[i].Source, methods[i].Descriptor, methods[j].Owner, methods[j].Source, methods[j].Descriptor)
	})

	classMap, err := classTargets(classes)
	if err != nil {
		return fmt.Errorf("validate SRG classes: %w", err)
	}
	writer := bufio.NewWriter(output)
	for _, class := range classes {
		if _, err := fmt.Fprintf(writer, "CL: %s %s\n", class.Source, class.Target); err != nil {
			return fmt.Errorf("write SRG class: %w", err)
		}
	}
	for _, field := range fields {
		targetOwner, exists := classMap[field.Owner]
		if !exists {
			return fmt.Errorf("SRG field %q has missing owner %q", field.Source, field.Owner)
		}
		if _, err := remapDescriptor(field.Descriptor, classMap, false); err != nil {
			return fmt.Errorf("SRG field %s/%s: %w", field.Owner, field.Source, err)
		}
		if _, err := fmt.Fprintf(writer, "FD: %s/%s %s/%s\n", field.Owner, field.Source, targetOwner, field.Target); err != nil {
			return fmt.Errorf("write SRG field: %w", err)
		}
	}
	for _, method := range methods {
		if isConstructor(method.Source) || isConstructor(method.Target) {
			continue
		}
		targetOwner, exists := classMap[method.Owner]
		if !exists {
			return fmt.Errorf("SRG method %q has missing owner %q", method.Source, method.Owner)
		}
		targetDescriptor, err := remapDescriptor(method.Descriptor, classMap, true)
		if err != nil {
			return fmt.Errorf("SRG method %s/%s: %w", method.Owner, method.Source, err)
		}
		if _, err := fmt.Fprintf(
			writer,
			"MD: %s/%s %s %s/%s %s\n",
			method.Owner,
			method.Source,
			method.Descriptor,
			targetOwner,
			method.Target,
			targetDescriptor,
		); err != nil {
			return fmt.Errorf("write SRG method: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush SRG output: %w", err)
	}
	return nil
}

func lessMember(leftOwner, leftName, leftDescriptor, rightOwner, rightName, rightDescriptor string) bool {
	if leftOwner != rightOwner {
		return leftOwner < rightOwner
	}
	if leftName != rightName {
		return leftName < rightName
	}
	return leftDescriptor < rightDescriptor
}

// Remap runs pinned SpecialSource when its input fingerprint changed.
func Remap(ctx context.Context, java, tool, input, output, mapping string) error {
	fingerprint, err := remapFingerprint(tool, input, mapping)
	if err != nil {
		return err
	}
	marker := output + ".lock"
	if data, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(data)) == fingerprint {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		return fmt.Errorf("create remap directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(output), ".remap-*.jar")
	if err != nil {
		return fmt.Errorf("create remap output: %w", err)
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close remap output: %w", err)
	}
	defer func() { _ = os.Remove(temporaryPath) }()
	command := exec.CommandContext(
		ctx, java, "-jar", tool,
		"--in-jar", input,
		"--out-jar", temporaryPath,
		"--srg-in", mapping,
	)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("SpecialSource remap: %w", err)
	}
	if err := os.Rename(temporaryPath, output); err != nil {
		return fmt.Errorf("publish remapped jar: %w", err)
	}
	if err := os.WriteFile(marker, []byte(fingerprint+"\n"), 0o600); err != nil {
		return fmt.Errorf("write remap marker: %w", err)
	}
	return nil
}

func remapFingerprint(paths ...string) (string, error) {
	hasher := sha256.New()
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("fingerprint %s: %w", path, err)
		}
		if _, err := io.Copy(hasher, file); err != nil {
			_ = file.Close()
			return "", fmt.Errorf("fingerprint %s: %w", path, err)
		}
		if err := file.Close(); err != nil {
			return "", fmt.Errorf("fingerprint %s: %w", path, err)
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
