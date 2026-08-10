# Исторические паттерны замечаний (reviewer-history)

Используй эти правила только как детерминированные `reviewer-history` checks. Не пересказывай стиль комментария; флагай только конкретный diff evidence.

## Подтверждённые паттерны

- `env-not-code`: если MR добавляет runtime connection/config value прямо в PHP/JS/service code или committed config, флагай `MAJOR`. Evidence: добавленная строка с host/DSN/AMQP/URL/credential/config literal. Fix: вынести значение в `.env`, env var или secret.
- `use-shared-event-client`: применяй только к MR, которые явно затрагивают новый event/eventclient flow или проект с уже подключённым общим event client. Если такой MR вручную добавляет дублирующий publisher/client/HTTP integration вместо существующего shared event client, флагай `MAJOR`. Fix: использовать общий клиент/пакет и передавать с хоста только конфиг транспорта.
- `event-payload-minimal`: применяй только к MR про event contract/payload. Если event payload тащит display-only, derived или mutable fields, которые потребитель может получить по id на своей стороне, флагай `MAJOR`. Fix: оставить в событии только устойчивый факт и идентификаторы.
- `no-internal-pubsub-http`: применяй только когда MR явно реализует notifications/events/pub-sub flow. Если MR добавляет внутренний HTTP endpoint для создания событий/уведомлений вместо принятого pub/sub или message-bus flow, флагай `MAJOR`; поднимай до `BLOCKER`, если endpoint может стать доступным снаружи без авторизации. Fix: публиковать событие через transport/client layer.
- `no-local-composer-repositories`: если `composer.json` добавляет `repositories` entry с `type: "vcs"` или `type: "path"` на соседний/локальный репозиторий без явной инфраструктурной причины, флагай `MAJOR`. Fix: убрать временный repository source и зависеть от нормального пакета/tag/registry.
- `composer-version-after-tag`: если `composer.json` приложения добавляет или меняет поле `version`, а MR не является release/tag metadata change, флагай `MINOR`. Fix: не фиксировать `version` до опубликованного Git tag.
- `gitignore-generated-config`: если diff добавляет generated/reference/local runtime config файл, который должен генерироваться или задаваться окружением, флагай `MINOR` или `MAJOR` при риске утечки. Fix: удалить файл из репозитория и добавить pattern в `.gitignore`.
- `newline-at-eof`: если diff содержит `\ No newline at end of file` в рукописном текстовом файле, флагай `NIT`. Fix: добавить trailing newline.
- `mysql-collation-general-ci`: если миграция/DDL добавляет `utf8mb4_0900_ai_ci`, флагай `MAJOR`. Fix: использовать `utf8mb4_general_ci`, если проект явно не требует другой collation.
- `no-schema-env-fallback`: если SQL/data-access code добавляет `$_ENV['DB_*_SCHEMA'] ?? 'prod_or_default_schema'`, флагай `MAJOR`. Fix: для cross-db queries требовать явный env и падать при отсутствии, чтобы не писать/читать production schema из dev.
- `inline-table-names`: если MR добавляет helper/autogeneration вроде `$this->table(...)` для SQL table names там, где принято явно видеть таблицу в запросе, флагай `MINOR`. Fix: указать таблицу явно в SQL.
- `reference-controller-readonly`: если reference controller/endpoint начинает содержать бизнес-логику, тяжёлые запросы или не справочные данные, флагай `MAJOR`. Fix: оставить reference endpoint только для справочников, бизнес-данные вынести в профильный endpoint/service.
- `complex-input-dto`: если controller/service вручную разбирает сложные request params/arrays без DTO, флагай `MAJOR`. Fix: завернуть входные данные в request DTO и передавать DTO в service.
- `mutation-with-log-transaction`: если diff добавляет критичное изменение данных без транзакции и логирования в той же атомарной ветке, флагай `MAJOR`. Fix: обернуть mutation и log write в transaction.
- `frontend-type-naming`: если frontend TypeScript добавляет/меняет type alias без префикса `T*` или interface без префикса `I*` в проекте, где применяется frontend naming convention, флагай `NIT` или `MINOR` при публичном/shared типе. Fix: переименовать type aliases в `TName`, interfaces в `IName`.
- `frontend-magic-business-values`: если frontend добавляет string/number literal union/status values, которые используются как бизнес-значения, без `as const` object/enum-like map, флагай `MINOR`. Fix: вынести значения в const object и строить type от него.

## Severity

- Поднимай `env-not-code` до `BLOCKER`, если в diff виден секрет, пароль, token, private key или production credential.
- Поднимай `gitignore-generated-config` до `BLOCKER`, если файл содержит секреты или production-only настройки.
- Поднимай `no-internal-pubsub-http` до `BLOCKER`, если endpoint public/external и создаёт сообщения/уведомления без явной авторизации.
- Поднимай `no-schema-env-fallback` до `BLOCKER`, если fallback может направить write operation в production schema.
- Для чисто косметических EOF/newline замечаний используй только `NIT`.

## Evidence requirements

- `file` должен быть изменённым файлом из MR diff.
- `line` должен указывать на добавленную или изменённую строку, если GitLab position доступен.
- `evidence` должен описывать конкретный добавленный fragment, не общую догадку.
- `rule` указывай как `reviewer-history:<pattern>`, например `reviewer-history:env-not-code`.

## Происхождение (анонимизировано)

Паттерны собраны из повторяющихся line/MR discussions по стеку PHP/Symfony + Vue/TS и из checklist'ов проекта. Конкретные MR URL, username'ы ревьюеров и локальные dump-файлы сюда не входят.

Типичные темы: API/contracts/DTO, frontend/Vue/TypeScript, tests, migrations/db/schema, env/config/secrets, composer/dependencies, generated/local files. Свежие кластеры по одному проекту отделяй от межпроектных повторяющихся замечаний.
