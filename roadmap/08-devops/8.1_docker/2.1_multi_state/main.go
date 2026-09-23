package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

/*
  УРОК 2.1: MULTI-STAGE BUILDS
  Компилятор Go нужен только для сборки. В рантайме он не нужен.
  Но если ты собираешь образ нативно — в него попадёт всё: Go SDK,
  gcc, git, промежуточные объектные файлы, кэш модулей. Образ
  раздувается до 800+ МБ. Из этих 800 МБ в рантайме нужно 15 —
  сам бинарник.
  Multi-stage build — это способ разделить «сборочную среду»
  и «среду запуска» на два (или больше) этапа в одном Dockerfile.
  На первом этапе компилируем, на втором — берём только
  результат и упаковываем в минимальный образ.
  Разница в размере — в 40-50 раз. Разница в скорости pull —
  в 40-50 раз. Разница в attack surface — колоссальная.

  СОДЕРЖАНИЕ:
    1.  Проблема: что попадает в образ без multi-stage
    2.  Идея multi-stage build
    3.  Базовый пример: два FROM в одном Dockerfile
    4.  Ключевая инструкция: COPY --from=builder
    5.  Именование stage через AS
    6.  Выбор финального базового образа
    7.  Почему alpine — не всегда лучший вариант
    8.  scratch: абсолютный минимум
    9.  distroless: золотая середина
    10. Кэширование слоёв в multi-stage
    11. Продвинутые паттерны: общий base, несколько бинарников
    12. Multi-arch и cross-compilation
    13. Оптимизация размера: -ldflags, -trimpath, upx
    14. Проверка образа: docker history, inspect, dive
    15. Частые ошибки при переходе на multi-stage
    16. Антипаттерны
    17. Финальные выводы

  1. ПРОБЛЕМА: ЧТО ПОПАДАЕТ В ОБРАЗ БЕЗ MULTI-STAGE
  Нативный Dockerfile для Go-сервиса:

    FROM golang:1.22
    WORKDIR /app
    COPY . .
    RUN go build -o server .
    CMD ["./server"]

  Что внутри получившегося образа:
    • Go SDK целиком: компилятор, стандартная библиотека
      в исходниках, инструменты (go, gofmt, go vet, godoc). ~500 МБ.
    • gcc и binutils — если базовый образ на Debian/Ubuntu.
    • git — нужен для go mod download приватных репо.
    • ca-certificates, базовые утилиты Debian/Ubuntu.
    • Кэш модулей go — может быть сотни МБ.
    • Исходники твоего проекта: все .go-файлы, .md, .git.
    • Промежуточные объектные файлы, если не убрал.

  Итог: 700-900 МБ. И в этом образе лежит компилятор, который
  в рантайме никогда не запустится. Но каждый раз при pull
  нового образа эти 800 МБ качаются. При каждом деплое.
  При каждом скейле. При каждом новом поде в Kubernetes.

  Что реально нужно в рантайме:
    • Скомпилированный бинарник (5-15 МБ).
    • Если CGO_ENABLED=0 — вообще ничего кроме бинарника.
    • Если динамическая линковка — glibc или musl.
  Всё. Остальное — балласт.

  2. ИДЕЯ MULTI-STAGE BUILD
  Разделить Dockerfile на два (или больше) этапа:

    Этап 1 — BUILDER:
      • Большой базовый образ с компилятором.
      • Копируем исходники.
      • Ставим зависимости.
      • Компилируем бинарник.
      • Здесь всё «грязно», этот этап в финальный образ не попадёт.

    Этап 2 — RUNTIME:
      • Минимальный базовый образ (alpine/distroless/scratch).
      • Копируем ТОЛЬКО бинарник из этапа 1.
      • Никаких исходников, компиляторов, мусора.

  В финальном образе остаётся только то, что скопировали
  из builder'а через COPY --from. Всё остальное — отбрасывается.

  Схема:
    ┌──────────────────────────────────────────────┐
    │  STAGE 1: builder                            │
    │                                              │
    │  FROM golang:1.22-alpine                     │
    │  ├─ COPY go.mod go.sum                       │
    │  ├─ RUN go mod download                      │
    │  ├─ COPY . .                                 │
    │  └─ RUN go build -o /out/server .            │
    │                                              │
    │  Итог: 500 МБ с исходниками и компилятором   │
    └──────────────────┬───────────────────────────┘
                       │
                       │ COPY --from=builder /out/server /server
                       │
                       ▼
    ┌──────────────────────────────────────────────┐
    │  STAGE 2: runtime                            │
    │                                              │
    │  FROM gcr.io/distroless/static               │
    │  ├─ COPY --from=builder /out/server /server  │
    │  └─ ENTRYPOINT ["/server"]                   │
    │                                              │
    │  Итог: 15 МБ — только бинарник               │
    └──────────────────────────────────────────────┘

  Финальный образ в 30-50 раз меньше. При этом работает
  точно так же.

  3. БАЗОВЫЙ ПРИМЕР: ДВА FROM В ОДНОМ DOCKERFILE
  Минимальный multi-stage Dockerfile для Go-сервиса:
    # ---- Stage 1: builder ----
    FROM golang:1.22-alpine AS builder

    WORKDIR /src

    COPY go.mod go.sum ./
    RUN go mod download

    COPY . .
    RUN CGO_ENABLED=0 go build -o /out/server .

    # ---- Stage 2: runtime ----
    FROM alpine:3.19

    COPY --from=builder /out/server /server

    ENTRYPOINT ["/server"]

  Разбор:
    • Два FROM в одном файле — это и есть multi-stage.
    • Первый FROM создаёт «builder» stage.
    • Второй FROM начинает «runtime» stage с чистого листа.
    • COPY --from=builder — единственная связь между stage.
    • Всё, что было в builder'е, кроме явно скопированного,
      в финальный образ не попадает.

  Размер:
    Без multi-stage:  ~800 МБ (golang:1.22-alpine + код + кэш).
    С multi-stage:    ~15 МБ  (alpine + бинарник).

  4. КЛЮЧЕВАЯ ИНСТРУКЦИЯ: COPY --FROM=BUILDER
  COPY --from — это инструкция, которая копирует файлы из одного
  stage в другой.

    COPY --from=builder /out/server /server

  Что она делает:
    • Ищет stage с именем builder.
    • Берёт файл /out/server из его файловой системы.
    • Копирует в текущий stage в /server.

  ВАРИАНТЫ СИНТАКСИСА:
    # По имени stage (рекомендуется)
    COPY --from=builder /out/server /server

    # По номеру stage (0 — первый, 1 — второй)
    COPY --from=0 /out/server /server

    # Из внешнего образа (без своего builder'а)
    COPY --from=nginx:1.27 /etc/nginx/nginx.conf /etc/nginx/nginx.conf

    # С изменением владельца (для non-root)
    COPY --from=builder --chown=1000:1000 /out/server /server

  ЧТО МОЖНО КОПИРОВАТЬ:
    • Один файл: /out/server.
    • Папку: /out/.
    • Симлинки, конфиги, статические ассеты.
    • Всё, что вообще есть в файловой системе stage.

  ЧТО НЕЛЬЗЯ КОПИРОВАТЬ:
    • Env-переменные.
    • Метаданные.
    • Историю слоёв. Только файлы.

  ЕСЛИ НЕ СКОПИРОВАТЬ ЯВНО — ФАЙЛ ИСЧЕЗНЕТ.
  Это ключевое правило multi-stage.

  5. ИМЕНОВАНИЕ STAGE ЧЕРЕЗ AS
  Каждый FROM может (и должен) иметь имя через AS:

    FROM golang:1.22-alpine AS builder
    FROM alpine:3.19 AS runtime

  Зачем:
    • Читаемость: COPY --from=builder читается понятнее,
      чем COPY --from=0.
    • Позволяет собирать только конкретный stage:
        docker build --target builder -t app:builder .
        docker build --target runtime -t app:runtime .
    • Позволяет ссылаться на stage в другом stage, даже если
      он объявлен позже (редко).

  ПРАВИЛО: всегда именуй stage. Это бесплатно и делает
  Dockerfile читаемым.

  ИМЕНОВАННЫЕ STAGE ПОЗВОЛЯЮТ СОБИРАТЬ ЧАСТИЧНО:
    docker build --target builder -t app:builder .
      → Соберёт только builder stage. Внутри будет компилятор,
        исходники, кэш. Полезно для отладки.

    docker build --target runtime -t app:runtime .
      → Соберёт всё до runtime включительно. Это обычный
        финальный образ.

    docker build --target prod -t app:prod .
      → Если у тебя stage-цепочка prod ← staging ← base,
        соберётся до prod.

  6. ВЫБОР ФИНАЛЬНОГО БАЗОВОГО ОБРАЗА
  Для runtime stage есть четыре варианта:
  ┌─────────────────────────┬──────────┬───────────────────────────────┐
  │ Образ                   │ Размер   │ Особенности                   │
  ├─────────────────────────┼──────────┼───────────────────────────────┤
  │ alpine:3.19             │ ~5 МБ    │ musl libc, есть sh, apk.      │
  │                         │          │ Годится для CGO и для отладки │
  ├─────────────────────────┼──────────┼───────────────────────────────┤
  │ gcr.io/distroless/...   │ ~2 МБ    │ Нет shell, нет пакетов.       │
  │                         │          │ Есть ca-certs, tzdata,        │
  │                         │          │ nonroot-пользователь.         │
  ├─────────────────────────┼──────────┼───────────────────────────────┤
  │ scratch                 │ 0 МБ     │ Пусто. Только то, что         │
  │                         │          │ скопируешь. Нет /etc/passwd,  │
  │                         │          │ ca-certs, tzdata. Настраивай  │
  │                         │          │ вручную.                      │
  ├─────────────────────────┼──────────┼───────────────────────────────┤
  │ debian:slim / ubuntu    │ ~30 МБ   │ Если нужны glibc,             │
  │                         │          │ сторонние бинарники.          │
  └─────────────────────────┴──────────┴───────────────────────────────┘

  ПРАВИЛА ВЫБОРА:
    • CGO_ENABLED=0 → можно alpine, distroless, scratch.
    • CGO_ENABLED=1 → нужно alpine (musl) или debian (glibc).
    • Нужен shell для отладки → alpine.
    • Строгий прод → distroless или scratch.
    • Нужны сторонние утилиты (psql, curl) → alpine.

  ДЛЯ GO-СЕРВИСА ОБЫЧНО: distroless. Не scratch (там нет
  ca-certs, tzdata), не alpine (лишний shell для атаки).
  Distroless — золотая середина.

  7. ПОЧЕМУ ALPINE — НЕ ВСЕГДА ЛУЧШИЙ ВАРИАНТ
  Alpine известен как «маленький образ». Это правда: 5 МБ.
  Но у него есть особенности.

  MUSL VS GLIBC:
    Alpine использует musl libc, а не glibc. Это значит:
      • Если бинарник собран с CGO — может не работать.
      • Некоторые пакеты ищут glibc — придётся ставить
        libc6-compat.
      • Race detector в Go не работает на musl.

  DNS RESOLVER:
    musl имеет другой DNS-резолвер. В больших k8s-кластерах
    с CoreDNS бывают проблемы с поиском сервисов. Иногда
    нужно копировать /etc/resolv.conf или ставить nss-dns.

  SHELL:
    Alpine использует busybox ash. Не bash. Многие скрипты
    с bash-синтаксисом не работают. Иногда это ок, иногда
    неожиданно.

  APK:
    Пакетный менеджер apk. Не apt. Синтаксис другой:
      apk add --no-cache curl
    Для простых случаев — ок. Для сложных зависимостей —
    не всё есть в apk-репозитории.

  КОГДА ALPINE — ПРАВИЛЬНО:
    • Нужен shell для entrypoint-скрипта.
    • Нужны сторонние утилиты (curl, jq, psql).
    • Нужно ставить сертификаты через apk.
    • Разработка и отладка.

  КОГДА ALPINE — ПЛОХО:
    • Строгий прод, где не нужен shell.
    • CGO-бинарники без тестирования.
    • Минимальный attack surface — тут distroless лучше.

  8. SCRATCH: АБСОЛЮТНЫЙ МИНИМУМ
  scratch — это пустой образ. Вообще ничего.

    FROM scratch
    COPY --from=builder /out/server /server
    ENTRYPOINT ["/server"]

  Что внутри:
    • Твой бинарник.
    • Больше ничего.

  ЧТО НУЖНО СДЕЛАТЬ ВРУЧНУЮ:

    • CA-СЕРТИФИКАТЫ. Без них не будут работать HTTPS-запросы
      к внешним API (Stripe, AWS, etc).

        COPY --from=builder /etc/ssl/certs/ca-certificates.crt \
                            /etc/ssl/certs/ca-certificates.crt

    • TZDATA. Без них time.LoadLocation("Europe/Moscow")
      вернёт ошибку.

        COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

    • /etc/passwd И /etc/group. Нужны, если хочешь работать
      под non-root с именем пользователя. Без них — только UID.

        COPY --from=builder /etc/passwd /etc/passwd
        COPY --from=builder /etc/group /etc/group

    • USER. Работать под root в scratch — плохая идея.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Абсолютный минимум.
    • Static binary, CGO_ENABLED=0.
    • Никаких внешних вызовов по HTTPS.
    • UID фиксирован числом, не именем.

  КОГДА НЕ ИСПОЛЬЗОВАТЬ:
    • Нужны HTTPS-запросы.
    • Нужны таймзоны.
    • Нужны нестандартные libc.

  В 90% случаев distroless лучше scratch. Distroless уже
  содержит ca-certs, tzdata, nonroot-user. Тебе не надо
  руками тащить их из builder'а.

  9. DISTROLESS: ЗОЛОТАЯ СЕРЕДИНА
  Distroless — образы от Google. Никакой shell, никакого
  package manager. Только то, что нужно для запуска бинарника.

  ВАРИАНТЫ:
    gcr.io/distroless/static-debian12:nonroot
      → Для static-бинарников. Внутри ca-certs, tzdata,
        nonroot-user. ~2 МБ.

    gcr.io/distroless/base-debian12:nonroot
      → Для CGO-бинарников (glibc). ~20 МБ.

    gcr.io/distroless/cc-debian12:nonroot
      → Добавляет libstdc++ для C++.

    gcr.io/distroless/static-debian12:debug
      → Есть busybox для отладки. Не для прода.

  ЧТО ВНУТРИ:
    • ca-certificates — HTTPS работает из коробки.
    • tzdata — таймзоны работают.
    • /etc/passwd с nonroot-user (UID 65532).
    • Никаких shell, пакетов, компиляторов.

  ПОЧЕМУ ЭТО КРУТО:
    • Минимальный attack surface. Даже если злоумышленник
      пробьёт сервис, ему нечего запускать: shell отсутствует.
    • CVE почти не накапливаются. Обновлять нечего.
    • Меньше размер.
    • Nonroot по умолчанию (с :nonroot тегом).

  ПРОБЛЕМА:
    • Нельзя зайти внутрь через docker exec.
    • Нет curl для healthcheck.
    • Нет sh для entrypoint-скриптов.

  РЕШЕНИЯ:
    • Debug-образ из того же Dockerfile (alpine + strace).
    • Встроенная подкоманда в бинарник для healthcheck.
    • kubectl debug в K8s для ephemeral container.

  10. КЭШИРОВАНИЕ СЛОЁВ В MULTI-STAGE
  В builder'е слои кэшируются так же, как в обычном Dockerfile.
  Порядок инструкций критичен.

  ПЛОХО:
    FROM golang:1.22-alpine AS builder
    WORKDIR /src
    COPY . .                          ← меняется при каждой правке кода
    RUN go mod download               ← перекачивает зависимости каждый раз
    RUN go build -o /out/server .

  ХОРОШО:
    FROM golang:1.22-alpine AS builder
    WORKDIR /src
    COPY go.mod go.sum ./             ← меняется редко
    RUN go mod download               ← кэшируется
    COPY . .                          ← меняется часто
    RUN go build -o /out/server .

  ПЛЮС CACHE MOUNTS (BuildKit):

    RUN --mount=type=cache,target=/go/pkg/mod \
        --mount=type=cache,target=/root/.cache/go-build \
        go build -o /out/server .

  Это сохраняет кэш модулей и кэш компиляции между билдами.
  На второй сборке — кратно быстрее.

  ВАЖНО: cache mounts работают только с BuildKit. В современном
  Docker (20.10+) BuildKit включён по умолчанию. Если нет —
  DOCKER_BUILDKIT=1.

  Кэш mount не попадает в финальный образ. Это отдельное
  хранилище между билдами.

  11. ПРОДВИНУТЫЕ ПАТТЕРНЫ

  11.1. ОБЩИЙ BASE STAGE
  Если несколько stage используют общие настройки:

    FROM alpine:3.19 AS base
    RUN apk add --no-cache ca-certificates tzdata
    WORKDIR /app

    FROM base AS service-a
    COPY --from=builder-a /out/a /app/a
    ENTRYPOINT ["/app/a"]

    FROM base AS service-b
    COPY --from=builder-b /out/b /app/b
    ENTRYPOINT ["/app/b"]

  Слои base переиспользуются между service-a и service-b.

  11.2. НЕСКОЛЬКО БИНАРНИКОВ В ОДНОМ ОБРАЗЕ
  Если сервис имеет несколько бинарников (api, worker, migrate)
  в одном монорепо:
    FROM golang:1.22-alpine AS builder
    WORKDIR /src
    COPY go.mod go.sum ./
    RUN go mod download
    COPY . .

    RUN CGO_ENABLED=0 go build -o /out/api     ./cmd/api
    RUN CGO_ENABLED=0 go build -o /out/worker  ./cmd/worker
    RUN CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate

    FROM gcr.io/distroless/static-debian12:nonroot
    COPY --from=builder /out/api     /api
    COPY --from=builder /out/worker  /worker
    COPY --from=builder /out/migrate /migrate
    ENTRYPOINT ["/api"]

  Один образ, три бинарника. В Compose/K8s выбираешь роль
  через command:
    services:
      api:
        image: app:1.0
        command: ["/api"]
      worker:
        image: app:1.0
        command: ["/worker"]
      migrate:
        image: app:1.0
        command: ["/migrate", "up"]

  Меньше билдов, меньше образов, единая версия.

  11.3. СБОРКА ТОЛЬКО КОНКРЕТНОГО STAGE
  Если у тебя Dockerfile с несколькими финальными stage,
  собирай только тот, что нужен:
    docker build --target api     -t app-api:1.0 .
    docker build --target worker  -t app-worker:1.0 .
  Это экономит время в CI.

  12. MULTI-ARCH И CROSS-COMPILATION
  Go умеет компилировать под любую архитектуру без эмуляции:

    GOOS=linux GOARCH=arm64 go build ...

  В Dockerfile через ARG:

    FROM golang:1.22-alpine AS builder

    ARG TARGETOS=linux
    ARG TARGETARCH=amd64

    WORKDIR /src
    COPY . .

    RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
        go build -o /out/server .

  Сборка multi-arch через buildx:

    docker buildx build \
      --platform linux/amd64,linux/arm64 \
      --push \
      -t app:1.0 .

  TARGETOS и TARGETARCH Docker подставит автоматически
  для каждой платформы. Go cross-compile сработает без QEMU.

  ПОЧЕМУ ЭТО ВАЖНО:
    • Твой CI на x86, прод-серверы на ARM (Graviton, Apple Silicon).
    • Или наоборот.
    • Один билд — оба образа.

  ВАЖНО: без cross-compile пришлось бы использовать QEMU
  для эмуляции ARM на x86. Это в 5-10 раз медленнее.
  Go cross-compile решает проблему бесплатно.

  13. ОПТИМИЗАЦИЯ РАЗМЕРА
  Флаги, которые уменьшают бинарник:

    CGO_ENABLED=0 go build \
      -ldflags="-s -w -X main.version=1.0.0 -X main.commit=abc123" \
      -trimpath \
      -o /out/server .

  РАЗБОР:
    CGO_ENABLED=0
      Отключает CGO. Бинарник — статический. Не нужны
      glibc/musl. Работает на scratch.

    -ldflags="-s -w"
      -s — убирает symbol table.
      -w — убирает DWARF debug info.
      Экономия: 30-40% размера.

    -X main.version=1.0.0 -X main.commit=abc123
      Заменяет строковые переменные в main. Вшивает версию
      и git-commit прямо в бинарник.

    -trimpath
      Убирает локальные пути сборки из бинарника. Безопаснее
      и чуть-чуть меньше.

    UPX (опционально)
      Упаковщик бинарников. Уменьшает в 2-3 раза. Но:
        • Требует UPX на этапе сборки.
        • Бинарник распаковывается в память → больше RAM.
        • Некоторые антивирусы флагают UPX как подозрительное.
        • Может ломать CGO.
      Использовать осторожно, обычно не нужно.

  ЭФФЕКТ:
    Без флагов:        бинарник 25 МБ.
    С -ldflags="-s -w": бинарник 15 МБ.
    Со всеми флагами:   бинарник 12 МБ.

  14. ПРОВЕРКА ОБРАЗА
  Как убедиться, что в образе только бинарник:

  14.1. DOCKER HISTORY
    docker history app:1.0

  Показывает слои и их размер. Если видишь слой на 500 МБ —
  значит что-то упустил.

  Хороший вывод:
    IMAGE    CREATED   SIZE   COMMENT
    abc123   1 min     0B     ENTRYPOINT ["/server"]
    def456   1 min     12MB   COPY /out/server /server
    ghi789   1 min     2MB    FROM gcr.io/distroless/static

  Никаких 500 МБ golang:1.22-alpine. Хорошо.

  14.2. DOCKER IMAGES
    docker images app:1.0

    REPOSITORY   TAG   SIZE
    app          1.0   14MB

  Если видишь 800 МБ — забыл multi-stage.

  14.3. DIVE
    dive app:1.0

  Интерактивный инструмент. Показывает слои и что в них.
  Можно ходить по файловой системе образа. Незаменим для
  аудита «а что у меня в образе».

  14.4. КОПИРОВАНИЕ ИЗ ОБРАЗА
    docker create --name tmp app:1.0
    docker cp tmp:/ /tmp/rootfs
    ls -la /tmp/rootfs

  Или через --output:
    docker save app:1.0 -o app.tar
    tar xf app.tar
    ls

  Можно посмотреть, что реально лежит в образе.

  15. ЧАСТЫЕ ОШИБКИ ПРИ ПЕРЕХОДЕ НА MULTI-STAGE

  15.1. ЗАБЫЛИ COPY --FROM
  Классическая ошибка. Пишешь runtime stage, ставишь ENTRYPOINT,
  но не копируешь бинарник. Образ собирается, но при запуске:

    exec: "/server": stat /server: no such file or directory

  Решение: всегда COPY --from=builder перед ENTRYPOINT.

  15.2. КОПИРУЮТ ИСХОДНИКИ В RUNTIME
    COPY . .    ← в runtime stage

  Смысл multi-stage теряется. Исходники снова в образе.
  Решение: в runtime копируй только конкретные файлы из builder'а.

  15.3. NONROOT USER НЕ СОЗДАН
  В scratch нет /etc/passwd. Если пишешь:
    USER appuser

  Ошибка: appuser не существует. Решение:
    USER 1000:1000     ← только UID
    или
    COPY --from=builder /etc/passwd /etc/passwd

  Или используй distroless:nonroot — там всё уже есть.

  15.4. CA-СЕРТИФИКАТЫ НЕ СКОПИРОВАНЫ
  В scratch нет /etc/ssl/certs. HTTPS-запросы падают с
  x509: certificate signed by unknown authority.
  Решение: скопировать ca-certificates из builder'а или
  использовать distroless (там уже есть).

  15.5. TZDATA НЕ СКОПИРОВАНЫ
  В scratch нет /usr/share/zoneinfo.
    time.LoadLocation("Europe/Moscow")
    // error: unknown time zone Europe/Moscow
  Решение: скопировать tzdata или использовать distroless.

  15.6. КЭШИРОВАНИЕ НАРУШЕНО
    COPY . .                 ← до COPY go.mod
    COPY go.mod go.sum ./
  Порядок неверный. Кэш инвалидируется на каждом изменении.

  15.7. RUNTIME STAGE ТОЖЕ БОЛЬШОЙ
  Если в runtime используешь golang:1.22-alpine — смысла нет.
  В golang-образе 500 МБ. Нужен alpine / distroless / scratch.

  16. АНТИПАТТЕРНЫ

  16.1. ОДИН STAGE БЕЗ MULTI-STAGE.
    Наивный Dockerfile на golang:1.22 даёт 800 МБ образ.
  16.2. COPY . . В RUNTIME STAGE.
    Исходники и мусор снова в образе.
  16.3. RUNTIME НА GOLANG-ОБРАЗЕ.
    Смысл multi-stage потерян.
  16.4. ЗАБЫЛИ COPY --FROM.
    Финальный образ пустой, контейнер падает с no such file.
  16.5. USER APPUSER БЕЗ /ETC/PASSWD.
    В scratch нет passwd. Только UID.
  16.6. НЕТ CA-CERTS НА SCRATCH.
    HTTPS-запросы к внешним API падают.
  16.7. НЕТ TZDATA НА SCRATCH.
    time.LoadLocation падает.
  16.8. НЕ СБРАСЫВАЮТ КЭШ В КОНЦЕ.
    Если builder запускает go test, кэш тестов остаётся в слоях.
    Сбрось `go clean -cache` в конце builder'а, если размер важен.
  16.9. latest В БАЗОВЫХ ОБРАЗАХ.
    База может измениться и всё сломать. Фиксируй версии.
  16.10. ОТСУТСТВИЕ .DOCKERIGNORE.
    Даже с multi-stage build context раздувается. Всё летит
    в Docker daemon.
  16.11. БЕЗ CGO_ENABLED=0 ДЛЯ SCRATCH.
    Динамически слинкованный бинарник не работает на scratch.
  16.12. КОПИРОВАНИЕ ВСЕГО /OUT ВМЕСТО /OUT/SERVER.
    Если в /out ещё что-то лежит — попадёт в финальный образ.

  17. ФИНАЛЬНЫЕ ВЫВОДЫ

  1.  Multi-stage — разделение build-среды и runtime-среды.
      Первое — с компилятором, второе — только с результатом.
  2.  Синтаксис: два или больше FROM в одном Dockerfile,
      между ними COPY --from=builder.
  3.  Именуй stage через AS. Читаемее и позволяет собирать
      только нужный stage через --target.
  4.  Копируй только бинарник. Не исходники. Не кэш модулей.
  5.  Без multi-stage: 800 МБ. С multi-stage: 10-20 МБ.
      Разница в 40-50 раз.
  6.  База для runtime:
        • distroless — золотая середина (ca-certs, tzdata, nonroot).
        • scratch    — абсолютный минимум (тащи ca-certs и tzdata руками).
        • alpine     — если нужен shell и утилиты.
        • debian-slim — если нужен glibc.
  7.  CGO_ENABLED=0 — обязательно для scratch/distroless.
      Даёт статический бинарник, работающий без libc.
  8.  Флаги -ldflags="-s -w" и -trimpath уменьшают бинарник
      на 30-40%.
  9.  -X main.version=... — вшиваем версию и git-commit
      в бинарник.
  10. Порядок COPY: go.mod → go mod download → COPY . .
      → go build. Кэш не инвалидируется при правке кода.
  11. Cache mounts (BuildKit) — кэш модулей и компиляции
      между билдами. Ускоряет сборку в разы.
  12. Cross-compile через ARG TARGETOS/TARGETARCH — без QEMU.
      Multi-arch buildx работает быстро.
  13. Multi-role в одном образе: api + worker + migrate.
      Роль задаётся через command в compose/k8s.
  14. Проверка через docker history и dive. Если видишь
      500 МБ слой — что-то упустил.
  15. Частые ошибки: забыл COPY --from, COPY . . в runtime,
      USER appuser без passwd, нет ca-certs, нет tzdata.
  16. Multi-stage — стандарт для Go. Без него прод-образ
      недопустимо большой.
  17. На собесе: «Multi-stage разделяет сборку и рантайм.
      Builder — golang:alpine, runtime — distroless.
      Копируем только бинарник через COPY --from.
      CGO_ENABLED=0, -ldflags="-s -w", кэш модулей через
      BuildKit. Итог — образ 15 МБ вместо 800. Nonroot,
      без shell, минимальный attack surface».
*/

var (
	version = "dev"
	commit  = "none"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("version=" + version + " commit=" + commit))
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	go func() {
		logger.Info("listening", "version", version, "commit", commit)
		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(), 15*time.Second)
	defer cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)
	logger.Info("stopped")
}
