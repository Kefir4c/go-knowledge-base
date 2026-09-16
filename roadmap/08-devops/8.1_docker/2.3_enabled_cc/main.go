package enabledcc

/*
  УРОК 2.3: CGO_ENABLED=0 И CROSS-COMPILATION
  Go известен тем, что компилирует статические бинарники, которые
  работают везде. Но это не происходит «само по себе». По умолчанию
  Go может использовать CGO — механизм вызова C-кода. Если CGO
  включён, бинарник линкуется с системной libc (glibc или musl).
  Это ломает переносимость и делает невозможным запуск на scratch
  или distroless.
  Cross-compilation — вторая половина истории. Go умеет собирать
  бинарники под любую архитектуру (amd64, arm64, armv7) без
  эмуляции. Это бесплатно и в разы быстрее, чем QEMU. Один
  Dockerfile — образы для Intel и ARM.
  Эти две темы — фундамент для продакшен-сборки Go-сервисов
  в контейнерах. Без CGO_ENABLED=0 нет минимальных образов.
  Без cross-compile нет multi-arch.

  СОДЕРЖАНИЕ:
    1.  Что такое CGO
    2.  Динамическая и статическая линковка
    3.  CGO_ENABLED=0 — что происходит
    4.  Что теряется при CGO_ENABLED=0
    5.  Когда CGO действительно нужен
    6.  Cross-compilation: GOOS и GOARCH
    7.  Cross-compile без QEMU — почему быстро
    8.  Multi-arch через buildx
    9.  Флаги линкера: -s, -w, -X, -trimpath
    10. Как проверить тип линковки
    11. Полный пример: Dockerfile для multi-arch
    12. Антипаттерны
    13. Финальные выводы

  1. ЧТО ТАКОЕ CGO
  CGO — это механизм Go, который позволяет вызывать C-код из Go
  и наоборот. Включается автоматически, если в проекте есть
  import "C" или используются пакеты, которые его используют.

  Пример кода с CGO:

    package main

    #include <stdio.h>
   import "C"

   func main() {
        C.printf(C.CString("hello from C\n"))
    }
  Что при этом происходит:
    • Go вызывает C-функцию printf.
    • При компиляции используется системный C-компилятор (gcc/clang).
    • Бинарник линкуется с системной libc (glibc или musl).
    • Результат — ДИНАМИЧЕСКИ СЛИНКОВАННЫЙ бинарник.

  Проблема: такой бинарник зависит от системной libc. Он не будет
  работать на другой системе, где libc другой версии или отсутствует.
  Не будет работать на scratch (там libc нет вообще).
  Не будет работать на distroless/static.

  ЧАСТО CGO ВКЛЮЧАЕТСЯ НЕЯВНО:
    • net пакет использует CGO для DNS-резолвинга (по умолчанию
      на Linux, если CGO доступен).
    • os/user использует CGO для getpwnam/getgrnam.
    • Некоторые сторонние библиотеки: SQLite (mattn/go-sqlite3),
      libgit2, image processing (libvips).

  То есть даже если ты не писал import "C", CGO может быть
  активен. И бинарник может быть динамически слинкован.

  2. ДИНАМИЧЕСКАЯ И СТАТИЧЕСКАЯ ЛИНКОВКА
  Линковка — это процесс объединения объектных файлов и библиотек
  в один исполняемый файл. Есть два способа.

  ДИНАМИЧЕСКАЯ ЛИНКОВКА:
    Бинарник не содержит код libc. Вместо этого он ссылается
    на системную библиотеку:

      ldd ./myapp
      # linux-vdso.so.1
      # libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6
      # /lib64/ld-linux-x86-64.so.2

    При запуске ядро ищет libc.so.6 и загружает её в память.
    Если libc нет — программа не запустится:

      error while loading shared libraries: libc.so.6:
      cannot open shared object file

  ПЛЮСЫ динамической:
    • Бинарник меньше (не тащит libc внутри).
    • libc можно обновлять отдельно (теоретически).

  МИНУСЫ:
    • Зависит от системы. Одна версия glibc на билде, другая
      на проде — «GLIBC_2.28 not found».
    • Не работает на scratch (там нет libc).
    • Не работает на distroless/static (там нет glibc).

  СТАТИЧЕСКАЯ ЛИНКОВКА:
    Весь код libc включён внутрь бинарника. Нет внешних зависимостей.

      ldd ./myapp
      # not a dynamic executable

    Или:
      file ./myapp
      # ELF 64-bit LSB executable, statically linked

  ПЛЮСЫ статической:
    • Работает на любом Linux без зависимостей.
    • Работает на scratch, distroless/static.
    • Нет «GLIBC_2.28 not found».
    • Простота деплоя.

  МИНУСЫ:
    • Бинарник больше (libc внутри).
    • Нельзя обновить libc без пересборки.
    • Некоторые функции недоступны (getpwnam, NSS).

  ДЛЯ GO: CGO_ENABLED=0 даёт статический бинарник.
  Это то, что нужно для минимальных контейнеров.

  3. CGO_ENABLED=0 — ЧТО ПРОИСХОДИТ
  Когда ты пишешь CGO_ENABLED=0 go build, происходит:
    • Go НЕ вызывает gcc/clang для C-компиляции.
    • Пакеты, использующие CGO, выбирают чистый Go-fallback.
    • net использует чистый Go DNS-резолвер вместо libc.
    • os/user использует /etc/passwd напрямую.
    • Бинарник линкуется статически.
    • Нет зависимости от libc.

  ПРОВЕРИТЬ:
    CGO_ENABLED=0 go build -o myapp .
    ldd myapp
    # not a dynamic executable

    file myapp
    # ELF 64-bit LSB executable, x86-64, statically linked

  БЕЗ CGO_ENABLED=0:

    go build -o myapp .
    ldd myapp
    # linux-vdso.so.1
    # libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6
    # /lib64/ld-linux-x86-64.so.2

    file myapp
    # ELF 64-bit LSB pie executable, dynamically linked,
    # interpreter /lib64/ld-linux-x86-64.so.2

  КАК ЗАДАТЬ CGO_ENABLED ГЛОБАЛЬНО:
    # Переменная окружения.
    export CGO_ENABLED=0

    # На один билд.
    CGO_ENABLED=0 go build .

    # В Dockerfile.
    ENV CGO_ENABLED=0
    RUN go build .

  ВАЖНО: CGO_ENABLED=0 должен быть указан В МОМЕНТ СБОРКИ,
  а не в рантайме. Это флаг компилятора, не переменная окружения
  контейнера.

  4. ЧТО ТЕРЯЕТСЯ ПРИ CGO_ENABLED=0
  Не всё так просто. CGO_ENABLED=0 имеет последствия.

  DNS-РЕЗОЛВИНГ:
    При CGO_ENABLED=1 Go использует libc getaddrinfo. Она умеет
    читать /etc/nsswitch.conf, /etc/hosts, работать с LDAP, NIS
    и другими системами именования.

    При CGO_ENABLED=0 Go использует чистый Go-резолвер. Он
    читает /etc/hosts и делает DNS-запросы напрямую. Но:
      • Не читает /etc/nsswitch.conf.
      • Не понимает LDAP, NIS.
      • Использует /etc/resolv.conf для DNS-серверов.

    Для контейнеров это обычно ок: DNS-резолвинг через /etc/hosts
    и /etc/resolv.conf работает. В k8s — через CoreDNS. Проблем нет.

    НО: если приложение в сложной корпоративной среде с LDAP —
    могут быть проблемы.

  OS/USER:
    При CGO_ENABLED=1 os/user использует getpwnam/getgrnam из libc.
    Работает с /etc/passwd, LDAP, NIS.

    При CGO_ENABLED=0 os/user читает /etc/passwd напрямую. Только
    локальные пользователи. LDAP не работает.

    Для контейнеров обычно не важно: нужен только root и appuser.

  RACE DETECTOR:
    go test -race требует CGO. Без CGO не работает.
    То есть: тесты с -race — с CGO, прод-бинарник — без CGO.

    В Dockerfile:
      # Тесты — с CGO.
      RUN CGO_ENABLED=1 go test -race ./...

      # Прод-бинарник — без CGO.
      RUN CGO_ENABLED=0 go build -o /out/server .

  SQLITE:
    mattn/go-sqlite3 требует CGO. С CGO_ENABLED=0 не работает.
    Альтернативы:
      • modernc.org/sqlite — чистый Go, работает с CGO_ENABLED=0.
      • Отказаться от SQLite в пользу Postgres.

  ДРУГИЕ БИБЛИОТЕКИ:
    Многие библиотеки с C-зависимостями:
      • libvips (обработка изображений).
      • libgit2 (git).
      • ffmpeg.
      • OpenCV.
      • libssh2.

    Все требуют CGO. В Go есть чистые аналоги, но не всегда
    с полной функциональностью.

  5. КОГДА CGO ДЕЙСТВИТЕЛЬНО НУЖЕН
  CGO_ENABLED=0 — это дефолт для контейнеров, но не серебряная
  пуля. Есть сценарии, где CGO обязателен.

  СЛУЧАЙ 1: SQLITE С MATTN/GO-SQLITE3.
    Если используешь эту библиотеку — CGO нужен. Варианты:
      • Перейти на modernc.org/sqlite (чистый Go).
      • Собрать с CGO и использовать alpine или debian-slim
        вместо distroless/scratch.

  СЛУЧАЙ 2: ОБРАБОТКА ИЗОБРАЖЕНИЙ/ВИДЕО.
    libvips, OpenCV, ffmpeg — требуют CGO. Для минимального
    образа придётся тащить библиотеки в alpine.

  СЛУЧАЙ 3: КРИПТОГРАФИЯ С АППАРАТНЫМ УСКОРЕНИЕМ.
    Некоторые крипто-операции (AES-NI, SHA-NI) быстрее через
    CGO + libcrypto. Но Go имеет ассемблерные реализации,
    которые тоже используют аппаратное ускорение. Так что
    CGO не обязателен.

  СЛУЧАЙ 4: LEGACY C-БИБЛИОТЕКИ.
    Если у тебя C-библиотека, для которой нет Go-аналога —
    CGO обязателен. Тогда собирай на alpine или debian-slim.

  В 90% случаев Go-сервисов CGO не нужен. Чистый Go покрывает
  всё: HTTP, gRPC, JSON, БД, шифрование, работу с сетью.

  6. CROSS-COMPILATION: GOOS И GOARCH
  Go компилируется под ЛЮБУЮ платформу. Это встроено в компилятор,
  никаких дополнительных тулов не нужно.

  ПЕРЕМЕННЫЕ:

    GOOS    — операционная система.
              linux, darwin, windows, freebsd, js, wasm.

    GOARCH  — архитектура процессора.
              amd64, arm64, arm, 386, riscv64, ppc64le, s390x.

  ПРИМЕРЫ:
    # Linux для Intel/AMD (серверы).
    GOOS=linux GOARCH=amd64 go build -o myapp .

    # Linux для ARM64 (Apple Silicon, AWS Graviton).
    GOOS=linux GOARCH=arm64 go build -o myapp-arm64 .

    # macOS для Apple Silicon.
    GOOS=darwin GOARCH=arm64 go build -o myapp-mac .

    # Windows для Intel.
    GOOS=windows GOARCH=amd64 go build -o myapp.exe .

    # ARMv7 (Raspberry Pi).
    GOOS=linux GOARCH=arm GOARM=7 go build -o myapp-pi .

    # RISC-V (экспериментально).
    GOOS=linux GOARCH=riscv64 go build -o myapp-riscv .

  ПОЛНЫЙ СПИСОК:
    go tool dist list

  Покажет все комбинации GOOS/GOARCH, которые поддерживает твой
  компилятор Go.

  КАК ЭТО РАБОТАЕТ:
    Go имеет встроенные реализации для каждой платформы. При
    сборке выбирается нужная реализация syscall-интерфейса.
    Никаких кросс-компиляторов не нужно.

  ВАЖНО: cross-compile работает ТОЛЬКО с CGO_ENABLED=0.
  С CGO нужен отдельный C-компилятор для целевой платформы.
  Это сложно настроить.

  Поэтому правило: CGO_ENABLED=0 + GOOS/GOARCH = бесплатный
  cross-compile.

  7. CROSS-COMPILE БЕЗ QEMU — ПОЧЕМУ БЫСТРО
  Есть два способа собрать бинарник под другую архитектуру.

  СПОСОБ 1: QEMU (ЭМУЛЯЦИЯ).
    Ты запускаешь ARM-контейнер на x86-хосте. QEMU эмулирует
    ARM-инструкции. Каждая инструкция транслируется в x86.

    ПЛЮСЫ:
      • Работает любой билд (Go, Python, Node.js, Rust).
      • Не нужно заботиться о cross-compile.

    МИНУСЫ:
      • В 5-10 раз медленнее.
      • Не всё эмулируется точно.
      • Требует настройки QEMU binfmt в Docker.

    ЗАМЕР: сборка Go-сервиса через QEMU — 60 секунд.
           Та же сборка через cross-compile — 6 секунд.

  СПОСОБ 2: CROSS-COMPILE (нативный).
    Ты говоришь Go: собери мне бинарник под ARM64. Go компилирует
    нативно, используя встроенные реализации. Никакой эмуляции.

    ПЛЮСЫ:
      • В 5-10 раз быстрее.
      • Работает в любом CI.
      • Не требует QEMU.
      • Никаких накладных расходов.

    МИНУСЫ:
      • Работает только для языков с нативным cross-compile
        (Go, Rust, C без CGO).
      • Для CGO нужен отдельный C-компилятор.

  В DOCKERFILE:
    docker buildx позволяет указать платформы:

      docker buildx build --platform linux/amd64,linux/arm64 ...

    Docker автоматически ставит ARG TARGETOS и TARGETARCH.
    В Dockerfile ты используешь их:

      ARG TARGETOS=linux
      ARG TARGETARCH=amd64

      RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH}
          go build -o /out/server .

    Для Go это работает БЕЗ QEMU. Docker подставит разные значения
    для разных платформ, и Go соберёт два разных бинарника за
    один прогон.

  АЛЬТЕРНАТИВА БЕЗ BUILDX: собрать отдельно.

    # Билд для amd64.
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server-amd64 .

    # Билд для arm64.
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o server-arm64 .

    # Собрать два Dockerfile или использовать ARG.

  8. MULTI-ARCH ЧЕРЕЗ BUILDX
  Buildx — расширение Docker для продвинутых сценариев сборки,
  включая multi-arch.

  ЧТО ТАКОЕ MULTI-ARCH ОБРАЗ:
    Один тег (например, my-app:1.0) указывает на НЕСКОЛЬКО
    manifest'ов для разных архитектур:

      my-app:1.0
        ├─ manifest amd64 (linux/amd64)
        ├─ manifest arm64 (linux/arm64)
        └─ manifest arm/v7 (linux/arm/v7)

    Когда ты делаешь docker pull my-app:1.0, Docker выбирает
    манифест по архитектуре хоста. На Mac M1 — arm64. На x86
    сервере — amd64.

  НАСТРОЙКА BUILDX:
    # Один раз создаём builder.
    docker buildx create --name mybuilder --use

    # Проверяем.
    docker buildx ls

  СБОРКА:
    docker buildx build \
      --platform linux/amd64,linux/arm64 \
      --target prod \
      --build-arg VERSION=1.0.0 \
      --push \
      -t myregistry/my-app:1.0.0 .

  ФЛАГ --push обязателен для multi-arch: результат нельзя
  сохранить только локально, нужно пушить в registry.

  ЧТО ПРОИСХОДИТ ВНУТРИ:
    • Docker собирает образ для amd64. Подставляет
      TARGETARCH=amd64.
    • Docker собирает образ для arm64. Подставляет
      TARGETARCH=arm64.
    • Оба бинарника Go компилирует нативно (без QEMU).
    • Оба манифеста пушатся в registry под одним тегом.

  БЕЗ QEMU — почему:
    Если бы внутри Dockerfile запускался Go-компилятор под ARM,
    Docker бы поднял ARM-контейнер через QEMU. Это медленно.

    Но мы используем cross-compile: Docker поднимает amd64-контейнер
    (нативный), и Go внутри генерирует ARM-бинарник. Быстро.

  В DOCKERFILE:
    FROM golang:1.22-alpine AS builder

    # Docker подставит эти ARG автоматически при buildx.
    ARG TARGETOS
    ARG TARGETARCH

    WORKDIR /src
    COPY . .

    RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
        go build -o /out/server .

  TARGETOS и TARGETARCH — это специальные ARG, которые buildx
  подставляет автоматически для каждой платформы.

  ПРОВЕРКА:
    # После пуша.
    docker buildx imagetools inspect myregistry/my-app:1.0.0

    # Manifests:
    #   Platform: linux/amd64
    #   Platform: linux/arm64

  В KUBERNETES:
    Kubelet сам выбирает подходящий манифест по архитектуре ноды.
    Тебе не нужно думать.

  9. ФЛАГИ ЛИНКЕРА: -S, -W, -X, -TRIMPATH
  Флаги линкера передаются через -ldflags в go build. Они
  уменьшают бинарник и вшивают метаданные.

  -S — ОТКЛЮЧИТЬ SYMBOL TABLE.

    Symbol table используется для дебага и stack traces. Убирает
    ~20-30% размера.

    Минус: stack traces становятся менее информативными
    (нет имён файлов и строк).

  -W — ОТКЛЮЧИТЬ DWARF DEBUG INFO.

    DWARF содержит полную информацию о типах, переменных, строках.
    Нужен для gdb и delve. Убирает ещё ~10-15%.

    Минус: gdb/delve не работают.

  -X — УСТАНОВИТЬ ЗНАЧЕНИЕ ПЕРЕМЕННОЙ.

    Вшивает версию и commit в бинарник.

      -X main.version=1.0.0
      -X main.commit=abc123

    В коде:
      var (
          version = "dev"
          commit  = "none"
      )

    После сборки:
      ./server --version
      # version=1.0.0 commit=abc123

  -TRIMPATH — УБРАТЬ ЛОКАЛЬНЫЕ ПУТИ.

    В бинарнике могут быть пути типа /home/user/project/main.go.
    -trimpath заменяет их на module/path/main.go. Безопаснее
    (не палит структуру директорий) и чуть-чуть меньше.

  ПОЛНАЯ КОМАНДА:

    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -ldflags="-s -w -X main.version=1.0.0 -X main.commit=abc123" \
      -trimpath \
      -o /out/server .

  ЭФФЕКТ:
    Без флагов:              25 МБ.
    С -ldflags="-s -w":      15 МБ.
    С -trimpath:             14 МБ.
    Со всем:                 14 МБ + вшитая версия.

  UPX (ОПЦИОНАЛЬНО):
    UPX — упаковщик бинарников. Уменьшает в 2-3 раза.

      upx --best --lzma server

    ПЛЮСЫ:
      • Бинарник 5 МБ вместо 14 МБ.
    МИНУСЫ:
      • Требует установки upx.
      • Бинарник распаковывается в память (больше RAM).
      • Некоторые антивирусы флагают UPX.
      • Может ломать CGO.
      • Медленнее старт.

    ПРАВИЛО: не используй UPX для Go-сервисов без веской причины.
    -s -w даёт достаточно.

  10. КАК ПРОВЕРИТЬ ТИП ЛИНКОВКИ
  Есть несколько способов проверить, статический ли бинарник.

  FILE:
    file ./server
    # ELF 64-bit LSB executable, x86-64, statically linked

    # Или:
    # ELF 64-bit LSB pie executable, dynamically linked

  LDD:
    ldd ./server
    # not a dynamic executable       ← статический
    # linux-vdso.so.1                ← динамический
    # libc.so.6 => ...

  READELF:
    readelf -d ./server | grep NEEDED
    # (пусто)      ← статический
    # (NEEDED) Shared library: [libc.so.6]   ← динамический

  ЧЕРЕЗ DOCKER:
    docker run --rm -v $(pwd):/app alpine file /app/server

  В GO:
    go version -m ./server
    # ./server: go1.22
    # path: myproject
    # mod: myproject  (devel)
    # build: -buildmode=exe
    # build: -compiler=gc
    # build: -ldflags="-s -w"

    Показывает, как бинарник был собран.

  В КОНТЕЙНЕРЕ:
    # Статический — работает на scratch.
    docker run --rm -v $(pwd):/app scratch /app/server --version
    # version=1.0.0

    # Динамический — падает на scratch.
    docker run --rm -v $(pwd):/app scratch /app/server --version
    # exec /app/server: no such file or directory
    # (потому что нет libc.so.6 и /lib64/ld-linux)

  11. ПОЛНЫЙ ПРИМЕР: DOCKERFILE ДЛЯ MULTI-ARCH
  Продакшен Dockerfile, всё вместе:

    # STAGE 1: builder
    FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

    # git для приватных репо.
    RUN apk add --no-cache git ca-certificates

    WORKDIR /src

    # Зависимости отдельным слоем — кэшируются.
    COPY go.mod go.sum* ./
    RUN --mount=type=cache,target=/go/pkg/mod \
        go mod download

    # Код.
    COPY . .

    # ARG от buildx.
    ARG TARGETOS
    ARG TARGETARCH
    ARG VERSION=dev
    ARG COMMIT=none

    # Cross-compile.
    RUN --mount=type=cache,target=/go/pkg/mod \
        --mount=type=cache,target=/root/.cache/go-build \
        CGO_ENABLED=0 \
        GOOS=${TARGETOS} \
        GOARCH=${TARGETARCH} \
        go build \
          -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
          -trimpath \
          -o /out/server .

    # STAGE 2: prod
    FROM gcr.io/distroless/static-debian12:nonroot AS prod

    ARG VERSION=dev
    LABEL org.opencontainers.image.title="my-app"
    LABEL org.opencontainers.image.version="${VERSION}"

    COPY --from=builder /out/server /server

    USER nonroot:nonroot
    EXPOSE 8080
    ENV APP_ENV=production
    STOPSIGNAL SIGTERM

    ENTRYPOINT ["/server"]

  ЧТО ЗДЕСЬ ВАЖНО:

    FROM --platform=$BUILDPLATFORM golang:1.22-alpine

      Это критично для multi-arch. Означает: builder-контейнер
      запускается на платформе ХОСТА (не целевой). То есть на
      x86 CI запускается x86-контейнер, но внутри собирается
      arm64-бинарник через cross-compile.

      Без $BUILDPLATFORM Docker запустил бы arm64-контейнер
      через QEMU (медленно).

    ARG TARGETOS / TARGETARCH

      Автоматически подставляются buildx для каждой платформы.

    GOOS=${TARGETOS} GOARCH=${TARGETARCH}

      Go компилирует под целевую платформу.

    CGO_ENABLED=0

      Обязательно. Без него cross-compile не сработает
      (нужен бы C-компилятор под ARM).

  СБОРКА:
    docker buildx build \
      --platform linux/amd64,linux/arm64 \
      --target prod \
      --build-arg VERSION=1.0.0 \
      --build-arg COMMIT=$(git rev-parse --short HEAD) \
      --push \
      -t myregistry/my-app:1.0.0 .

  ПРОВЕРКА:
    docker buildx imagetools inspect myregistry/my-app:1.0.0

  12. АНТИПАТТЕРНЫ

  12.1. CGO БЕЗ ПРИЧИНЫ.
    Если не используешь C-библиотеки — CGO не нужен. Он только
    добавляет зависимости.
  12.2. ЗАБЫЛИ CGO_ENABLED=0 ДЛЯ DISTROLESS.
    Динамический бинарник не работает на distroless/static.
    Ошибка при старте: no such file or directory.
  12.3. CROSS-COMPILE С CGO.
    GOOS=linux GOARCH=arm64 + CGO_ENABLED=1 — не сработает.
    Нужен отдельный C-компилятор под ARM. Или CGO_ENABLED=0.
  12.4. НЕ ИСПОЛЬЗУЮТ -LDFLAGS="-S -W".
    Бинарник 25 МБ вместо 15 МБ. 40% лишнего.
  12.5. НЕ ИСПОЛЬЗУЮТ -TRIMPATH.
    В бинарнике пути /home/user/project/... Безопасность и
    лишний размер.
  12.6. ЗАБЫЛИ ARG TARGETOS/TARGETARCH.
    Cross-compile не сработает. Соберётся под платформу
    builder'а.
  12.7. FROM GOLANG:1.22 БЕЗ --PLATFORM=$BUILDPLATFORM.
    В multi-arch build Docker поднимает ARM-контейнер через
    QEMU. Медленно.
  12.8. UPX БЕЗ ПОНИМАНИЯ.
    Бинарник меньше, но распаковка в память. Может не работать
    с CGO. Антивирусы флагают.
  12.9. CROSS-COMPILE БЕЗ ТЕСТОВ НА ЦЕЛЕВОЙ ПЛАТФОРМЕ.
    Собралось под arm64 — не значит, что работает. Тестируй
    на целевой архитектуре или через QEMU.
  12.10. RUN RACE DETECTOR БЕЗ CGO.
    go test -race требует CGO. Не работает с CGO_ENABLED=0.
  12.11. LD_FLAGS БЕЗ ПРОВЕРКИ.
    -X main.version=1.0.0 не сработает, если переменная
    называется иначе или объявлена в другом пакете.
  12.12. ДИНАМИЧЕСКИЙ БИНАРНИК НА SCRATCH.
    Не запустится. Никаких сообщений, кроме no such file.

  13. ФИНАЛЬНЫЕ ВЫВОДЫ

  1.  CGO — механизм вызова C-кода из Go. Включается при
      import "C" или при использовании некоторых пакетов.
  2.  Без CGO_ENABLED=0 бинарник может быть динамически
      слинкован с glibc. Это ломает переносимость.
  3.  CGO_ENABLED=0 даёт статический бинарник. Работает
      на любом Linux, включая scratch и distroless.
  4.  CGO_ENABLED=0 — обязательное условие для продакшен-
      контейнеров Go.
  5.  Что теряется: race detector, CGO-библиотеки (mattn/sqlite3,
      libvips), LDAP-резолвинг. В 90% случаев не важно.
  6.  Если CGO действительно нужен — используй alpine или
      debian-slim. Не distroless/static.
  7.  Cross-compilation — встроена в Go. GOOS/GOARCH работают
      без QEMU.
  8.  Cross-compile работает только с CGO_ENABLED=0. С CGO
      нужен отдельный C-компилятор.
  9.  Cross-compile в 5-10 раз быстрее QEMU. В CI критично.
  10. Multi-arch через buildx: один Dockerfile, две архитектуры
      за один прогон.
  11. FROM --platform=$BUILDPLATFORM для builder — обязательно
      для multi-arch. Иначе QEMU.
  12. ARG TARGETOS/TARGETARCH — автоматически подставляются
      buildx. Используй в GOOS/GOARCH.
  13. Флаги линкера -s -w уменьшают бинарник на 30-40%.
  14. -X main.version=... вшивает версию и commit в бинарник.
      Проверяется через ./server --version.
  15. -trimpath убирает локальные пути. Безопаснее и меньше.
  16. UPX — опционально. Может ломать CGO, антивирусы
      флагают. Не используй без веской причины.
  17. Проверяй тип линковки через file, ldd, readelf -d.
      Или через `docker run scratch ./server`.
  18. Антипаттерны: CGO без причины, забыли CGO_ENABLED=0
      для distroless, cross-compile с CGO, нет -s -w, нет
      $BUILDPLATFORM, race detector без CGO.
  19. На собесе: «Все Go-сервисы собираем с CGO_ENABLED=0 —
      это даёт статический бинарник, который работает на
      distroless без libc. Cross-compile через GOOS/GOARCH
      без QEMU. Multi-arch через buildx с
      --platform=$BUILDPLATFORM. Флаги -s -w -trimpath и
      вшивание версии через -X. Итог — образ 12 МБ под
      amd64 и arm64».
*/
