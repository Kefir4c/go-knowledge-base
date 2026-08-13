package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

/*
  УРОК 6.2. РОУТЕРЫ ЭКОСИСТЕМЫ
  1. ЭВОЛЮЦИЯ РОУТИНГА В GO: ОТ net/http ДО СТОРОННИХ БИБЛИОТЕК

  Изначально стандартный пакет net/http предлагал базовый маршрутизатор
  (ServeMux), который работал только с фиксированными путями. Для динамических
  параметров разработчики использовали регулярные выражения или ручной парсинг.
  Это было неудобно и вело к дублированию кода.

  С появлением Go 1.22 встроенный ServeMux получил поддержку методов и
  параметров пути, но всё ещё не дотягивает до сторонних решений в плане
  гибкости, производительности и экосистемы.

  Сторонние роутеры решают следующие задачи:
    - динамическая маршрутизация с параметрами,
    - группировка маршрутов и middleware,
    - оптимизация под высокую нагрузку,
    - удобные методы для работы с запросами и ответами,
    - интеграция с валидацией, сериализацией и т.д.

  2. ВНУТРЕННЕЕ УСТРОЙСТВО РОУТЕРОВ (RADIX TREE И ДРУГИЕ СТРУКТУРЫ)

  Все три роутера используют в своей основе эффективные структуры данных
  для поиска маршрутов:

  - Chi: использует собственную реализацию на основе trie (префиксное дерево)
          с поддержкой параметров. Код очень компактный и понятный.

  - Echo: использует Radix Tree (сжатое префиксное дерево), что позволяет
          достичь высокой скорости поиска (O(k), где k — длина пути).

  - Gin: также основан на Radix Tree, но дополнительно оптимизирован с
         использованием unsafe для ускорения работы с контекстом.

  Работа роутера происходит в несколько этапов:
    1. Разбор HTTP-запроса (метод, путь, заголовки).
    2. Поиск узла в дереве маршрутов по методу и пути.
    3. Извлечение параметров пути (если есть).
    4. Вызов цепочки middleware, а затем обработчика.

  Важно: все три роутера поддерживают группировку маршрутов, что позволяет
  применять middleware к целым группам, а не к каждому маршруту отдельно.

  3. ДЕТАЛЬНЫЙ РАЗБОР CHI

  Архитектура:
    - Полностью совместим с net/http (использует http.Handler и http.HandlerFunc).
    - Роутер chi реализует интерфейс http.Handler, поэтому его можно легко
      встраивать в любой HTTP-сервер.

  Особенности реализации:
    - Внутренний роутер основан на map (по методам) и trie (по путям).
    - Для извлечения параметров используется chi.URLParam(r, key).
    - Поддерживает вложенные роутеры (Mount) и группы (Route).

  Плюсы:
    - 100% идиоматичность, код выглядит как стандартный net/http.
    - Очень маленький размер (~1000 строк), легко читать и модифицировать.
    - Отличная документация и примеры.
    - Активное сообщество, но меньше плагинов, чем у Gin.

  Минусы:
    - Чуть ниже производительность (но для 95% проектов это не критично).
    - Выделяет память под запрос (не zero-alloc).

  Ключевые middleware из пакета chi/middleware:
    - RequestID: добавляет уникальный ID к каждому запросу.
    - Logger: логирует метод, путь, статус и время.
    - Recoverer: восстанавливает после паники.
    - RealIP: извлекает реальный IP из заголовков (X-Forwarded-For).
    - URLFormat: поддерживает формат .json, .xml и т.д.
    - Throttle: ограничение количества одновременных запросов.
    - Compress: сжатие ответов.

  4. ДЕТАЛЬНЫЙ РАЗБОР ECHO

  Архитектура:
    - Основан на собственном контексте echo.Context, который предоставляет
      богатый набор методов для работы с запросом и ответом.
    - Обработчики возвращают error (стандартный подход в Go).
    - Использует внешний валидатор (например, go-playground/validator).

  Особенности реализации:
    - Контекст реализован эффективно, содержит много полезных методов:
        c.Bind(&v)   — привязка JSON/XML/Form к структуре.
        c.JSON(code, v) — отправка JSON-ответа.
        c.Param(key) — получение параметра пути.
        c.QueryParam(key) — получение query-параметра.
    - Поддерживает HTTP/2, WebSocket, шаблоны.
    - Встроенная валидация через структуры.

  Плюсы:
    - Очень высокая производительность (zero-alloc).
    - Богатый функционал «из коробки» (не нужны дополнительные библиотеки).
    - Чистый и интуитивный API.
    - Не использует unsafe, что делает код безопаснее.

  Минусы:
    - Меньше совместимости с net/http (нужно использовать echo.Context).
    - Чуть сложнее для новичков, чем Gin.

  Ключевые middleware:
    - Logger, Recover, CORS, JWT, RateLimiter, BodyLimit, Gzip, Static.

  5. ДЕТАЛЬНЫЙ РАЗБОР GIN

  Архитектура:
    - Основан на контексте *gin.Context, который содержит всё необходимое
      для обработки запроса.
    - Использует высокооптимизированный роутер на Radix Tree.
    - Применяет unsafe для ускорения работы (спорное решение).

  Особенности реализации:
    - c.Param(key) — параметр пути.
    - c.Query(key) — query-параметр.
    - c.ShouldBind(&v) — привязка JSON/XML/Form/YAML.
    - c.String, c.JSON, c.HTML, c.File — удобные методы для ответов.
    - Поддерживает группировку и вложенные роутеры.

  Плюсы:
    - Самая высокая производительность (zero-alloc).
    - Огромное сообщество и тысячи плагинов.
    - Очень простой и быстрый старт.
    - Популярен среди новичков и стартапов.

  Минусы:
    - Использует unsafe (может быть проблемой в некоторых корпоративных средах).
    - Меньше идиоматичности (контекст не стандартный).
    - Бывает сложно поддерживать большие проекты из-за «магии».

  Ключевые middleware:
    - Logger, Recovery, CORS, JWT, RateLimiter, Static, Sessions.

  6. СРАВНЕНИЕ ПО КЛЮЧЕВЫМ МЕТРИКАМ (ПОДРОБНО)

  Производительность (бенчмарки на реальных API):
    - При 100 маршрутах с параметрами и обработкой JSON:
        Chi:   ~85 000 req/s,  ~6 ms задержка,  ~25 MB памяти на 1M запросов
        Echo:  ~125 000 req/s, ~4 ms задержка,  ~16 MB памяти
        Gin:   ~115 000 req/s, ~4.5 ms задержка, ~19 MB памяти

  Потребление памяти на запрос (Alloc/op):
    - Chi: ~1500 байт
    - Echo: ~100 байт (zero-alloc)
    - Gin: ~100 байт (zero-alloc)

  Количество выделений объектов (Allocs/op):
    - Chi: ~30
    - Echo: ~0 (при правильном использовании)
    - Gin: ~0

  Время компиляции и размер бинарника:
    - Chi: минимальный оверхед (код очень компактный).
    - Echo: небольшой оверхед из-за большей функциональности.
    - Gin: чуть больше из-за сложных оптимизаций.

  7. ЭКОСИСТЕМА И ПОПУЛЯРНОСТЬ

  - Chi: 18k звёзд, активно поддерживается, используется в крупных проектах,
          но сообщество меньше, чем у Gin.

  - Echo: 29k звёзд, стабильный и зрелый, используется в высоконагруженных
          системах (например, в некоторых сервисах Uber).

  - Gin: >80k звёзд, огромное сообщество, тысячи статей, плагинов и примеров.
          Самый популярный Go-фреймворк для API.

  8. КРИТЕРИИ ВЫБОРА ДЛЯ РЕАЛЬНЫХ ПРОЕКТОВ

  Если вы создаёте:
    - Микросервис с высокой нагрузкой → Echo или Gin.
    - Долгоживущий проект с акцентом на поддерживаемость → Chi.
    - Прототип для демонстрации → Gin.
    - API для внутреннего использования в корпорации → Chi (из-за совместимости).

  Также учитывайте:
    - Опыт команды: если все знают Gin, может быть проще его использовать.
    - Требования к безопасности: если unsafe запрещён → только Echo или Chi.
    - Необходимость в интеграции с другими системами: Chi лучше дружит с net/http.

  9. РЕКОМЕНДАЦИИ ПО АРХИТЕКТУРЕ ПРИ ИСПОЛЬЗОВАНИИ РОУТЕРОВ

  Независимо от выбора роутера, важно правильно организовать код:

    1. Выносите бизнес-логику в отдельные сервисы (не в хендлеры).
    2. Хендлеры должны быть тонкими: только приём/валидация запроса,
       вызов сервиса, формирование ответа.
    3. Используйте middleware для сквозной функциональности (лог, CORS, auth).
    4. Группируйте маршруты по модулям (users, orders, admin).
    5. Не забывайте про graceful shutdown.
    6. Покрывайте хендлеры тестами (используйте httptest).

  10. ОШИБКИ И ПОДВОДНЫЕ КАМНИ (ЧАСТО СПРАШИВАЮТ НА СОБЕСАХ)

  - В Gin и Echo контекст живёт только в пределах обработчика. Нельзя сохранять
    указатель на контекст для использования в горутинах — нужно передавать
    копию или использовать собственные механизмы.

  - При использовании unsafe в Gin будьте осторожны с обновлениями версий,
    так как внутренние структуры могут меняться.

  - В Chi все обработчики — стандартные http.HandlerFunc, поэтому их легко
    тестировать, но нужно самостоятельно обрабатывать ошибки и JSON.

  - Во всех роутерах middleware применяются в порядке объявления. Это важно
    для логирования, аутентификации и обработки ошибок.

  - Не забывайте про таймауты на уровне HTTP-сервера — они критичны для
    защиты от медленных клиентов.

  11. АЛЬТЕРНАТИВЫ: HTTPROUTER, FIBER, FASTHTTP

  - httprouter: используется в Gin как основа, очень быстрый, но минимальный.
  - Fiber: вдохновлён Express.js, быстрее Gin, но основан на fasthttp
           (не совместим с net/http).
  - fasthttp: очень быстрый, но имеет свои особенности и ограничения.

  Эти альтернативы тоже достойны внимания, но менее популярны в корпоративной
  среде из-за несовместимости с net/http и стандартными инструментами.

  12. БОНУС: КАК ВЫБРАТЬ РОУТЕР ДЛЯ ПРОЕКТА (ПОШАГОВО)

  Шаг 1: Оцените требования к производительности.
         Если >100k RPS — смотрите Echo или Gin.

  Шаг 2: Оцените требования к поддерживаемости и долгосрочности.
         Если проект будет жить >3 лет — лучше Chi.

  Шаг 3: Оцените опыт команды и экосистему.
         Если команда знает Gin и нужны готовые решения — берите Gin.

  Шаг 4: Учтите совместимость с существующими системами (middleware, логгеры).
         Если используются стандартные http.Handler — берите Chi.

  Шаг 5: Проверьте безопасность (unsafe) и возможные ограничения.
         Если unsafe запрещён — Echo или Chi.
*/

// respondJSON и respondError — универсальные для всех роутеров.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// УНИВЕРСАЛЬНАЯ ФУНКЦИЯ ЗАПУСКА
func runServers(servers map[string]*http.Server) {
	var wg sync.WaitGroup
	for name, srv := range servers {
		wg.Add(1)
		go func(name string, srv *http.Server) {
			defer wg.Done()
			log.Printf("[%s] запущен на %s", name, srv.Addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("[%s] ошибка: %v", name, err)
			}
		}(name, srv)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Получен сигнал остановки, завершаем работу...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for name, srv := range servers {
		log.Printf("[%s] остановка...", name)
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("[%s] ошибка при остановке: %v", name, err)
		}
	}
	wg.Wait()
	log.Println("Все серверы остановлены.")
}

//ПРИМЕР 1: БАЗОВЫЙ РОУТИНГ С ГРУППИРОВКОЙ

/*
ЗАЧЕМ: В реальных проектах всегда есть иерархия ресурсов (например, /api/v1/users,
       /api/v1/users/{id}/orders). Группировка позволяет применять общие middleware
       и избегать дублирования префиксов.

ФИШКА: Используем вложенные группы (Group в gin/echo) для создания иерархии.
       Параметры пути через :id в echo и :id в gin.
*/

// Gin часть
func primer1() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	v1 := r.Group("/api/v1")
	{
		users := v1.Group("/users")
		{
			users.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, []string{"user1", "user2"})
			})
			users.POST("", func(c *gin.Context) {
				c.JSON(http.StatusCreated, gin.H{"id": "123"})
			})
			users.GET("/:id", func(c *gin.Context) {
				id := c.Param("id")
				c.JSON(http.StatusOK, gin.H{"id": id, "name": "John"})
			})
			users.PUT("/:id", func(c *gin.Context) {
				id := c.Param("id")
				c.JSON(http.StatusOK, gin.H{"id": id, "name": "Updated"})
			})
			users.DELETE("/:id", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
			// Подресурс orders
			users.GET("/:id/orders", func(c *gin.Context) {
				userID := c.Param("id")
				c.JSON(http.StatusOK, gin.H{
					"user_id": userID,
					"orders":  []string{"order1"},
				})
			})
			users.POST("/:id/orders", func(c *gin.Context) {
				userID := c.Param("id")
				c.JSON(http.StatusCreated, gin.H{
					"user_id":  userID,
					"order_id": "456",
				})
			})
		}
	}
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	srv := &http.Server{Addr: ":8080", Handler: r}
	runServers(map[string]*http.Server{"gin": srv})
}

// Echo часть
func example1_echo() {
	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	v1 := e.Group("/api/v1")
	{
		users := v1.Group("/users")
		{
			users.GET("", func(c echo.Context) error {
				return c.JSON(http.StatusOK, []string{"user1", "user2"})
			})
			users.POST("", func(c echo.Context) error {
				return c.JSON(http.StatusCreated, map[string]string{"id": "123"})
			})
			users.GET("/:id", func(c echo.Context) error {
				id := c.Param("id")
				return c.JSON(http.StatusOK, map[string]string{"id": id, "name": "John"})
			})
			users.PUT("/:id", func(c echo.Context) error {
				id := c.Param("id")
				return c.JSON(http.StatusOK, map[string]string{"id": id, "name": "Updated"})
			})
			users.DELETE("/:id", func(c echo.Context) error {
				return c.NoContent(http.StatusNoContent)
			})
			users.GET("/:id/orders", func(c echo.Context) error {
				userID := c.Param("id")
				return c.JSON(http.StatusOK, map[string]interface{}{
					"user_id": userID,
					"orders":  []string{"order1"},
				})
			})
			users.POST("/:id/orders", func(c echo.Context) error {
				userID := c.Param("id")
				return c.JSON(http.StatusCreated, map[string]string{
					"user_id":  userID,
					"order_id": "456",
				})
			})
		}
	}
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	srv := &http.Server{Addr: ":8081", Handler: e}
	runServers(map[string]*http.Server{"echo": srv})
}

//ПРИМЕР 2: РАБОТА С ПАРАМЕТРАМИ И ВАЛИДАЦИЯ

/*
ЗАЧЕМ: В продакшене нужно не просто получить параметр, а проверить его формат,
       преобразовать в нужный тип и вернуть понятную ошибку при невалидных данных.

ФИШКА: В gin используем ShouldBindQuery и структуры с тегами binding.
       В echo используем c.Bind и структуры с тегами validate (или ручной парсинг).
*/

// Gin часть
func primer2() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	type UserParams struct {
		ID     int64  `uri:"id" binding:"required,min=1"`
		Limit  int    `form:"limit" binding:"min=1"`
		Offset int    `form:"offset" binding:"min=0"`
		Sort   string `form:"sort"`
		Order  string `form:"order"`
	}

	r.GET("/api/users/:id", func(c *gin.Context) {
		var params UserParams
		if err := c.ShouldBindUri(&params); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}
		if err := c.ShouldBindQuery(&params); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
			return
		}
		// Дефолты
		if params.Sort == "" {
			params.Sort = "created_at"
		}
		if params.Order == "" {
			params.Order = "desc"
		}
		c.JSON(http.StatusOK, gin.H{
			"id":     params.ID,
			"limit":  params.Limit,
			"offset": params.Offset,
			"sort":   params.Sort,
			"order":  params.Order,
		})
	})
	srv := &http.Server{Addr: ":8082", Handler: r}
	runServers(map[string]*http.Server{"gin": srv})
}

// Echo часть
func example2_echo() {
	e := echo.New()

	e.GET("/api/users/:id", func(c echo.Context) error {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		}
		limit := c.QueryParam("limit")
		limitInt := 10
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			limitInt = l
		}
		offset := c.QueryParam("offset")
		offsetInt := 0
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			offsetInt = o
		}
		sort := c.QueryParam("sort")
		if sort == "" {
			sort = "created_at"
		}
		order := c.QueryParam("order")
		if order == "" {
			order = "desc"
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"id": id, "limit": limitInt, "offset": offsetInt, "sort": sort, "order": order,
		})
	})
	srv := &http.Server{Addr: ":8083", Handler: e}
	runServers(map[string]*http.Server{"echo": srv})
}

//ПРИМЕР 3: MIDDLEWARE ДЛЯ ЛОГИРОВАНИЯ И ВОССТАНОВЛЕНИЯ

/*
ЗАЧЕМ: Логирование каждого запроса помогает отлаживать и мониторить систему.
       Recover защищает от паник, чтобы сервер не падал.

ФИШКА: Используем готовые middleware из пакетов. В gin — gin.Logger() и gin.Recovery().
       В echo — echomw.Logger() и echomw.Recoverer(). Добавляем request ID.
*/

// Gin часть
func primer3() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Кастомный логгер с request ID
	r.Use(func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		c.Set("request_id", reqID)
		c.Writer.Header().Set("X-Request-ID", reqID)
		c.Next()
	})
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/api/data", func(c *gin.Context) {
		reqID, _ := c.Get("request_id")
		c.JSON(http.StatusOK, gin.H{
			"data":       "ok",
			"request_id": reqID,
		})
	})
	srv := &http.Server{Addr: ":8084", Handler: r}
	runServers(map[string]*http.Server{"gin": srv})
}

// Echo часть
func example3_echo() {
	e := echo.New()

	// Request ID middleware
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqID := c.Request().Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = fmt.Sprintf("%d", time.Now().UnixNano())
			}
			c.Response().Header().Set("X-Request-ID", reqID)
			c.Set("request_id", reqID)
			return next(c)
		}
	})
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	e.GET("/api/data", func(c echo.Context) error {
		reqID := c.Get("request_id").(string)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"data":       "ok",
			"request_id": reqID,
		})
	})
	srv := &http.Server{Addr: ":8085", Handler: e}
	runServers(map[string]*http.Server{"echo": srv})
}

//ПРИМЕР 4: РАБОТА С JSON И ОШИБКАМИ

/*
ЗАЧЕМ: Единый формат ответов (всегда JSON с полем "error") упрощает интеграцию.
       Централизованные функции сокращают дублирование кода.

ФИШКА: Используем встроенные методы c.JSON и c.ShouldBindJSON (gin),
       c.Bind и c.JSON (echo). Возвращаем структурированные ошибки.
*/

// Gin часть
func primer4() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	type UserRequest struct {
		Name  string `json:"name" binding:"required"`
		Email string `json:"email" binding:"required,email"`
	}

	r.POST("/api/users", func(c *gin.Context) {
		var req UserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "123", "name": req.Name})
	})

	r.GET("/api/users/:id", func(c *gin.Context) {
		id := c.Param("id")
		if id == "0" {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id, "name": "John"})
	})
	srv := &http.Server{Addr: ":8086", Handler: r}
	runServers(map[string]*http.Server{"gin": srv})
}

// Echo часть
func example4_echo() {
	e := echo.New()

	type UserRequest struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	e.POST("/api/users", func(c echo.Context) error {
		var req UserRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid json"})
		}
		if req.Name == "" || req.Email == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "name and email required"})
		}
		return c.JSON(http.StatusCreated, map[string]string{"id": "123", "name": req.Name})
	})

	e.GET("/api/users/:id", func(c echo.Context) error {
		id := c.Param("id")
		if id == "0" {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
		}
		return c.JSON(http.StatusOK, map[string]string{"id": id, "name": "John"})
	})
	srv := &http.Server{Addr: ":8087", Handler: e}
	runServers(map[string]*http.Server{"echo": srv})
}

//ПРИМЕР 5: КАСТОМНЫЕ 404 И 405

/*
ЗАЧЕМ: В продакшене дефолтные ответы 404/405 выглядят непрофессионально.
       Нужно возвращать структурированный JSON.

ФИШКА: В gin используем NoRoute и NoMethod. В echo используем HTTPErrorHandler.
*/

// Gin часть
func primer5() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "not found",
			"path":  c.Request.URL.Path,
		})
	})
	r.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error":  "method not allowed",
			"method": c.Request.Method,
			"path":   c.Request.URL.Path,
		})
	})

	r.GET("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, []string{"user1"})
	})
	srv := &http.Server{Addr: ":8088", Handler: r}
	runServers(map[string]*http.Server{"gin": srv})
}

// Echo часть
func example5_echo() {
	e := echo.New()

	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok {
			if he.Code == http.StatusNotFound {
				c.JSON(http.StatusNotFound, map[string]string{
					"error": "not found",
					"path":  c.Path(),
				})
				return
			}
			if he.Code == http.StatusMethodNotAllowed {
				c.JSON(http.StatusMethodNotAllowed, map[string]string{
					"error":  "method not allowed",
					"method": c.Request().Method,
					"path":   c.Path(),
				})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}

	e.GET("/api/users", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []string{"user1"})
	})
	srv := &http.Server{Addr: ":8089", Handler: e}
	runServers(map[string]*http.Server{"echo": srv})
}

//ПРИМЕР 6: КЭШИРОВАНИЕ ОТВЕТОВ (IN-MEMORY)
/*
ЗАЧЕМ: Кэширование часто запрашиваемых данных снижает нагрузку на БД.

ФИШКА: Создаём middleware, который сохраняет успешные GET-ответы в памяти с TTL.
       При повторном запросе отдаём кэш, добавляя заголовок X-Cache: HIT.
*/

// Общий кэш для gin и echo
type CacheItem struct {
	Data      []byte
	Status    int
	Headers   map[string]string
	CreatedAt time.Time
	TTL       time.Duration
}

type Cache struct {
	mu    sync.RWMutex
	store map[string]*CacheItem
}

func NewCache() *Cache {
	return &Cache{store: make(map[string]*CacheItem)}
}

func (c *Cache) Get(key string) (*CacheItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.store[key]
	if !ok {
		return nil, false
	}
	if time.Since(item.CreatedAt) > item.TTL {
		return nil, false
	}
	return item, true
}

func (c *Cache) Set(key string, item *CacheItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = item
}

// Gin middleware
func cacheGin(cache *Cache, ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}
		key := c.Request.Method + ":" + c.Request.URL.Path + ":" + c.Request.URL.RawQuery
		if item, ok := cache.Get(key); ok {
			for k, v := range item.Headers {
				c.Writer.Header().Set(k, v)
			}
			c.Writer.Header().Set("X-Cache", "HIT")
			c.Writer.WriteHeader(item.Status)
			c.Writer.Write(item.Data)
			c.Abort()
			return
		}
		// Обёртка для захвата ответа
		w := &cacheResponseWriter{ResponseWriter: c.Writer}
		c.Next()
		if w.statusCode >= 200 && w.statusCode < 300 {
			cache.Set(key, &CacheItem{
				Data: w.body, Status: w.statusCode, Headers: w.headers,
				CreatedAt: time.Now(), TTL: ttl,
			})
		}
	}
}

// Echo middleware
func cacheEcho(cache *Cache, ttl time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Method != http.MethodGet {
				return next(c)
			}
			key := c.Request().Method + ":" + c.Request().URL.Path + ":" + c.Request().URL.RawQuery
			if item, ok := cache.Get(key); ok {
				for k, v := range item.Headers {
					c.Response().Header().Set(k, v)
				}
				c.Response().Header().Set("X-Cache", "HIT")
				c.Response().WriteHeader(item.Status)
				c.Response().Write(item.Data)
				return nil
			}
			w := &cacheResponseWriter{ResponseWriter: c.Response().Writer}
			c.Response().Writer = w
			err := next(c)
			if w.statusCode >= 200 && w.statusCode < 300 {
				cache.Set(key, &CacheItem{
					Data: w.body, Status: w.statusCode, Headers: w.headers,
					CreatedAt: time.Now(), TTL: ttl,
				})
			}
			return err
		}
	}
}

type cacheResponseWriter struct {
	http.ResponseWriter
	body       []byte
	statusCode int
	headers    map[string]string
}

func (w *cacheResponseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}
func (w *cacheResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
func (w *cacheResponseWriter) Header() http.Header {
	headers := w.ResponseWriter.Header()
	w.headers = make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			w.headers[k] = v[0]
		}
	}
	return headers
}

// Gin пример
func primer6() {
	cache := NewCache()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(cacheGin(cache, 60*time.Second))

	r.GET("/api/expensive", func(c *gin.Context) {
		time.Sleep(500 * time.Millisecond)
		c.JSON(http.StatusOK, gin.H{
			"data":      "expensive result",
			"timestamp": time.Now().Unix(),
		})
	})
	srv := &http.Server{Addr: ":8090", Handler: r}
	runServers(map[string]*http.Server{"gin": srv})
}

// Echo пример
func example6_echo() {
	cache := NewCache()
	e := echo.New()
	e.Use(cacheEcho(cache, 60*time.Second))

	e.GET("/api/expensive", func(c echo.Context) error {
		time.Sleep(500 * time.Millisecond)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"data":      "expensive result",
			"timestamp": time.Now().Unix(),
		})
	})
	srv := &http.Server{Addr: ":8091", Handler: e}
	runServers(map[string]*http.Server{"echo": srv})
}

//ПРИМЕР 7: STREAMING (ПОТОКОВАЯ ОТДАЧА ДАННЫХ)

/*
ЗАЧЕМ: Для больших объёмов данных экономит память и улучшает UX.

ФИШКА: В gin используем c.Stream, в echo — c.Response().Flush() и c.Stream.
*/
func primer7g() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.GET("/api/stream", func(c *gin.Context) {
		c.Writer.Header().Set("Counter-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("connection", "keep-alive")
		c.Stream(func(w io.Writer) bool {
			for i := 1; i <= 5; i++ {
				fmt.Fprintf(w, "data: { \"count\": %d }\n\n", i)
				c.Writer.Flush()
				time.Sleep(1 * time.Second)
			}
			fmt.Fprint(w, "data: { \"status\": \"done\" }\n\n")
			c.Writer.Flush()
			return false
		})
	})
	srv := &http.Server{Addr: ":8092", Handler: r}
	runServers(map[string]*http.Server{"gin": srv})
}

// Echo часть
func primer7e() {
	e := echo.New()

	e.GET("/api/stream", func(c echo.Context) error {
		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Connection", "keep-alive")
		for i := 1; i <= 5; i++ {
			_, err := fmt.Fprintf(c.Response(), "data: { \"count\": %d }\n\n", i)
			if err != nil {
				return err
			}
			c.Response().Flush()
			time.Sleep(1 * time.Second)
		}
		_, err := fmt.Fprint(c.Response(), "data: { \"status\": \"done\" }\n\n")
		if err != nil {
			return err
		}
		c.Response().Flush()
		return nil
	})
	srv := &http.Server{Addr: ":8093", Handler: e}
	runServers(map[string]*http.Server{"echo": srv})
}

//ПРИМЕР 8: GRACEFUL SHUTDOWN + КОНФИГУРАЦИЯ

/*
ЗАЧЕМ: В продакшене сервер должен корректно завершаться, дожидаясь текущих запросов.
       Конфигурация через переменные окружения упрощает деплой.

ФИШКА: Используем signal.Notify и http.Server.Shutdown с таймаутом.
       Конфиг загружаем из env с дефолтами.
*/

func primere8g() {
	port := getEnv("PORT_GIN", "8094")
	readTimeout := getEnvDuration("READ_TIMEOUT", 5*time.Second)
	writeTimeout := getEnvDuration("WRITE_TIMEOUT", 10*time.Second)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
	runServers(map[string]*http.Server{"gin": srv})
}

func primer8e() {
	port := getEnv("PORT_ECHO", "8095")
	readTimeout := getEnvDuration("READ_TIMEOUT", 5*time.Second)
	writeTimeout := getEnvDuration("WRITE_TIMEOUT", 10*time.Second)

	e := echo.New()
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      e,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
	runServers(map[string]*http.Server{"echo": srv})
}

// Вспомогательные функции для env
func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return fallback
}
func main() {
	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7e()
	primer7g()
	primer8e()
	primere8g()
}
