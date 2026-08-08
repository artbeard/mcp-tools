---
name: gitlab-mr-commenter
description: Публикация комментариев в GitLab merge request через локальный API-клиент. Используй, когда пользователь просит отправить, опубликовать, прокомментировать или добавить подготовленный текст в GitLab MR от его имени. Поддерживает MR-level notes и line discussions. Skill требует локальный env с GitLab token, обязательный dry-run и явное подтверждение exact MR URL, места и полного текста комментария перед реальной отправкой.
---
# GitLab MR Commenter

Используй этот skill только для комментариев в GitLab merge request'ах: MR-level notes или line discussions на строках diff.

## Обязательные Источники

Перед отправкой прочитай `references/safety.md`.

Для реальной публикации используй только Go CLI `bin/gitlab-mr-comment` (собрать: `make build-gitlab-mr-comment`).

## Конфигурация

Ожидаемые env-файлы в корне mcp-tools:

```text
.env
.env.local   # опционально, перекрывает .env
```

Минимальные переменные:

```bash
GITLAB_BASE_URL=https://gitlab.example.com
GITLAB_TOKEN=
```

Если `GITLAB_BASE_URL` пуст, берётся host из `--mr-url`.

Никогда не проси пользователя вставить токен в чат. Если токен отсутствует, попроси заполнить `GITLAB_TOKEN` локально.

## Workflow

1. Подготовь полный текст комментария.
2. Для MR-level note покажи пользователю exact final body, MR URL и источник токена как `GitLab token из .env`.
3. Для line discussion дополнительно покажи exact file path, line number и line type (`new` или `old`).
4. Получи явное подтверждение пользователя именно на отправку этого текста в этот MR и, если это line discussion, именно в эту строку.
5. Выполни dry-run для MR-level note:

```bash
./bin/gitlab-mr-comment \
  --mr-url "<MR_URL>" \
  --body-file "<BODY_FILE>"
```

Или dry-run для line discussion:

```bash
./bin/gitlab-mr-comment \
  --mr-url "<MR_URL>" \
  --body-file "<BODY_FILE>" \
  --line-comment \
  --file-path "<PATH_IN_DIFF>" \
  --line <LINE_NUMBER> \
  --line-type new
```

6. Только после успешного dry-run и подтверждения выполни реальную отправку:

```bash
./bin/gitlab-mr-comment \
  --mr-url "<MR_URL>" \
  --body-file "<BODY_FILE>" \
  --send \
  --confirm-send
```

Для line discussion добавь те же `--line-comment --file-path --line --line-type`.

7. Верни пользователю URL созданного комментария/discussion note или точную ошибку GitLab API.

## Запреты

- Не отправляй комментарий без свежего явного подтверждения пользователя.
- Не отправляй комментарий, если итоговый body отличается от подтверждённого.
- Не отправляй line discussion, если MR URL, file path, line number или line type отличаются от подтверждённых.
- Не печатай и не логируй `GITLAB_TOKEN`.
- Не сохраняй токен в skill, репозиторий или временный body-файл.
- Не используй `--send`, если dry-run не прошёл.
