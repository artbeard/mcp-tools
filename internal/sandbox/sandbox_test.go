package sandbox_test

import (
	"testing"

	"github.com/artbeard/mcp-tools/internal/sandbox"
	"github.com/artbeard/mcp-tools/internal/workspace"
)

// Порт :1 обычно закрыт → network error → ready=false.
// Проверяем контракт HTTPReady: не паникует, отдаёт url в details.
func TestHTTPReadyBadURL(t *testing.T) {
	ok, details := sandbox.HTTPReady("http://127.0.0.1:1/")
	if ok {
		t.Fatalf("expected not ready: %#v", details)
	}
	if details["url"] == nil {
		t.Fatal("url missing")
	}
}

// Validate ловит несуществующий directory до старта lifecycle.
// (остальные поля валидные, чтобы ошибка была именно про каталог).
func TestValidateMissingDir(t *testing.T) {
	errs := sandbox.Validate([]workspace.SandboxComponent{{
		Project: "p", Component: "c", Directory: "/no/such/dir/xyz",
		Runtime: "compose", StartCommand: []string{"true"}, StopCommand: []string{},
		Probe: map[string]any{"type": "compose"}, TimeoutSeconds: 10,
	}})
	if len(errs) == 0 {
		t.Fatal("expected errors")
	}
}

// runtime "process" в манифесте возможен как строка, но lifecycle его не умеет → state=invalid.
func TestComponentStatusInvalidRuntime(t *testing.T) {
	st := sandbox.ComponentStatus(workspace.SandboxComponent{
		Project: "p", Component: "c", Directory: t.TempDir(),
		Runtime: "process", StartCommand: []string{"true"}, StopCommand: []string{},
		Probe: map[string]any{"type": "compose"}, TimeoutSeconds: 5,
	})
	if st["state"] != "invalid" {
		t.Fatalf("%#v", st)
	}
}
