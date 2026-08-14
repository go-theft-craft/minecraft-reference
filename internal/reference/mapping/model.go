package mapping

import (
	"fmt"
	"strings"
)

// Class maps a JVM internal class name from the source namespace to the target namespace.
type Class struct {
	Source string
	Target string
}

// Field maps a field owned by a source-namespace class.
type Field struct {
	Owner, Descriptor, Source, Target string
}

// Method maps a method owned by a source-namespace class.
type Method struct {
	Owner, Descriptor, Source, Target string
}

// Mapping contains namespace-independent class and member mapping records.
type Mapping struct {
	Classes []Class
	Fields  []Field
	Methods []Method
}

func classTargets(classes []Class) (map[string]string, error) {
	targets := make(map[string]string, len(classes))
	sources := make(map[string]string, len(classes))
	for _, class := range classes {
		if class.Source == "" || class.Target == "" {
			return nil, fmt.Errorf("class mapping has an empty name")
		}
		if previous, exists := targets[class.Source]; exists {
			return nil, fmt.Errorf("duplicate source class %q maps to %q and %q", class.Source, previous, class.Target)
		}
		if previous, exists := sources[class.Target]; exists {
			return nil, fmt.Errorf("duplicate target class %q maps from %q and %q", class.Target, previous, class.Source)
		}
		targets[class.Source] = class.Target
		sources[class.Target] = class.Source
	}
	return targets, nil
}

func remapDescriptor(descriptor string, classes map[string]string, method bool) (string, error) {
	parser := descriptorParser{
		descriptor: descriptor,
		classes:    classes,
	}
	if method {
		return parser.method()
	}
	result, err := parser.fieldType(false)
	if err != nil {
		return "", err
	}
	if parser.offset != len(descriptor) {
		return "", fmt.Errorf("invalid field descriptor %q", descriptor)
	}
	return result, nil
}

type descriptorParser struct {
	descriptor string
	classes    map[string]string
	offset     int
}

func (p *descriptorParser) method() (string, error) {
	if !p.consume('(') {
		return "", fmt.Errorf("invalid method descriptor %q", p.descriptor)
	}
	var result strings.Builder
	result.WriteByte('(')
	for p.offset < len(p.descriptor) && p.descriptor[p.offset] != ')' {
		parameter, err := p.fieldType(false)
		if err != nil {
			return "", err
		}
		result.WriteString(parameter)
	}
	if !p.consume(')') {
		return "", fmt.Errorf("invalid method descriptor %q", p.descriptor)
	}
	result.WriteByte(')')
	returns, err := p.fieldType(true)
	if err != nil {
		return "", err
	}
	result.WriteString(returns)
	if p.offset != len(p.descriptor) {
		return "", fmt.Errorf("invalid method descriptor %q", p.descriptor)
	}
	return result.String(), nil
}

func (p *descriptorParser) fieldType(allowVoid bool) (string, error) {
	start := p.offset
	for p.consume('[') {
	}
	arrayDimensions := p.offset - start
	if p.offset >= len(p.descriptor) {
		return "", fmt.Errorf("invalid descriptor %q", p.descriptor)
	}

	var value string
	switch kind := p.descriptor[p.offset]; kind {
	case 'B', 'C', 'D', 'F', 'I', 'J', 'S', 'Z':
		p.offset++
		value = string(kind)
	case 'V':
		if !allowVoid || arrayDimensions != 0 {
			return "", fmt.Errorf("invalid void type in descriptor %q", p.descriptor)
		}
		p.offset++
		value = "V"
	case 'L':
		p.offset++
		nameStart := p.offset
		for p.offset < len(p.descriptor) && p.descriptor[p.offset] != ';' {
			p.offset++
		}
		if p.offset == nameStart || p.offset >= len(p.descriptor) {
			return "", fmt.Errorf("invalid object type in descriptor %q", p.descriptor)
		}
		name := p.descriptor[nameStart:p.offset]
		if strings.ContainsAny(name, ".[():") {
			return "", fmt.Errorf("invalid object type %q in descriptor %q", name, p.descriptor)
		}
		p.offset++
		if mapped, exists := p.classes[name]; exists {
			name = mapped
		}
		value = "L" + name + ";"
	default:
		return "", fmt.Errorf("invalid type in descriptor %q at byte %d", p.descriptor, p.offset)
	}
	return strings.Repeat("[", arrayDimensions) + value, nil
}

func (p *descriptorParser) consume(want byte) bool {
	if p.offset >= len(p.descriptor) || p.descriptor[p.offset] != want {
		return false
	}
	p.offset++
	return true
}

func isConstructor(name string) bool {
	return name == "<init>" || name == "<clinit>"
}
