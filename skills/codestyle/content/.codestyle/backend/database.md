# Правила БД и SQL

## Доступ к соединению
```php
// Через EntityManagerInterface (предпочтительно в сервисах)
$connection = $this->entityManager->getConnection();

// Через ManagerRegistry (альтернатива)
$connection = $this->doctrine->getConnection();
```

## Форматирование SQL
- **MUST**: Использовать heredoc `<<<SQL` для многострочного SQL
- **MUST**: Отступы внутри heredoc для читаемости

## SELECT-запросы
- **MUST**: Использовать `$connection->executeQuery()` для SELECT
- **MUST**: Передавать `ParameterType::*` третьим аргументом для всех параметров

```php
use Doctrine\DBAL\ParameterType;

$sql = <<<SQL
	SELECT
		id,
		username,
		status,
		config
	FROM manager_project__user
	WHERE id = :id
		AND status = :status
		AND mp = :mp
SQL;

$resultSet = $connection->executeQuery(
	$sql,
	['id' => $userId, 'status' => 1, 'mp' => 'ozon'],
	['id' => ParameterType::INTEGER, 'status' => ParameterType::INTEGER, 'mp' => ParameterType::STRING]
)->fetchAllAssociative();
```

## Справка по ParameterType
| Тип | Применение | Пример |
|-----|-----------|--------|
| `ParameterType::INTEGER` | целые числа | ID, счётчики, статус-коды |
| `ParameterType::STRING` | строки | Имена, коды МП, UUID |
| `ParameterType::BOOLEAN` | булевы значения | Флаги |
| `ParameterType::NULL` | null-значения | Явная привязка null |
| `ParameterType::LARGE_OBJECT` | BLOB-ы | Бинарные данные |
| `ParameterType::ASCII_STRING` | ASCII-строки | Коды, хэши |

Для `IN (...)` используй `ArrayParameterType::INTEGER` или `ArrayParameterType::STRING` с `Connection::PARAM_INT_ARRAY` / `Connection::PARAM_STR_ARRAY`.

## INSERT/UPDATE/DELETE
- **MUST**: Использовать `$connection->executeStatement()` для операций записи
- **MAY**: Использовать `$connection->update()` / `$connection->delete()` для простых операций с одной таблицей

```php
$sql = <<<SQL
	UPDATE manager_project__user
	SET status = :status
	WHERE id = :id
SQL;

$affectedRows = $connection->executeStatement(
	$sql,
	['status' => 2, 'id' => $userId],
	['status' => ParameterType::INTEGER, 'id' => ParameterType::INTEGER]
);
```

## Миграции
- **MUST**: Создавать через `php bin/console make:migration`
- **MUST**: Просмотреть сгенерированный SQL после создания — удалить лишние операции
- **MUST NOT**: Выполнять миграцию без проверки её кода
- **SHOULD**: Тестировать на стейджинге перед продакшеном
- **SHOULD**: Трансформации данных (смена формата, разделение колонок) в отдельной миграции от изменений схемы

## Фильтр схемы
При добавлении сущности для существующей таблицы, добавь имя таблицы в `doctrine.yaml` schema_filter:
```yaml
doctrine:
  dbal:
    connections:
      default:
        schema_filter: ~^(!?project_prefix_|existing_table)~
```

## Именование таблиц
- **MUST**: `<префикс_проекта>__<имя_таблицы>` с двойным подчёркиванием
- Стратегия именования: `underscore_number_aware` (автоматический snake_case)


Укажи обязательно кодировку в конфиге!!!
