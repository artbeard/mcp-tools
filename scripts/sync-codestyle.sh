#!/usr/bin/env bash
# Копирует правила из codestyle-docs и анонимизирует snapshot в skills/codestyle/content.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="${CODESTYLE_DOCS_ROOT:-/home/artbeard/projects/codestyle-docs}"
DST="$ROOT/skills/codestyle/content"

# Банлист после анонимизации (case-insensitive).
BANLIST='abcAge|abcage|mpvibe|yandexcloud|productflow|passport-users|CodestyleGuard|@abcage|productflowfront|productflowback|mprealtime|mpmpmp'

if [[ ! -d "$SRC/.codestyle" ]]; then
	echo "ОШИБКА: нет $SRC/.codestyle (задай CODESTYLE_DOCS_ROOT)" >&2
	exit 1
fi
if [[ ! -f "$SRC/mcp-meta.json" ]]; then
	echo "ОШИБКА: нет $SRC/mcp-meta.json" >&2
	exit 1
fi

rm -rf "$DST"
mkdir -p "$DST"
cp -a "$SRC/.codestyle" "$DST/.codestyle"
cp -a "$SRC/mcp-meta.json" "$DST/mcp-meta.json"
mkdir -p "$DST/prompts"
if [[ -d "$SRC/prompts" ]]; then
	cp -a "$SRC/prompts/." "$DST/prompts/"
fi

# --- анонимизация текстовых файлов ---
anonymize_file() {
	local f="$1"
	# Внутренние GitLab / Passport Guide URL → нейтральная отсылка
	# CRLF → LF
	sed -i 's/\r$//' "$f"

	sed -i -E \
		-e 's|\[[^]]*\]\(https://abcage\.gitlab\.yandexcloud\.net[^)]*\)|docs auth проекта (SSO/JWT)|gI' \
		-e 's|https://abcage\.gitlab\.yandexcloud\.net[^) ]*|docs auth проекта (SSO/JWT)|gI' \
		-e 's|https?://auth(-dev)?\.mpvibe\.ru|https://auth\1.example.com|gI' \
		-e 's|passport\.mpmpmp\.loc|passport.local|gI' \
		-e 's|@abcage/[a-z0-9_-]+|codestyle package|gI' \
		-e 's|CodestyleGuard|codestyle MCP|gI' \
		-e 's|команды abcage|стека PHP/Symfony и Vue/TS|gI' \
		-e 's|команды abcAge|стека PHP/Symfony и Vue/TS|gI' \
		-e 's|проектов abcage|проектов на этом стеке|gI' \
		-e 's|проектах abcage|проектах на этом стеке|gI' \
		-e 's|abcAge|codestyle|gI' \
		-e 's|abcage|codestyle|gI' \
		-e 's|mpvibe|example|gI' \
		"$f"

	# Битые markdown-ссылки после замены URL
	sed -i -E \
		-e 's|\[интеграция\]\(docs auth проекта\)|docs auth проекта (SSO/JWT)|g' \
		-e 's|\[Passport Guide\]\(docs auth проекта[^)]*\)|docs auth проекта (SSO/JWT)|gI' \
		-e 's|Passport Guide|docs auth проекта|gI' \
		"$f"

	# Имена конкретных реп / legacy-примеры проектов
	sed -i -E \
		-e 's/\(productflow\)/(пример проекта)/gI' \
		-e 's/\(manager\)/(пример проекта)/gI' \
		-e 's/\(mpCore\)/(пример проекта)/gI' \
		-e 's/\(wbcore\)/(пример проекта)/gI' \
		-e 's/\(ozon\/mprealtime\)/(пример проекта)/gI' \
		-e 's/\(mprealtime\)/(пример проекта)/gI' \
		-e 's/\(ozon\/?mprealtime\)/(пример проекта)/gI' \
		-e 's/managerfront, wbcore/типичные frontend-репы/gI' \
		-e 's/или `src\/use\/` \(productflow\)/или `src\/use\/` (по конвенции репо)/gI' \
		-e 's/productflow legacy/legacy Options API/gI' \
		-e 's/productflow сейчас имеет/некоторые legacy-репы имеют/gI' \
		-e 's/- productflow: `createWebHistory\(\)` \(чистые URL\)/- History mode: `createWebHistory()` (чистые URL)/gI' \
		-e "s/'ProductFlow'/'App'/g" \
		-e 's/Passport admin/SSO admin/gI' \
		-e 's/заявка в IT \(CORS/заявка в инфраструктуру (CORS/gI' \
		-e 's/Обязательные Resources/Обязательные правила (URI)/g' \
		-e 's/\| Resources \|/| URI правил |/g' \
		-e 's/PHP 8\.2\+ \(фичи 8\.4 разрешены\)/PHP 8.4+/g' \
		-e 's/PHP 8\.2\+/PHP 8.4+/g' \
		-e 's/php:8\.2-fpm/php:8.4-fpm/g' \
		-e 's/Symfony 6\.4–7\.x/Symfony 7.4+/g' \
		-e 's/Symfony 6\.4\+/Symfony 7.4+/g' \
		-e 's/Symfony 6\.4,/Symfony 7.4+,/g' \
		-e 's/Symfony 7\.x/Symfony 7.4+/g' \
		-e 's/\(PHP 8\.0\+\)/(PHP 8.4+)/g' \
		-e 's/\(PHP 8\.2\+\)/(PHP 8.4+)/g' \
		"$f"
	# длинное тире → обычный минус
	perl -i -CSD -pe 's/\x{2014}/-/g' "$f"
}

while IFS= read -r -d '' f; do
	anonymize_file "$f"
done < <(find "$DST" -type f \( -name '*.md' -o -name '*.json' \) -print0)

# Нейтральный bootstrap (полностью переписываем - исходник слишком брендовый).
cat > "$DST/prompts/codestyle-bootstrap.md" <<'EOF'
# Codestyle bootstrap - критичные правила стека

Универсальные соглашения для **PHP/Symfony** и **Vue/TypeScript** (не привязаны к конкретной компании). Используй MCP tools `codestyle_find_rules` и `codestyle_read_rule`.

## Обязательный workflow

**Перед написанием или изменением кода на этом стеке:**

1. Вызови `codestyle_find_rules` с описанием задачи (ru/en). Писать «по памяти» / по общим best practices нельзя - эти правила отличаются от PSR-12 и популярных конвенций.
2. Загрузи через `codestyle_read_rule(uri)` все матчи с `priority="critical"` (обычно 2–3 файла). Только релевантное.
3. Соблюдай уровни RFC 2119: `MUST` / `MUST NOT` = обязательно.

## ТОП-7 критичных правил

1. **Табы** для отступов: 4sp в PHP/JS/TS/SCSS, 3sp в HTML/Twig. Пробелы запрещены.
2. **Allman-скобки** - `{` на новой строке для `if`/`for`/`while`/`foreach`/`function`/класса. Не K&R, не PSR-12.
3. **Entity только через `make:entity`** - не пишем сущности руками. Plain int FK без `ManyToOne`/`JoinColumn`.
4. **`<script setup lang="ts">`** для всех новых Vue-компонентов. Options API в новом коде запрещён.
5. **`strict: true`** в `tsconfig.json`. Никакого `any`. Интерфейсы префиксуем `I*`.
6. **MUST NOT: success/status-обёртки** в API-ответах. HTTP-код = результат.
7. **MUST NOT: прямая инъекция Repository** в сервисы - `EntityManagerInterface`, репозиторий через `$em->getRepository(Entity::class)`.

## Пример

Задача: «Создать API-эндпоинт POST /api/orders».

```
→ codestyle_find_rules({ task: "новый Symfony контроллер с DTO" })
→ codestyle_read_rule("codestyle://backend/controller")
→ codestyle_read_rule("codestyle://backend/dto")
→ codestyle_read_rule("codestyle://patterns/crud-flow")
→ код по прочитанным правилам
```

## Стек

- Backend: PHP 8.4+ / Symfony 7.4+ / Doctrine ORM / MySQL
- Frontend: Vue 3.4+ / TypeScript strict / Pinia / Vite / PrimeVue / Tailwind / SCSS
- Auth: JWT (часто Lexik) + централизованный SSO при наличии; асинхронность: Symfony Messenger
EOF

# Проверка банлиста
if grep -RIiE "$BANLIST" "$DST" >/tmp/codestyle-banlist-hits.txt 2>/dev/null; then
	echo "ОШИБКА: после анонимизации остались запрещённые маркеры:" >&2
	cat /tmp/codestyle-banlist-hits.txt >&2
	exit 1
fi
rm -f /tmp/codestyle-banlist-hits.txt

echo "OK: $DST (анонимизировано, банлист чист)"
