# mcp-tools

Tools for AI agents (Cursor / Codex CLI / Zed) over one local **MCP** server: sandboxes, git branches, GitLab MRs, Yandex Tracker, codestyle lookup, MR review context.

Logic lives in Go CLIs (`bin/*`). `bin/mcp-server` is a thin stdio wrapper — no long-running daemon.

---

## English

### What this is

A single local MCP process that exposes deterministic tools so an agent can prepare task branches, open MRs, deploy to `dev`, talk to Tracker/GitLab, and look up project coding rules — without ad-hoc shell scripts in chat.

### Install

**Required to build/run mcp-tools**

| Dependency | Why |
|------------|-----|
| **Go** ≥ version in `go.mod` | `make build` |
| **git** | branch / MR / deploy tools |

**Optional (only if used in `.workspace.yaml` sandbox)**

| Dependency | Why |
|------------|-----|
| **tmux** | long-running `tmux` components |
| **Docker** + Compose plugin | `compose` components |
| **npm** / **node** | if `start_command` uses them (`npm i` you do in the project) |

```bash
git clone <repo-url> mcp-tools
cd mcp-tools
make build
```

Copy secrets and (for multi-repo workflows) a workspace manifest:

```bash
cp .env.example .env
cp config/workspace.example.yaml /path/to/your/workspace/.workspace.yaml
```

### Use

1. Register MCP in the agent config with the **absolute** path to `bin/mcp-server` (Cursor `~/.cursor/mcp.json`, Codex `~/.codex/config.toml`, Zed `settings.json`), then restart MCP / the agent.
2. In chat, ask in plain language — e.g. “prepare branches for IT-123”, “open MRs”, “comment on this MR”, “find codestyle for a Symfony DTO”. The agent calls the MCP tools; you confirm before any real write (`dry_run=false`).
3. Optional: run the same programs from a terminal without an agent (`./bin/prepare-task --check IT-123`, etc.).

### Disclaimer

These tools can change git state, remote repositories, Tracker issues, and GitLab MRs. You use them at your own risk. By using this software you confirm that you understand what it does and accept full responsibility for the consequences of incorrect, careless, or unintended use. The author is not liable for any damage, data loss, or other outcomes arising from such use.

---

## Русский

### Что это

Локальный MCP-сервер и набор Go-программ для AI-агентов: песочницы, ветки задач, merge request в GitLab, Yandex Tracker, правила кодстайла, контекст MR для ревью. Один процесс в редакторе/CLI — без отдельного демона.

### Установка

**Обязательно для сборки и работы mcp-tools**

| Зависимость | Зачем |
|-------------|--------|
| **Go** ≥ версии из `go.mod` | `make build` |
| **git** | tools веток / MR / deploy |

**Опционально (только если есть в sandbox `.workspace.yaml`)**

| Зависимость | Зачем |
|-------------|--------|
| **tmux** | долгие процессы в режиме `tmux` |
| **Docker** + Compose plugin | компоненты `compose` |
| **npm** / **node** | если `start_command` их вызывает (`npm i` в проекте сам) |

```bash
git clone <repo-url> mcp-tools
cd mcp-tools
make build
```

Секреты и (для мультирепо) манифест workspace:

```bash
cp .env.example .env
cp config/workspace.example.yaml /path/to/your/workspace/.workspace.yaml
```

### Использование

1. Пропиши MCP в конфиге агента **абсолютным** путём к `bin/mcp-server` (Cursor `~/.cursor/mcp.json`, Codex `~/.codex/config.toml`, Zed `settings.json`) и перезапусти MCP / агента. Примеры конфигов: [README_RU.md](README_RU.md).
2. В чате пиши обычным языком — «подготовь ветки IT-123», «открой MR», «прокомментируй MR», «найди правила для Symfony DTO». Агент сам вызовет MCP tools; перед реальной записью подтверди (`dry_run=false`).
3. Без агента те же программы можно гонять из терминала (`./bin/prepare-task --check IT-123` и т.д.). Полный список tools: [README_RU.md](README_RU.md).

### Отказ от ответственности

Эти инструменты могут изменять состояние git, удалённые репозитории, задачи Tracker и merge request в GitLab. Вы используете их на свой страх и риск. Используя это ПО, вы подтверждаете, что понимаете его действие и несёте полную ответственность за последствия неверного, неосторожного или непреднамеренного применения. Автор не отвечает за ущерб, потерю данных и иные последствия такого использования.
