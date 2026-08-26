package balance

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	pb "github.com/"
)

/*
  РАЗДЕЛ 4.5: БАЛАНСИРОВКА НАГРУЗКИ (CLIENT-SIDE + DNS)
  Балансировка нагрузки — это механизм распределения запросов между несколькими
  экземплярами сервиса. В мире микросервисов это КРИТИЧЕСКИ ВАЖНЫЙ компонент
  для обеспечения отказоустойчивости, масштабируемости и высокой доступности.

  В этом разделе мы разберём:
    1. Что такое балансировка нагрузки и зачем она нужна в gRPC
    2. Client-side vs Server-side балансировка
    3. Политика round_robin — как работает и когда использовать
    4. Включение round_robin на клиенте в Go
    5. Service Discovery через DNS — как это работает
    6. DNS резолвер в grpc-go
    7. Кастомные резолверы (etcd, Consul) — концепция
    8. Best practices и типичные ошибки
    9. Ключевые выводы для собеседования

  1.  ЧТО ТАКОЕ БАЛАНСИРОВКА НАГРУЗКИ И ЗАЧЕМ ОНА НУЖНА В GRPC
  Балансировка нагрузки (load balancing) — это процесс распределения
  входящих RPC-запросов между несколькими бэкендами (серверами).

  Зачем это нужно в gRPC:

    • Масштабируемость — при росте нагрузки добавляются новые экземпляры.
    • Отказоустойчивость — если один экземпляр упал, трафик перенаправляется
      на другие.
    • Равномерное распределение — чтобы ни один сервер не был перегружен.
    • Zero-downtime deployments — при обновлении трафик плавно переключается
      на новые версии.

  Без балансировки:
    • Один сервер может быть перегружен, а другие простаивать.
    • При падении сервера клиенты получают ошибки.
    • Нельзя масштабироваться горизонтально.

  gRPC изначально проектировался для работы в распределённых системах,
  поэтому имеет встроенные механизмы балансировки и service discovery.

  2.  CLIENT-SIDE VS SERVER-SIDE БАЛАНСИРОВКА
  В gRPC есть два подхода к балансировке: client-side и server-side.

  2.1. Client-side балансировка
    Клиент сам знает список всех доступных серверов и выбирает, к кому
    отправить запрос. Это подход по умолчанию в gRPC.

    Преимущества:
      • Нет дополнительного hop (прокси) — меньше задержка.
      • Клиент может использовать локальную информацию (загрузка, география).
      • Не требует дополнительной инфраструктуры.

    Недостатки:
      • Клиент должен знать список всех серверов (service discovery).
      • При добавлении/удалении серверов клиент должен обновлять список.

  2.2. Server-side балансировка
    Клиент отправляет запрос на балансировщик (обычно прокси), а тот
    перенаправляет запрос на один из серверов.

    Преимущества:
      • Клиенту не нужно знать о серверах — только адрес балансировщика.
      • Балансировщик может применять сложные политики.

    Недостатки:
      • Дополнительный hop — увеличивает задержку.
      • Балансировщик становится единой точкой отказа.

    В gRPC часто используется client-side балансировка через DNS или
    service discovery (etcd, Consul). Это эффективно и децентрализованно.


  3.  ПОЛИТИКА ROUND_ROBIN — КАК РАБОТАЕТ И КОГДА ИСПОЛЬЗОВАТЬ
  Политика round_robin — это самый простой и распространённый алгоритм
  балансировки в gRPC. Запросы распределяются по кругу между доступными
  серверами.

  3.1. Как работает round_robin
    Есть список серверов: [A, B, C, D]
    Каждый запрос отправляется на следующий сервер по кругу:
    Запрос 1 → A
    Запрос 2 → B
    Запрос 3 → C
    Запрос 4 → D
    Запрос 5 → A (снова)
    Запрос 6 → B (и так далее)
    Это гарантирует равномерное распределение запросов.

  3.2. Когда использовать round_robin
    + Когда все серверы имеют примерно одинаковую мощность.
    + Когда запросы примерно одинакового размера.
    + Для простых микросервисов без сложных требований.

  3.3. Другие политики балансировки
    • pick_first — всегда выбирается первый доступный сервер
      (по умолчанию в grpc-go).
    • weighted_round_robin — серверы имеют разные веса.
    • least_loaded — выбирается сервер с наименьшей нагрузкой.
    • consistent_hash — один клиент всегда попадает на один сервер
      (для кеширования, сессий).

  Для использования других политик может потребоваться установка
  дополнительных расширений (например, xds).

  4.  ВКЛЮЧЕНИЕ ROUND_ROBIN НА КЛИЕНТЕ В GO

  4.1. Базовое включение (grpc-go v1.30+)
    import (
      "google.golang.org/grpc"
      "google.golang.org/grpc/credentials/insecure"
    )

    conn, err := grpc.NewClient(
      "dns:///my-service:50051",
      grpc.WithTransportCredentials(insecure.NewCredentials()),
      grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
    )
    if err != nil {
      log.Fatal(err)
    }
    defer conn.Close()

    client := pb.NewUserServiceClient(conn)

  4.2. Для старых версий grpc-go (до v1.30)
    // В grpc-go v1.30+ это упрощено, но для старых версий:
    conn, err := grpc.Dial(
      "dns:///my-service:50051",
      grpc.WithBalancerName("round_robin"),
      // ...
    )

  4.3. Важный нюанс: форматы адресов
    При использовании round_robin нужно указывать адрес в формате
    с резолвером:

    • "dns:///my-service:50051" — использовать DNS резолвер.
    • "localhost:50051" — без резолвера (только один адрес, балансировки нет).

  Резолвер "dns:///" говорит grpc-go, что нужно использовать DNS
  для получения списка IP-адресов.

  4.4. Проверка, что round_robin работает
    Чтобы убедиться, что балансировка работает:
      • Запустить 2+ экземпляра сервера.
      • Клиент отправляет 10+ запросов.
      • В логах серверов видно, что запросы распределяются.

    Или используй grpcurl с verbose режимом:
      grpcurl -plaintext -v -d '{"user_id":"1"}' \
        dns:///my-service:50051 user.v1.UserService/GetUser

  В grpc-go есть много разных политик балансировки, но round_robin
  — самая простая и часто используется.

  Политика по умолчанию в grpc-go — pick_first, но вы можете настроить
  round_robin через сервисную конфигурацию. Однако для корректной работы
  необходимо использовать резолвер имён для получения адресов бэкендов.

  5.  SERVICE DISCOVERY ЧЕРЕЗ DNS — КАК ЭТО РАБОТАЕТ
  Service Discovery — это механизм, который позволяет клиенту узнавать
  список доступных серверов в рантайме. Самый простой способ — DNS.

  5.1. Как работает DNS service discovery
    Клиент указывает адрес в формате dns:///my-service:50051.
    grpc-go периодически (по умолчанию каждые 30 секунд) делает DNS-запрос
    к my-service и получает список IP-адресов (A-записи).

    Если за адресом my-service скрывается несколько IP-адресов (DNS round-robin),
    grpc-go получит список всех IP и будет балансировать между ними.

    Если серверов несколько, grpc-go автоматически обновляет список
    при изменении DNS-записей.

  5.2. Формат DNS резолвера
    • dns:///service:port — стандартный DNS (для локального окружения).
    • dns:///service:port?balancer=round_robin — передача параметров.
    • dns:///service — порт по умолчанию 443.

  5.3. Зачем нужен DNS резолвер в gRPC
    • Клиент автоматически получает список всех доступных серверов.
    • При добавлении/удалении серверов клиент узнаёт об этом.
    • Упрощает конфигурацию — не нужно хардкодить IP-адреса.
    • Это базовый механизм, который работает везде.

  Резолвер в grpc-go преобразует URI "dns:///service:port" в список адресов.
  Затем балансировщик использует эти адреса для распределения нагрузки.

  5.4. Период обновления DNS
    grpc-go использует системный TTL (Time To Live) DNS-записей.
    Также можно настроить интервал обновления через опции.

    // Создаём резолвер с кастомным интервалом
    resolverBuilder := &dns.ResolverBuilder{
      MinFreq: 5 * time.Second, // минимальный интервал обновления
    }
    resolver.Register(resolverBuilder)

  6.  DNS РЕЗОЛВЕР В GRPC-GO — ПОДРОБНО

  6.1. Подключение резолвера по умолчанию
    При импорте пакета google.golang.org/grpc резолвер регистрируется
    автоматически. Для DNS используется схема dns.

  6.2. Импорт резолвера явно (если нужно)
    import _ "google.golang.org/grpc/resolver"

    // или для конкретного резолвера
    import _ "google.golang.org/grpc/resolver/dns"

  6.3. Пример с кастомным резолвером
    // Создаём резолвер
    resolverBuilder := dns.NewBuilder(dns.ResolverConfig{
      MinFreq: 10 * time.Second,
    })
    resolver.Register(resolverBuilder)

    // Используем в клиенте
    conn, err := grpc.NewClient(
      "dns:///my-service:50051",
      grpc.WithResolvers(resolverBuilder),
    )

  6.4. Настройка балансировки через Service Config
    // Полная конфигурация через Service Config
    serviceConfig := `{
      "loadBalancingPolicy": "round_robin",
      "methodConfig": [
        {
          "name": [
            { "service": "user.v1.UserService" },
            { "service": "user.v1.UserService", "method": "GetUser" }
          ],
          "timeout": "5s"
        }
      ]
    }`

    conn, err := grpc.NewClient(
      "dns:///my-service:50051",
      grpc.WithDefaultServiceConfig(serviceConfig),
    )

  6.5. Поддержка SRV записей
    grpc-go не использует SRV-записи по умолчанию, но их можно использовать
    через кастомный резолвер.

    Пример SRV записи:
      _grpc._tcp.my-service IN SRV 10 5 50051 server1.example.com.

    Обычно SRV используется для сервисов с несколькими портами или
    для указания приоритетов.

  7.  КАСТОМНЫЕ РЕЗОЛВЕРЫ (ETCD, CONSUL, ZOOKEEPER) — КОНЦЕПЦИЯ
  DNS работает, но у него есть недостатки:
    • TTL может быть большим (до 5 минут) — клиент долго не узнаёт
      об изменениях.
    • Нельзя передавать метаданные о серверах (вес, зона).
    • Нет поддержки сложных стратегий обнаружения.
  Для продакшен-систем часто используют кастомные резолверы.

  7.1. Когда нужен кастомный резолвер
    + TTL слишком большой для ваших нужд.
    + Нужно передавать метаданные (вес сервера, версия, зона).
    + Нужно динамически обновлять список серверов (через API).
    + Сервера регистрируются через Service Discovery (etcd, Consul).

  7.2. Пример с etcd (концептуально)
    // Регистрация сервера в etcd
    client.Put(ctx, "/services/user-service/1", "192.168.1.10:50051")
    client.Put(ctx, "/services/user-service/2", "192.168.1.11:50051")

    // Клиентский резолвер, который читает из etcd
    type etcdResolver struct {
      client *etcd.Client
      // ...
    }

    func (r *etcdResolver) ResolveNow() {
      // Получаем список серверов из etcd
      resp, _ := r.client.Get(ctx, "/services/user-service/", clientv3.WithPrefix())
      addresses := []resolver.Address{}
      for _, kv := range resp.Kvs {
        addresses = append(addresses, resolver.Address{Addr: string(kv.Value)})
      }
      // Обновляем состояние резолвера
      r.w.UpdateState(resolver.State{Addresses: addresses})
    }

  7.3. Популярные решения
    • etcd — часто используется в Kubernetes-среде.
    • Consul — интеграция с service discovery из коробки.
    • ZooKeeper — классика для распределённых систем.

  В экосистеме grpc-ecosystem есть готовые реализации для etcd и consul.
  Стандартный резолвер для gRPC — это DNS резолвер, но вы можете заменить
  его на кастомный резолвер, который использует etcd, Consul или любую
  другую систему обнаружения сервисов (service discovery).

  8.  BEST PRACTICES И ТИПИЧНЫЕ ОШИБКИ

  8.1. Best Practices
    + Всегда используй клиентскую балансировку для микросервисов.
    + Используй round_robin как политику по умолчанию.
    + Используй DNS резолвер с TTL не более 30 секунд для быстрого
      обновления.
    + Для production используй service discovery (etcd, Consul) вместо DNS.
    + Всегда проверяй, что балансировка работает (по логам или метрикам).
    + Используй health checks для автоматического исключения нездоровых
      серверов из балансировки.
    + Для Kubernetes используй headless service для service discovery.

  8.2. Типичные ошибки
    - Не указывать dns:/// перед адресом при использовании round_robin.
      Решение: "dns:///my-service:50051", а не "my-service:50051".
    - Использовать round_robin с одним сервером (бесполезно).
      Решение: иметь минимум 2 экземпляра сервера.
    - Не учитывать TTL DNS записей (5-10 минут) при изменении серверов.
      Решение: настроить TTL на короткое время (30-60 секунд).
    - Использовать DNS без round_robin (по умолчанию pick_first).
      Решение: явно указывать политику.
    - Не использовать health checks вместе с балансировкой.
      Решение: всегда добавлять health checks для исключения нездоровых
      серверов.
    - Подключаться к балансировщику без резолвера (статический адрес).
      Решение: использовать DNS или service discovery.

  9.  КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1.  Балансировка нагрузки распределяет запросы между несколькими серверами.
  2.  Client-side балансировка — клиент сам выбирает сервер (подход gRPC).
  3.  Server-side балансировка — прокси/балансировщик перенаправляет запросы.
  4.  round_robin — простая политика распределения по кругу.
  5.  В grpc-go: grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`).
  6.  Для round_robin нужен резолвер: "dns:///my-service:50051".
  7.  DNS резолвер автоматически получает список IP-адресов.
  8.  TTL DNS влияет на скорость обновления списка серверов.
  9.  Для production используют кастомные резолверы (etcd, Consul).
  10. Health checks исключают нездоровые серверы из балансировки.
  11. В Kubernetes используют headless service для service discovery.
  12. round_robin — самая распространённая политика в gRPC.
*/

// КОНФИГУРАЦИЯ
var (
	port     = flag.Int("ports", 50051, "The port to listen on")
	serverID = flag.String("id", "server-default", "The server id")
)

//ХРАНИЛИЩЕ С КЭШЕМ

type UserStore struct {
	mu    sync.RWMutex
	users map[string]*pb.User
	cache map[string]*pb.User // имитация кэша
	hit   int64
	miss  int64
}

func NewUserStore() *UserStore {
	return &UserStore{
		users: map[string]*pb.User{
			"1": {Id: "1", Name: "Alice", Email: "alice@ex.com"},
			"2": {Id: "2", Name: "Bob", Email: "bob@ex.com"},
			"3": {Id: "3", Name: "Charlie", Email: "charlie@ex.com"},
			"4": {Id: "4", Name: "Diana", Email: "diana@ex.com"},
			"5": {Id: "5", Name: "Eve", Email: "eve@ex.com"},
		},
		cache: make(map[string]*pb.User),
	}
}

func (s *UserStore) Get(id string) (*pb.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Проверяем кэш
	if u, ok := s.cache[id]; ok {
		atomic.AddInt64(&s.hit, 1)
		return u, true
	}

	// Ищем в БД
	u, ok := s.users[id]
	if ok {
		s.cache[id] = u
	}
	atomic.AddInt64(&s.miss, 1)
	return u, ok
}

func (s *UserStore) Create(name, email string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := fmt.Sprintf("%d", len(s.users)+1)
	u := &pb.User{Id: id, Name: name, Email: email}
	s.users[id] = u
	s.cache[id] = u
	return id
}

func (s *UserStore) Stats() (hit, miss int64) {
	return atomic.LoadInt64(&s.hit), atomic.LoadInt64(&s.miss)
}

// СЕРВЕР
type Server struct {
	pb.UnimplementedUserServiceServer
	store     *UserStore
	id        string
	port      int
	reqCount  int64
	startTime time.Time
}

func (s *Server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	atomic.AddInt64((&s.reqCount), 1)

	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// Имитация разной нагрузки (рандомная задержка)
	time.Sleep(time.Duration(5+time.Now().UnixNano()%20) * time.Millisecond)

	user, ok := s.store.Get(req.Id)
	if !ok {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	user.ProcessedBy = fmt.Sprintf("%s:%d", s.id, s.port)

	log.Printf("[%s] GetUser: id=%s (кэш: hit=%d, miss=%d)",
		s.id, req.Id, atomic.LoadInt64(&s.store.hit), atomic.LoadInt64(&s.store.miss))

	return user, nil
}

func (s *Server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
	atomic.AddInt64(&s.reqCount, 1)

	if req.Name == "" || req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "name and email are required")
	}

	id := s.store.Create(req.Name, req.Email)
	user, _ := s.store.Get(id)
	user.ProcessedBy = fmt.Sprintf("%s:%d", s.id, s.port)

	log.Printf("[%s] CreateUser: id=%s, name=%s", s.id, id, req.Name)
	return user, nil
}

func main() {
	flag.Parse()

	store := NewUserStore()
	srv := &Server{
		store:     store,
		id:        *serverID,
		port:      *port,
		startTime: time.Now(),
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":d", *port))
	if err != nil {
		log.Fatalf("[%s] failed to listen: %v", *serverID, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcServer, srv)

	// Health Check
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	log.Printf("[%s] server started on :%d", *serverID, *port)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("[%s] serve error: %v", *serverID, err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("[%s] shutting down...", *serverID)

	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	time.Sleep(2 * time.Second)

	grpcServer.GracefulStop()
	log.Printf("[%s] stopped", *serverID)
}
