package syntax

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

/*
  УРОК 1.2: DOCKERFILE — СИНТАКСИС И ИНСТРУКЦИИ
  Dockerfile — рецепт сборки образа. Декларативное описание:
  какие файлы взять, что с ними сделать, как запустить результат.
  Docker читает его сверху вниз и выполняет каждую инструкцию,
  создавая новый слой.
  Синтаксис: ИНСТРУКЦИЯ аргументы. Регистр не важен, по конвенции
  КАПСОМ. Комментарии — через #.

  СОДЕРЖАНИЕ:
    ЧАСТЬ 1. БАЗОВЫЕ ИНСТРУКЦИИ
      1.  FROM — с чего начинаем
      2.  WORKDIR — где мы внутри контейнера
      3.  COPY и ADD — что копируем
      4.  RUN — что выполняем при сборке
      5.  ENV и ARG — переменные
      6.  EXPOSE и LABEL — метаданные
      7.  USER — под кем работаем
      8.  CMD и ENTRYPOINT — что запускаем
      9.  exec-форма vs shell-форма
      10. HEALTHCHECK
      11. STOPSIGNAL
      12. SHELL
      13. ONBUILD
      14. VOLUME (в Dockerfile)
      15. ARG перед FROM и scope переменных
      16. BuildKit: cache mounts и секреты
      17. Multi-stage: продвинутые паттерны
      18. Продовый Dockerfile для Go
      19. Антипаттерны
      20. Финальные выводы

  1. FROM — С ЧЕГО НАЧИНАЕМ
  FROM — первая инструкция в любом Dockerfile. Задаёт базовый
  образ, на котором строится всё остальное.

    FROM golang:1.22-alpine

  Что происходит: Docker скачивает образ golang:1.22-alpine
  из registry (если ещё не скачан) и делает его первым слоем.

  ВАРИАНТЫ БАЗОВЫХ ОБРАЗОВ:
    • golang:1.22-alpine    — для сборки Go (внутри Go SDK)
    • alpine:3.19           — маленький Linux для рантайма
    • scratch               — пустой образ, только статический бинарник
    • distroless/static     — без shell, для прода

  MULTI-STAGE: несколько FROM в одном Dockerfile. Каждый FROM
  начинает новый stage.

    FROM golang:1.22-alpine AS builder
    ...
    FROM alpine:3.19
    ...

  Второй stage независим. Чтобы взять файлы из первого,
  используют COPY --from=builder.

  ПРАВИЛА:
    • FROM всегда первый (кроме ARG перед ним — см. раздел 15).
    • Тег образа фиксируй (1.22-alpine, не latest).
    • Для сборки Go — golang:*.
    • Для рантайма — alpine/distroless/scratch.

  2. WORKDIR — ГДЕ МЫ ВНУТРИ КОНТЕЙНЕРА

  WORKDIR задаёт рабочую директорию для последующих RUN, CMD,
  ENTRYPOINT, COPY.
    WORKDIR /app

  Что происходит: создаётся папка /app (если её нет), и все
  последующие команды выполняются относительно неё. COPY . .
  скопирует файлы в /app, а не в корень.

  Можно использовать несколько WORKDIR — каждый относительно
  предыдущего или абсолютный:
    WORKDIR /app
    WORKDIR /app/cmd      ← абсолютный, перезаписывает

  Или:
    WORKDIR /app
    WORKDIR cmd           ← относительный, получится /app/cmd

  ПОЧЕМУ ЭТО ВАЖНО:
    • Без WORKDIR всё копируется в /, получается мусор.
    • С WORKDIR структура чистая, как в проекте.
    • Go-бинарники ищут файлы относительно рабочей директории.

  СОВЕТ: используй абсолютные пути. Относительные WORKDIR
  создают путаницу в больших Dockerfile.

  3. COPY И ADD — ЧТО КОПИРУЕМ
  COPY — копирует файлы из build context (папки, где Dockerfile) в образ.
    COPY go.mod go.sum ./
    COPY . .
    COPY main.go /app/main.go

  Синтаксис: COPY <источник> <назначение>. Источник — относительно
  build context. Назначение — относительно WORKDIR.

  Варианты:
    COPY . .                   — всё из текущей папки в WORKDIR
    COPY go.mod go.sum ./      — только манифесты
    COPY cmd/ /app/cmd/        — папку cmd
    COPY --from=builder /app/server /server   — из другого stage
    COPY --chown=1000:1000 . . — с изменением владельца

  --chown КРИТИЧНО ВАЖЕН ДЛЯ NON-ROOT:
    Если копируешь файлы под root, а потом переключаешься на
    USER 1000, файлы будут принадлежать root:root. Приложение
    не сможет их читать. Используй --chown=1000:1000.

  ADD — то же, НО:
    • Умеет распаковывать tar.gz автоматически.
    • Умеет скачивать файлы по URL.

    ADD https://example.com/file.tar.gz /tmp/

  НИКОГДА НЕ ИСПОЛЬЗУЙ ADD:
    • Распаковка автоматическая — неявное поведение.
    • Скачивание по URL — плохо для воспроизводимости.
    • Кэш ломается при изменении URL.

  ПРАВИЛО: COPY всегда. ADD — никогда.

  ВАЖНО ПРО КЭШ:
  Порядок COPY влияет на кэш. Сначала копируй то, что меняется
  редко (go.mod), потом код:

    COPY go.mod go.sum ./     ← редко меняется
    RUN go mod download       ← кэшируется
    COPY . .                  ← меняется часто

  4. RUN — ЧТО ВЫПОЛНЯЕМ ПРИ СБОРКЕ
  RUN выполняет команду во время сборки образа. Результат
  фиксируется в слое.

    RUN go mod download
    RUN go build -o server .
    RUN apt-get update && apt-get install -y curl

  Каждая RUN = новый слой. Поэтому команды объединяют через &&:

    Плохо (3 слоя):
      RUN apt-get update
      RUN apt-get install -y curl
      RUN rm -rf /var/lib/apt/lists/*

    Хорошо (1 слой):
      RUN apt-get update && \
          apt-get install -y curl && \
          rm -rf /var/lib/apt/lists/*

  Чистка в том же слое — критично. Иначе файлы останутся
  в предыдущем слое и образ не похудеет.

  exec-форма vs shell-форма:
    RUN go build -o server .           ← shell (через /bin/sh -c)
    RUN ["go", "build", "-o", "server", "."]  ← exec

  Для RUN — обычно shell-форма. Удобнее для цепочек команд.

  Для Go:
    RUN go mod download
    RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/server .
    RUN --mount=type=cache,target=/root/.cache/go-build go build ...  ← BuildKit

  5. ENV И ARG — ПЕРЕМЕННЫЕ
  ENV — переменная окружения, доступна и при сборке, и в рантайме.
  Остаётся в образе.

    ENV APP_ENV=production
    ENV PORT=8080
    ENV DB_HOST=postgres

  В контейнере доступна через os.Getenv("PORT"). Можно
  переопределить при запуске:

    docker run -e PORT=9090 my-app

  ARG — аргумент сборки. Доступен ТОЛЬКО во время сборки.
  Не остаётся в образе.

    ARG VERSION=dev
    RUN echo "Building $VERSION"

  Передаётся при сборке:
    docker build --build-arg VERSION=1.2.3 -t my-app .

  РАЗНИЦА:
    ENV:
      • Доступен в рантайме.
      • Остаётся в образе (виден в docker inspect).
      • Для конфигов приложения.

    ARG:
      • Только во время сборки.
      • Не остаётся в образе.
      • Для параметров сборки (версия, платформа, флаги).

  ПРИМЕР ДЛЯ GO:
    ARG TARGETOS=linux
    ARG TARGETARCH=amd64
    ENV APP_VERSION=1.2.3

    RUN GOOS=$TARGETOS GOARCH=$TARGETARCH \
        go build -ldflags="-X main.version=$APP_VERSION" -o /app/server .

  ПРАВИЛО: секреты (пароли, ключи) НЕЛЬЗЯ класть ни в ENV,
  ни в ARG. Они видны в docker history. Используй BuildKit
  secrets.

  6. EXPOSE И LABEL — МЕТАДАННЫЕ
  EXPOSE — документирует, какой порт слушает контейнер.
  НЕ публикует порт, только сообщает.

    EXPOSE 8080

  Что это даёт:
    • Документация в docker inspect.
    • Docker Compose может использовать это.
    • Некоторые инструменты читают EXPOSE.

  Чтобы реально опубликовать порт — нужен -p при run:
    docker run -p 8080:8080 my-app

  LABEL — метаданные образа. Видны в docker inspect.

    LABEL maintainer="me@example.com"
    LABEL version="1.2.3"
    LABEL description="My Go service"

  OCI-стандарт предписывает определённые имена:
    • org.opencontainers.image.source — URL репозитория.
    • org.opencontainers.image.version — версия.
    • org.opencontainers.image.created — дата сборки.
    • org.opencontainers.image.revision — git SHA.

  Trivy, Renovate, другие инструменты читают LABEL для
  сканирования и обновления.

  7. USER — ПОД КЕМ РАБОТАЕМ
  USER задаёт пользователя, под которым работает контейнер.
  По умолчанию — root.

    USER 1000:1000
    USER appuser

  ЗАЧЕМ ЭТО:
    • Безопасность. Если сервис пробьют — злоумышленник не root.
    • Многие k8s-кластеры запрещают root (PodSecurityPolicies).
    • Go-сервису на порту 8080 root не нужен.

  КАК СДЕЛАТЬ non-root:
    В Alpine:
      RUN adduser -D -u 1000 appuser
      USER appuser

    В distroless — уже есть nonroot:
      FROM gcr.io/distroless/static:nonroot
      USER nonroot:nonroot

    Вручную через UID:
      USER 1000:1000

  ВАЖНО: USER применяется ко всем последующим RUN и к CMD.
  Сначала root-команды, потом USER.

  Проблема с volumes:
    Если монтируешь bind mount под USER 1000, файлы на хосте
    должны принадлежать 1000:1000. Иначе Permission denied.

  8. CMD И ENTRYPOINT — ЧТО ЗАПУСКАЕМ
  CMD — дефолтные аргументы. Легко переопределить при run.
  ENTRYPOINT — сама команда. Переопределить сложнее.

    CMD ["./server"]
    ENTRYPOINT ["./server"]

  КАК ОНИ РАБОТАЮТ ВМЕСТЕ:

    ENTRYPOINT ["/app"]
    CMD ["--port", "8080"]

  Результат при docker run my-app:
    /app --port 8080

  При docker run my-app --port 9090:
    /app --port 9090    ← CMD переопределён, ENTRYPOINT сохранён

  При docker run --entrypoint /bin/sh my-app:
    /bin/sh --port 8080  ← ENTRYPOINT переопределён через флаг

  КОГДА ЧТО ИСПОЛЬЗОВАТЬ:
    • CMD — дефолтная команда, легко переопределить.
      Для generic-образов (nginx, alpine).
    • ENTRYPOINT — контейнер для одной конкретной задачи.
      Для твоего Go-сервиса.
    • ENTRYPOINT + CMD — дефолтные аргументы, но команда
      фиксированная. Стандарт для CLI-утилит.

  ДЛЯ GO-СЕРВИСА:
    ENTRYPOINT ["/app"]

  9. EXEC-ФОРМА VS SHELL-ФОРМА
  SHELL-ФОРМА (без скобок):

    CMD go run main.go
    ENTRYPOINT /app

  Docker оборачивает в /bin/sh -c:

    CMD ["/bin/sh", "-c", "go run main.go"]

  ПРОБЛЕМА: PID 1 — это /bin/sh, а не твой процесс. Сигналы
  (SIGTERM) идут шеллу, не приложению. Graceful shutdown
  не работает.

  EXEC-ФОРМА (массив строк):

    CMD ["go", "run", "main.go"]
    ENTRYPOINT ["/app"]

  Процесс запускается напрямую, без шелла. PID 1 — твой
  процесс. SIGTERM идёт приложению.

  ПРАВИЛО ДЛЯ GO:
    • ENTRYPOINT — ВСЕГДА exec-форма.
    • CMD — ВСЕГДА exec-форма.
    • RUN — можно shell-форму (там && удобны).

  Пример неправильного:

    ENTRYPOINT /app              ← shell, PID 1 = /bin/sh
    CMD /app --port 8080         ← то же самое

  Следствие: при docker stop Docker посылает SIGTERM шеллу.
  Шелл не пробрасывает его приложению. Docker ждёт 10 секунд
  и убивает через SIGKILL. Graceful shutdown не срабатывает.

  10. HEALTHCHECK
  HEALTHCHECK задаёт команду, которая проверяет, здоров ли
  контейнер. Docker запускает её периодически.

    HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
      --retries=3 \
      CMD curl -f http://localhost:8080/health || exit 1

  Параметры:
    • --interval=30s — как часто проверять (дефолт 30s).
    • --timeout=3s — сколько ждать ответа (дефолт 30s).
    • --start-period=5s — грейс-период при старте (дефолт 0).
    • --retries=3 — сколько провалов до статуса unhealthy.

  Статусы контейнера:
    • starting — до первого успешного чека.
    • healthy — чек проходит.
    • unhealthy — retries провалов подряд.

  ПРОБЛЕМА С DISTROLESS/SCRATCH:
    Нет curl, wget, sh. Классический healthcheck не работает.

  РЕШЕНИЯ:
    1. Встроенная команда в Go-бинарник:
       HEALTHCHECK CMD ["/app", "healthcheck"]
    2. Выключить HEALTHCHECK и использовать k8s probes.
    3. TCP-проверка через /dev/tcp (только если есть bash).

  11. STOPSIGNAL
  STOPSIGNAL задаёт, какой сигнал посылать контейнеру при
  docker stop. По умолчанию SIGTERM.

    STOPSIGNAL SIGTERM
    STOPSIGNAL SIGINT
    STOPSIGNAL SIGQUIT

  КОГДА НУЖНО:
    • Если приложение слушает не SIGTERM, а другой сигнал
      для graceful shutdown (например, nginx исторически
      любил SIGQUIT).
    • Для Go — обычно не нужно, SIGTERM стандарт.

  Для Go-сервиса оставь дефолт и просто слушай SIGTERM в коде.

  12. SHELL
  SHELL переопределяет шелл, который используется в shell-форме
  RUN, CMD, ENTRYPOINT.

    SHELL ["/bin/bash", "-o", "pipefail", "-c"]

  ЗАЧЕМ:
    • Дефолт — /bin/sh -c. Не всегда то, что нужно.
    • Для pipefail: если в цепочке `a | b` упадёт `a`, оболочка
      вернёт успех, потому что `b` завершился успешно. Это
      плохо — build пройдёт, хотя команда упала.
    • pipefail исправляет это.

  ПРИМЕР:
    SHELL ["/bin/bash", "-o", "pipefail", "-c"]
    RUN curl -sSL https://example.com/file.tar.gz | tar -xz

  Без pipefail: если curl упадёт, tar распакует пустоту,
  сборка «пройдёт», но результат будет пустой.
  С pipefail: сборка упадёт на ошибке curl.

  13. ONBUILD
  ONBUILD задаёт инструкцию, которая выполнится НЕ СЕЙЧАС,
  а когда кто-то возьмёт твой образ как базовый (FROM my-image).

    ONBUILD COPY . /app
    ONBUILD RUN go build -o /app/server

  ПРИМЕР ИСПОЛЬЗОВАНИЯ:
    Ты делаешь базовый образ company/go-base:1.0 с настройками
    компании. Все твои сервисы FROM company/go-base:1.0.
    При сборке каждого сервиса автоматически выполнятся ONBUILD-
    инструкции — например, `go mod download`.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Редко. Только для базовых образов внутри компании.
    • Если хочешь навязать поведение всем, кто использует
      твой образ.

  МИНУСЫ:
    • Неявное поведение. Разработчик не видит в своём Dockerfile,
      что происходит.
    • Плохо для отладки.
    • Устаревает — BuildKit даёт более явные альтернативы.

  ПРАВИЛО: по умолчанию не используй. Только если у тебя
  реально есть базовый образ для сотни сервисов.

  14. VOLUME (В DOCKERFILE)
  VOLUME создаёт точку монтирования для внешних данных.
  Всё, что записывается в эту точку, не попадает в writable-
  слой контейнера, а хранится в volume.

    VOLUME /data
    VOLUME ["/data", "/logs"]

  ЧТО ЭТО ДАЁТ:
    • Данные сохраняются между перезапусками контейнера.
    • Если при docker run не указать -v, Docker создаст
      анонимный volume и примонтирует его.

  ПРОБЛЕМА:
    • Анонимные volumes накапливаются. `docker volume ls`
      покажет кучу безымянных, которые никто не чистит.
    • Если хочешь свой volume — указывай -v при run:

      docker run -v mydata:/data my-app

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Редко. Обычно volumes настраивают при run или в compose.
    • VOLUME в Dockerfile оправдан, если образ по своей природе
      требует внешнего хранилища (например, образ БД).

  Для Go-сервиса VOLUME не нужен — данные храни в БД или S3.

  15. ARG ПЕРЕД FROM И SCOPE ПЕРЕМЕННЫХ
  ARG, объявленный ДО первого FROM, доступен во всех stage.
  ARG после FROM — только в этом stage.

    # Доступен везде
    ARG GO_VERSION=1.22

    FROM golang:${GO_VERSION}-alpine AS builder
    # ...

    FROM alpine:3.19
    # Тут GO_VERSION тоже виден, если объявить ARG повторно

  ВАЖНО: внутри каждого stage нужно ПОВТОРНО объявлять ARG,
  чтобы он был доступен:

    ARG VERSION=dev

    FROM golang:1.22-alpine AS builder
    ARG VERSION              ← повторно!
    RUN echo "building $VERSION"

    FROM alpine:3.19
    ARG VERSION              ← снова повторно!
    RUN echo "runtime $VERSION"

  ЭТО ЧАСТАЯ ОШИБКА. Разработчики объявляют ARG один раз
  в начале и удивляются, почему он пустой внутри stage.

  ЗАЧЕМ ЭТО НУЖНО:
    • Собирать разные варианты одного образа (dev/prod).
    • Передавать версии зависимостей.
    • Multi-arch через TARGETOS/TARGETARCH.

  Секреты сюда НЕ клади — они видны в docker history.

  16. BUILDKIT: CACHE MOUNTS И СЕКРЕТЫ
  BuildKit — новая система сборки, дефолт в современном Docker.
  Даёт две мощные фичи: cache mounts и secrets.

  16.1. CACHE MOUNTS
  Кэш Go-сборки и модулей сохраняется МЕЖДУ билдами. Каждый
  билд не начинает с нуля.

    RUN --mount=type=cache,target=/root/.cache/go-build \
        --mount=type=cache,target=/go/pkg/mod \
        go build -o /app/server .

  ЭФФЕКТ:
    • Первый билд: качает всё, компилирует всё.
    • Второй билд: использует кэш. Быстрее в разы.

  В CI с BuildKit cache эта штука экономит минуты на каждой
  сборке.

  Проверь, что BuildKit включён:
    docker buildx version

  С современным Docker включён по умолчанию. Если нет —
  переменная окружения DOCKER_BUILDKIT=1.

  16.2. SECRETS
  Секреты (токены, ключи для приватных репозиториев) можно
  передать в сборку так, что они НЕ попадут ни в один слой.

    # В Dockerfile
    RUN --mount=type=secret,id=github_token \
        GITHUB_TOKEN=$(cat /run/secrets/github_token) \
        go mod download

  При сборке:
    docker build --secret id=github_token,src=./token.txt -t my-app .

  ЧТО ПРОИСХОДИТ:
    • Секрет монтируется в /run/secrets/github_token во время
      выполнения RUN.
    • После RUN монтирование исчезает.
    • В слоях образа секрета НЕТ.

  Это правильный способ работать с секретами. НЕ пихай их
  в ENV или ARG — они видны через docker history.

  17. MULTI-STAGE: ПРОДВИНУТЫЕ ПАТТЕРНЫ
  Базовый multi-stage — два FROM (builder + runtime). Есть
  более продвинутые паттерны.

  17.1. ИМЕНОВАННЫЕ STAGE
  Давай имена, чтобы ссылаться:
    FROM golang:1.22 AS builder
    FROM alpine:3.19 AS runtime
    FROM runtime AS final
  COPY --from=builder /app/server /server

  17.2. ОБЩИЙ БАЗОВЫЙ STAGE
  Если несколько stage используют общие настройки:
    FROM alpine:3.19 AS base
    RUN apk add --no-cache ca-certificates
    WORKDIR /app

    FROM base AS app1
    COPY --from=builder1 /app/server /app/server

    FROM base AS app2
    COPY --from=builder2 /app/server /app/server

  Экономит время сборки — базовые слои переиспользуются.

  17.3. КОПИРОВАНИЕ ИЗ ВНЕШНЕГО ОБРАЗА
  Можно копировать не только из своих stage, но и из любых
  образов:
    COPY --from=nginx:1.27 /etc/nginx/nginx.conf /etc/nginx/nginx.conf
  Или даже из конкретного stage в registry:
    COPY --from=myregistry.com/base:1.0 /app/config.yaml /etc/

  17.4. BUILD ARGS ДЛЯ ВЫБОРА STAGE
  Можно передавать ARG, чтобы выбирать, какой stage собирать:
    ARG BASE=alpine
    FROM ${BASE}:3.19
  Полезно для сборки под разные окружения.

  18. ПРОДОВЫЙ DOCKERFILE ДЛЯ GO

  Всё вместе — production-ready Dockerfile:

    # ---- Stage 1: builder ----
    FROM golang:1.22-alpine AS builder

    # Cache mounts для скорости (BuildKit)
    RUN --mount=type=cache,target=/go/pkg/mod \
        --mount=type=cache,target=/root/.cache/go-build \
        true

    WORKDIR /app

    # Сначала манифесты — для кэша
    COPY go.mod go.sum ./
    RUN --mount=type=cache,target=/go/pkg/mod \
        go mod download

    # Потом код
    COPY . .

    # Cross-compile через ARG
    ARG TARGETOS=linux
    ARG TARGETARCH=amd64
    ARG VERSION=dev

    RUN --mount=type=cache,target=/go/pkg/mod \
        --mount=type=cache,target=/root/.cache/go-build \
        CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
        go build \
          -ldflags="-s -w -X main.version=$VERSION" \
          -trimpath \
          -o /app/server .

    # ---- Stage 2: runtime ----
    FROM gcr.io/distroless/static-debian12:nonroot

    # Метаданные
    LABEL org.opencontainers.image.title="order-service"
    LABEL org.opencontainers.image.version="${VERSION}"

    # Копируем только бинарник
    COPY --from=builder /app/server /server

    # Порт (документация)
    EXPOSE 8080

    # Non-root
    USER nonroot:nonroot

    # Graceful shutdown через SIGTERM (по умолчанию)
    STOPSIGNAL SIGTERM

    # Healthcheck через встроенную команду
    HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
      --retries=3 \
      CMD ["/server", "healthcheck"]

    ENTRYPOINT ["/server"]

  ЧТО ЗДЕСЬ ЧТО:
    • Cache mounts для go-build и go/pkg/mod — ускорение
      сборки в разы.
    • COPY go.mod до COPY . . — правильный порядок для кэша.
    • ARG TARGETOS/TARGETARCH — для cross-compile.
    • ARG VERSION — для прокидывания версии в бинарник.
    • -ldflags="-s -w" — уменьшает бинарник.
    • -trimpath — убирает локальные пути.
    • CGO_ENABLED=0 — статический бинарник для distroless.
    • distroless:nonroot — минимальный образ, non-root.
    • LABEL по OCI-стандарту.
    • STOPSIGNAL SIGTERM — явно, хотя это дефолт.
    • HEALTHCHECK через встроенную команду `/server healthcheck`.
  РЕЗУЛЬТАТ:
    • Размер образа: 15-20 МБ.
    • Non-root.
    • Graceful shutdown работает.
    • Multi-arch (amd64/arm64).
    • Кэш ускоряет сборку.
    • Нет shell → меньше attack surface.

  19. АНТИПАТТЕРНЫ

  19.1. ADD ВМЕСТО COPY.
    ADD делает неявные вещи (распаковка, скачивание).
    Используй COPY.
  19.2. RUN КОМАНДЫ В РАЗНЫХ СЛОЯХ.
    Чистка в другом слое не уменьшает образ. Объединяй через &&.
  19.3. COPY . . ПЕРЕД go mod download.
    Любое изменение кода = пересборка зависимостей.
  19.4. SHELL-ФОРМА ENTRYPOINT.
    PID 1 — sh, SIGTERM не доходит. Используй exec-форму.
  19.5. latest В FROM.
    Меняется без предупреждения. Фиксируй версию.
  19.6. СЕКРЕТЫ В ENV ИЛИ ARG.
    Видны в docker history. Используй BuildKit secrets.
  19.7. WORKDIR НЕ ЗАДАН.
    Всё копируется в корень. Мусор.
  19.8. ROOT USER.
    По умолчанию root. Добавь USER.
  19.9. МНОГО RUN ДЛЯ ОДНОЙ ЗАДАЧИ.
    Каждый RUN — слой. Объединяй.
  19.10. EXPOSE ПУТАЮТ С ПУБЛИКАЦИЕЙ.
    EXPOSE — только документация. Публикация — через -p.
  19.11. ARG ОБЪЯВЛЕН ОДИН РАЗ В НАЧАЛЕ.
    Внутри каждого stage нужно повторно объявлять ARG.
  19.12. НЕТ CACHE MOUNTS ДЛЯ GO.
    Каждый билд пересобирает всё с нуля. Медленно.

  20. ФИНАЛЬНЫЕ ВЫВОДЫ

  1.  FROM — базовый образ. Первая инструкция. Для сборки Go —
      golang, для рантайма — alpine/distroless/scratch.
  2.  WORKDIR — рабочая директория. Всегда задавай абсолютный.
  3.  COPY — копирование. ADD не используй. --chown для non-root.
  4.  RUN — команды сборки. Объединяй через &&. Чистка в том  же слое.
  5.  ENV — переменные окружения (рантайм). ARG — аргументы
      сборки (только при сборке). Секреты — НЕ здесь.
  6.  EXPOSE — документация порта. LABEL — метаданные,
      важны для OCI.
  7.  USER — non-root для безопасности.
  8.  CMD — дефолтные аргументы. ENTRYPOINT — команда.
  9.  EXEC-ФОРМА обязательна для CMD/ENTRYPOINT в Go. Иначе
      SIGTERM не доходит.
  10. HEALTHCHECK — проверка здоровья. Для distroless/scratch —
      встроенная команда в бинарник.
  11. STOPSIGNAL — обычно SIGTERM. Для Go — дефолт.
  12. SHELL — меняет шелл. Полезно для pipefail.
  13. ONBUILD — неявное поведение для базовых образов.
      Не используй без причины.
  14. VOLUME — создаёт точку монтирования. Используй через
      -v при run или compose, не в Dockerfile.
  15. ARG перед FROM — доступен во всех stage, но внутри
      каждого нужно повторно объявлять.
  16. BuildKit даёт cache mounts (ускорение в разы) и
      secrets (безопасная передача токенов). Используй.
  17. Multi-stage: именованные stage, общий базовый stage,
      копирование из внешних образов.
  18. Продовый Dockerfile: distroless, non-root, cache mounts,
      cross-compile через ARG, HEALTHCHECK через команду.
  19. Порядок для Go: FROM → WORKDIR → COPY go.mod → RUN download
      → COPY . . → RUN build → второй stage → ENTRYPOINT.
  20. Правильный порядок слоёв + cache mounts = сборка в 50 раз быстрее.
  21. Антипаттерны: ADD, shell-form ENTRYPOINT, COPY . . перед
      download, latest, root user, секреты в ENV, отсутствие
      cache mounts.
  22. Dockerfile — это навык. Собери 5 разных — закрепится.
*/

var (
	version = "dev"     // переопределяется через -ldflags
	appEnv  = "unknown" // переопределяется через ENV
)

func main() {
	// Если первым аргументом "healthcheck" — это healthcheck.
	// Так делают для distroless, где нет curl.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		resp, err := http.Get("http://localhost:8080/health")
		if err != nil || resp.StatusCode != 200 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "PID: %d\n", os.Getpid())
		fmt.Fprintf(w, "Version: %s\n", version)
		fmt.Fprintf(w, "AppEnv: %s\n", appEnv)
		fmt.Fprintf(w, "User: %d\n", os.Getuid())
		fmt.Fprintf(w, "Hostname: %s\n", hostname())
		fmt.Fprintf(w, "Time: %s\n", time.Now().Format(time.RFC3339))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	srv := &http.Server{Addr: ":" + port, Handler: mux}

	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	go func() {
		log.Printf("listening on :%s, version=%s, appEnv=%s",
			port, version, appEnv)
		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("bye")
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
