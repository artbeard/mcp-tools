# Local configuration (not committed)

## Credentials

```bash
cp config/tracker.env.example config/tracker.env
cp config/gitlab.env.example config/gitlab.env
chmod 600 config/tracker.env config/gitlab.env
# fill tokens
```

| File | Used by |
|------|---------|
| `tracker.env` | Tracker tools |
| `gitlab.env` | GitLab MR commenter + prepare MRs |

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
