# Docker & Docker Compose Roadmap
## Что ты будешь уметь после этого роадмапа:
- Объяснить разницу между образом, контейнером и слоями.
- Написать оптимальный Dockerfile для Go-сервиса.
- Собрать минимальный образ через multi-stage + distroless/alpine non-root.
- Понимать CGO_ENABLED=0 и статическую линковку.
- Работать с монорепозиториями через go.work в Docker.
- Описать мультисервисное окружение в docker-compose.
- Настроить networks, volumes, healthchecks, depends_on.
- Настроить graceful shutdown на уровне Compose (stop_grace_period).
- Настроить hot reload через docker compose watch или Air.
- Знать, как делать healthcheck для distroless/scratch.
- Решать проблемы прав доступа на volumes под non-root.
- Знать, когда Compose достаточно, а когда нужен Kubernetes.

## 🟢 БЛОК 1: ОСНОВЫ DOCKER
### 1.1. Образ vs Контейнер vs Слои
**Логика:** Образ — неизменяемый шаблон (класс). Контейнер — запущенный экземпляр (объект). Образ состоит из слоёв — каждая инструкция RUN/COPY создаёт новый слой. Слои кэшируются и переиспользуются, поэтому команды объединяют.
**Что учить:** `docker pull/run/ps/rm/rmi/images/logs/exec/inspect/stats`.
**Связь с Go:** Один образ → N контейнеров (реплик). Правильный порядок слоёв = быстрая пересборка.

### 1.2. Dockerfile: синтаксис
**Логика:** Dockerfile — декларативное описание сборки. Понимание каждой инструкции — база для оптимизации.
**Что учить:** FROM, WORKDIR, COPY, ADD, RUN, ENV, ARG, EXPOSE, CMD, ENTRYPOINT, USER, LABEL.
**Связь с Go:** Классический паттерн для кэша: `COPY go.mod go.sum` → `RUN go mod download` → `COPY . .` → `RUN go build`.

### 1.3. CMD vs ENTRYPOINT
**Логика:** CMD — дефолтные аргументы, ENTRYPOINT — сама команда. Разница критична для graceful shutdown.
**Что учить:** exec-форма (`["bin"]`) vs shell-форма (`bin arg`). Exec-форма пропускает сигналы напрямую процессу.
**Связь с Go:** `ENTRYPOINT ["/app"]` в exec-форме — Go-сервис получает SIGTERM и корректно завершается.

### 1.4. Управление контейнером
**Логика:** Контейнер — это процесс с изоляцией. Его можно остановить, перезапустить, ограничить ресурсами.
**Что учить:** `docker stop/start/restart`, `docker logs -f`, `docker exec -it`, `docker inspect`, `docker stats`. Restart policies: `no`, `always`, `on-failure`, `unless-stopped`.
**Связь с Go:** Отладка через `docker exec -it` (для alpine) или debug-образ (для distroless).

## 🟡 БЛОК 2: MULTI-STAGE И МОНОРЕПО
### 2.1. Multi-stage builds
**Логика:** Компилятор Go нужен только для сборки. В рантайме — только бинарник. Multi-stage разделяет build-среду и рантайм: образ уменьшается в 40 раз.
**Что учить:** Несколько `FROM` в одном Dockerfile. `FROM golang:1.22 AS builder` → `FROM scratch` → `COPY --from=builder /app/app /app`.
**Связь с Go:** Без multi-stage — 800 МБ, с ним — 10-20 МБ.

### 2.2. Сравнение базовых образов
**Логика:** Размер влияет на скорость pull, стоимость registry и attack surface.
**Что учить:**
- `ubuntu` — 80 МБ, полный Linux, для legacy.
- `alpine` — 5 МБ, musl libc, есть shell. Для dev/staging с non-root: `RUN adduser -D -u 1000 appuser && USER appuser`.
- `gcr.io/distroless/static` — 2 МБ, без shell, для строгого прода.
- `scratch` — 0 МБ, только статические бинарники.
**Связь с Go:** Для dev/staging — alpine с non-root. Для прода — distroless или scratch. Отладка distroless без shell — боль, поэтому в dev её не используют.

### 2.3. CGO_ENABLED=0 и cross-compilation
**Логика:** CGO может тянуть glibc и создавать динамическую линковку. CGO_ENABLED=0 — статическая линковка, работает на любом Linux. Go умеет компилироваться под любую платформу через переменные GOOS/GOARCH.
**Что учить:** `CGO_ENABLED=0 go build`. Флаги линкера: `-ldflags="-s -w"` (убирает символы), `-trimpath` (убирает пути). Для ARM: `GOOS=linux GOARCH=arm64 go build`.
**Связь с Go:** CGO_ENABLED=0 — обязательное условие для distroless и scratch. Cross-compile без QEMU — быстрее эмуляции.

### 2.4. .dockerignore и cache mounts
**Логика:** .dockerignore уменьшает build context. Cache mounts монтируют кэш сборки между билдами.
**Что учить:** .git, vendor, *.md в .dockerignore. BuildKit: `RUN --mount=type=cache,target=/root/.cache/go-build` и `/go/pkg/mod`.
**Связь с Go:** Порядок: go.mod → go mod download → COPY . . Без этого любое изменение кода = пересборка всех зависимостей.

### 2.5. Go Workspaces (go.work) и монорепозитории
**Логика:** В микросервисах часто используют монорепо с go.work или shared-библиотеками. Классический `COPY go.mod go.sum` ломается — Go не находит локальные модули.
**Что учить:**
- Как пробрасывать build context в монорепо (контекст — корень репо, а не отдельный сервис).
- Копировать манифесты всех модулей: `COPY go.work go.work.sum ./`, `COPY services/order/go.mod services/order/go.sum ./services/order/`, `COPY shared/go.mod shared/go.sum ./shared/`.
- `RUN go work sync` и `RUN go mod download` на уровне workspace.
- Как не сломать Docker-кэш: сначала копируем ВСЕ go.mod и go.sum, потом download, только потом код.
**Связь с Go:** В монорепо правильный Dockerfile копирует манифесты всех модулей до COPY кода. Иначе при изменении одного сервиса — пересборка всего.

## 🟠 БЛОК 3: БЕЗОПАСНОСТЬ ОБРАЗОВ
### 3.1. Non-root пользователь и права на volumes
**Логика:** По умолчанию контейнер работает под root. Пробили сервис — root внутри. Не должен.
**Что учить:**
- `USER 1000:1000` в Dockerfile.
- Alpine: `RUN adduser -D -u 1000 appuser && USER appuser`.
- Distroless `nonroot`.
- **Проблема прав:** при bind mount с хоста под USER 1000 часто ловишь Permission denied. Решения:
  - `chown -R 1000:1000 /path` в этапе сборки до переключения на USER.
  - На хосте: `sudo chown -R 1000:1000 ./data` (совпасть с UID в контейнере).
  - Named volume с предварительным `chown` через init-контейнер.
  - `user: "${UID}:${GID}"` в compose — использовать UID хоста.
**Связь с Go:** Порт 8080 вместо 80 — не требует root. Для монтирования data-директорий — либо chown в Dockerfile, либо init-контейнер.

## 🔴 БЛОК 4: DOCKER COMPOSE — ОСНОВЫ
### 4.1. Что такое Compose и зачем
**Логика:** Compose — декларативный YAML для запуска нескольких контейнеров. Один `docker compose up` — весь локальный стек.
**Что учить:** `compose.yaml`. Команды: `up`, `down`, `ps`, `logs`, `exec`, `build`, `restart`, `watch`. `down -v` для удаления volumes.
**Связь с Go:** Локальный запуск сервиса + БД + Kafka + Redis — одной командой.

### 4.2. Структура файла и сервисы
**Логика:** Верхние ключи: `services`, `networks`, `volumes`. Всё остальное — внутри сервисов.
**Что учить:** `version:` устарел. `services: { myservice: { image/build, ports, environment, command, depends_on, stop_grace_period, stop_signal } }`.
**Связь с Go:** Go-сервис, Postgres, Kafka — три сервиса в одном compose.

### 4.3. Порты и expose
**Логика:** `ports` публикует наружу, `expose` — только для внутренней сети. Ошибка = утечка.
**Что учить:** `ports: ["8080:8080"]` (хост:контейнер). `expose: [8080]`. Разница между `127.0.0.1:8080` и `0.0.0.0:8080`.
**Связь с Go:** Наружу только API-gateway. Внутренние — только expose.

## 🟣 БЛОК 5: NETWORKS, VOLUMES И HOT RELOAD
### 5.1. Networks
**Логика:** Compose создаёт default-сеть. Внутри неё сервисы видят друг друга по имени сервиса (DNS).
**Что учить:** Default network, aliases. `docker network ls/inspect`. Несколько сетей для изоляции: `frontend-net`, `backend-net`.
**Связь с Go:** `http://postgres:5432` вместо `localhost:5432` — вот в чём магия Compose. API в обеих сетях, Postgres — только backend.

### 5.2. Volumes, права и hot reload
**Логика:** Named volume — управляется Docker, живёт независимо. Bind mount — папка хоста, видная в контейнере.
**Что учить:**
- `volumes: { db-data: }` vs `volumes: ["./src:/app/src"]`.
- Права доступа (uid/gid). Для bind mount под non-root — либо `chown -R 1000:1000` в этапе сборки, либо `user: "${UID}:${GID}"` в compose.
- **Hot reload:** современный стандарт — `docker compose watch` (встроен в Compose v2.22+). Ничего дополнительно ставить не надо, работает через `develop.watch` в compose.yaml. Альтернатива — `cosmtrek/air` внутри контейнера.
- Три режима watch: `sync` (копирует изменения), `rebuild` (пересобирает образ), `sync+restart` (копирует и перезапускает).
**Связь с Go:** Для dev — `docker compose watch` + bind mount исходников. `develop.watch.action: sync` для .go файлов, `rebuild` для go.mod. Air — если нужен полный контроль над перезапуском.

### 5.3. Override-файлы и .env
**Логика:** База — общая. Override — локальные настройки. .env — секреты (не коммитить).
**Что учить:** `docker-compose.override.yml` (автоматический merge). `-f base.yml -f prod.yml`. `.env` в .gitignore.
**Связь с Go:** `os.Getenv("DB_HOST")` в коде. В compose — `environment: { DB_HOST: postgres }`.

## 🟤 БЛОК 6: HEALTHCHECKS, DEPENDS_ON И GRACEFUL SHUTDOWN
### 6.1. Healthcheck и его особенности для distroless
**Логика:** Docker проверяет, жив ли сервис. Без healthcheck depends_on не знает, что сервис готов.
**Что учить:**
- `healthcheck: { test, interval, timeout, retries }`.
- Для обычных образов: `pg_isready`, `redis-cli ping`, `curl -f http://localhost:8080/health`.
- **Проблема distroless/scratch:** нет curl, wget, sh, bash. Классический healthcheck не работает.
- **Решения для distroless/scratch:**
  1. **Встроенный healthcheck в Go-бинарник:** добавить команду `app healthcheck`, которая дёргает `/health` и возвращает exit code. В compose: `test: ["CMD", "/app", "healthcheck"]`.
  2. **Отключить healthcheck в Dockerfile и перенести в оркестратор:** в Kubernetes — `httpGet` probe на уровне пода.
  3. **TCP-проверка через /dev/tcp (только если есть bash):** `test: ["CMD-SHELL", "exec 3<>/dev/tcp/127.0.0.1/8080"]` — для alpine с bash, не для distroless.
- **Рекомендация:** для distroless/scratch — вариант 1 (встроенный probe в бинарник).
**Связь с Go:** В Go-сервисе добавляешь подкоманду `app healthcheck`, которая делает HTTP GET на localhost:8080/health. В compose `test: ["CMD", "/app", "healthcheck"]`. Работает и в distroless.

### 6.2. depends_on: короткий и длинный
**Логика:** Короткий `depends_on: [db]` ждёт только старта контейнера. Длинный — ждёт прохождения healthcheck.
**Что учить:** `depends_on: { db: { condition: service_healthy } }`. Условия: `service_started`, `service_healthy`, `service_completed_successfully`.
**Связь с Go:** API ждёт `service_healthy` для Postgres. Миграции — `service_completed_successfully`.

### 6.3. Graceful Shutdown на уровне Compose
**Логика:** По умолчанию `docker compose stop` ждёт 10 секунд, потом SIGKILL. SIGKILL нельзя перехватить в Go — твой graceful shutdown не докрутится.
**Что учить:**
- `stop_grace_period: 30s` — сколько ждать перед SIGKILL.
- `stop_signal: SIGTERM` — какой сигнал послать первым.
- Порядок: SIGTERM → приложение завершает работу → если не успело за stop_grace_period → SIGKILL.
**Связь с Go:** Без `stop_grace_period: 30s` при деплое твой `srv.Shutdown(ctx)` может не успеть закрыть соединения. SIGKILL в Go перехватить НЕЛЬЗЯ. Настрой stop_grace_period под свой worst-case shutdown.

### 6.4. Retry в приложении
**Логика:** Даже с healthcheck сервис может быть не готов сразу. Нужен retry в коде.
**Что учить:** Retry-логика подключения к БД, `wait-for-it.sh`, exponential backoff.
**Связь с Go:** Библиотека для retry-подключения, чтобы сервис не падал при старте.

## 🟠 БЛОК 7: COMPOSE НЕ ДЛЯ ПРОДА
**Логика:** Compose — для локальной разработки, CI и небольших деплоев. В проде — Kubernetes, Nomad, ECS.
**Что учить:** Ограничения Compose: нет HA, нет автоскейла, один хост. Docker Swarm устаревает.
**Связь с Go:** Compose для dev, k8s для прода — стандартный путь.

## 🔵 БЛОК 8: ПРАКТИКА
### 8.1. Dockerfile для Go-сервиса (production-ready)
**Логика:** Собираем всё вместе — правильный порядок слоёв, multi-stage, distroless, non-root.
**Что написать:**
- golang:1.22-alpine AS builder.
- CGO_ENABLED=0, -ldflags="-s -w" -trimpath.
- Cache mounts для go-build и go/pkg/mod.
- ARG TARGETOS TARGETARCH для cross-compile.
- FROM distroless/static:nonroot (или alpine + adduser для dev).
- USER nonroot.
- Минимальный .dockerignore.
**Ожидаемый результат:** образ < 20 МБ, время сборки < 30 сек после первого раза, работает на amd64 и arm64.

### 8.2. Dockerfile для монорепо (go.work)
**Логика:** Копируем манифесты ВСЕХ модулей до download, иначе кэш ломается.
**Что написать:**
- COPY go.work go.work.sum ./
- COPY services/order/go.mod services/order/go.sum ./services/order/
- COPY shared/go.mod shared/go.sum ./shared/
- RUN go work sync && go mod download
- COPY . .
- RUN go build ./services/order/cmd/...
**Ожидаемый результат:** изменение кода одного сервиса не пересобирает зависимости других.

### 8.3. Compose для маркетплейса
**Логика:** 4 Go-сервиса + Postgres + Redis + Kafka + Jaeger. Правильные сети, healthchecks, graceful shutdown, hot reload.
**Что написать:**
- api-gateway в frontend + backend.
- auth/order/notification — только backend.
- Postgres, Redis, Kafka — только backend, с healthchecks.
- Jaeger — отдельный сервис, запускается когда нужен.
- Volumes для Postgres.
- Override-файл для локальных портов.
- `stop_grace_period: 30s` для всех Go-сервисов.
- `develop.watch` для hot reload через `docker compose watch`.
- Healthcheck через встроенный `app healthcheck` (для distroless).
**Что покрутить:** `up`, `down -v`, `watch` — hot reload.

## КЛЮЧЕВЫЕ ВЫВОДЫ
1. Docker — инструмент сборки и запуска. Compose — оркестратор для локальной разработки. Одна тема.
2. Образ — шаблон, контейнер — экземпляр. Слои кэшируются, порядок инструкций влияет на скорость.
3. Multi-stage разделяет build и runtime. Образ уменьшается в 40 раз.
4. CGO_ENABLED=0 — обязательное условие для distroless и scratch. Go cross-compile через GOOS/GOARCH.
5. Distroless — для строгого прода. Alpine + non-root — для dev/staging (легче отлаживать).
6. .dockerignore и cache mounts — must-have для скорости сборки.
7. Монорепо с go.work: копируй манифесты ВСЕХ модулей до download, иначе кэш ломается.
8. Default network в Compose даёт DNS по имени сервиса. `postgres:5432` вместо localhost.
9. Изоляция сетей: frontend-net и backend-net.
10. Named volumes для постоянных данных. Bind mounts — для hot reload.
11. **Hot reload:** `docker compose watch` — современный стандарт без внешних бинарников. Air — если нужен полный контроль.
12. **Права на volumes под non-root:** `chown -R 1000:1000` в сборке или `user: "${UID}:${GID}"` в compose.
13. Healthcheck + depends_on с condition — правильное ожидание готовности.
14. **Healthcheck для distroless/scratch:** нет curl/sh — встроенный `app healthcheck` в Go-бинарник.
15. `stop_grace_period: 30s` обязателен, иначе SIGKILL убьёт Go-сервис до завершения graceful shutdown.
16. Override-файлы + .env — базовый набор для разных окружений.
17. Non-root — базовый security baseline.
18. Exec-форма ENTRYPOINT нужна для graceful shutdown.
19. Compose НЕ для прода. Он для dev, CI и небольших деплоев.
20. `down -v` — снести всё включая volumes. `down` — сохранить данные.
21. Compose — входная точка для нового разработчика. Должна работать из коробки.