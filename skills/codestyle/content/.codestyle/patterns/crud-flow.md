# CRUD-паттерн (End-to-End)

## Поток создания (Create)

### Backend
```php
// 1. DTO для входных данных
class DtoGoodsCreate implements DtoInterface
{
	public function __construct(
		readonly string $name,
		readonly string $mp,
		readonly int $entity_id
	) {}

	public static function fromArray(array $properties): static
	{
		return new static($properties['name'], $properties['mp'], $properties['entity_id']);
	}
}

// 2. Контроллер
#[Route('/api/goods', methods: ['POST'])]
public function create(GoodsService $service, Request $request): JsonResponse
{
	$dto = DtoGoodsCreate::fromArray($request->toArray());
	$result = $service->create($dto);
	return $this->json($result, Response::HTTP_CREATED);
}

// 3. Сервис
public function create(DtoGoodsCreate $dto): DtoGoods
{
	$entity = new Goods();
	$entity->setName($dto->name)->setMp($dto->mp);
	$this->entityManager->persist($entity);
	$this->entityManager->flush();
	return DtoGoods::fromEntity($entity);
}
```

> **Замечение**
> Даня, тут нужно сделать оговорку
> Возвращаем созданные сущности только для stateless приложений, для statefull возвращаем только id, и саму сущность, если она требует
> дополнительных данных и вычисления доплнительных свойств, которых нет на фронте

### Frontend
```typescript
import { goodsApi } from '@/api/goods';

// Экшен стора (setup-store): доменный вызов
async function createGoods(data: Partial<IGoods>): Promise<IGoods>
{
	const created = await goodsApi.create(data, { showGlobalLoading: true });
	items.value.push(created);
	return created;
}

// Компонент
async function handleCreate()
{
	await store.createGoods(formData.value);
}
```

> **Замечение**
> Этот пример не бъется с кодстайлом фронта

## Чтение (Read) — список + одиночная запись
- Список: `GET /api/goods` → Контроллер → Сервис → Репозиторий → DTO[] → json()
- Одиночная: `GET /api/goods/:id` → тот же поток с `findOneBy()`

## Обновление (Update)
- метод идемпотентный
- `PUT/PATCH /api/goods/:id` → DtoUpdate.fromArray() → Service.update() → flush()
- Фронтенд: оптимистичный патч стейта → API-вызов → откат при ошибке

## Удаление (Delete)
- Метод идемпотентный
- `DELETE /api/goods/:id` → Service.delete() → remove() + flush()
- Ответ: `Response::HTTP_NO_CONTENT` (204)

## Fullstate vs Stateless

- **Fullstate-приложение**: фронтенд хранит полный актуальный state сущностей и обычно может сам собрать итоговую модель после мутации.
- **Stateless-приложение**: фронтенд не полагается на локальный полный state как источник истины и после мутации получает готовую сущность от бэкенда.

### Create
- **MUST** (`fullstate`): `POST /...` возвращает `id` созданной сущности (минимальный payload).
- **MUST** (`stateless`): `POST /...` возвращает всю созданную сущность.

### Update
- **MUST** (`fullstate`): `PUT/PATCH /.../:id` может не возвращать полную сущность (достаточно кода успеха / минимального ответа по контракту).
- **MUST** (`stateless`): `PUT/PATCH /.../:id` возвращает всю изменённую сущность.

### Идемпотентность мутаций
- **MUST**: `PATCH` и `DELETE` считаются идемпотентными операциями в рамках проектного контракта.
- **MUST**: Если сущность не найдена при `DELETE`, операция считается успешной (целевое состояние «сущности нет» уже достигнуто).

### Когда fullstate всё-таки возвращает сущность
- **SHOULD NOT** (`fullstate`): Возвращать полную сущность при `create/update` без необходимости.
- **MAY** (`fullstate`): Возвращать полную сущность только если после мутации нужна сложная серверная логика/обогащение и эти данные недоступны фронтенду локально.

## Ключевые правила
- **MUST**: Входные данные обёрнуты в DTO на уровне контроллера
- **MUST**: Сервис возвращает DTO (не Entity)
- **MUST**: HTTP-коды: 201 (создано), 200 (чтение/обновление), 204 (удаление)
- **MUST NOT**: Entity-объекты возвращаются напрямую из контроллеров

> **Замечение**
> `Входные данные обёрнуты в DTO на уровне контроллера` - не обязательно - если настроена серилизация для сущности - можно возвращать саму сущность
> `Сервис возвращает DTO (не Entity)` дто возвращаются в 100% случаев - если рзеультат запроса массив а не сущность!
