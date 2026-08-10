# Правила DTO (Data Transfer Object)

## Объявление класса
- **MUST**: Использовать `final readonly class` для всех новых DTO (PHP 8.4+)

с final ты погорячился. их очень тяжело тестировать, и от них невозможно унаследоваться (привет предзаказам)

- **MUST**: Реализовывать `App\Model\DtoInterface`
- **MAY**: Наследоваться от `AbstractDto` (для общей логики)
- **MAY**: Реализовывать `\JsonSerializable` для DTO API-ответов

## Свойства
- **MUST**: Конструктор с промоутед-свойствами (PHP 8.4+). вся логика преобразования входных параметров должна находиться в конструкторе
- **MUST**: Все свойства `public readonly` (через `readonly class`)
- **MUST**: Все свойства типизированы (union-типы для nullable: `string|null`)
- **MUST**: В классе с dto нельзя использовать `declare(strict_types=1);` т.к. он должен приводить типы в конструторе автоматически:
```php
public float $id преобразует тип "0.35" => 0.35
```

## Обязательный метод: fromArray
- **MUST**: Статический `fromArray(array $properties): static`
- **MUST NOT**: Использовать `??` для обязательных полей — программа MUST упасть при отсутствии поля
- **MUST NOT**: Использовать nullsafe-оператор `?->` в `fromArray`
- **MUST NOT**: Использовать строгий режим `declare(strict_types=1);` в DTO классах
- **MUST**: Вся обработка данных (JSON decode, приведение типов) происходит в **конструкторе**, не в `fromArray`

## Опциональный метод: fromEntity
- **MAY**: Статический `fromEntity(Entity $entity): self` для конвертации Entity → DTO или лучше  `fromEntity(Entity $entity): static` если унаследовался

## Пример (стандартный DTO)
```php
namespace App\Modules\Goods\Dto;

use App\Model\DtoInterface;
use JsonSerializable;

final readonly class DtoGoods implements DtoInterface, JsonSerializable
{
	public function __construct(
		public int $id,
		public string $shortname,
		public string|null $category,
		public float|null $sebes
	)
	{
	}

	public static function fromArray(array $properties): self
	{
		return new self(
			$properties['id'],        // Упадёт при отсутствии — намеренно
			$properties['shortname'],
			$properties['category'],
			$properties['sebes']
		);
	}

	public static function fromEntity(\App\Entity\Goods $entity): self
	{
		return new self(
			$entity->getId(),
			$entity->getShortname(),
			$entity->getCategory(),
			$entity->getSebes()
		);
	}

	public function jsonSerialize(): array
	{
		//для простых случаев
		return get_object_vars($this);
	}
}
```

## Вложенные DTO (обработка в конструкторе)
```php
final readonly class DtoBag implements DtoInterface
{
	/** @var DtoBagProduct[] */
	public array $products;

	public function __construct(
		public string $mp,
		public int $card_id,
		array | string $products
	)
	{
		if (is_string($products)) //например, если в поле json выборка из бд
		{
		    $products = json_encode($products, true);
		}
		// Конвертация вложенных DTO MUST происходить в конструкторе
		$this->products = array_map(
			static fn($item) => DtoBagProduct::fromArray($item),
			$products
		);
	}

	public static function fromArray(array $properties): self
	{
		return new self(
			$properties['mp'],
			$properties['card_id'],
			$properties['products']
		);
	}
}
```

## Граница обработки данных
**Правило**: `fromArray()` — простой маппер свойств. Конструктор выполняет все трансформации.

```php
// НЕПРАВИЛЬНО: обработка в fromArray
public static function fromArray(array $properties): self
{
	return new self(
		$properties['id'] ?? null,                    // НЕПРАВИЛЬНО: ?? для обязательного поля
		json_decode($properties['config'] ?? '{}')    // НЕПРАВИЛЬНО: обработка здесь
	);
}

// ПРАВИЛЬНО: сырые данные в fromArray, обработка в конструкторе
final readonly class DtoConfig implements DtoInterface
{
	public array $configParsed;

	public function __construct(
		public int $id,
		array|string $configRaw
	)
	{
		// Парсинг JSON в конструкторе
		$this->configParsed = is_string($configRaw)
			? json_decode($configRaw, true) ?? []
			: $configRaw;
	}

	public static function fromArray(array $properties): self
	{
		return new self(
			$properties['id'],      // Упадёт при отсутствии — хорошо
			$properties['config']   // Сырое значение, конструктор обработает
		);
	}
}
```

## Мутабельный DTO (исключение)
Если DTO должен быть мутабельным (напр., пакетные обновления), убери `readonly` с класса и задокументируй причину:
```php
// Исключение: мутабельный DTO для накопления результатов пакетной обработки
/** Мутабельный: накапливает результаты расчётов при пакетной обработке */
final class DtoPlanResult implements DtoInterface { /* ... */ }
```

## Расположение
- DTO модуля: `src/Modules/{ModuleName}/Dto/`
- Общие DTO: `src/Dto/`


## универсальные DTO для post/patch

```php
namespace App\Dto;

final readonly class UndefinedProperty
{
	public function __construct()
	{
	}

	public static function is_undefined(mixed $value): bool
    {
        return $value instanceof self;
    }
}
```

```php
namespace App\Modules\Goods\Dto;

use App\Dto\UndefinedProperty

final readonly class DtoGoods implements DtoInterface
{
	public function __construct(
		public UndefinedProperty | int $id,
		public UndefinedProperty | string $shortname,
		public UndefinedProperty | string|null $category,
		public UndefinedProperty | float|null $sebes
	)
	{
	}

    /**
     * Создает полностью заполненную dto. если что-то не передали - падаем
     * @param array $properties
     * @return self
     */
	public static function fromArray(array $properties): static
	{
		return new self(
			$properties['id'] ?? new UndefinedProperty, //UndefinedProperty при создании новой сущности, int при полном обновлении
			$properties['shortname'],
			$properties['category'],
			$properties['sebes']
		);
	}

	/**
     * Создает dto с необязательными свойствами. если что-то не передали, тогда это поле не меняем
     * @param array $properties
     * @return self
     */
	public static function createFromRequestData(array $properties): static
	{
		return new self(
			$properties['id'] ?? new UndefinedProperty,
			$properties['shortname'] new UndefinedProperty,
			$properties['category'] new UndefinedProperty,
			$properties['sebes'] new UndefinedProperty
		);
	}

}
```

далее в сервисе

```php

foreach (get_object_vars($dto) as $name => $value)
{
    if (!UndefinedProperty::is_undefined($value))
    {
        // свойство задано - значит, меняем его в сущности
    }
}

```