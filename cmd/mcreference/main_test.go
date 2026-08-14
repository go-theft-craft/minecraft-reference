package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHelpIncludesExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"prepare", "--help"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "Examples:") || !strings.Contains(stderr.String(), "--versions 1.8.9,26.1.2") {
		t.Fatalf("help lacks examples:\n%s", stderr.String())
	}
}

func TestPrepareRequiresVersions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"prepare"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "--versions is required") {
		t.Fatalf("got %v", err)
	}
}

func TestCleanRequiresConfirmation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"clean"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "requires --yes") {
		t.Fatalf("got %v", err)
	}
}

func TestVersionPrintsBuildInformation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"version"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "mcreference ") {
		t.Fatalf("got %q", stdout.String())
	}
}
