package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

/*
Базовый REST API (Обработка JSON)
Проверить, умеешь ли ты работать с encoding/json и
понимаешь ли, что http.ResponseWriter — это интерфейс.
*/
/*
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var user User
		if err := json.UnmarshalRead(r.Body, &user); err != nil {
			http.Error(w, http.StatusText(400), http.StatusBadRequest)
			return
		}

		user.ID = 37

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.MarshalWrite(w, user); err != nil {
			log.Println("Ошибка отправки JSON:", err)
		}
	})
	log.Println("Сервер запущен на :8080")
	http.ListenAndServe(":8080", mux)
}
*/
/*
Полноценный CRUD сервис (In-Memory)
Умение работать с REST, парсить JSON, отдавать
правильные статус-коды и защищать мапу мьютексом.
*/
/*
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Store struct {
	mu    sync.RWMutex
	users map[string]User
}

func main() {
	store := &Store{users: make(map[string]User)}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		store.mu.RLock()
		user, exists := store.users[id]
		store.mu.RUnlock()

		if !exists {
			http.Error(w, http.StatusText(404), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.MarshalWrite(w, user); err != nil {
			http.Error(w, http.StatusText(500), http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var user User

		if err := json.UnmarshalRead(r.Body, &user); err != nil {
			http.Error(w, http.StatusText(400), http.StatusBadRequest)
			return
		}

		store.mu.Lock()
		store.users[user.ID] = user
		store.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	})

	log.Println("Server on :8080")
	http.ListenAndServe(":8080", mux)
}
*/
/*
Production HTTP Server (Timeouts + Graceful Shutdown)
Знание того, как запускать сервер в продакшене, чтобы он не падал при деплое и не зависал от Slowloris-атак.
*/
/*
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("Hello Men"))
	})
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Println("Starting server on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Graceful shutdown failed: %v", err)
	}

	log.Println("Server stopped gracefully")
}
*/

/*
Middleware (Logging + Recovery + Request ID)
Понимание цепочки оберток http.Handler и работы с контекстом.
*/
/*
type ctxKey string

const requestIDKey ctxKey = "reqID"

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		reqID := fmt.Sprintf("req-%d", time.Now().UnixNano())
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)

		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		logger.Info("Request handled",
			slog.String("id", reqID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

func RecoveryMiddleware(next http.Handler) http.Handler {
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

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("OXOXOX")
	})

	handler := RecoveryMiddleware(LoggerMiddleware(mux))
	http.ListenAndServe(":8080", handler)
}
*/

/*
Clean Architecture + Dependency Injection
Умеешь ли ты структурировать код. Никаких глобальных переменных. Только слои: Хендлер -> Сервис.
*/
/*
// 1. Слой Сервиса (Бизнес-логика)
type UserService struct {
	log *slog.Logger
}

func (s *UserService) GetUser(id string) (string, error) {
	s.log.Info("Fetching user from DB", slog.String("id", id))
	return "User: " + id, nil
}

// 2. Слой Хендлера (HTTP)
type HTTPHandler struct {
	svc *UserService
	log *slog.Logger
}

func (h *HTTPHandler) ErrorJSON(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.MarshalWrite(w, map[string]string{"code": code, "msg": msg}); err != nil {
		http.Error(w, http.StatusText(500), http.StatusInternalServerError)
		return
	}
}

func (h *HTTPHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	name, err := h.svc.GetUser(id)
	if err != nil {
		h.ErrorJSON(w, http.StatusInternalServerError, "DB_ERROR", "Internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.MarshalWrite(w, map[string]string{"name": name}); err != nil {
		http.Error(w, http.StatusText(500), http.StatusInternalServerError)
		return
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	svc := &UserService{logger}
	h := &HTTPHandler{svc, logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /user/{id}", h.GetUser)

	logger.Info("Starting HTTP server")
	http.ListenAndServe(":8080", mux)
}
*/

/*
Graceful Shutdown с фоновыми воркерами
Как выключить сервер, не потеряв данные, которые прямо сейчас
обрабатывают фоновые горутины (например, отправка метрик или запись в БД).
*/
type App struct {
	log *slog.Logger
	wg  sync.WaitGroup
}

func (a *App) backgroundWorker(ctx context.Context) {
	defer a.wg.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.log.Info("Worker stopping, flushing data...")
			// Имитация flush'а данных
			time.Sleep(1 * time.Second)
			return
		case <-ticker.C:
			a.log.Info("Worker tick: sending metrics")
		}
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	app := &App{log: logger}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app.wg.Add(1)
	go app.backgroundWorker(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Kyky Epta"))
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server failed", slog.Any("err", err))
		}
	}()

	<-ctx.Done()
	logger.Info("Shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
	logger.Info("HTTP server stopped")

	app.wg.Wait()
	logger.Info("Background workers stopped. App exited.")
}
