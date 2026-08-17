# gRPC Roadmap

Этот roadmap — минимум, который тебе нужен, чтобы на собеседовании по gRPC выглядеть уверенно.

## Что ты будешь уметь после этого роадмапа:
- Объяснить, что такое gRPC и почему он круче REST.
- Написать .proto и сгенерировать код.
- Реализовать unary и server-streaming RPC (это 90% задач).
- Понимать, как работают интерсепторы (сделаешь свой).
- Настроить дедлайны и правильно обрабатывать ошибки.
- На базовом уровне понимать TLS и балансировку (чтобы ответить, если спросят).

## БЛОК 1: БАЗА
### 1.1. Что такое gRPC и зачем он нужен
- gRPC vs REST: скорость, размер, стриминг.
- HTTP/2 и protobuf — что дают.
- Когда использовать gRPC, когда REST.

### 1.2. Protocol Buffers (.proto)
- Синтаксис proto3: syntax, package, option go_package.
- Сообщения (message), поля, типы (int32, string, bool, repeated).
- Правила именования (PascalCase, snake_case).
- Well-known types: google.protobuf.Timestamp.

### 1.3. Генерация кода через protoc
- Установка protoc, protoc-gen-go, protoc-gen-go-grpc.
- Команда protoc: --go_out, --go-grpc_out, paths=source_relative.
- Что генерируется: .pb.go (сообщения) и _grpc.pb.go (сервисы).

### 1.4. Unary RPC (запрос-ответ)
- Определение в proto: rpc GetUser(Request) returns (Response).
- Реализация сервера: структура, метод.
- Реализация клиента: подключение, вызов.

## БЛОК 2: СТРИМИНГИ
### 2.1. Server-Streaming RPC
- Определение: returns (stream Response).
- Сервер: stream.Send() в цикле.
- Клиент: stream.Recv() до io.EOF.

### 2.2. Client-Streaming RPC (базово)
- Определение: rpc Create(stream Request) returns (Response).
- Сервер: чтение в цикле, SendAndClose().
- Клиент: Send(), CloseAndRecv().

## БЛОК 3: НАДЁЖНОСТЬ И БЕЗОПАСНОСТЬ
### 3.1. Metadata (заголовки) — **ВАЖНО!**
- Что такое metadata (аналог HTTP-заголовков).
- Отправка на клиенте: `metadata.NewOutgoingContext(ctx, md)`.
- Чтение на сервере: `metadata.FromIncomingContext(ctx)`.
- **Практика:** передача `authorization: Bearer <token>`.

### 3.2. Interceptors (middleware)
- UnaryInterceptor и StreamInterceptor.
- Логирование, проверка токена (JWT).
- Цепочки интерсепторов (grpc_middleware.ChainUnaryServer).

### 3.3. Deadlines и Error Handling
- Установка таймаута на клиенте (context.WithTimeout).
- Проверка ctx.Err() на сервере.
- Коды статусов (codes.NotFound, Internal и др.).
- Возврат ошибки через status.Error().

## БЛОК 4: ПРОДАКШЕН

### 4.1. gRPC Reflection — **ОБЯЗАТЕЛЬНО ДЛЯ DEV**
- Что это: позволяет Postman/grpcurl видеть методы без .proto.
- Включение: `reflection.Register(grpcServer)`.
- **Практика:** подключить Postman/Bruno к своему сервису.

### 4.2. TLS/mTLS
- Настройка TLS на сервере и клиенте.
- Взаимная аутентификация (mTLS) — что это, зачем.

### 4.3. Graceful Shutdown
- Корректное завершение: `srv.GracefulStop()`.
- Обработка сигналов `SIGINT`, `SIGTERM`.

### 4.4. Health Checks
- Стандартный сервис `grpc.health.v1.Health`.
- Зачем нужен в K8s (liveness/readiness probes).

### 4.5. Балансировка (базово)
- Client-side балансировка: `round_robin`.
- Service discovery через DNS.

---