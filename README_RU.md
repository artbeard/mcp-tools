# mcp-tools (подробно)

Набор инструментов для AI-агентов (Cursor, Codex CLI, Zed).  
Агент подключается к одному процессу по протоколу MCP и получает готовые команды: поднять песочницу, подготовить ветки, открыть merge request, выкатить на `dev`, работать с Tracker/GitLab, искать правила кодстайла, собрать контекст MR для ревью.

Логика живёт в обычных Go-программах (`skills/*/cmd` → `bin/*`).  
Файл `bin/mcp-server` только принимает вызов от агента и запускает нужную программу. Отдельный фоновый сервис не нужен: редактор/CLI сам стартует процесс.

```text
mcp-tools/
  cmd/mcp-server/    # вход MCP → bin/mcp-server
  internal/          # общий Go-код
  skills/            # пакеты: SKILL.md + cmd + references
  config/            # пример .workspace.yaml + краткий README
  bin/               # собранные программы (после make build)
  go.mod
```

---

## Установка

Нужен **Go** (версия не ниже, чем в `go.mod`).

```bash
git clone <repo-url> mcp-tools
cd mcp-tools
make build
```

`make build` собирает `bin/mcp-server` и все CLI (sandbox, prepare, deploy, Tracker, GitLab, codestyle).

Проверка, что MCP отвечает:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ping","arguments":{}}}' \
  | ./bin/mcp-server
```

### Что ещё нужно на машине

`start-sandbox` **не ставит** Node, Docker и зависимости проектов. Он только запускает команды из `.workspace.yaml` и проверяет, что первый бинарь из команды есть в `PATH`. Набор зависит от твоих `sandbox.components`:

| Инструмент | Зачем |
|------------|--------|
| **tmux** | режим `tmux`: сессии для долгих процессов (например `npm run dev`) |
| **Docker** + Compose plugin | режим `compose`: `docker compose up/down`, проверка образов |
| **npm** / **node** | если в манифесте `start_command: [npm, run, ...]`; `node_modules` ставишь сам в проекте (`npm i`) |
| любой другой бинарь из `start_command[0]` | должен быть в `PATH` до `--start` |

Пример: [`config/workspace.example.yaml`](config/workspace.example.yaml) - backend через docker compose, frontend через `npm run dev` в tmux. Без `tmux` / `npm` / `docker` проверка манифеста упадёт с «исполняемый файл недоступен».

Остальные tools используют системный **git** и токены из `.env` / `.env.local` (см. ниже).

В конфигах агентов нужен **абсолютный** путь к клону (часто `~` в `command` не раскрывается):

```bash
REPO="$(pwd)"   # из корня клона, например /home/you/src/mcp-tools
echo "$REPO"
```

---

## Подключение к агенту

Один и тот же `command` везде; отличается только формат файла. `<REPO>` = абсолютный путь к клону. Сначала `make build`.

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

После правки перезапусти MCP / агента. В UI должны появиться tools из раздела ниже.

**Безопасность:** tools, которые что-то пишут, по умолчанию работают в режиме проверки (`dry_run=true`). Реальная запись только при `dry_run=false` после явного подтверждения в чате. Токены не в чат - в `.env`.

---

## Секреты и конфиг проектов

Токены: скопируй [`.env.example`](.env.example) → `.env` / `.env.local`.  
Список репозиториев и песочниц: файл `.workspace.yaml` рядом с твоими проектами (см. [config/README.md](config/README.md)).

```bash
cp .env.example .env
chmod 600 .env
# заполни токены; локальные переопределения → .env.local

cp config/workspace.example.yaml /path/to/workspace/.workspace.yaml
# относительные пути к репам + компоненты sandbox
```

| Файл | Назначение |
|------|------------|
| `.env` / `.env.local` | API Tracker и GitLab |
| `<workspace>/.workspace.yaml` | layout реп + sandbox (поиск вверх от рабочей директории) |
| `config/workspace.example.yaml` | пример манифеста |
| `~/.local/state/mcp-tools/tracker-time.json` | состояние локального таймера (не в git) |

Минимум Tracker: `TRACKER_OAUTH_TOKEN`, плюс `TRACKER_ORG_ID` или `TRACKER_CLOUD_ORG_ID`.  
Минимум GitLab: `GITLAB_BASE_URL`, `GITLAB_TOKEN`.

---

## Tools

### `ping`

Проверка, что сервер жив.

### `excalidraw_create`

Создаёт новый редактируемый файл `.excalidraw` из `diagram.nodes` и необязательных `diagram.connections` (`skills/excalidraw-diagrams`). Дефолтные стили и раскладка детерминированные; каждая объявленная связь создаётся как стрелка с bindings к обоим узлам, поэтому следует за ними при перемещении в Excalidraw. Не объявляй `connections`, если связи на схеме нет. Существующий `output` не перезаписывается.

### `start_project_sandbox`

Старт / стоп / статус песочницы проекта (`skills/start-project-sandbox`).

- **Зачем:** поднять или остановить docker/tmux-компоненты и дождаться готовности.
- **Параметры:** `action` = `list|check|status|start|stop|diagnose`; `project` опционален (имя/alias из манифеста); `cwd` / `manifest`.
- **Конфиг:** `.workspace.yaml` (`sandbox.components`); поиск от `cwd` / `--manifest`.
- **CLI:** `make build-start-sandbox` → `./bin/start-sandbox --status`

### `prepare_project_task`

Локальные ветки задачи во всех репах проекта. **Никогда не делает push.**

- **Зачем:** подготовить ветку задачи от `master`/`main`.
- **Параметры:** `mode` = `list|check|prepare`; `task_id`; опционально `project`, `cwd`.
- **Конфиг:** `.workspace.yaml` (repositories + branches.base)
- **CLI:** `make build-prepare-task` → `./bin/prepare-task --check IT-123`
- **Ветки:** база только `master`/`main` (зашито в `AllowedWorkBases`); id задачи не может быть `master`/`main`/`dev`.

### `prepare_project_merge_requests`

Публикация веток и создание (или повторное использование) GitLab MR.

- **Зачем:** открыть MR по задаче во всех репах.
- **Параметры:** `mode` = `list|check|create`; `task_id`; опционально `project`, `cwd`, `title`, `description`.
- **Конфиг:** `.workspace.yaml` + корневой `.env` / `.env.local` (`GITLAB_TOKEN`, `GITLAB_BASE_URL`)
- **CLI:** `make build-prepare-mr` → `./bin/prepare-mr --check IT-123`
- **Ветки:** цель MR только `master`/`main`; push task-ветки только через `PushTaskBranch`.

### `deploy_project_task_to_dev`

Слияние task → `dev` и push только `origin/dev`.

- **Зачем:** выкатить задачу на dev.
- **Параметры:** `mode` = `list|check|deploy|continue`; `task_id`; опционально `project`, `cwd`.
- **Конфиг:** `.workspace.yaml` (branches.dev)
- **CLI:** `make build-deploy-task` → `./bin/deploy-task --check IT-123`
- **Ветки:** `branches.dev` обязан быть литералом `dev`; push только `refs/heads/dev:refs/heads/dev` (`PushDevBranch`).

**Имена веток жёстко зашиты в коде** (не только значения по умолчанию в манифесте): `master`/`main` как рабочие базы и цели MR, `dev` как единственная ветка для deploy/push, запрет task-id из `{master,main,dev}`. См. `internal/workspace.AllowedWorkBases`, `internal/gitcmd.ForbiddenTaskBranches`, `PushDevBranch`. Манифест может указать `branches.*`, но значения вне этих литералов отклоняются.

### `yandex_tracker_timekeeper`

Локальный таймер и worklog в Tracker.

- **Зачем:** учёт времени по ключу задачи (`IT-123`).
- **Параметры:** `action` = `start|status|stop|write|clear`; `issue_key`; `comment`; для `write`: `dry_run` (по умолчанию true), `round_to`.
- **Env:** `.env` / `.env.local` для `write --send`; файл состояния см. выше.
- **CLI:** `make build-tracker-time` → `./bin/tracker-time status`

### `yandex_tracker_issue_creator`

Создание задачи в Tracker.

- **Зачем:** завести issue с очередью, summary, description, SP, исполнителем.
- **Параметры:** `queue`, `summary`, `description`; `dry_run=true` по умолчанию; теги / period / sp / assignee опционально.
- **Env:** `.env` / `.env.local` в корне mcp-tools
- **Важно:** реальный create только при `dry_run=false` после подтверждения данных.
- **CLI:** `make build-tracker-create-issue` → `./bin/tracker-create-issue`

### `yandex_tracker_comment` / `yandex_tracker_transition`

Комментарий и смена статуса в Tracker.

- **Зачем:** опубликовать комментарий или перевести задачу.
- **Параметры:** `issue_key`, `body` / `target`; `dry_run=true` по умолчанию.
- **Env:** `.env` / `.env.local` в корне mcp-tools
- **CLI:** `make build` → `bin/tracker-*`

### `gitlab_mr_comment`

Комментарий к MR в GitLab (к MR целиком или к строке в diff).

- **Зачем:** отправить комментарий в MR.
- **Параметры:** `mr_url`, `body`; для комментария к строке: `line_comment` + `file_path` / `line` / `line_type`; `dry_run=true` по умолчанию.
- **Env:** `.env` / `.env.local` (`GITLAB_TOKEN`)
- **CLI:** `make build-gitlab-mr-comment` → `./bin/gitlab-mr-comment`

### `gitlab_mr_context`

Данные MR в JSON для skill `gitlab-mr-reviewer` (метаданные, diffs, commits, discussions/notes). Само ревью (SKIP/FINDINGS) делает агент по SKILL, не этот tool.

- **Зачем:** одним вызовом получить факты перед ревью.
- **Параметры:** `mr_url`; опционально `author_filter` (список через запятую), `parse_only`, `env_dir`.
- **Env:** `.env` / `.env.local` (`GITLAB_TOKEN`, опционально `GITLAB_MR_AUTHOR_FILTER`)
- **CLI:** `make build-gitlab-mr-context` → `./bin/gitlab-mr-context --mr-url URL`

### `codestyle_find_rules` / `codestyle_read_rule`

Правила кодстайла для стека PHP/Symfony + Vue/TypeScript (снимок в `skills/codestyle/content`).

- **Зачем:** перед кодом на этом стеке найти и прочитать обязательные правила (не «по памяти»).
- **Параметры:** `task` (+ `max_results`); затем `uri` вида `codestyle://backend/dto`.
- **CLI:** `make build-codestyle` → `./bin/codestyle find-rules --task "..."` / `read-rule --uri ...`
- **Правила:** правятся напрямую в `skills/codestyle/content/` (`.codestyle/` + `mcp-meta.json`)

---

## Как добавить правила кодстайла

Каталог-источник: `skills/codestyle/content/`.

1. Markdown-файл в категорию:
   - `skills/codestyle/content/.codestyle/general/`
   - `skills/codestyle/content/.codestyle/backend/`
   - `skills/codestyle/content/.codestyle/frontend/`
   - `skills/codestyle/content/.codestyle/patterns/`

   Пример: `.codestyle/backend/events.md` (MUST / MUST NOT / SHOULD / MAY + примеры).

2. Запись в `skills/codestyle/content/mcp-meta.json` (иначе поиск правило не найдёт):

```json
{
  "file": "events.md",
  "category": "backend",
  "tags": ["events", "dispatcher"],
  "tasks": ["событие Symfony", "event listener"],
  "priority": "critical",
  "summary": "Кратко: что обязательно соблюдать."
}
```

URI получится: `codestyle://backend/events`.

3. Проверка:

```bash
make build-codestyle
./bin/codestyle find-rules --task "событие Symfony listener"
./bin/codestyle read-rule --uri "codestyle://backend/events"
```

По желанию добавь ссылку в `.codestyle/<category>/INDEX.md`. Сценарий для агента: `skills/codestyle/SKILL.md`.

---

## TODO

- [ ] Вынести жёстко зашитые имена веток (`master`/`main`/`dev`, ForbiddenTaskBranches, AllowedWorkBases, литерал push `dev`) в конфиг / `.workspace.yaml`, чтобы не править код под другую схему веток

---

## Детали сценариев

Полные правила безопасности и пошаговые сценарии - в `skills/*/SKILL.md` (это то, что агент читает в репозитории). Запись через MCP везде одинаковая: по умолчанию только проверка (`dry_run`), без явного подтверждения ничего не меняется снаружи.
