package openapi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// Импорт сгенерированной документации (будет создана после swag init)
	_ "github.com/example/openapi-example/docs"
)

/*
  УРОК 6.8: OPENAPI / SWAGGER
  OpenAPI (ранее Swagger) — это спецификация для описания RESTful API.
  Позволяет описывать эндпоинты, параметры, тела запросов и ответов в JSON/YAML.
  Это контракт между клиентом и сервером, аналог .proto для gRPC.

  СОДЕРЖАНИЕ:
    1.  Что такое OpenAPI и зачем нужен
    2.  OpenAPI 2.0 vs 3.0 vs 3.1 — разбор версий
    3.  Два подхода: Code-First и Spec-First
    4.  Code-First: swaggo/swag (генерация из комментариев)
    5.  Spec-First: oapi-codegen (генерация из спецификации)
    6.  Swagger UI — интерактивная документация
    7.  Генерация клиентов и валидация
    8.  Интеграция с CI/CD
    9.  Сравнение инструментов
    10. OpenAPI 3.1 — что нового и почему это важно
    11. Глубокая настройка swaggo/swag (продвинутые теги)
    12. Глубокая настройка oapi-codegen (конфигурация, strict-сервер)
    13. Генерация клиентов на основе OpenAPI
    14. Валидация запросов по OpenAPI-схеме
    15. Интеграция с CI/CD
    16. Частые ошибки в production
    17. Расширенные ключевые выводы для собеседования

  1.  ЧТО ТАКОЕ OPENAPI И ЗАЧЕМ ОН НУЖЕН
  OpenAPI Specification (OAS) — это стандарт описания RESTful API, который
  позволяет описывать структуру API в формате JSON или YAML. Это даёт
  возможность автоматически генерировать документацию, клиентские и
  серверные SDK, а также проводить валидацию запросов и ответов.

  Swagger — это набор инструментов, работающих со спецификацией OpenAPI:
    • Swagger UI — интерактивная документация, которая позволяет выполнять
      запросы прямо из браузера.
    • Swagger Editor — редактор для написания спецификации с подсветкой
      синтаксиса и автодополнением.
    • Swagger Codegen — генератор клиентских и серверных SDK на множестве
      языков программирования.

  OpenAPI позволяет:
    • Описывать эндпоинты, параметры, тела запросов и ответов.
    • Документировать API в едином формате (JSON/YAML).
    • Генерировать интерактивную документацию (Swagger UI).
    • Генерировать клиентские SDK на разных языках.
    • Генерировать серверные заглушки (boilerplate).
    • Валидировать запросы и ответы по схеме.

  Аналогия: OpenAPI для REST API — это как .proto для gRPC. Это контракт
  между клиентом и сервером, который обеспечивает строгую типизацию и
  единообразие.

  2.  OPENAPI 2.0 VS 3.0 VS 3.1

  2.1. OpenAPI 2.0 (Swagger 2.0) — устаревшая версия.
    Это первая широко распространённая версия спецификации. Она всё ещё
    используется в некоторых проектах, но для новых проектов не рекомендуется.

    Особенности:
      • Поддерживает базовые возможности: paths, parameters, responses.
      • Не поддерживает oneOf, anyOf, allOf (композиция схем).
      • Не поддерживает примеры (examples) в параметрах и ответах.
      • Не поддерживает callback'и и webhook'и.
      • Не поддерживает компоненты (components) для переиспользования.

    Когда использовать: только для поддержки старых проектов.

  2.2. OpenAPI 3.0 — текущий стандарт.
    Это основная версия, которую используют в большинстве современных проектов.
    Она значительно расширяет возможности OpenAPI 2.0.

    Особенности:
      • Поддерживает oneOf, anyOf, allOf (объединения и пересечения схем).
      • Поддерживает примеры (examples) для параметров и ответов.
      • Поддерживает callback'и и webhook'и.
      • Поддерживает компоненты (components) для переиспользования схем,
        параметров, ответов и т.д.
      • Поддерживает серверы (servers) для разных окружений.
      • Поддерживает Security Scheme (OAuth2, JWT, API Key).

    Когда использовать: стандартный выбор для новых проектов.

  2.3. OpenAPI 3.1 — последняя версия.
    Это самая современная версия, которая полностью совместима с JSON Schema.

    Особенности:
      • Полная совместимость с JSON Schema 2020-12.
      • Поддерживает patternProperties, dependentSchemas.
      • Поддерживает $ref в любом месте.
      • Поддерживает webhook'и как отдельную секцию.
      • Поддерживает XML-описания.
      • Поддерживает массивы типов (type: [string, integer]).

    Когда использовать: для новых проектов, где нужна полная совместимость
    с JSON Schema.

  3.  ДВА ПОДХОДА: CODE-FIRST И SPEC-FIRST

  В экосистеме Go есть два основных подхода к работе с OpenAPI.

  3.1. Code-First (swaggo/swag) — сначала код, потом документация.
    Сначала пишешь код на Go с аннотациями (комментариями). Потом запускаешь
    генератор, который парсит комментарии и создаёт спецификацию OpenAPI.

    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
    │  Go код +       │    │  swag init      │    │  OpenAPI spec   │
    │  комментарии    │───▶│  (генерация)    │───▶│  (JSON/YAML)    │
    └─────────────────┘    └─────────────────┘    └─────────────────┘

    Преимущества:
      + Документация всегда актуальна (синхронизирована с кодом).
      + Не нужно писать спецификацию вручную.
      + Быстрый старт — просто добавляешь комментарии.
      + Меньше дублирования — всё в одном месте.

    Недостатки:
      - Комментарии могут захламлять код.
      - Сложно поддерживать большие и сложные API.
      - Генератор может не справиться со сложными схемами.
      - Нет возможности генерировать клиенты на других языках.

    Когда использовать: небольшие и средние проекты, быстрая разработка,
    проекты с одним языком.

  3.2. Spec-First (oapi-codegen) — сначала спецификация, потом код.
    Сначала пишешь OpenAPI-спецификацию (YAML/JSON). Потом запускаешь
    генератор, который создаёт Go-код (модели, сервер, клиент) на основе
    спецификации.

    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
    │  OpenAPI spec   │    │  oapi-codegen   │    │  Go код         │
    │  (YAML/JSON)    │───▶│  (генерация)    │───▶│  (модели,       │
    └─────────────────┘    └─────────────────┘    │   сервер,       │
                                                  │   клиент)       │
                                                  └─────────────────┘

    Преимущества:
      + Полный контроль над спецификацией.
      + Можно использовать для генерации клиентов на других языках.
      + Подходит для больших и сложных API.
      + Документация — это источник истины.
      + Генерация типобезопасного кода, включая валидацию.
      + Возможность использовать strict-сервер.

    Недостатки:
      - Нужно писать спецификацию вручную (или использовать инструменты).
      - При изменении API нужно обновлять спецификацию и перегенерировать код.
      - Более высокий порог входа.
      - Риск рассинхронизации при ручном изменении сгенерированного кода.

    Когда использовать: большие проекты, публичные API, микросервисы,
    проекты с несколькими языками.

  4.  CODE-FIRST: SWAGGO/SWAG (ГЕНЕРАЦИЯ ИЗ КОММЕНТАРИЕВ)

  swaggo/swag — самая популярная библиотека для генерации OpenAPI
  документации из Go-комментариев.

  4.1. Установка
    go install github.com/swaggo/swag/cmd/swag@latest

    Убедись, что $GOPATH/bin добавлен в PATH:
      export PATH=$PATH:$(go env GOPATH)/bin

  4.2. Базовые аннотации для всего API (в main.go или отдельном файле)
    // @title My API
    // @version 1.0
    // @description This is a sample API.
    // @termsOfService https://example.com/terms
    // @contact.name API Support
    // @contact.email support@example.com
    // @license.name MIT
    // @license.url https://opensource.org/licenses/MIT
    // @host localhost:8080
    // @BasePath /api/v1
    // @securityDefinitions.apikey BearerAuth
    // @in header
    // @name Authorization
    // @schemes http https

  4.3. Аннотации для эндпоинтов
    // GetUser godoc
    // @Summary Get user by ID
    // @Description Returns a single user by ID
    // @Tags users
    // @Accept json
    // @Produce json
    // @Param id path int true "User ID"
    // @Param fields query string false "Comma-separated fields to return" example(id,name)
    // @Param X-Request-Id header string false "Request ID"
    // @Success 200 {object} User "User found"
    // @Failure 400 {object} ErrorResponse "Invalid request"
    // @Failure 404 {object} ErrorResponse "User not found"
    // @Failure 500 {object} ErrorResponse "Internal server error"
    // @Router /users/{id} [get]
    // @Security BearerAuth
    // @x-codeSamples [{"lang": "curl", "source": "curl -X GET /users/1"}]
    func GetUser(c *gin.Context) { ... }

  4.4. Основные теги
    ┌─────────────────────┬─────────────────────────────────────────────────┐
    │ Тег                 │ Описание                                        │
    ├─────────────────────┼─────────────────────────────────────────────────┤
    │ @Summary            │ Краткое описание метода                         │
    ├─────────────────────┼─────────────────────────────────────────────────┤
    │ @Description        │ Полное описание                                 │
    ├─────────────────────┼─────────────────────────────────────────────────┤
    │ @Tags               │ Теги для группировки                            │
    ├─────────────────────┼─────────────────────────────────────────────────┤
    │ @Accept             │ MIME-типы, которые принимает метод              │
    ├─────────────────────┼─────────────────────────────────────────────────┤
    │ @Produce            │ MIME-типы, которые возвращает метод             │
    ├─────────────────────┼─────────────────────────────────────────────────┤
    │ @Param              │ Параметр: name, in, type, required, description │
    ├─────────────────────┼─────────────────────────────────────────────────┤
    │ @Success            │ Успешный ответ: код, тип, описание              │
    ├─────────────────────┼─────────────────────────────────────────────────┤
    │ @Failure            │ Ошибочный ответ: код, тип, описание             │
    ├─────────────────────┼─────────────────────────────────────────────────┤
    │ @Router             │ Путь и метод: /users/{id} [get]                 │
    ├─────────────────────┼─────────────────────────────────────────────────┤
    │ @Security           │ Требуемая аутентификация                        │
    └─────────────────────┴─────────────────────────────────────────────────┘

  4.5. Аннотации для моделей (структур)
    type User struct {
        ID    int    `json:"id" example:"1"`
        Name  string `json:"name" example:"John Doe"`
        Email string `json:"email" example:"john@example.com"`
        Address Address `json:"address"`
    }
    type Address struct {
        Street string `json:"street" example:"123 Main St"`
        City   string `json:"city" example:"New York"`
    }
    type ErrorResponse struct {
        Code    int    `json:"code" example:"400"`
        Message string `json:"message" example:"Invalid request"`
        Details string `json:"details,omitempty"`
    }

  4.6. Генерация спецификации
    # В корне проекта
    swag init

    # С указанием главного файла
    swag init -g main.go

    # С указанием выходной директории
    swag init -o ./docs

    # Генерация OpenAPI 3.1
    swag init --v3.1 -o ./docs

    # С полными опциями
    swag init --parseDependency --parseInternal --generatedTime --output ./docs --v3.1

  4.7. Что генерируется
    После генерации появляется папка docs/ с файлами:
      • docs.go — Go-код с встроенной спецификацией.
      • swagger.json — спецификация в JSON.
      • swagger.yaml — спецификация в YAML.

  4.8. Интеграция с Gin
    import (
        "github.com/gin-gonic/gin"
        swaggerFiles "github.com/swaggo/files"
        ginSwagger "github.com/swaggo/gin-swagger"
        _ "github.com/yourproject/docs"
    )

    r := gin.Default()
    r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

  4.9. Интеграция с net/http
    import (
        httpSwagger "github.com/swaggo/http-swagger/v2"
        _ "github.com/yourproject/docs"
    )

    func main() {
        r := http.NewServeMux()
        r.Handle("GET /swagger/", httpSwagger.WrapHandler)
        // ... другие handlers
    }

  5.  SPEC-FIRST: OAPI-CODGEN (ГЕНЕРАЦИЯ ИЗ СПЕЦИФИКАЦИИ)

  oapi-codegen — инструмент для генерации Go-кода из OpenAPI-спецификации.
  Он активно используется в продакшене и протестирован на тысячах реальных
  OpenAPI-спецификаций.

  5.1. Установка
    go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

    Или для форка от DoorDash (более свежий):
      go install github.com/doordash-oss/oapi-codegen-dd/v3/cmd/oapi-codegen@latest

  5.2. Пример OpenAPI-спецификации (openapi.yaml)
    openapi: 3.0.0
    info:
      title: User API
      version: 1.0.0
    paths:
      /users/{id}:
        get:
          operationId: getUser
          parameters:
            - name: id
              in: path
              required: true
              schema:
                type: integer
          responses:
            '200':
              description: OK
              content:
                application/json:
                  schema:
                    $ref: '#/components/schemas/User'
    components:
      schemas:
        User:
          type: object
          properties:
            id:
              type: integer
            name:
              type: string
            email:
              type: string

  5.3. Генерация кода
    # Генерация моделей и серверной заглушки для Chi
    oapi-codegen -package api -generate types,chi-server -o api.gen.go openapi.yaml

    # Генерация только моделей
    oapi-codegen -package models -generate types -o models.gen.go openapi.yaml

    # Генерация клиента
    oapi-codegen -package client -generate client -o client.gen.go openapi.yaml

  5.4. Параметры генерации
    -generate types        — Go-структуры для схем
    -generate chi-server   — сервер для Chi-роутера
    -generate gin-server   — сервер для Gin
    -generate echo-server  — сервер для Echo
    -generate fiber-server — сервер для Fiber
    -generate std-http-server — сервер для net/http
    -generate client       — HTTP-клиент
    -generate spec         — встраивание спецификации в код

  5.5. Поддерживаемые фреймворки (13 штук)
    oapi-codegen поддерживает 13 популярных фреймворков:
      • Chi (chi-server)
      • Gin (gin-server)
      • Echo (echo-server)
      • Fiber (fiber-server)
      • std-http (std-http-server)
      • gorilla-mux (gorilla-server)
      • fasthttp (fasthttp-server)
      • Iris (iris-server)
      • Beego (beego-server)
      • go-zero (go-zero-server)
      • Kratos (kratos-server)
      • GoFrame (goframe-server)
      • Hertz (hertz-server)

  5.6. Strict-сервер — типобезопасные обработчики
    Strict-сервер — это продвинутый режим генерации, который создаёт
    типобезопасные обработчики с явными типами для параметров и ответов.

    // Сгенерированный интерфейс
    type StrictServerInterface interface {
        GetUser(ctx context.Context, request GetUserRequestObject) (GetUserResponseObject, error)
    }

    // Реализация
    func (s *Server) GetUser(ctx context.Context, request GetUserRequestObject) (GetUserResponseObject, error) {
        // request содержит все параметры: path, query, headers, body
        user := &User{Id: request.Id, Name: "John"}
        return GetUser200JSONResponse(*user), nil
    }

    // Подключение
    func main() {
        router := chi.NewRouter()
        server := &Server{}
        handler := HandlerFromMux(server, router)
        http.ListenAndServe(":8080", handler)
    }

  5.7. Конфигурационный файл (oapi-codegen.yaml)
    # oapi-codegen.yaml
    package: api
    generate:
      chi-server: true
      models: true
      embedded-spec: true
    output: api.gen.go

    # Генерация
    oapi-codegen --config=oapi-codegen.yaml openapi.yaml

  6.  SWAGGER UI — ИНТЕРАКТИВНАЯ ДОКУМЕНТАЦИЯ

  Swagger UI — это инструмент, который визуализирует OpenAPI-спецификацию
  в виде интерактивной документации, позволяя:
    • Просматривать все эндпоинты, параметры, схемы.
    • Выполнять запросы прямо из браузера.
    • Видеть примеры запросов и ответов.
    • Тестировать API без дополнительных инструментов.

  6.1. Интеграция Swagger UI (code-first подход)
    // В main.go после генерации документации
    import (
        "github.com/gin-gonic/gin"
        swaggerFiles "github.com/swaggo/files"
        ginSwagger "github.com/swaggo/gin-swagger"
        _ "github.com/yourproject/docs"
    )

    func main() {
        r := gin.Default()
        r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
        r.Run(":8080")
    }

    // Доступ: http://localhost:8080/swagger/index.html

  6.2. Кастомизация Swagger UI
    swaggerConfig := &swaggo.Config{
        URL: "http://localhost:8080/swagger/doc.json",
        DeepLinking: true,
        DocExpansion: "list", // "list", "none", "full"
        DefaultModelsExpandDepth: 1,
    }
    r.GET("/swagger/*any", ginSwagger.CustomWrapHandler(swaggerConfig, swaggerFiles.Handler))

  7.  ГЕНЕРАЦИЯ КЛИЕНТОВ И ВАЛИДАЦИЯ

  7.1. Зачем генерировать клиентов
    • Типобезопасность — ошибки обнаруживаются на этапе компиляции.
    • Автодополнение в IDE.
    • Единообразие — все клиенты используют один контракт.
    • Экономия времени — не нужно писать HTTP-клиент вручную.

  7.2. Генерация клиента через oapi-codegen
    oapi-codegen -package client -generate client -o client.gen.go openapi.yaml

  7.3. Использование клиента
    client := NewClient("https://api.example.com")
    ctx := context.Background()

    // Вызов с параметрами
    resp, err := client.GetUser(ctx, &GetUserParams{Id: 1})

    // Вызов с телом
    createResp, err := client.CreateUser(ctx, &CreateUserJSONRequestBody{
        Name:  "John",
        Email: "john@example.com",
    })

  7.4. Генерация клиентов на других языках
    openapi-generator generate -i openapi.yaml -g typescript -o ./typescript-client
    openapi-generator generate -i openapi.yaml -g python -o ./python-client
    openapi-generator generate -i openapi.yaml -g java -o ./java-client

  7.5. Валидация запросов по OpenAPI-схеме
    Strict-сервер oapi-codegen автоматически валидирует:
      • Типы параметров (path, query, header).
      • Типы тела запроса.
      • Обязательность полей.

    Если валидация не пройдена, strict-сервер возвращает 400
    без вызова бизнес-логики.

  8.  ИНТЕГРАЦИЯ С CI/CD

  8.1. Автоматическая генерация документации
    # .github/workflows/docs.yml
    name: Generate OpenAPI docs
    on:
      push:
        branches: [main]
    jobs:
      generate:
        runs-on: ubuntu-latest
        steps:
          - uses: actions/checkout@v3
          - uses: actions/setup-go@v4
            with:
              go-version: '1.21'
          - name: Install swag
            run: go install github.com/swaggo/swag/cmd/swag@latest
          - name: Generate docs
            run: swag init --v3.1 -o ./docs
          - name: Upload docs
            uses: actions/upload-artifact@v3
            with:
              name: openapi-docs
              path: docs/

  8.2. Проверка Breaking Changes (spectral)
    # Установка spectral
    npm install -g @stoplight/spectral-cli

    # Проверка изменений
    spectral diff old-spec.yaml new-spec.yaml

  8.3. Генерация клиентов в CI
    oapi-codegen -generate client -o client.gen.go openapi.yaml
    openapi-generator generate -i openapi.yaml -g typescript -o ./typescript-client

  9.  СРАВНЕНИЕ ИНСТРУМЕНТОВ
  ┌─────────────────────┬──────────────────┬──────────────────┬──────────────────┐
  │ Характеристика      │ swaggo/swag      │ go-swagger       │ oapi-codegen     │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Подход              │ Code-First       │ Both             │ Spec-First       │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ OpenAPI 2.0         │ Да               │ Да               │ Нет              │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ OpenAPI 3.0         │ Да               │ Нет              │ Да               │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ OpenAPI 3.1         │ Да (--v3.1)      │ Нет              │ Да (v2.8.0+)     │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Генерация сервера   │ Нет              │ Да               │ Да               │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Генерация клиента   │ Нет              │ Да               │ Да               │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Strict-сервер       │ Нет              │ Нет              │ Да               │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Популярность        │ Очень высокая    │ Высокая          │ Растущая         │
  └─────────────────────┴──────────────────┴──────────────────┴──────────────────┘

  10.  ЧАСТЫЕ ОШИБКИ В PRODUCTION
  1. Документация не синхронизирована с кодом.
  2. Использование OpenAPI 2.0 в новых проектах.
  3. Нет описания ошибок (400, 404, 500).
  4. Нет примеров (examples) для параметров и ответов.
  5. Нет security-схем.
  6. Спецификация хранится отдельно от кода (нарушение синхронизации).
  7. Нет тегов для группировки эндпоинтов.
  8. Слишком общие описания без деталей.
  9. Не используется валидация по схеме.
  10. Не генерируются клиенты — вручную пишутся HTTP-запросы.
  11. Нет проверки breaking changes в CI.
  12. Использование устаревших версий swaggo/swag.
  13. Не используется OpenAPI 3.1 (где это уместно).
  14. Игнорирование security схем в документации.
  15. Не документированы query-параметры.

  11. КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ

  1. OpenAPI — стандарт описания RESTful API (аналог .proto для gRPC).
  2. Swagger — набор инструментов (UI, Editor, Codegen).
  3. OpenAPI 3.1 — полная совместимость с JSON Schema 2020-12,
      webhooks, улучшенная работа с $ref.
  4. swaggo/swag — Code-First: генерация из комментариев.
      Поддерживает OpenAPI 3.1 через флаг --v3.1.
  5. oapi-codegen — Spec-First: генерация из спецификации.
      Поддерживает OpenAPI 3.1 и strict-сервер.
  6. go-swagger — устаревший инструмент, только OpenAPI 2.0.
      Использовать не рекомендуется для новых проектов.
  7. Strict-сервер oapi-codegen — типобезопасные обработчики
      с автоматической валидацией параметров и тела.
  8. Генерация клиентов — типобезопасные HTTP-клиенты
      из OpenAPI-спецификации.
  9. Валидация — защита от некорректных данных на раннем этапе.
  10. CI/CD — автоматическая генерация документации и клиентов,
      проверка breaking changes.
  11. Для новых проектов выбирай OpenAPI 3.1 + swaggo/swag
      (Code-First) или oapi-codegen (Spec-First).
  12. Для больших публичных API — oapi-codegen с strict-сервером.
  13. Для быстрой разработки — swaggo/swag.
  14. Всегда синхронизируй документацию с кодом (в CI).
  15. Документируй все возможные ошибки (400, 404, 500).
*/

//МОДЕЛИ
// User — модель пользователя.
// @Description Пользователь системы.

type User struct {
	ID        string    `json:"id" example:"1" description:"Уникальный идентификатор"`
	Name      string    `json:"name" example:"John Doe" description:"Имя пользователя"`
	Email     string    `json:"email" example:"john@example.com" description:"Email пользователя"`
	CreatedAt time.Time `json:"created_at" example:"2024-01-01T00:00:00Z" description:"Дата создания"`
	UpdatedAt time.Time `json:"updated_at" example:"2024-01-01T00:00:00Z" description:"Дата обновления"`
}

// CreateUserRequest — запрос на создание пользователя.
// @Description Запрос на создание нового пользователя.
type CreateUserRequest struct {
	Name  string `json:"name" binding:"required" example:"John Doe" description:"Имя пользователя"`
	Email string `json:"email" binding:"required,email" example:"john@example.com" description:"Email пользователя"`
}

// UpdateUserRequest — запрос на обновление пользователя.
// @Description Запрос на обновление пользователя.
type UpdateUserRequest struct {
	Name  string `json:"name" example:"John Doe" description:"Имя пользователя"`
	Email string `json:"email" example:"john@example.com" description:"Email пользователя"`
}

// ErrorResponse — стандартный ответ с ошибкой.
// @Description Ответ при ошибке.
type ErrorResponse struct {
	Code    int    `json:"code" example:"400" description:"HTTP код ошибки"`
	Message string `json:"message" example:"Invalid request" description:"Текст ошибки"`
	Details string `json:"details,omitempty" example:"field 'name' is required" description:"Дополнительные детали"`
}

//ХРАНИЛИЩЕ

var (
	users   = make(map[string]*User)
	usersMu sync.RWMutex
	idGen   = 1
)

func init() {
	// Тестовые данные
	now := time.Now()
	users["1"] = &User{ID: "1", Name: "Alice", Email: "alice@ex.com", CreatedAt: now, UpdatedAt: now}
	users["2"] = &User{ID: "2", Name: "Bob", Email: "bob@ex.com", CreatedAt: now, UpdatedAt: now}
	idGen = 3
}

//ОБРАБОТЧИКИ

// GetUsers godoc
// @Summary Получить список всех пользователей
// @Description Возвращает список всех зарегистрированных пользователей
// @Tags users
// @Accept json
// @Produce json
// @Success 200 {array} User "Список пользователей"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка"
// @Router /users [get]
// @Security BearerAuth
func GetUsers(c *gin.Context) {
	usersMu.RLock()
	defer usersMu.RUnlock()

	result := make([]*User, 0, len(users))
	for _, u := range users {
		result = append(result, u)
	}
	c.JSON(http.StatusOK, result)
}

// GetUser godoc
// @Summary Получить пользователя по ID
// @Description Возвращает одного пользователя по его ID
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "ID пользователя" minlength(1)
// @Success 200 {object} User "Пользователь найден"
// @Failure 400 {object} ErrorResponse "Неверный ID"
// @Failure 404 {object} ErrorResponse "Пользователь не найден"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка"
// @Router /users/{id} [get]
// @Security BearerAuth
func GetUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "id is required",
		})
		return
	}

	usersMu.RLock()
	user, ok := users[id]
	usersMu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    http.StatusNotFound,
			Message: "user not found",
		})
		return
	}
	c.JSON(http.StatusOK, user)
}

// CreateUser godoc
// @Summary Создать нового пользователя
// @Description Создаёт нового пользователя с указанными именем и email
// @Tags users
// @Accept json
// @Produce json
// @Param request body CreateUserRequest true "Данные пользователя"
// @Success 201 {object} User "Пользователь создан"
// @Failure 400 {object} ErrorResponse "Неверные данные"
// @Failure 409 {object} ErrorResponse "Пользователь с таким email уже существует"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка"
// @Router /users [post]
// @Security BearerAuth
func CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Details: err.Error(),
		})
		return
	}

	usersMu.Lock()
	defer usersMu.Unlock()

	// Проверка на дубликат email
	for _, u := range users {
		if u.Email == req.Email {
			c.JSON(http.StatusConflict, ErrorResponse{
				Code:    http.StatusConflict,
				Message: "user with this email already exists",
			})
			return
		}
	}

	id := fmt.Sprintf("%d", idGen)
	idGen++
	now := time.Now()
	user := &User{
		ID:        id,
		Name:      req.Name,
		Email:     req.Email,
		CreatedAt: now,
		UpdatedAt: now,
	}
	users[id] = user
	c.JSON(http.StatusCreated, user)
}

// UpdateUser godoc
// @Summary Обновить пользователя
// @Description Обновляет данные существующего пользователя
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "ID пользователя" minlength(1)
// @Param request body UpdateUserRequest true "Данные для обновления"
// @Success 200 {object} User "Пользователь обновлён"
// @Failure 400 {object} ErrorResponse "Неверные данные"
// @Failure 404 {object} ErrorResponse "Пользователь не найден"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка"
// @Router /users/{id} [put]
// @Security BearerAuth
func UpdateUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "id is required",
		})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Details: err.Error(),
		})
		return
	}

	usersMu.Lock()
	defer usersMu.Unlock()

	user, ok := users[id]
	if !ok {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    http.StatusNotFound,
			Message: "user not found",
		})
		return
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	user.UpdatedAt = time.Now()
	c.JSON(http.StatusOK, user)
}

// DeleteUser godoc
// @Summary Удалить пользователя
// @Description Удаляет пользователя по ID
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "ID пользователя" minlength(1)
// @Success 204 "Пользователь удалён"
// @Failure 400 {object} ErrorResponse "Неверный ID"
// @Failure 404 {object} ErrorResponse "Пользователь не найден"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка"
// @Router /users/{id} [delete]
// @Security BearerAuth
func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "id is required",
		})
		return
	}

	usersMu.Lock()
	defer usersMu.Unlock()

	if _, ok := users[id]; !ok {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    http.StatusNotFound,
			Message: "user not found",
		})
		return
	}
	delete(users, id)
	c.Status(http.StatusNoContent)
}

// AuthMiddleware — имитация аутентификации (для демонстрации security)
// @Description Проверяет наличие Bearer токена в заголовке Authorization
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Code:    http.StatusUnauthorized,
				Message: "missing authorization header",
			})
			c.Abort()
			return
		}
		// В реальном проекте здесь валидация JWT
		// Для примера пропускаем всё, кроме пустого токена
		if len(auth) > 7 && auth[:7] == "Bearer " {
			c.Next()
			return
		}
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    http.StatusUnauthorized,
			Message: "invalid token format",
		})
		c.Abort()
	}
}

// @title User API
// @version 1.0.0
// @description REST API для управления пользователями с OpenAPI документацией
// @termsOfService https://example.com/terms
// @contact.name API Support
// @contact.email support@example.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8080
// @BasePath /api/v1
// @schemes http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Введите "Bearer <token>". Для примера подойдёт любой токен.

// @tag.name users
// @tag.description Операции с пользователями

//go:generate swag init -g main.go -o docs --v3.1

func main() {
	//Настройка Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Группа API с аутентификацией
	api := r.Group("/api/v1")
	api.Use(AuthMiddleware())
	{
		api.GET("/users", GetUsers)
		api.GET("/users/:id", GetUser)
		api.POST("/users", CreateUser)
		api.PUT("/users/:id", UpdateUser)
		api.DELETE("/users/:id", DeleteUser)
	}

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Запуск сервера
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("🚀 Server started on http://localhost:8080")
		log.Printf("📚 Swagger UI: http://localhost:8080/swagger/index.html")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Received shutdown signal")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	log.Println("Server stopped")

}
