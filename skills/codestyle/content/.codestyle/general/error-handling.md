# Правила обработки ошибок

## Основной принцип
Ошибки MUST всплывать. Перехватывай исключения только когда нужно **изменить поведение программы**, а не для подавления или логирования.
важно! никогда не подавлять ошибки с помощью оператора "@"
на бэке - если нужно создать свое исключение - подбери родительское из стека symfony и наследуйся от него.
наследоваться от \Exception нежелательно, т.е. это обработчиком исключений обрабатывается как Internal Server Error
## Backend (PHP / Symfony)

### MUST NOT: Перехват с подавлением
```php
// НЕПРАВИЛЬНО: перехват без изменения логики
public function getUser(int $id): ?User
{
	try
	{
		return $this->userRepository->find($id);
	}
	catch (\Exception $e)
	{
		$this->logger->error($e->getMessage());
		return null; // Скрывает реальную проблему
	}
}
```

### MUST: Пусть исключения всплывают
```php
// ПРАВИЛЬНО: исключение обрабатывается Symfony автоматически
public function getUser(int $id): ?User
{
	return $this->userRepository->find($id);
}
```

### MAY: Перехват для альтернативной логики
```php
// ПРАВИЛЬНО: откат на альтернативный источник
public function getData(int $id): Data
{
	try
	{
		return $this->primaryService->fetch($id);
	}
	catch (ServiceUnavailableException $e)
	{
		return $this->cacheService->fetch($id);
	}
}
```

### MUST: Перехват в консольных командах
```php
// Команды — граница приложения, перехватывай для корректного завершения
protected function execute(InputInterface $input, OutputInterface $output): int
{
	try
	{
		$this->importService->import();
		return Command::SUCCESS;
	}
	catch (\Exception $e)
	{
		$output->writeln('Error: ' . $e->getMessage());
		return Command::FAILURE;
	}
}
```

### MUST NOT: Throw внутри try
```php
// НЕПРАВИЛЬНО: явный throw внутри try-блока
try
{
	$result = $this->service->process();
	throw new \Exception('unexpected'); // Запрещено!
}
catch (\Exception $e) { /* ... */ }
```

## Frontend (Vue / TypeScript)

### MUST NOT: Тихий catch
```typescript
// НЕПРАВИЛЬНО: catch без изменения логики
async function fetchUser(id: number): Promise<User | null>
{
	try
	{
		return await api.getUser(id);
	}
	catch (e)
	{
		console.error(e);
		return null; // Скрывает ошибку
	}
}
```

### MUST: Пусть ошибки пробрасываются
```typescript
// ПРАВИЛЬНО: ошибка попадает в глобальный обработчик
async function fetchUser(id: number): Promise<User>
{
	return await api.getUser(id);
}
```

### MAY: Перехват в обработчиках событий для безопасности UI
```typescript
// ПРАВИЛЬНО: предотвращает падение UI
function handleClick()
{
	try
	{
		performAction();
	}
	catch (e)
	{
		showErrorNotification('Действие не удалось'); //уведомление выводить только в ui, в сторах и глубже по слою это делать запрещено!
	}
}
```

## Где перехват допускается
| Слой | Backend | Frontend |
|------|---------|----------|
| Граница API | Symfony обрабатывает автоматически | Axios-перехватчики |
| Команды/Скрипты | метод `execute()` | — |
| Обработчики событий | — | Обработчики компонентов |
| Логика отката | Альтернативные сервисы | Кэш/повтор |

## Сводка антипаттернов
- **MUST NOT**: `try { ... } catch { log(); return null; }`
- **MUST NOT**: `throw` внутри `try`-блоков
- **MUST NOT**: Ловить `\Exception` когда существует конкретное исключение
- **MUST NOT**: `try/catch` вокруг каждого вызова функции
