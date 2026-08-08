// Package envfile - разбор KEY=VALUE env-файлов.
// Основной вход: LoadDir - .env (обязателен) + .env.local (опционально, поверх).
package envfile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadDir читает dir/.env (обязателен), затем dir/.env.local (если есть).
// Ключи из .env.local переопределяют .env. В os.Environ ничего не пишется.
func LoadDir(dir string) (map[string]string, error) {
	base := filepath.Join(dir, ".env")
	values, err := Load(base)
	if err != nil {
		return nil, fmt.Errorf(".env обязателен (%s): %w", base, err)
	}

	local := filepath.Join(dir, ".env.local")
	if _, err := os.Stat(local); err != nil {
		if os.IsNotExist(err) {
			return values, nil
		}
		return nil, fmt.Errorf(".env.local: %w", err)
	}
	override, err := Load(local)
	if err != nil {
		return nil, err
	}
	for k, v := range override {
		values[k] = v
	}
	return values, nil
}

// Load читает один файл path: KEY=VALUE, # и пустые строки пропускает.
func Load(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("env file: %w", err)
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`) // убрать кавычки вокруг значения
		if key != "" {
			values[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("env file: %w", err)
	}
	return values, nil
}
