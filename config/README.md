# Local configuration (not committed)

## Credentials (Go: `.env` + `.env.local`)

Go-тулы грузят из корня репо: обязательный `.env`, опциональный `.env.local` (перекрывает ключи).

```bash
cp .env.example .env
chmod 600 .env
# fill tokens; overrides → .env.local
```

## Credentials (Python skills: `config/*.env`)

Пока Python-скрипты ещё читают отдельные файлы:

```bash
cp config/tracker.env.example config/tracker.env
cp config/gitlab.env.example config/gitlab.env
chmod 600 config/tracker.env config/gitlab.env
# fill tokens
```

| File | Used by |
|------|---------|
| `.env` / `.env.local` | Go CLI (transition, comment, create-issue, time) |
| `config/tracker.env` | (legacy) Python Tracker tools, если ещё остались |
| `config/gitlab.env` | Python GitLab tools |

## Workspace layout (`.workspace.yaml`)

Project/sandbox layout lives **in each workspace**, not in mcp-tools:

```bash
cp config/workspace.example.yaml /path/to/your/workspace/.workspace.yaml
# edit relative paths / sandbox components
```

Discovery: tools walk up from `--cwd` until they find `.workspace.yaml`.

Migrate from legacy central JSON (if you still have local `config/projects-*.json` + `sandboxes.json`):

```bash
.venv/bin/python scripts/migrate_central_config_to_workspace.py          # dry-run
.venv/bin/python scripts/migrate_central_config_to_workspace.py --write   # create files
```

Legacy `config/projects-*.json` / `sandboxes.json` are gitignored and only used by the migrator.
