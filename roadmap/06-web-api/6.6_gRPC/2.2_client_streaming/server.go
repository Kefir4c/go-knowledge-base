package clientstreaming

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/"
)

/*
  РАЗДЕЛ 2.2: CLIENT-STREAMING RPC
  Client-Streaming RPC — это третий тип RPC в gRPC, который позволяет КЛИЕНТУ
  отправлять ПОТОК сообщений на сервер, а сервер возвращает ОДИН ответ
  после получения всех сообщений.

  Это зеркальное отражение Server-Streaming: там сервер шлёт поток,
  здесь — клиент.

  В этом разделе мы разберём:
    1.  Что такое Client-Streaming RPC и когда он используется
    2.  Определение в .proto-файле (ключевое слово stream перед запросом)
    3.  Реализация сервера: чтение в цикле, SendAndClose()
    4.  Реализация клиента: отправка в цикле, CloseAndRecv()
    5.  Важные нюансы: обработка ошибок, порядок сообщений, дедлайны
    6.  Полный рабочий пример (теория + код)
    7.  Client-Streaming vs Unary vs Server-Streaming
    8.  Частые ошибки и best practices

  1.  ЧТО ТАКОЕ CLIENT-STREAMING RPC И КОГДА ОН ИСПОЛЬЗУЕТСЯ
  Client-Streaming RPC — это вызов, при котором:
    • Клиент отправляет ПОТОК сообщений на сервер.
    • Сервер собирает все сообщения и после завершения потока
      возвращает ОДИН ответ.

  Аналогия: клиент загружает большой файл по кускам (чанкам),
  а сервер в конце отвечает "Загрузка завершена" с результатом.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Загрузка больших файлов (чанками).
    • Пакетная обработка (пакет заказов, массовое обновление).
    • Когда клиент не знает заранее, сколько данных он отправит.
    • Сбор данных с устройств (IoT).
    • Потоковое логирование.

  Преимущества:
    • Клиент может отправлять данные по мере их поступления.
    • Экономия памяти: не нужно собирать всё в один большой запрос.
    • Сервер может начать обрабатывать данные до получения всех частей
      (но ответ всё равно один).

  2.  ОПРЕДЕЛЕНИЕ В .PROTO-ФАЙЛЕ
  В .proto-файле Client-Streaming RPC описывается с ключевым словом stream
  перед типом запроса:

    service OrderService {
      rpc CreateOrders (stream OrderRequest) returns (CreateOrdersResponse) {}
    }

  Где:
    • CreateOrders — имя метода.
    • stream OrderRequest — тип запроса (поток).
    • CreateOrdersResponse — тип ответа (один).

  Полный пример:

    syntax = "proto3";
    package order.v1;
    option go_package = "github.com/example/api/order/v1;orderv1";

    message OrderRequest {
      string product_id = 1;
      int32 quantity = 2;
    }

    message CreateOrdersResponse {
      int32 total_created = 1;
      repeated string order_ids = 2;
    }

    service OrderService {
      rpc CreateOrders (stream OrderRequest) returns (CreateOrdersResponse) {}
    }

  После генерации кода:
    • order_grpc.pb.go будет содержать методы:
      - CreateOrders(OrderService_CreateOrdersServer) error — на сервере
      - CreateOrders(ctx context.Context, opts ...) (OrderService_CreateOrdersClient, error) — на клиенте

  3.  РЕАЛИЗАЦИЯ СЕРВЕРА
  3.1. Сигнатура метода
    func (s *server) CreateOrders(stream pb.OrderService_CreateOrdersServer) error

  В Client-Streaming сервер принимает НЕ запрос, а поток (интерфейс стрима).

  3.2. Чтение сообщений в цикле
    func (s *server) CreateOrders(stream pb.OrderService_CreateOrdersServer) error {
      var orderIDs []string
      var total int

      for {
        req, err := stream.Recv()
        if err == io.EOF {
          // Клиент закончил отправку, возвращаем ответ
          return stream.SendAndClose(&pb.CreateOrdersResponse{
            TotalCreated: int32(total),
            OrderIds: orderIDs,
          })
        }
        if err != nil {
          // Обработка ошибки
          return err
        }

        // Обработка заказа
        orderID := fmt.Sprintf("order-%d", total+1)
        orderIDs = append(orderIDs, orderID)
        total++
      }
    }

  3.3. Ключевые моменты
    • io.EOF — сигнал от клиента, что сообщений больше не будет.
    • stream.SendAndClose() — отправляет ответ и закрывает стрим.
    • SendAndClose() можно вызвать ТОЛЬКО ОДИН РАЗ.
    • Если вернуть ошибку до SendAndClose, стрим закроется с ошибкой.

  3.4. Контекст в Client-Streaming
    Контекст можно получить через stream.Context():

      ctx := stream.Context()
      if ctx.Err() == context.DeadlineExceeded {
        return status.Error(codes.DeadlineExceeded, "deadline exceeded")
      }

  3.5. Проверка контекста в цикле
    for {
      select {
      case <-stream.Context().Done():
        return status.Error(codes.Canceled, "client cancelled")
      default:
        req, err := stream.Recv()
        // ...
      }
    }

  4.  РЕАЛИЗАЦИЯ КЛИЕНТА
  4.1. Вызов метода
    stream, err := client.CreateOrders(ctx)
    if err != nil {
      log.Fatal(err)
    }

  4.2. Отправка сообщений в цикле
    for _, order := range orders {
      if err := stream.Send(&pb.OrderRequest{
        ProductId: order.ProductID,
        Quantity:  order.Quantity,
      }); err != nil {
        // Обработка ошибки отправки
        log.Fatal(err)
      }
    }

  4.3. Закрытие стрима и получение ответа
    resp, err := stream.CloseAndRecv()
    if err != nil {
      // Обработка ошибки
      st, ok := status.FromError(err)
      if ok {
        switch st.Code() {
        case codes.DeadlineExceeded:
          log.Println("timeout")
        default:
          log.Printf("error: %v", st.Message())
        }
      }
      return
    }
    log.Printf("Создано заказов: %d", resp.TotalCreated)

  4.4. Ключевые моменты
    • stream.CloseAndRecv() — закрывает стрим и ждёт ответ.
    • CloseAndRecv() блокируется, пока сервер не вызовет SendAndClose().
    • Если сервер возвращает ошибку, CloseAndRecv() вернёт её.
    • Отправлять сообщения после CloseAndRecv() нельзя.

  4.5. Отмена стрима со стороны клиента
    // Если нужно отменить стрим до завершения:
    cancel() // отменяем контекст
    // После этого stream.Recv() на сервере получит ошибку

  5.  ВАЖНЫЕ НЮАНСЫ
  5.1. Порядок сообщений
    gRPC ГАРАНТИРУЕТ сохранение порядка сообщений в стриме.
    Если клиент отправил [A, B, C], сервер получит [A, B, C].

  5.2. Ошибки во время отправки
    • Если клиент отправит сообщение после того, как сервер закрыл стрим,
      Send() вернёт ошибку.
    • Если сервер вернул ошибку, клиент получит её при CloseAndRecv().

  5.3. Дедлайны
    • Дедлайн устанавливается на клиенте через context.WithTimeout.
    • Если дедлайн истёк, сервер получит контекст с DeadlineExceeded.
    • Сервер может проверить ctx.Done() в цикле чтения.

  5.4. Частичные результаты
    • В Client-Streaming сервер НЕ МОЖЕТ отправлять промежуточные ответы.
    • Все результаты возвращаются только в SendAndClose().
    • Если нужно отправлять промежуточные результаты, используй
      Bidirectional Streaming.

  6.  ПОЛНЫЙ РАБОЧИЙ ПРИМЕР (ТЕОРИЯ + КОД)
  6.1. .proto-файл (proto/order.proto)
    syntax = "proto3";
    package order.v1;
    option go_package = "github.com/example/streaming-example/proto/order;orderpb";

    message OrderRequest {
      string product_id = 1;
      int32 quantity = 2;
    }

    message CreateOrdersResponse {
      int32 total_created = 1;
      repeated string order_ids = 2;
    }

    service OrderService {
      rpc CreateOrders (stream OrderRequest) returns (CreateOrdersResponse) {}
    }

  6.2. Сервер (server/main.go)

    type server struct {
      pb.UnimplementedOrderServiceServer
    }

    func (s *server) CreateOrders(stream pb.OrderService_CreateOrdersServer) error {
      ctx := stream.Context()
      var orderIDs []string
      var total int

      log.Println("Начало обработки потока заказов")

      for {
        // Проверяем, не отменил ли клиент
        select {
        case <-ctx.Done():
          return status.Error(codes.Canceled, "client cancelled")
        default:
        }

        req, err := stream.Recv()
        if err == io.EOF {
          log.Printf("Получено %d заказов, отправляем ответ", total)
          return stream.SendAndClose(&pb.CreateOrdersResponse{
            TotalCreated: int32(total),
            OrderIds:     orderIDs,
          })
        }
        if err != nil {
          log.Printf("Ошибка чтения: %v", err)
          return err
        }

        // Имитация обработки заказа
        orderID := fmt.Sprintf("order-%d", total+1)
        orderIDs = append(orderIDs, orderID)
        total++
        log.Printf("Обработан заказ: продукт=%s, кол-во=%d, ID=%s",
          req.ProductId, req.Quantity, orderID)

        // Имитация задержки
        time.Sleep(100 * time.Millisecond)
      }
    }

    func main() {
      s := grpc.NewServer()
      pb.RegisterOrderServiceServer(s, &server{})

      lis, _ := net.Listen("tcp", ":50051")
      log.Println("Сервер запущен на :50051")

      go func() {
        if err := s.Serve(lis); err != nil {
          log.Fatal(err)
        }
      }()

      stop := make(chan os.Signal, 1)
      signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
      <-stop

      log.Println("Остановка...")
      s.GracefulStop()
      log.Println("Остановлен")
    }

  6.3. Клиент (client/main.go)
    package main

    import (
      "context"
      "log"
      "time"

      "google.golang.org/grpc"
      "google.golang.org/grpc/codes"
      "google.golang.org/grpc/credentials/insecure"
      "google.golang.org/grpc/status"
      pb "github.com/example/streaming-example/proto/order"
    )

    func main() {
      conn, _ := grpc.NewClient("localhost:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
      )
      defer conn.Close()

      client := pb.NewOrderServiceClient(conn)

      ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
      defer cancel()

      stream, err := client.CreateOrders(ctx)
      if err != nil {
        log.Fatal(err)
      }

      // Отправляем заказы
      orders := []struct {
        ProductID string
        Quantity  int32
      }{
        {"p1", 2},
        {"p2", 1},
        {"p3", 5},
        {"p4", 3},
      }

      log.Println("Отправка заказов...")
      for _, o := range orders {
        if err := stream.Send(&pb.OrderRequest{
          ProductId: o.ProductID,
          Quantity:  o.Quantity,
        }); err != nil {
          log.Fatalf("Ошибка отправки: %v", err)
        }
        log.Printf("Отправлен заказ: %+v", o)
        time.Sleep(200 * time.Millisecond)
      }

      // Закрываем стрим и получаем ответ
      log.Println("Закрытие стрима...")
      resp, err := stream.CloseAndRecv()
      if err != nil {
        st, ok := status.FromError(err)
        if ok {
          switch st.Code() {
          case codes.DeadlineExceeded:
            log.Println("Таймаут")
          case codes.Canceled:
            log.Println("Отменено")
          default:
            log.Printf("Ошибка: %v", st.Message())
          }
        } else {
          log.Printf("Не-gRPC ошибка: %v", err)
        }
        return
      }

      log.Printf("✅ Создано заказов: %d", resp.TotalCreated)
      log.Printf("✅ ID заказов: %v", resp.OrderIds)
    }

  6.4. Запуск
    # Терминал 1
    go run server/main.go

    # Терминал 2
    go run client/main.go

    # Вывод:
    # Отправка заказов...
    # Отправлен заказ: {p1 2}
    # Отправлен заказ: {p2 1}
    # Отправлен заказ: {p3 5}
    # Отправлен заказ: {p4 3}
    # Закрытие стрима...
    # Создано заказов: 4
    # ID заказов: [order-1 order-2 order-3 order-4]

  7.  CLIENT-STREAMING VS UNARY VS SERVER-STREAMING
  ┌─────────────────────┬─────────────┬─────────────────────┬─────────────────────┐
  │ Характеристика      │ Unary       │ Server-Streaming    │ Client-Streaming    │
  ├─────────────────────┼─────────────┼─────────────────────┼─────────────────────┤
  │ Запросов            │ 1           │ 1                   │ N (поток)           │
  ├─────────────────────┼─────────────┼─────────────────────┼─────────────────────┤
  │ Ответов             │ 1           │ N (поток)           │ 1                   │
  ├─────────────────────┼─────────────┼─────────────────────┼─────────────────────┤
  │ Кто шлёт поток      │ -           │ Сервер              │ Клиент              │
  ├─────────────────────┼─────────────┼─────────────────────┼─────────────────────┤
  │ Серверный метод     │ принимает   │ принимает запрос    │ принимает поток     │
  │                     │ запрос      │ + стрим ответов     │                     │
  ├─────────────────────┼─────────────┼─────────────────────┼─────────────────────┤
  │ Клиентский метод    │ возвращает  │ возвращает стрим    │ возвращает стрим    │
  │                     │ ответ       │ для чтения          │ для отправки        │
  ├─────────────────────┼─────────────┼─────────────────────┼─────────────────────┤
  │ Use case            │ CRUD        │ Списки, подписки    │ Загрузка, пакетная  │
  └─────────────────────┴─────────────┴─────────────────────┴─────────────────────┘

  8.  ЧАСТЫЕ ОШИБКИ И BEST PRACTICES
  8.1. Частые ошибки
Закрывать стрим без отправки ответа. Клиент будет ждать ответ вечно.
Игнорировать ошибки при stream.Send(). При обрыве соединения отправка упадёт.
Думать, что io.EOF — ошибка на сервере. Это сигнал от клиента, что он закончил.
Вызывать SendAndClose() несколько раз. Это приведёт к панике.
Не проверять ctx.Done() в цикле чтения. Клиент может отменить запрос, а сервер продолжит чтение.

  8.2. Best Practices
	Всегда проверяй ctx.Done() в цикле чтения.
	Обрабатывай ошибки при Recv() и Send().
	Используй дедлайн на клиенте.
	Для больших объёмов данных используй стриминг, а не один запрос.
	Добавляй логирование в сервер (для отладки стримов).
	На клиенте всегда вызывай CloseAndRecv() после отправки.
	Используй интерсепторы для логирования стримов (StreamInterceptor).

  9.  КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1.  Client-Streaming RPC — «поток запросов → один ответ».
  2.  Определяется в .proto: rpc Create(stream Request) returns (Response).
  3.  Сервер читает в цикле через Recv() до io.EOF.
  4.  Сервер отвечает через SendAndClose() (один раз).
  5.  Клиент отправляет через Send() в цикле.
  6.  Клиент закрывает и получает ответ через CloseAndRecv().
  7.  io.EOF на сервере — признак завершения стрима от клиента.
  8.  SendAndClose() вызывается ТОЛЬКО ОДИН РАЗ.
  9.  Использовать для: загрузка файлов, пакетная обработка.
  10. При отмене контекста стрим прерывается с обеих сторон.
*/

const (
	maxFileSize  = 512 * 1024 * 1024
	maxChunkSize = 4 * 1024 * 1024
	uploadDir    = "./uploads"
)

//Сервер

type FileServer struct {
	pb.UnimplementedFileServiceServer
	mu        sync.RWMutex
	uploads   map[string]*UploadSession // активные загрузки
	fileCount int
}

// UploadSession — сессия загрузки (для трекинга)
type UploadSession struct {
	FileID    string
	Filename  string
	Size      int64
	Received  int64
	Writer    *os.File
	StartTime time.Time
	Metadata  *pb.FileMetadata
	mu        sync.Mutex
}

func NewFileServer() *FileServer {
	// Создаём директорию для загрузок, если её нет
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Не удалось создать директорию uploads: %v", err)
	}
	return &FileServer{
		uploads: make(map[string]*UploadSession),
	}
}

// UPLOADFILE — CLIENT-STREAMING
func (s *FileServer) UploadFile(stream pb.FileService_UploadFileServer) error {
	ctx := stream.Context()
	startTime := time.Now()

	// Проверяем дедлайн
	if ctx.Err() == context.DeadlineExceeded {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	log.Println("Начало загрузки файла")

	//ШАГ 1: Получаем метаданные
	firstMsg, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "metadata required as first message")
	}

	metadata := firstMsg.GetMetadata()
	if metadata == nil {
		return status.Error(codes.InvalidArgument, "first message must be metadata")
	}

	// Валидация метаданных
	if metadata.Filename == "" {
		return status.Error(codes.InvalidArgument, "filename is required")
	}
	if metadata.Size <= 0 || metadata.Size > maxFileSize {
		return status.Errorf(codes.InvalidArgument, "file size must be between 1 and %d bytes", maxFileSize)
	}

	log.Printf("Метаданные: файл=%s, размер=%d, тип=%s, user=%s",
		metadata.Filename, metadata.Size, metadata.ContentType, metadata.UserId)

	//ШАГ 2: Создаём файл на диске

	// Генерируем уникальный ID
	fileID := fmt.Sprintf("file-%d", time.Now().UnixNano())
	safeFilename := filepath.Base(metadata.Filename)
	filePath := filepath.Join(uploadDir, fileID+"_"+safeFilename)

	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("Ошибка создания файла: %v", err)
		return status.Error(codes.Internal, "failed to create file")
	}
	defer file.Close()

	// Создаём сессию
	session := &UploadSession{
		FileID:    fileID,
		Filename:  safeFilename,
		Size:      metadata.Size,
		Received:  0,
		Writer:    file,
		StartTime: startTime,
		Metadata:  metadata,
	}

	s.mu.Lock()
	s.uploads[fileID] = session
	s.mu.Unlock()

	//ШАГ 4: Получаем чанки и записываем на диск

	var totalReceived int64
	var firstChunk []byte
	var firstChunkReceived bool

	for {
		// Проверяем контекст (отмена клиентом)
		select {
		case <-ctx.Done():
			log.Printf("Клиент отменил загрузку: %v", ctx.Err())
			s.mu.Lock()
			delete(s.uploads, fileID)
			s.mu.Unlock()
			return status.Error(codes.Canceled, "client cancelled")
		default:
		}

		msg, err := stream.Recv()
		if err == io.EOF {
			// Клиент закончил отправку
			break
		}
		if err != nil {
			log.Printf("Ошибка чтения стрима: %v", err)
			s.mu.Lock()
			delete(s.uploads, fileID)
			s.mu.Unlock()
			return err
		}
		chunk := msg.GetChunk()
		if chunk == nil {
			// Если пришло что-то ещё — игнорируем (в реальности ошибка)
			continue
		}

		chunkLen := len(chunk)
		if chunkLen > maxChunkSize {
			return status.Errorf(codes.InvalidArgument, "chunk size %d exceeds max %d", chunkLen, maxChunkSize)
		}

		// Сохраняем первый чанк для проверки типа (Magic Bytes)
		if !firstChunkReceived {
			firstChunk = chunk
			firstChunkReceived = true
		}

		// Проверяем размер (не превысили ли лимит)
		totalReceived += int64(chunkLen)
		if totalReceived > maxFileSize {
			return status.Errorf(codes.InvalidArgument, "file size %d exceeds max %d", totalReceived, maxFileSize)
		}

		// Записываем чанк на диск [citation:5]
		if _, err := file.Write(chunk); err != nil {
			log.Printf("Ошибка записи на диск: %v", err)
			return status.Error(codes.Internal, "failed to write chunk")
		}

		// Обновляем сессию
		s.mu.Lock()
		session.Received = totalReceived
		s.mu.Unlock()

		// Логируем прогресс (каждые 10%)
		if totalReceived%int64(metadata.Size/10) == 0 {
			progress := float64(totalReceived) / float64(metadata.Size) * 100
			log.Printf("Прогресс: %.0f%%", progress)
		}
	}

	//ШАГ 5: Проверка типа файла
	// Проверяем реальный тип файла по магии байтов
	if firstChunkReceived {
		detectedType := http.DetectContentType(firstChunk)
		if metadata.ContentType != "" && strings.HasPrefix(detectedType, "application/octet-stream") {
			if !isContentTypeMatch(detectedType, metadata.ContentType) {
				log.Printf("⚠️ Несовпадение типов: заявлен %s, реальный %s",
					metadata.ContentType, detectedType)
				// В продакшене мы бы вернули ошибку, но для демо просто логируем
			}
		}
	}

	//ШАГ 6: Проверка, что все байты получены

	if totalReceived != metadata.Size {
		return status.Errorf(codes.InvalidArgument,
			"size mismatch: expected %d, received %d", metadata.Size, totalReceived)
	}

	// Закрываем файл (уже закрыто defer, но явно синхронизируем)
	if err := file.Sync(); err != nil {
		log.Printf("⚠️ Ошибка синхронизации: %v", err)
	}

	//ШАГ 7: Отправляем ответ

	log.Printf("✅ Файл загружен: %s, размер=%d, время=%v",
		safeFilename, totalReceived, time.Since(startTime))

	s.mu.Lock()
	delete(s.uploads, fileID)
	s.mu.Unlock()

	return stream.SendAndClose(&pb.UploadFileResponse{
		FileId:     fileID,
		Message:    fmt.Sprintf("File %s uploaded successfully", safeFilename),
		UploadedAt: timestamppb.Now(),
	})
}

// ─── DOWNLOADFILE — SERVER-STREAMING ────────────────────────────────────────

func (s *FileServer) DownloadFile(req *pb.DownloadFileRequest, stream pb.FileService_DownloadFileServer) error {
	ctx := stream.Context()

	if req.FileId == "" {
		return status.Error(codes.InvalidArgument, "file_id is required")
	}

	// Ищем файл на диске
	var filePath string
	var fileInfo os.FileInfo

	err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.Contains(path, req.FileId) {
			filePath = path
			fileInfo = info
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil || filePath == "" {
		return status.Error(codes.NotFound, "file not found")
	}

	// Открываем файл
	file, err := os.Open(filePath)
	if err != nil {
		return status.Error(codes.Internal, "failed to open file")
	}
	defer file.Close()

	// Отправляем информацию о файле
	info := &pb.FileInfo{
		Filename:    fileInfo.Name(),
		ContentType: http.DetectContentType([]byte{}), // упрощённо
		Size:        fileInfo.Size(),
	}
	if err := stream.Send(&pb.DownloadFileResponse{
		Data: &pb.DownloadFileResponse_Info{Info: info},
	}); err != nil {
		return err
	}

	// Отправляем чанки
	buffer := make([]byte, 64*1024) // 64KB [citation:5]
	for {
		select {
		case <-ctx.Done():
			return status.Error(codes.Canceled, "client cancelled")
		default:
		}

		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Error(codes.Internal, "failed to read file")
		}

		if err := stream.Send(&pb.DownloadFileResponse{
			Data: &pb.DownloadFileResponse_Chunk{Chunk: buffer[:n]},
		}); err != nil {
			return err
		}
	}

	log.Printf("📥 Файл скачан: %s", req.FileId)
	return nil
}

//ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ

// isContentTypeMatch — проверка соответствия типа файла
func isContentTypeMatch(actual, declared string) bool {
	// В реальном проекте здесь более сложная логика
	// Для демо просто проверяем, что типы совпадают по префиксу
	if actual == declared {
		return true
	}
	// image/jpeg vs image/png — оба image
	if strings.HasPrefix(actual, "image/") && strings.HasPrefix(declared, "image/") {
		return true
	}
	return false
}

//INTERCEPTORS

func StreamLoggingInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	log.Printf("🔁 [STREAM] Начало: %s", info.FullMethod)
	start := time.Now()
	err := handler(srv, ss)
	log.Printf("🔁 [STREAM] Конец: %s, ошибка: %v, длительность: %v",
		info.FullMethod, err, time.Since(start))
	return err
}

func main() {
	server := NewFileServer()

	s := grpc.NewServer(
		grpc.StreamInterceptor(StreamLoggingInterceptor),
	)

	pb.RegisterFileServiceServer(s, server)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("gRPC файловый сервер запущен на :50051")
	log.Printf("Директория загрузок: %s", uploadDir)
	log.Printf("Максимальный размер файла: %d MB", maxFileSize/1024/1024)
	log.Printf("Максимальный размер чанка: %d MB", maxChunkSize/1024/1024)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("⏳ Остановка сервера...")
	s.GracefulStop()
	log.Println("✅ Сервер остановлен")
}
