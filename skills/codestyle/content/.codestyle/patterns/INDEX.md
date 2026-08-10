# Навигация по архитектурным паттернам

Сквозные паттерны для full-stack разработки.

## Файлы секции

- [data-flow.md](data-flow.md) — Полный путь запроса/ответа: Пользователь → Компонент → Стор → API → Контроллер → Сервис → Репозиторий → Сущность → обратно
- [crud-flow.md](crud-flow.md) — Сквозной CRUD-паттерн по всем слоям
- [api-contract.md](api-contract.md) — API-контракты Frontend ↔ Backend, маппинг DTO, HTTP-коды
- [module-structure.md](module-structure.md) — Полный скаффолд нового модуля (бэкенд + фронтенд)
- [testing.md](testing.md) — PHPUnit, Vitest, стратегия тестирования
- [security.md](security.md) — Passport SSO, JWT verify, profile/roles, `#[CurrentUser]` (docs auth проекта (SSO/JWT))

## Помощник выбора
- Понять путь данных? → `data-flow.md`
- Новая CRUD-фича? → `crud-flow.md` + `module-structure.md`
- Проектирование API? → `api-contract.md`
- Создание нового модуля? → `module-structure.md`
- Добавление авторизации/ролей? → `security.md`
