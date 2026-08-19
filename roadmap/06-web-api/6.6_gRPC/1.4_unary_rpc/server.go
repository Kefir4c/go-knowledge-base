package __4_unary_rpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/"
)

/*
  РАЗДЕЛ 1.4: UNARY RPC (ЗАПРОС-ОТВЕТ)

  Unary RPC — это самый простой и наиболее часто используемый тип RPC в gRPC.
  Он работает по принципу «один запрос — один ответ», аналогично обычному
  HTTP-запросу.

  В этом разделе мы разберём:
    1.  Что такое Unary RPC и когда он используется
    2.  Определение в .proto-файле
    3.  Реализация сервера (структура, метод, обработка ошибок)
    4.  Реализация клиента (подключение, вызов, обработка ошибок)
    5.  Важные нюансы: контекст, дедлайны, connection pooling
    6.  Полный рабочий пример (теория + код)
    7.  Частые ошибки и best practices

  1.  ЧТО ТАКОЕ UNARY RPC И КОГДА ОН ИСПОЛЬЗУЕТСЯ
  Unary RPC — это синхронный вызов, при котором:
    • Клиент отправляет ОДИН запрос.
    • Сервер обрабатывает запрос и возвращает ОДИН ответ.
    • Вызов блокируется до получения ответа или ошибки.

  Аналогия: это как обычный HTTP-запрос (GET/POST) — отправил запрос,
  получил ответ.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Стандартные CRUD-операции (Create, Read, Update, Delete).
    • Получение данных по ID.
    • Любой сценарий, где нужно отправить запрос и дождаться ответа.

  Преимущества:
    • Простота реализации и понимания.
    • Подходит для 80% случаев в микросервисной архитектуре.
    • Отлично сочетается с дедлайнами и интерсепторами.

  2.  ОПРЕДЕЛЕНИЕ В .PROTO-ФАЙЛЕ
  В .proto-файле Unary RPC описывается следующим образом:

    service UserService {
      rpc GetUser (GetUserRequest) returns (User) {}
    }

  Где:
    • GetUser — имя метода (должно быть в PascalCase).
    • GetUserRequest — тип сообщения для запроса.
    • User — тип сообщения для ответа.

  Полный пример .proto-файла:

    syntax = "proto3";
    package user.v1;
    option go_package = "github.com/example/api/user/v1;userv1";

    message GetUserRequest {
      string user_id = 1;
    }

    message User {
      string id = 1;
      string name = 2;
      string email = 3;
    }

    service UserService {
      rpc GetUser (GetUserRequest) returns (User) {}
    }

  После генерации кода через protoc появляются:
    • user.pb.go — структуры GetUserRequest и User.
    • user_grpc.pb.go — интерфейсы UserServiceServer и UserServiceClient.

  3.  РЕАЛИЗАЦИЯ СЕРВЕРА
  3.1. Структура сервера
    Сервер должен реализовывать интерфейс UserServiceServer.
    Обычно это делается через встраивание UnimplementedUserServiceServer
    для обратной совместимости:
      type server struct {
        pb.UnimplementedUserServiceServer
      }

  3.2. Реализация метода
    Метод должен иметь сигнатуру:
      func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error)
    Внутри метода происходит бизнес-логика:
      • Получение данных (из БД, кэша, внешнего API).
      • Обработка ошибок с возвратом статусов.
      • Формирование ответа.

    Пример:

      func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
          // Проверка входных данных
          if req.UserId == "" {
              return nil, status.Error(codes.InvalidArgument, "user_id is required")
          }

          // Проверка контекста (дедлайн)
          if ctx.Err() == context.DeadlineExceeded {
              return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
          }

          // Получение данных (имитация)
          user, err := s.db.GetUser(req.UserId)
          if err != nil {
              if errors.Is(err, ErrNotFound) {
                  return nil, status.Error(codes.NotFound, "user not found")
              }
              return nil, status.Error(codes.Internal, "internal error")
          }

          return &pb.User{
              Id:    user.ID,
              Name:  user.Name,
              Email: user.Email,
          }, nil
      }

  3.3. Обработка ошибок
    Используй статусы из пакета status и коды из codes.
    Основные коды для Unary RPC:
      • codes.InvalidArgument — неверный запрос (400).
      • codes.NotFound — ресурс не найден (404).
      • codes.AlreadyExists — ресурс уже существует (409).
      • codes.PermissionDenied — недостаточно прав (403).
      • codes.Unauthenticated — не авторизован (401).
      • codes.DeadlineExceeded — таймаут (504).
      • codes.Internal — внутренняя ошибка (500).
      • codes.Unavailable — сервис недоступен (503).

  3.4. Регистрация сервера
    В main():

      lis, _ := net.Listen("tcp", ":50051")
      s := grpc.NewServer()
      pb.RegisterUserServiceServer(s, &server{})
      s.Serve(lis)

  4.  РЕАЛИЗАЦИЯ КЛИЕНТА
  4.1. Подключение к серверу
    Используй grpc.NewClient или grpc.Dial для создания соединения.
    В современном gRPC (v1.50+) рекомендуется использовать NewClient:

      conn, err := grpc.NewClient("localhost:50051",
          grpc.WithTransportCredentials(insecure.NewCredentials()),
      )
      if err != nil {
          log.Fatal(err)
      }
      defer conn.Close()

    Для старых версий использовался grpc.DialWithInsecure,
    но сейчас это устаревшее.

  4.2. Создание клиента
      client := pb.NewUserServiceClient(conn)

  4.3. Вызов метода
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()

      resp, err := client.GetUser(ctx, &pb.GetUserRequest{UserId: "123"})
      if err != nil {
          // Обработка ошибки
          st, ok := status.FromError(err)
          if ok {
              switch st.Code() {
              case codes.NotFound:
                  log.Println("User not found")
              case codes.DeadlineExceeded:
                  log.Println("Request timed out")
              default:
                  log.Printf("Other error: %v", st.Message())
              }
          }
          return
      }

      log.Printf("User: %+v", resp)

  4.4. Обработка ошибок на клиенте
    Всегда обрабатывай ошибки, которые возвращает gRPC-вызов.
    Используй status.FromError для извлечения кода и сообщения.

  4.5. Connection Pooling
    Одно соединение можно использовать для всех горутин.
    Создавай conn один раз и переиспользуй.

      var (
          once sync.Once
          conn *grpc.ClientConn
      )

      func getConn() *grpc.ClientConn {
          once.Do(func() {
              var err error
              conn, err = grpc.NewClient("localhost:50051",
                  grpc.WithTransportCredentials(insecure.NewCredentials()),
              )
              if err != nil {
                  panic(err)
              }
          })
          return conn
      }

  5.  ВАЖНЫЕ НЮАНСЫ: КОНТЕКСТ, ДЕДЛАЙНЫ, CONNECTION POOLING
  5.1. Контекст и дедлайны
    Контекст передаётся в каждый RPC-вызов. Он используется для:
      • Установки дедлайнов (таймаутов).
      • Отмены запроса (если клиент закрыл соединение).
      • Передачи metadata (заголовков).
    Всегда устанавливай дедлайн на клиенте:
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
    На сервере проверяй ctx.Done() в долгих операциях.

  5.2. Блокирующий вызов
    Unary RPC блокирует выполнение до получения ответа или ошибки.
    Для асинхронного вызова используй горутины.

  5.3. Metadata
    Для передачи JWT-токенов, trace_id и т.п. используй metadata:
      md := metadata.Pairs("authorization", "Bearer token")
      ctx := metadata.NewOutgoingContext(context.Background(), md)
      resp, err := client.GetUser(ctx, req)

  5.4. Интерсепторы
    На клиенте и сервере можно добавлять интерсепторы:
      • Логирование.
      • Аутентификация.
      • Трассировка.

  6.  ПОЛНЫЙ РАБОЧИЙ ПРИМЕР (ТЕОРИЯ + КОД)
  Ниже приведён полный код Unary RPC-сервиса.

  6.1. .proto-файл (proto/user.proto)

    syntax = "proto3";
    package user.v1;
    option go_package = "github.com/example/api/user/v1;userv1";

    message GetUserRequest {
      string user_id = 1;
    }

    message User {
      string id = 1;
      string name = 2;
      string email = 3;
    }

    service UserService {
      rpc GetUser (GetUserRequest) returns (User) {}
    }

  6.2. Генерация кода

    protoc --proto_path=proto \
           --go_out=. --go_opt=paths=source_relative \
           --go-grpc_out=. --go-grpc_opt=paths=source_relative \
           proto/user.proto

  6.3. Сервер (server/main.go)

    package main

    import (
      "context"
      "log"
      "net"
      "sync"

      "google.golang.org/grpc"
      "google.golang.org/grpc/codes"
      "google.golang.org/grpc/status"
      pb "github.com/example/api/user/v1"
    )

    type server struct {
      pb.UnimplementedUserServiceServer
      mu    sync.RWMutex
      users map[string]*pb.User
    }

    func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
      if req.UserId == "" {
        return nil, status.Error(codes.InvalidArgument, "user_id is required")
      }

      if ctx.Err() == context.DeadlineExceeded {
        return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
      }

      s.mu.RLock()
      user, ok := s.users[req.UserId]
      s.mu.RUnlock()

      if !ok {
        return nil, status.Error(codes.NotFound, "user not found")
      }
      return user, nil
    }

    func main() {
      lis, err := net.Listen("tcp", ":50051")
      if err != nil {
        log.Fatal(err)
      }

      srv := grpc.NewServer()
      pb.RegisterUserServiceServer(srv, &server{
        users: map[string]*pb.User{
          "1": {Id: "1", Name: "Alice", Email: "alice@ex.com"},
          "2": {Id: "2", Name: "Bob", Email: "bob@ex.com"},
        },
      })

      log.Println("Server listening on :50051")
      if err := srv.Serve(lis); err != nil {
        log.Fatal(err)
      }
    }

  6.4. Клиент (client/main.go)

    package main

    import (
      "context"
      "log"
      "time"

      "google.golang.org/grpc"
      "google.golang.org/grpc/codes"
      "google.golang.org/grpc/credentials/insecure"
      "google.golang.org/grpc/status"
      pb "github.com/example/api/user/v1"
    )

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

      resp, err := client.GetUser(ctx, &pb.GetUserRequest{UserId: "1"})
      if err != nil {
        st, ok := status.FromError(err)
        if ok {
          switch st.Code() {
          case codes.NotFound:
            log.Println("User not found")
          case codes.DeadlineExceeded:
            log.Println("Request timed out")
          default:
            log.Printf("Error: %v", st.Message())
          }
        } else {
          log.Printf("Non-gRPC error: %v", err)
        }
        return
      }

      log.Printf("User: ID=%s, Name=%s, Email=%s", resp.Id, resp.Name, resp.Email)
    }

  6.5. Запуск
    # Терминал 1
    go run server/main.go
    # Терминал 2
    go run client/main.go
    # Вывод: User: ID=1, Name=Alice, Email=alice@ex.com

  7.  ЧАСТЫЕ ОШИБКИ И BEST PRACTICES
  7.1. Частые ошибки
	Не обрабатывать ошибки на клиенте.
	Не устанавливать дедлайн (запрос может висеть вечно).
	Создавать новое соединение на каждый запрос (дорого).
	Не проверять ctx.Err() на сервере в долгих операциях.
	Игнорировать nil-указатели в ответах.
	Не использовать интерсепторы для сквозной логики.

  7.2. Best Practices
	Всегда устанавливай дедлайн на клиенте.
	Переиспользуй соединение (connection pool).
	Обрабатывай ошибки с статусами.
	На сервере проверяй ctx.Done() в долгих операциях.
	Добавляй интерсепторы для логирования, auth, метрик.
	Для больших запросов используй стриминг, а не unary.
	Документируй ошибки, которые может вернуть метод.

  8.  КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1.  Unary RPC — это «один запрос → один ответ», синхронный вызов.
  2.  Определяется в .proto: rpc GetUser(Request) returns (Response).
  3.  Сервер реализует интерфейс UserServiceServer, метод принимает
      контекст и запрос, возвращает ответ и ошибку (статус).
  4.  Клиент создаёт соединение через grpc.NewClient, создаёт клиента
      через NewUserServiceClient и вызывает метод.
  5.  Всегда используй контекст с дедлайном на клиенте.
  6.  На сервере проверяй ctx.Err() для обработки дедлайнов.
  7.  Ошибки возвращаются через status.Error с кодами из codes.
  8.  На клиенте ошибки обрабатываются через status.FromError.
  9.  Соединение должно быть переиспользуемым (создаётся один раз).
  10. Interceptors добавляют сквозную логику.
*/

/*
ФАЙЛ: proto/user.pb.go (теоретически сгенерирован)
ФАЙЛ: proto/user_grpc.pb.go (теоретически сгенерирован)
*/

//ХРАНИЛИЩЕ (IN-MEMORY)

type UserStore struct {
	mu     sync.RWMutex
	users  map[string]*pb.User
	nextID int
}

func NewUserStore() *UserStore {
	return &UserStore{
		users:  make(map[string]*pb.User),
		nextID: 1,
	}
}

func (s *UserStore) Create(email, name string, age int32) (*pb.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Проверка на дубликат email
	for _, u := range s.users {
		if u.Email == email {
			return nil, errors.New("user already exists")
		}
	}

	id := fmt.Sprintf("%d", s.nextID)
	s.nextID++
	now := timestamppb.Now()
	user := &pb.User{
		Id:        id,
		Email:     email,
		Name:      name,
		Age:       age,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.users[id] = user
	return user, nil
}

func (s *UserStore) Get(id string) (*pb.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (s *UserStore) Update(user *pb.User, mask []string) (*pb.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.users[user.Id]
	if !ok {
		return nil, errors.New("user not found")
	}

	// Частичное обновление по маске
	if len(mask) == 0 {
		// Если маска пустая, обновляем все поля (кроме id, created_at)
		existing.Email = user.Email
		existing.Name = user.Name
		existing.Age = user.Age
		existing.IsActive = user.IsActive
	} else {
		for _, field := range mask {
			switch field {
			case "email":
				existing.Email = user.Email
			case "name":
				existing.Name = user.Name
			case "age":
				existing.Age = user.Age
			case "is_active":
				existing.IsActive = user.IsActive
			}
		}
	}
	existing.UpdateAt = timestamppb.Now()
	return existing, nil
}

// СЕРВЕР (РЕАЛИЗАЦИЯ gRPC)

type UserServer string {
	pb.UnimplementedUserServiceServer
	store *UserStore
}

// GetUser — Unary RPC
func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	// Проверка дедлайна
	if ctx.Err() == context.DeadlineExceeded {
		return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	user, err := s.store.Get(req.Id)
	if err != nil {
		if err.Error() == "user not found" {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return user, nil
}

// CreateUser — Unary RPC
func (s *UserServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	if ctx.Err() == context.DeadlineExceeded {
		return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Age < 18 {
		return nil, status.Error(codes.InvalidArgument, "age must be >= 18")
	}

	user, err := s.store.Create(req.Email, req.Name, req.Age)
	if err != nil {
		if err.Error() == "user already exists" {
			return nil, status.Error(codes.AlreadyExists, "user already exists")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.CreateUserResponse{User: user}, nil
}

// UpdateUser — Unary RPC с частичным обновлением
func (s *UserServer) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse,error){
	if ctx.Err() == context.DeadlineExceeded {
		return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	if req.User == nil || req.User.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "user with id is required")
	}

	// Извлекаем маску полей
	var mask []string
	if req.UpdateMask != nil{
		mask = req.UpdateMask.Paths
	}
	updated,err:= s.store.Update(req.User,mask)
	if err != nil {
		if err.Error() == "user not found" {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.UpdateUserResponse{User: updated}, nil
}

//INTERCEPTORS

// AuthInterceptor — проверяет JWT-токен в metadata
func AuthInterceptor() grpc.UnaryServerInterceptor{
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Извлекаем metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		auth := md.Get("authorization")
		if len(auth) == 0 || auth[0] != "Bearer secret-token" {
			return nil, status.Error(codes.Unauthenticated, "invalid or missing token")
		}

		// Добавляем userID в контекст (для примера)
		ctx = context.WithValue(ctx, "userID", "123")
		return handler(ctx, req)
	}
}

// LoggingInterceptor — логирует запросы и ответы
func LoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		log.Printf("[%s] Запрос: %+v", info.FullMethod, req)
		resp, err := handler(ctx, req)
		log.Printf("[%s] Ответ: %+v, Ошибка: %v, Время: %v", info.FullMethod, resp, err, time.Since(start))
		return resp, err
	}
}

// Вспомогательные обёртки для цепочки интерсепторов (без внешних зависимостей)
func wrapAuth(handler grpc.UnaryHandler) grpc.UnaryHandler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok || len(md.Get("authorization")) == 0 || md.Get("authorization")[0] != "Bearer secret-token" {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}
		return handler(ctx, req)
	}
}
func wrapLogging(handler grpc.UnaryHandler) grpc.UnaryHandler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		log.Printf("LOG: метод=%T, req=%+v, resp=%+v, err=%v, time=%v", req, req, resp, err, time.Since(start))
		return resp, err
	}
}


func main(){
	// Создаём хранилище с тестовыми данными
	store := NewUserStore()
	store.Create("alice@example.com", "Alice", 30)
	store.Create("bob@example.com", "Bob", 25)

	// Создаём gRPC-сервер с интерсепторами
	s:= grpc.NewServer(
		grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any,error) {
			// Цепочка: сначала аутентификация, потом логирование
			// (можно использовать grpc_middleware.ChainUnaryServer)
			handler = wrapAuth(handler)
			handler = wrapLogging(handler)
			return handler(ctx, req)
		}),)

	pb.RegisterUserServiceServer(s, &UserServer{store: store})

	// Запускаем слушатель
	list,err:= net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		log.Println("Server listening on :50051")
		if err:= s.Serve(list); err != nil{
			log.Fatal(err)
		}
	}()

	stop:= make(chan os.Signal,1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server...")
	s.GracefulStop()
	log.Println("Server stopped")
}
