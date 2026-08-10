// Package workspace - загрузка .workspace.yaml (subset для prepare-task).
// Sandbox / полный validator - позже; prepare не требует probe/compose.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const ManifestName = ".workspace.yaml"

var (
	AllowedWorkBases = map[string]struct{}{"master": {}, "main": {}}
	AllowedTypes     = map[string]struct{}{"multi_repo": {}, "monorepo": {}}
)

// Error - ошибка манифеста / discovery.
type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

func errf(format string, args ...any) error {
	return &Error{Msg: fmt.Sprintf(format, args...)}
}

// Repo - репозиторий после resolve путей.
type Repo struct {
	ID           string // id из YAML (или label, если id нет)
	Label        string // то же, что ID - имя репы в JSON/логах
	Path         string // абсолютный путь к git work tree
	RelPath      string // path из YAML относительно корня манифеста
	Remote       string // имя remote (обычно origin)
	BaseBranch   string // ветка-база для task: master|main
	TargetBranch string // branches.mr_target (для --list / будущий MR)
	DevBranch    string // branches.dev (для --list / будущий deploy)
}

// Manifest - нормализованный манифест (prepare-subset + поля для list).
type Manifest struct {
	SchemaVersion         int      // сейчас только 1
	Name                  string   // ключ проекта (project key)
	Type                  string   // multi_repo | monorepo
	Aliases               []string // альтернативные имена для --project
	Root                  string   // каталог, где лежит .workspace.yaml
	ManifestPath          string   // полный путь к файлу манифеста
	ProtectedBaseBranches []string // git.protected_base_branches (master/main)
	ProtectedBranches     []string // git.protected_branches (для list/MR/deploy)
	Repositories          []Repo   // список реп с abs path
	SandboxComponentIDs   []string // id sandbox.components (только для --list)
}

// PrepareProject - адаптер как Python as_prepare_project.
type PrepareProject struct {
	Key          string        // = Manifest.Name
	Type         string        // multi_repo | monorepo
	Workspace    string        // = Manifest.Root
	Repositories []PrepareRepo // урезанный список полей для check/prepare
}

// PrepareRepo - repo для prepare/check.
type PrepareRepo struct {
	Label      string // имя репы в отчёте
	Path       string // abs path work tree
	Remote     string // remote для fetch/ls-remote
	BaseBranch string // master|main - откуда режем task-ветку
}

// Find ищет .workspace.yaml вверх от start.
func Find(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", errf("не найден %s, искали от %s", ManifestName, start)
	}
	info, err := os.Stat(current)
	if err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		candidate := filepath.Join(current, ManifestName)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", errf("не найден %s, искали от %s", ManifestName, start)
}

// Load читает и валидирует манифест (prepare-subset).
func Load(path string) (*Manifest, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, errf("манифест не найден: %s", path)
	}
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		path = filepath.Join(path, ManifestName)
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return nil, errf("манифест не найден: %s", path)
	}
	root := filepath.Dir(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errf("%s: %v", path, err)
	}
	var data map[string]any
	if err := yaml.Unmarshal(raw, &data); err != nil {
		return nil, errf("%s: %v", path, err)
	}
	if data == nil {
		return nil, errf("%s: манифест должен быть словарём (YAML mapping)", path)
	}
	return parseManifest(data, root, path)
}

// Discover - явный --manifest или walk-up от cwd.
func Discover(cwd, manifestPath string) (*Manifest, error) {
	if strings.TrimSpace(manifestPath) != "" {
		return Load(manifestPath)
	}
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	found, err := Find(cwd)
	if err != nil {
		return nil, err
	}
	return Load(found)
}

// AssertProjectName - --project должен быть name или alias.
func AssertProjectName(m *Manifest, explicit string) error {
	if strings.TrimSpace(explicit) == "" {
		return nil
	}
	allowed := map[string]struct{}{m.Name: {}}
	for _, a := range m.Aliases {
		allowed[a] = struct{}{}
	}
	if _, ok := allowed[explicit]; ok {
		return nil
	}
	names := []string{m.Name}
	names = append(names, m.Aliases...)
	return errf("проект %q не совпадает с name/aliases манифеста %v (%s)", explicit, names, m.ManifestPath)
}

// AsPrepare - проект для check/prepare.
func AsPrepare(m *Manifest) PrepareProject {
	repos := make([]PrepareRepo, 0, len(m.Repositories))
	for _, r := range m.Repositories {
		repos = append(repos, PrepareRepo{
			Label:      r.Label,
			Path:       r.Path,
			Remote:     r.Remote,
			BaseBranch: r.BaseBranch,
		})
	}
	return PrepareProject{
		Key:          m.Name,
		Type:         m.Type,
		Workspace:    m.Root,
		Repositories: repos,
	}
}

// ListPayload - JSON для --list (+ push_allowed добавляет CLI).
func ListPayload(m *Manifest) map[string]any {
	repos := make([]map[string]any, 0, len(m.Repositories))
	for _, r := range m.Repositories {
		repos = append(repos, map[string]any{
			"id":     r.ID,
			"path":   r.Path,
			"remote": r.Remote,
			"branches": map[string]any{
				"base":      r.BaseBranch,
				"mr_target": r.TargetBranch,
				"dev":       r.DevBranch,
			},
		})
	}
	return map[string]any{
		"status":             "ok",
		"action":             "list",
		"name":               m.Name,
		"type":               m.Type,
		"aliases":            m.Aliases,
		"root":               m.Root,
		"manifest_path":      m.ManifestPath,
		"repositories":       repos,
		"sandbox_components": m.SandboxComponentIDs,
	}
}

func parseManifest(data map[string]any, root, manifestPath string) (*Manifest, error) {
	version := 1
	if v, ok := data["schema_version"]; ok {
		switch n := v.(type) {
		case int:
			version = n
		case int64:
			version = int(n)
		case float64:
			version = int(n)
		default:
			return nil, errf("неподдерживаемая schema_version: %#v", v)
		}
	}
	if version != 1 {
		return nil, errf("неподдерживаемая schema_version: %d", version)
	}

	name, err := requireStr(data["name"], "name")
	if err != nil {
		return nil, err
	}
	kind, err := requireStr(data["type"], "type")
	if err != nil {
		return nil, err
	}
	if _, ok := AllowedTypes[kind]; !ok {
		return nil, errf("type должен быть monorepo или multi_repo")
	}

	aliases, err := stringList(data["aliases"], "aliases", true)
	if err != nil {
		return nil, err
	}

	reposRaw, ok := data["repositories"].([]any)
	if !ok || len(reposRaw) == 0 {
		return nil, errf("repositories: нужен непустой список")
	}
	if kind == "monorepo" && len(reposRaw) != 1 {
		return nil, errf("monorepo: нужна ровно одна репа")
	}
	if kind == "multi_repo" && len(reposRaw) < 2 {
		return nil, errf("multi_repo: нужно минимум две репы")
	}

	gitMap, _ := data["git"].(map[string]any)
	if gitMap == nil {
		gitMap = map[string]any{}
	}
	protectedBase, err := stringList(gitMap["protected_base_branches"], "git.protected_base_branches", true)
	if err != nil {
		return nil, err
	}
	if len(protectedBase) == 0 {
		protectedBase = []string{"master", "main"}
	}
	protected, err := stringList(gitMap["protected_branches"], "git.protected_branches", true)
	if err != nil {
		return nil, err
	}
	if len(protected) == 0 {
		protected = []string{"master", "main", "dev"}
	}
	protectedBaseSet := toSet(protectedBase)

	seenIDs := map[string]struct{}{}
	repos := make([]Repo, 0, len(reposRaw))
	for i, item := range reposRaw {
		rm, ok := item.(map[string]any)
		if !ok {
			return nil, errf("repositories[%d]: должен быть словарь", i)
		}
		idSrc := rm["id"]
		if idSrc == nil {
			idSrc = rm["label"]
		}
		repoID, err := requireStr(idSrc, fmt.Sprintf("repositories[%d].id", i))
		if err != nil {
			return nil, err
		}
		if _, dup := seenIDs[repoID]; dup {
			return nil, errf("дублируется id репозитория: %s", repoID)
		}
		seenIDs[repoID] = struct{}{}
		relPath, err := requireStr(rm["path"], fmt.Sprintf("repositories[%d].path", i))
		if err != nil {
			return nil, err
		}
		absPath, err := filepath.Abs(filepath.Join(root, relPath))
		if err != nil {
			return nil, err
		}
		remote := "origin"
		if rm["remote"] != nil {
			remote, err = requireStr(rm["remote"], fmt.Sprintf("repositories[%d].remote", i))
			if err != nil {
				return nil, err
			}
		}
		branches, _ := rm["branches"].(map[string]any)
		if branches == nil {
			branches = map[string]any{}
		}
		base := "master"
		if branches["base"] != nil {
			base, err = requireStr(branches["base"], fmt.Sprintf("repositories[%d].branches.base", i))
			if err != nil {
				return nil, err
			}
		}
		if _, ok := AllowedWorkBases[base]; !ok {
			return nil, errf("%s: branches.base только master или main, сейчас %s", repoID, base)
		}
		if _, ok := protectedBaseSet[base]; !ok {
			return nil, errf("%s: branches.base должен быть в git.protected_base_branches", repoID)
		}
		mrTarget := base
		if branches["mr_target"] != nil {
			mrTarget, err = requireStr(branches["mr_target"], fmt.Sprintf("repositories[%d].branches.mr_target", i))
			if err != nil {
				return nil, err
			}
		}
		dev := "dev"
		if branches["dev"] != nil {
			dev, err = requireStr(branches["dev"], fmt.Sprintf("repositories[%d].branches.dev", i))
			if err != nil {
				return nil, err
			}
		}
		repos = append(repos, Repo{
			ID: repoID, Label: repoID, Path: absPath, RelPath: relPath,
			Remote: remote, BaseBranch: base, TargetBranch: mrTarget, DevBranch: dev,
		})
	}

	// sandbox: только id для --list, без строгой валидации
	var sandboxIDs []string
	if sb, ok := data["sandbox"].(map[string]any); ok {
		if comps, ok := sb["components"].([]any); ok {
			for _, c := range comps {
				cm, ok := c.(map[string]any)
				if !ok {
					continue
				}
				idSrc := cm["id"]
				if idSrc == nil {
					idSrc = cm["component"]
				}
				if s, ok := idSrc.(string); ok && strings.TrimSpace(s) != "" {
					sandboxIDs = append(sandboxIDs, strings.TrimSpace(s))
				}
			}
		}
	}
	if sandboxIDs == nil {
		sandboxIDs = []string{}
	}
	if aliases == nil {
		aliases = []string{}
	}

	rootAbs, _ := filepath.Abs(root)
	manifestAbs, _ := filepath.Abs(manifestPath)
	return &Manifest{
		SchemaVersion:         1,
		Name:                  name,
		Type:                  kind,
		Aliases:               aliases,
		Root:                  rootAbs,
		ManifestPath:          manifestAbs,
		ProtectedBaseBranches: protectedBase,
		ProtectedBranches:     protected,
		Repositories:          repos,
		SandboxComponentIDs:   sandboxIDs,
	}, nil
}

func requireStr(v any, field string) (string, error) {
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", errf("%s: нужна непустая строка", field)
	}
	return strings.TrimSpace(s), nil
}

func stringList(v any, field string, allowNil bool) ([]string, error) {
	if v == nil {
		if allowNil {
			return nil, nil
		}
		return nil, errf("%s: нужен список строк", field)
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, errf("%s: нужен список строк", field)
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok || s == "" {
			return nil, errf("%s: нужен список строк", field)
		}
		out = append(out, s)
	}
	return out, nil
}

func toSet(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, i := range items {
		out[i] = struct{}{}
	}
	return out
}
