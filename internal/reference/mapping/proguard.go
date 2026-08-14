package mapping

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type proGuardMember struct {
	owner string
	line  int
	text  string
}

// ParseProGuard parses a named-to-obfuscated ProGuard mapping and reverses its direction.
func ParseProGuard(input io.Reader) (Mapping, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)

	var result Mapping
	var members []proGuardMember
	currentOwner := ""
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			if currentOwner == "" {
				return Mapping{}, fmt.Errorf("ProGuard line %d has a member without an owner", lineNumber)
			}
			members = append(members, proGuardMember{owner: currentOwner, line: lineNumber, text: trimmed})
			continue
		}

		named, obfuscated, err := parseProGuardClass(trimmed)
		if err != nil {
			return Mapping{}, fmt.Errorf("ProGuard line %d: %w", lineNumber, err)
		}
		currentOwner = named
		result.Classes = append(result.Classes, Class{Source: obfuscated, Target: named})
	}
	if err := scanner.Err(); err != nil {
		return Mapping{}, fmt.Errorf("scan ProGuard mapping: %w", err)
	}

	obfuscatedToNamed, err := classTargets(result.Classes)
	if err != nil {
		return Mapping{}, fmt.Errorf("validate ProGuard classes: %w", err)
	}
	namedToObfuscated := make(map[string]string, len(obfuscatedToNamed))
	for obfuscated, named := range obfuscatedToNamed {
		namedToObfuscated[named] = obfuscated
	}
	for _, member := range members {
		owner, exists := namedToObfuscated[member.owner]
		if !exists {
			return Mapping{}, fmt.Errorf("ProGuard line %d has missing owner %q", member.line, member.owner)
		}
		field, method, err := parseProGuardMember(owner, member.text, namedToObfuscated)
		if err != nil {
			return Mapping{}, fmt.Errorf("ProGuard line %d: %w", member.line, err)
		}
		if field != nil {
			result.Fields = append(result.Fields, *field)
		}
		if method != nil && !isConstructor(method.Target) && !isConstructor(method.Source) {
			result.Methods = append(result.Methods, *method)
		}
	}
	return result, nil
}

func parseProGuardClass(line string) (string, string, error) {
	if !strings.HasSuffix(line, ":") {
		return "", "", fmt.Errorf("invalid class record %q", line)
	}
	named, obfuscated, err := splitProGuardArrow(strings.TrimSuffix(line, ":"))
	if err != nil {
		return "", "", fmt.Errorf("invalid class record %q", line)
	}
	named, err = javaClassName(named)
	if err != nil {
		return "", "", err
	}
	obfuscated, err = javaClassName(obfuscated)
	if err != nil {
		return "", "", err
	}
	return named, obfuscated, nil
}

func parseProGuardMember(owner, line string, classes map[string]string) (*Field, *Method, error) {
	namedDeclaration, obfuscatedName, err := splitProGuardArrow(line)
	if err != nil || obfuscatedName == "" {
		return nil, nil, fmt.Errorf("invalid member record %q", line)
	}
	namedDeclaration = stripLineNumberPrefix(namedDeclaration)
	separator := strings.IndexByte(namedDeclaration, ' ')
	if separator <= 0 || separator == len(namedDeclaration)-1 {
		return nil, nil, fmt.Errorf("invalid member declaration %q", namedDeclaration)
	}
	typeName := namedDeclaration[:separator]
	namedMember := strings.TrimSpace(namedDeclaration[separator+1:])
	if namedMember == "" || strings.ContainsAny(obfuscatedName, " \t") {
		return nil, nil, fmt.Errorf("invalid member declaration %q", namedDeclaration)
	}

	if !strings.Contains(namedMember, "(") {
		if strings.ContainsAny(namedMember, "):") {
			return nil, nil, fmt.Errorf("invalid field declaration %q", namedDeclaration)
		}
		namedDescriptor, err := javaTypeDescriptor(typeName, false)
		if err != nil {
			return nil, nil, err
		}
		sourceDescriptor, err := remapDescriptor(namedDescriptor, classes, false)
		if err != nil {
			return nil, nil, err
		}
		return &Field{Owner: owner, Descriptor: sourceDescriptor, Source: obfuscatedName, Target: namedMember}, nil, nil
	}

	open := strings.IndexByte(namedMember, '(')
	closeOffset := strings.LastIndexByte(namedMember, ')')
	if open <= 0 || closeOffset < open || !validLineNumberSuffix(namedMember[closeOffset+1:]) {
		return nil, nil, fmt.Errorf("invalid method declaration %q", namedDeclaration)
	}
	methodName := namedMember[:open]
	if strings.ContainsAny(methodName, " \t") {
		return nil, nil, fmt.Errorf("invalid method name %q", methodName)
	}
	parameters, err := javaParameterDescriptors(namedMember[open+1 : closeOffset])
	if err != nil {
		return nil, nil, err
	}
	returns, err := javaTypeDescriptor(typeName, true)
	if err != nil {
		return nil, nil, err
	}
	namedDescriptor := "(" + parameters + ")" + returns
	sourceDescriptor, err := remapDescriptor(namedDescriptor, classes, true)
	if err != nil {
		return nil, nil, err
	}
	return nil, &Method{Owner: owner, Descriptor: sourceDescriptor, Source: obfuscatedName, Target: methodName}, nil
}

func splitProGuardArrow(line string) (string, string, error) {
	parts := strings.Split(line, " -> ")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected one mapping arrow")
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if left == "" || right == "" {
		return "", "", fmt.Errorf("mapping arrow has an empty side")
	}
	return left, right, nil
}

func javaParameterDescriptors(parameters string) (string, error) {
	if strings.TrimSpace(parameters) == "" {
		return "", nil
	}
	parts := strings.Split(parameters, ",")
	var result strings.Builder
	for _, parameter := range parts {
		descriptor, err := javaTypeDescriptor(strings.TrimSpace(parameter), false)
		if err != nil {
			return "", err
		}
		result.WriteString(descriptor)
	}
	return result.String(), nil
}

func javaTypeDescriptor(name string, allowVoid bool) (string, error) {
	dimensions := 0
	for strings.HasSuffix(name, "[]") {
		dimensions++
		name = strings.TrimSuffix(name, "[]")
	}
	if name == "" || strings.ContainsAny(name, "[]() ,:") {
		return "", fmt.Errorf("invalid Java type %q", name)
	}

	primitive := map[string]string{
		"boolean": "Z",
		"byte":    "B",
		"char":    "C",
		"double":  "D",
		"float":   "F",
		"int":     "I",
		"long":    "J",
		"short":   "S",
		"void":    "V",
	}
	descriptor, isPrimitive := primitive[name]
	if isPrimitive {
		if descriptor == "V" && (!allowVoid || dimensions != 0) {
			return "", fmt.Errorf("invalid Java void type %q", name+strings.Repeat("[]", dimensions))
		}
	} else {
		internal, err := javaClassName(name)
		if err != nil {
			return "", err
		}
		descriptor = "L" + internal + ";"
	}
	return strings.Repeat("[", dimensions) + descriptor, nil
}

func javaClassName(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, " \t[]();:") || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid Java class name %q", name)
	}
	return strings.ReplaceAll(name, ".", "/"), nil
}

func stripLineNumberPrefix(declaration string) string {
	first := strings.IndexByte(declaration, ':')
	if first <= 0 || !isDecimal(declaration[:first]) {
		return declaration
	}
	secondRelative := strings.IndexByte(declaration[first+1:], ':')
	if secondRelative <= 0 {
		return declaration
	}
	second := first + 1 + secondRelative
	if !isDecimal(declaration[first+1 : second]) {
		return declaration
	}
	return declaration[second+1:]
}

func validLineNumberSuffix(suffix string) bool {
	if suffix == "" {
		return true
	}
	if suffix[0] != ':' {
		return false
	}
	parts := strings.Split(suffix[1:], ":")
	if len(parts) < 1 || len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if !isDecimal(part) {
			return false
		}
	}
	return true
}

func isDecimal(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
