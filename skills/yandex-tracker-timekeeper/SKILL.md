---
name: yandex-tracker-timekeeper
description: Automatically track local work time for Yandex Tracker issues during task work, starting and stopping a local timer for issue keys like PROJ-123 and preparing or writing Tracker worklogs with dry-run safety.
---
# Yandex Tracker Timekeeper

Use this skill whenever the user asks you to work on, investigate, implement, review, comment on, transition, or otherwise handle a Tracker issue key like `PROJ-123`.

Goal: time accounting should happen without the user having to explicitly say "start timer". The user can still override duration, stop tracking, or ask not to write worklog.

## Tool

```bash
python3 skills/yandex-tracker-timekeeper/scripts/tracker_time.py ...
```

Local timer state:

```text
~/.local/state/mcp-tools/tracker-time.json
```

If that file does not exist, the script may migrate from `/tmp/.tracker-time.json`, then writes to the persistent path above. The state file must not store tokens.

Tracker auth is read from:

```text
config/tracker.env
```

## Automatic Workflow

Local timer operations (`status`, `start`, `stop`, dry-run `write`) may run without asking for approval. Real worklog write requires explicit confirmation and `dry_run=false` / `--send --confirm-send`.

1. On substantive work for an issue key:

```bash
python3 skills/yandex-tracker-timekeeper/scripts/tracker_time.py start <ISSUE_KEY> --comment "<short work description>"
```

2. If another timer is active for a different issue, stop it first when the switch is clear; otherwise report and ask.

3. Status:

```bash
python3 skills/yandex-tracker-timekeeper/scripts/tracker_time.py status
```

4. Stop:

```bash
python3 skills/yandex-tracker-timekeeper/scripts/tracker_time.py stop --comment "<what was done>"
```

5. Dry-run write:

```bash
python3 skills/yandex-tracker-timekeeper/scripts/tracker_time.py write --issue-key <ISSUE_KEY> --round-to 1
```

6. After user confirms exact duration/comment, send:

```bash
python3 skills/yandex-tracker-timekeeper/scripts/tracker_time.py write --issue-key <ISSUE_KEY> --round-to 1 --send --confirm-send
```
