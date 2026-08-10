# Правила сервисов

## Внедрение зависимостей
- **MUST**: Конструкторное внедрение с промоутед-свойствами (PHP 8.4+)
- **MUST**: Внедрять `EntityManagerInterface` для доступа к БД (основная цель DI для сервисов)
- **MUST NOT**: Внедрять репозитории напрямую как зависимости конструктора
- **MUST**: Получать репозитории из `EntityManagerInterface` при необходимости в методах

## Stateless
- **MUST**: Сервисы stateless между запросами
- **MAY**: Сеттеры для опциональных runtime-зависимостей (логгер, CLI-вывод)
- **SHOULD**: Сеттеры возвращают `self` для цепочки вызовов

## Возвращаемые типы
- **MUST**: Явные типы возврата на всех методах
- **SHOULD**: Возвращать DTO[] для методов извлечения данных (не сырые сущности)
- **SHOULD**: Использовать union-типы: `string|null`, `int|array`

## Пример
```php
namespace App\Modules\Fbo\Service;

use Doctrine\ORM\EntityManagerInterface;

class FboService
{
	protected SymfonyStyle|null $log = null;

	public function __construct(
		protected EntityManagerInterface $entityManager
	)
	{
	}

	public function setLog(SymfonyStyle $log): self
	{
		$this->log = $log;
		return $this;
	}

	public function loadData(string $mp, int $areaId): array
	{
		// Получаем репозиторий при необходимости, а не через конструктор
		$repo = $this->entityManager->getRepository(FboPlanObject::class);
		return $repo->findBy(['mp' => $mp, 'area_id' => $areaId]);
	}

	public function create(DtoGoodsCreate $dto): DtoGoods
	{
		$entity = new Goods();
		$entity->setName($dto->name)->setMp($dto->mp);
		$this->entityManager->persist($entity);
		$this->entityManager->flush();
		return DtoGoods::fromEntity($entity);
	}
}
```

## Почему нельзя внедрять репозитории напрямую?
Сервисы должны зависеть от `EntityManagerInterface`, а не от конкретных репозиториев, потому что:
1. Уменьшает количество параметров конструктора когда сервис работает с несколькими сущностями
2. Делает зависимости явными в месте использования
3. Предотвращает циклические зависимости в модульной архитектуре

```php
// НЕПРАВИЛЬНО: прямое внедрение репозитория
class FboService
{
	public function __construct(
		protected UserRepository $userRepository  // Запрещено!
	) {}
}

// ПРАВИЛЬНО: получить из EntityManager
$userRepository = $this->entityManager->getRepository(User::class);
```

## Пакетная обработка
- **SHOULD**: Разбивать крупные операции: flush каждые **100** записей (по умолчанию), до **500** для простых вставок
- **SHOULD**: Отслеживать побочные эффекты в массивах, выполнять после завершения основной операции
- **SHOULD**: Вызывать `gc_collect_cycles()` при `memory_get_usage(true) > 255 * 1024 * 1024`

```php
$count = 0;
foreach ($items as $item)
{
	$this->entityManager->persist($entity);
	if (++$count % 100 === 0)
	{
		$this->entityManager->flush();
		if (memory_get_usage(true) > 255 * 1024 * 1024)
		{
			gc_collect_cycles();
		}
	}
}
$this->entityManager->flush(); // Финальный flush
```
