package __1_metadata

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
