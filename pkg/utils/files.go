package utils

import (
	"github.com/AdamShannag/volare/pkg/types"
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
