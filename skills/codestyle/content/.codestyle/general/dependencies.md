# Управление зависимостями

## Процесс согласования
- **MUST**: Новая зависимость MUST обсуждаться на дейли/груминге или в командном чате
- **MUST NOT**: Самовольное добавление зависимостей без предварительного согласования
- **SHOULD**: Проверить, есть ли альтернатива в существующем стеке, перед предложением новой зависимости
- **SHOULD**: Обосновать необходимость кратким объяснением

## Нарушение
- Несогласованные зависимости будут отклонены на код-ревью
- При сомнениях — спроси команду

## Фиксация версий
- **MUST**: Зависимости фиксируются через `composer.lock` (PHP) и `package-lock.json` (JS)
- **MUST NOT**: Коммитить изменения в lock-файлах без проверки диффа

## Справка по текущему стеку
Backend: Symfony 7.4+, Doctrine ORM, Passport SSO + Lexik JWT (verify-only), Symfony Messenger, Mercure
Frontend: Vue 3, Pinia, Vue Router, ky (новые) / Axios (legacy), jwt-decode, Vite, PrimeVue, Tailwind CSS, ag-grid, Vuelidate
Auth: docs auth проекта (SSO/JWT)
