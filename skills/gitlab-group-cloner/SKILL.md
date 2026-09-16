---
name: gitlab-group-cloner
description: Получение списка и клонирование всех доступных репозиториев GitLab-группы и её подгрупп по HTTPS или SSH для локального анализа. Для API требуется GITLAB_TOKEN; HTTPS использует также GITLAB_USERNAME.
---
# GitLab Group Cloner

Используй локальные `GITLAB_BASE_URL` и `GITLAB_TOKEN`. Для HTTPS нужен также `GITLAB_USERNAME`; для SSH доступ к репозиториям настраивается обычными средствами Git/SSH (ключ, `ssh-agent`, `~/.ssh/config`). Сначала запускай `mode=list`, затем после явного подтверждения — `mode=clone`.

CLI: `bin/gitlab-group-clone` (сборка: `make build-gitlab-group-clone`).

Передавай `protocol=https` для HTTPS-клонирования с токеном или `protocol=ssh` для SSH-клонирования. При HTTPS токен никогда не печатается, не попадает в URL и не сохраняется в `.git/config`: CLI использует временный `GIT_ASKPASS`. При SSH CLI не передаёт токен процессу Git. Уже существующие репозитории пропускаются, обычные непустые каталоги не перезаписываются.
