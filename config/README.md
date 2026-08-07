# Local configuration (not committed)

Copy examples and fill secrets / absolute paths:

```bash
cp config/tracker.env.example config/tracker.env
cp config/gitlab.env.example config/gitlab.env
cp config/projects-prepare.example.json config/projects-prepare.json
cp config/projects-mr.example.json config/projects-mr.json
cp config/projects-deploy.example.json config/projects-deploy.json
cp config/sandboxes.example.json config/sandboxes.json
chmod 600 config/tracker.env config/gitlab.env
```

| File | Used by |
|------|---------|
| `tracker.env` | Tracker tools (timekeeper, issue creator, commenter) |
| `gitlab.env` | GitLab MR commenter + prepare MRs |
| `projects-prepare.json` | `prepare-project-task` |
| `projects-mr.json` | `prepare-project-merge-requests` |
| `projects-deploy.json` | `deploy-project-task-to-dev` |
| `sandboxes.json` | `start-project-sandbox` |

`gitlab_env_file` in `projects-mr.json` may be `config/gitlab.env` (relative to repo root) or an absolute path.
