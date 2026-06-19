package flagosgraceshut

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

/*
1. FLAG — ПАРСИНГ АРГУМЕНТОВ (ПОЛНАЯ ТЕОРИЯ)

flag — стандартный пакет для парсинга аргументов командной строки.

ОСНОВНЫЕ ФУНКЦИИ:
  flag.String(name, default, usage) *string
  flag.Int(name, default, usage) *int
  flag.Bool(name, default, usage) *bool
  flag.Duration(name, default, usage) *time.Duration

  flag.StringVar(&var, name, default, usage) // в переменную
  flag.IntVar(&var, name, default, usage)
  flag.BoolVar(&var, name, default, usage)
  flag.DurationVar(&var, name, default, usage)

  flag.Parse()   // парсит os.Args[1:]
  flag.Args()    // возвращает позиционные аргументы
  flag.NArg()    // количество позиционных аргументов
  flag.NFlag()   // количество переданных флагов
  flag.VisitAll(func(*Flag)) // перебор всех флагов
  flag.PrintDefaults() // вывод помощи

СТАНДАРТНАЯ ПОМОЩЬ:
  -h, --help   // автоматически выводят usage

ПРИМЕРЫ ЗАПУСКА:
  go run main.go -port 8080
  go run main.go -port 8080 -env prod arg1 arg2
  go run main.go -h

ЧАСТЫЕ ОШИБКИ:
  1. Забыли flag.Parse() — все флаги остаются default
  2. Неправильный тип: -port abc → ошибка
  3. flag.Int("port", 8080, "port") — нужно разыменовывать *port
  4. Использование flag после flag.Parse()

2. OS/SIGNAL — ПЕРЕХВАТ СИГНАЛОВ (ПОЛНАЯ ТЕОРИЯ)

os/signal — пакет для обработки системных сигналов.

ОСНОВНЫЕ СИГНАЛЫ UNIX:
  syscall.SIGINT  // Ctrl+C (прерывание)
  syscall.SIGTERM // завершение (от kill, docker stop)
  syscall.SIGHUP  // перезагрузка терминала
  syscall.SIGQUIT // Ctrl+\ (дамп стека)
  syscall.SIGUSR1 // пользовательский сигнал 1
  syscall.SIGUSR2 // пользовательский сигнал 2
  syscall.SIGKILL // безусловное завершение (нельзя перехватить)

СИГНАЛЫ В WINDOWS:
  syscall.SIGINT  // Ctrl+C
  syscall.SIGTERM // завершение

ФУНКЦИИ ПАКЕТА:
  signal.Notify(c chan<- os.Signal, sig ...os.Signal)
    • Перехватывает указанные сигналы
    • Если сигналы не указаны — перехватывает все

  signal.Stop(c chan<- os.Signal)
    • Останавливает перехват сигналов
    • Возвращает канал в исходное состояние

  signal.Ignore(sig ...os.Signal)
    • Игнорирует указанные сигналы

  signal.Reset(sig ...os.Signal)
    • Восстанавливает обработку сигналов по умолчанию

  signal.NotifyContext(ctx context.Context, sig ...os.Signal) (context.Context, context.CancelFunc)
    • Создаёт контекст, который отменяется при получении сигнала
    • Go 1.16+

ВАЖНО:
  • Канал для сигналов должен быть буферизированным
  • Сигналы не ставятся в очередь, если канал занят
  • SIGKILL и SIGSTOP нельзя перехватить

ПАТТЕРНЫ:
  1. Один канал для всех сигналов
  2. Отдельный канал для каждого сигнала
  3. signal.NotifyContext (рекомендуемый)

3. SIGNAL.NOTIFYCONTEXT (ПОДРОБНО)

signal.NotifyContext — самый удобный способ работы с сигналами.

КАК РАБОТАЕТ:
  ctx, stop := signal.NotifyContext(ctx, sig ...os.Signal)

  • Создаёт дочерний контекст
  • При получении сигнала вызывает cancel()
  • ctx.Done() закрывается
  • stop() отключает перехват сигналов

ПРИМЕР:
  ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
  defer stop()

  select {
  case <-ctx.Done():
      fmt.Println("Signal received")
  }

ПРЕИМУЩЕСТВА:
  • Не нужен отдельный канал для сигналов
  • Автоматическая отмена всех горутин через ctx.Done()
  • Удобно для graceful shutdown
  • ctx.Err() возвращает context.Canceled

4. GRACEFUL SHUTDOWN — ПОЛНАЯ ТЕОРИЯ

http.Server.Shutdown(ctx) — корректно останавливает сервер.

ЧТО ДЕЛАЕТ SHUTDOWN:
  1. Перестаёт принимать новые соединения
  2. Ждёт завершения активных запросов
  3. Закрывает idle-соединения
  4. Возвращает управление, когда всё завершено

ЧТО НЕ ДЕЛАЕТ SHUTDOWN:
  • Не закрывает активные соединения принудительно
  • Не завершает горутины, не связанные с сервером

ТАЙМАУТ SHUTDOWN:
  ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
  defer cancel()

  server.Shutdown(ctx)

  Если таймаут истекает → возвращает ошибку (контекст отменён)

БЕЗ SHUTDOWN (ПЛОХО):
  При Ctrl+C сервер убивается мгновенно:
    • Активные запросы обрываются
    • Клиенты получают ошибки
    • Данные могут быть потеряны

С SHUTDOWN (ХОРОШО):
  При Ctrl+C сервер:
    • Не принимает новые запросы
    • Ждёт завершения активных
    • Завершается плавно

ПОЛНЫЙ ПАТТЕРН:
  server := &http.Server{Addr: ":8080", Handler: mux}

  go func() {
      if err := server.ListenAndServe(); err != http.ErrServerClosed {
          log.Fatal(err)
      }
  }()

  ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
  defer stop()

  <-ctx.Done()

  shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
  defer cancel()

  server.Shutdown(shutdownCtx)

ОБРАБОТКА ОШИБОК:
  err := server.Shutdown(ctx)
  if err == context.DeadlineExceeded {
      log.Println("Shutdown timeout")
  } else if err != nil {
      log.Printf("Shutdown error: %v", err)
  }

5. ДРУГИЕ МЕХАНИЗМЫ ОСТАНОВКИ

TIME.SLEEP (НЕ РЕКОМЕНДУЕТСЯ):
  // ❌ ПЛОХО: не реагирует на сигналы
  time.Sleep(10 * time.Second)

SELECT С КАНАЛОМ:
  // ✅ РЕКОМЕНДУЕТСЯ: реагирует на сигналы
  select {
  case <-time.After(10 * time.Second):
      fmt.Println("timeout")
  case <-ctx.Done():
      fmt.Println("cancelled")
  }

TIMER С ОСТАНОВКОЙ:
  // ✅ РЕКОМЕНДУЕТСЯ: можно остановить
  timer := time.NewTimer(10 * time.Second)
  defer timer.Stop()

  select {
  case <-timer.C:
      fmt.Println("timeout")
  case <-ctx.Done():
      fmt.Println("cancelled")
  }

TICKER С ОСТАНОВКОЙ:
  // ✅ ВСЕГДА СТОПИТЬ!
  ticker := time.NewTicker(1 * time.Second)
  defer ticker.Stop()

6. ОШИБКИ И РЕШЕНИЯ

1. СЕРВЕР НЕ ОСТАНАВЛИВАЕТСЯ:
   • Проверь, что server.Shutdown вызывается
   • Проверь, что сервер запущен в горутине
   • Проверь, что есть активные запросы

2. SHUTDOWN ТАЙМАУТ:
   • Увеличь таймаут shutdown
   • Убедись, что хендлеры не зависают

3. СИГНАЛЫ НЕ ПЕРЕХВАТЫВАЮТСЯ:
   • Проверь, что канал буферизированный
   • Проверь, что signal.Notify вызван
   • SIGKILL нельзя перехватить

4. ПАНЕЛЬ ВХОДА:
   • Добавь recovery middleware
   • Используй defer в горутинах

5. ГОРУТИНЫ НЕ ОСТАНАВЛИВАЮТСЯ:
   • Передавай ctx.Done() в горутины
   • Используй select для проверки
*/

// 1: FLAG — ПАРСИНГ АРГУМЕНТОВ
func primer1() {
	port := flag.Int("port", 8080, "server port")
	timeout := flag.Duration("timeout", 30*time.Second, "shutdown timeout")
	debug := flag.Bool("debug", false, "enable debug mode")
	name := flag.String("name", "server", "server name")
	env := flag.String("env", "dev", "environment (dev/prod)")
	// Флаг в переменную
	var logFile string
	flag.StringVar(&logFile, "log", "", "log file path")

	//Парсим
	flag.Parse()

	// Позиционные аргументы
	args := flag.Args()

	// Вывод
	fmt.Printf("port: %d\n", *port)
	fmt.Printf("timeout: %v\n", *timeout)
	fmt.Printf("debug: %v\n", *debug)
	fmt.Printf("name: %s\n", *name)
	fmt.Printf("env: %s\n", *env)
	fmt.Printf("log: %s\n", logFile)
	fmt.Printf("args: %v\n", args)

	// Список всех флагов
	fmt.Println("\nAll flags:")
	flag.VisitAll(func(f *flag.Flag) {
		fmt.Printf("  -%s: %s (default: %s)\n", f.Name, f.Usage, f.DefValue)
	})
}

// 2: БАЗОВЫЙ ПЕРЕХВАТ СИГНАЛОВ
func primer2() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigs

	switch sig {
	case syscall.SIGINT:
		fmt.Println("SIGINT (Ctrl +C)")
		os.Exit(0)
	case syscall.SIGTERM:
		fmt.Println("SIGTERM (termination)")
		os.Exit(0)
	default:
		fmt.Printf("Получен неизвестный сигнал: %v\n", sig)
	}
}

// 3: SIGNAL.NOTIFYCONTEXT
func primer3() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Println("Press Ctrl+C to stop...")
	<-ctx.Done()

	fmt.Println("Signal received!")
	fmt.Printf("Error: %v\n", ctx.Err()) // context.Canceled
}

// 4: ГРАСЕФУЛ ШАТДАУН СЕРВЕРА
func primer4() {
	// 1. Сервер с одним хендлером
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Имитация долгой работы (2 секунды)
		time.Sleep(1 * time.Second)
		w.Write([]byte("ok"))
	})

	// 2. Быстрый хендлер для проверки
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("healthy"))
	})

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// 3. Запуск сервера в горутине
	go func() {
		log.Println("Server starting on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 4. Ожидание сигнала
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	// 5. Shutdown с таймаутом 10 секунд
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// 5: МНОЖЕСТВЕННЫЕ СИГНАЛЫ
func primer5() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-sigs
		fmt.Printf("Received signal: %v\n", sig)

		switch sig {
		case syscall.SIGINT:
			fmt.Println("Got SIGINT, exiting...")
			return
		case syscall.SIGTERM:
			fmt.Println("Got SIGTERM, exiting...")
			return
		case syscall.SIGHUP:
			fmt.Println("Got SIGHUP, reloading config...")
			// Перезагрузка конфига
		}
	}
}

// 6: ИГНОРИРОВАНИЕ СИГНАЛОВ
func primer6() {
	// Игнорируем SIGINT (Ctrl+C не сработает)
	signal.Ignore(syscall.SIGINT)

	// Перехватываем SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)

	fmt.Println("SIGINT is ignored, press Ctrl+C...")
	fmt.Println("To stop, send SIGTERM (kill PID)")

	<-sigs
	fmt.Println("SIGTERM received, exiting")
}

// 7: ОСТАНОВКА ГОРУТИН ЧЕРЕЗ КОНТЕКСТ
func primer7() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer stop()

	// Горутина 1: периодическая задача
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fmt.Println("Task 1: working...")
			case <-ctx.Done():
				fmt.Println("Task 1: stopping")
				return
			}
		}
	}()

	// Горутина 2: бесконечный цикл с проверкой
	go func() {
		for {
			select {
			case <-ctx.Done():
				fmt.Println("Task 2: stopping")
				return
			default:
				// Проверяем отмену в цикле
				fmt.Println("Task 2: processing...")
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

	fmt.Println("Press Ctrl+C to stop...")
	<-ctx.Done()

	// Даём горутинам время завершиться
	time.Sleep(100 * time.Millisecond)
	fmt.Println("All goroutines stopped")
}

// 8: ПРОДАКШЕН-ПРИМЕР
func primer8() {
	// 1. Флаги
	port := flag.Int("port", 8080, "server port")
	timeout := flag.Duration("timeout", 30*time.Second, "shutdown timeout")
	readTO := flag.Duration("read-timeout", 5*time.Second, "read timeout")
	writeTO := flag.Duration("write-timeout", 10*time.Second, "write timeout")
	env := flag.String("env", "dev", "enviroment")
	flag.Parse()

	// 2. Роутер (Go 1.22+)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /heath", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /api/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"` + id + `","name":"User"}`))
	})

	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		// Имитация создания
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"123"}`))
	})

	// 3. Сервер
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      mux,
		ReadTimeout:  *readTO,
		WriteTimeout: *writeTO,
		IdleTimeout:  60 * time.Second,
	}

	// 4. Запуск
	go func() {
		log.Printf("Server starting on port %d (env=%s)", *port, *env)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 5. Graceful shutdown

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

func main() {
	fmt.Println("primer1")
	primer1()
	fmt.Println("primer2")
	primer2()
	fmt.Println("primer3")
	primer3()
	fmt.Println("primer4")
	primer4()
	fmt.Println("primer5")
	primer5()
	fmt.Println("primer6")
	primer6()
	fmt.Println("primer7")
	primer7()
	fmt.Println("primer8")
	primer8()
}
