package mapping

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ParseTinyV1 parses an official-to-named Tiny v1 mapping.
func ParseTinyV1(input io.Reader) (Mapping, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return Mapping{}, fmt.Errorf("scan Tiny v1 header: %w", err)
		}
		return Mapping{}, fmt.Errorf("tiny v1 mapping is empty")
	}
	if header := strings.Split(scanner.Text(), "\t"); len(header) != 3 || header[0] != "v1" || header[1] != "official" || header[2] != "named" {
		return Mapping{}, fmt.Errorf("unsupported Tiny v1 namespaces in header %q", scanner.Text())
	}

	var result Mapping
	lineNumber := 1
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if line == "" {
			continue
		}
		record := strings.Split(line, "\t")
		switch record[0] {
		case "CLASS":
			if len(record) != 3 || record[1] == "" || record[2] == "" {
				return Mapping{}, fmt.Errorf("tiny v1 line %d has an invalid CLASS record", lineNumber)
			}
			result.Classes = append(result.Classes, Class{Source: record[1], Target: record[2]})
		case "FIELD":
			if len(record) != 5 || hasEmptyValue(record[1:]) {
				return Mapping{}, fmt.Errorf("tiny v1 line %d has an invalid FIELD record", lineNumber)
			}
			result.Fields = append(result.Fields, Field{
				Owner:      record[1],
				Descriptor: record[2],
				Source:     record[3],
				Target:     record[4],
			})
		case "METHOD":
			if len(record) != 5 || hasEmptyValue(record[1:]) {
				return Mapping{}, fmt.Errorf("tiny v1 line %d has an invalid METHOD record", lineNumber)
			}
			result.Methods = append(result.Methods, Method{
				Owner:      record[1],
				Descriptor: record[2],
				Source:     record[3],
				Target:     record[4],
			})
		default:
			return Mapping{}, fmt.Errorf("tiny v1 line %d has unknown record type %q", lineNumber, record[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return Mapping{}, fmt.Errorf("scan Tiny v1 mapping: %w", err)
	}

	classes, err := classTargets(result.Classes)
	if err != nil {
		return Mapping{}, fmt.Errorf("validate Tiny v1 classes: %w", err)
	}
	for _, field := range result.Fields {
		if _, exists := classes[field.Owner]; !exists {
			return Mapping{}, fmt.Errorf("tiny v1 field %q has missing owner %q", field.Source, field.Owner)
		}
		if _, err := remapDescriptor(field.Descriptor, classes, false); err != nil {
			return Mapping{}, fmt.Errorf("tiny v1 field %s/%s: %w", field.Owner, field.Source, err)
		}
	}
	for _, method := range result.Methods {
		if _, exists := classes[method.Owner]; !exists {
			return Mapping{}, fmt.Errorf("tiny v1 method %q has missing owner %q", method.Source, method.Owner)
		}
		if _, err := remapDescriptor(method.Descriptor, classes, true); err != nil {
			return Mapping{}, fmt.Errorf("tiny v1 method %s/%s: %w", method.Owner, method.Source, err)
		}
	}
	methods := result.Methods[:0]
	for _, method := range result.Methods {
		if !isConstructor(method.Source) && !isConstructor(method.Target) {
			methods = append(methods, method)
		}
	}
	result.Methods = methods
	return result, nil
}

func hasEmptyValue(values []string) bool {
	for _, value := range values {
		if value == "" {
			return true
		}
	}
	return false
}
