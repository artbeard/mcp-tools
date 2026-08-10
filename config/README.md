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
| `config/tracker.env` / `config/gitlab.env` | legacy leftovers (игнорируются git); не используются кодом |

## Workspace layout (`.workspace.yaml`)

Project/sandbox layout lives **in each workspace**, not in mcp-tools:

```bash
cp config/workspace.example.yaml /path/to/your/workspace/.workspace.yaml
# edit relative paths / sandbox components
```

Discovery: tools walk up from `--cwd` until they find `.workspace.yaml`.
