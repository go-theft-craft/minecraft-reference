package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/archive"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/artifact"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/mapping"
)

type namingOptions struct {
	Version      config.Version
	Side         string
	AnalysisJar  string
	VersionDir   string
	ReferenceDir string
	Java         string
	Tools        map[string]config.Tool
	Metadata     artifact.VersionMetadata
	Downloader   artifact.Downloader
}

var remapJar = mapping.Remap

func prepareNamedJar(ctx context.Context, options namingOptions) (string, []artifact.DownloadResult, error) {
	if !options.Version.SupportsSide(options.Side) {
		return "", nil, fmt.Errorf("version %q does not support side %q", options.Version.ID, options.Side)
	}
	if options.Version.Naming == "identity" {
		return options.AnalysisJar, nil, nil
	}

	var (
		mappingFile string
		results     []artifact.DownloadResult
		err         error
	)
	switch options.Version.Naming {
	case "mcp":
		mappingFile, results, err = prepareMCPMapping(ctx, options)
	case "mojang":
		mappingFile, results, err = prepareMojangMapping(ctx, options)
	default:
		return "", nil, fmt.Errorf("unsupported naming strategy %q for version %q", options.Version.Naming, options.Version.ID)
	}
	if err != nil {
		return "", results, err
	}

	specialSource, result, err := downloadRequiredTool(ctx, options.Downloader, options.ReferenceDir, options.Tools, "specialsource-1.11.4")
	if err != nil {
		return "", results, err
	}
	results = append(results, result)
	named := filepath.Join(options.VersionDir, options.Side, "named.jar")
	if err := remapJar(ctx, options.Java, specialSource, options.AnalysisJar, named, mappingFile); err != nil {
		return "", results, err
	}
	return named, results, nil
}

func prepareMCPMapping(ctx context.Context, options namingOptions) (string, []artifact.DownloadResult, error) {
	if options.Version.Mapping == nil {
		return "", nil, fmt.Errorf("version %q has no MCP mapping configuration", options.Version.ID)
	}
	mappingDir := filepath.Join(options.VersionDir, "mappings")
	switch options.Version.Mapping.Format {
	case "tiny-v1":
		path, result, err := downloadRequiredTool(ctx, options.Downloader, options.ReferenceDir, options.Tools, options.Version.Mapping.Tool)
		if err != nil {
			return "", nil, err
		}
		input, err := os.Open(path)
		if err != nil {
			return "", []artifact.DownloadResult{result}, fmt.Errorf("open Tiny v1 mapping: %w", err)
		}
		parsed, parseErr := mapping.ParseTinyV1(input)
		closeErr := input.Close()
		if parseErr != nil {
			return "", []artifact.DownloadResult{result}, fmt.Errorf("parse Tiny v1 mapping for %s: %w", options.Version.ID, parseErr)
		}
		if closeErr != nil {
			return "", []artifact.DownloadResult{result}, fmt.Errorf("close Tiny v1 mapping: %w", closeErr)
		}
		destination := filepath.Join(mappingDir, "named.srg")
		if err := writeSRGFile(destination, parsed); err != nil {
			return "", []artifact.DownloadResult{result}, err
		}
		return destination, []artifact.DownloadResult{result}, nil
	case "srg-csv":
		return prepareMCPArchiveMapping(ctx, options, mappingDir)
	default:
		return "", nil, fmt.Errorf("unsupported MCP mapping format %q for version %q", options.Version.Mapping.Format, options.Version.ID)
	}
}

func prepareMCPArchiveMapping(ctx context.Context, options namingOptions, mappingDir string) (string, []artifact.DownloadResult, error) {
	srgArchive, srgResult, err := downloadRequiredTool(ctx, options.Downloader, options.ReferenceDir, options.Tools, options.Version.Mapping.SRGTool)
	if err != nil {
		return "", nil, err
	}
	results := []artifact.DownloadResult{srgResult}
	namesArchive, namesResult, err := downloadRequiredTool(ctx, options.Downloader, options.ReferenceDir, options.Tools, options.Version.Mapping.NamesTool)
	if err != nil {
		return "", results, err
	}
	results = append(results, namesResult)

	joined := filepath.Join(mappingDir, "joined.srg")
	fields := filepath.Join(mappingDir, "fields.csv")
	methods := filepath.Join(mappingDir, "methods.csv")
	for _, extraction := range []struct {
		archive, name, destination string
	}{
		{srgArchive, "joined.srg", joined},
		{namesArchive, "fields.csv", fields},
		{namesArchive, "methods.csv", methods},
	} {
		if err := archive.ExtractFile(extraction.archive, extraction.name, extraction.destination); err != nil {
			return "", results, err
		}
	}
	composed := filepath.Join(mappingDir, "joined-named.srg")
	if err := mapping.ComposeMCP(joined, fields, methods, composed); err != nil {
		return "", results, err
	}
	return composed, results, nil
}

func prepareMojangMapping(ctx context.Context, options namingOptions) (string, []artifact.DownloadResult, error) {
	remote, ok := options.Metadata.MappingDownload(options.Side)
	if !ok {
		return "", nil, fmt.Errorf("version %q has no %s_mappings download for side %q", options.Version.ID, options.Side, options.Side)
	}
	mappingDir := filepath.Join(options.VersionDir, "mappings")
	downloaded := filepath.Join(mappingDir, options.Side+".txt")
	result, err := options.Downloader.Download(ctx, artifact.DownloadSpec{
		URL:  remote.URL,
		Size: remote.Size,
		SHA1: remote.SHA1,
	}, downloaded)
	if err != nil {
		return "", nil, err
	}
	input, err := os.Open(downloaded)
	if err != nil {
		return "", []artifact.DownloadResult{result}, fmt.Errorf("open Mojang mapping: %w", err)
	}
	parsed, parseErr := mapping.ParseProGuard(input)
	closeErr := input.Close()
	if parseErr != nil {
		return "", []artifact.DownloadResult{result}, fmt.Errorf("parse Mojang mapping for %s %s: %w", options.Version.ID, options.Side, parseErr)
	}
	if closeErr != nil {
		return "", []artifact.DownloadResult{result}, fmt.Errorf("close Mojang mapping: %w", closeErr)
	}
	destination := filepath.Join(mappingDir, options.Side+"-named.srg")
	if err := writeSRGFile(destination, parsed); err != nil {
		return "", []artifact.DownloadResult{result}, err
	}
	return destination, []artifact.DownloadResult{result}, nil
}

func writeSRGFile(destination string, parsed mapping.Mapping) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return fmt.Errorf("create mapping directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".mapping-*.srg")
	if err != nil {
		return fmt.Errorf("create SRG output: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := mapping.WriteSRG(temporary, parsed); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close SRG output: %w", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("publish SRG output: %w", err)
	}
	return nil
}

func downloadRequiredTool(ctx context.Context, downloader artifact.Downloader, referenceDir string, tools map[string]config.Tool, id string) (string, artifact.DownloadResult, error) {
	tool, err := requireTool(tools, id)
	if err != nil {
		return "", artifact.DownloadResult{}, err
	}
	return downloadTool(ctx, downloader, referenceDir, tool)
}
