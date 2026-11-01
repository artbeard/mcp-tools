---
name: gitlab-mr-reviewer
description: Детерминированный ревьюер GitLab merge request. Используй, когда пользователь даёт ссылку на MR, пару project/iid, source branch или ключ Tracker и просит найти замечания перед merge, deploy или прогоном тестов. Skill проверяет diff по локальному codestyle, developer checklist и паттернам из reviewer-history; результат только SKIP или стабильный список FINDINGS.
---

# GitLab MR Reviewer

Ревью GitLab MR с детерминированным выводом (pre-deploy / pre-merge).

## Обязательные источники

Перед замечаниями прочитай `references/review-rules.md` и `references/reviewer-history.md`.

Если результат для автоматизации или MCP-стадии деплоя - прочитай `references/output-contract.md`.

## Workflow

1. Разбери каждый входной MR в `project_path` и `merge_request_iid` (или полный MR URL).
2. Если передан ключ Tracker - только для понимания scope задачи; ничего не записывай в Tracker.
3. Получи контекст одним вызовом `gitlab_mr_context` (или CLI `bin/gitlab-mr-context`): metadata, changed files/diffs, commits, discussions/notes.
   Не передавай `env_dir` проверяемого репозитория: по умолчанию инструмент читает `.env` / `.env.local`
   корня `mcp-tools`, где настроен доступ к GitLab. `env_dir` допустим только для явно выбранного
   каталога с учётными данными GitLab.
   API использует `GITLAB_TOKEN` (PAT); `GITLAB_USERNAME` нужен только HTTPS-клонированию и не
   заменяет PAT для API.
4. Notes: по умолчанию все; опциональный фильтр авторов через `author_filter` / `GITLAB_MR_AUTHOR_FILTER` (CSV username/name/email/id). Пустой фильтр = все notes.
5. Вызови `codestyle_find_rules` с коротким описанием по изменённым файлам. Прочитай все релевантные правила с `priority="critical"` через `codestyle_read_rule`.
6. Оценивай только факты из diff/metadata/discussions, задачи Tracker и загруженных правил.
7. Верни `SKIP`, если нет actionable-замечаний с file/line или evidence из diff.
8. Верни `FINDINGS`, если найдено хотя бы одно детерминированное нарушение.

## Направление веток

- Для MR в `master` или `main` считай `dev` только downstream-демонстрационной веткой.
- Флагай `BLOCKER`, если source равен `dev` или в коммитах source вне target виден merge-коммит из истории `dev`/`into dev`; такой MR нельзя merge до пересборки ветки от актуального `master`/`main`.
- Не предлагай вливать, ребейзить или cherry-pick'ать `dev` в task/master/main. Исправление: новая ветка от актуального target и перенос только явно выбранных task-коммитов.

## Правила детерминизма

- Не выдавай предположительные замечания.
- Не выдавай best-practice советы без нарушенного правила codestyle или повторяющегося паттерна из `references/reviewer-history.md`.
- Не падай из-за отсутствия опционального контекста; верни `SKIP` и укажи недостающий источник в `checked.limitations`.
- Не проверяй generated/vendor файлы, если diff не меняет рукописный проектный код вокруг них.
- `BLOCKER` только для security, потери данных, сломанных контрактов, небезопасных миграций или deploy-breaking поведения.
- `MAJOR` - исправить до merge, но не immediate blocker.
- `MINOR` - низкорисковые проблемы корректности или поддержки.
- `NIT` только для style/cosmetic, явно покрытых codestyle или `reviewer-history`.

## Вывод

Строго следуй `references/output-contract.md`.

Для человека можно добавить короткое резюме на русском после машинно-стабильного блока. Для deploy-stage - только машинно-стабильный блок.

## Tools

- `gitlab_mr_context` / `bin/gitlab-mr-context` - факты MR
- `codestyle_find_rules` / `codestyle_read_rule` - правила
- Публикация комментариев в MR - отдельно через `gitlab_mr_comment` (этот skill сам не пишет в GitLab)
