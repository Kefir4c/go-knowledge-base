package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

/*
Пример 1: Базовый Graceful Shutdown (Только HTTP)
В этом примере мы ловим сигнал ОС и просим http.Server завершить работу.
*/
/*
func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		fmt.Fprintln(w, "Hello from Server!")
	})

	srv := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	<-stop
	log.Println("Получен сигнал завершения, начинаем Graceful Shutdown...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Сервер принудительно закрыт: %v", err)
	}
	log.Println("Сервер успешно остановлен")
}
*/

/*
Пример 2: Graceful Shutdown с БД и WaitGroup
В реальном сервисе у тебя есть не только HTTP, но и фоновые воркеры,
пул соединений с БД и другие горутины. Все их нужно завершить.
*/
/*
func main() {
	db, _ := sql.Open("postgres", "postgres://user:pass@localhost/db")
	defer db.Close()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Println("Воркер: завершаюсь")
				return
			case <-time.After(1 * time.Second):
				log.Println("Воркер: выполняю фоновую задачу")
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var count int
		db.QueryRow("SELECT 1").Scan(&count)
		fmt.Fprintln(w, "OK")
	})
	srv := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	<-stop
	log.Println("Получен сигнал, начинаем Graceful Shutdown...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)

	cancel()
	wg.Wait()
	log.Println("Все горутины завершены")
	log.Println("БД закрыта. Программа выходит.")
}
*/

/*
Пример 3: Graceful Shutdown через signal.NotifyContext (Современный Go)
Начиная с Go 1.16 появилась функция signal.NotifyContext,
которая делает весь предыдущий код в 2 раза короче.
*/
/*
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{Addr: ":8080"}
	go func() {
		log.Println("Сервер запущен")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка: %v", err)
		}
	}()

	// 3. Ждем отмену контекста (сигнал)
	<-ctx.Done()
	log.Println("Получен сигнал, завершаем работу...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Ошибка при Shutdown: %v", err)
	}

	var wg sync.WaitGroup
	wg.Wait()

	fmt.Println("Сервер успешно остановлен")
}
*/
/*
type DB struct {
	conn *sql.DB
}

func (d *DB) Close() error {
	log.Println("БД: Закрываем пул соединений...")
	return d.conn.Close()
}

func main() {
	log.Println("Запуск сервиса...")

	dbConn, _ := sql.Open("postgres", "postgres://user:pass@localhost/db")
	db := &DB{conn: dbConn}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	jobs := make(chan int, 100)

	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					log.Printf("Воркер %d: получил сигнал, завершаюсь...\n", workerID)
					return
				case job := <-jobs:
					// Имитация работы
					time.Sleep(1 * time.Second)
					log.Printf("Воркер %d: обработал задачу %d\n", workerID, job)
				}
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; ; i++ {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- i:
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(3 * time.Second):
			fmt.Fprintln(w, "Успешный ответ")
		case <-r.Context().Done(): // Если клиент отключился во время запроса
			log.Println("Клиент отменил запрос")
		}
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx // Передаем контекст в каждый запрос
		},
	}

	go func() {
		log.Println("HTTP сервер запущен на :8080")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Сигнал завершения")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Println("1. Останавливаем HTTP-сервер...")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Сервер не успел завершиться корректно: %v\n", err)
	} else {
		log.Println("HTTP-сервер успешно остановлен")
	}

	log.Println("2. Ждем завершения фоновых воркеров...")
	wg.Wait()
	log.Println("Все фоновые воркеры остановлены")

	log.Println("3. Закрываем подключение к БД...")
	if err := db.Close(); err != nil {
		log.Printf("Ошибка при закрытии БД: %v\n", err)
	}

	log.Println("Сервис остановился")
}
*/

/*
Production-реализация
Полноценный канонический шаблон graceful shutdown с раздельными
контекстами, логированием и очисткой внешних ресурсов:
*/
/*
func main() {
	// 1. Инициализация сервера с адекватными таймаутами
	srv := &http.Server{
		Addr:              ":8080",
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		Handler:           setupRouter(),
	}

	// 2. Запуск в отдельной горутине
	go func() {
		log.Println("Starting HTTP server on :8080")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server fatal error: %v", err)
		}
	}()

	// 3. Ожидание сигнала завершения ОС
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	log.Println("Shutdown signal received. Starting graceful shutdown...")

	// 4. Контекст с жестким таймаутом на весь процесс остановки
	// Важно: таймаут должен быть меньше terminationGracePeriodSeconds в K8s
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// 5. Остановка HTTP-сервера
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("HTTP server shutdown error: %v", err)
	}

	// 6. Очистка внешних зависимостей
	if err := cleanupDependencies(shutdownCtx); err != nil {
		log.Printf("Dependencies cleanup failed: %v", err)
	}

	log.Println("Server exited gracefully")
}

func cleanupDependencies(ctx context.Context) error {
	// Закрытие пула БД, отправка финальных метрик, flush логов
	// db.Close(), redis.Close(), otel.Shutdown(ctx)
	return nil
}

func setupRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte("OK"))
	})
	return mux
}
*/

/*
Имитация gRPC-сервера на TCP (Graceful Shutdown + Стриминг)
Вот полноценный сервер, который принимает TCP-соединения
(делает gRPC под капотом) и умеет отдавать поток данных (Server Streaming).
*/

type GRPCServer struct {
	listener net.Listener
	wg       sync.WaitGroup
}

func (s *GRPCServer) Start() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		s.wg.Add(1)
		go func(c net.Conn) {
			defer c.Close()
			defer s.wg.Done()
			s.handleConnection(c)
		}(conn)
	}
}

func (s *GRPCServer) handleConnection(conn net.Conn) {
	reader := bufio.NewReader(conn)

	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		log.Printf("Получен запрос: %s", msg)

		for i := 1; i <= 3; i++ {
			response := fmt.Sprintf("Стрим-ответ %d на запрос: %s", i, msg)
			conn.Write([]byte(response))
			time.Sleep(1 * time.Second)
		}
	}
}

func (s *GRPCServer) Shutdown(ctx context.Context) {
	s.listener.Close()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Сервер: все соединения завершены")
	case <-ctx.Done():
		log.Println("Сервер: таймаут Graceful Shutdown, принудительное закрытие")
	}
}

func main() {
	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatal(err)
	}
	srv := GRPCServer{listener: listener}
	go srv.Start()
	log.Println("gRPC-сервер запущен на :9090")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Println("Получен сигнал завершения...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv.Shutdown(shutdownCtx)
	log.Println("Сервер успешно завершен")
}
