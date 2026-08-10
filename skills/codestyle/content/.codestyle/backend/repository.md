# Правила репозиториев

## Базовый класс
- **MUST**: Наследоваться от `ServiceEntityRepository`
- **MUST**: Конструктор вызывает `parent::__construct($registry, Entity::class)`

## Подходы к запросам

### QueryBuilder (предпочтительно для результатов сущностей)
```php
public function findGroupsWithoutBarcodes(): array
{
	return $this->createQueryBuilder('f')
		->andWhere('f.status = :status')
		->andWhere("f.barcode IS NULL OR f.barcode = ''")
		->setParameter('status', FboGroupStatus::Plan->value)
		->getQuery()
		->getResult()
	;
}
```

### Нативный SQL с ResultSetMapping (для сложных JOIN-ов)
```php
use Doctrine\ORM\Query\ResultSetMappingBuilder;

public function loadPlansByProduct(int $globalProductId): array
{
	$rsm = new ResultSetMappingBuilder($this->_em);
	$rsm->addRootEntityFromClassMetadata(SalesPlan::class, 'p');

	$sql = <<<SQL
		SELECT {$rsm->generateSelectClause()}
		FROM mp_core__sales_plan p
		LEFT JOIN wb_core__card wcc ON wcc.card_id = p.card_id
		WHERE wcc.global_product_id = :globalProductId
	SQL;

	$query = $this->_em->createNativeQuery($sql, $rsm);
	$query->setParameter('globalProductId', $globalProductId);

	return $query->getResult();
}
```

## Правила
- **MUST**: Именованные параметры (`:param`), не позиционные (`?`)
- **SHOULD**: QueryBuilder для простых запросов сущностей
- **SHOULD**: Нативный SQL для критичных по производительности агрегаций
- **MAY**: Возвращать массивы DTO для сложных запросов (см. `database.md`)

## Антипаттерн
```php
// НЕПРАВИЛЬНО: сырой SQL без привязки параметров
$this->_em->getConnection()->executeQuery(
	"SELECT * FROM users WHERE id = $id"  // SQL-инъекция!
);
```
