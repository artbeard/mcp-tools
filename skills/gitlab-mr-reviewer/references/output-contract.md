# Контракт вывода

Используй эту детерминированную форму для каждого результата ревью.

## SKIP

```json
{
	"status": "SKIP",
	"merge_requests": [
		{
			"project_id": "group/project",
			"iid": 123,
			"url": "https://...",
			"reason": "no_findings"
		}
	],
	"checked": {
		"sources": ["mr_metadata", "changed_files", "diffs", "discussions", "codestyle"],
		"limitations": []
	}
}
```

Допустимые значения `reason`:

- `no_findings`
- `no_code_changes`
- `unsupported_input`
- `missing_mr_access`
- `tool_error`

## FINDINGS

```json
{
	"status": "FINDINGS",
	"merge_requests": [
		{
			"project_id": "group/project",
			"iid": 123,
			"url": "https://...",
			"findings": [
				{
					"priority": "BLOCKER",
					"file": "src/Controller/ExampleController.php",
					"line": 42,
					"title": "API-ответ обёрнут в success/data",
					"evidence": "return $this->json(['success' => true, 'data' => $result], 200);",
					"rule": "codestyle://backend/controller",
					"fix": "Вернуть данные напрямую и использовать Response::HTTP_*."
				}
			]
		}
	],
	"checked": {
		"sources": ["mr_metadata", "changed_files", "diffs", "discussions", "codestyle"],
		"limitations": []
	}
}
```

## Правила полей

- `status` MUST быть ровно `SKIP` или `FINDINGS`.
- `priority` MUST быть одним из `BLOCKER`, `MAJOR`, `MINOR`, `NIT`.
- `file` MUST быть путём изменённого файла из diff MR.
- `line` SHOULD быть номером новой строки из diff, если он доступен; используй `null` только для замечаний ко всему файлу.
- `evidence` MUST цитировать или кратко описывать конкретный фрагмент diff.
- `rule` MUST ссылаться на codeStyle URI, `dev-checklist`, `reviewer-history` или `reviewer-history:<pattern>`.
- `fix` MUST быть одним actionable-предложением.
- Порядок замечаний MUST быть стабильным: порядок priority, затем путь файла, затем строка.
