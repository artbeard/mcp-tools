# mcp-tools

Единый **stdio MCP-сервер** с детерминированными tools для агентов (Zed / Codex CLI / Cursor).  
Логика живёт в `skills/*/scripts`; MCP — thin wrapper. Отдельный daemon не нужен: хост сам стартует процесс по `command` + `args`.

```text
mcp-tools/
  server.py          # точка входа MCP (stdio)
  skills/            # пакеты (SKILL.md + scripts + references)
  TODO.md            # статус переноса
  requirements.txt
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

После правки — перезапуск MCP / агента. В UI должны появиться tools ниже.

**Safety:** write-tools по умолчанию `dry_run=true`. Реальная запись только при `dry_run=false` после явного подтверждения в чате. Токены не в чат — в env-файлах.

## Env / secrets

Все креды и machine-specific layout — в [`config/`](config/README.md). Примеры коммитятся как `*.example*`; рабочие файлы в `.gitignore`.

```bash
cp config/tracker.env.example config/tracker.env
cp config/gitlab.env.example config/gitlab.env
cp config/projects-prepare.example.json config/projects-prepare.json
cp config/projects-mr.example.json config/projects-mr.json
cp config/projects-deploy.example.json config/projects-deploy.json
cp config/sandboxes.example.json config/sandboxes.json
chmod 600 config/tracker.env config/gitlab.env
# затем заполни токены и абсолютные пути к своим репам
```

| Файл | Назначение |
|------|------------|
| `config/tracker.env` | Yandex Tracker API |
| `config/gitlab.env` | GitLab API |
| `config/projects-*.json` | multi-repo layout для prepare/MR/deploy |
| `config/sandboxes.json` | sandbox components |
| `~/.local/state/mcp-tools/tracker-time.json` | локальный timer state (не в git) |

Минимум Tracker: `TRACKER_OAUTH_TOKEN`, `TRACKER_ORG_ID` или `TRACKER_CLOUD_ORG_ID`.  
Минимум GitLab: `GITLAB_BASE_URL`, `GITLAB_TOKEN`.

Скрипты и MCP читают **только** `config/` (или override через `env_file` / `*_CONFIG` env vars).
---

## Tools

### `ping`

Health-check сервера.

### `start_project_sandbox`

Старт/стоп/статус песочницы проекта (`skills/start-project-sandbox`).

- **Зачем:** lifecycle sandbox (docker/tmux/процессы) с readiness.
- **Параметры:** `action` = `list|check|status|start|stop|diagnose`; `project` (кроме `list`); опционально `cwd`.
- **Конфиг:** `config/sandboxes.json` (пример: `sandboxes.example.json`; override `SANDBOX_CONFIG_FILE`).
- **Запуск CLI:** `python skills/start-project-sandbox/scripts/start_sandbox.py --status <project>`

### `prepare_project_task`

Локальные task-ветки по всем репам проекта. **Никогда не push.**

- **Зачем:** подготовить ветку задачи от `master`/`main`.
- **Параметры:** `mode` = `list|check|prepare`; `task_id`; опционально `project`, `cwd`.
- **Конфиг:** `config/projects-prepare.json`
- **CLI:** `python .../prepare_task_branch.py --check IT-123`

### `prepare_project_merge_requests`

Publish веток + create/reuse GitLab MR.

- **Зачем:** открыть MR по задаче во всех репах.
- **Параметры:** `mode` = `list|check|create`; `task_id`; опционально `project`, `cwd`, `title`, `description`.
- **Конфиг:** `config/projects-mr.json` + `config/gitlab.env`
- **CLI:** `python .../prepare_merge_requests.py --check IT-123`

### `deploy_project_task_to_dev`

Merge task → `dev` и push только `origin/dev`.

- **Зачем:** выкатить задачу на dev.
- **Параметры:** `mode` = `list|check|deploy|continue`; `task_id`; опционально `project`, `cwd`.
- **Конфиг:** `config/projects-deploy.json`
- **CLI:** `python .../deploy_task_to_dev.py --check IT-123`

### `yandex_tracker_timekeeper`

Локальный таймер + worklog в Tracker.

- **Зачем:** учёт времени по ключу задачи (`IT-123`).
- **Параметры:** `action` = `start|status|stop|write|clear`; `issue_key`; `comment`; для `write`: `dry_run` (default true), `round_to`.
- **Env:** `tracker.env`; state-файл см. выше.
- **CLI:** `python skills/yandex-tracker-timekeeper/scripts/tracker_time.py status`

### `yandex_tracker_issue_creator`

Создание задачи Tracker.

- **Зачем:** завести issue с queue/summary/description/SP/assignee.
- **Параметры:** `queue`, `summary`, `description`; `dry_run=true` по умолчанию; теги/period/sp/assignee опционально.
- **Env:** `config/tracker.env`
- **Важно:** реальный create только `dry_run=false` после подтверждения payload.

### `yandex_tracker_comment` / `yandex_tracker_transition`

Комментарий и смена статуса в Tracker.

- **Зачем:** опубликовать комментарий / сделать transition.
- **Параметры:** `issue_key`, `body` / `target`; `dry_run=true` по умолчанию.
- **Env:** `tracker.env`

### `gitlab_mr_comment`

MR note или line discussion в GitLab.

- **Зачем:** отправить комментарий в MR.
- **Параметры:** `mr_url`, `body`; `line_comment` + `file_path`/`line`/`line_type` для discussion; `dry_run=true` по умолчанию.
- **Env:** `config/gitlab.env`

---

## Отложено (фаза 5)

- `gitlab-mr-reviewer` — оркестратор поверх GitLab MCP + codeStyle
- отдельный codestyle skill/MCP

См. [TODO.md](TODO.md).

## Детали workflow

Полные safety-правила и сценарии — в `skills/*/SKILL.md` (source of truth для агента при чтении репо). Поведение write-операций для всех хостов одинаковое: default dry-run в MCP layer.
