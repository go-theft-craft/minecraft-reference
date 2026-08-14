// Package pipeline coordinates the complete local reference workflow.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/artifact"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/decompile"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/index"
)

// Options configures one non-interactive reference preparation run.
type Options struct {
	WorkspaceRoot string
	ConfigDir     string
	ReferenceDir  string
	Versions      []string
	Sides         []string
	Java          string
	Javap         string
	HTTPClient    *http.Client
	Progress      func(string)
}

// Lock records verified inputs and selected strategies for one version.
type Lock struct {
	GeneratedAt string                    `json:"generated_at"`
	Version     string                    `json:"version"`
	Naming      string                    `json:"naming"`
	Java        string                    `json:"java"`
	Javap       string                    `json:"javap"`
	Artifacts   []artifact.DownloadResult `json:"artifacts"`
}

// Prepare downloads, names, decompiles, and indexes requested artifacts.
func Prepare(ctx context.Context, options Options) (resultErr error) {
	var activeVersionDir string
	var activeSides []string
	defer func() {
		if resultErr == nil || activeVersionDir == "" {
			return
		}
		if err := invalidateVersionOutputs(activeVersionDir, activeSides); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("invalidate failed version outputs: %w", err))
		}
	}()
	if len(options.Versions) == 0 {
		return errors.New("at least one version is required")
	}
	if len(options.Sides) == 0 {
		return errors.New("at least one side is required")
	}
	for _, side := range options.Sides {
		if side != "client" && side != "server" {
			return fmt.Errorf("unsupported side %q; use client or server", side)
		}
	}
	referenceDir, err := artifact.ResolveReferenceDir(options.WorkspaceRoot, options.ReferenceDir)
	if err != nil {
		return err
	}
	versions, err := config.LoadVersions(options.ConfigDir)
	if err != nil {
		return err
	}
	versionIDs := unique(options.Versions)
	selectedVersions := make([]config.Version, 0, len(versionIDs))
	requestedSides := unique(options.Sides)
	requiredJava := 0
	requiredBy := ""
	for _, versionID := range versionIDs {
		version, err := config.RequireVersion(versions, versionID)
		if err != nil {
			return err
		}
		selectedVersions = append(selectedVersions, version)
		if version.Java > requiredJava {
			requiredJava = version.Java
			requiredBy = version.ID
		}
	}
	for _, version := range selectedVersions {
		versionDir := filepath.Join(referenceDir, "versions", version.ID)
		if err := invalidateVersionOutputs(versionDir, requestedSides); err != nil {
			return fmt.Errorf("invalidate version %s outputs: %w", version.ID, err)
		}
	}
	tools, err := config.LoadTools(options.ConfigDir)
	if err != nil {
		return err
	}
	progress(options, "preflight java and javap")
	toolchain, err := preflightJava(ctx, options.Java, options.Javap, requiredJava, requiredBy)
	if err != nil {
		return err
	}
	java := toolchain.javaPath
	javap := toolchain.javapPath
	progress(options, fmt.Sprintf("preflight java=%d javap=%d required=%d", toolchain.javaMajor, toolchain.javapMajor, requiredJava))
	downloader := artifact.Downloader{Client: options.HTTPClient}
	resolver := artifact.Resolver{Client: options.HTTPClient}

	for _, version := range selectedVersions {
		versionID := version.ID
		activeVersionDir = filepath.Join(referenceDir, "versions", versionID)
		activeSides = requestedSides
		progress(options, "resolve "+versionID)
		_, metadataSpec, err := resolver.Resolve(ctx, versionID)
		if err != nil {
			return err
		}
		versionDir := filepath.Join(referenceDir, "versions", versionID)
		metadataPath := filepath.Join(referenceDir, "cache", "metadata", versionID+".json")
		metadataResult, err := downloader.Download(ctx, metadataSpec, metadataPath)
		if err != nil {
			return err
		}
		metadataBytes, err := os.ReadFile(metadataPath)
		if err != nil {
			return fmt.Errorf("read downloaded metadata: %w", err)
		}
		metadata, err := resolver.DecodeVersion(metadataBytes, versionID)
		if err != nil {
			return err
		}

		lock := Lock{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Version:     versionID,
			Naming:      version.Naming,
			Java:        java,
			Javap:       javap,
			Artifacts:   []artifact.DownloadResult{metadataResult},
		}
		libraries, libraryResults, err := downloadLibraries(ctx, downloader, metadata, versionDir, options)
		if err != nil {
			return err
		}
		lock.Artifacts = append(lock.Artifacts, libraryResults...)

		vineflowerTool, err := requireTool(tools, "vineflower-1.12.0")
		if err != nil {
			return err
		}
		vineflower, result, err := downloadTool(ctx, downloader, referenceDir, vineflowerTool)
		if err != nil {
			return err
		}
		lock.Artifacts = append(lock.Artifacts, result)

		for _, side := range requestedSides {
			download, ok := metadata.Downloads[side]
			if !ok {
				return fmt.Errorf("version %s has no %s download", versionID, side)
			}
			sideDir := filepath.Join(versionDir, side)
			original := filepath.Join(sideDir, "original.jar")
			progress(options, fmt.Sprintf("download %s %s", versionID, side))
			result, err := downloader.Download(ctx, artifact.DownloadSpec{URL: download.URL, Size: download.Size, SHA1: download.SHA1}, original)
			if err != nil {
				return err
			}
			lock.Artifacts = append(lock.Artifacts, result)

			analysisJar := original
			if side == "server" {
				analysisJar, err = decompile.ExecutableServer(original, filepath.Join(sideDir, "executable.jar"))
				if err != nil {
					return err
				}
			}
			progress(options, fmt.Sprintf("name %s %s", versionID, side))
			analysisJar, namingResults, err := prepareNamedJar(ctx, namingOptions{
				Version:      version,
				Side:         side,
				AnalysisJar:  analysisJar,
				VersionDir:   versionDir,
				ReferenceDir: referenceDir,
				Java:         java,
				Tools:        tools,
				Metadata:     metadata,
				Downloader:   downloader,
			})
			if err != nil {
				return err
			}
			lock.Artifacts = appendArtifactResults(lock.Artifacts, namingResults...)

			sourceDir := filepath.Join(referenceDir, "sources", versionID, side)
			progress(options, fmt.Sprintf("decompile %s %s", versionID, side))
			if err := decompile.RunVineflower(ctx, java, vineflower, analysisJar, sourceDir, libraries); err != nil {
				return err
			}
			indexDir := filepath.Join(referenceDir, "index", versionID, side)
			progress(options, fmt.Sprintf("index %s %s", versionID, side))
			if err := index.GenerateJavap(ctx, javap, analysisJar, filepath.Join(indexDir, "symbols.jsonl"), versionID, side); err != nil {
				return err
			}
			if err := index.GenerateSource(sourceDir, filepath.Join(indexDir, "sources.jsonl")); err != nil {
				return err
			}
			progress(options, fmt.Sprintf("validate %s %s", versionID, side))
			if _, err := validateOutput(validationOptions{
				Version:      version,
				Side:         side,
				Validation:   version.Sides[side],
				NamedJar:     analysisJar,
				SourcesIndex: filepath.Join(indexDir, "sources.jsonl"),
				SymbolsIndex: filepath.Join(indexDir, "symbols.jsonl"),
				ReportPath:   filepath.Join(sideDir, "compatibility.json"),
				JavaMajor:    toolchain.javaMajor,
				JavapMajor:   toolchain.javapMajor,
			}); err != nil {
				return err
			}
		}
		if err := writeJSON(filepath.Join(versionDir, "manifest.lock.json"), lock); err != nil {
			return err
		}
		activeVersionDir = ""
		activeSides = nil
	}
	return nil
}

// Clean removes one validated workspace-local reference directory.
func Clean(workspaceRoot, requested string) (string, error) {
	target, err := artifact.ResolveReferenceDir(workspaceRoot, requested)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(target); err != nil {
		return "", fmt.Errorf("remove %s: %w", target, err)
	}
	return target, nil
}

func downloadLibraries(ctx context.Context, downloader artifact.Downloader, metadata artifact.VersionMetadata, versionDir string, options Options) ([]string, []artifact.DownloadResult, error) {
	paths := make([]string, 0, len(metadata.Libraries))
	results := make([]artifact.DownloadResult, 0, len(metadata.Libraries))
	for _, library := range metadata.Libraries {
		if library.HasClassifier() {
			continue
		}
		remote := library.Downloads.Artifact
		if remote == nil {
			continue
		}
		relative, err := artifact.LibraryPath(library)
		if err != nil {
			return nil, nil, fmt.Errorf("library %s: %w", library.Name, err)
		}
		destination := filepath.Join(versionDir, "libraries", filepath.FromSlash(relative))
		progress(options, "download library "+library.Name)
		result, err := downloader.Download(ctx, artifact.DownloadSpec{URL: remote.URL, Size: remote.Size, SHA1: remote.SHA1}, destination)
		if err != nil {
			return nil, nil, err
		}
		paths = append(paths, destination)
		results = append(results, result)
	}
	sort.Strings(paths)
	return paths, results, nil
}

func downloadTool(ctx context.Context, downloader artifact.Downloader, referenceDir string, tool config.Tool) (string, artifact.DownloadResult, error) {
	parsed, err := url.Parse(tool.URL)
	if err != nil {
		return "", artifact.DownloadResult{}, fmt.Errorf("parse tool URL: %w", err)
	}
	extension := filepath.Ext(parsed.Path)
	path := filepath.Join(referenceDir, "cache", "tools", tool.ID+extension)
	result, err := downloader.Download(ctx, artifact.DownloadSpec{URL: tool.URL, SHA256: tool.SHA256}, path)
	return path, result, err
}

func requireTool(tools map[string]config.Tool, id string) (config.Tool, error) {
	tool, ok := tools[id]
	if !ok {
		return config.Tool{}, fmt.Errorf("required tool %q is not configured", id)
	}
	return tool, nil
}

func resolveExecutable(requested, fallback string) (string, error) {
	name := requested
	if name == "" {
		name = fallback
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("find %s executable: %w", name, err)
	}
	return path, nil
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func appendArtifactResults(existing []artifact.DownloadResult, additions ...artifact.DownloadResult) []artifact.DownloadResult {
	paths := make(map[string]struct{}, len(existing)+len(additions))
	for _, result := range existing {
		paths[result.Path] = struct{}{}
	}
	for _, result := range additions {
		if _, exists := paths[result.Path]; exists {
			continue
		}
		paths[result.Path] = struct{}{}
		existing = append(existing, result)
	}
	return existing
}

func invalidateVersionOutputs(versionDir string, sides []string) error {
	if err := removeIfPresent(filepath.Join(versionDir, "manifest.lock.json")); err != nil {
		return err
	}
	for _, side := range sides {
		if err := removeIfPresent(filepath.Join(versionDir, side, "compatibility.json")); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create JSON directory: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".json-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary JSON file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("set temporary JSON permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write temporary JSON file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary JSON file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary JSON file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func progress(options Options, message string) {
	if options.Progress != nil {
		options.Progress(message)
	}
}
