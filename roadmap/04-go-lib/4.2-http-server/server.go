package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

/*
1. HTTP.HANDLER И HTTP.HANDLERFUNC

http.Handler — это интерфейс, который лежит в основе всего HTTP-сервера в Go:

    type Handler interface {
        ServeHTTP(ResponseWriter, *Request)
    }

Любая структура, реализующая метод ServeHTTP, уже является Handler'ом.

http.HandlerFunc — это тип-адаптер. Он позволяет использовать ОБЫЧНУЮ функцию
как Handler. Именно благодаря ему мы можем писать:

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { ... })

Реализация HandlerFunc гениально проста:

    type HandlerFunc func(ResponseWriter, *Request)
    func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) { f(w, r) }

То есть HandlerFunc — это функция, которая реализует интерфейс Handler.

ResponseWriter — интерфейс для формирования ответа. Главные методы:
    - WriteHeader(statusCode int) — отправляет HTTP-статус (200, 404, 500...)
      Важно: вызывать ДО Write, иначе статус будет 200.
    - Write([]byte) — пишет тело ответа. Неявно вызывает WriteHeader(200) если не вызван.
    - Header() — возвращает заголовки для установки.

Request — структура, содержащая все данные запроса:
    - Method — HTTP-метод (GET, POST, PUT, DELETE...)
    - URL — адрес запроса (содержит Path, Query, RawQuery)
    - Header — заголовки запроса (тип http.Header)
    - Body — тело запроса (io.ReadCloser, нужно закрывать!)
    - Context() — контекст запроса (для отмены и передачи данных)
    - PathValue(name) — получение переменной пути (Go 1.22+)
    - Query — параметры строки запроса (GET /api?name=value)
*/

/*
2. HTTP.SERVEMUX И РОУТИНГ (GO 1.22+)

ServeMux — мультиплексор (роутер), который направляет запросы к нужным Handler'ам.

ДО Go 1.22 (СТАРЫЙ СТИЛЬ):
    - mux.HandleFunc("/users/", handler)
    - НЕ проверял HTTP-метод (приходилось внутри handler проверять r.Method)
    - НЕ поддерживал переменные в пути (/users/123 приходилось парсить вручную)
    - Приоритет: более длинный путь побеждает (/users/123 важнее /users/)

ПОСЛЕ Go 1.22 (НОВЫЙ СТИЛЬ):
    - Поддержка МЕТОДОВ: "GET /users", "POST /users", "DELETE /users/{id}"
    - ПЕРЕМЕННЫЕ ПУТИ: "/users/{id}" — значение через r.PathValue("id")
    - WILDCARD: "/files/{path...}" — захватывает всю оставшуюся часть пути
    - ПРИОРИТЕТ: точный путь важнее шаблонного (/users/me важнее /users/{id})
    - Игнорирование косой черты в конце: /users и /users/ обрабатываются одинаково

СИНТАКСИС ПАТТЕРНОВ:
    - Простой путь: "/users"
    - Метод + путь: "GET /users"
    - Переменная: "GET /users/{id}"
    - Wildcard: "GET /files/{path...}"
    - Без метода: "/static/" (обрабатывает все методы)

ПОЛУЧЕНИЕ ЗНАЧЕНИЙ:
    - r.PathValue("id") — извлечение переменной пути
*/

/*
3. MIDDLEWARE (ПРОСЛОЙКИ)

Middleware — это функция-декоратор, которая оборачивает Handler и добавляет
сквозную логику. Типичные задачи middleware:
    - Логирование запросов
    - Аутентификация/Авторизация
    - Восстановление после паники (recovery)
    - CORS (кросс-доменные запросы)
    - Сжатие (gzip)
    - Добавление RequestID
    - Тайминг и метрики

СТАНДАРТНАЯ СИГНАТУРА MIDDLEWARE:
    func exampleMiddleware(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // ДО вызова следующего обработчика
            next.ServeHTTP(w, r)
            // ПОСЛЕ вызова следующего обработчика
        })
    }

ВАЖНО: всегда вызывайте next.ServeHTTP(w, r), иначе цепочка оборвётся.

ЦЕПОЧКА MIDDLEWARE (композиция):
    handler = loggingMiddleware(metricsMiddleware(authMiddleware(apiHandler)))
    // Выполняется: logging -> metrics -> auth -> apiHandler -> auth -> metrics -> logging

УДОБНАЯ ФУНКЦИЯ ДЛЯ ЦЕПОЧКИ:
    func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
        for _, mw := range middlewares {
            h = mw(h)
        }
        return h
    }
*/

/*
4. GRACEFUL SHUTDOWN (КОРРЕКТНАЯ ОСТАНОВКА)

Проблема: без graceful shutdown при Ctrl+C сервер мгновенно завершается,
обрывая все активные запросы клиентов.

Решение: http.Server.Shutdown(ctx) — не принимает новые соединения,
но даёт существующим запросам завершиться.

АЛГОРИТМ:
    1. Запускаем сервер в горутине
    2. Создаём канал для сигналов (syscall.SIGINT, syscall.SIGTERM)
    3. Ждём сигнала в main горутине
    4. При сигнале вызываем server.Shutdown(ctx) с таймаутом
    5. Ждём завершения всех обработчиков

ВАЖНО: server.ListenAndServe() возвращает http.ErrServerClosed при нормальном shutdown.
Это не ошибка, проверяйте: err != http.ErrServerClosed

ТАЙМАУТЫ СЕРВЕРА (защита от медленных клиентов):
    - ReadTimeout     — максимум на чтение всего запроса (включая тело)
    - ReadHeaderTimeout — максимум на чтение заголовков (обычно меньше ReadTimeout)
    - WriteTimeout    — максимум на запись ответа
    - IdleTimeout     — максимум на keep-alive соединение
    - MaxHeaderBytes  — максимум на размер заголовков

ПОЧЕМУ ТАЙМАУТЫ ВАЖНЫ? Без них злоумышленник может открыть соединение и
ничего не отправлять, заняв ресурсы сервера навсегда.
*/

/*
5. HTTP/2 И HTTP/3

HTTP/2:
    - Включён АВТОМАТИЧЕСКИ при использовании HTTPS (TLS)
    - Мультиплексирование: несколько запросов по одному соединению
    - Сжатие заголовков (HPACK)
    - Server Push (может отправлять ресурсы до запроса)
    - Не требует дополнительного кода

HTTP/3:
    - Использует QUIC вместо TCP (быстрее, особенно при потере пакетов)
    - НЕ встроен в стандартную библиотеку
    - Требуется сторонняя библиотека (quic-go)
    - Встроен не будет, т.к. зависит от UDP и имеет другую модель
*/

/*
6. ТИПИЧНЫЕ ОШИБКИ И ИХ РЕШЕНИЕ

1. ГОНКА ДАННЫХ (DATA RACE) В ХЕНДЛЕРАХ
    Проблема: несколько горутин одновременно читают/пишут общую переменную
    Решение: atomic, sync.Mutex, или каналы

2. УТЕЧКА ГОРУТИН
    Проблема: забыли закрыть r.Body или не читали его
    Решение: даже если не читаете тело, вызовите r.Body.Close()
        или используйте io.Copy(io.Discard, r.Body)

3. PANIC В ХЕНДЛЕРЕ
    Проблема: одна паника — весь сервер упал
    Решение: recovery middleware с recover()

4. СЕРВЕР НЕ ОСТАНАВЛИВАЕТСЯ
    Проблема: забыли graceful shutdown
    Решение: signal.Notify + server.Shutdown

5. МЕДЛЕННЫЕ КЛИЕНТЫ ЗАНИМАЮТ РЕСУРСЫ
    Проблема: клиент читает ответ медленно, соединение висит
    Решение: WriteTimeout и ReadTimeout

6. БОЛЬШИЕ ЗАГОЛОВКИ
    Проблема: атака через огромные заголовки
    Решение: MaxHeaderBytes

7. УТЕЧКА ПАМЯТИ ПРИ ЧТЕНИИ ТЕЛА
    Проблема: чтение всего тела может исчерпать память
    Решение: использовать http.MaxBytesReader для ограничения

8. ЗАБЫТЫЙ RESP.BODY.CLOSE() В КЛИЕНТЕ
    Проблема: утечка соединений из пула
    Решение: всегда defer resp.Body.Close()
*/

// 1. HTTP.HANDLER И HTTP.HANDLERFUNC

// customHandler — структура, реализующая интерфейс http.Handler
type customHandler struct {
	version string
}

func (h *customHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Server-Version", h.version)
	fmt.Fprintf(w, "Custom Handler response\n")
}

// обычная функция, которую мы превратим в Handler через HandlerFunc
func helloHandler(w http.ResponseWriter, r *http.Request) {
	// Работа с запросом
	log.Printf("Method: %s, Path: %s", r.Method, r.URL.Path)

	// Параметры строки запроса (query)
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "Word"
	}

	// Заголовки ответа
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Powered-By", "Go")

	// Тело ответа (json)
	response := map[string]string{"message": fmt.Sprintf("Hello, %s!", name)}
	json.NewEncoder(w).Encode(response)
}

// Пример работы с телом запроса
func echoHandler(w http.ResponseWriter, r *http.Request) {
	// Читаем тело запроса
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// Закрывать r.Body не обязательно — http.Server сделает это сам,
	// но если вы не читали всё тело, лучше закрыть
	// defer r.Body.Close() // http.Server уже вызывает Close()

	// Отправляем обратно
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func primer1() {
	mux := http.NewServeMux()

	// 1. Использование HandlerFunc (функция-адаптер) — самый частый способ
	mux.HandleFunc("/hello", helloHandler)

	// 2. Использование собственной структуры, реализующей Handler
	mux.Handle("/custom", &customHandler{version: "v1.0"})

	// 3. Анонимная функция как HandlerFunc
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 4. Echo endpoint (читает и возвращает JSON)
	mux.HandleFunc("/echo", echoHandler)

	log.Println("Primer1: http.Handler and HandlerFunc on :8081")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		log.Fatal(err)
	}
}

// 2. SERVEMUX И РОУТИНГ GO 1.22+

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var users = map[string]User{
	"1": {ID: "1", Name: "Kolya"},
	"2": {ID: "2", Name: "Petr"},
}

// listUsers — GET /users
func listUsers(w http.ResponseWriter, r *http.Request) {
	userList := make([]User, len(users))
	for _, u := range userList {
		userList = append(userList, u)
	}
	json.NewEncoder(w).Encode(userList)
}

// getUserByID — GET /users/{id}
func getUserByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, ok := users[id]
	if !ok {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(user)
}

// createUser — POST /users
func createUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid Request Body", http.StatusBadRequest)
		return
	}
	users[user.ID] = user
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// updateUser — PUT /users/{id}
func updateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if _, ok := users[id]; !ok {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	user.ID = id
	users[id] = user
	json.NewEncoder(w).Encode(user)
}

// deleteUser — DELETE /users/{id}
func deleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := users[id]; !ok {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	delete(users, id)
	w.WriteHeader(http.StatusNoContent)
}

// wildcardHandler — GET /files/{path...}
func wildcardHandler(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path") // ← захватывает весь оставшийся путь
	fmt.Fprintf(w, "File path: %s\n", path)
	// Пример: http.ServeFile(w, r, "./static/"+path)
}

// exactPathPriority — демонстрация приоритета: точный путь важнее шаблонного
// GET /users/me — обрабатывается этим хендлером, а не /users/{id}
func currentUserHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Current user profile\n")
}

func primer2() {
	mux := http.NewServeMux()

	// CRUD для пользователей с использованием нового синтаксиса (Go 1.22+)
	mux.HandleFunc("GET /users", listUsers)
	mux.HandleFunc("POST /users", createUser)
	mux.HandleFunc("GET /users/{id}", getUserByID)
	mux.HandleFunc("PUT /users/{id}", updateUser)
	mux.HandleFunc("DELETE /users/{id}", deleteUser)

	// Точный путь важнее шаблонного: /users/me НЕ попадает в /users/{id}
	mux.HandleFunc("GET /users/me", currentUserHandler)

	// Wildcard — все подпути
	mux.HandleFunc("GET /files/{path...}", wildcardHandler)

	// Без указания метода — обрабатывает все методы
	mux.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static"+r.URL.Path)
	})

	log.Println("Primer2: Go 1.22+ routing on :8082")
	log.Println("Test endpoints:")
	log.Println("  GET  /users")
	log.Println("  GET  /users/1")
	log.Println("  POST /users -d '{\"id\":\"3\",\"name\":\"Charlie\"}'")
	log.Println("  PUT  /users/3 -d '{\"name\":\"Charles\"}'")
	log.Println("  DELETE /users/3")
	log.Println("  GET  /users/me")
	log.Println("  GET  /files/foo/bar/baz.txt")

	if err := http.ListenAndServe(":8082", mux); err != nil {
		log.Fatal(err)
	}
}

// 3. MIDDLEWARE (ПРОСЛОЙКИ)

// loggingResponseWriter — обёртка для захвата HTTP-статуса
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *loggingResponseWriter) writeHandler(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// 1. LOGGING MIDDLEWARE — логирует каждый запрос
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %v", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

// 2. RECOVERY MIDDLEWARE — ловит панику, чтобы сервер не упал
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC: %v\n%s", err, debug.Stack())
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// 3. AUTH MIDDLEWARE — проверяет токен
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, "Missing authorization header", http.StatusUnauthorized)
			return
		}
		// Ожидаем формат "Bearer secret-token"
		parts := strings.Split(token, " ")
		if len(parts) != 2 || parts[0] != "Bearer" || parts[1] != "secret-token" {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// 4. CORS MIDDLEWARE — разрешает кросс-доменные запросы
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// 5. GZIP MIDDLEWARE — сжимает ответ, если клиент поддерживает
type gzipResponseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (grw *gzipResponseWriter) Write(b []byte) (int, error) {
	return grw.Writer.Write(b)
}

func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		grw := &gzipResponseWriter{ResponseWriter: w, Writer: gz}
		next.ServeHTTP(grw, r)
	})
}

// 6. REQUEST ID MIDDLEWARE — добавляет уникальный ID каждому запросу
var requestID atomic.Uint64

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestID.Add(1)
		w.Header().Set("X-Request-ID", time.Now().Format("20060102150405")+"-"+itoa(id))
		next.ServeHTTP(w, r)
	})
}

func itoa(i uint64) string {
	return strings.TrimSpace(strings.Replace(
		strings.Trim(fmt.Sprint(i), "[]"),
		" ", "", -1))
}

// chain — вспомогательная функция для применения цепочки middleware
func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for _, mw := range middlewares {
		h = mw(h)
	}
	return h
}

func primer3() {
	// Финальный обработчик
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("API response"))
	})

	// Паникующий обработчик (для проверки recovery)
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})

	// Применяем middleware к разным ручкам
	apiWithMiddleware := chain(apiHandler,
		requestIDMiddleware,
		loggingMiddleware,
		recoveryMiddleware,
		authMiddleware,
	)

	publicHandler := chain(apiHandler,
		loggingMiddleware,
		recoveryMiddleware,
		gzipMiddleware,
	)

	mux := http.NewServeMux()
	mux.Handle("/api", apiWithMiddleware)
	mux.Handle("/public", publicHandler)
	mux.Handle("/panic", chain(panicHandler, recoveryMiddleware, loggingMiddleware))

	log.Println("Primer3: Middleware examples on :8083")
	log.Println("  GET /api           — requires auth header")
	log.Println("  GET /public        — no auth, gzip enabled")
	log.Println("  GET /panic         — test panic recovery")

	log.Println("\nTest commands:")
	log.Println("  curl http://localhost:8083/public")
	log.Println("  curl -H 'Authorization: Bearer secret-token' http://localhost:8083/api")
	log.Println("  curl http://localhost:8083/panic")

	http.ListenAndServe(":8083", mux)
}

// 4. GRACEFUL SHUTDOWN

func primer4() {
	mux := http.NewServeMux()

	// Долгий обработчик (имитация долгой операции)
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Slow request started")
		time.Sleep(5 * time.Second)
		fmt.Fprintln(w, "Slow request completed")
		log.Println("Slow request finished")
	})

	// Быстрый обработчик
	mux.HandleFunc("/fast", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Fast response")
	})

	// Healthcheck для проверки
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:         ":8084",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Println("Primer4: Graceful shutdown server on :8084")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Ожидаем сигнал прерывания
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server gracefully...")

	// Даём серверу 10 секунд на завершение текущих запросов
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited")
}

// Mini API
type task struct {
	id   int    `json:"id"`
	name string `json:"name"`
	done bool   `json:"done"`
}

// Хранилище задач (in-memory)
var (
	tasks  = make(map[int]task)
	nextID = 1
	taskMu sync.RWMutex
)

func primerAPI() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /tasks", getTasks)
	mux.HandleFunc("POST /tasks", createTask)
	mux.HandleFunc("DELETE /tasks/{id}", deleteTask)

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func getTasks(w http.ResponseWriter, r *http.Request) {
	defer taskMu.RUnlock()
	taskMu.RLock()

	// Превращаем map в слайс
	taskList := make([]task, len(tasks))
	for _, t := range tasks {
		taskList = append(taskList, t)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(taskList)
}

// POST /tasks — создать задачу
func createTask(w http.ResponseWriter, r *http.Request) {
	var task task

	// Читаем JSON из тела запроса
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Ivalid JSON", http.StatusBadRequest)
		return
	}

	// Валидация
	if task.name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	defer taskMu.RUnlock()
	taskMu.RLock()

	task.id = nextID
	nextID++
	tasks[task.id] = task

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// DELETE /tasks/{id} — удалить задачу
func deleteTask(w http.ResponseWriter, r *http.Request) {
	// Получаем ID из URL (Go 1.22+)
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	defer taskMu.RUnlock()
	taskMu.RLock()

	if _, exist := tasks[id]; !exist {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	delete(tasks, id)
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	//primer1()
	//primer2()
	//primer3()
	//primer4()
}
