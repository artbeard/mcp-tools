# Соглашения об именовании

## Общая таблица

| Элемент | Стиль | Пример |
|---------|-------|--------|
| PHP/TS классы | PascalCase | `UserRepository`, `OrderService` |
| Vue-компоненты (файл) | PascalCase | `StatusCommentPopover.vue` |
| Методы / функции | camelCase | `getUserProfile()`, `loadAllStocks()` |
| Переменные | camelCase | `$userName`, `resultSet` |
| Константы | UPPER_SNAKE_CASE | `MAX_RETRY_COUNT` |
| Таблицы БД | snake_case с префиксом | `pf_project__fbo_plan_object` |
| Колонки БД | snake_case | `created_at`, `card_id` |
| Файлы сторов | camelCase с префиксом `use` | `useProductStore.ts` |
| Файлы composables | camelCase с префиксом `use` | `useFormatters.ts` |
| Файлы конфигов | kebab-case | `eslint.config.js` |
| PHP Enum cases | PascalCase | `Draft`, `Exported` |
| TS интерфейсы | PascalCase с префиксом `I` | `IState`, `ITrigger` |
| TS типы | PascalCase с префиксом `T` | `TRealtimeTooltipParamName` |
TODO добавь про enum в ts - создаем объект as const вместо использования enum!
если объект или dto содержат выборки из бд, то свойства должны носить такие же названия в snake_case.
Запрещено смешивать в одном объекте разные регистры

## Специфика PHP
- **MUST**: Имя класса = имя файла (`UserRepository.php`)
- **MUST**: Один класс на файл
- **SHOULD**: Приватные свойства в camelCase (исключение: `$created_at` для маппинга БД)
- **MAY**: Префикс `_` для внутренних/приватных хелперов

## Специфика фронтенда
- **MUST**: Vue-файлы в PascalCase: `StatusCommentPopover.vue`
- **MUST**: Файлы сторов: `use[Name]Store.ts` (напр., `useTriggerStore.ts`)
- **MUST**: Файлы composables: `use[Name].ts` (напр., `useFormatters.ts`)
- **SHOULD**: Файлы типов/интерфейсов в PascalCase: `General.ts`

## Таблицы БД
- **MUST**: Формат: `<префикс_проекта>__<имя_сущности>`
- **MUST**: Двойное подчёркивание `__` как разделитель
- Известные префиксы: `pf_project` (пример проекта), `manager_project` (пример проекта), `mp_core` (пример проекта), `wb_core` (пример проекта), `ozon_core` (пример проекта)

## Соглашение о ключах API запросов/ответов
- Бэкенд возвращает ключи в **snake_case** (из имён колонок БД)
- Фронтенд в типах API и DTO использует ключи как есть — без автоматической трансформации в camelCase
- Для внутренних view-model и UI-состояния допустим camelCase, если данные явно нормализуются в отдельном helper/API-адаптере
- Фронтенд-интерфейсы зеркалят именование бэкенда: `{ card_id: number, created_at: string }`

## Комментарии
- **SHOULD**: Комментарии и тексты ошибок предпочтительно на русском языке
- **MUST**: Удалять мёртвый/закомментированный код перед коммитом
- **MAY**: TODO-комментарии допускаются — должны содержать описание задачи

## Антипаттерн
```
class userRepository {}    // НЕПРАВИЛЬНО: Должен быть PascalCase
let UserName = 'test';     // НЕПРАВИЛЬНО: Переменная должна быть camelCase
const max_count = 10;      // НЕПРАВИЛЬНО: Константа должна быть UPPER_SNAKE_CASE
```
