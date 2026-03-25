package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/null3000/cws-cli/internal/manifest"
	cwszip "github.com/null3000/cws-cli/internal/zip"
)

// --- End-to-end validation: good manifest ---

func TestValidate_GoodManifest(t *testing.T) {
	dir := t.TempDir()

	// Write a valid Chrome extension
	writeFile(t, filepath.Join(dir, "manifest.json"), `{
		"name": "My Cool Extension",
		"version": "1.3.0",
		"manifest_version": 3,
		"description": "Does cool things",
		"icons": {
			"16": "icon16.png",
			"48": "icon48.png",
			"128": "icon128.png"
		},
		"action": {
			"default_popup": "popup.html"
		},
		"permissions": ["storage"]
	}`)
	writeFile(t, filepath.Join(dir, "popup.html"), "<html><body>Hello</body></html>")
	writeFile(t, filepath.Join(dir, "popup.js"), "console.log('popup')")
	writeFile(t, filepath.Join(dir, "icon16.png"), "png16")
	writeFile(t, filepath.Join(dir, "icon48.png"), "png48")
	writeFile(t, filepath.Join(dir, "icon128.png"), "png128")

	// Parse manifest
	m, err := manifest.Parse(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// All required fields present
	missing := manifest.ValidateRequired(m)
	if len(missing) != 0 {
		t.Errorf("expected no missing fields, got %v", missing)
	}

	// Version is valid
	if err := manifest.ValidateVersion(m.Version); err != nil {
		t.Errorf("version should be valid: %v", err)
	}

	// Can zip and the zip contains manifest
	zipData, err := cwszip.ZipDirectory(dir)
	if err != nil {
		t.Fatalf("ZipDirectory failed: %v", err)
	}
	hasManifest, err := cwszip.ContainsManifestInZip(zipData)
	if err != nil {
		t.Fatalf("ContainsManifestInZip failed: %v", err)
	}
	if !hasManifest {
		t.Error("zip should contain manifest.json")
	}

	// Can parse manifest from the zip too
	mFromZip, err := manifest.ParseFromZip(zipData)
	if err != nil {
		t.Fatalf("ParseFromZip failed: %v", err)
	}
	if mFromZip.Version != "1.3.0" {
		t.Errorf("version from zip = %q, want %q", mFromZip.Version, "1.3.0")
	}

	// Version 1.3.0 > 1.2.0 (simulating published version)
	higher, err := manifest.CompareVersions(m.Version, "1.2.0")
	if err != nil {
		t.Fatalf("CompareVersions failed: %v", err)
	}
	if !higher {
		t.Error("1.3.0 should be higher than 1.2.0")
	}
}

// --- End-to-end validation: bad manifest ---

func TestValidate_BadManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{name: broken, no quotes}`)

	_, err := manifest.Parse(filepath.Join(dir, "manifest.json"))
	if err == nil {
		t.Fatal("expected parse error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("error should mention invalid JSON, got: %v", err)
	}
}

func TestValidate_BadManifest_MissingName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{
		"version": "1.0.0",
		"manifest_version": 3
	}`)

	m, err := manifest.Parse(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	missing := manifest.ValidateRequired(m)
	if len(missing) != 1 || missing[0] != "name" {
		t.Errorf("expected [name] missing, got %v", missing)
	}
}

func TestValidate_BadManifest_MissingVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{
		"name": "No Version Extension",
		"manifest_version": 3
	}`)

	m, err := manifest.Parse(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	missing := manifest.ValidateRequired(m)
	if len(missing) != 1 || missing[0] != "version" {
		t.Errorf("expected [version] missing, got %v", missing)
	}
}

func TestValidate_BadManifest_MissingManifestVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{
		"name": "Old Extension",
		"version": "1.0.0"
	}`)

	m, err := manifest.Parse(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	missing := manifest.ValidateRequired(m)
	if len(missing) != 1 || missing[0] != "manifest_version" {
		t.Errorf("expected [manifest_version] missing, got %v", missing)
	}
}

func TestValidate_BadManifest_AllFieldsMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{"description": "just a description"}`)

	m, err := manifest.Parse(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	missing := manifest.ValidateRequired(m)
	if len(missing) != 3 {
		t.Errorf("expected 3 missing fields, got %v", missing)
	}
}

func TestValidate_BadManifest_BadVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{
		"name": "Bad Version Extension",
		"version": "not.a.version.lol",
		"manifest_version": 3
	}`)

	m, err := manifest.Parse(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if err := manifest.ValidateVersion(m.Version); err == nil {
		t.Error("expected version validation to fail for 'not.a.version.lol'")
	}
}

func TestValidate_BadManifest_VersionNotBumped(t *testing.T) {
	// Simulates the exact scenario: local version == published version
	localVersion := "2026.3.20"
	publishedVersion := "2026.3.20"

	higher, err := manifest.CompareVersions(localVersion, publishedVersion)
	if err != nil {
		t.Fatalf("CompareVersions failed: %v", err)
	}
	if higher {
		t.Error("same version should NOT be considered higher")
	}
}

func TestValidate_BadManifest_VersionLowerThanPublished(t *testing.T) {
	localVersion := "1.0.0"
	publishedVersion := "1.0.1"

	higher, err := manifest.CompareVersions(localVersion, publishedVersion)
	if err != nil {
		t.Fatalf("CompareVersions failed: %v", err)
	}
	if higher {
		t.Error("lower version should NOT be considered higher")
	}
}

func TestValidate_NoManifestFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "popup.html"), "<html></html>")

	if cwszip.ContainsManifest(dir) {
		t.Error("directory without manifest.json should fail ContainsManifest")
	}
}

func TestValidate_EmptyManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{}`)

	m, err := manifest.Parse(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	missing := manifest.ValidateRequired(m)
	if len(missing) != 3 {
		t.Errorf("empty manifest should have 3 missing fields, got %v", missing)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
