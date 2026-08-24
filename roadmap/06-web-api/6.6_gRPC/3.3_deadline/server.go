package deadline

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
  РАЗДЕЛ 3.3: DEADLINES И ERROR HANDLING
  Deadlines (таймауты) и Error Handling (обработка ошибок) — это КРИТИЧЕСКИ
  ВАЖНЫЕ механизмы в gRPC. Без дедлайнов твой сервис будет зависать навечно
  и убивать всю систему каскадными отказами. Без правильной обработки ошибок
  клиент не поймёт, что пошло не так.

  В этом разделе мы разберём:
    1.  Что такое deadlines и почему они критичны в микросервисах
    2.  Установка таймаута на клиенте (context.WithTimeout)
    3.  Проверка ctx.Err() на сервере
    4.  Пропагация дедлайнов через цепочку вызовов
    5.  Коды статусов (codes.NotFound, Internal, InvalidArgument и др.)
    6.  Возврат ошибки через status.Error()
    7.  Обработка ошибок на клиенте
    8.  Расширенные ошибки с деталями (WithDetails)
    9.  Best practices и типичные ошибки
    10. Ключевые выводы для собеседования

  1.  ЧТО ТАКОЕ DEADLINES И ПОЧЕМУ ОНИ КРИТИЧНЫ
  Deadline (срок, таймаут) — это МАКСИМАЛЬНОЕ ВРЕМЯ, которое клиент готов
  ждать ответа от сервера. Если ответ не получен за это время, запрос
  отменяется.

  Без дедлайна запрос может висеть ВЕЧНО, если сервер завис, сеть упала
  или произошла какая-то другая проблема. В микросервисной архитектуре
  это приводит к КАСКАДНОМУ ОТКАЗУ (cascading failure) — один медленный
  сервис тянет за собой все остальные.

  ПОЧЕМУ ДЕДЛАЙНЫ ВАЖНЫ:

    • Предотвращают исчерпание ресурсов — запросы, которые выполняются
      слишком долго, автоматически отменяются.
    • Обеспечивают предсказуемую задержку — пользователь знает,
      максимальное время ожидания.
    • Позволяют graceful degradation — сервисы могут вернуть частичные
      результаты вместо бесконечного ожидания.
    • Пропагируются через цепочку вызовов — каждый сервис знает,
      сколько времени осталось.

  Аналогия: ты заказываешь пиццу и говоришь: "Если за 40 минут не привезут —
  я отменяю заказ". Это и есть deadline.

  2.  УСТАНОВКА ТАЙМАУТА НА КЛИЕНТЕ (CONTEXT.WITHTIMEOUT)
  На клиенте deadline устанавливается через контекст с таймаутом.

  2.1. Базовое использование
    // Устанавливаем таймаут 5 секунд
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel() // ОБЯЗАТЕЛЬНО: освобождаем ресурсы

    // Вызываем gRPC-метод с этим контекстом
    resp, err := client.GetUser(ctx, &pb.GetUserRequest{UserId: "1"})
    if err != nil {
        // обрабатываем ошибку
    }

  2.2. Почему важно вызывать cancel()
    • Даже если запрос завершился успешно, контекст всё равно нужно отменить.
    • Это освобождает ресурсы и предотвращает утечки памяти.
    • Используй defer cancel() сразу после создания контекста.

  2.3. Использование с метаданными
    // Сначала создаём контекст с metadata
    md := metadata.Pairs("authorization", "Bearer token")
    ctx := metadata.NewOutgoingContext(context.Background(), md)

    // Потом добавляем таймаут
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    resp, err := client.GetUser(ctx, req)

  2.4. Использование с отменой (cancel)
    // Можно отменить запрос вручную до истечения таймаута
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Какая-то логика, после которой нужно отменить запрос
    if condition {
        cancel() // запрос будет отменён, сервер получит ctx.Done()
    }

  3.  ПРОВЕРКА CTX.ERR() НА СЕРВЕРЕ
  На сервере нужно проверять, не истёк ли deadline, и не отменил ли клиент
  запрос. Это делается через контекст, который приходит в метод.

  3.1. Проверка перед выполнением работы
    func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
        // Проверка перед долгой операцией
        if ctx.Err() == context.DeadlineExceeded {
            return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
        }
        if ctx.Err() == context.Canceled {
            return nil, status.Error(codes.Canceled, "request cancelled by client")
        }
        // ... бизнес-логика
    }

  3.2. Проверка в циклах (для долгих операций)
    func (s *server) ProcessItems(ctx context.Context, req *pb.Request) (*pb.Response, error) {
        for i, item := range items {
            // Проверяем на каждой итерации
            select {
            case <-ctx.Done():
                return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
            default:
                // продолжаем работу
            }

            // обрабатываем item
            result := s.processItem(item)
            // ...
        }
        return &pb.Response{}, nil
    }

  3.3. Проверка при вызове других сервисов
    // При вызове другого сервиса контекст передаётся автоматически
    func (s *server) Handle(ctx context.Context, req *pb.Request) (*pb.Response, error) {
        // deadline автоматически пропагируется в вызов serviceB
        resp, err := s.clientB.Call(ctx, reqB)
        if err != nil {
            // проверяем статус ошибки
        }
        return resp, nil
    }

  3.4. Варианты ctx.Err()
    • context.DeadlineExceeded — дедлайн истёк.
    • context.Canceled — клиент отменил запрос (через cancel()).
    • nil — всё в порядке, контекст не завершён.

  4.  ПРОПАГАЦИЯ ДЕДЛАЙНОВ ЧЕРЕЗ ЦЕПОЧКУ ВЫЗОВОВ
  Одно из главных преимуществ дедлайнов в gRPC — они автоматически
  пропагируются через всю цепочку вызовов.

  4.1. Схема пропагации
    Клиент → Сервис A → Сервис B → Сервис C
       │           │           │           │
       │ deadline  │ deadline  │ deadline  │
       │ 5s        │ 4.5s      │ 4.0s      │

    • Клиент устанавливает deadline 5 секунд.
    • Сервис A получает этот deadline и передаёт его дальше.
    • Если deadline истёк в Сервисе B, он возвращает ошибку.
    • Ошибка доходит до клиента.

  4.2. Как это работает в Go
    // Клиент
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    resp, err := clientA.Call(ctx, req)

    // Сервис A
    func (s *serviceA) Call(ctx context.Context, req *pb.Request) (*pb.Response, error) {
        // deadline уже в ctx
        // Передаём ctx в вызов Сервиса B
        resp, err := s.clientB.Call(ctx, reqB)
        // deadline автоматически пропагируется
        return resp, err
    }

  4.3. Важный нюанс: создание нового контекста
    //ПЛОХО: создаём новый контекст, теряем deadline
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    //ХОРОШО: используем входящий контекст
    resp, err := s.clientB.Call(ctx, reqB)

  5.  КОДЫ СТАТУСОВ (CODES.NOTFOUND, INTERNAL И ДР.)
  В gRPC ошибки возвращаются через статусы с кодами из пакета codes.
  Это делает обработку ошибок предсказуемой на всех языках.

  5.1. Полный список основных кодов
  ┌─────────────────────┬──────┬──────────────────────────────────────────────┐
  │ Код                 │ №    │ Описание и аналог в HTTP                     │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ OK                  │ 0    │ Успех                                        │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ Canceled            │ 1    │ Операция отменена (клиент)                   │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ Unknown             │ 2    │ Неизвестная ошибка                           │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ InvalidArgument     │ 3    │ Неверный аргумент (400 Bad Request)          │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ DeadlineExceeded    │ 4    │ Истек таймаут (504 Gateway Timeout)          │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ NotFound            │ 5    │ Ресурс не найден (404 Not Found)             │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ AlreadyExists       │ 6    │ Ресурс уже существует (409 Conflict)         │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ PermissionDenied    │ 7    │ Недостаточно прав (403 Forbidden)            │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ ResourceExhausted   │ 8    │ Исчерпаны лимиты (429 Too Many Requests)     │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ FailedPrecondition  │ 9    │ Предварительное условие не выполнено         │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ Aborted             │ 10   │ Операция прервана                            │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ OutOfRange          │ 11   │ Значение вне диапазона                       │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ Unimplemented       │ 12   │ Метод не реализован (501 Not Implemented)    │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ Internal            │ 13   │ Внутренняя ошибка (500 Internal Server)      │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ Unavailable         │ 14   │ Сервис недоступен (503 Service Unavailable)  │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ DataLoss            │ 15   │ Потеря данных                                │
  ├─────────────────────┼──────┼──────────────────────────────────────────────┤
  │ Unauthenticated     │ 16   │ Не авторизован (401 Unauthorized)            │
  └─────────────────────┴──────┴──────────────────────────────────────────────┘

  6.  ВОЗВРАТ ОШИБКИ ЧЕРЕЗ STATUS.ERROR()

  6.1. Базовая ошибка
    // Импорт пакета status
    import "google.golang.org/grpc/status"

    // Проверка входных данных
    if req.UserId == "" {
        return nil, status.Error(codes.InvalidArgument, "user_id is required")
    }

    // Пользователь не найден
    user, err := s.store.Get(req.UserId)
    if errors.Is(err, ErrNotFound) {
        return nil, status.Error(codes.NotFound, "user not found")
    }

    // Внутренняя ошибка
    if err != nil {
        return nil, status.Error(codes.Internal, "database error")
    }

  6.2. Ошибка с дополнительными деталями (WithDetails)
    // Создаём статус с деталями
    st, err := status.New(codes.InvalidArgument, "validation failed").
        WithDetails(&pb.ErrorDetails{
            Field:  "email",
            Reason: "must be valid email format",
        })
    if err != nil {
        return nil, status.Error(codes.Internal, "failed to create error details")
    }
    return nil, st.Err()

  6.3. Получение деталей на клиенте
    st, ok := status.FromError(err)
    if ok {
        for _, detail := range st.Details() {
            if errDetail, ok := detail.(*pb.ErrorDetails); ok {
                log.Printf("Field: %s, Reason: %s", errDetail.Field, errDetail.Reason)
            }
        }
    }

  7.  ОБРАБОТКА ОШИБОК НА КЛИЕНТЕ

  7.1. Базовая обработка
    resp, err := client.GetUser(ctx, req)
    if err != nil {
        st, ok := status.FromError(err)
        if !ok {
            // Не gRPC-ошибка (например, сеть упала)
            log.Printf("Non-gRPC error: %v", err)
            return
        }

        switch st.Code() {
        case codes.NotFound:
            log.Println("User not found:", st.Message())
        case codes.InvalidArgument:
            log.Println("Invalid argument:", st.Message())
        case codes.DeadlineExceeded:
            log.Println("Request timed out")
        case codes.Unauthenticated:
            log.Println("Authentication failed")
        case codes.PermissionDenied:
            log.Println("Access denied")
        case codes.Internal:
            log.Println("Internal server error:", st.Message())
        default:
            log.Printf("Other error: %s", st.Message())
        }
        return
    }
    log.Printf("User: %+v", resp)

  7.2. Проверка ошибки с деталями
    resp, err := client.GetUser(ctx, req)
    if err != nil {
        st, ok := status.FromError(err)
        if ok {
            // Проверяем детали
            for _, detail := range st.Details() {
                if errDetail, ok := detail.(*pb.ErrorDetails); ok {
                    log.Printf("Field error: %s → %s", errDetail.Field, errDetail.Reason)
                }
            }
        }
    }

  7.3. Обработка таймаута
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    resp, err := client.GetUser(ctx, req)
    if err != nil {
        st, ok := status.FromError(err)
        if ok && st.Code() == codes.DeadlineExceeded {
            log.Println("Request timed out — retrying...")
            // Логика повторного запроса
        }
    }

  8.  РАСШИРЕННЫЕ ОШИБКИ С ДЕТАЛЯМИ (WITHDETAILS)

  8.1. Создание ошибки с несколькими деталями
    // Определяем структуру деталей в .proto
    message ErrorDetails {
        string field = 1;
        string reason = 2;
        string value = 3;
    }

    // Сервер возвращает ошибку с деталями
    func (s *server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
        var details []*pb.ErrorDetails

        if req.Email == "" {
            details = append(details, &pb.ErrorDetails{
                Field:  "email",
                Reason: "email is required",
            })
        }
        if len(req.Name) < 3 {
            details = append(details, &pb.ErrorDetails{
                Field:  "name",
                Reason: "name must be at least 3 characters",
                Value:  req.Name,
            })
        }

        if len(details) > 0 {
            st, _ := status.New(codes.InvalidArgument, "validation failed").
                WithDetails(details...)
            return nil, st.Err()
        }
        // ... создание пользователя
    }

  8.2. Клиент обрабатывает детали
    for _, detail := range st.Details() {
        if errDetail, ok := detail.(*pb.ErrorDetails); ok {
            switch errDetail.Field {
            case "email":
                log.Printf("Email error: %s", errDetail.Reason)
            case "name":
                log.Printf("Name error: %s (value: %s)", errDetail.Reason, errDetail.Value)
            }
        }
    }

  9.  BEST PRACTICES И ТИПИЧНЫЕ ОШИБКИ

  9.1. Best Practices
	Всегда устанавливай дедлайн на клиенте.
	Всегда вызывай cancel() (через defer).
	Проверяй ctx.Err() в долгих операциях на сервере.
	Передавай входящий контекст в дочерние вызовы.
	Используй правильные коды статусов.
	Добавляй детали к ошибкам для удобства клиента.
	Обрабатывай ошибки на клиенте через status.FromError().
	Логируй ошибки с контекстом (request_id, trace_id).

  9.2. Типичные ошибки
Нет дедлайна на клиенте. Запрос может висеть вечно.
Не вызывать cancel(). Утечка ресурсов.
Не проверять ctx.Err() в долгих операциях. Сервер продолжает работу после того, как клиент отменил запрос.
Создавать новый контекст вместо использования входящего. Потеря дедлайна.
Возвращать обычный error вместо статуса. Клиент не сможет определить тип ошибки.
Игнорировать ошибки на клиенте. Неправильное поведение приложения.

  10. КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1.  Deadline — таймаут на клиенте, защита от бесконечного ожидания.
  2.  Устанавливается через context.WithTimeout(ctx, duration).
  3.  Всегда вызывай cancel() через defer.
  4.  На сервере проверяй ctx.Err() == context.DeadlineExceeded.
  5.  Дедлайны автоматически пропагируются через цепочку вызовов.
  6.  Коды ошибок из пакета codes: NotFound, InvalidArgument, Internal и др.
  7.  Возвращай ошибки через status.Error(codes, message).
  8.  Добавляй детали через status.New(...).WithDetails(...).
  9.  На клиенте обрабатывай ошибки через status.FromError(err).
  10. Без дедлайнов — каскадный отказ (cascading failure).
*/

//ХРАНИЛИЩЕ

type UserStore struct {
	mu     sync.RWMutex
	users  map[string]*pb.User
	emails map[string]string // email -> id
	nextID int
}

func NewUserStore() *UserStore {
	return &UserStore{
		users:  make(map[string]*pb.User),
		emails: make(map[string]string),
		nextID: 1,
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

func (s *UserStore) Create(email, name string, age int32) (*pb.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Проверка на дубликат email
	if _, ok := s.emails[email]; ok {
		return nil, errors.New("user already exists")
	}

	id := fmt.Sprintf("%d", s.nextID)
	s.nextID++
	user := &pb.User{
		Id:        id,
		Email:     email,
		Name:      name,
		Age:       age,
		CreatedAt: timestamppb.Now(),
	}
	s.users[id] = user
	s.emails[email] = id
	return user, nil
}

//СЕРВЕР

type UserServer struct {
	pb.UnimplementedUserServiceServer
	store *UserStore
}

// GetUser — демонстрация разных ошибок
func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	// 1. ПРОВЕРКА ДЕДЛАЙНА
	if ctx.Err() == context.DeadlineExceeded {
		return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}
	if ctx.Err() == context.Canceled {
		return nil, status.Error(codes.Canceled, "request cancelled")
	}

	// 2. ПРОВЕРКА АУТЕНТИФИКАЦИИ (для демонстрации Unauthenticated)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	if len(md.Get("authorization")) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	// 3. ВАЛИДАЦИЯ ВХОДНЫХ ДАННЫХ
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// 4. ИМИТАЦИЯ ДОЛГОЙ ОПЕРАЦИИ (для демонстрации DeadlineExceeded)
	// Если userId == "slow", то искусственно замедляем запрос
	if req.UserId == "slow" {
		// Проверяем контекст в цикле
		for i := 0; i < 10; i++ {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return nil, status.Error(codes.DeadlineExceeded, "operation took too long")
				}
				return nil, status.Error(codes.Canceled, "request cancelled")
			default:
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	// 5. ПОЛУЧЕНИЕ ДАННЫХ
	user, err := s.store.Get(req.UserId)
	if err != nil {
		if err.Error() == "user not found" {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return user, nil
}

// CreateUser — демонстрация ошибок с деталями
func (s *UserServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	// 1. ПРОВЕРКА ДЕДЛАЙНА
	if ctx.Err() == context.DeadlineExceeded {
		return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	// 2. ВАЛИДАЦИЯ С ДЕТАЛЯМИ
	var details []*pb.ErrorDetails

	if req.Email == "" {
		details = append(details, &pb.ErrorDetails{
			Field:  "email",
			Reason: "email is required",
			Value:  req.Email,
		})
	}
	if len(req.Name) < 3 {
		details = append(details, &pb.ErrorDetails{
			Field:  "name",
			Reason: "name must be at least 3 characters",
			Value:  req.Name,
		})
	}
	if req.Age < 18 || req.Age > 99 {
		details = append(details, &pb.ErrorDetails{
			Field:  "age",
			Reason: "age must be between 18 and 99",
			Value:  fmt.Sprintf("%d", req.Age),
		})
	}

	if len(details) > 0 {
		st, _ := status.New(codes.InvalidArgument, "validation failed").
			WithDetails(details...)
		return nil, st.Err()
	}

	// 3. СОЗДАНИЕ ПОЛЬЗОВАТЕЛЯ
	user, err := s.store.Create(req.Email, req.Name, req.Age)
	if err != nil {
		if err.Error() == "user already exists" {
			return nil, status.Error(codes.AlreadyExists, "user already exists")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &pb.CreateUserResponse{User: user}, nil
}

func main() {
	store := NewUserStore()

	// Добавляем тестовых пользователей
	store.Create("Lizok.l@yandex.ru", "Liza", 25)
	store.Create("PepsiSesi.ps@main.ru", "pepsi", 24)

	s := grpc.NewServer()
	pb.RegisterUserServiceServer(s, &UserServer{store: store})

	list, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Сервер запущен на :50051")

	go func() {
		if err := s.Serve(list); err != nil {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Остановка сервера...")
	s.GracefulStop()
	log.Println("Сервер остановлен")
}
