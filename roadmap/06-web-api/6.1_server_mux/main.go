package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

/*
СОВРЕМЕННЫЙ HTTP.SERVEMUX В GO 1.22+ - ПОЛНЫЙ РАЗБОР ДЛЯ СОБЕСЕДОВАНИЙ
   ПОЛНАЯ ТЕОРЕТИЧЕСКАЯ БАЗА

  1. СИНТАКСИС ПАТТЕРНОВ
     Паттерн = [METHOD] [PATH]
     - METHOD — опционально, если опущен — соответствует любому методу.
     - PATH — может содержать статические сегменты, именованные параметры
       {id} и один catch‑all параметр {path...} в конце.
     Примеры:
         "GET /"                  → только GET на корень
         "/"                      → любой метод на корень (старое поведение)
         "POST /users"            → POST на /users
         "GET /users/{id}"        → GET с параметром id (один сегмент)
         "PUT /users/{id}/profile"→ PUT с id и статическим /profile
         "DELETE /files/{path...}"→ DELETE с захватом всего остатка пути

  2. ИЗВЛЕЧЕНИЕ ПАРАМЕТРОВ
     В обработчике используйте r.PathValue(name) — возвращает строку.
     Если параметр отсутствует, вернёт пустую строку.
     Для чисел используйте strconv.Atoi.

  3. ПРИОРИТЕТЫ ПАТТЕРНОВ (КЛЮЧЕВОЙ МОМЕНТ)
     Правила разрешения конфликтов (по убыванию приоритета):
       а) Точное совпадение (без параметров) — самый высокий приоритет.
          Например: "/api/users" перекроет "/api/{entity}".
       б) Совпадение с параметрами, но более длинный путь имеет приоритет.
          Например: "/users/{id}/profile" перекроет "/users/{id}".
       в) Если два паттерна одинаково специфичны (оба с параметрами и
          одинаковой длиной), побеждает первый зарегистрированный.
       г) Паттерн с указанным методом имеет приоритет над паттерном без метода
          (для одного и того же пути).
     Эти правила жёстко зафиксированы в реализации и часто спрашивают на собесах.

  4. ПАРАМЕТР {path...} (CATCH‑ALL)
     - Захватывает все оставшиеся сегменты пути, включая слеши.
     - Может использоваться только в конце паттерна.
     - Имеет более низкий приоритет, чем конкретные паттерны.
     - Например: "/files/{path...}" перекроется "/files/public/{file}"
       для пути "/files/public/photo.jpg".

  5. ОБРАБОТКА НЕПОДДЕРЖИВАЕМЫХ МЕТОДОВ
     Если паттерн содержит метод, а приходит запрос с другим методом,
     ServeMux автоматически возвращает 405 Method Not Allowed.
     Это НОВОЕ поведение — раньше нужно было писать вручную.

  6. ВЛОЖЕННЫЕ МАРШРУТИЗАТОРЫ (ГРУППИРОВКА)
     Можно создавать несколько экземпляров ServeMux и объединять их через
     http.StripPrefix или http.Handle. Это удобно для модульной организации.
     Пример:
         api := http.NewServeMux()
         api.HandleFunc("GET /users", ...)
         root.Handle("/api/", http.StripPrefix("/api", api))

  7. MIDDLEWARE (ПРОМЕЖУТОЧНЫЕ ОБРАБОТЧИКИ)
     Middleware — это функция, принимающая http.Handler и возвращающая
     http.Handler. Оборачивать можно как весь роутер, так и отдельные хендлеры.
     Важно: порядок обёртывания определяет порядок выполнения.

  8. GRACEFUL SHUTDOWN (КОРРЕКТНОЕ ЗАВЕРШЕНИЕ)
     Используйте http.Server.Shutdown(ctx) — он дожидается завершения всех
     текущих запросов. Всегда применяйте в продакшене.

  9. КОНТЕКСТ И ТАЙМАУТЫ
     Каждый запрос имеет r.Context(). Его можно использовать для:
       - передачи данных между обработчиками,
       - отмены операций при отключении клиента,
       - установки таймаутов через context.WithTimeout.
     Серверные таймауты: ReadTimeout, WriteTimeout, IdleTimeout.

 10. ОБРАБОТКА ОШИБОК
     Используйте http.Error(w, message, code) для отправки ошибок.
     Никогда не выводите стек трейс клиенту — только логируйте на сервере.

 11. СРАВНЕНИЕ СО СТОРОННИМИ РОУТЕРАМИ
     Новый ServeMux покрывает 80% задач. Он легче, быстрее и не требует
     внешних зависимостей. Для сложной валидации, middleware-цепочек,
     поддержки WebSocket или работы с большим количеством паттернов
     всё ещё могут пригодиться chi, gin или echo, но в большинстве
     проектов теперь достаточно стандартной библиотеки.

 12. ПРОИЗВОДИТЕЛЬНОСТЬ
     Новый ServeMux использует эффективное дерево маршрутизации (radix tree),
     поэтому сопоставление происходит за O(log n). Для большинства приложений
     этого достаточно с запасом.

 13. ПОВЕДЕНИЕ С ЗАКРЫВАЮЩИМ СЛЕШЕМ (TRAILING SLASH)
     Паттерн "/users" соответствует как "/users", так и "/users/".
     Паттерн "/users/" соответствует только "/users/" (со слешем).
     Будьте внимательны — это может влиять на маршрутизацию.

 14. АВТОМАТИЧЕСКАЯ ОБРАБОТКА HEAD
     Для GET‑паттернов ServeMux автоматически обрабатывает HEAD‑запросы,
     вызывая тот же обработчик, но без тела ответа (если обработчик не
     переопределяет метод вручную). Это стандартное поведение.

 15. ПАНИКИ И ВОССТАНОВЛЕНИЕ
     Если обработчик паникует, сервер по умолчанию закрывает соединение.
     Для централизованного перехвата паник используйте middleware с recover.

 16. ВАЛИДАЦИЯ ПАРАМЕТРОВ
     Параметры пути всегда строковые, их нужно валидировать самостоятельно
     (например, проверять на числовой формат, длину и т.д.).

 17. ОГРАНИЧЕНИЯ НОВОГО SERVEMUX
     - Нет поддержки регулярных выражений в паттернах (и не нужно, т.к.
       параметры покрывают основные сценарии).
     - Нет встроенной поддержки query-параметров (они доступны через
       r.URL.Query(), как и раньше).
     - Нет автоматической привязки JSON к структурам (это оставлено на
       усмотрение разработчика).
     - Поддерживается только один catch‑all параметр, и только в конце.

 18. СОЧЕТАНИЕ С OLD‑STYLE ПАТТЕРНАМИ
     Старые паттерны без метода продолжают работать. Вы можете спокойно
     мигрировать постепенно, добавляя методы к уже существующим.

 19. ЛОГИРОВАНИЕ ЗАПРОСОВ
     Рекомендуется логировать метод, путь, статус ответа и время обработки.
     Это легко сделать через middleware.

 20. ТЕСТИРОВАНИЕ
     Новый ServeMux отлично тестируется через httptest.NewRequest и
     httptest.NewRecorder. Для проверки параметров пути используйте
     r.PathValue.
*/

//ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ

func runServer(port int, handler http.Handler, name string) {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}
	fmt.Printf("\n--- %s ---\n", name)
	fmt.Printf("Запущен на http://localhost:%d\n", port)
	fmt.Println("Нажмите Ctrl+C для остановки")
	fmt.Println("")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

//  ПРИМЕР 1: УНИВЕРСАЛЬНЫЙ ОБРАБОТЧИК (ЛЮБОЙ МЕТОД)
/*
ЗАЧЕМ: Показывает, как использовать паттерн без метода для создания
       универсального обработчика, который сам разбирает метод.
       Полезно для логирования, метрик, fallback-обработчиков.

ФИШКА: Паттерн "/api" без метода обрабатывает ЛЮБОЙ метод.
       Идеально для точек входа, где нужно логировать все запросы.
*/
func primer1() {
	mux := http.NewServeMux()

	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Поймали запрос: %s %s", r.Method, r.URL.Path)
		fmt.Fprintf(w, "Метод: %s, Путь: %s", r.Method, r.URL.Path)
	})

	mux.HandleFunc("GET /api/users", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Специальный: GET /api/users")
	})

	runServer(8001, mux, "Пример 1: Универсальный обработчик")
}

//  ПРИМЕР 2: ДИНАМИЧЕСКОЕ ВКЛЮЧЕНИЕ/ОТКЛЮЧЕНИЕ РОУТОВ
/*
ЗАЧЕМ: Показывает, как можно динамически включать/отключать эндпоинты
       без перезапуска сервера. Например, для feature-флагов или
       временного отключения проблемных ручек.

ФИШКА: Используем замыкание и переменную-флаг в обработчике.
       Параметр {action} позволяет управлять состоянием через API.
*/
func primer2() {
	mux := http.NewServeMux()

	featureEnabled := true

	mux.HandleFunc("POST /admin/feature/{action}", func(w http.ResponseWriter, r *http.Request) {
		action := r.PathValue("action")
		switch action {
		case "enable":
			featureEnabled = true
			fmt.Println(w, "Фича включена")
		case "disable":
			featureEnabled = false
			fmt.Println(w, "Фича отключена")
		default:
			http.Error(w, "Неизвестное действие", http.StatusBadRequest)
		}
	})

	mux.HandleFunc("GET /api/new-feature", func(w http.ResponseWriter, r *http.Request) {
		if !featureEnabled {
			http.Error(w, "Фича временно отключена", http.StatusServiceUnavailable)
			return
		}
		fmt.Println(w, "Менчик, фича работает")
	})
	runServer(8002, mux, "Пример 2: Динамическое отключение роутов")
}

//  ПРИМЕР 3: ВЛОЖЕННЫЕ ПАРАМЕТРЫ
/*
ЗАЧЕМ: Показывает, как работать с параметрами на разных уровнях вложенности.
       Реальный кейс: /api/v1/users/123/orders/456

ФИШКА: Два параметра в одном паттерне — userId и orderId.
       Можно извлекать оба через r.PathValue.
*/
func primer3() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/users/{usersId}/orders/{ordersId}", func(w http.ResponseWriter, r *http.Request) {
		userId := r.FormValue("userId")
		orderId := r.FormValue("ordersId")
		uID, _ := strconv.Atoi(userId)
		oID, _ := strconv.Atoi(orderId)
		fmt.Sprintf("Пользователь: %d, Заказ: %d", uID, oID)
	})

	mux.HandleFunc("DELETE /api/users/{userId}/orders/{orderId}", func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		orderID := r.PathValue("orderId")
		fmt.Sprintf("Удалён заказ %s пользователя %s", orderID, userID)
	})
	runServer(8003, mux, "Пример 3: Вложенные параметры")
}

//  ПРИМЕР 4: ОБОГАЩЕНИЕ КОНТЕКСТА ЧЕРЕЗ MIDDLEWARE
/*
ЗАЧЕМ: Показывает, как middleware может обогащать контекст запроса,
       передавая данные дальше по цепочке. Реальный кейс: аутентификация,
       извлечение пользователя из JWT, логирование с request ID.

ФИШКА: Контекст передаётся через r.Context(), а хендлеры получают
       обогащённые данные через кастомные функции-геттеры.
*/
type contextKey string

const (
	userIDKey    contextKey = "userID"
	requestIDKey contextKey = "requestID"
)

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		if idStr != "" {
			ctx := context.WithValue(r.Context(), userIDKey, idStr)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func getUserID(ctx context.Context) string {
	if v := ctx.Value(userIDKey); v != nil {
		return v.(string)
	}
	return "unknown"
}

func getRequestID(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		return v.(string)
	}
	return "no-request-id"
}

func primer4() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := getUserID(ctx)
		reqID := getUserID(ctx)
		fmt.Fprintf(w, "UserID: %s\nRequestID: %s", userID, reqID)
	})
	handler := requestIDMiddleware(userIDMiddleware(mux))
	runServer(8004, handler, "Пример 4: Обогащение контекста")
}

//  ПРИМЕР 5: ГРУППИРОВКА МАРШРУТОВ С ПРЕФИКСАМИ
/*
ЗАЧЕМ: Показывает мощную технику структурирования больших API.
       Каждый модуль (users, products, admin) имеет свой роутер,
       а корневой роутер их монтирует.

ФИШКА: Используем http.StripPrefix для автоматической обрезки префикса.
       Теперь внутри модуля пути пишутся без префикса.
*/
func primer5() {
	root := http.NewServeMux()

	usersMux := http.NewServeMux()
	usersMux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Список пользователей")
	})
	usersMux.HandleFunc("GET /{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Пользователь: %s", r.PathValue("id"))
	})
	usersMux.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Создан пользователь")
	})
	root.Handle("/api/users/", http.StripPrefix("/api/users", usersMux))

	productsMux := http.NewServeMux()
	productsMux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Список продуктов")
	})
	productsMux.HandleFunc("GET /{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Продукт: %s", r.PathValue("id"))
	})
	root.Handle("/api/products/", http.StripPrefix("/api/products", productsMux))

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Статистика: 1000 пользователей, 500 заказов")
	})
	root.Handle("/admin/", http.StripPrefix("/admin", adminMux))

	root.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Главная страница\nДоступно:\n/api/users\n/api/products\n/admin")
	})
	runServer(8005, root, "Пример 5: Группировка маршрутов")
}

//  ПРИМЕР 6: PATH + QUERY ПАРАМЕТРЫ
/*
ЗАЧЕМ: Показывает комбинирование path-параметров и query-параметров.
       Реальный кейс: /api/orders?status=pending&limit=10

ФИШКА: Path-параметры через r.PathValue, query-параметры через r.URL.Query()
       Работают идеально вместе.
*/
func primer6() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		orderID := r.FormValue("id")
		status := r.URL.Query().Get("status")
		limit := r.URL.Query().Get("limit")
		fmt.Fprintf(w, "Заказ: %s\n", orderID)
		if status == "" {
			fmt.Fprintf(w, "   Статус: %s\n", status)
		}
		if limit != "" {
			fmt.Fprintf(w, "   Лимит: %s\n", limit)
		}
	})

	mux.HandleFunc("GET /api/orders", func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		limit := r.URL.Query().Get("limit")
		fmt.Fprintf(w, "Список заказов\n")
		if status != "" {
			fmt.Fprintf(w, "   Фильтр по статусу: %s\n", status)
		}
		if limit != "" {
			fmt.Fprintf(w, "   Лимит: %s\n", limit)
		}
	})
	runServer(8006, mux, "Пример 6: Path + Query параметры")
}

//  ПРИМЕР 7: JSON С ВАЛИДАЦИЕЙ
/*
ЗАЧЕМ: Показывает, как элегантно обрабатывать JSON с валидацией.
       Используем отдельную структуру для запроса и проверяем поля.

ФИШКА: Валидация вынесена в отдельную функцию, хендлеры становятся чистыми.
       Ошибки валидации возвращаются в понятном формате.
*/
type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

func (c *CreateUserRequest) Validate() map[string]string {
	errors := make(map[string]string)
	if c.Name == "" {
		errors["name"] = "обязательное поле"
	}
	if c.Email == "" || !strings.Contains(c.Email, "@") {
		errors["email"] = "должен быть корректный email"
	}
	if c.Age < 18 || c.Age > 100 {
		errors["age"] = "должен быть от 18 до 100"
	}
	return errors
}

func primer7() {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		var req CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Неверный JSON", http.StatusBadRequest)
			return
		}
		if errors := req.Validate(); len(errors) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  "Ошибка валидации",
				"fields": errors,
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Пользователь создан",
			"user":    req,
		})
	})
	runServer(8007, mux, "Пример 7: JSON с валидацией")
}

//  ПРИМЕР 8: ДИНАМИЧЕСКАЯ ЗАДЕРЖКА
/*
ЗАЧЕМ: Показывает, как эмулировать разную скорость ответа для разных эндпоинтов.
       Полезно для тестирования timeout-ов, retry-логики.

ФИШКА: Параметр {delay} задаёт задержку в секундах.
       Показывает, как можно влиять на поведение через path-параметры.
*/
func primer8() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/delay/{seconds}", func(w http.ResponseWriter, r *http.Request) {
		secSter := r.PathValue("seconds")
		sec, err := strconv.Atoi(secSter)
		if err != nil || sec < 0 || sec > 30 {
			http.Error(w, "Задержка должна быть от 0 до 30 секунд", http.StatusBadRequest)
			return
		}
		if sec > 0 {
			time.Sleep(time.Duration(sec) * time.Second)
		}
		fmt.Fprintf(w, "Ответ через %d секунд", sec)
	})
	runServer(8008, mux, "Пример 8: Динамическая задержка")
}

//  ПРИМЕР 9: A/B ТЕСТИРОВАНИЕ ЧЕРЕЗ ЗАГОЛОВКИ
/*
ЗАЧЕМ: Показывает, как реализовать A/B тестирование на уровне роутинга.
       Разные пользователи видят разные версии API.

ФИШКА: Middleware проверяет заголовок X-Experiment и направляет
       на разные хендлеры. Всё прозрачно для клиента.
*/
func primer9() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/feature", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ВЕРСИЯ A: Старый интерфейс")
	})

	mux.HandleFunc("GET /api/v2/feature", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ВЕРСИЯ B: Новый интерфейс с улучшениями")
	})

	abMux := http.NewServeMux()
	abMux.HandleFunc("GET /feature", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Experiment") == "B" {
			http.Redirect(w, r, "/api/v2/feature", http.StatusTemporaryRedirect)
		} else {
			http.Redirect(w, r, "/api/v1/feature", http.StatusTemporaryRedirect)
		}
	})
	mux.Handle("/api/", http.StripPrefix("/api", abMux))

	runServer(8009, mux, "Пример 9: A/B Тестирование")
}

//  ПРИМЕР 10: ФАБРИКА ОБРАБОТЧИКОВ (ЗАМЫКАНИЕ)
/*
ЗАЧЕМ: Показывает, как можно генерировать обработчики на лету.
       Полезно для создания фабрик обработчиков.

ФИШКА: Функция makeHandler возвращает разные обработчики в зависимости
       от переданного параметра. Паттерн один, поведение разное.
*/
func primer10() {
	mux := http.NewServeMux()

	makeHandler := func(resource string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			fmt.Fprintf(w, "%s (ID: %s)", resource, id)
		}
	}

	mux.HandleFunc("GET /users/{id}", makeHandler("Пользователь"))
	mux.HandleFunc("GET /orders/{id}", makeHandler("Заказ"))
	mux.HandleFunc("GET /products/{id}", makeHandler("Продукт"))

	mux.HandleFunc("GET /api/{version}/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		version := r.PathValue("version")
		id := r.PathValue("id")
		fmt.Fprintf(w, "API v%s: Пользователь %s", version, id)
	})

	runServer(8010, mux, "Пример 10: Фабрика обработчиков")
}
func main() {
	fmt.Println("HTTP.SERVEMUX — 10 ПРИМЕРОВ")
	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
	primer9()
	primer10()
}
