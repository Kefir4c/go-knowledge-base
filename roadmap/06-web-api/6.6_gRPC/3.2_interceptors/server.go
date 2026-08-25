package __2_interceptors

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	_ "google.golang.org/grpc/grpclog"

	pb "github.com/"
)

/*
  РАЗДЕЛ 3.2: INTERCEPTORS
  Interceptors (перехватчики) — это мощнейший механизм gRPC, который позволяет
  добавлять сквозную функциональность (cross-cutting concerns) без изменения
  бизнес-логики. Это аналог middleware в HTTP-фреймворках.

  В этом разделе мы разберём:
    1.  Что такое Interceptors и зачем они нужны
    2.  Четыре типа Interceptors (клиент/сервер × unary/stream)
    3.  UnaryServerInterceptor — сигнатура, примеры
    4.  StreamServerInterceptor — сигнатура, примеры
    5.  Client-side Interceptors — для клиентов
    6.  Цепочки Interceptors (ChainUnaryServer, ChainStreamServer)
    7.  Типичные сценарии использования: логирование, аутентификация
    8.  Best practices и типичные ошибки
    9.  Ключевые выводы для собеседования

  1.  ЧТО ТАКОЕ INTERCEPTORS И ЗАЧЕМ ОНИ НУЖНЫ

  Interceptors — это функции, которые выполняются до или после обработки
  RPC-вызова. Они позволяют перехватывать запросы и ответы, добавлять
  логику, не затрагивая бизнес-код.

  gRPC предоставляет простые API для реализации и установки перехватчиков
  как на стороне клиента, так и на стороне сервера.

  ЗАЧЕМ ОНИ НУЖНЫ:
    • Логирование запросов и ответов.
    • Аутентификация и авторизация (JWT, API-ключи).
    • Сбор метрик (Prometheus, OpenTelemetry).
    • Трассировка распределённых запросов.
    • Валидация входных данных.
    • Retry (повторные попытки при ошибках).
    • Rate limiting (ограничение частоты запросов).
    • Восстановление после паник (recovery).

  Это идеальный способ реализовать общие паттерны, которые используются
  во всех микросервисах.

  2.  ЧЕТЫРЕ ТИПА INTERCEPTORS

  В gRPC существует ЧЕТЫРЕ типа перехватчиков:
  ┌────────────┬────────────────┬────────────────────────────────────────────┐
  │ Сторона    │ Тип RPC        │ Тип Interceptor                            │
  ├────────────┼────────────────┼────────────────────────────────────────────┤
  │ Сервер     │ Unary          │ grpc.UnaryServerInterceptor                │
  ├────────────┼────────────────┼────────────────────────────────────────────┤
  │ Сервер     │ Streaming      │ grpc.StreamServerInterceptor               │
  ├────────────┼────────────────┼────────────────────────────────────────────┤
  │ Клиент     │ Unary          │ grpc.UnaryClientInterceptor                │
  ├────────────┼────────────────┼────────────────────────────────────────────┤
  │ Клиент     │ Streaming      │ grpc.StreamClientInterceptor               │
  └────────────┴────────────────┴────────────────────────────────────────────┘

  Unary Interceptors обрабатывают одиночные RPC-вызовы (запрос-ответ).
  Stream Interceptors оборачивают стриминговые RPC, позволяя перехватывать
  каждый Send/Recv через обёртку.

  3.  UNARY SERVER INTERCEPTOR — ПОЛНЫЙ РАЗБОР

  3.1. Сигнатура
    type UnaryServerInterceptor func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (interface{}, error)

  3.2. Компоненты
    • ctx — контекст запроса (содержит metadata, дедлайны).
    • req — запрос (proto-сообщение от клиента).
    • info — информация о RPC-методе (имя, сервер).
    • handler — функция, которая вызывает следующий обработчик в цепочке.

  3.3. Структура перехватчика
    Любой unary interceptor состоит из трёх частей:

      1. PRE-PROCESSING (до вызова handler):
         - Логирование запроса.
         - Проверка токена.
         - Валидация данных.
         - Модификация контекста.

      2. INVOKE (вызов handler):
         - Вызов handler(ctx, req) для передачи управления дальше.

      3. POST-PROCESSING (после вызова handler):
         - Логирование ответа/ошибки.
         - Сбор метрик (время выполнения).
         - Обработка ошибок.

  3.4. Полный пример: логирующий интерсептор
    func LoggingInterceptor() grpc.UnaryServerInterceptor {
        return func(ctx context.Context, req interface{},
            info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

            // 1. PRE-PROCESSING
            log.Printf("[%s] Запрос: %+v", info.FullMethod, req)

            // 2. INVOKE
            start := time.Now()
            resp, err := handler(ctx, req)

            // 3. POST-PROCESSING
            log.Printf("[%s] Ответ: %+v, Ошибка: %v, Время: %v",
                info.FullMethod, resp, err, time.Since(start))

            return resp, err
        }
    }

  3.5. Полный пример: аутентификация (JWT)
    func AuthInterceptor() grpc.UnaryServerInterceptor {
        return func(ctx context.Context, req interface{},
            info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

            // 1. Извлекаем metadata
            md, ok := metadata.FromIncomingContext(ctx)
            if !ok {
                return nil, status.Error(codes.Unauthenticated, "no metadata")
            }

            // 2. Проверяем токен
            auth := md.Get("authorization")
            if len(auth) == 0 {
                return nil, status.Error(codes.Unauthenticated, "missing token")
            }

            parts := strings.SplitN(auth[0], " ", 2)
            if len(parts) != 2 || parts[0] != "Bearer" {
                return nil, status.Error(codes.Unauthenticated, "invalid auth format")
            }

            userID, err := validateJWT(parts[1])
            if err != nil {
                return nil, status.Error(codes.Unauthenticated, "invalid token")
            }

            // 3. Добавляем userID в контекст
            ctx = context.WithValue(ctx, "userID", userID)

            // 4. Вызываем следующий обработчик
            return handler(ctx, req)
        }
    }

  4.  STREAM SERVER INTERCEPTOR — ПОЛНЫЙ РАЗБОР

  4.1. Сигнатура
    type StreamServerInterceptor func(
        srv interface{},
        ss grpc.ServerStream,
        info *grpc.StreamServerInfo,
        handler grpc.StreamHandler,
    ) error

  4.2. Отличие от Unary Interceptor
    В отличие от unary, где мы просто вызываем handler и получаем ответ,
    в stream interceptor мы работаем с потоком. Вместо вызова handler
    и пост-обработки, мы перехватываем операции на стриме.

  4.3. Структура
    Для стримов нужно обернуть ServerStream и переопределить методы Send/Recv.

  4.4. Полный пример: логирующий стрим-интерсептор
    type wrappedStream struct {
        grpc.ServerStream
    }

    func (w *wrappedStream) SendMsg(m interface{}) error {
        log.Printf("Stream Send: %+v", m)
        return w.ServerStream.SendMsg(m)
    }

    func (w *wrappedStream) RecvMsg(m interface{}) error {
        log.Printf("Stream Recv: %+v", m)
        return w.ServerStream.RecvMsg(m)
    }

    func StreamLoggingInterceptor() grpc.StreamServerInterceptor {
        return func(srv interface{}, ss grpc.ServerStream,
            info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

            log.Printf("[STREAM] Начало: %s", info.FullMethod)
            err := handler(srv, &wrappedStream{ss})
            log.Printf("[STREAM] Конец: %s, ошибка: %v", info.FullMethod, err)
            return err
        }
    }

  5.  CLIENT-SIDE INTERCEPTORS

  Клиентские интерсепторы работают аналогично серверным, но на стороне
  клиента. Они позволяют перехватывать вызовы до отправки на сервер.

  5.1. UnaryClientInterceptor
    type UnaryClientInterceptor func(
        ctx context.Context,
        method string,
        req, reply interface{},
        cc *grpc.ClientConn,
        invoker grpc.UnaryInvoker,
        opts ...grpc.CallOption,
    ) error

  5.2. Пример: клиентский интерсептор для логирования
    func ClientLoggingInterceptor() grpc.UnaryClientInterceptor {
        return func(ctx context.Context, method string, req, reply interface{},
            cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

            log.Printf("[CLIENT] Вызов: %s, Запрос: %+v", method, req)
            start := time.Now()
            err := invoker(ctx, method, req, reply, cc, opts...)
            log.Printf("[CLIENT] Ответ: %+v, Ошибка: %v, Время: %v",
                reply, err, time.Since(start))
            return err
        }
    }

  5.3. Применение на клиенте
    conn, err := grpc.NewClient("localhost:50051",
        grpc.WithUnaryInterceptor(ClientLoggingInterceptor()),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )

  6.  ЦЕПОЧКИ INTERCEPTORS (CHAINING)

  По умолчанию gRPC не позволяет использовать более одного интерсептора
  одновременно. Для этого нужна цепочка.

  6.1. Пакет go-grpc-middleware
    import "github.com/grpc-ecosystem/go-grpc-middleware"

  Этот пакет предоставляет функции для объединения нескольких интерсепторов
  в один.

  6.2. ChainUnaryServer — для серверных unary
    s := grpc.NewServer(
        grpc.UnaryInterceptor(
            grpc_middleware.ChainUnaryServer(
                RecoveryInterceptor(),   // 1. Восстановление после паник
                LoggingInterceptor(),     // 2. Логирование
                AuthInterceptor(),        // 3. Аутентификация
            ),
        ),
    )

  6.3. ChainStreamServer — для серверных стримов
    s := grpc.NewServer(
        grpc.StreamInterceptor(
            grpc_middleware.ChainStreamServer(
                StreamLoggingInterceptor(),
                StreamAuthInterceptor(),
            ),
        ),
    )

  6.4. ChainUnaryClient — для клиентских unary
    conn, err := grpc.NewClient("localhost:50051",
        grpc.WithUnaryInterceptor(
            grpc_middleware.ChainUnaryClient(
                ClientLoggingInterceptor(),
                ClientRetryInterceptor(),
            ),
        ),
    )

  6.5. Порядок выполнения
    Интерсепторы выполняются в порядке, указанном в цепочке.
    В примере выше:
      1. RecoveryInterceptor (внешний)
      2. LoggingInterceptor
      3. AuthInterceptor (внутренний, ближе к бизнес-логике)

    ВАЖНО: порядок имеет значение. Например, трассировщик должен
    создавать span до того, как будут собираться метрики.

  7.  ТИПИЧНЫЕ СЦЕНАРИИ ИСПОЛЬЗОВАНИЯ

  7.1. Аутентификация и авторизация
    • Проверка JWT из metadata.
    • Проверка прав доступа к методу.
    • Добавление userID/роли в контекст.

  7.2. Логирование
    • Логирование запросов, ответов, ошибок.
    • Добавление request_id/trace_id в логи.
    • Логирование времени выполнения.

  7.3. Метрики (Prometheus)
    • Сбор количества запросов.
    • Время выполнения.
    • Коды ошибок.

  7.4. Трассировка (OpenTelemetry)
    • Создание span'ов для каждого RPC.
    • Пропагация trace_id через metadata.

  7.5. Recovery (восстановление после паник)
    • Перехват паник в обработчиках.
    • Возврат status.Error(codes.Internal).

  7.6. Retry (повторные попытки)
    • Автоматический повтор при временных ошибках.

  7.7. Rate limiting
    • Ограничение количества запросов от одного клиента.

  8.  BEST PRACTICES И ТИПИЧНЫЕ ОШИБКИ

  8.1. Best Practices
	Используй интерсепторы для сквозной функциональности (logging, auth).
	Не смешивай бизнес-логику с интерсепторами.
	Всегда передавай контекст дальше (не создавай новый).
	Используй go-grpc-middleware для цепочек.
	Для стримов оборачивай ServerStream, не пытайся перехватывать иначе.
	Порядок интерсепторов имеет значение — располагай их логически
       (recovery → logging → auth → metrics)
	Логируй ошибки с достаточным контекстом (метод, время, request_id).

  8.2. Типичные ошибки
	Создавать новый контекст вместо использования входящего. Потеря metadata, дедлайнов.
	Не вызывать handler в unary interceptor. Запрос зависнет.
	Не обрабатывать паники в recovery interceptor. Сервер упадёт.
	Возвращать nil-ответ без обработки ошибок. Клиент получит неожиданный nil.
	Использовать интерсепторы для бизнес-логики. Нарушение разделения ответственности.

  9.  КЛЮЧЕВЫЕ ВЫВОДЫ

  1.  Interceptors — middleware для gRPC, позволяют добавлять сквозную
      логику без изменения бизнес-кода.
  2.  Существует 4 типа: серверный/клиентский × unary/stream.
  3.  UnaryServerInterceptor:
      - Сигнатура: func(ctx, req, info, handler) (resp, error).
      - Три части: pre-processing → invoke → post-processing.
  4.  StreamServerInterceptor:
      - Сигнатура: func(srv, ss, info, handler) error.
      - Требует обёртки ServerStream для перехвата Send/Recv.
  5.  Цепочки: grpc_middleware.ChainUnaryServer() и ChainStreamServer().
  6.  Client-side интерсепторы аналогичны серверным.
  7.  Типичные use cases: logging, auth, metrics, recovery, retry, rate limiting.
  8.  Порядок интерсепторов важен — сначала recovery, потом auth, потом logging.
  9.  Для стримов нужно оборачивать ServerStream.
  10. Используй go-grpc-middleware для готовых интерсепторов.
*/

// ХРАНИЛИЩЕ
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
		},
	}
}

func (s *UserStore) Get(id string) (*pb.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return user, nil
}

func (s *UserStore) List(limit int32) []*pb.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*pb.User
	for _, user := range s.users {
		result = append(result, user)
		if limit > 0 && len(result) >= int(limit) {
			break
		}
	}
	return result
}

//JWT-ФУНКЦИИ (УПРОЩЁННЫЕ, ДЛЯ ДЕМОНСТРАЦИИ)

const jwtSecret = "my-secret-jojo-mne-palky-b-popy"

type Claims struct {
	UserID string
}

func generateJWT(userID string) string {
	// В реальности используй библиотеку github.com/golang-jwt/jwt/v5
	payload := fmt.Sprintf(`{"user_id":"%s","exp":%d}`, userID, time.Now().Add(time.Hour).Unix())
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

func validateJWT(token string) (string, error) {
	if token == "" {
		return "", errors.New("empty token")
	}
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", errors.New("invalid token format")
	}
	// Простой парсинг
	parts := strings.Split(string(decoded), ",")
	for _, part := range parts {
		if strings.Contains(part, "user_id") {
			userID := strings.TrimPrefix(strings.TrimSpace(parts), `"user_id":"`)
			userID = strings.TrimSuffix(userID, `"`)
			if userID != "" {
				return userID, nil
			}
		}
	}
	return "", errors.New("invalid claims")
}

// СЕРВЕР (БИЗНЕС-ЛОГИКА)
type UserServer struct {
	pb.UnimplementedUserServiceServer
	store *UserStore
}

func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	// userID добавлен в контекст интерсептором аутентификации
	userID, ok := ctx.Value("userID").(string)
	if ok {
		log.Printf("🔐 Запрос от пользователя: %s", userID)
	}
	user, err := s.store.Get(req.UserId)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return user, nil
}

func (s *UserServer) ListUser(req *pb.ListUsersRequest, stream pb.UserService_ListUsersServer) error {
	ctx := stream.Context()
	if ctx.Err() == context.DeadlineExceeded {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	users := s.store.List(req.limit)
	for _, u := range users {
		select {
		case <-ctx.Done():
			return status.Error(codes.Canceled, "client cancelled")
		default:
			if err := stream.Send(u); err != nil {
				return err
			}
			time.Sleep(200 * time.Millisecond) // имитация
		}
	}
	return nil
}

// INTERCEPTOR
// 1. RecoveryInterceptor — защита от паник
func RecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC: %v, метод: %s", r, info.FullMethod)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// 2. AuthInterceptor — JWT-аутентификация
func AuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "no metadata")
		}
		auth := md.Get("authorization")
		if len(auth) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}
		parts := strings.SplitN(auth[0], " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return nil, status.Error(codes.Unauthenticated, "invalid auth format")
		}
		userID, err := validateJWT(parts[1])
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token: "+err.Error())
		}
		ctx = context.WithValue(ctx, "userID", userID)
		return handler(ctx, req)
	}
}

// 3. LoggingInterceptor — логирование запросов и ответов
func LoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		requestID := md.Get("x-request-id")
		reqID := "unknown"
		if len(requestID) > 0 {
			reqID = requestID[0]
		}
		start := time.Now()
		log.Printf("[%s] [%s] Запрос: %+v", reqID, info.FullMethod, req)
		resp, err := handler(ctx, req)
		log.Printf("[%s] [%s] Ответ: %+v, Ошибка: %v, Время: %v",
			reqID, info.FullMethod, resp, err, time.Since(start))
		return resp, err
	}
}

// 4. MetricsInterceptor — сбор времени выполнения (упрощённо)
func MetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)
		// В реальности здесь отправка в Prometheus
		log.Printf("[%s] Duration: %v", info.FullMethod, duration)
		return resp, err
	}
}

// Stream interceptor — логирование стримов
type wrappedStream struct {
	grpc.ServerStream
}

func (w *wrappedStream) SendMsg(m interface{}) error {
	log.Printf("Stream Send: %+v", m)
	return w.ServerStream.SendMsg(m)
}

func (w *wrappedStream) RecvMsg(m interface{}) error {
	log.Printf("Stream Recv: %+v", m)
	return w.ServerStream.RecvMsg(m)
}

func StreamLoggingInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		log.Printf("[STREAM] Начало: %s", info.FullMethod)
		err := handler(srv, &wrappedStream{ss})
		log.Printf("[STREAM] Конец: %s, ошибка: %v", info.FullMethod, err)
		return err
	}
}

// MAIN
func main() {
	store := NewUserStore()

	// Создаём gRPC-сервер с цепочкой интерсепторов
	serv := grpc.NewServer(
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				RecoveryInterceptor(),
				LoggingInterceptor(),
				AuthInterceptor(),
				MetricsInterceptor(),
			)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(StreamLoggingInterceptor())),
	)
	pb.RegisterUserServiceServer(serv, &UserServer{store: store})

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Сервер запущен на :50051")
	log.Println("Требуется JWT в metadata (authorization: Bearer <token>)")

	go func() {
		if err := serv.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Остановка сервера...")
	serv.GracefulStop()
	log.Println("Сервер остановлен")
}
