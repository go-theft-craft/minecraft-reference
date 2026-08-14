package artifact

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const (
	// VersionManifestURL is Mojang's Java Edition version index.
	VersionManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"
	libraryBaseURL     = "https://libraries.minecraft.net/"
	maxMetadataSize    = int64(16 << 20)
)

// RemoteFile is a downloadable artifact declared by Mojang metadata.
type RemoteFile struct {
	Path string `json:"path,omitempty"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

// Library is a Java library entry from a version manifest.
type Library struct {
	Name      string `json:"name"`
	Downloads struct {
		Artifact *RemoteFile `json:"artifact"`
	} `json:"downloads"`
}

// HasClassifier reports whether the Maven coordinate includes a classifier.
func (library Library) HasClassifier() bool {
	parts := strings.Split(library.Name, ":")
	return len(parts) > 3
}

// VersionMetadata contains the analysis inputs for one Minecraft version.
type VersionMetadata struct {
	ID        string                `json:"id"`
	Downloads map[string]RemoteFile `json:"downloads"`
	Libraries []Library             `json:"libraries"`
}

type manifest struct {
	Versions []manifestVersion `json:"versions"`
}

type manifestVersion struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
}

// Resolver reads Mojang's version metadata without downloading game artifacts.
type Resolver struct {
	Client      *http.Client
	ManifestURL string
}

// Resolve finds the verified version-metadata download for an exact version ID.
func (r Resolver) Resolve(ctx context.Context, version string) (VersionMetadata, DownloadSpec, error) {
	manifestURL := r.ManifestURL
	if manifestURL == "" {
		manifestURL = VersionManifestURL
	}
	var index manifest
	if err := r.getJSON(ctx, manifestURL, &index); err != nil {
		return VersionMetadata{}, DownloadSpec{}, fmt.Errorf("resolve version manifest: %w", err)
	}

	var selected *manifestVersion
	for i := range index.Versions {
		entry := &index.Versions[i]
		if entry.ID != version {
			continue
		}
		if selected != nil {
			return VersionMetadata{}, DownloadSpec{}, fmt.Errorf("version manifest contains duplicate id %q", version)
		}
		selected = entry
	}
	if selected == nil {
		return VersionMetadata{}, DownloadSpec{}, fmt.Errorf("version %q is absent from Mojang metadata", version)
	}
	if selected.URL == "" || selected.SHA1 == "" {
		return VersionMetadata{}, DownloadSpec{}, fmt.Errorf("version %q has incomplete metadata", version)
	}

	spec := DownloadSpec{URL: selected.URL, SHA1: selected.SHA1}
	return VersionMetadata{}, spec, nil
}

// DecodeVersion validates downloaded version metadata and fills legacy library URLs.
func (r Resolver) DecodeVersion(data []byte, expectedID string) (VersionMetadata, error) {
	var metadata VersionMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return VersionMetadata{}, fmt.Errorf("decode version metadata: %w", err)
	}
	if metadata.ID != expectedID {
		return VersionMetadata{}, fmt.Errorf("version metadata id mismatch: got %q, want %q", metadata.ID, expectedID)
	}
	for side, download := range metadata.Downloads {
		if download.URL == "" || download.SHA1 == "" || download.Size <= 0 {
			return VersionMetadata{}, fmt.Errorf("download %q has incomplete metadata", side)
		}
	}
	for i := range metadata.Libraries {
		artifact := metadata.Libraries[i].Downloads.Artifact
		if artifact == nil {
			continue
		}
		if artifact.URL == "" && artifact.Path != "" {
			artifact.URL = libraryBaseURL + strings.TrimPrefix(artifact.Path, "/")
		}
	}
	return metadata, nil
}

func (r Resolver) getJSON(ctx context.Context, source string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	client := r.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("GET %s: %w", source, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: unexpected HTTP status %s", source, response.Status)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxMetadataSize))
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode %s: %w", source, err)
	}
	return nil
}

// LibraryPath returns a safe relative cache path for a Java library.
func LibraryPath(library Library) (string, error) {
	artifact := library.Downloads.Artifact
	if artifact == nil {
		return "", errors.New("library has no artifact")
	}
	if artifact.Path != "" {
		cleaned := path.Clean("/" + artifact.Path)
		if strings.Contains(cleaned, "..") {
			return "", errors.New("library path contains traversal")
		}
		return strings.TrimPrefix(cleaned, "/"), nil
	}
	parsed, err := url.Parse(artifact.URL)
	if err != nil {
		return "", fmt.Errorf("parse library URL: %w", err)
	}
	name := path.Base(parsed.Path)
	if name == "." || name == "/" || name == "" {
		return "", errors.New("library URL has no file name")
	}
	return name, nil
}
