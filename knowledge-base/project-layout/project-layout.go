package projectlayout

//ОСНОВЫ
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. Пакет main:
   Это точка входа. Если в папке лежит файл с `package main`, то компилятор Go
   понимает, что результатом сборки должен быть исполняемый файл (бинарник).
   Если package НЕ main — это библиотека (библиотечный пакет).

2. $GOPATH vs Go Modules:
   Раньше (до 2018 года) все проекты ДОЛЖНЫ были лежать в одной папке $GOPATH/src.
   Сейчас стандарт — Go Modules (файл go.mod). Теперь проект может лежать
   ГДЕ УГОДНО на диске. На собесе скажи: "Мы используем Go Modules, $GOPATH в прошлом".

3. Соглашения по именованию:
   - Имена пакетов: только строчные буквы, коротко, без подчеркиваний.
   - Тесты: файлы всегда заканчиваются на _test.go и лежат в той же папке, что и код.
*/

//ПРИМЕР 1: Минимальная "плоская" структура (для мелких утилит)
/*
my-tool/
├── go.mod        <- Описание зависимостей
├── main.go       <- package main
├── parser.go     <- package main (вспомогательный код)
└── parser_test.go
*/

//ПРИМЕР 2: Классический стартовый проект (Разделение cmd и логики)
/*
my-app/
├── go.mod
├── cmd/
│   └── server/   <- Подпапка для конкретного бинарника
│       └── main.go
├── server.go     <- Основная логика (package app)
└── server_test.go
*/

//ПРИМЕР 3: "Internal" — Секретное оружие Go
/*
my-project/
├── internal/     <- Код отсюда НЕЛЬЗЯ импортировать в другие проекты
│   └── auth/     <- Инкапсуляция на уровне компилятора
│       └── logic.go
├── pkg/          <- (Опционально) Код, который можно отдавать другим
└── main.go
*/

//ПРИМЕР 4: Структура с подпакетами (Иерархия)
/*
weather-api/
├── go.mod
├── main.go       <- Создает объекты и запускает всё
├── client/       <- Пакет для работы с внешним API (weather.com)
│   └── http.go
└── storage/      <- Пакет для работы с БД
    └── postgres.go
*/

//ПРИМЕР 5: Организация тестов и моков
/*
auth-service/
├── service.go
├── service_test.go
├── mock_test.go  <- Вспомогательные структуры для тестов
└── testdata/     <- Стандартная папка для хранения фикстур (JSON, txt для тестов)
    └── user.json
*/

// 2. МОДУЛИ, КОНТРОЛЬ ДОСТУПА И СТРУКТУРА
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. CMD, PKG, INTERNAL — "Золотой стандарт":
   - /cmd: Точки входа. Здесь только main-пакеты. Один сервис = одна подпапка.
   - /internal: Код, который "заперт" внутри проекта. Компилятор Go запрещает
     импортировать пакеты из internal во внешние проекты. Это защита инкапсуляции.
   - /pkg: Код, который МОЖНО импортировать снаружи. Сейчас многие советуют
     избегать /pkg, если вы не пишете публичную библиотеку.

2. GO MODULES (go.mod):
   Это файл-паспорт проекта. Он фиксирует:
   - Имя модуля (например, github.com/user/project).
   - Версию Go.
   - Прямые и косвенные зависимости с их четкими версиями.

3. GO.SUM:
   Файл с контрольными суммами. Гарантирует, что скачанная зависимость
   не была подменена злоумышленником на лету.
*/

// ПРИМЕР 1: Промышленная структура микросервиса
/*
auth-service/
├── go.mod            <- Имя модуля: "github.com/mycompany/auth"
├── go.sum            <- Хеши зависимостей
├── cmd/
│   ├── server/       <- Основной API сервер
│   │   └── main.go
│   └── migrator/     <- Утилита для миграции БД (тоже бинарник)
│       └── main.go
├── internal/         <- БИЗНЕС-ЛОГИКА (закрыта от всех)
│   ├── auth/         <- Логика авторизации
│   └── storage/      <- Работа с Postgres/Redis
└── pkg/              <- ПУБЛИЧНЫЙ КОД (например, SDK для клиентов)
    └── client/
*/

// ПРИМЕР 2: Использование internal для защиты архитектуры
/* Если кто-то попытается сделать:
import "github.com/mycompany/auth/internal/storage"
в другом проекте — Go выдаст ошибку "use of internal package not allowed".
*/

// ПРИМЕР 3: Управление зависимостями через CLI
/*
Команды, которые ты обязан знать:
1. go mod init github.com/me/myapp  <- Создать новый модуль
2. go mod tidy                      <- Удалить лишние и добавить нужные зависимости
3. go mod vendor                    <- Скопировать все зависимости в локальную папку проекта
4. go get github.com/pkg/errors@v0.9.1 <- Скачать конкретную версию
*/

// ПРИМЕР 4: Git-интеграция и .gitignore
/*
Для Go проекта в Git обязательно игнорируем бинарники и кэш:
# .gitignore
bin/                  <- Скомпилированные файлы
vendor/               <- (Опционально) если не используете вендоринг
.env                  <- СЕКРЕТЫ (НИКОГДА НЕ КОММИТИМ)
*/

// ПРИМЕР 5: Локальные замены (Replace) в go.mod
/*
Иногда нужно подправить чужую библиотеку локально или работать над двумя
модулями одновременно без пуша в репозиторий:

// в go.mod:
replace github.com/other/lib => ../local_lib_copy
*/

// 3. ARCHITECTURE, LAYERS & MULTI-MODULE (GRADE 3)
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. ЧИСТАЯ АРХИТЕКТУРА (CLEAN ARCHITECTURE):
   Главная идея — зависимость направлена внутрь. Бизнес-логика (Domain)
   ничего не знает о том, как она доставляется (HTTP/gRPC) или где она
   хранится (Postgres/Redis). Это позволяет менять БД или фреймворк,
   не переписывая логику приложения.

2. СЛОИ ПРИЛОЖЕНИЯ:
   - Domain/Entity: Базовые структуры данных и бизнес-правила.
   - Usecase/Service: Сценарии использования (оркестрация логики).
   - Repository/Store: Интерфейсы для работы с данными.
   - Transport/Delivery: Внешний мир (Handlers, Controllers).

3. МНОГОМОДУЛЬНЫЕ ПРОЕКТЫ (MONOREPO):
   В больших компаниях несколько сервисов могут лежать в одном репозитории.
   Для этого используется Go Workspaces (go.work), позволяющий работать
   с несколькими go.mod файлами одновременно.
*/

// ПРИМЕР 1: Структура по Clean Architecture (Standard)
/*
app/
├── cmd/
│   └── main.go       <- DI (Dependency Injection) Container
├── internal/
│   ├── domain/       <- Models & Interfaces (НИКАКИХ импортов других папок)
│   │   └── user.go
│   ├── usecase/      <- Business Logic (импортит только domain)
│   │   └── user_login.go
│   ├── repository/   <- Implementation of store (Postgres, Mongo)
│   │   └── pg_user.go
│   └── transport/    <- HTTP/gRPC Handlers (импортит usecase)
│       └── rest/
└── pkg/
*/

// ПРИМЕР 2: Go Workspaces (go.work) для многомодульности
/* Если у тебя в одном репозитории есть общая библиотека и сервис:
/my-monorepo
├── go.work           <- Содержит: use ( ./service ; ./shared )
├── service/          <- go.mod (зависит от shared)
└── shared/           <- go.mod (общие утилиты)
*/

// ПРИМЕР 3: Dependency Injection (DI) в main.go
// main пишется так, чтобы зависимости "прокидывались" сверху вниз.
func main() {
	// 1. Слой данных
	// repo := repository.NewPostgres(dbPool)

	// 2. Слой логики (зависит от интерфейса репозитория)
	// service := usecase.NewUserUseCase(repo)

	// 3. Слой транспорта (зависит от интерфейса сервиса)
	// handler := transport.NewHTTPHandler(service)

	// Запуск...
}

// ПРИМЕР 4: Package by Feature (Альтернатива слоям)
/*
Для очень больших проектов слои (usecase, repo) могут стать свалкой.
Тогда используют разделение по ФИЧАМ:
internal/
├── billing/
│   ├── delivery.go
│   ├── service.go
│   └── repository.go
└── catalog/
    ├── delivery.go
    └── service.go
*/

// ПРИМЕР 5: Интеграция библиотек (FX / Dig / Wire)
// На экспертном уровне часто используют Uber Fx для автоматического DI.
/*
func main() {
    fx.New(
        fx.Provide(NewConfig, NewDB, NewRepository, NewService),
        fx.Invoke(RegisterHandlers),
    ).Run()
}
*/

// 4.EXPERT, DEVOPS & SCALABILITY
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. МАСШТАБИРУЕМОСТЬ И ПОДДЕРЖКА:
   Эксперт понимает, что структура должна позволять 10 разработчикам работать
   над одним репозиторием, не мешая друг другу. Это достигается через:
   - Высокую связность (Cohesion): всё, что относится к одной фиче, лежит рядом.
   - Низкую зацепленность (Coupling): пакеты общаются через интерфейсы.

2. СОБСТВЕННЫЕ БИБЛИОТЕКИ (INTERNAL SDK):
   В больших компаниях создается "платформа" (Core/Shared библиотека), которая
   унифицирует логи, метрики, работу с БД и HTTP-ответы для всех микросервисов.

3. DOCKER & KUBERNETES:
   Структура проекта должна учитывать контейнеризацию. Например, Dockerfile
   обычно лежит в корне, а скрипты деплоя (Helm charts) — в папке /deploy.

4. CI/CD И ТЕСТИРОВАНИЕ:
   Структура должна разделять Unit-тесты (рядом с кодом) и Integration/E2E-тесты
   (обычно в /tests или /e2e), чтобы пайплайны сборки работали быстро.
*/

// ПРИМЕР 1: Экспертная структура с учетом DevOps
/*
enterprise-service/
├── .github/          <- CI/CD Pipelines (GitHub Actions)
├── api/              <- Определение протоколов (Protobuf/OpenAPI)
├── build/            <- Скрипты сборки и Docker-файлы
│   └── package/
│       └── Dockerfile
├── cmd/
│   └── app/
├── deploy/           <- Инфраструктура (Helm, K8s manifests, Terraform)
│   └── charts/
├── internal/
│   ├── platform/     <- Своеобразный "внутренний SDK" компании (логи, телеметрия)
│   └── domain/       <- Чистая логика
├── scripts/          <- Makefile, скрипты миграции
└── tests/            <- Интеграционные и нагрузочные тесты
*/

// ПРИМЕР 2: Унификация через Makefile (Стандарт сборки)
/*
На экспертном уровне ручной запуск go build запрещен. Всё через Makefile:
# Makefile
build:
	go build -o ./bin/app ./cmd/app

test-unit:
	go test -v -short ./internal/...

docker-build:
	docker build -t my-app:latest -f build/package/Dockerfile .
*/

// ПРИМЕР 3: Оптимизированный Dockerfile (Multi-stage build)
/*
Важно для структуры: Dockerfile должен быть в корне, чтобы видеть весь контекст.
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /main ./cmd/app

FROM alpine:latest
COPY --from=builder /main /main
ENTRYPOINT ["/main"]
*/

// ПРИМЕР 4: Организация Integration Tests (Separate package)
/*
Чтобы не раздувать основные пакеты, интеграционные тесты выносят отдельно.
tests/
└── integration/
    ├── user_db_test.go   <- Проверяет реальную связку с Postgres
    └── main_test.go      <- Настраивает окружение (TestMain)
*/

// ПРИМЕР 5: Кодогенерация (buf, swag, stringer)
/*
Эксперты автоматизируют всё. В папке /api лежат .proto файлы,
а генерация кода происходит в /internal/generated:
$ buf generate api/proto
*/
