# TODO: миграция скиллов → mcp-tools

Единый stdio MCP (`server.py`) + пакеты в `skills/`. Runtime для агентов — MCP tools, не `~/.codex/skills`.

## Перечень

| Было (`~/.codex/skills`) | Стало | Фаза | Статус |
|--------------------------|-------|------|--------|
| `start-project-sandbox` | `start-project-sandbox` | 2 | done |
| `prepare-project-task` | `prepare-project-task` | 2 | done |
| `prepare-project-merge-requests` | `prepare-project-merge-requests` | 2 | done |
| `deploy-project-task-to-dev` | `deploy-project-task-to-dev` | 2 | done |
| `abcage-tracker-timekeeper` | `yandex-tracker-timekeeper` | 3 | done |
| `abcage-tracker-issue-creator` | `yandex-tracker-issue-creator` | 4 | done |
| `abcage-tracker-commenter` | `yandex-tracker-commenter` | 4 | done |
| `abcage-mr-commenter` | `gitlab-mr-commenter` | 4 | done |
| `abcage-mr-reviewer` | `gitlab-mr-reviewer` | 5 | deferred |
| — | codestyle MCP/skill | 5 | deferred |

## Фазы

- [x] **1** — `TODO.md`, каркас `server.py`, README с конфигами Codex/Cursor/Zed
- [x] **2** — sandbox/git skills + MCP tools
- [x] **3** — `yandex-tracker-timekeeper`
- [x] **4** — Tracker issue/comment + `gitlab-mr-commenter` (`dry_run=true` по умолчанию)
- [ ] **5** — `gitlab-mr-reviewer` + кодстайл (отдельная сессия)

## MCP tools (текущие)

- `ping`
- `start_project_sandbox`
- `prepare_project_task`
- `prepare_project_merge_requests`
- `deploy_project_task_to_dev`
- `yandex_tracker_timekeeper`
- `yandex_tracker_issue_creator`
- `yandex_tracker_comment`
- `yandex_tracker_transition`
- `gitlab_mr_comment`

## Follow-up: `.workspace.yaml`

- [x] loader `workspace_manifest.py`
- [x] prepare / mr / deploy / sandbox на манифест
- [x] `config/workspace.example.yaml`
- [x] `scripts/migrate_central_config_to_workspace.py`
- Локально: `python scripts/migrate_central_config_to_workspace.py --write`
