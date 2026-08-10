# Правила Symfony Messenger

## Класс сообщения
- **MUST**: Размещать в `src/Message/`
- **MUST**: Простой контейнер данных с конструкторными свойствами
- **SHOULD**: Использовать readonly-свойства
- TODO опиши кейс работы с rabbitmq, мы скоро на него будем переходить


```php
namespace App\Message;

class RefreshSalesPlanMessage
{
	public function __construct(
		readonly public string $date,
		readonly public int|null $cardId = null,
		readonly public string|null $mp = null
	)
	{
	}
}
```

## Класс обработчика
- **MUST**: Размещать в `src/MessageHandler/`
- **MUST**: Использовать атрибут `#[AsMessageHandler]`
- **MUST**: Реализовать метод `__invoke(MessageClass $message)`
- **SHOULD**: Публиковать Mercure-обновления для обратной связи в реальном времени
- **SHOULD**: Управлять памятью для крупных операций (`gc_collect_cycles()` выше 255МБ)

```php
namespace App\MessageHandler;

use Symfony\Component\Messenger\Attribute\AsMessageHandler;

#[AsMessageHandler]
class RefreshSalesPlanMessageHandler
{
	public function __construct(
		private readonly SalePlanService $service,
		private readonly HubInterface $hub
	)
	{
	}

	public function __invoke(RefreshSalesPlanMessage $message): void
	{
		$this->service->calculatePlanForCard($message->cardId);

		// Публикация Mercure-обновления
		$this->hub->publish(new Update('sales-plan', json_encode([
			'type' => 'refresh_complete',
			'cardId' => $message->cardId
		])));
	}
}
```

## Конфигурация транспорта (`messenger.yaml`)
```yaml
framework:
  messenger:
    transports:
      async:
        dsn: '%env(MESSENGER_TRANSPORT_DSN)%'
        retry_strategy:
          max_retries: 3
          multiplier: 2
    routing:
      'App\Message\RefreshSalesPlanMessage': async
```

## Правила
- **MUST**: Один обработчик на класс сообщения
- **SHOULD**: Логировать ошибки в `var/log/` для отладки
- **SHOULD**: Делать flush EntityManager и собирать мусор в долгоживущих обработчиках
