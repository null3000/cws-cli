package tests

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"

	cwszip "github.com/null3000/cws-cli/internal/zip"
)

func createFile(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file %s: %v", path, err)
	}
}

func zipEntryNames(t *testing.T, data []byte) []string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("failed to read zip: %v", err)
	}
	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names
}

func TestZipDirectory_Basic(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "manifest.json"), `{"name":"test"}`)
	createFile(t, filepath.Join(dir, "popup.html"), "<html></html>")
	createFile(t, filepath.Join(dir, "background.js"), "console.log('hi')")

	data, err := cwszip.ZipDirectory(dir)
	if err != nil {
		t.Fatalf("ZipDirectory error: %v", err)
	}

	names := zipEntryNames(t, data)
	expected := []string{"background.js", "manifest.json", "popup.html"}
	if len(names) != len(expected) {
		t.Fatalf("zip entries = %v, want %v", names, expected)
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("entry[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestZipDirectory_ExcludesDefaults(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "manifest.json"), `{}`)
	createFile(t, filepath.Join(dir, "popup.js"), "")

	// Create files/dirs that should be excluded
	createFile(t, filepath.Join(dir, ".git", "config"), "")
	createFile(t, filepath.Join(dir, ".gitignore"), "")
	createFile(t, filepath.Join(dir, ".github", "workflows", "ci.yml"), "")
	createFile(t, filepath.Join(dir, ".DS_Store"), "")
	createFile(t, filepath.Join(dir, "Thumbs.db"), "")
	createFile(t, filepath.Join(dir, "node_modules", "lodash", "index.js"), "")
	createFile(t, filepath.Join(dir, "cws.toml"), "secret")
	createFile(t, filepath.Join(dir, "package.json"), "{}")
	createFile(t, filepath.Join(dir, "package-lock.json"), "{}")
	createFile(t, filepath.Join(dir, "yarn.lock"), "")
	createFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "")
	createFile(t, filepath.Join(dir, "tsconfig.json"), "{}")
	createFile(t, filepath.Join(dir, ".npmrc"), "//registry")
	createFile(t, filepath.Join(dir, ".vscode", "settings.json"), "")
	createFile(t, filepath.Join(dir, ".idea", "workspace.xml"), "")
	createFile(t, filepath.Join(dir, "__MACOSX", "._file"), "")

	data, err := cwszip.ZipDirectory(dir)
	if err != nil {
		t.Fatalf("ZipDirectory error: %v", err)
	}

	names := zipEntryNames(t, data)
	expected := []string{"manifest.json", "popup.js"}
	if len(names) != len(expected) {
		t.Fatalf("zip entries = %v, want only %v", names, expected)
	}
}

func TestZipDirectory_ExcludesMapFiles(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "manifest.json"), `{}`)
	createFile(t, filepath.Join(dir, "app.js"), "")
	createFile(t, filepath.Join(dir, "app.js.map"), "")
	createFile(t, filepath.Join(dir, "style.css.map"), "")

	data, err := cwszip.ZipDirectory(dir)
	if err != nil {
		t.Fatalf("ZipDirectory error: %v", err)
	}

	names := zipEntryNames(t, data)
	for _, name := range names {
		if filepath.Ext(name) == ".map" {
			t.Errorf("zip should not contain .map file: %s", name)
		}
	}
}

func TestZipDirectory_PreservesNestedStructure(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "manifest.json"), `{}`)
	createFile(t, filepath.Join(dir, "icons", "icon16.png"), "png")
	createFile(t, filepath.Join(dir, "scripts", "content", "main.js"), "")

	data, err := cwszip.ZipDirectory(dir)
	if err != nil {
		t.Fatalf("ZipDirectory error: %v", err)
	}

	names := zipEntryNames(t, data)
	expected := []string{"icons/icon16.png", "manifest.json", "scripts/content/main.js"}
	if len(names) != len(expected) {
		t.Fatalf("zip entries = %v, want %v", names, expected)
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("entry[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestContainsManifest_Present(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "manifest.json"), `{}`)

	if !cwszip.ContainsManifest(dir) {
		t.Error("ContainsManifest returned false, want true")
	}
}

func TestContainsManifest_Absent(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "index.html"), "")

	if cwszip.ContainsManifest(dir) {
		t.Error("ContainsManifest returned true, want false")
	}
}

func TestContainsManifestInZip_Present(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("manifest.json")
	f.Write([]byte(`{}`))
	w.Close()

	got, err := cwszip.ContainsManifestInZip(buf.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("ContainsManifestInZip returned false, want true")
	}
}

func TestContainsManifestInZip_Absent(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("index.html")
	f.Write([]byte("<html></html>"))
	w.Close()

	got, err := cwszip.ContainsManifestInZip(buf.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("ContainsManifestInZip returned true, want false")
	}
}

func TestZipDirectory_SkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "manifest.json"), `{}`)
	createFile(t, filepath.Join(dir, "real.js"), "console.log('real')")

	// Create a symlink pointing outside the directory
	symlinkPath := filepath.Join(dir, "sneaky_link")
	target := filepath.Join(os.TempDir(), "symlink_target_test.txt")
	os.WriteFile(target, []byte("secret data"), 0644)
	defer os.Remove(target)

	err := os.Symlink(target, symlinkPath)
	if err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	data, err := cwszip.ZipDirectory(dir)
	if err != nil {
		t.Fatalf("ZipDirectory error: %v", err)
	}

	names := zipEntryNames(t, data)
	for _, name := range names {
		if name == "sneaky_link" {
			t.Error("zip should not contain symlinked file")
		}
	}
	// Should still contain real files
	found := false
	for _, name := range names {
		if name == "real.js" {
			found = true
		}
	}
	if !found {
		t.Error("zip should contain real.js")
	}
}

func TestShouldExclude(t *testing.T) {
	tests := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{".git", true, true},
		{".gitignore", false, true},
		{".github", true, true},
		{".DS_Store", false, true},
		{"Thumbs.db", false, true},
		{"__MACOSX", true, true},
		{".vscode", true, true},
		{".idea", true, true},
		{"node_modules", true, true},
		{"package.json", false, true},
		{"package-lock.json", false, true},
		{"yarn.lock", false, true},
		{"pnpm-lock.yaml", false, true},
		{"tsconfig.json", false, true},
		{".npmrc", false, true},
		{"cws.toml", false, true},
		{"app.js.map", false, true},
		{"style.css.map", false, true},
		// Should NOT be excluded
		{"manifest.json", false, false},
		{"popup.html", false, false},
		{"background.js", false, false},
		{"icons", true, false},
		{"scripts/content.js", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := cwszip.ShouldExclude(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("ShouldExclude(%q, %v) = %v, want %v", tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}
