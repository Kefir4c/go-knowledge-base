package main

/*
  УРОК 9.2.5: GO WORKSPACES И МОНОРЕПОЗИТОРИИ
  Монорепо — единый git-репозиторий для сервисов, библиотек
  и инфраструктуры. Google, Uber, Netflix используют его.
  Плюсы: атомарные коммиты, общий код, единый CI. Минусы:
  большой репозиторий, сложный CI, большой Docker-контекст.
  В Go монорепо особенно удобен: модули живут рядом и
  ссылаются друг на друга через go.work. Но классический
  Dockerfile ломается — COPY go.mod ./ не видит
  shared-модули. Разбираем, как это чинить.

  СОДЕРЖАНИЕ:
    1.  Что такое монорепозиторий
    2.  Проблема: обычный Dockerfile ломается
    3.  Что такое go.work и go work sync
    4.  Структура монорепо
    5.  Варианты работы без go.work
    6.  Build context и локальные модули
    7.  Правильный Dockerfile
    8.  Кэширование слоёв
    9.  Один Dockerfile на все сервисы
    10. Отдельные Dockerfile по сервисам
    11. Shared-библиотеки
    12. Версионирование
    13. CI: matrix builds
    14. Антипаттерны
    15. Финальные выводы

  1. ЧТО ТАКОЕ МОНОРЕПОЗИТОРИЙ
  Монорепо — один git-репозиторий для всего кода компании:
  сервисов, shared-библиотек, инфраструктуры, тулов.

  ПЛЮСЫ:
    • Атомарные коммиты. Меняешь сервис и его зависимость —
      один коммит, одна ревизия.
    • Единая версия кода. Нет синхронизации версий
      shared-библиотек между репо.
    • Легко шарить код. Внутренние пакеты — просто импорты,
      не надо публиковать в registry.
    • Единый CI/CD. Один pipeline, один набор правил.
    • Простой рефакторинг. Изменение API затрагивает всех
      потребителей, и ты видишь это в одном PR.

  МИНУСЫ:
    • Большой репозиторий. git clone — минуты.
    • Сложный CI. Нужно понимать, что пересобирать.
    • Большой Docker-контекст. Всё летит в daemon, если
      не правильно организовать.
    • Единая версия — не всегда хорошо. Иногда сервисы
      хотят разные версии зависимостей.

  ПРИМЕР АТОМАРНОГО КОММИТА:
    Ты меняешь API в shared/logging и одновременно обновляешь
    order-service, который хочет его использовать. Один
    коммит, один PR. В полирепо это была бы пара PR в двух
    репо с синхронизацией порядка мержа.

  2. ПРОБЛЕМА: ОБЫЧНЫЙ DOCKERFILE ЛОМАЕТСЯ
  Классический Dockerfile для Go-сервиса:

    FROM golang:1.22-alpine AS builder
    COPY go.mod go.sum ./
    RUN go mod download
    COPY . .
    RUN go build -o /out/server ./cmd/order

  В монорепо go.mod лежит в services/order/go.mod, а не
  в корне. Сервис зависит от shared/logging.

  СЦЕНАРИЙ 1: CONTEXT — ПАПКА СЕРВИСА.

    docker build -f services/order/Dockerfile services/order

    COPY go.mod → OK (services/order/go.mod)
    RUN go mod download → OK (внешние зависимости)
    COPY . . → только файлы из services/order/
    RUN go build → ПАДЕНИЕ:
      package shared/logging: module not found

    Почему: shared/ лежит вне build context.

  СЦЕНАРИЙ 2: CONTEXT — КОРЕНЬ, НО DOCKERFILE КОПИРУЕТ GO.MOD.

    docker build -f services/order/Dockerfile .

    COPY go.mod go.sum ./ → в корне лежит go.work, а не go.mod.
      COPY падает: "/go.mod": not found

  СЦЕНАРИЙ 3: DOCKERFILE КОПИРУЕТ ВСЁ.

    COPY . .
    RUN go build ./services/order/cmd/order

    Работает. Но context — весь монорепо, сотни МБ летят
    в daemon. 30+ секунд только на передачу.

  ВЫВОД: для монорепо нужен другой Dockerfile и другое
  правило про build context. Плюс .dockerignore.

  3. ЧТО ТАКОЕ GO.WORK И GO WORK SYNC
  go.work — файл в корне монорепо. Говорит Go: «вот модули,
  работай с ними как с единым целым».

    go 1.22

    use (
        ./services/order
        ./services/payment
        ./shared/logging
        ./shared/proto
    )

  ЧТО ЭТО ДАЁТ:
    • Модули видят друг друга без replace-директив.
    • go build из корня собирает любой сервис.
    • Изменения в shared/ сразу видны всем сервисам.
    • go mod tidy работает на уровне workspace.

  КОМАНДЫ:
    go work init ./services/order ./shared/logging
      Создать workspace.

    go work use ./services/payment
      Добавить модуль в workspace.

    go work use -r .
      Рекурсивно добавить все модули.

    go work edit -dropuse ./services/old
      Убрать модуль.

    go work sync
      Синхронизировать версии между модулями.

  GO WORK SYNC ПОДРОБНЕЕ:
    Приводит версии общих зависимостей к одной, обновляет
    go.sum каждого модуля.

    ПРИМЕР: order использует github.com/foo v1.2.0,
    payment использует github.com/foo v1.3.0. go work sync
    приведёт их к одной версии.

    Почему важно: без sync легко получить разные версии
    одной библиотеки в разных сервисах. Это приводит
    к багам, которые невозможно воспроизвести.

    В Dockerfile: go work sync обязателен перед
    go mod download.

  ВАЖНО: go.work ИСПОЛЬЗУЕТ ОТНОСИТЕЛЬНЫЕ ПУТИ. Абсолютные
  не работают на CI, потому что пути на билд-сервере другие.

  go.work.sum — аналог go.sum для workspace. Хэши всех
  зависимостей всех модулей. Коммитится в git.

  4. СТРУКТУРА МОНОРЕПО
  Типичная структура:
    company/
    ├── go.work
    ├── go.work.sum
    ├── services/
    │   ├── order/
    │   │   ├── go.mod
    │   │   ├── Dockerfile
    │   │   └── cmd/order/main.go
    │   ├── payment/
    │   │   ├── go.mod
    │   │   └── cmd/payment/main.go
    │   └── notification/
    │       ├── go.mod
    │       └── cmd/notification/main.go
    └── shared/
        ├── logging/
        │   ├── go.mod
        │   └── log.go
        └── proto/
            ├── go.mod
            └── proto.go

  Каждый сервис — отдельный Go-модуль со своим go.mod.
  go.work связывает их.

  ВАРИАНТЫ СТРУКТУРЫ:
    SERVICES + SHARED (самый частый):
      services/ — сервисы.
      shared/ — внутренние модули.
      pkg/ — публичные библиотеки.

    ПО КОМАНДАМ:
      team-checkout/order/
      team-checkout/cart/
      team-identity/auth/
      shared/

    FLAT:
      order/
      payment/
      common/

  ПРАВИЛО ИМЕНОВАНИЯ МОДУЛЕЙ:
    module github.com/company/services/order
    import "github.com/company/services/order/internal/handler"

    module github.com/company/shared/logging
    import "github.com/company/shared/logging"

    Совместимо с приватным registry, если часть модулей
    когда-то опубликуется.

  5. ВАРИАНТЫ РАБОТЫ БЕЗ GO.WORK
  REPLACE В GO.MOD КАЖДОГО СЕРВИСА:

    // services/order/go.mod
    require (
        github.com/company/shared/logging v0.0.0
        github.com/company/shared/proto v0.0.0
    )

    replace github.com/company/shared/logging => ../../shared/logging
    replace github.com/company/shared/proto => ../../shared/proto

  ПЛЮСЫ:
    • Работает без go.work.
    • Каждый сервис сам объявляет зависимости.

  МИНУСЫ:
    • Много дублирования. 20 сервисов × 5 shared = 100 replace.
    • Легко забыть в новом сервисе.

  ДРУГОЙ ВАРИАНТ: SHARED В REGISTRY.

    Каждый shared-модуль публикуется в приватный Go registry
    (Gitea, GitLab, Artifactory, Athens). Сервисы импортируют
    как обычные зависимости.

    Плюсы: без go.work и replace, классическая модель
      версионирования.
    Минусы: нужен registry, теряется атомарность монорепо.

  ПРАВИЛО: используй go.work. Стандарт с Go 1.18.

  6. BUILD CONTEXT И ЛОКАЛЬНЫЕ МОДУЛИ
  В монорепо build context — ВСЕГДА КОРЕНЬ РЕПОЗИТОРИЯ.

  Почему: Docker не разрешает COPY ../shared. Всё, что за
  пределами context, недоступно. Контекст должен включать
  и сервис, и его shared-зависимости.

  СБОРКА:
    docker build -f services/order/Dockerfile -t app-order .

  Точка в конце — корень. Dockerfile может лежать в сервисе,
  но context — корень.

  МИНУС: контекст весь монорепо, сотни МБ.
  ЛЕЧЕНИЕ: .dockerignore.

  ПРОВЕРКА РАЗМЕРА КОНТЕКСТА:

    docker build --progress=plain -f services/order/Dockerfile . 2>&1 \
      | grep "transferring context"

  7. ПРАВИЛЬНЫЙ DOCKERFILE ДЛЯ МОНОРЕПО
  Полный продакшен Dockerfile для services/order:

    # ---- Stage 1: builder ----
    FROM golang:1.22-alpine AS builder

    RUN apk add --no-cache git ca-certificates
    WORKDIR /src

    # --- Слой 1: манифесты workspace. ---
    COPY go.work go.work.sum* ./

    # --- Слой 2: манифесты нужных модулей. ---
    COPY services/order/go.mod services/order/go.sum* ./services/order/
    COPY shared/logging/go.mod shared/logging/go.sum* ./shared/logging/
    COPY shared/proto/go.mod shared/proto/go.sum* ./shared/proto/

    # --- Слой 3: синхронизация и скачивание. ---
    RUN --mount=type=cache,target=/go/pkg/mod \
        go work sync && go mod download

    # --- Слой 4: код. ---
    COPY services/order ./services/order
    COPY shared/logging ./shared/logging
    COPY shared/proto ./shared/proto

    ARG VERSION=dev
    ARG COMMIT=none

    # --- Слой 5: сборка. ---
    RUN --mount=type=cache,target=/go/pkg/mod \
        --mount=type=cache,target=/root/.cache/go-build \
        CGO_ENABLED=0 \
        go build \
          -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
          -trimpath \
          -o /out/server \
          ./services/order/cmd/order

    # ---- Stage 2: prod ----
    FROM gcr.io/distroless/static-debian12:nonroot AS prod

    ARG VERSION=dev
    LABEL org.opencontainers.image.title="order-service"
    LABEL org.opencontainers.image.version="${VERSION}"

    COPY --from=builder /out/server /server

    USER nonroot:nonroot
    EXPOSE 8080
    ENV APP_ENV=production
    STOPSIGNAL SIGTERM

    ENTRYPOINT ["/server"]

  КЛЮЧЕВЫЕ МОМЕНТЫ:
    • COPY go.work — первым слоем. Редко меняется, кэшируется.
    • COPY go.mod нужных модулей — вторым слоем. Кэшируется
      до изменения любого из них.
    • go work sync + go mod download — с cache mount.
    • COPY кода только нужных модулей.
    • go build с явным путём к нужному сервису.

  ЧТО ПОПАДЁТ В ФИНАЛЬНЫЙ ОБРАЗ:
    Только /server. Модули, компиляторы, кэши остаются
    в builder'е.

  .DOCKERIGNORE ДЛЯ МОНОРЕПО:

    # Исключить всё.
    *

    # Вернуть нужное.
    !go.work
    !go.work.sum
    !services/order/**
    !services/order
    !shared/logging/**
    !shared/logging
    !shared/proto/**
    !shared/proto

  Паттерны применяются последовательно. Сначала * исключает
  всё, потом ! возвращает конкретные пути. Если написать
  !до * — ничего не сработает.

  8. КЭШИРОВАНИЕ СЛОЁВ
  Два подхода в монорепо.

  ПОДХОД 1: КОПИРУЕМ ВСЕ МОДУЛИ.

    COPY go.work
    COPY services/order/go.mod
    COPY services/payment/go.mod
    COPY shared/logging/go.mod
    COPY . .

    ПЛЮСЫ: просто.
    МИНУСЫ: изменение в payment инвалидирует order.
      Контекст большой.

  ПОДХОД 2: КОПИРУЕМ ТОЛЬКО НУЖНОЕ СЕРВИСУ.

    COPY go.work
    COPY services/order/go.mod
    COPY shared/logging/go.mod
    COPY services/order ./services/order
    COPY shared/logging ./shared/logging

    ПЛЮСЫ: изменение в payment не трогает order. Кэш
      независимый. Контекст маленький.
    МИНУСЫ: при изменении shared нужно править Dockerfile.

  ЗАМЕР СКОРОСТИ (подход 2):

    Первый билд order:              ~15 сек
    Второй билд без изменений:      ~1 сек
    После правки в order:           ~3 сек
    После правки в shared/logging:  ~5 сек
    После правки в payment:         ~0 сек

  СРАВНЕНИЕ С ПОДХОДОМ 1:
    После правки в payment: ~15 сек (order тоже пересобирается).

  ДЛЯ 5-20 СЕРВИСОВ: подход 2. Для 50+: Bazel/Dagger/Earthly.

  CACHE MOUNTS:
    /go/pkg/mod          — модули (зависимости).
    /root/.cache/go-build — объектники компиляции.
    Оба обязательны.

  9. ОДИН DOCKERFILE НА ВСЕ СЕРВИСЫ
  Параметризация через ARG SERVICE:

    FROM golang:1.22-alpine AS builder

    ARG SERVICE=order

    RUN apk add --no-cache git ca-certificates
    WORKDIR /src

    COPY go.work go.work.sum* ./
    COPY services ./services
    COPY shared ./shared

    RUN --mount=type=cache,target=/go/pkg/mod \
        go work sync && go mod download

    COPY . .

    RUN --mount=type=cache,target=/go/pkg/mod \
        --mount=type=cache,target=/root/.cache/go-build \
        CGO_ENABLED=0 \
        go build -o /out/server ./services/${SERVICE}/cmd/${SERVICE}

    FROM gcr.io/distroless/static-debian12:nonroot AS prod
    COPY --from=builder /out/server /server
    USER nonroot:nonroot
    ENTRYPOINT ["/server"]

  СБОРКА:
    docker build --build-arg SERVICE=order -t app-order .
    docker build --build-arg SERVICE=payment -t app-payment .

  ПЛЮСЫ: один Dockerfile.
  МИНУСЫ: копируем все services. Изменение в payment
    инвалидирует order. Кэш общий. Контекст большой.

  ГОДИТСЯ ДЛЯ: маленьких монорепо (2-5 сервисов), прототипов.

  10. ОТДЕЛЬНЫЕ DOCKERFILE ПО СЕРВИСАМ
  Для средних и больших монорепо — свой Dockerfile на сервис.

    services/order/Dockerfile
    services/payment/Dockerfile
    services/notification/Dockerfile

  Каждый копирует только свои зависимости + нужные shared.

  СБОРКА:
    docker build -f services/order/Dockerfile -t app-order .
    docker build -f services/payment/Dockerfile -t app-payment .

  ПЛЮСЫ:
    • Кэш независимый между сервисами.
    • Каждый сервис знает только свои зависимости.
    • Контекст маленький через .dockerignore.
    • Изменение в payment не трогает order.

  МИНУСЫ:
    • Дублирование структуры Dockerfile.
    • При изменении shared нужно править все Dockerfile.

  ШАБЛОНИЗАТОРЫ (ДЛЯ 20+ СЕРВИСОВ):
    • DAGGER — программируемый CI/CD на Go.
    • EARTHLY — Dockerfile с функциями и кэшем.
    • MAKEFILE с шаблонами — простое решение.

  ПРАВИЛО: 5-20 сервисов — отдельные Dockerfile.
  20+ — отдельные + шаблонизатор.

  11. SHARED-БИБЛИОТЕКИ
  ТРИ ТИПА ОБЩИХ МОДУЛЕЙ:
    • pkg/ — публичные библиотеки. Могут использоваться
      снаружи.
    • shared/ — внутренние модули. Только для сервисов
      компании.
    • services/X/internal/ — доступно только внутри одного
      сервиса. Компилятор Go не даст импортировать из другого.

  В DOCKERFILE:
    Копируй только те shared-модули, которые реально нужны
    этому сервису. Если order не использует shared/auth —
    не копируй его.

  ОБРАТНАЯ СОВМЕСТИМОСТЬ:
    Shared-модули должны меняться только additive.
    Изменение API ломает все сервисы одновременно.

    ПРАВИЛА:
      • Добавлять новые функции — ок.
      • Добавлять новые поля в структуры — ок.
      • Удалять или переименовывать — только через
        deprecation период.
      • Менять сигнатуру — только через новую функцию.

  12. ВЕРСИОНИРОВАНИЕ В МОНОРЕПО
  В монорепо версия — это git SHA и тег.

  В DOCKERFILE:
    ARG VERSION=dev
    ARG COMMIT=none

    RUN go build \
      -ldflags="-X main.version=${VERSION} -X main.commit=${COMMIT}"

  ПРИ СБОРКЕ:
    docker build \
      --build-arg VERSION=$(git describe --tags) \
      --build-arg COMMIT=$(git rev-parse --short HEAD) \
      ...

  В ОБРАЗЕ:
    LABEL org.opencontainers.image.version="${VERSION}"
    LABEL org.opencontainers.image.revision="${COMMIT}"

  ПРОБЛЕМЫ МОНОРЕПО:
    • Откат одного сервиса без отката shared — сложно.
      Всё меняется одним коммитом.
    • Разные циклы релиза у сервисов.
    • Изменение shared требует пересборки всех.

  РЕШЕНИЯ:
    • Feature flags.
    • Versioned API (v2 эндпоинты, старые работают).
    • Contract tests.
    • Обратная совместимость — единственный надёжный способ.

  13. CI: MATRIX BUILDS
  Собираем только изменённые сервисы.

  GITHUB ACTIONS С PATHS-FILTER:
    jobs:
      detect-changes:
        runs-on: ubuntu-latest
        outputs:
          services: ${{ steps.filter.outputs.changes }}
        steps:
          - uses: actions/checkout@v4
          - uses: dorny/paths-filter@v3
            id: filter
            with:
              filters: |
                order:
                  - 'services/order/**'
                  - 'shared/logging/**'
                payment:
                  - 'services/payment/**'
                  - 'shared/logging/**'

      build:
        needs: detect-changes
        strategy:
          matrix:
            service: ${{ fromJSON(needs.detect-changes.outputs.services) }}
        steps:
          - uses: actions/checkout@v4
          - run: |
              docker buildx build \
                -f services/${{ matrix.service }}/Dockerfile \
                --build-arg VERSION=${{ github.ref_name }} \
                --build-arg COMMIT=${{ github.sha }} \
                -t myregistry/${{ matrix.service }}:${{ github.sha }} \
                --push \
                .

  ЧТО ЭТО ДАЁТ:
    Изменён order → собирается только order.
    Изменён shared/logging → собираются оба.
    Изменён payment → собирается только payment.

  ДЛЯ 50+ СЕРВИСОВ: Bazel, Dagger, Earthly — граф
  зависимостей, инкрементальные сборки.

  14. АНТИПАТТЕРНЫ
  14.1. BUILD CONTEXT — ПАПКА СЕРВИСА.
    Docker не видит shared. Всегда используй корень.
  14.2. НЕТ .DOCKERIGNORE В МОНОРЕПО.
    Контекст 500 МБ. Паттерн: сначала *, потом !пути.
  14.3. COPY . . ПЕРЕД GO MOD DOWNLOAD.
    Инвалидирует зависимости при каждой правке кода.
  14.4. КОПИРОВАНИЕ ВСЕХ GO.MOD.
    50 сервисов — 50 go.mod. Копируй только нужные.
  14.5. ОДИН DOCKERFILE С ARG SERVICE.
    Кэш общий. Изменение в payment инвалидирует order.
  14.6. НЕТ CACHE MOUNTS.
    Каждая сборка качает все зависимости заново.
  14.7. БЕЗ GO.WORK, ТОЛЬКО REPLACE.
    Каждый сервис пишет 5-10 replace. Легко забыть.
  14.8. ЗАБЫЛИ GO WORK SYNC В DOCKERFILE.
    Версии модулей рассинхронизируются.
  14.9. SHARED БЕЗ ВЕРСИОНИРОВАНИЯ.
    Изменение ломает все сервисы одновременно.
  14.10. LATEST В БАЗОВЫХ ОБРАЗАХ.
    Версия Go не совпадёт с go.work.
  14.11. ОТСУТСТВИЕ MATRIX BUILD.
    Каждый коммит собирает все 50 сервисов.
  14.12. ЦИКЛИЧЕСКИЕ ЗАВИСИМОСТИ.
    order → shared/logging → shared/config → order.
    Go не соберёт.
  14.13. ABSOLUTE PATHS В GO.WORK.
    CI не соберёт. Только относительные пути.
  14.14. GO.WORK.SUM НЕ КОММИТИТСЯ.
    У других разработчиков и в CI будут разные версии.

  15. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Монорепо — единый репозиторий. Плюсы: атомарные
      коммиты, общий код, единый CI.
  2.  go.work — стандарт с Go 1.18. Список модулей через
      use. Работает без replace.
  3.  go work sync — синхронизация версий. Обязательна
      перед go mod download.
  4.  Build context для монорепо — ВСЕГДА корень репо.
      Иначе shared-модули не видны.
  5.  .dockerignore обязателен. Паттерн: сначала *,
      потом !конкретные пути.
  6.  В Dockerfile: go.work → go.mod нужных модулей →
      go work sync + go mod download → код → build.
  7.  Cache mounts обязательны: /go/pkg/mod и
      /root/.cache/go-build.
  8.  Отдельный Dockerfile на сервис — оптимально.
      Один с ARG SERVICE — проще, но кэш общий.
  9.  Shared-библиотеки меняются только additive.
      Обратная совместимость обязательна.
  10. Версионирование — git SHA + tags через ARG
      VERSION/COMMIT и -ldflags.
  11. CI: matrix builds с paths-filter. Собирай только
      изменённые сервисы.
  12. Антипаттерны: context = папка сервиса, нет
      .dockerignore, COPY . . перед download, все go.mod,
      нет go work sync, нет cache mounts, нет matrix.
  13. Для 5-20 сервисов: отдельные Dockerfile +
      paths-filter + cache mounts.
  14. Для 50+: Bazel/Dagger/Earthly для графа зависимостей.
*/
