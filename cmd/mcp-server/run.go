package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// root - корень mcp-tools (go.mod).
var root string

// resolveCmd: bin/<name> если есть и исполняемый, иначе go run <pkg>.
func resolveCmd(binName, goPkg string) []string {
	binary := filepath.Join(root, "bin", binName)
	if info, err := os.Stat(binary); err == nil && !info.IsDir() && isExecutable(binary) {
		return []string{binary}
	}
	return []string{"go", "run", goPkg}
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&0o111 != 0
}

// optFlag добавляет flag+value, если value не пустой.
func optFlag(flag, value string) []string {
	if value == "" {
		return nil
	}
	return []string{flag, value}
}

// writeTemp пишет текст во временный файл; вызывающий обязан удалить.
func writeTemp(text, suffix string) (string, error) {
	f, err := os.CreateTemp("", "mcp-tools-*"+suffix)
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// runCLI запускает команду из корня репо (или cwdOverride), склеивает stdout+stderr.
// При ненулевом exit: "exit=N\n<body>" - как Python server.py.
func runCLI(command []string, args []string, cwdOverride string) string {
	cmdLine := append(append([]string{}, command...), args...)
	cmd := exec.Command(cmdLine[0], cmdLine[1:]...)
	cmd.Dir = root
	if cwdOverride != "" {
		cmd.Dir = cwdOverride
	}
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	parts := make([]string, 0, 2)
	if s := strings.TrimRight(stdout.String(), "\n"); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimRight(stderr.String(), "\n"); s != "" {
		parts = append(parts, s)
	}
	body := strings.TrimSpace(strings.Join(parts, "\n"))

	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			if body == "" {
				return fmt.Sprintf("ОШИБКА: %v", err)
			}
			return fmt.Sprintf("ОШИБКА: %v\n%s", err, body)
		}
	}
	if code != 0 {
		if body == "" {
			return fmt.Sprintf("exit=%d", code)
		}
		return fmt.Sprintf("exit=%d\n%s", code, body)
	}
	if body == "" {
		return fmt.Sprintf("exit=%d", code)
	}
	return body
}

func hasArg(args map[string]any, key string) bool {
	_, ok := args[key]
	return ok
}
