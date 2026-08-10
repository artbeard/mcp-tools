# TODO: миграция скиллов → mcp-tools

Единый stdio MCP (`bin/mcp-server`) + пакеты в `skills/`. Runtime для агентов - MCP tools, не `~/.codex/skills`.

## Перечень (исходная миграция в репо)

| Было (`~/.codex/skills`) | Стало | MCP | Статус |
|--------------------------|-------|-----|--------|
| `start-project-sandbox` | `start-project-sandbox` | `start_project_sandbox` | Go |
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

Даже как пакет в `skills/` - пока только в `~/.codex/skills` (перенос одним приёмом позже):

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
- [x] `start-sandbox` → `bin/start-sandbox`
- [x] MCP-сервер → `bin/mcp-server` (`cmd/mcp-server`)
- [x] общие пакеты: `internal/envfile` (`.env` + `.env.local`), `internal/tracker`, `internal/gitlab`, `internal/timerstate`, `internal/repopath`, `internal/workspace`, `internal/gitcmd`, `internal/sandbox`

## Cleanup

- [x] Один источник кредов: корневой `.env` / `.env.local` (legacy `config/*.env` не читается)
- [x] README без Python MCP / `config/*.env` как primary
- [x] Runtime Python убран (`server.py`, `workspace_manifest.py`, `mcp_paths.py`, `requirements.txt`)
- [x] Манифест workspace на Go (`internal/workspace`); `scripts/migrate_*.py` оставлен как one-shot для legacy JSON
- [ ] Вынести жёстко зашитые имена веток (`master`/`main`/`dev`, ForbiddenTaskBranches, AllowedWorkBases, литерал push `dev`) в конфиг / `.workspace.yaml`

## Отложено

- [ ] `gitlab-mr-reviewer` + **кодстайл** (см. секцию выше - сначала занести в репо из `~/.codex/skills`, одним приёмом)

## Follow-up: `.workspace.yaml`

- [x] loader Go `internal/workspace`
- [x] prepare / mr / deploy / sandbox на манифест
- [x] `config/workspace.example.yaml`
- [x] `scripts/migrate_central_config_to_workspace.py` (опциональный one-shot)
- Локально (если ещё есть `config/projects-*.json`): `python3 scripts/migrate_central_config_to_workspace.py --write`
