package pipeline

import (
	"strings"
	"testing"
)

func TestParseJavaMajor(t *testing.T) {
	tests := map[string]struct {
		output string
		want   int
	}{
		"legacy java":  {output: `java version "1.8.0_472"`, want: 8},
		"modern java":  {output: `openjdk version "25.0.4" 2026-04-21`, want: 25},
		"legacy javap": {output: "1.8.0_472\n", want: 8},
		"modern javap": {output: "25.0.4\n", want: 25},
		"noisy java":   {output: "Picked up JAVA_TOOL_OPTIONS: -Xmx2g\nopenjdk version \"21.0.8\"", want: 21},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseJavaMajor(test.output)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %d, want %d", got, test.want)
			}
		})
	}
}

func TestParseJavaMajorRejectsUnknownOutput(t *testing.T) {
	_, err := parseJavaMajor("unknown")
	if err == nil || !strings.Contains(err.Error(), "cannot parse") {
		t.Fatalf("got %v", err)
	}
}

func TestValidateJavaMajorRejectsOldTool(t *testing.T) {
	err := validateJavaMajor("javap", "/usr/bin/javap", 17, 21, "1.21.8")
	if err == nil || !strings.Contains(err.Error(), "Minecraft 1.21.8 requires Java 21 or newer") {
		t.Fatalf("got %v", err)
	}
}
