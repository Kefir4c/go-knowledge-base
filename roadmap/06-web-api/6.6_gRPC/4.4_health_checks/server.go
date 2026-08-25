package healthchecks

/*
  РАЗДЕЛ 4.4: HEALTH CHECKS
  Health Checks — это механизм, позволяющий внешним системам (балансировщикам,
  оркестраторам, мониторингу) определять, готов ли сервер принимать трафик
  и не находится ли он в состоянии отказа. В мире Kubernetes это КРИТИЧЕСКИ
  ВАЖНЫЙ компонент для zero-downtime deployments и самовосстановления.

  В этом разделе мы разберём:
    1. Что такое Health Checks и зачем они нужны
    2. Стандартный протокол grpc.health.v1.Health
    3. Реализация Health сервера на Go (health.NewServer)
    4. Методы Check и Watch
    5. Использование grpc_health_probe для Kubernetes
    6. Нативные gRPC probes в Kubernetes (начиная с v1.27)
    7. Liveness vs Readiness vs Startup probes
    8. Best practices и типичные ошибки
    9. Ключевые выводы для собеседования

  1.  ЧТО ТАКОЕ HEALTH CHECKS И ЗАЧЕМ ОНИ НУЖНЫ
  Health Check (проверка здоровья) — это механизм, с помощью которого
  внешняя система (балансировщик нагрузки, оркестратор, мониторинг)
  опрашивает сервис и определяет, может ли он обрабатывать запросы.

  Зачем это нужно:
    • Балансировка нагрузки — если сервис нездоров, балансировщик должен исключить его из пула.
    • Самовосстановление (self-healing) — Kubernetes перезапускает нездоровые Pod'ы.
    • Zero-downtime deployments — при обновлении старые Pod'ы выводятся из ротации до завершения.
    • Graceful shutdown — перед остановкой сервис переводится в состояние NOT_SERVING.
    • Обнаружение проблем на ранней стадии — до того, как они затронут пользователей.

  2.  СТАНДАРТНЫЙ ПРОТОКОЛ GRPC.HEALTH.V1.HEALTH
  gRPC предоставляет стандартный протокол для health checks, описанный в health.proto.

  2.1. Определение протокола
    message HealthCheckRequest {
      string service = 1;
    }

    message HealthCheckResponse {
      enum ServingStatus {
        UNKNOWN = 0;
        SERVING = 1;
        NOT_SERVING = 2;
      }
      ServingStatus status = 1;
    }

    service Health {
      rpc Check(HealthCheckRequest) returns (HealthCheckResponse);
      rpc Watch(HealthCheckRequest) returns (stream HealthCheckResponse);
    }

  2.2. Статусы ServingStatus
    • UNKNOWN (0) — статус неизвестен (обычно начальное состояние).
    • SERVING (1) — сервис здоров, готов принимать запросы.
    • NOT_SERVING (2) — сервис не готов принимать запросы
      (завершается, перегружен, не инициализирован).

  2.3. Метод Check
    Unary RPC, который возвращает текущий статус здоровья.
    Используется для:
      • Периодических проверок балансировщиками.
      • Ручной проверки через grpcurl.
      • exec probes в Kubernetes (до v1.27).

    Если запрошенный сервис не найден, возвращается NOT_FOUND.

  2.4. Метод Watch
    Streaming RPC, который позволяет клиенту подписаться на изменения
    статуса здоровья. Используется для:
      • Клиентской балансировки нагрузки (client-side LB).
      • Автоматического обновления состояния при изменении.
      • Эффективного мониторинга без постоянных опросов.

  2.5. Пустой service name ("") — общее здоровье сервера
    Если клиент не указывает конкретный сервис (передаёт пустую строку),
    сервер должен ответить общим статусом здоровья всего сервера.


  3.  РЕАЛИЗАЦИЯ HEALTH СЕРВЕРА НА GO
  В Go есть стандартная реализация health сервера в пакете
  google.golang.org/grpc/health.

  3.1. Импорт и создание Health сервера
    import (
      "google.golang.org/grpc/health"
      healthpb "google.golang.org/grpc/health/grpc_health_v1"
    )

    healthServer := health.NewServer()
    healthpb.RegisterHealthServer(s, healthServer)

    // Устанавливаем начальный статус
    healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

  3.2. Установка статуса для конкретного сервиса
    // Для конкретного сервиса
    healthServer.SetServingStatus("user.v1.UserService",
      healthpb.HealthCheckResponse_SERVING)

    // Для общего статуса сервера (пустая строка)
    healthServer.SetServingStatus("",
      healthpb.HealthCheckResponse_SERVING)

  3.3. Изменение статуса при shutdown
    // Перед graceful shutdown переводим в NOT_SERVING
    healthServer.SetServingStatus("",
      healthpb.HealthCheckResponse_NOT_SERVING)

    // Даём время балансировщику обновить состояние
    time.Sleep(5 * time.Second)

    // Теперь можно выполнять graceful shutdown
    s.GracefulStop()

  3.4. Полный пример включения Health сервера
    func main() {
      // Создаём gRPC сервер
      s := grpc.NewServer()

      // Регистрируем бизнес-сервисы
      pb.RegisterUserServiceServer(s, &UserServer{})

      //РЕГИСТРИРУЕМ HEALTH СЕРВИС
      healthServer := health.NewServer()
      healthpb.RegisterHealthServer(s, healthServer)
      healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

      // Запускаем сервер
      lis, _ := net.Listen("tcp", ":50051")
      s.Serve(lis)
    }

  4.  ИСПОЛЬЗОВАНИЕ GRPC_HEALTH_PROBE В KUBERNETES (ИСТОРИЧЕСКИЙ ПОДХОД)
  До Kubernetes v1.27 не было нативной поддержки gRPC health checks.
  Вместо этого использовался exec probe с grpc_health_probe.

  4.1. Что такое grpc_health_probe
  Это утилита командной строки, которая делает RPC вызов к
  /grpc.health.v1.Health/Check.

  Если ответ содержит SERVING, утилита завершается с кодом 0 (успех),
  иначе — с ненулевым кодом.

  4.2. Установка grpc_health_probe
    # Через go install
    go install github.com/grpc-ecosystem/grpc-health-probe@latest

    # Или скачать бинарник с GitHub Releases
    # https://github.com/grpc-ecosystem/grpc-health-probe/releases

  4.3. Использование в Kubernetes (exec probe)
    livenessProbe:
      exec:
        command:
        - /grpc_health_probe
        - -addr=:50051
      initialDelaySeconds: 10
      periodSeconds: 10

    readinessProbe:
      exec:
        command:
        - /grpc_health_probe
        - -addr=:50051
      initialDelaySeconds: 5
      periodSeconds: 5

  4.4. Дополнительные параметры grpc_health_probe
    • -addr — адрес сервера (по умолчанию localhost:50051)
    • -connect-timeout — таймаут соединения
    • -rpc-timeout — таймаут RPC вызова
    • -service — имя сервиса для проверки
    • -tls — использовать TLS
    • -tls-ca-cert — путь к CA сертификату[

  5.  НАТИВНЫЕ GRPC PROBES В KUBERNETES (НАЧИНАЯ С V1.27)
  Начиная с Kubernetes v1.27 (GA), появилась нативная поддержка
  gRPC health probes[.

  5.1. Преимущества нативных probes
    • Не нужно носить дополнительный бинарник (10MB) в образе[.
    • Exec probes медленнее — они требуют создания нового процесса[.
    • Меньше потребление ресурсов при проверках.
    • Единообразная конфигурация для всех gRPC сервисов.

  5.2. Конфигурация нативных gRPC probes
    livenessProbe:
      grpc:
        port: 50051
        service: user.v1.UserService  # опционально
      initialDelaySeconds: 10
      periodSeconds: 10

    readinessProbe:
      grpc:
        port: 50051
      initialDelaySeconds: 5
      periodSeconds: 5

    startupProbe:
      grpc:
        port: 50051
      failureThreshold: 30
      periodSeconds: 10

  5.3. Как это работает
    • Kubernetes отправляет gRPC запрос к /grpc.health.v1.Health/Check.
    • Если ответ SERVING — probe успешен.
    • Если ответ NOT_SERVING или ошибка — probe неуспешен.
    • Можно указать конкретный service для проверки[.

  6.  LIVENESS VS READINESS VS STARTUP PROBES
  В Kubernetes существует три типа probes, каждый со своей целью.

  6.1. Liveness Probe (проверка живости)
    • Определяет, жив ли контейнер.
    • Если probe неуспешен — контейнер перезапускается.
    • Используется для обнаружения deadlock, бесконечных циклов,
      утечек памяти.
    • Должен быть "лёгким" и быстрым.

    livenessProbe:
      grpc:
        port: 50051
      initialDelaySeconds: 30
      periodSeconds: 10

  6.2. Readiness Probe (проверка готовности)
    • Определяет, готов ли контейнер принимать трафик.
    • Если probe неуспешен — Pod исключается из Service (балансировки).
    • Используется при старте (дожидаемся инициализации) и при
      временных проблемах (перегрузка, восстановление).
    • Если probe успешен — трафик направляется на Pod.

    readinessProbe:
      grpc:
        port: 50051
      initialDelaySeconds: 5
      periodSeconds: 5

  6.3. Startup Probe (проверка запуска)
    • Используется для приложений с долгим стартом.
    • Даёт приложению больше времени на инициализацию.
    • После успешного завершения startup probe, liveness и readiness
      probes начинают работать.
    • Защищает от преждевременных перезапусков.

    startupProbe:
      grpc:
        port: 50051
      failureThreshold: 30
      periodSeconds: 10

  7.  ПРАКТИЧЕСКАЯ РЕАЛИЗАЦИЯ НА GO (ПОЛНЫЙ ПРИМЕР)

  7.1. Сервер с Health Checks
    type server struct {
      pb.UnimplementedUserServiceServer
    }

    func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
      // бизнес-логика
    }

    func main() {
      lis, _ := net.Listen("tcp", ":50051")

      s := grpc.NewServer()

      // Регистрируем бизнес-сервис
      pb.RegisterUserServiceServer(s, &server{})

      // РЕГИСТРИРУЕМ HEALTH СЕРВИС
      healthServer := health.NewServer()
      healthpb.RegisterHealthServer(s, healthServer)

      // Устанавливаем начальный статус — SERVING
      healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

      log.Println("Сервер запущен на :50051")
      log.Println("Health status: SERVING")

      go func() {
        if err := s.Serve(lis); err != nil {
          log.Fatal(err)
        }
      }()

      // Graceful shutdown
      quit := make(chan os.Signal, 1)
      signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
      <-quit

      log.Println("Получен сигнал завершения")

      //ПЕРЕВОДИМ В NOT_SERVING
      healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
      log.Println("Health status: NOT_SERVING")

      // Даём время балансировщику обновить состояние
      time.Sleep(5 * time.Second)

      s.GracefulStop()
      log.Println("Сервер остановлен")
    }

  7.2. Проверка через grpcurl
    # Проверить общий статус
    grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check

    # Проверить статус конкретного сервиса
    grpcurl -plaintext -d '{"service":"user.v1.UserService"}' \
      localhost:50051 grpc.health.v1.Health/Check


  8.  BEST PRACTICES И ТИПИЧНЫЕ ОШИБКИ

  8.1. Best Practices
    + Всегда регистрируй Health сервис на всех gRPC серверах.
    + Используй пустую строку ("") для общего статуса сервера.
    + Переводи статус в NOT_SERVING перед graceful shutdown.
    + Давай 5-10 секунд между NOT_SERVING и остановкой для обновления
      балансировщика.
    + Для Kubernetes используй нативные gRPC probes (начиная с v1.27).
    + Для старых версий K8s используй grpc_health_probe.
    + Отличай liveness (перезапуск) от readiness (исключение из балансировки).
    + Для сервисов с долгим стартом используй startup probe.
    + Проверяй не только общий статус, но и зависимости (БД, кэш).
    + Логируй изменения статуса health для отладки.

  8.2. Типичные ошибки
    - Не регистрировать Health сервис.
      Решение: всегда добавлять healthpb.RegisterHealthServer(s, healthServer).
    - Не переводить статус в NOT_SERVING при shutdown.
      Решение: healthServer.SetServingStatus("", NOT_SERVING).
    - Использовать liveness probe для проверки зависимостей.
      Решение: liveness только для "жив" ли процесс, readiness для зависимостей.
    - Слишком частые проверки (periodSeconds < 1).
      Решение: не чаще 1 секунды, обычно 5-30 секунд.
    - Слишком строгие проверки (fail на любую ошибку).
      Решение: допускать временные сбои.
    - Использовать exec probe с grpc_health_probe в новых K8s версиях.
      Решение: использовать нативные gRPC probes.

  9.  КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1.  Health Checks — механизм определения готовности сервиса
      принимать запросы.
  2.  Стандартный протокол: grpc.health.v1.Health с методами Check и Watch.
  3.  Статусы: UNKNOWN, SERVING, NOT_SERVING.
  4.  В Go включается: health.NewServer() + healthpb.RegisterHealthServer().
  5.  Пустая строка ("") — общий статус всего сервера.
  6.  grpc_health_probe — утилита для проверки health (исторический подход).
  7.  Kubernetes v1.27+ имеет нативные gRPC probes.
  8.  Liveness — перезапуск Pod'а при проблемах.
      Readiness — исключение из балансировки.
      Startup — дополнительное время для инициализации.
  9.  Перед shutdown переводи статус в NOT_SERVING и жди 5-10 секунд.
  10. Health Checks — обязательный компонент для production gRPC сервисов.
*/

/*
2. Проверка через grpcurl
bash
# Проверить общий статус
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check

# Ответ:
# {
#   "status": "SERVING"
# }

# Проверить статус конкретного сервиса (если нужно)
grpcurl -plaintext -d '{"service":"user.v1.UserService"}' \
  localhost:50051 grpc.health.v1.Health/Check

3. Kubernetes манифест (pod.yaml) — как это используется в реальном проекте

yaml
apiVersion: v1
kind: Pod
metadata:
  name: grpc-server
spec:
  containers:
  - name: server
    image: my-grpc-server:latest
    ports:
    - containerPort: 50051
    #NATIVE GRPC PROBES (KUBERNETES v1.27+)
    livenessProbe:
      grpc:
        port: 50051
      initialDelaySeconds: 10
      periodSeconds: 10

    readinessProbe:
      grpc:
        port: 50051
      initialDelaySeconds: 5
      periodSeconds: 5

    startupProbe:
      grpc:
        port: 50051
      failureThreshold: 30
      periodSeconds: 10

*/
