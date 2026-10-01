package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func getFrontendDistDir(value string) (string, error) {
	configured := strings.TrimSpace(value)
	if configured != "" {
		directory, err := filepath.Abs(configured)
		if err != nil {
			return "", fmt.Errorf("resolve FRONTEND_DIST_DIR: %w", err)
		}
		info, err := os.Stat(filepath.Join(directory, "index.html"))
		if err != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("FRONTEND_DIST_DIR %q does not contain index.html", configured)
		}
		return directory, nil
	}

	for _, candidate := range []string{"frontend/dist", "../frontend/dist"} {
		directory, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		info, err := os.Stat(filepath.Join(directory, "index.html"))
		if err == nil && info.Mode().IsRegular() {
			return directory, nil
		}
	}
	return "", nil
}
