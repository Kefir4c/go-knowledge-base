package __1_server_streaming

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
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/"
)

/*
  РАЗДЕЛ 2.1: SERVER-STREAMING RPC

  Server-Streaming RPC — это второй тип RPC в gRPC, который позволяет серверу
  возвращать ПОТОК сообщений в ответ на ОДИН запрос от клиента.

  Это принципиальное отличие от Unary RPC, где сервер всегда возвращает
  только одно сообщение. Server-Streaming — это про передачу нескольких
  сообщений в рамках одного вызова.

  В этом разделе мы разберём:
    1.  Что такое Server-Streaming RPC и когда он используется
    2.  Определение в .proto-файле (ключевое слово stream)
    3.  Реализация сервера: методы Send и контекст
    4.  Реализация клиента: чтение в цикле до io.EOF
    5.  Важные нюансы: порядок сообщений, обработка ошибок, дедлайны
    6.  Полный рабочий пример (теория + код)
    7.  Server-Streaming vs Unary: когда что использовать
    8.  Частые ошибки и best practices

  1.  ЧТО ТАКОЕ SERVER-STREAMING RPC И КОГДА ОН ИСПОЛЬЗУЕТСЯ
  Server-Streaming RPC — это вызов, при котором:
    • Клиент отправляет ОДИН запрос.
    • Сервер отправляет ПОТОК сообщений в ответ.
    • Клиент читает сообщения из потока до тех пор, пока сервер не закроет его.

  Аналогия: как HTTP-стриминг или Server-Sent Events (SSE) — отправил запрос,
  и получаешь данные по кусочкам.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Получение больших списков данных (пагинация через поток).
    • Подписка на события (server push).
    • Выгрузка больших файлов/данных.
    • Любой сценарий, где клиент не знает заранее, сколько данных придёт.

  Преимущества:
    • Экономия памяти: данные не загружаются целиком на сервере.
    • Быстрый старт: клиент получает данные по мере их готовности.
    • Удобно для real-time сценариев.

  2.  ОПРЕДЕЛЕНИЕ В .PROTO-ФАЙЛЕ
  В .proto-файле Server-Streaming RPC описывается с ключевым словом stream
  перед типом ответа:

    service UserService {
      rpc ListUsers (ListUsersRequest) returns (stream User) {}
    }

  Где:
    • ListUsers — имя метода.
    • ListUsersRequest — тип запроса (один).
    • stream User — тип ответа (поток).

  Полный пример:

    syntax = "proto3";
    package user.v1;
    option go_package = "github.com/example/api/user/v1;userv1";

    message ListUsersRequest {
      int32 page_size = 1;
      string page_token = 2;
    }

    message User {
      string id = 1;
      string name = 2;
      string email = 3;
    }

    service UserService {
      rpc ListUsers (ListUsersRequest) returns (stream User) {}
    }

  После генерации кода:
    • user_grpc.pb.go будет содержать методы:
      - ListUsers(*ListUsersRequest, UserService_ListUsersServer) error
      - UserService_ListUsersServer — интерфейс с методами Send и RecvMsg

  3.  РЕАЛИЗАЦИЯ СЕРВЕРА
  3.1. Сигнатура метода
    func (s *server) ListUsers(req *pb.ListUsersRequest, stream pb.UserService_ListUsersServer) error

  В отличие от Unary, в Server-Streaming вторым аргументом передаётся
  НЕ контекст, а интерфейс стрима (UserService_ListUsersServer).

  3.2. Отправка сообщений через stream.Send()
    func (s *server) ListUsers(req *pb.ListUsersRequest, stream pb.UserService_ListUsersServer) error {
      for _, user := range s.users {
        if err := stream.Send(user); err != nil {
          return err
        }
      }
      return nil
    }

  3.3. Контекст в Server-Streaming
  Контекст можно получить через stream.Context():

    func (s *server) ListUsers(req *pb.ListUsersRequest, stream pb.UserService_ListUsersServer) error {
      ctx := stream.Context()

      // Проверка дедлайна
      if ctx.Err() == context.DeadlineExceeded {
        return status.Error(codes.DeadlineExceeded, "deadline exceeded")
      }

      for _, user := range s.users {
        // Проверка отмены на каждой итерации
        select {
        case <-ctx.Done():
          return status.Error(codes.Canceled, "client cancelled")
        default:
          if err := stream.Send(user); err != nil {
            return err
          }
        }
      }
      return nil
    }

  3.4. Обработка ошибок при отправке
    • Если клиент закрыл соединение, stream.Send() вернёт ошибку.
    • Ошибку нужно возвращать, gRPC автоматически завершит стрим.

  3.5. Варианты завершения стрима
    • Естественное завершение: функция возвращает nil.
    • Ошибка: функция возвращает error (gRPC статус).
    • При ошибке дальнейшие Send() будут игнорироваться.

  4.  РЕАЛИЗАЦИЯ КЛИЕНТА

  4.1. Вызов метода
    stream, err := client.ListUsers(ctx, &pb.ListUsersRequest{PageSize: 10})
    if err != nil {
      log.Fatal(err)
    }

  4.2. Чтение сообщений в цикле
    for {
      user, err := stream.Recv()
      if err == io.EOF {
        break // стрим закрыт, сообщений больше нет
      }
      if err != nil {
        // Обработка ошибки
        st, ok := status.FromError(err)
        if ok {
          switch st.Code() {
          case codes.DeadlineExceeded:
            log.Println("timeout")
          case codes.Canceled:
            log.Println("cancelled")
          default:
            log.Printf("error: %v", st.Message())
          }
        }
        break
      }
      log.Printf("User: %+v", user)
    }

  4.3. Важные нюансы
    • io.EOF — это НЕ ошибка, это признак завершения потока.
    • Если сервер возвращает ошибку, Recv() вернёт её, а не io.EOF.
    • Сообщения приходят в том же порядке, в котором их отправил сервер.

  4.4. Отмена стрима со стороны клиента
    // Клиент может отменить запрос через контекст
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    stream, err := client.ListUsers(ctx, req)

    // В какой-то момент:
    cancel() // стрим будет прерван, server получит контекст с отменой

  5.  ВАЖНЫЕ НЮАНСЫ: ПОРЯДОК СООБЩЕНИЙ, ОБРАБОТКА ОШИБОК, ДЕДЛАЙНЫ
  5.1. Порядок сообщений
    gRPC ГАРАНТИРУЕТ сохранение порядка сообщений в стриме.
    Если сервер отправил [A, B, C], клиент получит [A, B, C].

  5.2. Ошибки в стриме
    • Если сервер возвращает ошибку, стрим завершается.
    • Клиент получит эту ошибку при вызове Recv() (не io.EOF).
    • Ошибку нужно обрабатывать через status.FromError.

  5.3. Дедлайны
    • Дедлайн устанавливается на клиенте через context.WithTimeout.
    • Если дедлайн истёк, сервер получит контекст с DeadlineExceeded.
    • Сервер должен проверять ctx.Done() в цикле отправки.

  5.4. Метаданные (metadata)
    Работает так же, как и в Unary RPC:
      • Клиент: metadata.NewOutgoingContext.
      • Сервер: metadata.FromIncomingContext (из stream.Context()).

  5.5. Интерсепторы
    • Для Server-Streaming используются StreamInterceptor.
    • Отличается от UnaryInterceptor (работает с потоками).

  6.  ПОЛНЫЙ РАБОЧИЙ ПРИМЕР (ТЕОРИЯ + КОД)
  6.1. .proto-файл (proto/user.proto)

    message ListUsersRequest {
      int32 limit = 1;
    }

    message User {
      string id = 1;
      string name = 2;
      string email = 3;
    }

    service UserService {
      rpc ListUsers (ListUsersRequest) returns (stream User) {}
    }

  6.2. Сервер (server/main.go)

    type server struct {
      pb.UnimplementedUserServiceServer
      mu    sync.RWMutex
      users []*pb.User
    }

    func (s *server) ListUsers(req *pb.ListUsersRequest, stream pb.UserService_ListUsersServer) error {
      ctx := stream.Context()

      // Проверка дедлайна
      if ctx.Err() == context.DeadlineExceeded {
        return status.Error(codes.DeadlineExceeded, "deadline exceeded")
      }

      // Подготовка данных
      s.mu.RLock()
      users := make([]*pb.User, len(s.users))
      copy(users, s.users)
      s.mu.RUnlock()

      limit := int(req.Limit)
      if limit == 0 || limit > len(users) {
        limit = len(users)
      }

      // Отправляем пользователей по одному
      for i := 0; i < limit; i++ {
        // Проверяем, не отменил ли клиент
        select {
        case <-ctx.Done():
          return status.Error(codes.Canceled, "client cancelled")
        default:
          if err := stream.Send(users[i]); err != nil {
            return err
          }
          // Имитация задержки (для демонстрации стриминга)
          time.Sleep(200 * time.Millisecond)
        }
      }
      return nil
    }

    func main() {
      srv := &server{
        users: []*pb.User{
          {Id: "1", Name: "Alice", Email: "alice@ex.com"},
          {Id: "2", Name: "Bob", Email: "bob@ex.com"},
          {Id: "3", Name: "Charlie", Email: "charlie@ex.com"},
          {Id: "4", Name: "Diana", Email: "diana@ex.com"},
          {Id: "5", Name: "Eve", Email: "eve@ex.com"},
        },
      }

      lis, _ := net.Listen("tcp", ":50051")
      s := grpc.NewServer()
      pb.RegisterUserServiceServer(s, srv)
      log.Println("Server listening on :50051")
      s.Serve(lis)
    }

  6.3. Клиент (client/main.go)

    func main() {
      conn, err := grpc.NewClient("localhost:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
      )
      if err != nil {
        log.Fatal(err)
      }
      defer conn.Close()

      client := pb.NewUserServiceClient(conn)

      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()

      stream, err := client.ListUsers(ctx, &pb.ListUsersRequest{Limit: 3})
      if err != nil {
        log.Fatal(err)
      }

      log.Println("Получение пользователей (стрим):")
      for {
        user, err := stream.Recv()
        if err == io.EOF {
          log.Println("Стрим завершён")
          break
        }
        if err != nil {
          st, ok := status.FromError(err)
          if ok {
            switch st.Code() {
            case codes.DeadlineExceeded:
              log.Println("Таймаут")
            case codes.Canceled:
              log.Println("Отменено клиентом")
            default:
              log.Printf("Ошибка: %v", st.Message())
            }
          } else {
            log.Printf("Не-gRPC ошибка: %v", err)
          }
          break
        }
        log.Printf("  User: ID=%s, Name=%s, Email=%s", user.Id, user.Name, user.Email)
      }
    }

  6.4. Запуск
    # Терминал 1
    go run server/main.go

    # Терминал 2
    go run client/main.go

    # Вывод:
    # Получение пользователей (стрим):
    #   User: ID=1, Name=Alice, Email=alice@ex.com
    #   User: ID=2, Name=Bob, Email=bob@ex.com
    #   User: ID=3, Name=Charlie, Email=charlie@ex.com
    # Стрим завершён

  7.  SERVER-STREAMING VS UNARY: КОГДА ЧТО ИСПОЛЬЗОВАТЬ
  ┌─────────────────────┬──────────────────────┬────────────────────────────┐
  │ Критерий            │ Unary RPC            │ Server-Streaming RPC       │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Количество ответов  │ 1                    │ N (поток)                  │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Память на сервере   │ Весь ответ в памяти  │ По N сообщению        │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Скорость получения  │ Всё сразу            │ Постепенно                 │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Обработка ошибок    │ После всего запроса  │ В любой момент стрима      │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Use case            │ CRUD, запрос-ответ   │ Списки, подписки, файлы    │
  └─────────────────────┴──────────────────────┴────────────────────────────┘

  Server-Streaming стоит использовать, когда:
    • Данные большие (списки > 1000 записей).
    • Данные поступают постепенно (не все сразу готовы).
    • Нужно начать показывать данные до того, как все будут готовы.
    • Клиент может отменить загрузку в любой момент.

  8.  ЧАСТЫЕ ОШИБКИ И BEST PRACTICES
  8.1. Частые ошибки
Не проверять ctx.Done() в цикле отправки.
→ Если клиент отменит запрос, сервер продолжит отправку.

Игнорировать ошибки при stream.Send().
→ Если клиент закрыл соединение, отправка упадёт.

Думать, что io.EOF — это ошибка.
→ Это нормальное завершение стрима.

Возвращать nil, не закрыв стрим.
→ Стрим закроется автоматически при возврате из метода.

Отправлять слишком много данных без проверки контекста.
→ Может привести к утечке ресурсов.

  8.2. Best Practices

	Всегда проверяй ctx.Done() в цикле отправки.
	Обрабатывай ошибки при stream.Send().
	Используй дедлайн на клиенте.
	Для больших списков используй пагинацию (limit/offset) в запросе.
	Добавляй интерсепторы для логирования стримов.
	Закрывай стрим явно через возврат из метода или ошибку.
	На клиенте обрабатывай io.EOF как признак завершения.

  9.  КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1.  Server-Streaming RPC — «один запрос → поток ответов».
  2.  Определяется в .proto: rpc ListUsers(Request) returns (stream User).
  3.  Сервер отправляет сообщения через stream.Send() в цикле.
  4.  Клиент читает в цикле через stream.Recv() до io.EOF.
  5.  Контекст: stream.Context() на сервере, ctx на клиенте.
  6.  Проверяй ctx.Done() в цикле отправки (отмена/дедлайн).
  7.  Порядок сообщений гарантируется.
  8.  io.EOF — не ошибка, а признак завершения стрима.
  9.  Использовать для: большие списки, подписки, загрузка файлов.
  10. В отличие от Unary, не возвращает всю пачку, а по одному.
*/

//ХРАНИЛИЩЕ

type UserStore struct {
	mu    sync.RWMutex
	users map[string]*pb.User
}

func NewUserStore() *UserStore {
	return &UserStore{
		users: map[string]*pb.User{
			"1": {Id: "1", Name: "Alice", Email: "alice@ex.com", CreatedAt: timestamppb.Now()},
			"2": {Id: "2", Name: "Bob", Email: "bob@ex.com", CreatedAt: timestamppb.Now()},
			"3": {Id: "3", Name: "Charlie", Email: "charlie@ex.com", CreatedAt: timestamppb.Now()},
			"4": {Id: "4", Name: "Diana", Email: "diana@ex.com", CreatedAt: timestamppb.Now()},
			"5": {Id: "5", Name: "Eve", Email: "eve@ex.com", CreatedAt: timestamppb.Now()},
		},
	}
}

func (s *UserStore) ListAll() []*pb.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]*pb.User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	return users
}

func (s *UserStore) Get(id string) (*pb.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	return u, ok
}

//СЕРВЕР

type UserServer struct {
	pb.UnimplementedUserServiceServer
	store *UserStore
}

// ListUsers — server-streaming: отдаём пользователей по одному
func (s *UserServer) ListUsers(req *pb.ListUsersRequest, stream pb.UserService_ListUsersServer) error {
	ctx := stream.Context()

	// Проверяем, не истёк ли дедлайн
	if ctx.Err() == context.DeadlineExceeded {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	users := s.store.ListAll()
	limit := int(req.Limit)
	if limit <= 0 || limit > len(users) {
		limit = len(users)
	}

	for i := 0; i < limit; i++ {
		// Проверяем, не отменил ли клиент запрос
		select {
		case <-ctx.Done():
			return status.Error(codes.Canceled, "client cancelled")
		default:
			// Отправляем сообщение
			if err := stream.Send(users[i]); err != nil {
				return err
			}
			// Имитация задержки (чтобы показать поток)
			time.Sleep(300 * time.Millisecond)
		}
	}
	return nil
}

// SubscribeToUpdates — имитация подписки на события
func (s *UserServer) SubscribeToUpdates(req *pb.SubscribeToUpdatesRequest, stream pb.UserService_SubscribeToUpdatesServer) error {
	ctx := stream.Context()
	if ctx.Err() == context.DeadlineExceeded {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	// Проверяем, есть ли такой пользователь (если указан user_id)
	if req.UserId != "" {
		if _, ok := s.store.Get(req.UserId); !ok {
			return status.Error(codes.NotFound, "user not found")
		}
	}

	// Имитация отправки обновлений (раз в 2 секунды)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Имитация счётчика обновлений
	var counter int

	for {
		select {
		case <-ctx.Done():
			log.Println("Клиент отключился, закрываем стрим")
			return nil // или status.Error(codes.Canceled, "client cancelled")
		case <-ticker.C:
			counter++
			update := &pb.UserUpdate{
				User: &pb.User{
					Id:        "999",
					Name:      "Update #" + string(rune(counter+48)),
					Email:     "update@ex.com",
					CreatedAt: timestamppb.Now(),
				},
				Type:       pb.UserUpdate_UPDATE_TYPE_CREATED,
				OccurredAt: timestamppb.Now(),
			}
			if err := stream.Send(update); err != nil {
				log.Printf("Ошибка отправки: %v", err)
				return err
			}
			log.Printf("Отправлено обновление #%d", counter)
			if counter >= 5 { // имитация 5 обновлений, потом завершаем стрим
				log.Println("Завершаем стрим по достижении лимита")
				return nil
			}
		}
	}
}

//INTERCEPTOR ДЛЯ СТРИМОВ (ЛОГИРОВАНИЕ)

// StreamLoggingInterceptor логирует начало и конец стрима
func StreamLoggingInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	log.Printf("[STREAM] Начало: %s", info.FullMethod)
	err := handler(srv, ss)
	log.Printf("[STREAM] Конец: %s, ошибка: %v", info.FullMethod, err)
	return err
}

func main() {
	store := NewUserStore()
	server := &UserServer{store: store}

	// Создаём gRPC-сервер с интерсептором для стримов
	s := grpc.NewServer(
		grpc.StreamInterceptor(StreamLoggingInterceptor),
	)

	pb.RegisterUserServiceServer(s, server)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		log.Println("Сервер запущен на :50051")
		if err := s.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Остановка сервера...")
	s.GracefulStop()
	log.Println("Сервер остановлен")
}
