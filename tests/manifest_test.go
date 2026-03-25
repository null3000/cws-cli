package tests

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/null3000/cws-cli/internal/manifest"
)

// --- ValidateVersion tests ---

func TestValidateVersion_Valid(t *testing.T) {
	valid := []string{
		"1",
		"0.1",
		"1.0",
		"1.0.0",
		"1.0.0.0",
		"0.0.0.1",
		"1.2.3.4",
		"65535.65535.65535.65535",
		"0.0.1",
	}
	for _, v := range valid {
		if err := manifest.ValidateVersion(v); err != nil {
			t.Errorf("ValidateVersion(%q) = %v, want nil", v, err)
		}
	}
}

func TestValidateVersion_Invalid(t *testing.T) {
	invalid := []struct {
		version string
		reason  string
	}{
		{"", "empty"},
		{"0", "all zeros"},
		{"0.0", "all zeros"},
		{"0.0.0.0", "all zeros"},
		{"1.2.3.4.5", "too many segments"},
		{"1.2.abc", "non-integer"},
		{"65536", "out of range"},
		{"-1", "negative"},
		{"01.2", "leading zero"},
		{"1..2", "empty segment"},
		{"1.2.", "trailing dot (empty segment)"},
	}
	for _, tt := range invalid {
		if err := manifest.ValidateVersion(tt.version); err == nil {
			t.Errorf("ValidateVersion(%q) = nil, want error (%s)", tt.version, tt.reason)
		}
	}
}

// --- CompareVersions tests ---

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.2.3", "1.2.2", true},
		{"1.1", "1.0.9", true},
		{"2.0", "1.9.9.9", true},
		{"1.0.1", "1.0.0", true},
		{"0.0.0.2", "0.0.0.1", true},
		// Equal
		{"1.0", "1.0.0", false},
		{"1.0.0.0", "1.0", false},
		// Less
		{"1.2.2", "1.2.3", false},
		{"1.0", "1.1", false},
		{"0.9", "1.0", false},
	}
	for _, tt := range tests {
		got, err := manifest.CompareVersions(tt.a, tt.b)
		if err != nil {
			t.Errorf("CompareVersions(%q, %q) error: %v", tt.a, tt.b, err)
			continue
		}
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

// --- Parse tests ---

func TestParse_Valid(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	os.WriteFile(manifestPath, []byte(`{
		"name": "Test Extension",
		"version": "1.2.3",
		"manifest_version": 3
	}`), 0644)

	m, err := manifest.Parse(manifestPath)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if m.Name != "Test Extension" {
		t.Errorf("Name = %q, want %q", m.Name, "Test Extension")
	}
	if m.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", m.Version, "1.2.3")
	}
	if m.ManifestVersion != 3 {
		t.Errorf("ManifestVersion = %d, want 3", m.ManifestVersion)
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	os.WriteFile(manifestPath, []byte(`not json`), 0644)

	_, err := manifest.Parse(manifestPath)
	if err == nil {
		t.Fatal("Parse() should fail for invalid JSON")
	}
}

func TestParse_FileNotFound(t *testing.T) {
	_, err := manifest.Parse("/nonexistent/manifest.json")
	if err == nil {
		t.Fatal("Parse() should fail for missing file")
	}
}

func TestParseFromZip_Valid(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("manifest.json")
	f.Write([]byte(`{"name": "Zip Ext", "version": "2.0", "manifest_version": 3}`))
	w.Close()

	m, err := manifest.ParseFromZip(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseFromZip() error: %v", err)
	}
	if m.Name != "Zip Ext" {
		t.Errorf("Name = %q, want %q", m.Name, "Zip Ext")
	}
	if m.Version != "2.0" {
		t.Errorf("Version = %q, want %q", m.Version, "2.0")
	}
}

func TestParseFromZip_NoManifest(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("other.txt")
	f.Write([]byte("hello"))
	w.Close()

	_, err := manifest.ParseFromZip(buf.Bytes())
	if err == nil {
		t.Fatal("ParseFromZip() should fail when no manifest.json")
	}
}

// --- ValidateRequired tests ---

func TestValidateRequired_AllPresent(t *testing.T) {
	m := &manifest.Manifest{Name: "Test", Version: "1.0", ManifestVersion: 3}
	missing := manifest.ValidateRequired(m)
	if len(missing) != 0 {
		t.Errorf("expected no missing fields, got %v", missing)
	}
}

func TestValidateRequired_MissingFields(t *testing.T) {
	m := &manifest.Manifest{}
	missing := manifest.ValidateRequired(m)
	if len(missing) != 3 {
		t.Errorf("expected 3 missing fields, got %v", missing)
	}
}

func TestValidateRequired_PartialMissing(t *testing.T) {
	m := &manifest.Manifest{Name: "Test"}
	missing := manifest.ValidateRequired(m)
	if len(missing) != 2 {
		t.Errorf("expected 2 missing fields (version, manifest_version), got %v", missing)
	}
}
