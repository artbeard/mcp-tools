# Конфигурация ESLint

## Формат конфигурации
- **MUST**: Flat config (`eslint.config.js`) — ESLint 9+ (legacy `.eslintrc.*` не используется)
- **MUST NOT**: Использовать legacy-конфигурацию (`.eslintrc.js`, `.eslintrc.json`)

## Основные правила
```javascript
const commonStyleRules = {
	'indent': ['error', 'tab', { ImportDeclaration: 1, CallExpression: { arguments: 1 } }],
	'custom/no-trailing-spaces-except-tabs': 'error',
	'eol-last': 'error',
	'no-multiple-empty-lines': ['error', { max: 2 }],
	'curly': ['error', 'multi-line', 'consistent'],
	'brace-style': ['error', 'allman'],
	'semi': ['error', 'always'],
	'quotes': ['error', 'single', { avoidEscape: true, allowTemplateLiterals: true }],
	'camelcase': ['error', { properties: 'never', ignoreDestructuring: true }],
	'linebreak-style': ['error', 'unix'],
	'object-curly-spacing': ['error', 'always'],
};
```

## Кастомные правила
- `custom/no-trailing-spaces-except-tabs` — нет пробелов в конце строк, табы допускаются
- **MUST NOT**: Добавлять кастомное правило для brace style; использовать встроенное ESLint-правило `'brace-style': ['error', 'allman']`

## Игнорируемые пути
- `node_modules/`, `dist/`, `.vscode/`, `coverage/`, `*.min.js`, `public/`

## Запуск
```bash
npm run lint          # Проверка + автоисправление
npm run lint:watch    # Режим наблюдения
npm run dev:lint      # Dev с автопроверкой
```

## Требование перед MR
- **MUST**: Запустить `npm run dev:lint` перед созданием Merge Request
