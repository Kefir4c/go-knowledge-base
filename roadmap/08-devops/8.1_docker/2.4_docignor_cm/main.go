package main

/*
  УРОК 2.4: .DOCKERIGNORE И CACHE MOUNTS
  Два механизма, которые превращают медленную сборку в быструю.
  Оба кажутся мелочью, но дают в 10-50 раз больше эффекта, чем
  любая другая оптимизация Dockerfile.
  .dockerignore уменьшает build context — то, что отправляется
  в Docker daemon перед началом сборки. Без него .git, тестовые
  данные и мусор летят по сети на каждый билд.
  Cache mounts (BuildKit) сохраняют кэш сборки между билдами.
  Go-кэш модулей и кэш компиляции монтируются в контейнер, и
  второй билд не пересобирает всё с нуля.
  Без этих двух вещей типичная сборка Go-сервиса занимает
  минуты. С ними — секунды. В CI на сотнях билдов это
  превращается в часы экономии.

  СОДЕРЖАНИЕ:
    1.  Что такое build context
    2.  Проблема: что попадает в build context без .dockerignore
    3.  Как работает .dockerignore
    4.  Синтаксис и паттерны
    5.  Что включать в .dockerignore для Go
    6.  Как проверить размер build context
    7.  Что такое cache mounts
    8.  Два кэша Go: /go/pkg/mod и /root/.cache/go-build
    9.  Как использовать cache mounts в Dockerfile
    10. Разница с обычным слоем кэша
    11. Cache mounts в CI: cache export/import
    12. Замер скорости: до и после
    13. Взаимодействие с порядком слоёв
    14. Антипаттерны
    15. Финальные выводы

  1. ЧТО ТАКОЕ BUILD CONTEXT
  Когда ты пишешь docker build ., точку в конце — это путь
  к build context. Docker забирает ВСЁ, что лежит по этому
  пути, и отправляет в daemon. Только после этого начинается
  сборка.

  Что происходит:
    1. Docker CLI сканирует папку . и все подпапки.
    2. Собирает список файлов.
    3. Применяет .dockerignore (исключает указанное).
    4. Формирует tar-архив.
    5. Отправляет архив в Docker daemon.
    6. Daemon распаковывает и начинает выполнять Dockerfile.

  ВАЖНО: шаг 5 может быть очень дорогим. Если в папке лежит
  .git на 500 МБ — эти 500 МБ идут по сети в daemon. На каждый
  билд. Даже если Dockerfile их не использует.

  В удалённых сценариях (CI + удалённый Docker) это критично:
    • Локально daemon на той же машине — больно, но терпимо.
    • CI runner + удалённый Docker — 500 МБ по сети.
    • Docker Desktop на macOS/Windows — через виртуализацию,
      ещё медленнее.

  ПРАВИЛО: build context должен содержать только то, что
  реально нужно для сборки.

  2. ПРОБЛЕМА: ЧТО ПОПАДАЕТ В BUILD CONTEXT БЕЗ .DOCKERIGNORE
  Типичный Go-проект без .dockerignore:
    myapp/
    ├── .git/                ← 200 МБ истории
    ├── .gitignore
    ├── .idea/               ← IDE-мусор
    ├── .vscode/
    ├── vendor/              ← 50-200 МБ зависимостей
    ├── node_modules/        ← если есть frontend
    ├── docs/                ← Markdown-документация
    ├── coverage.out         ← 5 МБ
    ├── tmp/
    ├── .env                 ← СЕКРЕТЫ!
    ├── *.log                ← логи
    ├── go.mod
    ├── go.sum
    ├── main.go
    └── internal/
        └── ...

  Что летит в build context:
    • Вся git-история — 200 МБ.
    • vendor/ — если он есть, до 200 МБ.
    • node_modules — ещё больше.
    • Секреты из .env — они попадают в tar, а оттуда могут
      утечь в промежуточные слои (если ты их копируешь).
    • IDE-мусор, логи, тестовые данные.

  ИТОГО: build context легко раздувается до 500 МБ — 1 ГБ.

  Что реально нужно для сборки:
    • go.mod, go.sum.
    • Все .go файлы.
    • Иногда — Makefile, если он используется в RUN.
    • Прочие ассеты, которые реально копируются.

  Обычно — 1-5 МБ.
  РАЗНИЦА В 100-500 РАЗ.

  БЕЗ .DOCKERIGNORE:
    • Билд медленный (передача контекста).
    • Кэш инвалидируется чаще (любое изменение в .git ломает
      слой COPY . .).
    • Секреты могут попасть в образ.
    • Больше нагрузка на daemon.

  3. КАК РАБОТАЕТ .DOCKERIGNORE
  .dockerignore — обычный текстовый файл в корне build context
  (обычно рядом с Dockerfile). Содержит список паттернов файлов
  и папок, которые исключаются из контекста.

  Принцип работы:
    1. Docker сканирует build context.
    2. Читает .dockerignore.
    3. Исключает всё, что совпало с паттернами.
    4. Остальное — в tar.

  ВАЖНО:
    • Файлы, исключённые .dockerignore, недоступны для COPY.
      Если ты исключил .env, но попытаешься COPY .env — ошибка.
    • Исключения применяются на этапе CLI, до отправки в daemon.
      То есть .dockerignore экономит сеть.
    • Синтаксис похож на .gitignore, но есть отличия.
    • Файл должен называться ровно .dockerignore — с точкой.
    • Работает для docker build, docker buildx, docker compose build.

  ЧТО .DOCKERIGNORE НЕ ДЕЛАЕТ:
    • Не уменьшает финальный образ. Он влияет на то, что
      попадает в build context, а не в образ.
    • Не работает для COPY --from других stage. Внутри stage
      уже своя файловая система, .dockerignore не действует.
    • Не может исключить сам Dockerfile.

  4. СИНТАКСИС И ПАТТЕРНЫ
  Паттерны похожи на .gitignore, но не идентичны.

  БАЗОВЫЕ ПРАВИЛА:
    # Комментарий — строки, начинающиеся с #.
    # Пустые строки игнорируются.

    # Точное имя файла.
    .env

    # Папка (со слэшем на конце).
    vendor/
    .git/

    # Glob-паттерн.
    *.md
    *.log

    # Двойная звёздочка — любая вложенность.
    *+/node_modules

# Восклицательный знак — исключение из исключений.
*.md
!README.md      ← README.md всё-таки оставить

ОТЛИЧИЯ ОТ .GITIGNORE:
• В .dockerignore нет понятия «следить за порядком».
Все паттерны применяются одновременно.
• `!` работает так же, но с нюансами: если файл уже
исключён родительской папкой, `!` может не сработать.
Порядок иногда важен, но обычно нет.
• Нет поддержки .dockerignore в подпапках (в отличие
от .gitignore).

ГЛАВНЫЕ ПАТТЕРНЫ:
.git/                 вся git-история
.gitignore            сам файл
.github/              CI-конфиги
*.md                  Markdown
docs/                 документация
vendor/               вендоринг (если не нужен в билде)
node_modules/         если есть frontend
tmp/                  временные файлы
bin/                  собранные бинарники
dist/                 дистрибутивы
build/                артефакты сборки
.env                  СЕКРЕТЫ
.env.*
*.log                 логи
coverage.out          покрытие
*.test                тестовые бинарники
.idea/                IDE
.vscode/
.DS_Store             macOS
Thumbs.db             Windows

СЛОЖНЫЕ ПАТТЕРНЫ:
# Все файлы с расширением .md, кроме README.md.
*.md
!README.md

# Все .log в любой папке.
/*.log

  # Папки __pycache__ в любой вложенности.
  /__pycache__/

# Временные файлы редактора.
*~
*.swp
*.swo

5. ЧТО ВКЛЮЧАТЬ В .DOCKERIGNORE ДЛЯ GO
Минимальный набор для типичного Go-проекта:

# Git.
.git
.gitignore
.gitattributes

# CI/CD.
.github
.gitlab-ci.yml
.circleci

# IDE.
.idea
.vscode
*.swp
*.swo
*~

# Документация.
*.md
docs/
LICENSE
CHANGELOG.md

# Артефакты сборки.
bin/
dist/
build/
*.exe
*.test
coverage.out
coverage.html

# Локальные файлы.
tmp/
.env
.env.*
*.log

# Если vendor НЕ используется при сборке — исключить.
# Если используется (go build -mod=vendor) — НЕ исключать!
# vendor/

# Docker-related.
Dockerfile*
docker-compose*.yml
.dockerignore

ОБРАТИ ВНИМАНИЕ:
• Dockerfile* и .dockerignore в контексте не нужны. Они
нужны только Docker CLI.
• vendor/ — если используешь модули и go mod download,
исключай vendor. Если собираешь с -mod=vendor — оставляй.
• .env — секреты. Всегда исключай. Никогда не копируй
в образ.
• *.md — если у тебя в коде нет embed директив на .md
(например, embed.FS для документации), исключай.

ПРО EMBED:
Если используешь //go:embed на файлы .md, .yaml или что-то
ещё, что хочешь исключить — исключение сломает сборку.
Проверяй перед исключением.

6. КАК ПРОВЕРИТЬ РАЗМЕР BUILD CONTEXT
Есть несколько способов.

СПОСОБ 1: DOCKER BUILD С VERBOSE
docker build --progress=plain -t test . 2>&1 | head -20

Увидишь строку:
=> [internal] load build context
=> => transferring context: 245.32MB 0.5s
Показывает, сколько реально передано в daemon.

СПОСОБ 2: DOCKER BUILD С ПРОВЕРКОЙ
DOCKER_BUILDKIT=1 docker build --no-cache --progress=plain -t test . 2>&1 | grep "transferring"
Получишь размер контекста.

СПОСОБ 3: ЗАМЕРИТЬ РАЗМЕР ПАПКИ ВРУЧНУЮ
# Linux/macOS.
du -sh .
du -sh .git
du -sh vendor/

# PowerShell.
(Get-ChildItem -Recurse | Measure-Object -Property Length -Sum).Sum / 1MB
Быстрая оценка. Сравни с реальным размером контекста.

7. ЧТО ТАКОЕ CACHE MOUNTS
Cache mounts — фича BuildKit (новой системы сборки Docker).
Позволяет монтировать директорию в контейнер во время RUN,
но при этом НЕ включать её в финальный слой.

Зачем:
• go mod download складывает модули в /go/pkg/mod.
• go build складывает промежуточные объектные файлы
в /root/.cache/go-build.
• Без cache mounts эти директории становятся слоями
образа — растят размер.
• С cache mounts они сохраняются между билдами, но в образ
не попадают.

СХЕМА:

Без cache mounts:
RUN go mod download
# модули скачались, легли в /go/pkg/mod
# следующая инструкция RUN — модули уже в предыдущем
# слое образа (не перекачиваются)
# НО при изменении go.mod слой инвалидируется, и модули
# перекачиваются заново

С cache mounts:
RUN --mount=type=cache,target=/go/pkg/mod go mod download
# модули скачались, легли в /go/pkg/mod
# /go/pkg/mod — это отдельное хранилище, НЕ слой образа
# при изменении go.mod модули НЕ перекачиваются —
# те, что уже в кэше, используются
# в финальном образе /go/pkg/mod пустой (или отсутствует)

КЛЮЧЕВОЕ ОТЛИЧИЕ ОТ СЛОЯ:
Обычный слой образа: кэш сбрасывается, если что-то изменилось
в этом слое или выше. Живёт в контейнере/образе.

Cache mount: кэш сохраняется между билдами независимо.
Не входит в образ. Не сбрасывается от изменений.

АНАЛОГИЯ:
Слой образа = git-коммит. Изменение ломает всё, что после.

Cache mount = локальный node_modules. Пока package.json
не изменился — не переустанавливается. Даже если ты
меняешь код.

8. ДВА КЭША GO: /GO/PKG/MOD И /ROOT/.CACHE/GO-BUILD
У Go есть два важных места для кэша.

8.1. /GO/PKG/MOD — КЭШ МОДУЛЕЙ
Сюда go mod download кладёт скачанные модули.

Что внутри:
• Исходники всех зависимостей.
• Версии, хэши, метаданные.
• Может занимать сотни МБ при большом проекте.

Когда используется:
• go mod download — скачивает сюда.
• go build — читает отсюда, если модули есть.

Экономия от cache mount:
• Первый билд: скачивает всё.
• Второй билд (даже с новым go.mod): скачивает только
новые модули, остальные — из кэша.

Разница: 30 секунд vs 2 секунды на второй сборке.

8.2. /ROOT/.CACHE/GO-BUILD — КЭШ КОМПИЛЯЦИИ
Сюда Go кладёт промежуточные результаты компиляции.

Что внутри:
• Скомпилированные объектные файлы пакетов.
• Кэш результатов компиляции для повторного использования.
• Может занимать гигабайты на большом проекте.

Когда используется:
• go build читает и пишет сюда.

Экономия от cache mount:
• Первый билд: компилирует всё.
• Второй билд (после изменения одного файла): компилирует
только изменённый пакет, остальное — из кэша.

Разница: 30 секунд vs 2 секунды при сборке большого проекта.

8.3. КАК ПРОВЕРИТЬ В КОНТЕЙНЕРЕ
Если нужно посмотреть, что в кэше:
docker run --rm golang:1.22-alpine sh -c "
go mod download &&
ls -la /go/pkg/mod &&
ls -la /root/.cache/go-build
"

9. КАК ИСПОЛЬЗОВАТЬ CACHE MOUNTS В DOCKERFILE
Синтаксис: --mount=type=cache,target=<путь>.

БАЗОВЫЙ ПРИМЕР:
FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
--mount=type=cache,target=/root/.cache/go-build \
CGO_ENABLED=0 go build -o /out/server .

ЧТО ЗДЕСЬ ЧТО:
• Первый RUN — только кэш модулей. go mod download
складывает скачанное.
• Второй RUN — оба кэша. go build читает модули из
/go/pkg/mod и компилирует, складывая объектники
в /root/.cache/go-build.
• Оба кэша НЕ попадают в финальный образ.
• Оба кэша сохраняются между билдами независимо
от слоёв.

ДОПОЛНИТЕЛЬНЫЕ ПАРАМЕТРЫ MOUNT:
--mount=type=cache,target=/go/pkg/mod \
--mount=type=cache,target=/go/pkg/mod,sharing=locked \
--mount=type=cache,target=/go/pkg/mod,mode=0777 \
--mount=type=cache,target=/go/pkg/mod,uid=1000,gid=1000 \
--mount=type=cache,target=/go/pkg/mod,id=gomod

ЧТО ЗНАЧАТ:
sharing=locked — блокировка кэша между параллельными билдами.
По умолчанию shared — параллельные билды видят один кэш.
locked — второй билд ждёт первого. Для Go не обязательно.

mode=0777 — права. По умолчанию наследуются.

uid/gid — владелец. Нужно, если контейнер работает под non-root.

id=gomod — имя кэша. Нужно, если хочешь использовать
один кэш из разных Dockerfile.

ВАЖНО: BUILDKIT ОБЯЗАТЕЛЕН.

Cache mounts не работают со старым builder'ом Docker.
Нужен BuildKit.

Проверка:
docker buildx version      # должен быть установлен
docker info | grep BuildKit

В современном Docker Desktop BuildKit включён по умолчанию.
Если нет — переменная окружения DOCKER_BUILDKIT=1.

10. РАЗНИЦА С ОБЫЧНЫМ СЛОЕМ КЭША
В чём разница между cache mount и обычным кэшем слоёв? Кажется,
они делают одно и то же. Не совсем.

ОБЫЧНЫЙ СЛОЙ:
RUN go mod download

• Слой содержит результат команды в файловой системе образа.
• /go/pkg/mod становится частью образа.
• При изменении go.mod слой инвалидируется и пересобирается.
• Модули перекачиваются заново.
• Финальный образ содержит модули (мусор).

CACHE MOUNT:
RUN --mount=type=cache,target=/go/pkg/mod go mod download

• Слой НЕ содержит /go/pkg/mod. Только команду и её вывод.
• /go/pkg/mod — отдельное хранилище BuildKit.
• При изменении go.mod слой инвалидируется и пересобирается.
• НО модули не перекачиваются — они в кэше.
• Финальный образ НЕ содержит /go/pkg/mod.

СРАВНЕНИЕ:
┌────────────────────┬──────────────────┬──────────────────┐
│ Характеристика     │ Обычный слой     │ Cache mount      │
├────────────────────┼──────────────────┼──────────────────┤
│ В образе           │ Да               │ Нет              │
│ Инвалидация        │ При изм. выше    │ Не инвалидируется│
│ Экономия на 2-й    │ Мало             │ Много            │
│ сборке             │                  │                  │
│ Размер образа      │ Больше           │ Меньше           │
│ Работает без       │ Да               │ Нет (нужен       │
│ BuildKit           │                  │ BuildKit)        │
└────────────────────┴──────────────────┴──────────────────┘

ВЫВОД:
Cache mount — это как отдельный кэш рядом с образом, который
переиспользуется между билдами. Слой — как часть образа,
которая кэшируется только при неизменности выше.

11. CACHE MOUNTS В CI: CACHE EXPORT/IMPORT
В CI между разными запусками pipeline кэш обычно теряется.
Каждый запуск — новый runner, чистый Docker, пустой кэш.

Решение: cache export/import через registry.

КАК РАБОТАЕТ:
BuildKit умеет сохранять кэш в registry и восстанавливать
его из registry в следующий раз.

СИНТАКСИС:
docker buildx build \
--cache-from type=registry,ref=myregistry/my-app:buildcache \
--cache-to type=registry,ref=myregistry/my-app:buildcache,mode=max \
--target prod \
-t myregistry/my-app:1.0.0 \
--push \
.

ЧТО ЗНАЧАТ:
--cache-from — откуда брать кэш при старте билда.
BuildKit скачает предыдущий кэш из registry.

--cache-to — куда сохранять кэш после билда.
BuildKit зальёт новый кэш в registry.

mode=max — сохранять все слои, включая промежуточные.
По умолчанию mode=min сохраняет только финальные слои.

ДРУГИЕ BACKEND'Ы:
• type=local,dest=/path — сохранить локально.
• type=gha — GitHub Actions cache.
• type=s3 — S3-бакет.
• type=azblob — Azure Blob Storage.

ДЛЯ GITHUB ACTIONS:
- name: Build
uses: docker/build-push-action@v5
with:
context: .
target: prod
push: true
tags: my-app:1.0.0
cache-from: type=gha
cache-to: type=gha,mode=max

ДЛЯ GITLAB CI:
build:
script:
- docker buildx build
--cache-from type=registry,ref=$CI_REGISTRY_IMAGE:cache
--cache-to type=registry,ref=$CI_REGISTRY_IMAGE:cache,mode=max
--push
-t $CI_REGISTRY_IMAGE:$CI_COMMIT_SHA .

ЭФФЕКТ В CI:
Без cache export:    каждая сборка — с нуля (5 минут).
С cache export:      второй и последующие — секунды.

На 100 билдов в день: 8 часов vs 10 минут.

12. ВЗАИМОДЕЙСТВИЕ С ПОРЯДКОМ СЛОЁВ
Cache mounts НЕ ЗАМЕНЯЮТ правильный порядок слоёв. Они
дополняют его.

ПРАВИЛЬНЫЙ ПОРЯДОК:
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
--mount=type=cache,target=/root/.cache/go-build \
go build -o /out/server .

ПОЧЕМУ ЭТО ВАЖНО ДАЖЕ С CACHE MOUNTS:

• COPY go.mod → отдельный слой. Кэшируется пока go.mod
не меняется.
• go mod download → отдельный слой. Кэшируется.
• COPY . . → меняется при каждой правке кода.
• go build → пересобирается, но использует оба кэша.

БЕЗ ПРАВИЛЬНОГО ПОРЯДКА:

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod go mod download
RUN --mount=type=cache,target=/root/.cache/go-build go build ...

• COPY . . меняется → инвалидирует go mod download.
• go mod download перезапускается, но модули в кэше —
только echo-вывод.
• go build перезапускается, использует cache mount.

Функционально работает, но:
• Много лишних шагов в логах.
• Нельзя собрать, не скопировав код.
• Менее эффективно.

ПРАВИЛО:
Правильный порядок слоёв + cache mounts = максимальная
скорость.
Только cache mounts = работает, но менее эффективно.
Только правильный порядок = медленно на втором билде.

14. АНТИПАТТЕРНЫ
14.1. НЕТ .DOCKERIGNORE.
Build context 500 МБ. Каждый билд передаёт всё в daemon.
14.2. .GIT В BUILD CONTEXT.
История на 200 МБ летит в daemon на каждой сборке.
14.3. .ENV И СЕКРЕТЫ В КОНТЕКСТЕ.
Если случайно COPY . . — секреты в образе. Утечка.
14.4. VENDOR БЕЗ ПРИЧИНЫ.
Если используешь go mod download, vendor не нужен.
50-200 МБ лишнего контекста.
14.5. CACHE MOUNT БЕЗ BUILDKIT.
--mount=type=cache игнорируется старым builder'ом. Ничего
не кэшируется.
14.6. CACHE MOUNT НЕ НА ВСЕХ RUN.
Если go mod download без cache mount, а go build с ней —
модули не переиспользуются между билдами.
14.7. ТОЛЬКО /GO/PKG/MOD БЕЗ /ROOT/.CACHE/GO-BUILD.
Модули кэшируются, но компиляция — нет. Медленно.
14.8. CACHE MOUNT НА БОЛЬШУЮ ПАПКУ.
Если смонтировать /src в cache — можно потерять свежие
исходники. Не монтируй то, где живёт код.
14.9. НЕТ CACHE EXPORT/IMPORT В CI.
Каждый запуск pipeline — с нуля. Часы впустую.
14.10. CACHE MOUNT С ID ИЗ РАЗНЫХ DOCKERFILE.
Если id не совпадает — кэш разный. Путаница.
14.11. ИГНОРИРОВАНИЕ РАЗМЕРА КЭША.
Кэш модулей может вырасти до гигабайт. Регулярно чисти:
docker builder prune
14.12. COPY . . ПЕРЕД GO MOD DOWNLOAD.
Правильный порядок — часть оптимизации. Cache mounts
не отменяют его.
14.13. .DOCKERIGNORE НЕ В КОРНЕ КОНТЕКСТА.
Docker ищет его в корне build context. Если он лежит
в подпапке — не работает.

15. ФИНАЛЬНЫЕ ВЫВОДЫ
1.  Build context — всё, что отправляется в Docker daemon
перед сборкой. Без .dockerignore может быть 500 МБ.
2.  .dockerignore уменьшает контекст в 100 раз. Билд
быстрее, кэш реже инвалидируется, секреты не утекают.
3.  Синтаксис .dockerignore похож на .gitignore. Glob,
папки, !-исключения.
4.  Для Go исключай: .git, .idea, *.md, docs, vendor
(если не используешь), .env, bin, tmp, Dockerfile.
5.  Проверяй размер контекста через --progress=plain и
«transferring context: X MB».
6.  Cache mounts — фича BuildKit. Монтируют директорию
в контейнер во время RUN, но НЕ включают в образ.
7.  Два кэша Go: /go/pkg/mod (модули) и /root/.cache/go-build
(компиляция). Оба важны.
8.  Синтаксис: RUN --mount=type=cache,target=/go/pkg/mod.
9.  Cache mounts НЕ инвалидируются при изменении go.mod.
Второй билд использует модули из кэша.
10. Cache mounts уменьшают финальный образ — кэш не входит
в слои.
11. Порядок слоёв + cache mounts = максимальная скорость.
Правильный порядок не отменяется.
12. В CI — cache export/import в registry (или gha-кэш).
Без этого каждая сборка с нуля.
13. Замер: без cache mounts второй билд 25 сек, с ними
3 сек. Разница в 10 раз.
14. Антипаттерны: нет .dockerignore, .git в контексте,
secrets в контексте, cache mount без BuildKit,
только один кэш Go, нет cache export в CI.
15. Проверяй размер кэша через docker builder prune
--dry-run. Чисти регулярно.
*/
