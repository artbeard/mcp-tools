// Package sandbox - lifecycle compose/tmux + HTTP readiness для start-sandbox CLI.
//
// Ownership: чужие tmux-сессии не трогаем. Свои помечаем опциями
// @codex_sandbox_project / @codex_sandbox_component при создании.
package sandbox

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/artbeard/mcp-tools/internal/gitcmd"
	"github.com/artbeard/mcp-tools/internal/workspace"
)

const (
	// Метки владения на tmux-сессии (user options). Без них сессия = unmanaged.
	optProject   = "@codex_sandbox_project"
	optComponent = "@codex_sandbox_component"
)

// Component - алиас нормализованного item из .workspace.yaml (после workspace.Load).
type Component = workspace.SandboxComponent

// Status - JSON-объект статуса компонента (поля как в Python component_status).
type Status map[string]any

// Run - subprocess с capture/timeout (переиспользуем gitcmd.Run, без git-safety).
func Run(cwd string, timeout time.Duration, command string, args ...string) gitcmd.Result {
	return gitcmd.Run(cwd, timeout, command, args...)
}

// composeBase - ["docker","compose", optional -f file...].
func composeBase(item Component) []string {
	cmd := []string{"docker", "compose"}
	for _, f := range item.ComposeFiles {
		cmd = append(cmd, "-f", f)
	}
	return cmd
}

// ComposeState - что compose считает «живым».
//
// ready  = все expected services в статусе running
// exists = хотя бы один expected есть в ps -q (контейнер создан)
// expected: из compose_services манифеста, иначе из `compose config --services`
//
// probe.type из YAML для compose игнорируется - всегда смотрим docker compose,
// не HTTP (как в Python).
func ComposeState(item Component) (ready, exists bool, details map[string]any) {
	cwd := item.Directory
	base := composeBase(item)
	expectedRes := Run(cwd, 30*time.Second, base[0], append(base[1:], "config", "--services")...)
	runningRes := Run(cwd, 30*time.Second, base[0], append(base[1:], "ps", "--status", "running", "--services")...)
	createdRes := Run(cwd, 30*time.Second, base[0], append(base[1:], "ps", "-q")...)
	if !expectedRes.Ok() || !runningRes.Ok() || !createdRes.Ok() {
		errMsg := strings.TrimSpace(expectedRes.Stderr)
		if errMsg == "" {
			errMsg = strings.TrimSpace(runningRes.Stderr)
		}
		if errMsg == "" {
			errMsg = strings.TrimSpace(createdRes.Stderr)
		}
		return false, false, map[string]any{"error": errMsg}
	}
	expected := map[string]struct{}{}
	if len(item.ComposeServices) > 0 {
		for _, s := range item.ComposeServices {
			expected[s] = struct{}{}
		}
	} else {
		for _, line := range strings.Split(expectedRes.Stdout, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				expected[line] = struct{}{}
			}
		}
	}
	running := map[string]struct{}{}
	for _, line := range strings.Split(runningRes.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			running[line] = struct{}{}
		}
	}
	created := map[string]struct{}{}
	for _, line := range strings.Split(createdRes.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			created[line] = struct{}{}
		}
	}
	// Пустой expected → ready=false (не считаем «всё ок, сервисов нет»).
	ready = len(expected) > 0 && subset(expected, running)
	exists = intersect(expected, created)
	return ready, exists, map[string]any{
		"expected_services": sortedKeys(expected),
		"running_services":  sortedKeys(running),
	}
}

func subset(a, b map[string]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func intersect(a, b map[string]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// HTTPReady - probe для tmux-фронтов.
//
// Намеренно без HTTP_PROXY (как Python ProxyHandler({})), иначе корпоративный
// proxy может сломать localhost.
// Ready = любой 2xx/3xx/4xx: 4xx значит «сервер отвечает», не «ещё поднимается».
// 5xx / network error = не ready.
func HTTPReady(rawURL string) (bool, map[string]any) {
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: func(*http.Request) (*url.URL, error) { return nil, nil },
		},
	}
	resp, err := client.Get(rawURL)
	if err != nil {
		return false, map[string]any{"url": rawURL, "error": err.Error()}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	ok := resp.StatusCode >= 200 && resp.StatusCode < 500
	return ok, map[string]any{"url": rawURL, "http_status": resp.StatusCode}
}

// tmuxHas - `=session` exact match (без prefix-поиска чужих имён).
func tmuxHas(session string) bool {
	r := Run("", 15*time.Second, "tmux", "has-session", "-t", "="+session)
	return r.Ok()
}

func tmuxOwner(session string) (project, component string) {
	p := Run("", 15*time.Second, "tmux", "show-options", "-qv", "-t", session, optProject)
	c := Run("", 15*time.Second, "tmux", "show-options", "-qv", "-t", session, optComponent)
	return strings.TrimSpace(p.Stdout), strings.TrimSpace(c.Stdout)
}

// ComponentStatus - единая state machine для JSON/CLI.
//
// compose: running | degraded (часть сервисов) | stopped
// tmux:    stopped | unmanaged (сессия есть, но не наши метки) |
//
//	running/degraded по probe
//
// иное:    invalid (process и т.п. не реализованы)
func ComponentStatus(item Component) Status {
	result := Status{
		"component": item.Component,
		"runtime":   item.Runtime,
		"ready":     false,
	}
	if item.Runtime == "compose" {
		ready, _, details := ComposeState(item)
		running := toSet(asStringSlice(details["running_services"]))
		expected := toSet(asStringSlice(details["expected_services"]))
		partial := intersect(expected, running)
		state := "stopped"
		if ready {
			state = "running"
		} else if partial {
			state = "degraded"
		}
		result["state"] = state
		result["ready"] = ready
		result["details"] = details
		return result
	}
	if item.Runtime != "tmux" {
		result["state"] = "invalid"
		result["details"] = map[string]any{"error": fmt.Sprintf("unsupported runtime: %s", item.Runtime)}
		return result
	}
	session := item.Session
	result["session"] = session
	if !tmuxHas(session) {
		result["state"] = "stopped"
		result["details"] = map[string]any{"session_exists": false}
		return result
	}
	ownerProject, ownerComponent := tmuxOwner(session)
	owned := ownerProject == item.Project && ownerComponent == item.Component
	if !owned {
		// Чужой/ручной session - start/stop откажутся его трогать.
		result["state"] = "unmanaged"
		result["details"] = map[string]any{
			"session_exists": true, "owner_project": ownerProject, "owner_component": ownerComponent,
		}
		return result
	}
	probeType, _ := item.Probe["type"].(string)
	var ready bool
	var details map[string]any
	if probeType == "http" {
		urlStr, _ := item.Probe["url"].(string)
		ready, details = HTTPReady(urlStr)
	} else {
		// Нет http-probe → достаточно «сессия наша и жива».
		ready, details = true, map[string]any{"session_exists": true}
	}
	state := "degraded"
	if ready {
		state = "running"
	}
	result["state"] = state
	result["ready"] = ready
	result["details"] = details
	return result
}

func asStringSlice(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, i := range x {
			if s, ok := i.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func toSet(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, i := range items {
		out[i] = struct{}{}
	}
	return out
}

// Validate - preflight до первой мутации (CLI зовёт перед start/stop/status...).
// Смотрит: обязательные поля, каталог, LookPath бинарей, уникальность tmux session,
// наличие required_docker_images. Не стартует ничего.
func Validate(items []Component) []string {
	var errors []string
	seenSessions := map[string]struct{}{}
	for _, item := range items {
		label := item.Project + "/" + item.Component
		if item.Project == "" || item.Component == "" || item.Directory == "" || item.Runtime == "" ||
			item.StartCommand == nil || item.StopCommand == nil || item.Probe == nil || item.TimeoutSeconds <= 0 {
			errors = append(errors, label+": неполные обязательные поля")
			continue
		}
		if st, err := os.Stat(item.Directory); err != nil || !st.IsDir() {
			errors = append(errors, fmt.Sprintf("%s: каталог не существует: %s", label, item.Directory))
		}
		// stop_command может быть [] - тогда LookPath не зовём.
		for _, cmd := range [][]string{item.StartCommand, item.StopCommand} {
			if len(cmd) > 0 {
				if _, err := exec.LookPath(cmd[0]); err != nil {
					errors = append(errors, fmt.Sprintf("%s: исполняемый файл недоступен: %s", label, cmd[0]))
				}
			}
		}
		if item.Runtime == "tmux" {
			session := item.Session
			if session == "" {
				errors = append(errors, fmt.Sprintf("%s: некорректная или дублирующаяся tmux session: %s", label, session))
			} else if _, dup := seenSessions[session]; dup {
				errors = append(errors, fmt.Sprintf("%s: некорректная или дублирующаяся tmux session: %s", label, session))
			} else {
				seenSessions[session] = struct{}{}
			}
			if _, err := exec.LookPath("tmux"); err != nil {
				errors = append(errors, fmt.Sprintf("%s: tmux недоступен", label))
			}
		}
		if item.Runtime != "tmux" && item.Runtime != "compose" {
			errors = append(errors, fmt.Sprintf("%s: неподдерживаемый runtime: %s", label, item.Runtime))
		}
		for _, image := range item.RequiredDockerImages {
			r := Run("", 60*time.Second, "docker", "image", "inspect", image)
			if !r.Ok() {
				errors = append(errors, fmt.Sprintf("%s: нет нужного Docker-образа: %s", label, image))
			}
		}
	}
	return errors
}

// Diagnostics - урезанные логи для diagnose / failed start|stop (≤12k символов).
// compose: ps + logs --tail 60; tmux: последние ~80 строк pane :0.0.
func Diagnostics(item Component) string {
	if item.Runtime == "compose" {
		var parts []string
		base := composeBase(item)
		for _, extra := range [][]string{{"ps"}, {"logs", "--tail", "60"}} {
			args := append(append([]string{}, base[1:]...), extra...)
			cmdLine := append([]string{base[0]}, args...)
			r := Run(item.Directory, 60*time.Second, base[0], args...)
			parts = append(parts, fmt.Sprintf("$ %s\n%s%s", shellJoin(cmdLine), r.Stdout, r.Stderr))
		}
		out := strings.Join(parts, "\n")
		if len(out) > 12000 {
			return out[len(out)-12000:]
		}
		return out
	}
	session := item.Session
	if !tmuxHas(session) {
		return "tmux session absent: " + session
	}
	r := Run("", 30*time.Second, "tmux", "capture-pane", "-p", "-S", "-80", "-t", session+":0.0")
	out := r.Stdout + r.Stderr
	if len(out) > 12000 {
		return out[len(out)-12000:]
	}
	return out
}

// shellJoin - простая shell-quoting для логов и аргумента tmux new-session
// (tmux получает одну команду-строку, не argv).
func shellJoin(parts []string) string {
	quoted := make([]string, len(parts))
	for i, p := range parts {
		if strings.ContainsAny(p, " \t\"'\\") {
			quoted[i] = "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
		} else {
			quoted[i] = p
		}
	}
	return strings.Join(quoted, " ")
}

// WaitUntilReady - крутим ComponentStatus каждые 500ms до timeout_seconds.
//
// Для tmux ранний выход, если сессия уже умерла / unmanaged / invalid
// (дальше ждать бессмысленно). Compose крутим до дедлайна даже на degraded.
func WaitUntilReady(item Component) Status {
	deadline := time.Now().Add(time.Duration(item.TimeoutSeconds) * time.Second)
	last := ComponentStatus(item)
	for time.Now().Before(deadline) {
		last = ComponentStatus(item)
		if ready, _ := last["ready"].(bool); ready {
			return last
		}
		state, _ := last["state"].(string)
		if item.Runtime == "tmux" && (state == "stopped" || state == "unmanaged" || state == "invalid") {
			return last
		}
		time.Sleep(500 * time.Millisecond)
	}
	return last
}

// StartComponent - идемпотентный start + wait ready.
//
// Отказы (не чиним сами):
//   - unmanaged: чужая сессия
//   - tmux degraded: наша сессия жива, но probe ещё/уже красный - не new-session поверх
//
// После создания tmux обязательно ставим ownership; если set-option упал -
// kill-session, чтобы не оставить «сироту» без меток.
func StartComponent(item Component) (Status, string) {
	current := ComponentStatus(item)
	if ready, _ := current["ready"].(bool); ready {
		current["operation"] = "already_running"
		return current, ""
	}
	state, _ := current["state"].(string)
	if state == "unmanaged" || (item.Runtime == "tmux" && state == "degraded") {
		return current, fmt.Sprintf("отказ заменить компонент в состоянии %s", state)
	}

	if item.Runtime == "compose" {
		timeout := time.Duration(item.TimeoutSeconds) * time.Second
		started := Run(item.Directory, timeout, item.StartCommand[0], item.StartCommand[1:]...)
		if !started.Ok() {
			return ComponentStatus(item), strings.TrimSpace(started.Stdout + started.Stderr)
		}
	} else {
		command := shellJoin(item.StartCommand)
		created := Run("", 30*time.Second, "tmux", "new-session", "-d",
			"-s", item.Session, "-c", item.Directory, "-n", item.Component, command)
		if !created.Ok() {
			return ComponentStatus(item), strings.TrimSpace(created.Stdout + created.Stderr)
		}
		for _, pair := range [][2]string{{optProject, item.Project}, {optComponent, item.Component}} {
			marked := Run("", 15*time.Second, "tmux", "set-option", "-q", "-t", item.Session, pair[0], pair[1])
			if !marked.Ok() {
				_ = Run("", 15*time.Second, "tmux", "kill-session", "-t", item.Session)
				return ComponentStatus(item), strings.TrimSpace(marked.Stdout + marked.Stderr)
			}
		}
	}

	status := WaitUntilReady(item)
	if ready, _ := status["ready"].(bool); ready {
		status["operation"] = "started"
		return status, ""
	}
	status["operation"] = "failed"
	return status, "таймаут readiness или процесс завершился"
}

// StopComponent - graceful stop_command, затем kill своей tmux-сессии.
//
// unmanaged не трогаем. volumes / docker volume rm - никогда (только stop_command из YAML).
// Успех = state снова stopped.
func StopComponent(item Component) (Status, string) {
	current := ComponentStatus(item)
	state, _ := current["state"].(string)
	if state == "stopped" {
		current["operation"] = "already_stopped"
		return current, ""
	}
	if state == "unmanaged" {
		return current, "отказ остановить unmanaged tmux session"
	}

	if len(item.StopCommand) > 0 {
		timeout := time.Duration(item.TimeoutSeconds) * time.Second
		stopped := Run(item.Directory, timeout, item.StopCommand[0], item.StopCommand[1:]...)
		if !stopped.Ok() {
			return ComponentStatus(item), strings.TrimSpace(stopped.Stdout + stopped.Stderr)
		}
	}
	if item.Runtime == "tmux" && tmuxHas(item.Session) {
		killed := Run("", 15*time.Second, "tmux", "kill-session", "-t", item.Session)
		if !killed.Ok() {
			return ComponentStatus(item), strings.TrimSpace(killed.Stdout + killed.Stderr)
		}
	}

	status := ComponentStatus(item)
	state, _ = status["state"].(string)
	if state == "stopped" {
		status["operation"] = "stopped"
		return status, ""
	}
	status["operation"] = "failed"
	return status, "компонент всё ещё присутствует после stop"
}

// AbsJoin - helper для тестов путей.
func AbsJoin(parts ...string) string {
	p := filepath.Join(parts...)
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
