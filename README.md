# mcp-tools

Единый **stdio MCP-сервер** с детерминированными tools для агентов (Zed / Codex CLI / Cursor).  
Логика в Go CLI (`skills/*/cmd` → `bin/*`); MCP (`bin/mcp-server`) - thin wrapper. Отдельный daemon не нужен: хост сам стартует процесс по `command` + `args`.

```text
mcp-tools/
  cmd/mcp-server/    # точка входа MCP (stdio) → bin/mcp-server
  internal/          # общий Go-код
  skills/            # пакеты (SKILL.md + cmd + references)
  config/            # workspace.example.yaml + README
  bin/               # собранные CLI + mcp-server
  TODO.md
  go.mod
```

## Установка

```bash
git clone <repo-url> mcp-tools
cd mcp-tools
make build   # mcp-server + CLI (prepare/deploy/sandbox/tracker/gitlab/codestyle)
```

Нужен **Go** (≥ версии из `go.mod`).

Проверка MCP:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ping","arguments":{}}}' \
  | ./bin/mcp-server
```

### Зависимости хоста (sandbox и команды из манифеста)

`start-sandbox` **не ставит** Node/Docker/пакеты проекта. Он запускает `start_command` / `stop_command` из `.workspace.yaml` и проверяет, что первый бинарь есть в `PATH`. Что нужно на машине - зависит от твоих `sandbox.components`:

| Инструмент | Зачем |
|------------|--------|
| **tmux** | runtime `tmux`: сессии для long-running процессов (типа `npm run dev`) |
| **Docker** + Compose plugin | runtime `compose`: `docker compose up/down`, inspect образов |
| **npm** / **node** | если в манифесте `start_command: [npm, run, ...]`; сами `node_modules` ставишь в проекте (`npm i`), скилл этого не делает |
| любой другой бинарь из `start_command[0]` | то же правило: должен быть в `PATH` до `--start` |

Пример из [`config/workspace.example.yaml`](config/workspace.example.yaml): backend → docker compose; frontend → `npm run dev` в tmux + HTTP probe. Без `tmux`/`npm`/`docker` validate упадёт с «исполняемый файл недоступен».

Остальные tools (Tracker/GitLab/git) используют системный **git** и креды из `.env` / `.env.local` (см. ниже).

В конфигах агентов нужен **абсолютный** путь к клону (хосты обычно не резолвят `~` в `command`/`args`). Подставь свой:

```bash
REPO="$(pwd)"   # из корня клона, например /home/you/src/mcp-tools
echo "$REPO"
```

## Подключение MCP

Один и тот же `command`/`args` везде; ключ конфига разный. Ниже `<REPO>` = абсолютный путь к клону. Сначала `make build`.

### Cursor (`~/.cursor/mcp.json` или `.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "dev-tools": {
      "command": "<REPO>/bin/mcp-server"
    }
  }
}
```

### Codex CLI (`~/.codex/config.toml`)

```toml
[mcp_servers.dev-tools]
command = "<REPO>/bin/mcp-server"
```

### Zed (`settings.json`)

```json
{
  "context_servers": {
    "dev-tools": {
      "source": "custom",
      "command": "<REPO>/bin/mcp-server"
    }
  }
}
```

После правки - перезапуск MCP / агента. В UI должны появиться tools ниже.

**Safety:** write-tools по умолчанию `dry_run=true`. Реальная запись только при `dry_run=false` после явного подтверждения в чате. Токены не в чат - в `.env`.

## Env / secrets

Креды - в корневом [`.env.example`](.env.example) → `.env` / `.env.local`. Layout проектов - в `.workspace.yaml` у workspace (см. [config/README.md](config/README.md)).

```bash
cp .env.example .env
chmod 600 .env
# заполнить токены; overrides → .env.local

cp config/workspace.example.yaml /path/to/workspace/.workspace.yaml
# относительные пути к репам + sandbox components
```

| Файл | Назначение |
|------|------------|
| `.env` / `.env.local` | Tracker + GitLab API |
| `<workspace>/.workspace.yaml` | layout реп + sandbox (discovery от cwd) |
| `config/workspace.example.yaml` | пример манифеста |
| `~/.local/state/mcp-tools/tracker-time.json` | локальный timer state (не в git) |

Минимум Tracker: `TRACKER_OAUTH_TOKEN`, `TRACKER_ORG_ID` или `TRACKER_CLOUD_ORG_ID`.  
Минимум GitLab: `GITLAB_BASE_URL`, `GITLAB_TOKEN`.

## Tools

### `ping`

Health-check сервера.

### `start_project_sandbox`

Старт/стоп/статус песочницы проекта (`skills/start-project-sandbox`).

- **Зачем:** lifecycle sandbox (docker/tmux) с readiness.
- **Параметры:** `action` = `list|check|status|start|stop|diagnose`; `project` опционален (name/alias манифеста); `cwd` / `manifest`.
- **Конфиг:** `.workspace.yaml` (sandbox.components); discovery от `cwd` / `--manifest`.
- **CLI (Go):** `make build-start-sandbox` → `./bin/start-sandbox --status`

### `prepare_project_task`

Локальные task-ветки по всем репам проекта. **Никогда не push.**

- **Зачем:** подготовить ветку задачи от `master`/`main`.
- **Параметры:** `mode` = `list|check|prepare`; `task_id`; опционально `project`, `cwd`.
- **Конфиг:** `.workspace.yaml` (repositories + branches.base)
- **CLI (Go):** `make build-prepare-task` → `./bin/prepare-task --check IT-123`
- **Ветки:** work base только `master`/`main` (зашито в `AllowedWorkBases`); task-id не может быть `master`/`main`/`dev`.

### `prepare_project_merge_requests`

Publish веток + create/reuse GitLab MR.

- **Зачем:** открыть MR по задаче во всех репах.
- **Параметры:** `mode` = `list|check|create`; `task_id`; опционально `project`, `cwd`, `title`, `description`.
- **Конфиг:** `.workspace.yaml` + корневой `.env` / `.env.local` (`GITLAB_TOKEN`, `GITLAB_BASE_URL`)
- **CLI (Go):** `make build-prepare-mr` → `./bin/prepare-mr --check IT-123`
- **Ветки:** MR target только `master`/`main`; push task только через `PushTaskBranch`.

### `deploy_project_task_to_dev`

Merge task → `dev` и push только `origin/dev`.

- **Зачем:** выкатить задачу на dev.
- **Параметры:** `mode` = `list|check|deploy|continue`; `task_id`; опционально `project`, `cwd`.
- **Конфиг:** `.workspace.yaml` (branches.dev)
- **CLI (Go):** `make build-deploy-task` → `./bin/deploy-task --check IT-123`
- **Ветки:** `branches.dev` обязан быть литералом `dev`; push только `refs/heads/dev:refs/heads/dev` (`PushDevBranch`).

**Имена веток жёстко зашиты в коде** (не только defaults манифеста): `master`/`main` как work bases и MR targets, `dev` как единственная deploy/push-ветка, запрет task-id ∈ `{master,main,dev}`. См. `internal/workspace.AllowedWorkBases`, `internal/gitcmd.ForbiddenTaskBranches`, `PushDevBranch`. Манифест может указать `branches.*`, но значения вне этих литералов отклоняются.

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
- **CLI (Go):** `make build` → `bin/tracker-*`, `bin/gitlab-mr-comment`, `bin/gitlab-mr-context`

### `gitlab_mr_comment`

MR note или line discussion в GitLab.

- **Зачем:** отправить комментарий в MR.
- **Параметры:** `mr_url`, `body`; `line_comment` + `file_path`/`line`/`line_type` для discussion; `dry_run=true` по умолчанию.
- **Env:** `.env` / `.env.local` (`GITLAB_TOKEN`)
- **CLI (Go):** `make build-gitlab-mr-comment` → `./bin/gitlab-mr-comment`

### `gitlab_mr_context`

JSON-контекст MR для skill `gitlab-mr-reviewer` (metadata, diffs, commits, discussions/notes). Ревью (SKIP/FINDINGS) делает агент по SKILL, не этот tool.

- **Зачем:** один bundle фактов перед ревью.
- **Параметры:** `mr_url`; опционально `author_filter` (CSV), `parse_only`, `env_dir`.
- **Env:** `.env` / `.env.local` (`GITLAB_TOKEN`, опционально `GITLAB_MR_AUTHOR_FILTER`)
- **CLI (Go):** `make build-gitlab-mr-context` → `./bin/gitlab-mr-context --mr-url URL`

### `codestyle_find_rules` / `codestyle_read_rule`

Правила кодстайла для стека PHP/Symfony + Vue/TS (анонимизированный snapshot в `skills/codestyle/content`).

- **Зачем:** перед кодом на этом стеке найти и прочитать релевантные MUST-правила (не «по памяти»).
- **Параметры:** `task` (+ `max_results`); затем `uri` вида `codestyle://backend/dto`.
- **CLI:** `make build-codestyle` → `./bin/codestyle find-rules --task "..."` / `read-rule --uri ...`
- **Правила:** править напрямую `skills/codestyle/content/` (`.codestyle/` + `mcp-meta.json`)

### Как добавить правила кодстайла

SoT: `skills/codestyle/content/`.

1. Markdown в категорию:
   - `skills/codestyle/content/.codestyle/general/`
   - `skills/codestyle/content/.codestyle/backend/`
   - `skills/codestyle/content/.codestyle/frontend/`
   - `skills/codestyle/content/.codestyle/patterns/`

   Пример: `.codestyle/backend/events.md` (MUST/MUST NOT/SHOULD/MAY + примеры).

2. Запись в `skills/codestyle/content/mcp-meta.json` (иначе `find_rules` не найдёт):

```json
{
  "file": "events.md",
  "category": "backend",
  "tags": ["events", "dispatcher"],
  "tasks": ["событие Symfony", "event listener"],
  "priority": "critical",
  "summary": "Кратко: что MUST соблюдать."
}
```

URI: `codestyle://backend/events`.

3. Проверка:

```bash
make build-codestyle
./bin/codestyle find-rules --task "событие Symfony listener"
./bin/codestyle read-rule --uri "codestyle://backend/events"
```

Опционально ссылку в `.codestyle/<category>/INDEX.md`. Workflow агента: `skills/codestyle/SKILL.md`.

---

## TODO

См. также полный список в [TODO.md](TODO.md).

- [ ] Вынести жёстко зашитые имена веток (`master`/`main`/`dev`, ForbiddenTaskBranches, AllowedWorkBases, литерал push `dev`) в конфиг / `.workspace.yaml`, чтобы не править код под другой branching model

## Детали workflow

Полные safety-правила и сценарии - в `skills/*/SKILL.md` (source of truth для агента при чтении репо). Поведение write-операций для всех хостов одинаковое: default dry-run в MCP layer.
