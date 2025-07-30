package utils_test

import (
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
	"path/filepath"
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
