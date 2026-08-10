// Package repopath - поиск корня репозитория mcp-tools на диске.
package repopath

import (
	"fmt"
	"os"
	"path/filepath"
)

// Root ищет корень mcp-tools: поднимается вверх от бинаря и от cwd,
// пока не найдёт go.mod.
func Root() (string, error) {
	seen := map[string]struct{}{}
	var starts []string

	// Откуда запущен бинарь (bin/tracker-transition → …/mcp-tools)
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			starts = append(starts, filepath.Dir(resolved))
		} else {
			starts = append(starts, filepath.Dir(exe))
		}
	}
	// И текущая рабочая директория (если go run из корня репо)
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}

	for _, start := range starts {
		for dir := start; ; dir = filepath.Dir(dir) {
			if _, ok := seen[dir]; ok {
				break
			}
			seen[dir] = struct{}{}
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break // дошли до корня ФС
			}
		}
	}
	return "", fmt.Errorf("mcp-tools корень не найден (нужен go.mod)")
}

// EnvDir - каталог с .env / .env.local (корень mcp-tools).
func EnvDir() (string, error) {
	return Root()
}
