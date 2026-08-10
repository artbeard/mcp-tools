# Local configuration (not committed)

## Credentials (`.env` + `.env.local`)

Все Go-тулы и MCP грузят креды из корня репо: обязательный `.env`, опциональный `.env.local` (перекрывает ключи).

```bash
cp .env.example .env
chmod 600 .env
# fill tokens; overrides → .env.local
```

| File | Used by |
|------|---------|
| `.env` / `.env.local` | Tracker + GitLab CLI / MCP |

## Workspace layout (`.workspace.yaml`)

Layout проектов живёт **в каждом workspace**, не в mcp-tools:

```bash
cp config/workspace.example.yaml /path/to/your/workspace/.workspace.yaml
# edit relative paths / sandbox components
```

Discovery: tools ищут `.workspace.yaml` вверх от `--cwd`.
