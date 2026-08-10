# Правила сущностей

## Создание
- **MUST**: Создавать сущности только через `php bin/console make:entity`
- **MUST NOT**: Создавать файлы сущностей вручную
- **SHOULD**: Модифицировать сущности через `make:entity EntityName` по возможности

## ORM-атрибуты
- **MUST**: Использовать PHP-атрибуты `#[ORM\...]` (не DocBlock-аннотации)
- **MUST**: Явное имя таблицы: `#[ORM\Table(name: 'prefix__entity_name')]`
- **MUST**: Типы колонок из `Doctrine\DBAL\Types\Types` (`Types::JSON`, `Types::TEXT` и т.д.)
- **MUST**: Комментарии полей: `options: ['comment' => 'Описание']` на каждой колонке
- **MUST**: Каждый индекс в отдельном атрибуте `#[ORM\Index(...)]`
- **MUST NOT**: Использовать мутабельный `\DateTime` — использовать `\DateTimeImmutable` с `Types::DATETIME_IMMUTABLE`
- TODO добавь сюда, что все свойства в snaсk_case, в аттрибутах параметр name не используем

## Свойства
- **MUST**: `private` (или `protected` для родительских сущностей)
- **MUST**: Геттеры и сеттеры для всех свойств
- **MUST**: Сеттеры возвращают `static` для цепочки вызовов
- **MAY**: PHP 8.4 — асимметричная видимость (`public private(set)`) для простого read-only доступа

## Именование таблиц
- **MUST**: Формат: `<префикс_проекта>__<имя_сущности>` (двойное подчёркивание)
- Известные префиксы: `pf_project` (пример проекта), `manager_project` (пример проекта), `mp_core` (пример проекта), `wb_core` (пример проекта), `ozon_core` (пример проекта)

## Справка по типам колонок
| Тип PHP | Тип Doctrine | Применение |
|---------|-------------|------------|
| `?int` | `Types::INTEGER` | ID, счётчики |
| `?string` | `Types::STRING` (+ `length:`) | Короткий текст |
| `?string` | `Types::TEXT` | Длинный текст |
| `?float` | `Types::DECIMAL` (+ `precision:`, `scale:`) | Деньги, проценты |
| `?\DateTimeImmutable` | `Types::DATETIME_IMMUTABLE` | Все даты |
| `array` | `Types::JSON` | JSON-структуры |
| `?bool` | `Types::BOOLEAN` | Флаги |

## Пример
```php
namespace App\Entity;

use App\Repository\FboPlanObjectRepository;
use Doctrine\DBAL\Types\Types;
use Doctrine\ORM\Mapping as ORM;

#[ORM\Entity(repositoryClass: FboPlanObjectRepository::class)]
#[ORM\Table(
	name: 'pf_project__fbo_plan_object',
	options: ['comment' => 'Объект планирования FBO отгрузки']
)]
#[ORM\Index(name: 'idx_status', fields: ['status'])]
#[ORM\Index(name: 'idx_mp_area', fields: ['mp', 'area_id'])]
class FboPlanObject
{
	#[ORM\Id]
	#[ORM\GeneratedValue]
	#[ORM\Column(options: ['comment' => 'ID'])]
	private ?int $id = null;

	#[ORM\Column(length: 10, options: ['comment' => 'Маркетплейс (wb, ozon, ym)'])]
	private ?string $mp = null;

	#[ORM\Column(type: Types::DECIMAL, precision: 4, scale: 1, nullable: true,
		options: ['comment' => 'Количество паллет'])]
	private ?float $pallet_count = null;

	#[ORM\Column(type: Types::DATETIME_IMMUTABLE, options: ['comment' => 'Дата создания'])]
	private ?\DateTimeImmutable $created_at = null;

	#[ORM\Column(type: Types::JSON, options: ['comment' => 'Конфигурация объекта'])]
	private array $config = [];

	public function getMp(): ?string
	{
		return $this->mp;
	}

	public function setMp(?string $mp): static
	{
		$this->mp = $mp;
		return $this;
	}
}
```

## Связи
```php
// ManyToOne (дочерний → родитель)
#[ORM\ManyToOne(targetEntity: FboSupply::class, inversedBy: 'objects')]
#[ORM\JoinColumn(name: 'fbo_supply_id', referencedColumnName: 'id')]
private ?FboSupply $fboSupply = null;

// OneToMany (родитель → дочерние)
#[ORM\OneToMany(targetEntity: FboPlanObject::class, mappedBy: 'fboSupply')]
private Collection $objects;

// В конструкторе:
public function __construct()
{
	$this->objects = new ArrayCollection();
}
```
- **SHOULD**: Всегда указывать `inversedBy` / `mappedBy` для двунаправленных связей
- **SHOULD**: Использовать `fetch: 'LAZY'` (по умолчанию) для предотвращения N+1 запросов
- **SHOULD**: Использовать `#[Ignore]` на связях, которые не должны сериализоваться в API

## Сериализация (API-ответы)
- **MAY**: Использовать `#[Groups(['public'])]` для выбора полей API
- **MAY**: Использовать `#[SerializedName('custom_name')]` для алиасов полей
- **MAY**: Использовать `#[Ignore]` для исключения из сериализации
- TODO сериализия - проверь, как в crm реализована серилизация например, на продуктах, закзаах, как там преобразуются даты и прочее через Context. это нужно делтельно описать для LLM
```php
# пример, через контекст
/** DECIMAL в ORM — string; в JSON — float через {@see ProductSerializerContext}. */
#[Context(
    normalizationContext: ProductSerializerContext::SALES_PRICE_NORMALIZATION,
    denormalizationContext: ProductSerializerContext::SALES_PRICE_DENORMALIZATION,
)]
#[Groups(['default'])]
#[ORM\Column(type: Types::DECIMAL, precision: 10, scale: 2, nullable: true, options: ['comment' => 'цена продажи', 'unsigned' => true])]
private ?string $sales_price = null;
```


## Антипаттерн
```php
// НЕПРАВИЛЬНО: нет имени таблицы, нет комментариев, мутабельный DateTime, строковый тип
#[ORM\Entity]
class Product
{
	#[ORM\Column(type: 'datetime')]  // НЕПРАВИЛЬНО: строковый тип, не Types::*
	private ?\DateTime $created;     // НЕПРАВИЛЬНО: мутабельный DateTime
}
```
