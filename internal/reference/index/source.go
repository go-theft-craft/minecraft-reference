package index

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	packagePattern = regexp.MustCompile(`^\s*package\s+([A-Za-z0-9_.]+)\s*;`)
	typePattern    = regexp.MustCompile(`\b(class|interface|enum|record)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
)

// SourceFile records searchable metadata for one decompiled Java source file.
type SourceFile struct {
	Path    string   `json:"path"`
	Package string   `json:"package"`
	Types   []string `json:"types"`
	Lines   int      `json:"lines"`
}

// GenerateSource writes a deterministic JSON Lines source-tree index.
func GenerateSource(root, output string) error {
	entries := make([]SourceFile, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".java") {
			return nil
		}
		indexed, err := indexSourceFile(root, path)
		if err != nil {
			return err
		}
		entries = append(entries, indexed)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk decompiled sources: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		return fmt.Errorf("create source index directory: %w", err)
	}
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create source index: %w", err)
	}
	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			_ = file.Close()
			return fmt.Errorf("write source index: %w", err)
		}
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close source index: %w", err)
	}
	return nil
}

func indexSourceFile(root, path string) (SourceFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return SourceFile{}, err
	}
	defer func() { _ = file.Close() }()
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return SourceFile{}, err
	}
	result := SourceFile{Path: filepath.ToSlash(relative), Types: make([]string, 0)}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	seenTypes := make(map[string]struct{})
	for scanner.Scan() {
		result.Lines++
		line := scanner.Text()
		if result.Package == "" {
			match := packagePattern.FindStringSubmatch(line)
			if len(match) == 2 {
				result.Package = match[1]
			}
		}
		matches := typePattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			name := match[2]
			if _, exists := seenTypes[name]; exists {
				continue
			}
			seenTypes[name] = struct{}{}
			result.Types = append(result.Types, name)
		}
	}
	if err := scanner.Err(); err != nil {
		return SourceFile{}, err
	}
	sort.Strings(result.Types)
	return result, nil
}
