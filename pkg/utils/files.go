package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/AdamShannag/volare/pkg/types"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ResolveTargetPath(mountPath string, file types.ObjectToDownload) string {
	relPath := file.ActualPath

	if strings.HasPrefix(file.ActualPath, file.Path+"/") {
		relPath = strings.TrimPrefix(file.ActualPath, file.Path+"/")
	} else if file.ActualPath == file.Path {
		relPath = filepath.Base(file.Path)
	}

	return filepath.Join(mountPath, relPath)
}

func IsFile(p string) bool {
	return !strings.HasSuffix(p, "/") && strings.Contains(filepath.Base(p), ".") && p != ""
}

func ReadFilesAsBase64(root string) (map[string]string, error) {
	files := make(map[string]string)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		encoded := base64.StdEncoding.EncodeToString(content)
		files[relPath] = encoded
		return nil
	})

	if err != nil {
		return nil, err
	}
	return files, nil
}

func WriteResourcesDir(data string, path string) error {
	resourceMap := make(map[string]string)
	if err := json.Unmarshal([]byte(data), &resourceMap); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("failed to create resources directory: %w", err)
	}

	for relPath, b64Content := range resourceMap {
		bytes, err := base64.StdEncoding.DecodeString(b64Content)
		if err != nil {
			return fmt.Errorf("failed to decode base64 for %q: %w", relPath, err)
		}

		fullPath := filepath.Join(path, relPath)
		if err = os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return fmt.Errorf("failed to create directory for %q: %w", fullPath, err)
		}

		if err = os.WriteFile(fullPath, bytes, 0o644); err != nil {
			return fmt.Errorf("failed to write file %q: %w", fullPath, err)
		}
	}

	return nil
}
