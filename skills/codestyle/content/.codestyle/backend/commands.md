# Правила консольных команд

## Объявление
- **MUST**: Использовать атрибут `#[AsCommand]` (Symfony 7.4+)
- **MUST**: Наследоваться от `Symfony\Component\Console\Command\Command`
- **SHOULD**: Использовать `SymfonyStyle` для форматированного ввода-вывода
```
я бы тут  добавил еще один параметр verbose и выводил через SymfonyStyle только при добавлении:
-v: уроверь логов Warninng, Note, error, caotion
-vv: уровень логов write, info и проч..
-vvv: уровень debug - вывод подробносетй, данных для отладки и прочее

если verbose не передан - вывод осуществлять в logger PSR
```

## Структура
```php
use Symfony\Component\Console\Attribute\AsCommand;
use Symfony\Component\Console\Command\Command;
use Symfony\Component\Console\Style\SymfonyStyle;

#[AsCommand(
	name: 'app:process-data',
	description: 'Обработка данных маркетплейсов'
)]
class ProcessDataCommand extends Command
{
	public function __construct(
		private readonly DataService $dataService
	)
	{
		parent::__construct();
	}

	protected function configure(): void
	{
		$this->addOption('account_id', null, InputOption::VALUE_OPTIONAL, 'ID аккаунта');
		$this->addOption('dry_run', null, InputOption::VALUE_NONE, 'Пробный запуск');
	}

	protected function execute(InputInterface $input, OutputInterface $output): int
	{
		$io = new SymfonyStyle($input, $output);

		try
		{
			$this->dataService->setLog($io);
			$this->dataService->process();
			$io->success('Completed');
			return Command::SUCCESS;
		}
		catch (\Exception $e)
		{
			$io->error('Error: ' . $e->getMessage());
			return Command::FAILURE;
		}
	}
}
```

## Правила
- **MUST**: Описание на русском языке
- **MUST**: Возвращать `Command::SUCCESS` или `Command::FAILURE`
- **MUST**: Перехватывать исключения в `execute()` для корректного завершения
- **SHOULD**: Поддерживать `--dry_run` для режима предпросмотра
- **SHOULD**: Использовать `LockableTrait` для команд с мьютексом
- **MAY**: Использовать `CommandServiceTrait` для управления аккаунтами (парсер-проекты)

## Именование
- Имя команды: `app:verb-noun` или `parser:action`
- Имя класса: `{Action}Command` (`RefreshFinanceRealtimeCommand`)
