---
name: codestyle
description: Универсальные правила кодстайла для стека PHP/Symfony и Vue/TypeScript. Используй перед написанием или изменением кода на этом стеке — найти релевантные правила через MCP/CLI и применить MUST/MUST NOT. Не привязан к конкретной компании или репозиторию.
---

# Codestyle

Соглашения для **PHP 8.4+ / Symfony 7.4+ / Doctrine** и **Vue 3.4+ / TypeScript / Pinia**. Контент лежит в `skills/codestyle/content/` (анонимизированный snapshot).

## Обязательный workflow

Перед кодом на этом стеке:

1. `codestyle_find_rules` (или CLI `bin/codestyle find-rules --task "..."`) с описанием задачи.
2. `codestyle_read_rule` для каждого URI с `priority="critical"`.
3. Пиши код строго по RFC 2119 (`MUST` / `MUST NOT`).

Писать «по памяти» / по PSR-12 / «общепринятым best practices» **нельзя** — эти правила отличаются (табы, Allman-скобки и т.д.).

На другом стеке (Go, Python, …) скилл обычно не нужен: `find_rules` вернёт мало/ноль матчей.

## CLI

```bash
make build-codestyle
./bin/codestyle find-rules --task "новый Symfony контроллер с DTO"
./bin/codestyle read-rule --uri "codestyle://backend/dto"
./bin/codestyle list
```

## MCP tools

- `codestyle_find_rules` — `task`, `max_results?`
- `codestyle_read_rule` — `uri` (`codestyle://…`)

Подробный bootstrap: `content/prompts/codestyle-bootstrap.md`.
