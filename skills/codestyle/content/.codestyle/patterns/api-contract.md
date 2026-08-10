# Правила API-контрактов (Frontend ↔ Backend)

## Формат ответа
- **MUST**: Данные возвращаются напрямую (без объектов-обёрток)
- **MUST**: HTTP-код несёт информацию о результате
- **MUST NOT**: Паттерн `{ success: true, data: [...] }`

## Коды ответов
| Код | Значение | Применение |
|-----|----------|------------|
| 200 | OK | Чтение, обновление |
| 201 | Created | Создание |
| 204 | No Content | Удаление (пустое тело у HTTP-клиента: **ky** / **axios**) |
| 400 | Bad Request | Ошибки валидации |
| 401 | Unauthorized | Отсутствующий/невалидный JWT — см. docs auth проекта (SSO/JWT) |
| 403 | Forbidden | Недостаточная роль |
| 404 | Not Found | Сущность не найдена |
| 500 | Server Error | Необработанное исключение |

## Формат ответа об ошибке
**Системная ошибка** (одно сообщение):
```json
{ "message": "Entity not found" }
```

**Ошибки валидации** (поле → сообщение):
```json
{
	"errors": {
		"name": "Field is required",
		"mp": "Invalid marketplace value"
	}
}
```
- Правило: `message` для 401/403/404/500. `errors` для 400 (валидация).
- **401 JWT** (стандарт Passport): expired → `Your token is expired, please renew it.`; invalid → `Your token is invalid, please login again to get a new one`

## Обработка 204 (No Content) на фронтенде
```typescript
const response = await http.delete(url.goodsDelete.replace(':id', String(id)));
if (response.status === 204)
{
	items.value = items.value.filter((el) => el.id !== id);
	return;
}
```

## Маппинг DTO ↔ Интерфейс
Ключи бэкенд-DTO = snake_case (из БД). Фронтенд-интерфейсы зеркалят это.

```php
// Backend DTO
final readonly class DtoGoods implements DtoInterface
{
	public function __construct(
		public int $id,
		public string $short_name,    // snake_case из БД
		public int|null $entity_id
	) {}
}
```

```typescript
// Frontend интерфейс — зеркалит бэкенд точно
export interface IGoods
{
	id: number;
	short_name: string;     // Тот же snake_case
	entity_id: number | null;
}
```

## Пагинация
```
GET /api/goods?page=1&limit=50
```
- `page`: с 1 (первая страница = 1)
- `limit`: элементов на страницу (по умолчанию 50)
- Ответ содержит массив данных напрямую; total count через отдельный эндпоинт или заголовок ответа

на самом деле не только. если пагинация простая постраничная, удобнее использовать GET /api/goods (1 страница) и GET /api/goods/{page_number} для остальных

## Фильтрация
```
GET /api/goods?status=draft&mp=ozon&sort=created_at&direction=desc
```
- Query-параметры для фильтрации, сортировки
- Бэкенд валидирует и санитизирует все параметры

## Правила
- **MUST**: Фронтенд-типы (`IProduct`) совпадают со структурой бэкенд-DTO точно
- **MUST**: UI-логика переключается по HTTP-коду, не по текстовым меткам
- **MUST NOT**: Изобретать клиентские коды статуса или кастомные форматы ошибок
