package __1_metadata

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
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

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	// Замени путь на свой
	pb "github.com/example/metadata-example/proto/user"
)

/*
  РАЗДЕЛ 3.1: METADATA (ЗАГОЛОВКИ)
  Metadata — это механизм передачи произвольной key-value информации
  в gRPC-вызовах. Это аналог HTTP-заголовков (headers) и трейлеров (trailers).

  СОДЕРЖАНИЕ:
    1.  Что такое metadata и зачем она нужна
    2.  Структура metadata: ключи и значения
    3.  Как передавать metadata на клиенте
    4.  Как читать metadata на сервере
    5.  Как сервер может отправлять metadata обратно (headers и trailers)
    6.  Практический пример: передача JWT-токена через authorization
    7.  Важные нюансы: бинарные значения, регистронезависимость, резервированные ключи
    8.  NewOutgoingContext vs AppendToOutgoingContext
    9.  Пропагация metadata через цепочку микросервисов
    10. Metadata в стриминговых RPC
    11. Ограничения и подводные камни
    12. Best practices и типичные ошибки
    13. Ключевые выводы для собеседования

  1.  ЧТО ТАКОЕ METADATA И ЗАЧЕМ ОНА НУЖНА
  Metadata — это key-value пары, которые передаются вместе с gRPC-запросом
  и ответом. Они используются для:
    • Аутентификации и авторизации (JWT-токены, API-ключи).
    • Трассировки (trace_id, request_id, span_id).
    • Управления нагрузкой (балансировка, rate limiting).
    • Передачи контекстной информации (язык, версия клиента).
    • Логирования и мониторинга (корреляционные ID).

  Metadata — это НЕ часть данных сообщения. Она передаётся вне тела запроса,
  аналогично HTTP-заголовкам.

  2.  СТРУКТУРА METADATA: КЛЮЧИ И ЗНАЧЕНИЯ
  В Go metadata представлена типом `metadata.MD`, который является
  `map[string][]string`. То есть ключ → список строк (значений).

  КЛЮЧИ:
    • Должны быть в нижнем регистре (gRPC автоматически приводит их к lowercase).
    • Не должны начинаться с префикса "grpc-", т.к. он зарезервирован.
    • Для бинарных значений ключ должен заканчиваться на "-bin".

  ЗНАЧЕНИЯ:
    • Могут быть строками (ASCII).
    • Могут быть бинарными (ключ с суффиксом "-bin").
    • Одно значение может быть представлено как слайс строк.

  ПРИМЕР СОЗДАНИЯ:
    md := metadata.Pairs(
      "authorization", "Bearer my-jwt-token",
      "x-request-id", "abc-123",
      "x-binary-bin", string([]byte{0x01, 0x02}), // бинарное значение
    )

  3.  КАК ПЕРЕДАВАТЬ METADATA НА КЛИЕНТЕ

  3.1. Создание metadata
    // Способ 1: через metadata.Pairs
    md := metadata.Pairs(
      "authorization", "Bearer my-token",
      "x-request-id", "12345",
    )

    // Способ 2: через map
    md := metadata.New(map[string]string{
      "authorization": "Bearer my-token",
      "x-request-id":  "12345",
    })

    // Способ 3: добавление значений по ключу
    md := metadata.MD{}
    md.Set("authorization", "Bearer my-token")
    md.Append("x-request-id", "12345")

  3.2. Отправка metadata в контексте
    // В gRPC metadata передаётся через контекст.
    ctx := metadata.NewOutgoingContext(context.Background(), md)

    // После этого все gRPC-вызовы с этим контекстом будут включать metadata.
    resp, err := client.GetUser(ctx, req)

  3.3. Добавление metadata в существующий контекст
    // Если нужно добавить к уже существующему контексту:
    ctx := context.Background()
    ctx = metadata.AppendToOutgoingContext(ctx,
      "authorization", "Bearer new-token",
    )

  3.4. Отправка бинарных значений
    // Для бинарных данных ключ должен заканчиваться на "-bin"
    md := metadata.Pairs(
      "binary-data-bin", base64.StdEncoding.EncodeToString(data),
    )

  Важно: бинарные значения передаются в base64-кодированном виде.

  4.  КАК ЧИТАТЬ METADATA НА СЕРВЕРЕ

  4.1. Получение из контекста
    // В любом gRPC-методе можно получить metadata из контекста
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
      // metadata отсутствует
      return nil, status.Error(codes.Unauthenticated, "no metadata")
    }

  4.2. Чтение конкретного ключа
    // Получить все значения для ключа (слайс строк)
    values := md.Get("authorization")
    if len(values) == 0 {
      return nil, status.Error(codes.Unauthenticated, "no token")
    }
    token := values[0] // "Bearer my-jwt-token"

    // Или через методы MD
    auth := md["authorization"] // то же самое

  4.3. Проверка наличия ключа
    if _, ok := md["authorization"]; ok {
      // ключ существует
    }

  4.4. Чтение бинарных значений
    binValues := md.Get("binary-data-bin")
    if len(binValues) > 0 {
      // значение уже декодировано из base64
      data, _ := base64.StdEncoding.DecodeString(binValues[0])
    }

  5.  КАК СЕРВЕР МОЖЕТ ОТПРАВЛЯТЬ METADATA ОБРАТНО
  Сервер может отправлять metadata в двух формах:
    • HEADERS — отправляются в начале ответа (до основного сообщения).
    • TRAILERS — отправляются в конце ответа (после основного сообщения).

  5.1. Отправка headers
    // В начале обработки запроса
    header := metadata.Pairs("x-server-version", "1.0.0")
    if err := grpc.SetHeader(ctx, header); err != nil {
      // обработка ошибки
    }

  5.2. Отправка trailers
    // В конце обработки (можно использовать defer)
    defer func() {
      trailer := metadata.Pairs("x-processing-time", "42ms")
      grpc.SetTrailer(ctx, trailer)
    }()

    // ... бизнес-логика ...

  5.3. Получение headers и trailers на клиенте
    var header, trailer metadata.MD

    resp, err := client.GetUser(
      ctx,
      req,
      grpc.Header(&header),   // захватываем headers
      grpc.Trailer(&trailer), // захватываем trailers
    )

    if err == nil {
      version := header.Get("x-server-version")
      processingTime := trailer.Get("x-processing-time")
    }

  6.  ПРАКТИЧЕСКИЙ ПРИМЕР: ПЕРЕДАЧА JWT-ТОКЕНА ЧЕРЕЗ AUTHORIZATION

  6.1. Клиент отправляет токен
    // Создаём metadata с JWT-токеном
    md := metadata.Pairs(
      "authorization", "Bearer "+jwtToken,
    )
    ctx := metadata.NewOutgoingContext(context.Background(), md)

    // Вызываем защищённый метод
    resp, err := client.GetProtectedResource(ctx, req)

  6.2. Сервер читает и проверяет токен
    func (s *server) GetProtectedResource(ctx context.Context, req *pb.Request) (*pb.Response, error) {
      // Извлекаем metadata
      md, ok := metadata.FromIncomingContext(ctx)
      if !ok {
        return nil, status.Error(codes.Unauthenticated, "no metadata")
      }

      // Получаем заголовок authorization
      auth := md.Get("authorization")
      if len(auth) == 0 {
        return nil, status.Error(codes.Unauthenticated, "missing auth token")
      }

      // Парсим "Bearer <token>"
      parts := strings.SplitN(auth[0], " ", 2)
      if len(parts) != 2 || parts[0] != "Bearer" {
        return nil, status.Error(codes.Unauthenticated, "invalid auth format")
      }
      token := parts[1]

      // Валидируем JWT
      claims, err := validateJWT(token)
      if err != nil {
        return nil, status.Error(codes.Unauthenticated, "invalid token")
      }

      // Кладём userID в контекст для обработчиков
      ctx = context.WithValue(ctx, "userID", claims.UserID)

      // Вызываем бизнес-логику
      return s.businessLogic(ctx, req)
    }

  6.3. Полный цикл с интерсептором (рекомендованный подход)
    // Вместо проверки в каждом методе используем интерсептор
    func AuthInterceptor() grpc.UnaryServerInterceptor {
      return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        md, ok := metadata.FromIncomingContext(ctx)
        if !ok {
          return nil, status.Error(codes.Unauthenticated, "no metadata")
        }

        auth := md.Get("authorization")
        if len(auth) == 0 {
          return nil, status.Error(codes.Unauthenticated, "missing auth token")
        }

        parts := strings.SplitN(auth[0], " ", 2)
        if len(parts) != 2 || parts[0] != "Bearer" {
          return nil, status.Error(codes.Unauthenticated, "invalid auth format")
        }

        claims, err := validateJWT(parts[1])
        if err != nil {
          return nil, status.Error(codes.Unauthenticated, "invalid token")
        }

        ctx = context.WithValue(ctx, "userID", claims.UserID)
        return handler(ctx, req)
      }
    }

    // Регистрация интерсептора при создании сервера
    s := grpc.NewServer(
      grpc.UnaryInterceptor(AuthInterceptor()),
    )

  7.  ВАЖНЫЕ НЮАНСЫ: БИНАРНЫЕ ЗНАЧЕНИЯ, РЕГИСТРОНЕЗАВИСИМОСТЬ, ЗАРЕЗЕРВИРОВАННЫЕ КЛЮЧИ

  7.1. Регистронезависимость
    • Ключи metadata регистронезависимы. "Authorization" и "authorization"
      будут обрабатываться одинаково.
    • При получении ключи автоматически приводятся к нижнему регистру.

  7.2. Зарезервированные префиксы
    • Ключи, начинающиеся с "grpc-", зарезервированы для внутренних нужд gRPC.
    • Использование таких ключей может привести к ошибкам.
    • Кастомные ключи обычно имеют префикс "x-" (по аналогии с HTTP).

  7.3. Бинарные значения
    • Для бинарных данных ключ должен заканчиваться на "-bin".
    • Значение автоматически кодируется/декодируется в base64.
    • Это необходимо, потому что HTTP/2 не поддерживает бинарные данные в заголовках.

  8.  NEWOUTGOINGCONTEXT VS APPENDTOOUTGOINGCONTEXT
  Это частый вопрос на собеседовании.

  8.1. metadata.NewOutgoingContext
    // Создаёт НОВЫЙ контекст с metadata.
    // Если в контексте уже была metadata, она перезаписывается.
    md := metadata.Pairs("authorization", "Bearer token")
    ctx := metadata.NewOutgoingContext(context.Background(), md)

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Когда нужно полностью заменить существующие metadata.
    • Когда контекст новый, а не получен из запроса.

  8.2. metadata.AppendToOutgoingContext
    // ДОБАВЛЯЕТ ключи-значения к уже существующей metadata.
    // Если ключ уже существует, значение будет добавлено в список.
    ctx := metadata.AppendToOutgoingContext(ctx,
      "authorization", "Bearer new-token",
      "x-request-id", "456",
    )

    // Можно вызывать несколько раз
    ctx = metadata.AppendToOutgoingContext(ctx, "x-trace-id", "trace-789")

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Когда нужно дополнить существующие metadata.
    • Когда контекст пришёл от клиента (например, в интерсепторе)
      и нужно добавить свои значения.

  8.3. Ключевое отличие
    NewOutgoingContext — ЗАМЕНЯЕТ всё.
    AppendToOutgoingContext — ДОБАВЛЯЕТ к существующему.

  9.  ПРОПАГАЦИЯ METADATA ЧЕРЕЗ ЦЕПОЧКУ МИКРОСЕРВИСОВ
  В микросервисной архитектуре нужно передавать контекстную информацию
  (trace_id, user_id, токен) через цепочку вызовов.

  9.1. Клиент → Сервис A → Сервис B
    ┌─────────┐    ┌─────────┐    ┌─────────┐
    │ Клиент  │───▶│Сервис A │───▶│Сервис B │
    └─────────┘    └─────────┘    └─────────┘
       │               │               │
       │ metadata      │ metadata      │ metadata
       │ (токен)       │ (токен)       │ (токен)
       │               │ + trace_id    │ + trace_id
       │               │ + user_id     │ + user_id

  9.2. Как правильно пропагировать
    // В Сервисе A, который вызывает Сервис B
    func (s *serviceA) Handle(ctx context.Context, req *Request) (*Response, error) {
        // Получаем входящие metadata
        md, ok := metadata.FromIncomingContext(ctx)
        if !ok {
            md = metadata.New(nil)
        }

        // Добавляем свои значения
        md.Set("x-trace-id", generateTraceID())
        md.Set("x-user-id", "user-123")

        // Создаём новый контекст для исходящего вызова
        outCtx := metadata.NewOutgoingContext(ctx, md)

        // Вызываем Сервис B
        resp, err := s.clientB.Call(outCtx, reqB)
        // ...
    }

  9.3. Client-интерсептор для автоматической пропагации
    func ClientMetadataInterceptor() grpc.UnaryClientInterceptor {
        return func(ctx context.Context, method string, req, reply interface{},
            cc *grpc.ClientConn, opts ...grpc.CallOption) error {

            // Добавляем metadata перед каждым вызовом
            ctx = metadata.AppendToOutgoingContext(ctx,
                "x-request-id", getRequestID(),
            )
            return invoker(ctx, method, req, reply, cc, opts...)
        }
    }

  10. METADATA В СТРИМИНГОВЫХ RPC

  10.1. Unary vs Streaming
    • В Unary RPC metadata доступна через контекст сразу.
    • В Streaming RPC metadata доступна через контекст стрима:
        ctx := stream.Context()

  10.2. Отправка headers в стриминге
    // Сервер может отправить header в начале стрима
    func (s *server) Chat(stream pb.ChatService_ChatServer) error {
        // Отправляем header до первого сообщения
        if err := stream.SendHeader(metadata.Pairs("x-stream-id", "123")); err != nil {
            return err
        }

        // ... обработка стрима

        // В конце можно отправить trailers
        stream.SetTrailer(metadata.Pairs("x-processing-time", "42ms"))
        return nil
    }

  10.3. Получение headers на клиенте (стриминг)
    // Клиент может получить header после вызова стрима
    var header metadata.MD
    stream, err := client.Chat(ctx, grpc.Header(&header))
    if err != nil {
        return err
    }

    // Читаем header (они доступны после первого сообщения)
    if values, ok := header["x-stream-id"]; ok {
        log.Printf("Stream ID: %v", values)
    }

  10.4. Отправка metadata в стриминге от клиента
    // В стриминге клиент может отправить metadata только в первом сообщении,
    // либо через контекст (как в Unary).
    // Но можно отправить metadata вместе с первым сообщением (вложить в message),
    // либо использовать отдельный канал.

  11. ОГРАНИЧЕНИЯ И ПОДВОДНЫЕ КАМНИ

  11.1. Максимальный размер
    • gRPC не документирует явного ограничения на размер metadata.
    • Но HTTP/2 имеет ограничение на размер заголовков (обычно 8KB).
    • Если metadata слишком большая, сервер может вернуть ошибку
      "grpc: received message larger than max".

  Рекомендация: не передавать большие данные через metadata (>4KB).

  11.2. Ключи с префиксом "grpc-"
    • Ключи, начинающиеся с "grpc-", зарезервированы для gRPC.
    • Использование таких ключей может привести к ошибкам или
      непредсказуемому поведению.
    • Некоторые ключи (grpc-status, grpc-message) используются для ошибок.

  11.3. Регистронезависимость
    • Ключи приводятся к нижнему регистру.
    • "Authorization" и "authorization" — одно и то же.
    • Это важно помнить при сравнении ключей.

  11.4. Некорректное использование AppendToOutgoingContext
    //ПЛОХО: перезаписываем контекст без сохранения существующего
    ctx = metadata.AppendToOutgoingContext(ctx, "x-trace", "123")

    //ХОРОШО: сначала извлекаем, потом добавляем
    md, _ := metadata.FromOutgoingContext(ctx)
    ctx = metadata.NewOutgoingContext(ctx, md)
    ctx = metadata.AppendToOutgoingContext(ctx, "x-trace", "123")

  11.5. Бинарные значения
    • Бинарные ключи ДОЛЖНЫ заканчиваться на "-bin".
    • Иначе gRPC не закодирует их в base64.
    • При декодировании нужно использовать base64.StdEncoding.

  12. BEST PRACTICES И ТИПИЧНЫЕ ОШИБКИ

  12.1. Best Practices
     Используй интерсепторы для обработки metadata (JWT, логирование).
     Всегда проверяй наличие ключа перед чтением.
     Для обязательной metadata возвращай понятный статус (Unauthenticated).
     Передавай trace_id и request_id через metadata для трассировки.
     Используй константы для имён ключей.
     Для JWT используй заголовок "authorization" с "Bearer <token>".
     В стриминге отправляй headers через SendHeader().
     Для пропагации через цепочку микросервисов используй интерсепторы.

  12.2. Типичные ошибки
	* Парсить "Bearer" без проверки на " ". В строке может не быть пробела.
	* Класть чувствительные данные в metadata без шифрования (если без TLS). Используй TLS в продакшене.
	* Использовать зарезервированные префиксы ("grpc-"). Может сломать поведение gRPC.
	* Не проверять срок действия токена. Всегда проверяй exp в JWT.
	* Создавать новый контекст без копирования metadata. Используй metadata.NewOutgoingContext или AppendToOutgoingContext.
	* Передавать большие данные через metadata (>4KB). Используй message для больших данных.

  13. КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1.  Metadata — key-value пары, передаваемые вне тела сообщения (аналог HTTP-headers).
  2.  Используется для JWT, трассировки, rate limiting и т.д.
  3.  Клиент передаёт через metadata.NewOutgoingContext(ctx, md).
  4.  Сервер читает через metadata.FromIncomingContext(ctx).
  5.  Сервер может отправлять headers (в начале) и trailers (в конце).
  6.  Ключи приводятся к нижнему регистру, бинарные значения — с суффиксом "-bin".
  7.  Ключи с префиксом "grpc-" зарезервированы.
  8.  Для авторизации используй заголовок "authorization" с "Bearer <token>".
  9.  Обработку metadata лучше выносить в интерсепторы.
  10. Всегда проверяй существование ключа и валидность токена.
  11. NewOutgoingContext — ЗАМЕНЯЕТ, AppendToOutgoingContext — ДОБАВЛЯЕТ.
  12. В стриминге metadata доступна через stream.Context().
  13. При пропагации через цепочку сервисов копируй metadata из входящего контекста.
  14. Размер metadata должен быть небольшим (до 4-8KB).
*/

// В продакшене секреты — из переменных окружения
const jwtSecret = "my-prod-secret-key"

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
		},
	}
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

//JWT-ФУНКЦИИ
// В реальном проекте используй библиотеку "github.com/golang-jwt/jwt/v5"
// Здесь мы имитируем JWT для демонстрации.

// JWTClaims — структура утверждений
type JWTClaims struct {
	UserID string
	Exp    int64
}

// generateJWT — создаёт JWT-токен (имитация)
func generateJWT(userID string) (string, error) {
	// В реальности — подпись с секретом
	payload := fmt.Sprintf(`{"user_id":"%s","exp":%d}`, userID, time.Now().Add(time.Hour).Unix())

	token := base64.StdEncoding.EncodeToString([]byte(payload))
	return token, nil
}

// validateJWT — проверяет JWT-токен и возвращает userID
func validateJWT(token string) (string, error) {
	if token == "" {
		return "", errors.New("empty token")
	}
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", errors.New("invalid token format")
	}
	// Парсим user_id из JSON-подобной строки (упрощённо)
	parts := strings.Split(string(decoded), ",")
	for _, p := range parts {
		if strings.Contains(p, "user_id") {
			userID := strings.TrimPrefix(strings.TrimSpace(p), `"user_id":"`)
			userID = strings.TrimSuffix(userID, `"`)
			if userID != "" {
				return userID, nil
			}
		}
	}
	return "", errors.New("invalid token claims")
}

//СЕРВЕР

type UserServer struct {
	pb.UnimplementedUserServiceServer
	store *UserStore
}

// GetUser — unary RPC с использованием metadata
func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	// Проверяем дедлайн
	if ctx.Err() == context.DeadlineExceeded {
		return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	// 1. ИЗВЛЕКАЕМ USERID ИЗ КОНТЕКСТА (добавлен интерсептором)
	userID, ok := ctx.Value("userID").(string)
	if !ok || userID == "" {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}
	log.Printf("Аутентифицирован пользователь: %s", userID)

	// 2. ВАЛИДАЦИЯ ВХОДНЫХ ДАННЫХ
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// 3. ПОЛУЧАЕМ ДАННЫЕ
	user, err := s.store.Get(req.UserId)
	if err != nil {
		if err.Error() == "user not found" {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "internal error")
	}

	// 4. ОТПРАВЛЯЕМ HEADERS (в начале ответа)
	header := metadata.Pairs(
		"x-server-version", "1.2.3",
		"x-request-id", generateRequestID())
	if err := grpc.SetHeader(ctx, header); err != nil {
		log.Printf("Ошибка отправки headers: %v", err)
	}

	// 5. ОТПРАВЛЯЕМ TRAILERS
	defer func() {
		trailer := metadata.Pairs(
			"x-processing-time", fmt.Sprintf("%d", time.Since(time.Now()).Milliseconds()),
			"x-user-role", "user")
		if err := grpc.SetTrailer(ctx, trailer); err != nil {
			log.Printf("Ошибка отправки trailers: %v", err)
		}
	}()
	log.Printf("Пользователь %s запросил данные о %s", userID, req.UserId)
	return user, nil
}

// generateRequestID — генерирует уникальный ID для трейсинга
func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// INTERCEPTORS
func AuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 1. ИЗВЛЕКАЕМ METADATA
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "metadata not found")
		}

		// 2. ЛОГИРУЕМ ВСЕ METADATA (для отладки)
		log.Printf("Входящие metadata: %+v", md)

		// 3. ПРОВЕРЯЕМ AUTHORIZATION
		auth := md.Get("authorization")
		if len(auth) == 0 {
			return nil, status.Error(codes.Unauthenticated, "authorization header missing")
		}

		// 4. ПАРСИМ "Bearer <token>"
		parts := strings.SplitN(auth[0], " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization format (expected Bearer <token>)")
		}
		token := parts[1]

		// 5. ВАЛИДИРУЕМ JWT
		userID, err := validateJWT(token)
		if err != nil {
			log.Printf("Ошибка валидации JWT: %v", err)
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		// 6. ДОБАВЛЯЕМ USERID В КОНТЕКСТ
		ctx := context.WithValue(ctx, "userID", userID)
		log.Printf("JWT валиден, userID: %s", userID)

		// 7. ВЫЗЫВАЕМ СЛЕДУЮЩИЙ ОБРАБОТЧИК
		return handler(ctx, req)
	}
}

// LoggingInterceptor — логирует запросы и ответы
func LoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		log.Printf("[%s] Запрос: %+v", info.FullMethod, req)
		resp, err := handler(ctx, req)
		log.Printf("[%s] Ответ: %+v, Ошибка: %v, Время: %v", info.FullMethod, resp, err, time.Since(start))
		return resp, err
	}
}

func main() {
	// Создаём хранилище
	store := NewUserStore()

	// Создаём gRPC-сервер с интерсепторами (порядок важен!)
	s := grpc.NewServer(
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				AuthInterceptor(),
				LoggingInterceptor(),
			),
		),
	)

	pb.RegisterUserServiceServer(s, &UserServer{store: store})

	// Запускаем слушатель
	list, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Сервер запущен на :50051")
	log.Println("Требуется JWT в metadata (authorization: Bearer <token>)")

	go func() {
		if err := s.Serve(list); err != nil {
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
