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
| codeStyle (корп. MCP hub) | `codestyle` | `codestyle_find_rules` / `codestyle_read_rule` | Go (анонимизированный snapshot) |
| `abcage-mr-reviewer` | `gitlab-mr-reviewer` | - | **не в репо** (остался в `~/.codex/skills`) |

## Ещё не перенесено

- [ ] `abcage-mr-reviewer` → `gitlab-mr-reviewer` (оркестратор ревью MR; будет звать `codestyle_*`)

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
- [x] `codestyle` → `bin/codestyle`
- [x] MCP-сервер → `bin/mcp-server` (`cmd/mcp-server`)
- [x] общие пакеты: `internal/envfile`, `internal/tracker`, `internal/gitlab`, `internal/timerstate`, `internal/repopath`, `internal/workspace`, `internal/gitcmd`, `internal/sandbox`, `internal/codestyle`

## Cleanup

- [x] Один источник кредов: корневой `.env` / `.env.local`
- [x] README без Python MCP
- [x] Runtime Python убран
- [x] Манифест workspace на Go (`internal/workspace`)
- [x] Codestyle: анонимизация + offline Go tools
- [ ] Вынести жёстко зашитые имена веток (`master`/`main`/`dev`) в конфиг / `.workspace.yaml`

## Отложено

- [ ] `gitlab-mr-reviewer` (оркестратор поверх GitLab + локальный `codestyle_*`)

## Follow-up: `.workspace.yaml`

- [x] loader Go `internal/workspace`
- [x] prepare / mr / deploy / sandbox на манифест
- [x] `config/workspace.example.yaml`
