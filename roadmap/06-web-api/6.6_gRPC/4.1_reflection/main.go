package __1_reflection

/*
РАЗДЕЛ 4.1: gRPC REFLECTION
Reflection (рефлексия) — это механизм, который позволяет gRPC-серверу
"рассказывать" клиентам о своих возможностях в рантайме. Это как
OpenAPI для REST, но встроенное в протокол gRPC.

В этом разделе мы разберём:
  1. Что такое Reflection и зачем он нужен
  2. Как работает Reflection (протокол, v1 vs v1alpha)
  3. Включение Reflection на сервере (Go)
  4. Использование grpcurl для тестирования
  5. Использование Postman / Bruno с Reflection
  6. Безопасность: когда включать, когда выключать
  7. Ограничения и подводные камни
  8. Best practices и типичные ошибки
  9. Ключевые выводы для собеседования

	1. ЧТО ТАКОЕ REFLECTION И ЗАЧЕМ ОН НУЖЕН

	1.1. Определение
Reflection — это протокол, который позволяет gRPC-серверам декларировать
свои Protobuf-определения (сервисы, методы, сообщения) через
стандартизированный RPC-сервис.
Сервер предоставляет информацию о всех публично-доступных gRPC-сервисах,
а также типах, на которые ссылаются запросы и ответы.

	1.2. Зачем это нужно
Protobuf — бинарный, нечеловекочитаемый формат. Чтобы вручную отправить gRPC-запрос, нужно:
  • Знать, какие RPC-сервисы доступны на сервере.
  • Знать protobuf-определение request-сообщения и всех вложенных типов.
  • Знать protobuf-определение response-сообщения и всех вложенных типов.
  • Вручную собирать бинарные сообщения и декодировать ответы.
Reflection автоматизирует этот процесс. Инструменты (grpcurl, Postman)
"спрашивают" сервер о его возможностях и получают ответ в понятном виде.

	1.3. Аналогия из мира REST
Reflection для gRPC — это как открытый OpenAPI-документ (Swagger)
для REST API.

	2. КАК РАБОТАЕТ REFLECTION (ПРОТОКОЛ)

	2.1. Техническая реализация
Reflection — это отдельный gRPC-сервис, который сервер добавляет
к своим основным сервисам.

Протокол описан в репозитории grpc/grpc:
  https://github.com/grpc/grpc/blob/master/src/proto/grpc/reflection/v1/reflection.proto
Сервис называется grpc.reflection.v1.ServerReflection.

	2.2. Архитектура протокола
Reflection-сервис структурирован как bidirectional stream.
Это позволяет клиенту отправлять несколько запросов в одном соединении
и получать ответы в том же порядке.

Протокол решает две основные задачи:
  1. Method Reflection — определение, какие методы экспортирует сервер,
     являются ли они унарными или стриминговыми, и типы аргументов и
     результатов.
  2. Argument Reflection — преобразование между человекочитаемым форматом
     (JSON/ASCII-proto) и бинарным wire-форматом.

	2.3. FileDescriptorProtos
В основе reflection лежат FileDescriptorProtos — proto-кодированное
представление распарсенных .proto-файлов.

Сервер экспортирует:
  • FileDescriptorProto для заданного имени файла.
  • FileDescriptorProto для файла с заданным символом.
  • Список известных extension tag numbers.
Важно: запросы также возвращают все транзитивные зависимости
запрошенных файлов.

	2.4. v1 vs v1alpha
Ранее использовалась версия v1alpha, но она помечена как deprecated.
В новых версиях grpc-go используется v1.
В Go пакет google.golang.org/grpc/reflection использует v1-протокол.
При этом для обратной совместимости некоторые реализации поддерживают
обе версии.

	3. ВКЛЮЧЕНИЕ REFLECTION НА СЕРВЕРЕ (GO)

	3.1. Импорт пакета и регистрация
  import "google.golang.org/grpc/reflection"

  s := grpc.NewServer()
  pb.RegisterUserServiceServer(s, &server{})
  reflection.Register(s)
  s.Serve(lis)

	3.2. Полный пример (из grpc-go examples)
  func main() {
      lis, _ := net.Listen("tcp", ":50051")
      s := grpc.NewServer()
      pb.RegisterGreeterServer(s, &server{})
      reflection.Register(s)
      s.Serve(lis)
  }

	3.3. Проверка
После включения в списке сервисов появляется:
grpc.reflection.v1.ServerReflection


	4. ИСПОЛЬЗОВАНИЕ GRPCURL
grpcurl — это аналог curl для gRPC. Он использует reflection
для автоматического обнаружения сервисов и методов.

	4.1. Установка
  # macOS
  brew install grpcurl

  # Скачать бинарник с GitHub
  # https://github.com/fullstorydev/grpcurl/releases

  # Docker
  docker pull fullstorydev/grpcurl:latest

	4.2. Основные команды (с --plaintext для незашифрованных соединений)
  # Список всех сервисов
  grpcurl -plaintext localhost:50051 list

  # Вывод:
  # grpc.reflection.v1.ServerReflection
  # user.v1.UserService

  # Список методов в сервисе
  grpcurl -plaintext localhost:50051 list user.v1.UserService

  # Вывод:
  # user.v1.UserService.GetUser
  # user.v1.UserService.ListUsers

  # Описание сервиса
  grpcurl -plaintext localhost:50051 describe user.v1.UserService

  # Описание метода
  grpcurl -plaintext localhost:50051 describe user.v1.UserService.GetUser

  # Вызов unary метода
  grpcurl -plaintext -d '{"user_id":"1"}' \
    localhost:50051 user.v1.UserService/GetUser

  # Вызов server-streaming метода
  grpcurl -plaintext -d '{"limit":3}' \
    localhost:50051 user.v1.UserService/ListUsers

	4.3. Без reflection
Если reflection не включён, можно использовать:
  • .proto файлы: grpcurl -proto path/to/file.proto ...
  • Protoset файлы (собранные descriptor protos)


	5. ИСПОЛЬЗОВАНИЕ POSTMAN / BRUNO

	5.1. Postman
Postman поддерживает gRPC-запросы с использованием Reflection.

Как использовать:
  1. Создать новый gRPC-запрос.
  2. Ввести URL сервера (например, localhost:50051).
  3. В секции "Service definition" выбрать "Use server reflection".
  4. Postman автоматически подтянет все сервисы и методы.
  5. Выбрать метод и отправить запрос.

	5.2. Bruno
Bruno также поддерживает gRPC Reflection.

Как использовать:
  1. Открыть gRPC-запрос в Bruno.
  2. В интерфейсе найти секцию "Using Reflection".
  3. Включить toggle "Using Reflection".
  4. Нажать кнопку "Refresh" для загрузки последних изменений с сервера.
  5. Выбрать метод из выпадающего списка.

	5.3. Если reflection не работает
Можно загрузить .proto-файлы вручную:
  • В Postman: загрузить .proto файлы в секции "Service definition".
  • В Bruno: нажать "Browse for proto file" и выбрать файлы.

	6. БЕЗОПАСНОСТЬ: КОГДА ВКЛЮЧАТЬ, КОГДА ВЫКЛЮЧАТЬ

	6.1. Риски безопасности
Reflection раскрывает полную структуру API:
  • Все сервисы и методы.
  • Все типы сообщений и их поля.
  • Все enum и их значения.

Если Reflection оставлен включенным в production, злоумышленник получает
полную карту API. Это значительно упрощает поиск уязвимостей
и неавторизованных методов.

Атаки могут быть направлены на:
  • Exposed service methods (неавторизованные методы).
  • Weak method-level authorization.
  • Unsafe Protobuf parsing.
  • Server reflection.

	6.2. Когда включать
  + Разработка и локальное тестирование.
  + Staging-окружение для отладки.
  + Внутренние сервисы, недоступные извне.
  + CI/CD для автоматического тестирования.

	6.3. Когда выключать
  - Публичные API.
  - Production с внешним доступом.
  - Любое окружение, где API доступно из ненадёжных сетей.

	6.4. Практические рекомендации
  1. Включать Reflection только через feature flag или переменную окружения:
     if os.Getenv("ENABLE_REFLECTION") == "true" {
         reflection.Register(s)
     }
  2. В Kubernetes: включать только в dev/staging, в production отключать.
  3. Использовать mTLS или token-аутентификацию для доступа к reflection
     сервису, если он должен быть доступен в production.
  4. Регистрировать reflection только в non-production сборках.
  5. Ограничивать доступ к reflection-сервису через сетевые политики (только с доверенных IP).

	6.5. Влияние на производительность
Reflection не влияет на производительность основных RPC-вызовов.
Единственное влияние — дополнительная память для хранения дескрипторов
protobuf.


	7. ОГРАНИЧЕНИЯ И ПОДВОДНЫЕ КАМНИ

	7.1. Reverse proxy traversal
Если gRPC-сервер находится за reverse proxy, reflection-сервис также
должен быть правильно маршрутизирован на бэкенд.
Прокси может не поддерживать bidirectional streaming, который использует
reflection. Убедитесь, что прокси поддерживает HTTP/2 стриминг.

	7.2. Неполный список методов
Сервер не обязан возвращать полный список всех методов.
Например, reverse proxy может поддерживать reflection только для методов,
реализованных непосредственно на прокси, но не для всех методов бэкендов.

	7.3. Не все клиенты поддерживают v1
Некоторые старые инструменты могут использовать только v1alpha.
В grpc-go используется v1, но сохраняется обратная совместимость.

	7.4. Бинарные протобуфы с комментариями
Reflection возвращает дескрипторы, которые не включают комментарии
из .proto-файлов. Это важно для документации.

	8. BEST PRACTICES И ТИПИЧНЫЕ ОШИБКИ

	8.1. Best Practices
  + Всегда включай Reflection в dev-окружении — это экономит часы отладки.
  + Используй grpcurl для быстрого тестирования методов.
  + Включай Reflection через feature flag или переменную окружения.
  + Документируй, включён ли Reflection в разных окружениях.
  + Для автоматических тестов используй Reflection для динамического
    обнаружения методов.
  + При использовании прокси проверяй поддержку bidirectional streaming.

	8.2. Типичные ошибки
  - Reflection не включён, а клиент пытается его использовать.
    grpcurl выдаст: "failed to query for service descriptor".
    Решение: включить reflection.Register(s).

  - Reflection включён в production с публичным доступом.
    Злоумышленник может узнать все методы и структуры API.
    Решение: отключать через переменную окружения.

  - Сервер за прокси/балансировщиком, а reflection-сервис не проброшен.
    Reflection работает только если запросы доходят до сервера.
    Решение: настроить маршрутизацию для reflection-сервиса.

  - Использование устаревшей v1alpha-версии reflection.
    В новых версиях grpc-go используется v1.
    Решение: использовать актуальную версию пакета.

  - Забыть про -plaintext флаг при работе с insecure-соединениями.
    grpcurl ожидает TLS по умолчанию.
    Решение: добавить -plaintext.


	9. КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1. Reflection — это протокол, позволяющий серверу "рассказывать" о своих API в рантайме.
  2. В Go включается одной строкой: reflection.Register(s).
  3. Reflection используется grpcurl, Postman, Bruno и другими инструментами.
  4. Без Reflection для тестирования gRPC нужно вручную знать структуру всех сообщений и иметь .proto-файлы.
  5. Reflection — аналог OpenAPI для REST.
  6. Reflection строится на FileDescriptorProtos — proto-представлении .proto-файлов.
  7. Reflection использует bidirectional streaming.
  8. Включать: dev, staging, внутренние сервисы. Выключать: public production API.
  9. В production можно включать через feature flag.
 10. grpcurl — основной CLI-инструмент для работы с gRPC через Reflection.
 11. При использовании прокси/балансировщика нужно пробрасывать запросы к reflection-сервису.
 12. Reflection — это инструмент разработки, а не production-фича.
*/
