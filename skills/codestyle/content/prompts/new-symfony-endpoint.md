# Новый Symfony API-эндпоинт

Сценарий: добавляешь новый REST-эндпоинт (POST/GET/PUT/DELETE) в Symfony-проекте.

## Обязательные правила (URI) (загрузи до начала кодинга)

- `codestyle://backend/controller` — структура контроллера, `#[Route]`, `#[MapRequestPayload]`, HTTP-коды, формат API
- `codestyle://backend/dto` — `final readonly class`, `fromArray()`, без `??` на обязательных полях
- `codestyle://backend/service` — DI через `EntityManagerInterface`, stateless, бизнес-логика

## Опционально (если применимо)

- `codestyle://backend/entity` — если эндпоинт создаёт/изменяет сущность
- `codestyle://backend/database` — если используется raw SQL
- `codestyle://patterns/crud-flow` — если это полный CRUD-цикл (Create/Read/Update/Delete)
- `codestyle://patterns/api-contract` — если эндпоинт публичный или меняет контракт с фронтом
- `codestyle://patterns/security` — если требуется ролевая проверка / `#[CurrentUser]`

## Чек-лист перед коммитом

- [ ] Контроллер «тонкий» — делегирует Service'у, не содержит бизнес-логики.
- [ ] Входные данные обёрнуты в DTO (через `#[MapRequestPayload]` или `Dto::fromArray($request->toArray())`).
- [ ] HTTP-коды осмысленные: 201 Created, 204 No Content, 4xx/5xx, без success-обёртки в JSON.
- [ ] Service внедряет `EntityManagerInterface`, не Repository напрямую.
- [ ] Allman-скобки. Табы 4sp. `final readonly class` для DTO.
- [ ] Entity, если новая — создана через `php bin/console make:entity`.
- [ ] Если ролевая проверка — `#[IsGranted]` или явная проверка через `Security`.
