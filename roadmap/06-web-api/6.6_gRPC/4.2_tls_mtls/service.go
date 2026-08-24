package __2_tls_mtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	pb "github.com/"
)

/*
  РАЗДЕЛ 4.2: TLS/mTLS
  TLS (Transport Layer Security) и его разновидность mTLS (Mutual TLS) —
  это фундамент безопасности в gRPC. В продакшене gRPC БЕЗ TLS — это
  красный флаг и грубая ошибка безопасности.

  В этом разделе мы разберём:
    1.  Что такое TLS и зачем он нужен в gRPC
    2.  Настройка TLS на сервере (односторонняя аутентификация)
    3.  Настройка TLS на клиенте
    4.  Что такое mTLS и зачем он нужен
    5.  Настройка mTLS на сервере
    6.  Настройка mTLS на клиенте
    7.  Сравнение TLS vs mTLS
    8.  Best practices и типичные ошибки
    9.  Ключевые выводы для собеседования
    10. Почему gRPC не безопасен по умолчанию
    11. TLS vs mTLS — практическая разница
    12. Service Mesh и автоматизация mTLS
    13. Реальные уязвимости (CVE) и подводные камни
    14. Альтернативы TLS/mTLS
    15. Связка mTLS с интерсепторами (Authorization)
    16. Чек-лист вопросов для собеседования

  1.  ЧТО ТАКОЕ TLS И ЗАЧЕМ ОН НУЖЕН В gRPC
  TLS (Transport Layer Security) — это протокол шифрования, который обеспечивает:
    • Шифрование данных — защита от прослушивания (eavesdropping).
    • Аутентификацию сервера — клиент проверяет, что он подключился именно к тому серверу, которому доверяет.
    • Целостность данных — защита от подмены (man-in-the-middle).

  В gRPC TLS используется для защиты всех передаваемых данных.
  Без TLS все данные передаются в открытом виде (plaintext).

  В продакшене TLS ОБЯЗАТЕЛЕН. Использование insecure-соединений
  допустимо ТОЛЬКО для локальной разработки и тестирования.

  gRPC имеет встроенную интеграцию с SSL/TLS и рекомендует использовать
  TLS для аутентификации сервера и шифрования всех данных.

  2.  НАСТРОЙКА TLS НА СЕРВЕРЕ (ОДНОСТОРОННЯЯ АУТЕНТИФИКАЦИЯ)
  "Односторонняя" означает, что сервер предоставляет сертификат,
  а клиент его проверяет. Клиент НЕ предоставляет свой сертификат.

  2.1. Что нужно для настройки
    • Серверный сертификат (server_cert.pem) — публичная часть.
    • Приватный ключ сервера (server_key.pem) — должен храниться в секрете.
    • (Опционально) CA-сертификат для проверки цепочки.

  2.2. Код сервера с TLS
    // Создаём TLS-креденшелы из файлов
    creds, err := credentials.NewServerTLSFromFile(
      "certs/server_cert.pem",
      "certs/server_key.pem",
    )
    if err != nil {
      log.Fatalf("Failed to load TLS credentials: %v", err)
    }

    // Создаём gRPC-сервер с TLS
    s := grpc.NewServer(grpc.Creds(creds))

  2.3. Вариант с кастомной TLS-конфигурацией
    // Загрузка сертификата
    serverCert, err := tls.LoadX509KeyPair(
      "certs/server_cert.pem",
      "certs/server_key.pem",
    )
    if err != nil {
      log.Fatal(err)
    }

    // Создаём TLS-конфиг
    tlsConfig := &tls.Config{
      Certificates: []tls.Certificate{serverCert},
      MinVersion:   tls.VersionTLS13, // принудительно TLS 1.3
    }

    creds := credentials.NewTLS(tlsConfig)
    s := grpc.NewServer(grpc.Creds(creds))

  3.  НАСТРОЙКА TLS НА КЛИЕНТЕ

  3.1. Базовый клиент с TLS
    // Создаём клиентские креденшелы
    // nil означает использование системного хранилища CA
    creds := credentials.NewClientTLSFromCert(nil, "")

    conn, err := grpc.NewClient(
      "localhost:50051",
      grpc.WithTransportCredentials(creds),
    )

  3.2. Клиент с кастомным CA (для самоподписанных сертификатов)
    // Загружаем CA-сертификат
    caCert, err := os.ReadFile("certs/ca_cert.pem")
    if err != nil {
      log.Fatal(err)
    }

    caCertPool := x509.NewCertPool()
    if !caCertPool.AppendCertsFromPEM(caCert) {
      log.Fatal("Failed to append CA certificate")
    }

    creds := credentials.NewClientTLSFromCert(caCertPool, "")

    conn, err := grpc.NewClient(
      "localhost:50051",
      grpc.WithTransportCredentials(creds),
    )

  3.3. Вариант с кастомной TLS-конфигурацией
    tlsConfig := &tls.Config{
      RootCAs: caCertPool,
      MinVersion: tls.VersionTLS13,
      ServerName: "localhost", // проверка имени сервера
    }
    creds := credentials.NewTLS(tlsConfig)

  3.4. Важный нюанс: ServerName
    При использовании TLS клиент должен проверить, что сертификат
    сервера соответствует ожидаемому имени хоста.
    Это делается через поле ServerName в tls.Config.

    Если сертификат выдан на домен example.com, а клиент подключается
    по IP, нужно либо указать ServerName, либо добавить IP в SAN.

  4.  ЧТО ТАКОЕ mTLS И ЗАЧЕМ ОН НУЖЕН
  mTLS (Mutual TLS) — это расширение TLS, при котором ОБЕ стороны
  предоставляют и проверяют сертификаты друг друга.

  В обычном TLS клиент проверяет сервер.
  В mTLS:
    • Клиент проверяет сервер (как в обычном TLS).
    • Сервер проверяет клиента (дополнительный шаг).

  Зачем это нужно:
    • Аутентификация клиента — сервер знает, кто именно подключился.
    • Zero Trust архитектура — ни одна сторона не доверяет другой
      без проверки.
    • Защита от подделки клиентов (например, если кто-то украл JWT,
      но не имеет сертификата).

  mTLS часто используется для:
    • Service-to-Service коммуникации в микросервисах.
    • Внутренних API (админки, мониторинг).
    • Систем с высокими требованиями к безопасности.

  mTLS обеспечивает взаимную аутентификацию, при которой и клиент,
  и сервер проверяют сертификаты друг друга, что позволяет реализовать
  zero-trust коммуникацию.

  5.  НАСТРОЙКА mTLS НА СЕРВЕРЕ

  5.1. Что нужно для настройки
    • Серверный сертификат и ключ.
    • CA-сертификат (или несколько) для проверки клиентских сертификатов.
    • Настройка ClientAuth: tls.RequireAndVerifyClientCert.

  5.2. Полный код сервера с mTLS
    func main() {
      // 1. Загружаем серверный сертификат и ключ
      serverCert, err := tls.LoadX509KeyPair(
        "certs/server_cert.pem",
        "certs/server_key.pem",
      )
      if err != nil {
        log.Fatalf("Failed to load server certificate: %v", err)
      }

      // 2. Загружаем CA-сертификат для проверки клиентов
      caCert, err := os.ReadFile("certs/ca_cert.pem")
      if err != nil {
        log.Fatalf("Failed to load CA certificate: %v", err)
      }

      clientCACertPool := x509.NewCertPool()
      if !clientCACertPool.AppendCertsFromPEM(caCert) {
        log.Fatalf("Failed to append CA certificate to pool")
      }

      // 3. Создаём TLS-конфиг с mTLS
      tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{serverCert},
        ClientCAs:    clientCACertPool,              // CA для проверки клиентов
        ClientAuth:   tls.RequireAndVerifyClientCert, // ОБЯЗАТЕЛЬНО: проверять клиентский сертификат
        MinVersion:   tls.VersionTLS13,               // принудительно TLS 1.3
      }

      // 4. Создаём gRPC-сервер с mTLS
      creds := credentials.NewTLS(tlsConfig)
      s := grpc.NewServer(grpc.Creds(creds))

      // ... регистрация сервисов и запуск
    }

  5.3. Вариант ClientAuth
    • tls.NoClientCert — клиентский сертификат не требуется (обычный TLS).
    • tls.RequestClientCert — запрашивать сертификат, но не проверять.
    • tls.RequireAnyClientCert — требовать любой сертификат (без проверки CA).
    • tls.RequireAndVerifyClientCert — требовать И проверять (настоящий mTLS).

  6.  НАСТРОЙКА mTLS НА КЛИЕНТЕ
  6.1. Что нужно для настройки
    • Клиентский сертификат и ключ.
    • CA-сертификат для проверки сервера.

  6.2. Полный код клиента с mTLS
    func main() {
      // 1. Загружаем клиентский сертификат и ключ
      clientCert, err := tls.LoadX509KeyPair(
        "certs/client_cert.pem",
        "certs/client_key.pem",
      )
      if err != nil {
        log.Fatalf("Failed to load client certificate: %v", err)
      }

      // 2. Загружаем CA-сертификат для проверки сервера
      caCert, err := os.ReadFile("certs/ca_cert.pem")
      if err != nil {
        log.Fatalf("Failed to load CA certificate: %v", err)
      }

      caCertPool := x509.NewCertPool()
      if !caCertPool.AppendCertsFromPEM(caCert) {
        log.Fatalf("Failed to append CA certificate")
      }

      // 3. Создаём TLS-конфиг с mTLS
      tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{clientCert}, // клиентский сертификат
        RootCAs:      caCertPool,                     // CA для проверки сервера
        MinVersion:   tls.VersionTLS13,
        ServerName:   "localhost",
      }

      // 4. Создаём gRPC-клиент с mTLS
      creds := credentials.NewTLS(tlsConfig)
      conn, err := grpc.NewClient(
        "localhost:50051",
        grpc.WithTransportCredentials(creds),
      )
      // ... использование клиента
    }

  7.  СРАВНЕНИЕ TLS vs mTLS
  ┌─────────────────────┬──────────────────────┬────────────────────────────┐
  │ Характеристика      │ TLS (односторонний)  │ mTLS (взаимный)            │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Шифрование          │ Да                   │ Да                         │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Аутентификация      │ Только сервера       │ Сервера И клиента          │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Клиентский сертификат│ Не требуется        │ Требуется                  │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Сложность настройки │ Низкая               │ Высокая                    │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Use case            │ Публичные API        │ Service-to-Service,        │
  │                     │                      │ Zero Trust, внутренние     │
  └─────────────────────┴──────────────────────┴────────────────────────────┘

  8.  BEST PRACTICES И ТИПИЧНЫЕ ОШИБКИ

  8.1. Best Practices
    Всегда используй TLS в продакшене.
    Используй TLS 1.2 или выше, отключай устаревшие версии.
    Используй mTLS для service-to-service коммуникации.
    Используй короткоживущие сертификаты (90 дней или меньше) с автоматической ротацией.
    Храни приватные ключи в безопасном месте (Vault, KMS, переменные окружения).
    В production отключай reflection (чтобы не светить API).
    Используй системное хранилище CA, если это возможно.
    Для самоподписанных сертификатов используй свой CA.

  8.2. Типичные ошибки
	• Использование insecure-соединений в продакшене. Это полностью ломает безопасность.
	• Хранение приватных ключей в репозитории. Утечка ключей = полная компрометация.
	• Игнорирование проверки ServerName. Клиент может подключиться к неверному серверу.
	• Использование слабых TLS-версий (SSLv3, TLS 1.0, 1.1). Уязвимости (POODLE, BEAST, Heartbleed).
	• Не использовать mTLS для внутренних сервисов. Клиент может быть подделан.
	• Не обновлять сертификаты (истекают → сервис падает). Автоматизируй ротацию.

  9.  КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ (БАЗА)
  1.  TLS — шифрование и аутентификация сервера. В продакшене ОБЯЗАТЕЛЕН.
  2.  mTLS — взаимная аутентификация: И клиент, И сервер предоставляют сертификаты.
  3.  TLS на сервере: grpc.Creds(credentials.NewServerTLSFromFile()).
  4.  TLS на клиенте: grpc.WithTransportCredentials(credentials.NewClientTLSFromCert()).
  5.  mTLS на сервере: tls.Config с ClientAuth: RequireAndVerifyClientCert и ClientCAs.
  6.  mTLS на клиенте: tls.Config с Certificates (клиентский сертификат) и RootCAs.
  7.  mTLS используется для service-to-service, Zero Trust, внутренних систем.
  8.  Используй TLS 1.2/1.3, отключай слабые протоколы.
  9.  Сертификаты должны быть короткоживущими с автоматической ротацией.
  10. Никогда не храни приватные ключи в репозитории.
  10. ПОЧЕМУ gRPC НЕ БЕЗОПАСЕН ПО УМОЛЧАНИЮ (И ЭТО ВАЖНО ЗНАТЬ)

  10.1. Факт
    gRPC в Go НЕ ЗАЩИЩЁН по умолчанию. При создании сервера через
    `grpc.NewServer()` он принимает НЕЗАШИФРОВАННЫЕ (plaintext) соединения.

    //НЕБЕЗОПАСНО — никакого TLS
    s := grpc.NewServer()

  10.2. Почему так сделано
    • Простота разработки — можно быстро запустить локально.
    • Гибкость — разработчик сам решает, когда включать шифрование.
    • Совместимость — в некоторых окружениях TLS может быть избыточен
      (например, внутри доверенной сети).

  10.3. Как это становится проблемой
    • В продакшене insecure-соединения — это КРАСНЫЙ ФЛАГ.
    • В Istio есть режим PERMISSIVE — сервер принимает и TLS, и plaintext.
      Это часто используют для миграции, но забывают выключить.
    • Если оставить PERMISSIVE, злоумышленник может подсунуть plaintext-запрос
      и обойти проверку сертификатов.

  11. TLS vs mTLS — ПРАКТИЧЕСКАЯ РАЗНИЦА

  11.1. Аналогия
    • TLS (односторонний): Клиент проверяет сервер.
      Это как ты проверяешь удостоверение у курьера, прежде чем открыть дверь.

    • mTLS (взаимный): И клиент, и сервер проверяют друг друга.
      Это как на секретном объекте, где и ты, и курьер должны показать пропуска.

  11.2. Почему mTLS — это не просто "круче"
    mTLS решает конкретную задачу — **аутентификацию клиента**.
    В микросервисной архитектуре ты должен быть уверен, что запрос
    пришёл от доверенного сервиса, а не от злоумышленника.

    mTLS — это рекомендованный механизм аутентификации для
    внутренних gRPC-коммуникаций (например, в документации gRPC).

  12. SERVICE MESH И АВТОМАТИЗАЦИЯ mTLS
  В мире Kubernetes и микросервисов mTLS редко настраивают вручную в коде.

  12.1. Что такое Service Mesh
    • Service Mesh — это инфраструктурный слой для управления взаимодействием между сервисами.
    • Популярные решения: Istio, Linkerd, Consul Connect.
    • Они автоматически управляют mTLS между сервисами в mesh.

  12.2. Как это работает в Istio
    • Istio может автоматически включать mTLS для всех сервисов в mesh.
    • Разработчику не нужно писать код для TLS — Istio перехватывает трафик
      и шифрует его.
    • Режим PERMISSIVE позволяет сервису принимать как TLS, так и plaintext
      (используется для миграции, но опасен, если не выключить).

  13. РЕАЛЬНЫЕ УЯЗВИМОСТИ (CVE) И ПОДВОДНЫЕ КАМНИ
  Сеньор знает не только "как сделать", но и "как сломать" и "что сломалось
  у других".

  13.1. CVE-2023-44487 (HTTP/2 Rapid Reset)
    • Уязвимость в HTTP/2, позволяющая злоумышленнику класть серверы
      через множество быстрых сбросов потоков.
    • Затронула все grpc-go сервисы, включая защищённые TLS.
    • Исправлена в обновлениях grpc-go.

  13.2. CVE-2023-32732
    • Уязвимость, связанная с неправильной обработкой бинарных метаданных
      в gRPC-Go.
    • Могла привести к DoS.

  13.3. Подводные камни
    • Сертификаты с истекшим сроком → сервис падает.
    • Автоматическая ротация сертификатов — обязательна.
    • Если сертификат отозван (CRL/OCSP), его нужно обновить.
    • Использование самоподписанных сертификатов в продакшене — риск.

  14. АЛЬТЕРНАТИВЫ TLS/mTLS
  mTLS — не единственный способ аутентификации и шифрования.

  14.1. ALTS (Application Layer Transport Security)
    • Разработан Google для своих облачных сервисов.
    • Аналог mTLS, но оптимизирован для Google Cloud / GKE.
    • Использует доверие на основе сервисных аккаунтов.
    • Менее распространён, но бывает в проектах на GCP.

  14.2. JWT + TLS
    • TLS обеспечивает шифрование (конфиденциальность).
    • JWT обеспечивает аутентификацию и авторизацию.
    • Это самый распространённый подход для публичных API:
      клиент передаёт JWT через metadata, сервер проверяет его.
    • mTLS — это "кто" (аутентификация сервиса).
    • JWT — это "кто" и "что можно" (аутентификация пользователя + права).

  15. СВЯЗКА mTLS С ИНТЕРСЕПТОРАМИ (AUTHORIZATION)
  Это логическое продолжение темы интерсепторов (Блок 3).

  15.1. Разделение ответственности
    • mTLS отвечает на вопрос "кто" (аутентификация клиента/сервиса).
    • Интерсептор отвечает на вопрос "что можно" (авторизация).

  15.2. Как это работает на практике
    // Сервер получает клиентский сертификат через mTLS
    // Из сертификата можно извлечь информацию о клиенте (CN, O, SAN)
    // Интерсептор проверяет, имеет ли клиент доступ к методу

    func AuthInterceptor() grpc.UnaryServerInterceptor {
        return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
            // Извлекаем информацию о клиенте из контекста (после mTLS)
            peer, ok := peer.FromContext(ctx)
            if !ok {
                return nil, status.Error(codes.Unauthenticated, "no peer info")
            }

            // Проверяем сертификат клиента (если есть)
            tlsInfo, ok := peer.AuthInfo.(credentials.TLSInfo)
            if !ok {
                return nil, status.Error(codes.Unauthenticated, "no TLS info")
            }

            // Извлекаем CommonName или SAN из сертификата
            if len(tlsInfo.State.PeerCertificates) == 0 {
                return nil, status.Error(codes.Unauthenticated, "no client certificate")
            }
            cert := tlsInfo.State.PeerCertificates[0]
            clientCN := cert.Subject.CommonName

            // Проверяем права клиента на доступ к методу
            if !hasPermission(clientCN, info.FullMethod) {
                return nil, status.Error(codes.PermissionDenied, "access denied")
            }

            // Добавляем информацию в контекст для бизнес-логики
            ctx = context.WithValue(ctx, "clientCN", clientCN)
            return handler(ctx, req)
        }
    }

  16. ЧЕК-ЛИСТ ВОПРОСОВ ДЛЯ СОБЕСЕДОВАНИЯ
  ┌─────────────────────┬─────────────────────────────────────────────────────┐
  │ Вопрос              │ Ответ (коротко)                                     │
  ├─────────────────────┼─────────────────────────────────────────────────────┤
  │ Безопасен ли gRPC   │ Нет, по умолчанию insecure, нужно включать TLS.     │
  │ по умолчанию?       │                                                     │
  ├─────────────────────┼─────────────────────────────────────────────────────┤
  │ В чём отличие TLS   │ TLS — только сервер, mTLS — и сервер, и клиент.     │
  │ от mTLS?            │                                                     │
  ├─────────────────────┼─────────────────────────────────────────────────────┤
  │ Зачем нужен mTLS    │ Аутентификация сервисов, Zero Trust, защита         │
  │ в микросервисах?    │ от подделки клиентов.                               │
  ├─────────────────────┼─────────────────────────────────────────────────────┤
  │ Как настроить mTLS  │ Использовать tls.Config с ClientAuth:               │
  │ на сервере?         │ RequireAndVerifyClientCert и ClientCAs.             │
  ├─────────────────────┼─────────────────────────────────────────────────────┤
  │ Как настроить mTLS  │ Использовать tls.Config с Certificates и RootCAs.   │
  │ на клиенте?         │                                                     │
  ├─────────────────────┼─────────────────────────────────────────────────────┤
  │ Что такое режим     │ Сервер принимает и TLS, и plaintext.                │
  │ PERMISSIVE в Istio? │ Опасен, если не выключить после миграции.           │
  ├─────────────────────┼─────────────────────────────────────────────────────┤
  │ Какие уязвимости    │ CVE-2023-44487 (HTTP/2 Rapid Reset),                │
  │ были в gRPC?        │ CVE-2023-32732 (обработка бинарных метаданных).     │
  ├─────────────────────┼─────────────────────────────────────────────────────┤
  │ Есть ли альтернатива│ ALTS (Google Cloud), JWT + TLS.                     │
  │ mTLS?               │                                                     │
  ├─────────────────────┼─────────────────────────────────────────────────────┤
  │ Как mTLS связан     │ mTLS — аутентификация (кто), интерсептор —          │
  │ с интерсепторами?   │ авторизация (что можно).                            │
  └─────────────────────┴─────────────────────────────────────────────────────┘
*/

//ХРАНИЛИЩЕ

type UserStore struct {
	mu    sync.RWMutex
	users map[string]*pb.User
}

func NewUserStore() *UserStore {
	return &UserStore{
		users: map[string]*pb.User{
			"1": {Id: "1", Name: "Alice", Email: "alice@ex.com"},
			"2": {Id: "2", Name: "Bob", Email: "bob@ex.com"},
			"3": {Id: "3", Name: "Charlie", Email: "charlie@ex.com"},
		},
	}
}

func (s *UserStore) Get(id string) (*pb.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

//СЕРВЕР

type UserServer struct {
	pb.UnimplementedUserServiceServer
	store *UserStore
}

// GetUser — unary RPC
func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Извлекаем информацию о клиенте из контекста (добавлена интерсептором)
	clientCN, ok := ctx.Value("clientCN").(string)
	if ok && clientCN != "" {
		log.Printf("Клиент %s запросил пользователя %s", clientCN, req.UserId)
	}

	user, err := s.store.Get(req.UserId)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return user, nil
}

// INTERCEPTOR ДЛЯ mTLS (АВТОРИЗАЦИЯ)
func AuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 1. Извлекаем информацию о пире (клиенте)
		peerInfo, ok := peer.FromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "no peer info")
		}

		// 2. Проверяем, что это TLS-соединение
		tlsInfo, ok := peerInfo.AuthInfo.(credentials.TLSInfo)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "not a TLS connection")
		}

		// 3. Извлекаем клиентский сертификат
		if len(tlsInfo.State.PeerCertificates) == 0 {
			return nil, status.Error(codes.Unauthenticated, "no client certificate")
		}
		cert := tlsInfo.State.PeerCertificates[0]

		// 4. Извлекаем CommonName из сертификата
		clientCN := cert.Subject.CommonName
		if clientCN == "" {
			return nil, status.Error(codes.Unauthenticated, "no CommonName in certificate")
		}

		// 5. Проверяем права доступа (для примера — только клиентам с CN=client-1)
		if clientCN != "client-1" {
			return nil, status.Error(codes.PermissionDenied, "access denied")
		}

		// 6. Добавляем CN в контекст для бизнес-логики
		ctx = context.WithValue(ctx, "clientCN", clientCN)

		log.Printf("Клиент %s аутентифицирован через mTLS", clientCN)

		// 7. Вызываем следующий обработчик
		return handler(ctx, req)
	}
}

// ФУНКЦИИ ЗАГРУЗКИ КРЕДЕНШЕЛОВ
// loadTLSCredentials — загружает серверный сертификат для обычного TLS
func loadTLSCredentials() (credentials.TransportCredentials, error) {
	serverCert, err := tls.LoadX509KeyPair(
		"certs/server_cert.pem",
		"certs/server_key.pem",
	)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS13,
	}
	return credentials.NewTLS(tlsConfig), nil
}

// loadMTLSCredentials — загружает серверный сертификат + CA для проверки клиентов
func loadMTLSCredentials() (credentials.TransportCredentials, error) {
	// 1. Загружаем серверный сертификат и ключ
	serverCert, err := tls.LoadX509KeyPair(
		"certs/server_cert.pem",
		"certs/server_key.pem",
	)
	if err != nil {
		return nil, err
	}

	// 2. Загружаем CA-сертификат для проверки клиентов
	caCert, err := os.ReadFile("certs/ca_cert/pem")
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	// 3. Создаём TLS-конфиг с mTLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}
	return credentials.NewTLS(tlsConfig), nil
}

func main() {
	// Парсим флаги для выбора режима TLS/mTLS
	// По умолчанию — mTLS
	mode := os.Getenv("TLS_MODE")
	if mode == "" {
		mode = "mtls"
	}

	var opts []grpc.ServerOption

	if mode == "mtls" {
		log.Println("Запуск сервера в режиме mTLS (взаимная аутентификация)")
		creds, err := loadMTLSCredentials()
		if err != nil {
			log.Fatalf("Failed to load mTLS credentials: %v", err)
		}
		opts = append(opts, grpc.Creds(creds))
		opts = append(opts, grpc.UnaryInterceptor(AuthInterceptor()))
	} else {
		log.Println("Запуск сервера в режиме TLS (односторонняя аутентификация)")
		creds, err := loadTLSCredentials()
		if err != nil {
			log.Fatalf("Failed to load TLS credentials: %v", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	// Создаём gRPC-сервер
	s := grpc.NewServer(opts...)

	// Регистрируем сервис
	store := NewUserStore()
	pb.RegisterUserServiceServer(s, &UserServer{store: store})

	// Запускаем слушатель
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("🚀 Сервер запущен на :50051")

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("⏳ Остановка сервера...")
	s.GracefulStop()
	log.Println("✅ Сервер остановлен")
}
