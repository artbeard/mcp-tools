---
name: yandex-tracker-issue-creator
description: Создание задач в Yandex Tracker через локальный API-клиент. Используй, когда пользователь просит создать/завести задачу в Tracker, в очереди или проекте, указать проект/теги/период/SP/исполнителя/описание. Skill требует локальный env с OAuth/Org ID, обязательный dry-run и явное подтверждение exact payload перед реальной отправкой.
---
# Yandex Tracker Issue Creator

Используй этот skill для создания задач в Yandex Tracker  от имени пользователя, чей OAuth token лежит в локальном env.

## Обязательные источники

Перед созданием задачи прочитай `references/safety.md`.

Для создания задач используй только `scripts/create_tracker_issue.py`.

## Конфигурация

Ожидаемый env-файл:

```text
config/tracker.env
```

Минимальные переменные:

```bash
TRACKER_BASE_URL=https://api.tracker.yandex.net
TRACKER_ORG_ID=
TRACKER_OAUTH_TOKEN=
```

Альтернативно вместо `TRACKER_ORG_ID` можно задать `TRACKER_CLOUD_ORG_ID`.

Опциональные field mapping переменные для нестандартных полей очереди:

```bash
TRACKER_PROJECT_FIELD=project
TRACKER_PERIOD_FIELD=period
TRACKER_SP_FIELD=storyPoints
```

Если Tracker в конкретной очереди использует другие keys для проекта, периода или story points, передай их через `--project-field`, `--period-field`, `--sp-field` или через `--field KEY=VALUE`.

## Workflow

1. Определи queue key, summary, description, project, tags, period, SP и assignee из запроса пользователя.
2. Если queue/project/assignee неясны, задай короткий уточняющий вопрос. Не угадывай очередь.
3. Подготовь финальное описание задачи в UTF-8 файле во временной директории.
4. Покажи пользователю exact будущую задачу:
   - queue;
   - summary;
   - description;
   - project и field key, если указан;
   - tags;
   - period и field key, если указан;
   - SP и field key, если указан;
   - assignee;
   - extra fields;
   - source authorization как `Tracker token из `config/tracker.env``.
5. Выполни dry-run:

```bash
python3 skills/yandex-tracker-issue-creator/scripts/create_tracker_issue.py \
  --queue "<QUEUE>" \
  --summary "<SUMMARY>" \
  --description-file "<DESCRIPTION_FILE>" \
  --project "<PROJECT>" \
  --tag "<TAG>" \
  --period "<PERIOD>" \
  --sp "<SP>" \
  --assignee "<ASSIGNEE>"
```

6. Покажи пользователю dry-run payload и получи явное подтверждение на создание именно этой задачи с этим payload.
7. Только после успешного dry-run и свежего подтверждения выполни реальное создание:

```bash
CONFIRM_SEND=YES python3 skills/yandex-tracker-issue-creator/scripts/create_tracker_issue.py \
  --queue "<QUEUE>" \
  --summary "<SUMMARY>" \
  --description-file "<DESCRIPTION_FILE>" \
  --project "<PROJECT>" \
  --tag "<TAG>" \
  --period "<PERIOD>" \
  --sp "<SP>" \
  --assignee "<ASSIGNEE>" \
  --send
```

8. Верни пользователю key/URL созданной задачи или точную ошибку Tracker API.

## Поля

- `--queue`: обязательная очередь Tracker, например `IT`.
- `--summary`: обязательный заголовок.
- `--description-file`: файл с описанием.
- `--project`: значение проекта; по умолчанию попадет в поле из `TRACKER_PROJECT_FIELD` или `project`.
- `--tag`: тег, можно повторять. Также можно передать несколько через запятую.
- `--period`: период задачи, например `2026-07-01..2026-07-12` или `2026-W29`.
- `--sp`: story points, число.
- `--assignee`: исполнитель. Plain значение трактуется как login; можно использовать `login:example.user`, `id:...`, `uid:...`, `passport:...`, `tracker:...`, `cloud:...`.
- `--field KEY=VALUE`: произвольное поле Tracker. JSON value поддерживается, например `--field parent='{"key":"IT-123"}'`.

## Запреты

- Не создавай задачу без dry-run.
- Не создавай задачу без свежего явного подтверждения пользователя.
- Не создавай задачу, если итоговый payload отличается от подтвержденного.
- Не проси и не печатай `TRACKER_OAUTH_TOKEN`, Client secret, cookies или другие секреты.
- Не сохраняй токены/секреты в skill, репозиторий или body-файл.
- Не меняй существующие задачи, статусы, комментарии, связи или worklog этим skill.
- Не используй `--send`, если dry-run не прошел.
