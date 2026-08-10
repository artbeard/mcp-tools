# mcp-tools

Единый **stdio MCP-сервер** с детерминированными tools для агентов (Zed / Codex CLI / Cursor).  
Логика живёт в `skills/*/scripts`; MCP - thin wrapper. Отдельный daemon не нужен: хост сам стартует процесс по `command` + `args`.

```text
mcp-tools/
  server.py          # точка входа MCP (stdio)
  cmd/               # Go CLI tools (постепенно)
  internal/          # общий Go-код
  skills/            # пакеты (SKILL.md + scripts + references)
  config/            # env examples + workspace.example.yaml
  TODO.md
  requirements.txt
  go.mod
  .venv/             # локальный python + mcp
```

## Установка

```bash
git clone <repo-url> mcp-tools
cd mcp-tools
curl -LsSf https://astral.sh/uv/install.sh | sh   # если ещё нет uv
uv venv .venv
uv pip install --python .venv/bin/python -r requirements.txt
```

Проверка:

```bash
.venv/bin/python -c "from server import mcp; print('ok', mcp.name)"
```

В конфигах агентов нужен **абсолютный** путь к клону (хосты обычно не резолвят `~` в `command`/`args`). Подставь свой:

```bash
REPO="$(pwd)"   # из корня клона, например /home/you/src/mcp-tools
echo "$REPO"
```

## Подключение MCP

Один и тот же `command`/`args` везде; ключ конфига разный. Ниже `<REPO>` = абсолютный путь к клону.

### Cursor (`~/.cursor/mcp.json` или `.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "dev-tools": {
      "command": "<REPO>/.venv/bin/python",
      "args": ["<REPO>/server.py"]
    }
  }
}
```

### Codex CLI (`~/.codex/config.toml`)

```toml
[mcp_servers.dev-tools]
command = "<REPO>/.venv/bin/python"
args = ["<REPO>/server.py"]
```

### Zed (`settings.json`)

```json
{
  "context_servers": {
    "dev-tools": {
      "source": "custom",
      "command": "<REPO>/.venv/bin/python",
      "args": ["<REPO>/server.py"]
    }
  }
}
```

После правки - перезапуск MCP / агента. В UI должны появиться tools ниже.

**Safety:** write-tools по умолчанию `dry_run=true`. Реальная запись только при `dry_run=false` после явного подтверждения в чате. Токены не в чат - в env-файлах.

## Env / secrets

Все креды и machine-specific layout - в [`config/`](config/README.md). Примеры коммитятся как `*.example*`; рабочие файлы в `.gitignore`.

```bash
cp config/tracker.env.example config/tracker.env
cp config/gitlab.env.example config/gitlab.env
chmod 600 config/tracker.env config/gitlab.env
# заполнить токены

cp config/workspace.example.yaml /path/to/workspace/.workspace.yaml
# относительные пути к репам + sandbox components
```

| Файл | Назначение |
|------|------------|
| `config/tracker.env` | Yandex Tracker API |
| `config/gitlab.env` | GitLab API |
| `<workspace>/.workspace.yaml` | layout реп + sandbox (discovery от cwd) |
| `config/workspace.example.yaml` | пример манифеста |
| `~/.local/state/mcp-tools/tracker-time.json` | локальный timer state (не в git) |

Минимум Tracker: `TRACKER_OAUTH_TOKEN`, `TRACKER_ORG_ID` или `TRACKER_CLOUD_ORG_ID`.  
Минимум GitLab: `GITLAB_BASE_URL`, `GITLAB_TOKEN`.

Креды - в `config/*.env`. Layout проектов - в `.workspace.yaml` у workspace (см. [config/README.md](config/README.md)).

## Tools

### `ping`

Health-check сервера.

### `start_project_sandbox`

Старт/стоп/статус песочницы проекта (`skills/start-project-sandbox`).

- **Зачем:** lifecycle sandbox (docker/tmux/процессы) с readiness.
- **Параметры:** `action` = `list|check|status|start|stop|diagnose`; `project` опционален (name/alias манифеста); `cwd` / `manifest`.
- **Конфиг:** `.workspace.yaml` (sandbox.components); discovery от `cwd` / `--manifest`.
- **Запуск CLI:** `python skills/start-project-sandbox/scripts/start_sandbox.py --status --cwd /path/to/workspace`

### `prepare_project_task`

Локальные task-ветки по всем репам проекта. **Никогда не push.**

- **Зачем:** подготовить ветку задачи от `master`/`main`.
- **Параметры:** `mode` = `list|check|prepare`; `task_id`; опционально `project`, `cwd`.
- **Конфиг:** `.workspace.yaml` (repositories + branches.base)
- **CLI (Go):** `make build-prepare-task` → `./bin/prepare-task --check IT-123`

### `prepare_project_merge_requests`

Publish веток + create/reuse GitLab MR.

- **Зачем:** открыть MR по задаче во всех репах.
- **Параметры:** `mode` = `list|check|create`; `task_id`; опционально `project`, `cwd`, `title`, `description`.
- **Конфиг:** `.workspace.yaml` + `config/gitlab.env`
- **CLI:** `python .../prepare_merge_requests.py --check IT-123`

### `deploy_project_task_to_dev`

Merge task → `dev` и push только `origin/dev`.

- **Зачем:** выкатить задачу на dev.
- **Параметры:** `mode` = `list|check|deploy|continue`; `task_id`; опционально `project`, `cwd`.
- **Конфиг:** `.workspace.yaml` (branches.dev)
- **CLI:** `python .../deploy_task_to_dev.py --check IT-123`

### `yandex_tracker_timekeeper`

Локальный таймер + worklog в Tracker.

- **Зачем:** учёт времени по ключу задачи (`IT-123`).
- **Параметры:** `action` = `start|status|stop|write|clear`; `issue_key`; `comment`; для `write`: `dry_run` (default true), `round_to`.
- **Env:** `.env` / `.env.local` для `write --send`; state-файл см. выше.
- **CLI (Go):** `make build-tracker-time` → `./bin/tracker-time status`

### `yandex_tracker_issue_creator`

Создание задачи Tracker.

- **Зачем:** завести issue с queue/summary/description/SP/assignee.
- **Параметры:** `queue`, `summary`, `description`; `dry_run=true` по умолчанию; теги/period/sp/assignee опционально.
- **Env:** `.env` / `.env.local` в корне mcp-tools
- **Важно:** реальный create только `dry_run=false` после подтверждения payload.
- **CLI (Go):** `make build-tracker-create-issue` → `./bin/tracker-create-issue`

### `yandex_tracker_comment` / `yandex_tracker_transition`

Комментарий и смена статуса в Tracker.

- **Зачем:** опубликовать комментарий / сделать transition.
- **Параметры:** `issue_key`, `body` / `target`; `dry_run=true` по умолчанию.
- **Env:** `.env` / `.env.local` в корне mcp-tools
- **CLI (Go):** `make build` → `bin/tracker-*`, `bin/gitlab-mr-comment`

### `gitlab_mr_comment`

MR note или line discussion в GitLab.

- **Зачем:** отправить комментарий в MR.
- **Параметры:** `mr_url`, `body`; `line_comment` + `file_path`/`line`/`line_type` для discussion; `dry_run=true` по умолчанию.
- **Env:** `.env` / `.env.local` (`GITLAB_TOKEN`)
- **CLI (Go):** `make build-gitlab-mr-comment` → `./bin/gitlab-mr-comment`

---

## Отложено (фаза 5)

- `gitlab-mr-reviewer` - оркестратор поверх GitLab MCP + codeStyle
- отдельный codestyle skill/MCP

См. [TODO.md](TODO.md).

## Детали workflow

Полные safety-правила и сценарии - в `skills/*/SKILL.md` (source of truth для агента при чтении репо). Поведение write-операций для всех хостов одинаковое: default dry-run в MCP layer.
