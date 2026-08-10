# TODO: миграция скиллов → mcp-tools

Единый stdio MCP (`server.py`) + пакеты в `skills/`. Runtime для агентов - MCP tools, не `~/.codex/skills`.

## Перечень (исходная миграция в репо)

| Было (`~/.codex/skills`) | Стало | MCP | Статус |
|--------------------------|-------|-----|--------|
| `start-project-sandbox` | `start-project-sandbox` | `start_project_sandbox` | в репо (Python) |
| `prepare-project-task` | `prepare-project-task` | `prepare_project_task` | Go |
| `prepare-project-merge-requests` | `prepare-project-merge-requests` | `prepare_project_merge_requests` | Go |
| `deploy-project-task-to-dev` | `deploy-project-task-to-dev` | `deploy_project_task_to_dev` | Go |
| `abcage-tracker-timekeeper` | `yandex-tracker-timekeeper` | `yandex_tracker_timekeeper` | Go |
| `abcage-tracker-issue-creator` | `yandex-tracker-issue-creator` | `yandex_tracker_issue_creator` | Go |
| `abcage-tracker-commenter` | `yandex-tracker-commenter` | `yandex_tracker_comment` / `yandex_tracker_transition` | Go |
| `abcage-mr-commenter` | `gitlab-mr-commenter` | `gitlab_mr_comment` | Go |
| `abcage-mr-reviewer` | `gitlab-mr-reviewer` | - | **не в репо** (остался в `~/.codex/skills`) |
| codeStyle / правила ревью (внутри reviewer) | отдельный skill/MCP или часть reviewer | - | **не в репо** |

## Ещё не перенесено из оригинальных скиллов

Даже как Python-пакет в `skills/` - пока только в `~/.codex/skills`:

- [ ] `abcage-mr-reviewer` → `gitlab-mr-reviewer` (оркестратор ревью MR)
- [ ] **кодстайл / codeStyle** - правила и чеклисты из `abcage-mr-reviewer/references/` (и связанный стиль ревью); отдельный skill/MCP или часть reviewer

## Go: уже перенесено

- [x] `tracker-transition` → `bin/tracker-transition`
- [x] `tracker-comment` → `bin/tracker-comment`
- [x] `tracker-create-issue` → `bin/tracker-create-issue`
- [x] `tracker-time` → `bin/tracker-time`
- [x] `gitlab-mr-comment` → `bin/gitlab-mr-comment`
- [x] `prepare-task` → `bin/prepare-task`
- [x] `prepare-mr` → `bin/prepare-mr`
- [x] `deploy-task` → `bin/deploy-task`
- [x] общие пакеты: `internal/envfile` (`.env` + `.env.local`), `internal/tracker`, `internal/gitlab`, `internal/timerstate`, `internal/repopath`, `internal/workspace`, `internal/gitcmd`

## Go: осталось перенести

Локальный git / sandbox. Пока Python под `skills/*/scripts/`.

- [x] `prepare-project-task` → `bin/prepare-task`
- [x] `prepare-project-merge-requests` → `bin/prepare-mr`
- [x] `deploy-project-task-to-dev` → `bin/deploy-task`
- [ ] `start-project-sandbox` (`start_sandbox.py`) - docker/tmux/process lifecycle

Порядок дальше: sandbox.

## После порта git-скиллов (cleanup)

- [ ] Один источник кредов: только корневой `.env` / `.env.local`; убрать `config/tracker.env` / `config/gitlab.env` и чтение из `mcp_paths`
- [ ] Обновить README (сейчас местами ещё про `config/*.env`)
- [ ] Удалить пустые/legacy `scripts/` и `.sh`-обёртки, если не нужны
- [ ] `workspace_manifest` на Go (или тонкий вызов из CLI), чтобы не тащить Python ради манифеста
- [ ] Вынести жёстко зашитые имена веток (`master`/`main`/`dev`, ForbiddenTaskBranches, AllowedWorkBases, литерал push `dev`) в конфиг / `.workspace.yaml`

## Отложено

- [ ] `gitlab-mr-reviewer` + **кодстайл** (см. секцию выше - сначала занести в репо из `~/.codex/skills`)
- [ ] MCP-сервер на Go (сейчас `server.py` - тонкая обёртка над CLI)

## Follow-up: `.workspace.yaml`

- [x] loader `workspace_manifest.py`
- [x] prepare / mr / deploy / sandbox на манифест
- [x] `config/workspace.example.yaml`
- [x] `scripts/migrate_central_config_to_workspace.py`
- Локально: `python scripts/migrate_central_config_to_workspace.py --write`
