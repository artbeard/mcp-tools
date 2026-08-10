# Правила контроллеров

## Базовый класс
- **MUST**: Наследоваться от `AbstractController` (или проектного `BaseController`)

## Маршрутизация
- **MUST**: Использовать PHP-атрибуты: `#[Route('/api/...', name: '...', methods: ['GET'])]`
- **MUST NOT**: Использовать маршрутизацию на аннотациях (`@Route`)

## Привязка запросов (Symfony 7.4+)
- **SHOULD**: Использовать `#[MapRequestPayload]` для маппинга тела POST/PUT → DTO
- **SHOULD**: Использовать `#[MapQueryString]` для маппинга GET-параметров → DTO
- **MAY**: Откат на `$request->toArray()` + `DtoCreate::fromArray()` для простых случаев

```php
// Современный подход: автоматический маппинг DTO через Symfony serializer
#[Route('/api/goods', methods: ['POST'])]
public function create(
	#[MapRequestPayload] DtoGoodsCreate $dto,
	GoodsService $goodsService
): JsonResponse
{
	$result = $goodsService->create($dto);
	return $this->json($result, Response::HTTP_CREATED);
}

// Тоже допустимо: ручное создание DTO
#[Route('/api/goods', methods: ['POST'])]
public function create(GoodsService $goodsService, Request $request): JsonResponse
{
	$dto = DtoGoodsCreate::fromArray($request->toArray());
	$result = $goodsService->create($dto);
	return $this->json($result, Response::HTTP_CREATED);
}
```

## Формат API-ответов
- **MUST**: Возвращать данные напрямую через `$this->json($data, $statusCode)`
- **MUST**: Использовать константы `Response::HTTP_*` (не сырые числа)
- **MUST NOT**: Добавлять обёртки `"success"`, `"status"`, `"ok"`
- HTTP-код несёт всю информацию о результате

## Внедрение зависимостей
- **MUST**: Конструкторное DI с промоутед-свойствами для общих зависимостей
- **MUST**: `EntityManagerInterface` внедряется в **контроллер** или **сервис** — не в оба для одной операции
- **MAY**: DI через параметры метода для зависимостей одного эндпоинта
- **MAY**: Атрибут `#[CurrentUser]` для получения авторизованного пользователя — см. `../patterns/security.md`, docs auth проекта (SSO/JWT)

## Пример
```php
namespace App\Controller;

use Symfony\Component\HttpFoundation\JsonResponse;
use Symfony\Component\HttpFoundation\Response;
use Symfony\Component\Routing\Attribute\Route;

class GoodsController extends AbstractController
{
	#[Route('/api/goods', name: 'app_api_goods_list', methods: ['GET'])]
	public function index(GoodsService $goodsService): JsonResponse
	{
		$goodsList = $goodsService->getGoodsList();
		return $this->json($goodsList);
	}

	#[Route('/api/goods/{id}', name: 'app_api_goods_update', methods: ['PUT'])]
	public function update(int $id, GoodsService $goodsService, Request $request): JsonResponse
	{
		$dto = DtoGoodsUpdate::fromArray($request->toArray());
		$result = $goodsService->update($id, $dto);
		return $this->json($result, Response::HTTP_OK);
	}

	#[Route('/api/goods/{id}', name: 'app_api_goods_delete', methods: ['DELETE'])]
	public function delete(int $id, GoodsService $goodsService): JsonResponse
	{
		$goodsService->delete($id);
		return $this->json(null, Response::HTTP_NO_CONTENT);
	}
}
```

## HTTP-коды ответов
| Действие | Код | Константа |
|----------|-----|-----------|
| Чтение / Обновление | 200 | `Response::HTTP_OK` |
| Создание | 201 | `Response::HTTP_CREATED` |
| Удаление | 204 | `Response::HTTP_NO_CONTENT` |
| Ошибка валидации | 400 | `Response::HTTP_BAD_REQUEST` |
| Не авторизован | 401 | `Response::HTTP_UNAUTHORIZED` |
| Доступ запрещён | 403 | `Response::HTTP_FORBIDDEN` |
| Не найдено | 404 | `Response::HTTP_NOT_FOUND` |

## Антипаттерн
```php
// НЕПРАВИЛЬНО: обёртка + сырое число
return $this->json([
	'success' => true,
	'data' => $result
], 200);

// ПРАВИЛЬНО
return $this->json($result, Response::HTTP_OK);
```

Я бы добавил сюда формирование эндпоинтов для api:
- /api/*<модуль>*/ ...
- /api/<версия>/*<модуль>*/ ... (для версионированных api)

Семантика:
GET `/api/entity` - список сущностей, фильтрация через query
POST/PUT `/api/entity` - создание сущности, в результате вренуть только id сущности для fullstate приложений, код Response::HTTP_CREATED, для stateless приложений вернуть всю сущность
DELETE/PATCH `/api/entity/{entity_id<\d+>}` - опреация над сущностью по id. по умолчанию, методы **идемпотентны** для fullstate, возвращают только Response::HTTP_NO_CONTENT
Аналогично с дочерними сущностями
GET `/api/entity/{entity_id<\d+>}/sub-entity` - список дочених сущностей, допфильтрация через query

Как ты написал выше - просто отличная практика все входящие данные принимать через DTO!!!
