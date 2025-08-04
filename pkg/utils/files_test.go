package utils_test

import (
	"encoding/base64"
	"encoding/json"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTargetPath(t *testing.T) {
	tests := []struct {
		name      string
		mountPath string
		file      types.ObjectToDownload
		expected  string
	}{
		{
			name:      "Nested file inside directory",
			mountPath: "/mnt",
			file: types.ObjectToDownload{
				Path:       "folder",
				ActualPath: "folder/subdir/file.txt",
			},
			expected: filepath.Join("/mnt", "subdir/file.txt"),
		},
		{
			name:      "Single file with same path and actual path",
			mountPath: "/mnt",
			file: types.ObjectToDownload{
				Path:       "folder/file.txt",
				ActualPath: "folder/file.txt",
			},
			expected: filepath.Join("/mnt", "file.txt"),
		},
		{
			name:      "File at root level",
			mountPath: "/mnt",
			file: types.ObjectToDownload{
				Path:       "file.txt",
				ActualPath: "file.txt",
			},
			expected: filepath.Join("/mnt", "file.txt"),
		},
		{
			name:      "Folder structure preserved when no match",
			mountPath: "/mnt",
			file: types.ObjectToDownload{
				Path:       "data",
				ActualPath: "other/path/file.txt",
			},
			expected: filepath.Join("/mnt", "other/path/file.txt"),
		},
		{
			name:      "Path is prefix but with trailing slash",
			mountPath: "/mnt",
			file: types.ObjectToDownload{
				Path:       "data/",
				ActualPath: "data/file.txt",
			},
			expected: filepath.Join("/mnt", "data/file.txt"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.ResolveTargetPath(tt.mountPath, tt.file)
			if result != tt.expected {
				t.Errorf("ResolveTargetPath() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestIsFile(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{input: "folder/file.txt", expected: true},
		{input: "folder/file", expected: false},
		{input: "folder/", expected: false},
		{input: "", expected: false},
		{input: "index.html", expected: true},
		{input: "data/file.tar.gz", expected: true},
		{input: "data/.hiddenfile", expected: true},
		{input: "data/folder.with.dots/", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := utils.IsFile(tt.input)
			if result != tt.expected {
				t.Errorf("IsFile(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestReadFilesAsBase64(t *testing.T) {
	tmpDir := t.TempDir()

	sampleFiles := map[string]string{
		"file1.txt":        "hello world",
		"subdir/file2.txt": "another file",
	}
	for path, content := range sampleFiles {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write file %s: %v", path, err)
		}
	}

	filesMap, err := utils.ReadFilesAsBase64(tmpDir)
	if err != nil {
		t.Fatalf("ReadFilesAsBase64 returned error: %v", err)
	}

	for k, want := range sampleFiles {
		enc, ok := filesMap[k]
		if !ok {
			t.Errorf("missing key %q in result", k)
			continue
		}
		decoded, dErr := base64.StdEncoding.DecodeString(enc)
		if dErr != nil {
			t.Errorf("failed to decode base64 for key %q: %v", k, dErr)
			continue
		}
		if got := string(decoded); got != want {
			t.Errorf("content mismatch for key %q: got %q, want %q", k, got, want)
		}
	}

	err = os.Chmod(tmpDir, 0o000)
	if err != nil {
		t.Logf("skipping permission error test due to platform limitations: %v", err)
	} else {
		defer func(name string, mode os.FileMode) {
			_ = os.Chmod(name, mode)
		}(tmpDir, 0o755)
		_, err = utils.ReadFilesAsBase64(tmpDir)
		if err == nil {
			t.Errorf("expected error on unreadable directory, got nil")
		}
	}
}

func TestWriteResourcesDir(t *testing.T) {
	tmpDir := t.TempDir()

	sampleFiles := map[string]string{
		"foo.txt":        base64.StdEncoding.EncodeToString([]byte("foo content")),
		"nested/bar.txt": base64.StdEncoding.EncodeToString([]byte("bar content")),
	}
	jsonBytes, err := json.Marshal(sampleFiles)
	if err != nil {
		t.Fatalf("failed to marshal sampleFiles: %v", err)
	}

	err = utils.WriteResourcesDir(string(jsonBytes), tmpDir)
	if err != nil {
		t.Fatalf("WriteResourcesDir returned error: %v", err)
	}

	for relPath, b64Content := range sampleFiles {
		fullPath := filepath.Join(tmpDir, relPath)
		data, dErr := os.ReadFile(fullPath)
		if dErr != nil {
			t.Errorf("failed to read written file %q: %v", fullPath, dErr)
			continue
		}
		expected, _ := base64.StdEncoding.DecodeString(b64Content)
		if string(data) != string(expected) {
			t.Errorf("file content mismatch for %q: got %q, want %q", fullPath, string(data), string(expected))
		}
	}

	err = utils.WriteResourcesDir("not-json", tmpDir)
	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal JSON") {
		t.Errorf("expected JSON unmarshal error, got %v", err)
	}

	badJSON := `{"badfile.txt": "!!!not base64!!!"}`
	err = utils.WriteResourcesDir(badJSON, tmpDir)
	if err == nil || !strings.Contains(err.Error(), "failed to decode base64") {
		t.Errorf("expected base64 decode error, got %v", err)
	}
}
