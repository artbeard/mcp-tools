# mcp-tools

Tools for AI agents (Cursor / Codex CLI / Zed) over one local **MCP** server: sandboxes, git branches, GitLab MRs, Yandex Tracker, codestyle lookup, MR review context.

Logic lives in Go CLIs (`bin/*`). `bin/mcp-server` is a thin stdio wrapper — no long-running daemon.

---

## English

### What this is

A single local MCP process that exposes deterministic tools so an agent can prepare task branches, open MRs, deploy to `dev`, talk to Tracker/GitLab, and look up project coding rules — without ad-hoc shell scripts in chat.

### Install

Needs **Go** (see `go.mod`).

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

Point the agent at the **absolute** path of `bin/mcp-server` (Cursor `mcp.json`, Codex `config.toml`, or Zed `settings.json`). Details: [README_RU.md](README_RU.md) (Russian, full).

### Use

- Call tools from the agent UI after MCP is connected (`ping` first).
- Or run CLIs directly: `./bin/prepare-task --check IT-123`, `./bin/gitlab-mr-context --mr-url …`, etc.
- Write operations default to dry-run; real writes need explicit confirmation (`dry_run=false`).
- Full tool list, branch rules, and how to add codestyle rules: [README_RU.md](README_RU.md).

---

## Русский

### Что это

Локальный MCP-сервер и набор Go-программ для AI-агентов: песочницы, ветки задач, merge request в GitLab, Yandex Tracker, правила кодстайла, контекст MR для ревью. Один процесс в редакторе/CLI — без отдельного демона.

### Установка

Нужен **Go** (версия в `go.mod`).

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

В агенте укажи **абсолютный** путь к `bin/mcp-server`. Полная инструкция: [README_RU.md](README_RU.md).

### Использование

- После подключения MCP вызывай tools из UI агента (сначала `ping`).
- Или CLI напрямую: `./bin/prepare-task --check IT-123`, `./bin/gitlab-mr-context --mr-url …` и т.д.
- Операции записи по умолчанию в режиме проверки; реальная запись только после явного подтверждения.
- Список tools, правила веток и как добавлять правила кодстайла: [README_RU.md](README_RU.md).
