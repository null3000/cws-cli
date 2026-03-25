package manifest

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Manifest represents the parsed fields from a Chrome extension manifest.json.
type Manifest struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ManifestVersion int    `json:"manifest_version"`
}

// Parse reads and parses a manifest.json file from disk.
func Parse(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest.json: %w", err)
	}
	return parseJSON(data)
}

// ParseFromZip reads and parses manifest.json from zip data.
func ParseFromZip(data []byte) (*Manifest, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to read zip: %w", err)
	}

	for _, f := range reader.File {
		if f.Name == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open manifest.json in zip: %w", err)
			}
			defer rc.Close()

			manifestData, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("failed to read manifest.json from zip: %w", err)
			}
			return parseJSON(manifestData)
		}
	}

	return nil, fmt.Errorf("manifest.json not found in zip")
}

func parseJSON(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest.json is not valid JSON: %w", err)
	}
	return &m, nil
}

// ValidateRequired checks that all required fields are present.
// Returns a list of missing field names.
func ValidateRequired(m *Manifest) []string {
	var missing []string
	if m.Name == "" {
		missing = append(missing, "name")
	}
	if m.Version == "" {
		missing = append(missing, "version")
	}
	if m.ManifestVersion == 0 {
		missing = append(missing, "manifest_version")
	}
	return missing
}

// ValidateVersion checks that a version string follows Chrome's version format:
// 1-4 dot-separated integers, each 0-65535, not all zeros.
func ValidateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version is empty")
	}

	parts := strings.Split(version, ".")
	if len(parts) < 1 || len(parts) > 4 {
		return fmt.Errorf("version must have 1-4 dot-separated parts, got %d", len(parts))
	}

	allZero := true
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("version contains empty segment")
		}
		// No leading zeros (except "0" itself)
		if len(part) > 1 && part[0] == '0' {
			return fmt.Errorf("version segment %q has leading zeros", part)
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("version segment %q is not a valid integer", part)
		}
		if n < 0 || n > 65535 {
			return fmt.Errorf("version segment %d is out of range (0-65535)", n)
		}
		if n != 0 {
			allZero = false
		}
	}

	if allZero {
		return fmt.Errorf("version cannot be all zeros")
	}

	return nil
}

// parseVersionParts splits a version string into integer segments,
// padding to 4 parts with zeros.
func parseVersionParts(version string) ([4]int, error) {
	var result [4]int
	parts := strings.Split(version, ".")
	for i, part := range parts {
		if i >= 4 {
			break
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return result, fmt.Errorf("invalid version segment %q", part)
		}
		result[i] = n
	}
	return result, nil
}

// CompareVersions returns true if version a is strictly greater than version b.
// Uses Chrome's left-to-right comparison with missing segments treated as 0.
func CompareVersions(a, b string) (bool, error) {
	pa, err := parseVersionParts(a)
	if err != nil {
		return false, fmt.Errorf("invalid version %q: %w", a, err)
	}
	pb, err := parseVersionParts(b)
	if err != nil {
		return false, fmt.Errorf("invalid version %q: %w", b, err)
	}

	for i := 0; i < 4; i++ {
		if pa[i] > pb[i] {
			return true, nil
		}
		if pa[i] < pb[i] {
			return false, nil
		}
	}

	return false, nil // equal
}
