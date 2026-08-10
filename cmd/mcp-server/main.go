// stdio MCP-сервер: тонкая обёртка над Go CLI в bin/ (раньше server.py).
package main

import (
	"fmt"
	"os"

	"github.com/artbeard/mcp-tools/internal/repopath"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	var err error
	root, err = repopath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-server: %v\n", err)
		os.Exit(1)
	}

	s := server.NewMCPServer(
		"dev-tools",
		"0.1.0",
		server.WithInstructions(
			"Детерминированные tools агента для sandbox/git, Yandex Tracker и GitLab. "+
				"Запись по умолчанию dry-run; dry_run=false только после явного подтверждения пользователем.",
		),
		server.WithToolCapabilities(false),
	)
	registerTools(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "mcp-server: %v\n", err)
		os.Exit(1)
	}
}
