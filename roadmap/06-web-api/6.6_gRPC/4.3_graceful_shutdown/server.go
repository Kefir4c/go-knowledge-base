package __3_graceful_shutdown

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/status"

	pb "github.com/"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

/*
РАЗДЕЛ 4.3: GRACEFUL SHUTDOWN
Graceful Shutdown (корректное завершение) — это механизм, который позволяет
серверу остановиться без потери данных и без разрыва активных соединений.
В микросервисной архитектуре это КРИТИЧЕСКИ ВАЖНЫЙ паттерн, без которого
невозможны rolling updates и zero-downtime deployments.

В этом разделе мы разберём:
  1. Что такое Graceful Shutdown и зачем он нужен
  2. Почему Stop() — это плохо для продакшена
  3. Как работает GracefulStop() (внутреннее устройство)
  4. Обработка сигналов SIGINT и SIGTERM
  5. Полная реализация с таймаутом
  6. Особенности для стриминговых RPC
  7. Интеграция с health checks
  8. Best practices и типичные ошибки
  9. Ключевые выводы для собеседования

1. ЧТО ТАКОЕ GRACEFUL SHUTDOWN И ЗАЧЕМ ОН НУЖЕН
Graceful Shutdown (корректное завершение) — это процесс остановки сервера,
при котором он:

  1. Прекращает принимать НОВЫЕ соединения и запросы.
  2. Даёт возможность ЗАВЕРШИТЬСЯ уже активным RPC-вызовам.
  3. Освобождает все ресурсы (соединения, файловые дескрипторы).
  4. Только после этого завершает работу.

Зачем это нужно:

В распределённых системах сервисы постоянно перезапускаются:
  • Rolling updates (постепенное обновление версий).
  • Масштабирование (увеличение/уменьшение количества реплик).
  • Сбои и восстановление после ошибок.
  • Плановое обслуживание.

Если сервер завершится "жёстко" (без graceful shutdown), то:
  • Клиенты получат ошибки (connection refused, broken pipe).
  • Активные запросы будут прерваны на середине.
  • Могут возникнуть проблемы с целостностью данных.
  • Пользователи увидят ошибки 5xx.

В Kubernetes при завершении Pod'а сначала отправляется SIGTERM,
и только через terminationGracePeriodSeconds (по умолчанию 30 секунд)
— SIGKILL. Если сервер не успеет завершиться за это время,
Kubernetes принудительно убьёт процесс.


2. ПОЧЕМУ STOP() — ЭТО ПЛОХО ДЛЯ ПРОДАКШЕНА
В gRPC-Go есть два метода для остановки сервера:
  • Stop() — жёсткая остановка.
  • GracefulStop() — корректная остановка.

2.1. Server.Stop()
Stop немедленно закрывает все открытые соединения и слушатели.
Он отменяет все активные RPC на стороне сервера, и соответствующие
ожидающие RPC на стороне клиента получат ошибки соединения.

  s.Stop() //НЕЛЬЗЯ в продакшене

Что происходит:
  • Все активные RPC немедленно прерываются.
  • Клиенты получают ошибки (например, "connection closed").
  • Данные могут быть потеряны.
  • Нет возможности завершить обработку.

2.2. Server.GracefulStop()
GracefulStop останавливает сервер корректно:
  • Прекращает принимать новые соединения и RPC.
  • Блокируется до тех пор, пока все ожидающие RPC не завершатся.
  • Только после этого закрывает слушатели и освобождает ресурсы.

  s.GracefulStop() //ПРАВИЛЬНО для продакшена

Ключевое отличие: GracefulStop ДОЖИДАЕТСЯ завершения активных RPC,
а Stop — НЕ ДОЖИДАЕТСЯ и обрывает всё сразу.

3. КАК РАБОТАЕТ GRACEFULSTOP() (ВНУТРЕННЕЕ УСТРОЙСТВО)
Алгоритм работы GracefulStop() в grpc-go (упрощённо):

  1. Устанавливается флаг s.drain = true — сервер начинает отклонять
     новые соединения и RPC.
  2. Закрываются все слушатели (listeners) — новые соединения
     больше не принимаются.
  3. Ожидается, пока все активные соединения (s.conns) завершатся.
  4. Если включена опция waitForHandlers, ожидается завершение
     всех обработчиков (handlersWG.Wait()).
  5. Вызывается cleanup() — освобождение всех ресурсов.

Важно: GracefulStop() блокируется на всё время ожидания активных RPC.
Если у вас есть долгоживущие стримы, они могут заблокировать остановку
на неопределённое время.

  // Упрощённая схема работы GracefulStop
  func (s *Server) GracefulStop() {
      s.mu.Lock()
      s.serve = false
      s.drain = true
      s.mu.Unlock()

      // Закрываем все слушатели
      for lis := range s.lis {
          lis.Close()
      }

      // Ждём завершения всех соединений
      s.mu.Lock()
      for len(s.conns) > 0 {
          s.cv.Wait()
      }
      s.mu.Unlock()

      // Ждём завершения всех обработчиков
      if s.opts.waitForHandlers {
          s.handlersWG.Wait()
      }

      s.cleanup()
  }

4. ОБРАБОТКА СИГНАЛОВ SIGINT И SIGTERM
В Go для graceful shutdown нужно перехватывать системные сигналы,
которые отправляются процессу при завершении.

4.1. Какие сигналы нужно обрабатывать
  • SIGINT (Signal Interrupt) — отправляется при нажатии Ctrl+C.
  • SIGTERM (Signal Terminate) — отправляется Kubernetes, systemd
    и другими оркестраторами при завершении процесса.

  Важно: SIGKILL (kill -9) НЕЛЬЗЯ перехватить. Это "последнее слово"
  операционной системы.

4.2. Базовая реализация
  quit := make(chan os.Signal, 1)
  signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
  <-quit

  log.Println("Получен сигнал завершения, начинаем graceful shutdown...")
  s.GracefulStop()

4.3. Важный нюанс: буферизация канала
  Канал для сигналов должен быть буферизированным (размер 1).
  Это гарантирует, что сигнал не будет потерян, если он придёт
  до того, как мы начнём его читать.

  quit := make(chan os.Signal, 1) // ✅ ПРАВИЛЬНО
  signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

4.4. Запуск в отдельной горутине
  go func() {
      <-quit
      log.Println("Получен сигнал завершения")
      s.GracefulStop()
  }()

  Это позволяет основному потоку продолжать обработку запросов
  до самого момента получения сигнала.

5. ПОЛНАЯ РЕАЛИЗАЦИЯ С ТАЙМАУТОМ
GracefulStop() блокируется на неопределённое время. Если какой-то RPC
завис, сервер никогда не завершится. Поэтому нужен ТАЙМАУТ с fallback
на жёсткую остановку.

5.1. Реализация с таймаутом
  func main() {
      // ... создание сервера и запуск ...

      quit := make(chan os.Signal, 1)
      signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
      <-quit

      log.Println("Получен сигнал завершения")

      // Канал для сигнала о завершении graceful shutdown
      done := make(chan struct{})
      go func() {
          s.GracefulStop()
          close(done)
      }()

      // Таймаут на graceful shutdown (рекомендуется 30 секунд)
      ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
      defer cancel()

      select {
      case <-done:
          log.Println("Graceful shutdown завершён")
      case <-ctx.Done():
          log.Println("Таймаут graceful shutdown, принудительная остановка")
          s.Stop()
      }
  }

5.2. Почему 30 секунд
  • В Kubernetes terminationGracePeriodSeconds по умолчанию = 30 секунд.
  • Нужно успеть завершиться до того, как K8s отправит SIGKILL.
  • Рекомендуется устанавливать таймаут меньше, чем terminationGracePeriodSeconds
    (например, 25-28 секунд), чтобы осталось время на завершение.


6. ОСОБЕННОСТИ ДЛЯ СТРИМИНГОВЫХ RPC
Долгоживущие стримы (server-streaming, client-streaming, bidirectional)
могут заблокировать GracefulStop() на неопределённое время.

6.1. Проблема
  • Клиент может держать стрим открытым бесконечно.
  • GracefulStop() будет ждать, пока стрим не закроется.
  • Сервер никогда не завершится.

6.2. Решение
  В обработчике стрима нужно слушать контекст стрима и завершаться
  при его отмене:

  func (s *server) StreamData(req *pb.Request, stream pb.Service_StreamDataServer) error {
      for {
          select {
          case <-stream.Context().Done():
              // Клиент отключился или сервер завершается
              return stream.Context().Err()
          case data := <-s.dataCh:
              if err := stream.Send(data); err != nil {
                  return err
              }
          }
      }
  }

  Когда вызывается GracefulStop(), контекст стрима отменяется,
  и обработчик завершается, позволяя завершиться и самому стриму.

7. ИНТЕГРАЦИЯ С HEALTH CHECKS
Перед началом graceful shutdown важно сообщить балансировщику нагрузки,
что сервер больше не должен принимать трафик.

7.1. Стандартный health check в gRPC
  import (
      "google.golang.org/grpc/health"
      healthpb "google.golang.org/grpc/health/grpc_health_v1"
  )

  healthServer := health.NewServer()
  healthpb.RegisterHealthServer(s, healthServer)

  // В начале работы — статус SERVING
  healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

7.2. Изменение статуса при shutdown
  // Перед вызовом GracefulStop()
  healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

  // Даём время балансировщику обновить состояние (5-10 секунд)
  time.Sleep(5 * time.Second)

  // Теперь можно вызывать GracefulStop()
  s.GracefulStop()

7.3. Зачем это нужно
  • Балансировщик (например, Kubernetes Service, Envoy) перестаёт
    направлять трафик на этот экземпляр.
  • Новые запросы не будут приходить на уже завершающийся сервер.
  • Время ожидания даёт балансировщику время обновить таблицу маршрутизации.

8. BEST PRACTICES И ТИПИЧНЫЕ ОШИБКИ

8.1. Best Practices
  + Всегда используй GracefulStop() в продакшене, никогда — Stop().
  + Всегда устанавливай таймаут на graceful shutdown с fallback на Stop().
  + Перехватывай SIGINT и SIGTERM.
  + Используй буферизированный канал для сигналов (размер 1).
  + Для стриминговых методов проверяй stream.Context().Done().
  + Перед shutdown переводи health-статус в NOT_SERVING.
  + В Kubernetes настраивай terminationGracePeriodSeconds (например, 60 секунд).
  + Логируй каждый этап shutdown для отладки.

8.2. Типичные ошибки
  - Использование Stop() вместо GracefulStop().
    Решение: всегда использовать GracefulStop().
  - GracefulStop() без таймаута.
    Решение: всегда использовать таймаут с fallback на Stop().
  - Не обрабатывать сигналы в отдельной горутине.
    Решение: использовать горутину для ожидания сигналов.
  - Не закрывать соединения с БД и другие ресурсы.
    Решение: добавить cleanup-хуки после GracefulStop().
  - Не переводить health-статус в NOT_SERVING перед shutdown.
    Решение: переводить статус за 5-10 секунд до shutdown.
  - Игнорировать стриминговые RPC.
    Решение: проверять stream.Context().Done() в обработчиках.
  - Не проверять ошибку при s.Serve() (может вернуть grpc.ErrServerStopped).
    Решение: игнорировать только эту ошибку.


9. КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1. Graceful Shutdown — корректное завершение сервера с ожиданием
     завершения активных RPC.
  2. Stop() — жёсткая остановка, обрывает все соединения.
     GracefulStop() — корректная остановка, дожидается завершения RPC.
  3. GracefulStop() блокируется до завершения всех активных RPC.
  4. Для обработки сигналов используй signal.Notify() с SIGINT и SIGTERM.
  5. Всегда устанавливай таймаут на graceful shutdown с fallback на Stop().
  6. Для стриминговых RPC проверяй stream.Context().Done() в цикле.
  7. Перед shutdown переводи health-статус в NOT_SERVING.
  8. В Kubernetes учитывай terminationGracePeriodSeconds (обычно 30 секунд).
  9. Канал для сигналов должен быть буферизированным (размер 1).
  10. Graceful shutdown — обязательный паттерн для zero-downtime deployments.

*/

// ХРАНИЛИЩЕ
type TaskStore struct {
	mu    sync.RWMutex
	tasks map[string]int32 // task_id -> progress
}

func NewTaskStore() *TaskStore {
	return &TaskStore{
		tasks: make(map[string]int32),
	}
}

func (s *TaskStore) GetProgress(taskID string) int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[taskID]
}

func (s *TaskStore) SetProgress(taskID string, progress int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[taskID] = progress
}

//СЕРВЕР

type TaskServer struct {
	pb.UnimplementedTaskServiceServer
	store *TaskStore
}

func (s *TaskServer) ProcessTask(ctx context.Context, req *pb.ProcessTaskRequest) (*pb.TaskStatus, error) {
	if req.TaskId == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}
	if req.DelayMs > 0 {
		time.Sleep(time.Duration(req.DelayMs) * time.Millisecond)
	}
	return &pb.TaskStatus{
		TaskId:   req.TaskId,
		Status:   "completed",
		Progress: 100,
	}, nil
}

func (s *TaskServer) ProcessTaskStream(req *pb.ProcessTaskRequest, stream pb.TaskService_ProcessTaskStreamServer) error {
	ctx := stream.Context()
	taskID := req.TaskId
	delay := time.Duration(req.DealyMs) * time.Millisecond

	log.Printf("Начало обработки задачи %s (задержка: %v)", taskID, delay)

	for progress := 0; progress <= 100; progress += 10 {
		/*
			КРИТИЧЕСКИ ВАЖНО: проверяем контекст на каждой итерации
			Если сервер завершается (GracefulStop), контекст отменяется,
			и мы должны выйти из стрима, чтобы не блокировать shutdown
		*/
		select {
		case <-ctx.Done:
			log.Printf("Стрим задачи %s прерван (shutdown)", taskID)
			return status.Error(codes.Canceled, "stream cancelled")
		default:
			// продолжаем
		}

		// Имитация обработки
		time.Sleep(delay)

		s.store.SetProgress(taskID, int32(progress))

		status := "processing"
		if progress == 100 {
			status = "completed"
		}

		if err := stream.Send(&pb.TaskStatus{
			TaskId:   taskID,
			Status:   status,
			Progress: int32(progress),
		}); err != nil {
			return err
		}
		log.Printf("Задача %s: прогресс %d%%", taskID, progress)
	}
	log.Printf("Задача %s завершена", taskID)
	return nil
}

func main() {
	// 1. СОЗДАЁМ СЕРВЕР
	store := NewTaskStore()
	s := grpc.NewServer()
	pb.RegisterTaskServiceServer(s, &TaskServer{store: store})

	// 2. HEALTH CHECKS (для Kubernetes)
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	log.Println("Health status: SERVING")

	// 3. ЗАПУСК
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Println("Сервер запущен на :50051")

	go func() {
		if err := s.Serve(lis); err != nil {
			if err != grpc.ErrServerStopped {
				log.Fatalf("Server error: %v", err)
			}
		}
	}()

	// 4. ОЖИДАНИЕ СИГНАЛА
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Получен сигнал завершения, начинаем graceful shutdown...")

	// 5. HEALTH: NOT_SERVING
	log.Println("Установка health статуса: NOT_SERVING")
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

	// Даём время балансировщику обновить состояние
	time.Sleep(5 * time.Second)

	// 6. GRACEFUL SHUTDOWN С ТАЙМАУТОМ
	shutdownDone := make(chan struct{})
	go func() {
		log.Println("Ожидание завершения активных RPC...")
		s.GracefulStop()
		close(shutdownDone)
		log.Println("Все RPC завершены")
	}()

	// Таймаут 28 секунд (как в Kubernetes)
	select {
	case <-shutdownDone:
		log.Println("Graceful shutdown завершён")
	case <-time.After(28 * time.Second):
		log.Println("Таймаут graceful shutdown, принудительная остановка...")
		s.Stop()
	}

	log.Println("Сервер остановлен")
}
