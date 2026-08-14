// Package mapping applies reviewed readable names to legacy Minecraft jars.
package mapping

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ComposeMCP replaces SRG member names with stable MCP names.
func ComposeMCP(joinedSRG, fieldsCSV, methodsCSV, destination string) error {
	fields, err := loadStableNames(fieldsCSV)
	if err != nil {
		return fmt.Errorf("load fields: %w", err)
	}
	methods, err := loadStableNames(methodsCSV)
	if err != nil {
		return fmt.Errorf("load methods: %w", err)
	}
	input, err := os.Open(joinedSRG)
	if err != nil {
		return fmt.Errorf("open joined SRG: %w", err)
	}
	defer func() { _ = input.Close() }()
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return fmt.Errorf("create mapping directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".mapping-*")
	if err != nil {
		return fmt.Errorf("create mapping output: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	writer := bufio.NewWriter(temporary)
	for scanner.Scan() {
		line, err := replaceStableName(scanner.Text(), fields, methods)
		if err != nil {
			_ = temporary.Close()
			return err
		}
		if _, err := fmt.Fprintln(writer, line); err != nil {
			_ = temporary.Close()
			return fmt.Errorf("write composed SRG: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("scan joined SRG: %w", err)
	}
	if err := writer.Flush(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("flush composed SRG: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close composed SRG: %w", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("publish composed SRG: %w", err)
	}
	return nil
}

func loadStableNames(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	seargeColumn, nameColumn := -1, -1
	for i, name := range header {
		switch name {
		case "searge":
			seargeColumn = i
		case "name":
			nameColumn = i
		}
	}
	if seargeColumn < 0 || nameColumn < 0 {
		return nil, errors.New("CSV lacks searge or name column")
	}
	result := make(map[string]string)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) <= seargeColumn || len(record) <= nameColumn {
			return nil, errors.New("short CSV record")
		}
		if record[seargeColumn] == "" || record[nameColumn] == "" {
			continue
		}
		result[record[seargeColumn]] = record[nameColumn]
	}
	return result, nil
}

func replaceStableName(line string, fields, methods map[string]string) (string, error) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return line, nil
	}
	var index int
	var names map[string]string
	switch parts[0] {
	case "FD:":
		if len(parts) != 3 {
			return "", fmt.Errorf("invalid SRG field line %q", line)
		}
		index, names = 2, fields
	case "MD:":
		if len(parts) != 5 {
			return "", fmt.Errorf("invalid SRG method line %q", line)
		}
		index, names = 3, methods
	default:
		return line, nil
	}
	member := lastSegment(parts[index])
	owner := strings.TrimSuffix(parts[index], "/"+member)
	if owner == parts[index] || owner == "" {
		return "", fmt.Errorf("invalid SRG owner/member %q", parts[index])
	}
	if stable, ok := names[member]; ok {
		parts[index] = owner + "/" + stable
	}
	return strings.Join(parts, " "), nil
}

func lastSegment(value string) string {
	if index := strings.LastIndexByte(value, '/'); index >= 0 {
		return value[index+1:]
	}
	return value
}
