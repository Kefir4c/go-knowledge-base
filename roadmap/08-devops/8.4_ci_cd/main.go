package main

/*
  УРОК 8.4: CI/CD НА GITHUB ACTIONS
  Ты пишешь Go-сервис. Локально go test проходит, Dockerfile собирается. Но как это автоматизировать?
  Каждый push в main — тесты, линтер, сборка образа, публикация в registry. Без CI/CD это руками, с ошибками и болью.
  GitHub Actions — самый простой вход в CI/CD для Go-проекта: репозиторий уже в GitHub, CI рядом, secrets в настройках, публикация в ghcr.io одной строкой.
  Этот урок — минимум, чтобы собрать production-ready pipeline для Go-сервиса.

  СОДЕРЖАНИЕ:
    1.  Зачем CI/CD и почему GitHub Actions
    2.  Workflows, jobs, steps — базовые понятия
    3.  Triggers: on (push, pull_request, schedule, workflow_dispatch)
    4.  Runners и окружения
    5.  Actions — actions/checkout, actions/setup-go, actions/cache
    6.  Jobs и steps: последовательность и параллельность
    7.  Matrix builds — несколько версий Go/OS
    8.  Кэш зависимостей и build cache
    9.  Secrets и Environment secrets
    10. Environments — dev/staging/prod с approvals
    11. Reusable workflows и composite actions
    12. Concurrency — отмена старых запусков
    13. Artifacts — сохранение результатов
    14. Публикация Docker-образа в ghcr.io
    15. Практика: pipeline linter → tests → build → push в ghcr.io
    16. Связь с Go
    17. Антипаттерны
    18. Финальные выводы

  1. ЗАЧЕМ CI/CD И ПОЧЕМУ GITHUB ACTIONS

  ПРОБЛЕМА: без CI/CD всё руками.
    • Забыл запустить тесты перед push.
    • Линтер не прогнали — баги в main.
    • Собрать образ, запушить, обновить Deployment — вручную.
    • Опечатка в теге — сломанный релиз.

  РЕШЕНИЕ: CI/CD.
    CI (Continuous Integration) — автоматические тесты, линтер, сборка на каждый push.
    CD (Continuous Delivery/Deployment) — автоматическая доставка в окружения (dev/staging/prod).

  ПОЧЕМУ GITHUB ACTIONS:
    • Если код в GitHub — Actions уже там. Не нужен отдельный Jenkins/GitLab CI.
    • YAML в .github/workflows/.
    • Бесплатно для публичных репо и до 2000 минут/мес для приватных.
    • Огромный маркетплейс готовых actions.
    • Secrets в настройках репо.
    • Публикация в ghcr.io (GitHub Container Registry) — из коробки.

  АЛЬТЕРНАТИВЫ:
    GitLab CI, Jenkins, CircleCI, Buildkite, Drone.
    Принципы те же: workflow, jobs, steps, secrets, cache, matrix.

  ЧТО ДАЁТ ДЛЯ GO-ПРОЕКТА:
    • На каждый PR — тесты, линтер, race detector.
    • На merge в main — сборка образа, push в registry.
    • На тег (v1.2.3) — релиз с бинарниками.
    • Матрица тестов на Go 1.21, 1.22, 1.23.

  2. WORKFLOWS, JOBS, STEPS — БАЗОВЫЕ ПОНЯТИЯ
  WORKFLOW — файл YAML в .github/workflows/. Описывает, что и когда запускать.

  СТРУКТУРА:
    .github/
    └── workflows/
        ├── ci.yml
        ├── release.yml
        └── deploy.yml

  КАЖДЫЙ WORKFLOW:
    • name — отображается в UI.
    • on — триггеры (push, PR, schedule).
    • jobs — список джобов.

  JOB — набор шагов, выполняется на одном runner'е.
    • Все шаги одного job'а на одной машине.
    • Jobs по умолчанию параллельны.
    • Зависимости через needs: job2 needs job1.

  STEP — один шаг внутри job'а.
    • Либо uses: — использовать готовый action.
    • Либо run: — выполнить команду shell.

  МИНИМАЛЬНЫЙ WORKFLOW:
    name: CI

    on:
      push:
        branches: [main]
      pull_request:
        branches: [main]

    jobs:
      test:
        runs-on: ubuntu-latest
        steps:
        - uses: actions/checkout@v4
        - uses: actions/setup-go@v5
          with:
            go-version: "1.22"
        - run: go test ./...

  ЧТО ЗДЕСЬ:
    name: CI — название workflow.
    on: — триггеры (push в main, PR в main).
    jobs.test — один джоб с именем test.
    runs-on: ubuntu-latest — runner.
    steps — шаги: checkout, setup-go, run go test.

  ПРАВИЛО: один workflow — одна цель. Не пихай всё в один файл. Разделяй: ci.yml, release.yml, deploy.yml.

  3. TRIGGERS: ON (PUSH, PULL_REQUEST, SCHEDULE, WORKFLOW_DISPATCH)
  Триггеры определяют, когда запускать workflow.

  PUSH — на пуш в ветки:
    on:
      push:
        branches: [main, "release/*"]
        tags: ["v*"]
        paths-ignore: ["docs/**", "*.md"]

  PULL_REQUEST — на PR:
    on:
      pull_request:
        branches: [main]
        types: [opened, synchronize, reopened]

  SCHEDULE — по расписанию (cron):
    on:
      schedule:
      - cron: "0 3 * * *"    # каждый день в 3:00 UTC

  WORKFLOW_DISPATCH — вручную из UI:
    on:
      workflow_dispatch:
        inputs:
          environment:
            description: "Deploy target"
            required: true
            default: "dev"
            type: choice
            options: [dev, staging, prod]

  КОМБИНАЦИЯ:
    on:
      push:
        branches: [main]
      pull_request:
        branches: [main]
      schedule:
      - cron: "0 3 * * 1"
      workflow_dispatch:

  ЧТО ИСПОЛЬЗУЮТ В ПРОДЕ:
    • push в main — сборка + push образа.
    • pull_request — линтер, тесты.
    • push tags v* — релиз.
    • workflow_dispatch — ручной деплой в прод.
    • schedule — ночные проверки (зависимости, E2E).

  ФИЛЬТРЫ:
    branches / branches-ignore — по веткам.
    tags / tags-ignore — по тегам.
    paths / paths-ignore — по изменённым файлам.

  ПРАВИЛО: paths-ignore для docs/** экономит минуты. Не гоняй CI на изменения README.

  4. RUNNERS И ОКРУЖЕНИЯ
  Runner — виртуалка или контейнер, где выполняется job.

  GITHUB-HOSTED:
    ubuntu-latest    — Ubuntu 22.04/24.04, самый популярный.
    ubuntu-22.04     — фиксированная версия.
    windows-latest   — Windows Server 2022.
    macos-latest     — macOS Sonoma (для сборки под macOS).

  ЧТО ПРЕДУСТАНОВЛЕНО НА ubuntu-latest:
    Docker, Go, Node, Python, gcc, make, git, kubectl, helm, много чего.
    Не надо ставить руками.

  SELF-HOSTED:
    runs-on: [self-hosted, linux, x64]
    Если нужны свои ресурсы: GPU, ARM, доступ к внутренней сети, лицензионный софт.

  СТОИМОСТЬ:
    Публичные репо — бесплатно.
    Приватные — 2000 минут/мес бесплатно (Linux), дальше платно.
    macOS ×25-50, Windows ×2 от Linux по стоимости минут.

  ПРАВИЛО: для Go-проекта ubuntu-latest хватает почти всегда. Self-hosted — только если нужны специфичные ресурсы.

  5. ACTIONS — ACTIONS/CHECKOUT, ACTIONS/SETUP-GO, ACTIONS/CACHE

  ACTIONS/CHECKOUT — клонирует репозиторий:
    - uses: actions/checkout@v4
      with:
        fetch-depth: 0      # полная история (для git describe, tag)
        submodules: recursive

  Без checkout runner пустой — нечего делать.

  ACTIONS/SETUP-GO — устанавливает Go:
    - uses: actions/setup-go@v5
      with:
        go-version: "1.22"           # конкретная версия
        go-version-file: "go.mod"    # взять из go.mod
        cache: true                   # включить кэш Go-модулей и build cache

  cache: true — самый важный флаг. Кэширует $GOPATH/pkg/mod и build cache. Ускоряет в 2-5 раз.

  ACTIONS/CACHE — для других кэшей:
    - uses: actions/cache@v4
      with:
        path: |
          ~/.cache/go-build
          ~/go/pkg/mod
        key: ${{ runner.os }}-go-${{ hashFiles('/go.sum') }}
		restore-keys: |
          ${{ runner.os }}-go-

  Ключ: если go.sum изменился — новый кэш. Если нет — старая копия.

  ДРУГИЕ ПОЛЕЗНЫЕ ACTIONS:
    actions/upload-artifact@v4     — сохранить артефакты.
    actions/download-artifact@v4   — скачать артефакты.
    docker/setup-buildx-action@v3  — настроить buildx для multi-arch.
    docker/login-action@v3         — логин в registry.
    docker/build-push-action@v6    — сборка и push образа.
    golangci/golangci-lint-action@v6 — линтер.
    softprops/action-gh-release@v2 — релиз с бинарниками.

  ПРАВИЛО: используй официальные actions (actions/*, docker/*) — они поддерживаются и безопасны. Community — проверяй, кто автор.

  6. JOBS И STEPS: ПОСЛЕДОВАТЕЛЬНОСТЬ И ПАРАЛЛЕЛЬНОСТЬ
  JOBS по умолчанию параллельны. Зависимости — через needs.

    jobs:
      lint:
        runs-on: ubuntu-latest
        steps: ...

      test:
        runs-on: ubuntu-latest
        steps: ...

      build:
        needs: [lint, test]    # запустится после lint и test
        runs-on: ubuntu-latest
        steps: ...

  ЧТО ЭТО ДАЁТ:
    lint и test запускаются параллельно. build ждёт оба.
    Если lint или test падает — build не запускается.

  STEPS внутри job'а — последовательны.
    Если step падает — job фейлится, следующие steps не выполняются.

  УСЛОВИЯ НА STEP:
    - name: Notify
      if: failure()
      run: echo "Something failed"

    if: — только для этого step.
    Условия: success(), failure(), always(), cancelled().

  ПАРАЛЛЕЛЬНОСТЬ:
    max 256 jobs в workflow. Практически — хватает с головой.
    Для больших проектов — parallel matrix.

  ПРАВИЛО: разделяй lint, test, build на отдельные jobs. Параллельно быстрее, и в UI видно, что упало.

  7. MATRIX BUILDS — НЕСКОЛЬКО ВЕРСИЙ GO/OS
  Matrix — запустить один job с разными параметрами.

    strategy:
      matrix:
        go-version: ["1.21", "1.22", "1.23"]
        os: [ubuntu-latest, macos-latest]

    runs-on: ${{ matrix.os }}

    steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version: ${{ matrix.go-version }}
    - run: go test ./...

  ЧТО ЗДЕСЬ: 3 версии Go × 2 OS = 6 параллельных jobs.

  INCLUDE / EXCLUDE:
    matrix:
      go-version: ["1.21", "1.22", "1.23"]
      os: [ubuntu-latest, macos-latest]
      exclude:
      - go-version: "1.21"
        os: macos-latest
      include:
      - go-version: "1.22"
        os: ubuntu-latest
        experimental: true

  FAIL-FAST:
    strategy:
      fail-fast: false       # не отменять остальные, если один упал
      max-parallel: 4        # ограничить параллельность

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Тесты на нескольких версиях Go (совместимость).
    • Сборка под Linux/macOS/Windows.
    • Разные архитектуры (amd64, arm64) через go build с GOOS/GOARCH.

  ПРАВИЛО: для Go-сервиса — matrix по 2-3 версиям Go. Это ловит баги совместимости.

  8. КЭШ ЗАВИСИМОСТЕЙ И BUILD CACHE
  Кэш — главный ускоритель CI. Без кэша go mod download каждый раз с нуля.

  ДЛЯ GO — через setup-go:
    - uses: actions/setup-go@v5
      with:
        go-version: "1.22"
        cache: true

    Автоматически кэширует:
      • $GOPATH/pkg/mod — скачанные модули.
      • ~/.cache/go-build — build cache.

    Ключ формируется из go.sum. Если go.sum не меняется — кэш переиспользуется.

  ЯВНЫЙ КЭШ (если нужен контроль):
    - uses: actions/cache@v4
      with:
        path: |
          ~/.cache/go-build
          ~/go/pkg/mod
        key: ${{ runner.os }}-go-${{ hashFiles('/go.sum') }}
        restore-keys: |
          ${{ runner.os }}-go-

  ЭФФЕКТ:
    Без кэша: go test ./... — 3-5 минут.
    С кэшем: 30-60 секунд.

  ДРУГИЕ КЭШИ:
    • Docker layers — через docker/build-push-action с cache-from/cache-to.
    • golangci-lint — ~/.cache/golangci-lint.
    • npm/yarn — для фронта рядом.

  ПРАВИЛО: cache: true в setup-go — обязательно. Без кэша CI будет медленным и дорогим.

  9. SECRETS И ENVIRONMENT SECRETS
  Secrets — переменные, которые не видны в логах.

  ГДЕ ЗАДАЮТСЯ:
    • Settings → Secrets and variables → Actions → Repository secrets.
    • Settings → Environments → <env> → Environment secrets.

  КАК ИСПОЛЬЗУЮТСЯ:
    steps:
    - run: echo "${{ secrets.MY_SECRET }}"
    - env:
        GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

  ВСТРОЕННЫЙ GITHUB_TOKEN:
    Автоматически создаётся для каждого workflow. Не надо задавать.
    Права через permissions:
      permissions:
        contents: read
        packages: write      # для push в ghcr.io

  МАСКИРОВАНИЕ:
    GitHub автоматически маскирует secrets в логах (заменяет на ***).
    Но если ты сам распечатаешь secret в base64 или через переменную — маскировка может не сработать.

  ЧЕГО НЕ ДЕЛАТЬ:
    • Не логировать secrets.
    • Не передавать их в PR из форков (по умолчанию secrets недоступны для PR из форков).
    • Не коммитить .env с секретами.
    • Не использовать один и тот же секрет для dev и prod.

  ЧТО ПОЛЕЗНО ХРАНИТЬ В SECRETS:
    • Docker registry credentials.
    • API-ключи (для интеграционных тестов).
    • SONAR_TOKEN, CODECOV_TOKEN.
    • SSH-ключи (для деплоя).
    • Kubeconfig (для kubectl apply).

  ПРАВИЛО: secrets только для чувствительного. Всё остальное — в variables (Settings → Variables) или прямо в YAML.

  10. ENVIRONMENTS — DEV/STAGING/PROD С APPROVALS

  Environment — окружение для деплоя. Позволяет задать:
    • Свои secrets.
    • Required reviewers (ручное подтверждение).
    • Wait timer (задержка перед деплоем).
    • Branch restrictions (только main может деплоить в prod).

  КАК ЗАДАТЬ:
    Settings → Environments → New environment → prod.
    Добавить reviewers, secrets, branch policy.

  КАК ИСПОЛЬЗУЕТСЯ В WORKFLOW:
    jobs:
      deploy-prod:
        runs-on: ubuntu-latest
        environment:
          name: prod
          url: https://api.example.com
        steps:
        - run: kubectl apply -f k8s/

  ЧТО ЭТО ДАЁТ:
    • Деплой в prod требует ручного approve.
    • Secrets разные для dev/staging/prod.
    • Видно в UI, кто когда деплоил.

  PATTERN ДЛЯ ПРОДА:
    push в main → deploy в dev (автоматически).
    workflow_dispatch → deploy в staging (ручной запуск).
    environment prod → deploy в prod (ручной approve).

  ПРАВИЛО: prod — только через environment с reviewers. Один человек не должен случайно задеплоить в прод.

  11. REUSABLE WORKFLOWS И COMPOSITE ACTIONS
  Когда у тебя 10 репозиториев, и в каждом одинаковый CI — копипаста.

  REUSABLE WORKFLOW — workflow, который вызывается из другого:
    В reusable workflow (общий репо):
      on:
        workflow_call:
          inputs:
            go-version:
              required: true
              type: string
          secrets:
            registry-token:
              required: true

    В основном репо:
      jobs:
        ci:
          uses: my-org/workflows/.github/workflows/go-ci.yml@main
          with:
            go-version: "1.22"
          secrets:
            registry-token: ${{ secrets.GHCR_TOKEN }}

  COMPOSITE ACTION — набор steps, который можно переиспользовать внутри job'а:
    В action.yml:
      name: Setup Go project
      runs: composite
      inputs:
        go-version:
          required: true
      steps:
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ inputs.go-version }}
          cache: true
      - run: go mod download

    Использование:
      - uses: ./.github/actions/setup-go-project
        with:
          go-version: "1.22"

  КОГДА ЧТО:
    Reusable workflow — для целых CI/CD процессов (весь pipeline).
    Composite action — для набора шагов внутри job'а (setup, login, build).

  ПРАВИЛО: если CI одинаков в 3+ репо — выноси в reusable workflow. Не копипасти YAML.

  12. CONCURRENCY — ОТМЕНА СТАРЫХ ЗАПУСКОВ
  ПРОБЛЕМА: ты пушишь 5 коммитов подряд. Запускается 5 workflow. Первые 4 уже неактуальны.

  РЕШЕНИЕ: concurrency.
    concurrency:
      group: ${{ github.workflow }}-${{ github.ref }}
      cancel-in-progress: true

  ЧТО ЭТО ДЕЛАЕТ:
    • Все запуски workflow с одинаковым group — в одной очереди.
    • cancel-in-progress: true — отменять предыдущий, если пришёл новый.
    • Для PR — идеально: пуш → отмена старого запуска → новый.

  РАЗНЫЕ GROUP:
    Для main:
      group: ci-main
      cancel-in-progress: false    # не отменять main

    Для PR:
      group: ci-pr-${{ github.ref }}
      cancel-in-progress: true

  ПРАВИЛО: concurrency с cancel-in-progress для PR — экономит минуты и нервы.

  13. ARTIFACTS — СОХРАНЕНИЕ РЕЗУЛЬТАТОВ
  Artifacts — файлы, которые сохраняются после workflow. Скачиваются из UI.

  ЗАЧЕМ:
    • Бинарники Go для релиза.
    • Coverage-отчёты.
    • Логи тестов при падении.
    • Docker build context.

  ПРИМЕР:
    - name: Build
      run: go build -o dist/server .

    - uses: actions/upload-artifact@v4
      with:
        name: server-linux-amd64
        path: dist/server
        retention-days: 7

  СКАЧАТЬ В ДРУГОМ JOB'Е:
    - uses: actions/download-artifact@v4
      with:
        name: server-linux-amd64
        path: dist/

  ДЛЯ РЕЛИЗА:
    softprops/action-gh-release@v2
    Прикрепляет бинарники к GitHub Release.

  ПРАВИЛО: артефакты — для временных файлов. Для бинарников релиза — GitHub Releases.

  14. ПУБЛИКАЦИЯ DOCKER-ОБРАЗА В GHCR.IO
  ghcr.io — GitHub Container Registry. Бесплатно для публичных образов.

  ПОДГОТОВКА:
    permissions:
      contents: read
      packages: write

  СБОРКА И PUSH:
    - uses: docker/setup-buildx-action@v3

    - uses: docker/login-action@v3
      with:
        registry: ghcr.io
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}

    - uses: docker/metadata-action@v5
      id: meta
      with:
        images: ghcr.io/${{ github.repository }}
        tags: |
          type=ref,event=branch
          type=ref,event=pr
          type=semver,pattern={{version}}
          type=sha,prefix=sha-

    - uses: docker/build-push-action@v6
      with:
        context: .
        push: ${{ github.event_name != 'pull_request' }}
        tags: ${{ steps.meta.outputs.tags }}
        labels: ${{ steps.meta.outputs.labels }}
        cache-from: type=gha
        cache-to: type=gha,mode=max

  ЧТО ЗДЕСЬ:
    setup-buildx — multi-arch сборка.
    login — логин в ghcr.io (использует GITHUB_TOKEN).
    metadata — генерация тегов (по ветке, semver, sha).
    build-push — сборка и push. push: false для PR.

  ЧТО ТАКОЕ metadata-action:
    Автоматически создаёт теги: latest на main, 1.2.3 на tag v1.2.3, sha-abc123 всегда.
    Не надо руками писать теги.

  CACHE-FROM/CACHE-TO:
    type=gha — кэш слоёв Docker в GitHub Actions Cache. Ускоряет сборку в 3-5 раз.

  MULTI-ARCH:
    - uses: docker/build-push-action@v6
      with:
        platforms: linux/amd64,linux/arm64

  ПРАВИЛО: docker/build-push-action с metadata-action и cache-from/cache-to — стандарт для публикации образов.

  15. ПРАКТИКА: PIPELINE LINTER → TESTS → BUILD → PUSH В GHCR.IO
  Соберём production-ready pipeline для Go-сервиса.

  СТРУКТУРА:
    .github/
    └── workflows/
        ├── ci.yml
        └── release.yml

  CI.YML (на каждый push и PR):
    name: CI

    on:
      push:
        branches: [main]
      pull_request:
        branches: [main]

    concurrency:
      group: ci-${{ github.ref }}
      cancel-in-progress: true

    permissions:
      contents: read
      packages: write

    env:
      GO_VERSION: "1.22"

    jobs:
      lint:
        runs-on: ubuntu-latest
        steps:
        - uses: actions/checkout@v4
        - uses: actions/setup-go@v5
          with:
            go-version: ${{ env.GO_VERSION }}
            cache: true
        - uses: golangci/golangci-lint-action@v6
          with:
            version: v1.60
            args: --timeout=5m

      test:
        runs-on: ubuntu-latest
        strategy:
          fail-fast: false
          matrix:
            go-version: ["1.21", "1.22", "1.23"]
        steps:
        - uses: actions/checkout@v4
        - uses: actions/setup-go@v5
          with:
            go-version: ${{ matrix.go-version }}
            cache: true
        - name: Run tests
          run: go test -race -coverprofile=coverage.out -covermode=atomic ./...
        - name: Upload coverage
          if: matrix.go-version == '1.22'
          uses: actions/upload-artifact@v4
          with:
            name: coverage
            path: coverage.out

      build:
        needs: [lint, test]
        runs-on: ubuntu-latest
        steps:
        - uses: actions/checkout@v4
        - uses: actions/setup-go@v5
          with:
            go-version: ${{ env.GO_VERSION }}
            cache: true
        - name: Build
          run: |
            CGO_ENABLED=0 GOOS=linux go build -o dist/server .
        - uses: actions/upload-artifact@v4
          with:
            name: server-linux-amd64
            path: dist/server

      docker:
        needs: [build]
        runs-on: ubuntu-latest
        steps:
        - uses: actions/checkout@v4

        - uses: docker/setup-buildx-action@v3

        - uses: docker/login-action@v3
          if: github.event_name != 'pull_request'
          with:
            registry: ghcr.io
            username: ${{ github.actor }}
            password: ${{ secrets.GITHUB_TOKEN }}

        - uses: docker/metadata-action@v5
          id: meta
          with:
            images: ghcr.io/${{ github.repository }}
            tags: |
              type=ref,event=branch
              type=sha,prefix=sha-
              type=raw,value=latest,enable=${{ github.ref == 'refs/heads/main' }}

        - uses: docker/build-push-action@v6
          with:
            context: .
            push: ${{ github.event_name != 'pull_request' }}
            tags: ${{ steps.meta.outputs.tags }}
            labels: ${{ steps.meta.outputs.labels }}
            cache-from: type=gha
            cache-to: type=gha,mode=max
            platforms: linux/amd64

  RELEASE.YML (на тег v*):
    name: Release

    on:
      push:
        tags: ["v*"]

    permissions:
      contents: write
      packages: write

    jobs:
      release:
        runs-on: ubuntu-latest
        steps:
        - uses: actions/checkout@v4
          with:
            fetch-depth: 0

        - uses: actions/setup-go@v5
          with:
            go-version: "1.22"
            cache: true

        - name: Build binaries
          run: |
            mkdir -p dist
            for GOOS in linux darwin; do
              for GOARCH in amd64 arm64; do
                CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH \
                  go build -o dist/server-${GOOS}-${GOARCH} .
              done
            done

        - uses: docker/setup-buildx-action@v3

        - uses: docker/login-action@v3
          with:
            registry: ghcr.io
            username: ${{ github.actor }}
            password: ${{ secrets.GITHUB_TOKEN }}

        - uses: docker/metadata-action@v5
          id: meta
          with:
            images: ghcr.io/${{ github.repository }}
            tags: |
              type=semver,pattern={{version}}
              type=semver,pattern={{major}}.{{minor}}
              type=semver,pattern={{major}}

        - uses: docker/build-push-action@v6
          with:
            context: .
            push: true
            tags: ${{ steps.meta.outputs.tags }}
            labels: ${{ steps.meta.outputs.labels }}
            platforms: linux/amd64,linux/arm64
            cache-from: type=gha
            cache-to: type=gha,mode=max

        - uses: softprops/action-gh-release@v2
          with:
            files: dist/*

            generate_release_notes: true

  DOCKERFILE (multi-stage):
    FROM golang:1.22-alpine AS builder
    WORKDIR /src
    COPY go.mod go.sum ./
    RUN go mod download
    COPY . .
    RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .

    FROM gcr.io/distroless/static-debian12:nonroot
    COPY --from=builder /out/server /server
    USER nonroot:nonroot
    EXPOSE 8080
    ENTRYPOINT ["/server"]

  ЧТО СДЕЛАЛОСЬ:
    • lint — golangci-lint на каждый push/PR.
    • test — матрица Go 1.21/1.22/1.23 с race detector и coverage.
    • build — сборка бинарника после lint и test.
    • docker — сборка и push в ghcr.io (только не для PR).
    • release — на тег v* собирает бинарники и образы, создаёт GitHub Release.

  ПРОВЕРИТЬ ЛОКАЛЬНО (перед push):
    act -j lint        # запустить workflow локально через act (Docker).

  ПОСМОТРЕТЬ РЕЗУЛЬТАТ:
    • Actions tab в GitHub.
    • Badge: ![CI](https://github.com/USER/REPO/actions/workflows/ci.yml/badge.svg)

  BADGE В README:
    ![CI](https://github.com/org/repo/actions/workflows/ci.yml/badge.svg)
    ![Release](https://github.com/org/repo/actions/workflows/release.yml/badge.svg)

  16. СВЯЗЬ С GO
  Что нужно от Go-разработчика для CI/CD:

  1. ГОТОВЫЙ DOCKERFILE.
     Multi-stage, distroless, CGO_ENABLED=0.
     В CI собирается образ, не бинарник на бинарник.

  2. ТЕСТЫ С RACE DETECTOR.
     go test -race ./... — обязательно. Ловит гонки, которые не видны без -race.

  3. COVERAGE.
     go test -coverprofile=coverage.out ./...
     Отправлять в Codecov или SonarQube.

  4. ЛИНТЕР.
     golangci-lint с конфигом .golangci.yml.
     В CI — golangci/golangci-lint-action.

  5. ВЕРСИЯ GO В GO.MOD.
     go 1.22
     В CI — setup-go с go-version-file: go.mod.

  6. СТРУКТУРИРОВАННЫЙ КОД.
     cmd/, internal/, pkg/. Тесты рядом с кодом.

  7. Makefile (опционально).
     make lint, make test, make build.
     В CI — make test. Удобно и локально, и в CI.

  8. НЕТ СЕКРЕТОВ В КОДЕ.
     Всё через env. В CI — secrets.

  ПРИМЕР MAKE TARGETS:
    .PHONY: lint test build
    lint:
        golangci-lint run --timeout=5m
    test:
        go test -race -coverprofile=coverage.out ./...
    build:
        CGO_ENABLED=0 go build -o dist/server .

  В CI:
    - run: make lint
    - run: make test
    - run: make build

  ЧТО ЧАСТО ЛОМАЕТСЯ:
    • Тесты падают только в CI (race, время, окружение).
    • Линтер ругается на код, который прошёл локально (другая версия).
    • Образ собирается, но не запускается (CGO, отсутствует libc).
    • Cache ключ ломается (go.sum меняется в каждой сборке).

  ПРАВИЛО: CI должен запускать ровно то же, что ты делаешь локально. make test, make lint, make build. Не изобретай разное.

  17. АНТИПАТТЕРНЫ
  17.1. НЕТ КЭША. Go test каждый раз качает модули с нуля. Медленно и дорого.
  17.2. ВСЁ В ОДНОМ JOB'Е. Линтер, тесты, сборка — последовательно. Долго.
  17.3. НЕТ MATRIX. Тесты только на одной версии Go. Пропускаешь несовместимость.
  17.4. СЕКРЕТЫ В YAML. Видны всем. Только через secrets.
  17.5. ЛОГИРОВАТЬ СЕКРЕТЫ. Даже в debug. Утечка.
  17.6. GITHUB_TOKEN С ДЕФОЛТНЫМИ ПРАВАМИ. Дают больше, чем нужно. Ограничивай permissions.
  17.7. NO CONCURRENCY. 5 пушей = 5 параллельных запусков. Отменяй старые.
  17.8. PUSH ОБРАЗА НА PR. Не надо. push: false для PR.
  17.9. LATEST В ОБРАЗЕ. Фиксируй тег по sha или semver.
  17.10. НЕТ REUSABLE WORKFLOWS. Копипаста в 10 репо.
  17.11. ДЕПЛОЙ В ПРОД БЕЗ APPROVAL. Один коммит в main — прод обновлён.
  17.12. ДЕПЛОЙ С ЛОКАЛЬНОЙ МАШИНЫ. Только через CI/CD.
  17.13. ТЕСТЫ БЕЗ -RACE. Гонки ловятся только с race detector.
  17.14. ДОЛГИЕ CI (>15 МИНУТ). Ускоряй: кэш, matrix параллельно, paths-ignore.
  17.15. НЕТ BADGE В README. Видно только тем, кто заходит в Actions.
  17.16. НЕ ИСПОЛЬЗОВАТЬ ENVIRONMENTS. Prod secrets в общих secrets.
  17.17. СБОРКА DOCKER БЕЗ BUILDX. Нет multi-arch, нет кэша layers.
  17.18. ОБРАЗ ТОЛЬКО LINUX/AMD64. Apple Silicon не сможет запустить локально.

  18. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  CI/CD автоматизирует тесты, линтер, сборку, публикацию. Без CI/CD — руками и с ошибками.
  2.  GitHub Actions — самый простой вход для Go-проекта в GitHub.
  3.  Workflow = jobs + steps. Jobs параллельны, steps последовательны.
  4.  Triggers: push, pull_request, schedule, workflow_dispatch.
  5.  Runners: ubuntu-latest обычно достаточно. Self-hosted — только если нужно.
  6.  actions/checkout, actions/setup-go, actions/cache — базовый набор.
  7.  cache: true в setup-go — обязательно для скорости.
  8.  Matrix — тесты на нескольких версиях Go/OS. Ловит несовместимость.
  9.  Secrets для токенов. GITHUB_TOKEN для ghcr.io. Environment secrets для окружений.
  10. Environments с reviewers для прода. Деплой в прод — только через approve.
  11. Reusable workflows и composite actions — против копипасты.
  12. Concurrency с cancel-in-progress — для PR. Экономит минуты.
  13. Artifacts — временные файлы. GitHub Releases — для релизов.
  14. ghcr.io — бесплатный registry. docker/build-push-action + metadata-action — стандарт.
  15. Практика: lint → test (matrix) → build → docker push. Плюс release на тег.
  16. Go-разработчик: Dockerfile multi-stage, race detector, coverage, golangci-lint, Makefile.
*/
