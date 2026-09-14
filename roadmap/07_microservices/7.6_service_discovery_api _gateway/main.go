package main

/*
  УРОК 7.6: SERVICE DISCOVERY И API GATEWAY
  В микросервисной архитектуре сервисы постоянно рождаются и умирают.
  Поды перезапускаются, инстансы масштабируются, ноды падают. Если
  ты захардкодил IP-адрес соседнего сервиса — ты обречён. Service
  Discovery решает вопрос "где сейчас находится сервис B?" для
  каждого межсервисного вызова. API Gateway решает вопрос "как
  внешний клиент попадает в мою систему?".
  Это две стороны одной медали. Discovery — про внутреннюю сетку
  (east-west traffic). Gateway — про вход снаружи (north-south
  traffic). Без первого система не масштабируется. Без второго
  клиенты видят хаос из сотни эндпоинтов.

  СОДЕРЖАНИЕ:
    1.  Проблема: почему hardcoded IP — это смерть
    2.  Service Discovery — что это и зачем
    3.  Три модели Service Discovery
    4.  Service Registry — сердце discovery
    5.  Consul, etcd, ZooKeeper — сравнение
    6.  Kubernetes Services — discovery из коробки
    7.  Health Checks — кто живой, кто мёртвый
    8.  API Gateway — входная дверь в систему
    9.  Edge vs Internal traffic — разные задачи
    10. Kong, Traefik, Ambassador — сравнение
    11. Интеграция API Gateway с Service Discovery
    12. Service Mesh — следующий уровень
    13. Антипаттерны
    14. Финальные выводы

  1. ПРОБЛЕМА: ПОЧЕМУ HARDCODED IP — ЭТО СМЕРТЬ
  Классический монолит. Все компоненты в одном процессе. Вызов
  между модулями — это вызов метода. Нет сети, нет IP, нет портов.
  Всё просто.

  Микросервисы ломают эту картину. Каждый сервис — отдельный
  процесс, часто на разных машинах. Чтобы вызвать соседа, нужно
  знать его адрес.

  Первое решение, которое приходит в голову — захардкодить:
    orderServiceURL = "http://10.0.0.5:8080"

  Это работает ровно до первого перезапуска. Контейнер поднялся
  с новым IP — всё сломалось. Инстанс упал — запросы летят
  в никуда. Добавили второй инстанс для нагрузки — клиент про
  него не знает.

  Можно вести конфиг-файл со списком адресов. Но кто его будет
  обновлять? Как часто? Что делать, если сервис упал в 3 часа
  ночи, а дежурный обновляет конфиг вручную?

  Service Discovery решает эту проблему. Он даёт сервисам
  логические имена вместо физических адресов. Клиент говорит
  "мне нужен order-service", а система сама находит живой инстанс.

  2. SERVICE DISCOVERY — ЧТО ЭТО И ЗАЧЕМ
  Service Discovery — это механизм, который позволяет сервисам
  находить друг друга динамически, без хардкода адресов.

  Два ключевых процесса:
    • SERVICE REGISTRATION — когда сервис стартует, он
      регистрирует себя в реестре: "я order-service, мой адрес
      10.0.0.5:8080, я здоров".
    • SERVICE DISCOVERY — когда клиенту нужен order-service,
      он спрашивает реестр: "дай мне живые инстансы order-service".
      Реестр возвращает список адресов.

  Реестр (Service Registry) — это центральная база данных,
  которая знает всё о всех сервисах. Это динамическая адресная
  книга для микросервисов. Она хранит hostname, IP, порт,
  метаданные и статус здоровья каждого инстанса.

  Ключевое свойство: реестр обновляется в реальном времени.
  Сервис упал — health check пометил его как unavailable.
  Новый инстанс поднялся — зарегистрировался. Клиенты всегда
  видят актуальную картину.

  3. ТРИ МОДЕЛИ SERVICE DISCOVERY
  Есть три принципиально разных подхода к тому, кто и как
  разрешает имя сервиса в адрес.

  3.1. CLIENT-SIDE DISCOVERY
  Клиент сам запрашивает реестр и сам выбирает инстанс.

    Client → Registry: "дай инстансы order-service"
    Registry → Client: [10.0.0.5:8080, 10.0.0.6:8080]
    Client → Instance 1: запрос

  Плюсы:
    • Нет дополнительного сетевого хопа. Клиент общается
      с сервисом напрямую.
    • Клиент сам выбирает алгоритм балансировки
      (round-robin, least-connections, random).
    • Меньше latency.

  Минусы:
    • Каждый язык программирования нуждается в зрелой
      клиентской библиотеке. Netflix Ribbon для Java — ок,
      но для Go, Python, Node.js — своя реализация.
    • Клиент должен знать про реестр и уметь с ним работать.
    • Логика discovery размазана по всем сервисам.

  Классический пример: Netflix Eureka + Ribbon. Netflix
  в итоге отказался от этого подхода в пользу Envoy.

  3.2. SERVER-SIDE DISCOVERY
  Клиент обращается к балансировщику (или API Gateway), который
  сам запрашивает реестр и форвардит запрос.

    Client → Load Balancer: "дай order-service"
    LB → Registry: "дай инстансы order-service"
    Registry → LB: [10.0.0.5:8080, 10.0.0.6:8080]
    LB → Instance 1: запрос
    Instance 1 → LB: ответ
    LB → Client: ответ

  Плюсы:
    • Клиент ничего не знает про реестр. Просто стучится
      в балансировщик по стабильному адресу.
    • Discovery-логика централизована.
    • Любой язык, любой клиент — работают одинаково.

  Минусы:
    • Дополнительный сетевой хоп. Latency растёт.
    • Балансировщик — единая точка отказа и потенциальное
      узкое место.
    • Нужно масштабировать сам балансировщик.

  Классический пример: Kubernetes Service + kube-proxy.
  AWS ELB/ALB.

  3.3. DNS-BASED DISCOVERY
  Самый простой подход. Реестр — это DNS-сервер. Клиент делает
  DNS-запрос, получает IP-адрес.
    Client → DNS: "order-service.internal"
    DNS → Client: 10.0.0.5

  Плюсы:
    • Работает из коробки. Любой клиент умеет использовать DNS.
    • Нет дополнительных библиотек.
    • Простота.
  Минусы:
    • DNS кэшируется. Если инстанс упал, клиент ещё
      какое-то время будет стучаться по старому IP.
    • Нет health-check на уровне DNS. Сервер может быть
      мёртв, но DNS всё ещё отдаёт его адрес.
    • Нет балансировки между инстансами на уровне DNS
      (если только не round-robin DNS, но это плохо
      работает с кэшированием).

  Решение: короткие TTL. Но слишком короткий TTL = нагрузка
  на DNS. Слишком длинный = stale данные.

  Kubernetes использует DNS-based discovery для Service:
  order-service.production.svc.cluster.local резолвится
  в ClusterIP. Но за этим стоит kube-proxy, который делает
  server-side discovery.

  4. SERVICE REGISTRY — СЕРДЦЕ DISCOVERY
  Реестр — это центральный компонент. От его надёжности зависит
  вся система. Если реестр недоступен, сервисы не могут найти
  друг друга. Это критическая точка.

  Что хранит реестр:
    • Имя сервиса (order-service).
    • Список инстансов с адресами (10.0.0.5:8080).
    • Метаданные (версия, регион, зона, weight).
    • Статус здоровья (healthy/unhealthy).
    • TTL регистрации (если сервис не обновляет — удаляется).

  Как сервис регистрируется:
    • SELF-REGISTRATION — сервис сам стучится в реестр
      при старте и обновляет heartbeat.
    • THIRD-PARTY REGISTRATION — отдельный компонент
      (например, Kubernetes) регистрирует поды.

  Что важно для реестра:
    • HIGH AVAILABILITY — реестр не должен падать.
      Обычно это кластер из 3-5 нод.
    • CONSISTENCY vs AVAILABILITY — CAP-теорема.
      Consul выбирает CP (consistency), Eureka — AP
      (availability).
    • WATCH MECHANISM — клиенты должны узнавать
      об изменениях. Обычно через long-polling или
      streaming.

  5. CONSUL, ETCD, ZOOKEEPER — СРАВНЕНИЕ
  Три классических реестра. Все три — распределённые
  key-value хранилища, но с разными акцентами.

  CONSUL (HashiCorp):
    • Что это: распределённый service registry + KV store
      + health checking + service mesh.
    • CAP: CP (consistency).
    • Discovery: DNS + HTTP API.
    • Health checks: встроенные, очень гибкие.
    • Multi-datacenter: из коробки.
    • Язык: Go.
    • Когда использовать: если нужен полноценный
      service discovery с health checks и multi-DC.

  ETCD:
    • Что это: распределённый KV store. Используется
      Kubernetes как backend для state.
    • CAP: CP.
    • Discovery: HTTP API + watch.
    • Health checks: нет встроенных. Нужно писать самому.
    • Multi-datacenter: сложнее, чем Consul.
    • Язык: Go.
    • Когда использовать: если уже есть etcd
      (например, в Kubernetes) и нужно минимальное
      решение.

  ZOOKEEPER:
    • Что это: распределённый coordination service.
      Старожил, использовался в Hadoop, Kafka.
    • CAP: CP.
    • Discovery: через znodes.
    • Health checks: нет встроенных.
    • Multi-datacenter: сложно.
    • Язык: Java.
    • Когда использовать: если уже есть ZK-инфраструктура.
      Для новых проектов — редко.

  Сводная таблица:
    ┌──────────────┬──────────┬──────────┬──────────────┐
    │              │ Consul   │ etcd     │ ZooKeeper    │
    ├──────────────┼──────────┼──────────┼──────────────┤
    │ CAP          │ CP       │ CP       │ CP           │
    │ DNS          │ Да       │ Нет      │ Нет          │
    │ HTTP API     │ Да       │ Да       │ Нет          │
    │ Health check │ Встроен  │ Нет      │ Нет          │
    │ Multi-DC     │ Да       │ Сложно   │ Сложно       │
    │ Язык         │ Go       │ Go       │ Java         │
    │ Сложность    │ Средняя  │ Низкая   │ Высокая      │
    └──────────────┴──────────┴──────────┴──────────────┘

  6. KUBERNETES SERVICES — DISCOVERY ИЗ КОРОБКИ
  В Kubernetes тебе не нужен Consul или etcd для discovery.
  Всё уже есть. Service — это абстракция, которая даёт
  стабильный IP и DNS-имя для группы подов.

  Как это работает:
    1. Ты создаёшь Deployment с 3 репликами order-service.
    2. Ты создаёшь Service с селектором app=order-service.
    3. Kubernetes создаёт ClusterIP (виртуальный IP).
    4. kube-proxy на каждой ноде настраивает iptables/IPVS
       для форвардинга трафика с ClusterIP на реальные поды.
    5. DNS-запись order-service.production.svc.cluster.local
       резолвится в ClusterIP.

  Клиент внутри кластера просто использует имя сервиса:
    http://order-service.production.svc.cluster.local:8080

  Или коротко, если в том же namespace:
    http://order-service:8080

  Типы Service:
    • CLUSTERIP — внутренний IP, доступен только внутри
      кластера. Дефолт. Для внутренней коммуникации.
    • NODEPORT — открывает порт на каждой ноде. Доступен
      снаружи по IP ноды и порту (30000-32767). Неудобно,
      но работает для отладки.
    • LOADBALANCER — создаёт внешний load balancer
      (в облаке — ELB, ALB, Azure LB). Даёт публичный IP.
      Дорого, по одному LB на сервис.
    • EXTERNALNAME — маппит сервис на внешний DNS.
      Не проксирует трафик.

  Ключевое: Kubernetes делает server-side discovery через
  kube-proxy. Клиент не знает про поды, он стучится
  в ClusterIP.

  7. HEALTH CHECKS — КТО ЖИВОЙ, КТО МЁРТВЫЙ
  Service Discovery бесполезен, если он отдаёт адреса мёртвых
  инстансов. Health checks — обязательная часть.

  Три типа проверок:
    • LIVENESS PROBE — "ты ещё жив?". Если нет — Kubernetes
      перезапускает под. Проверяет, что процесс не завис.
    • READINESS PROBE — "ты готов принимать трафик?".
      Если нет — Kubernetes убирает под из Service endpoints.
      Трафик не идёт на этот под.
    • STARTUP PROBE — "ты уже стартовал?". Для медленно
      стартующих приложений. Пока не пройдёт — liveness
      и readiness не проверяются.

  В Consul health checks настраиваются гибко:
    • HTTP check — GET /health, ожидаем 200.
    • TCP check — просто открыт ли порт.
    • TTL check — сервис сам пингует Consul каждые N секунд.
    • Script check — кастомная логика.

  Что важно:
    • Health check должен быть ЛЁГКИМ. Не делай тяжёлых
      запросов в /health.
    • Readiness должен проверять зависимости (БД, кэш).
      Если БД недоступна — сервис не готов.
    • Liveness должен проверять только сам процесс.
      Если БД упала — не надо перезапускать под.
    • Не путай их. Перепутаешь — получишь каскадные
      перезапуски.

  8. API GATEWAY — ВХОДНАЯ ДВЕРЬ В СИСТЕМУ
  API Gateway — это сервер, который стоит на входе в систему
  и обрабатывает все внешние запросы. Это единственная точка
  входа для клиентов.

  Зачем он нужен:
    • ЕДИНАЯ ТОЧКА ВХОДА. Клиент не знает про 50 микросервисов.
      Он знает про один API Gateway. Проще для клиента,
      проще для безопасности.
    • РОУТИНГ. Gateway решает, какой запрос в какой сервис
      направить. /api/orders/* → order-service,
      /api/users/* → user-service.
    • АУТЕНТИФИКАЦИЯ И АВТОРИЗАЦИЯ. JWT-токены проверяются
      на Gateway. Микросервисы не думают про auth.
    • RATE LIMITING. Ограничение запросов на клиента.
      Защита от DDoS и abuse.
    • SSL TERMINATION. HTTPS расшифровывается на Gateway.
      Внутри — HTTP. Проще управлять сертификатами.
    • КЭШИРОВАНИЕ. GET-запросы кэшируются на Gateway.
      Снижение нагрузки на бэкенд.
    • ЛОГИРОВАНИЕ И МОНИТОРИНГ. Все запросы логируются
      в одном месте.
    • ТРАНСФОРМАЦИЯ. Можно менять формат запроса/ответа.
      Versioning API (v1 → v2).

  API Gateway — это reverse proxy с дополнительными
  возможностями. По сути, это L7-балансировщик с
  бизнес-логикой.

  9. EDGE VS INTERNAL TRAFFIC — РАЗНЫЕ ЗАДАЧИ
  Это ключевое различие, которое часто путают.

  NORTH-SOUTH TRAFFIC (edge):
    • Трафик между внешним миром и системой.
    • Клиенты → API Gateway → сервисы.
    • Задачи: auth, rate limiting, SSL, routing,
      versioning, кэширование.
    • Клиент не под твоим контролем.
    • API Gateway управляет этим трафиком.

  EAST-WEST TRAFFIC (internal):
    • Трафик между сервисами внутри системы.
    • order-service → payment-service.
    • Задачи: discovery, load balancing, retries,
      circuit breaking, mTLS, tracing.
    • Клиент — твой собственный сервис.
    • Service Mesh управляет этим трафиком.

  Разница фундаментальная:
    • В north-south ты не контролируешь клиента. Он может
      быть старым, медленным, злонамеренным.
    • В east-west обе стороны — твои сервисы. Ты можешь
      требовать определённые библиотеки, протоколы,
      поведение.

  API Gateway НЕ должен управлять east-west трафиком.
  Это не его работа. Для east-west нужен service mesh
  или client-side discovery.

  Service Mesh — это следующий уровень. Это слой
  инфраструктуры, который управляет east-west трафиком.
  Он вставляет sidecar-прокси (Envoy, Linkerd) рядом
  с каждым подом. Весь трафик идёт через прокси.

  Service Mesh даёт:
    • mTLS между сервисами (шифрование, аутентификация).
    • Retries, timeouts, circuit breaking на уровне сети.
    • Distributed tracing.
    • Traffic splitting (canary, blue-green).
    • Observability.

  Но у него высокая цена: sidecar'ы потребляют CPU
  и память. Istio sidecar — ~0.20 vCPU и 60 MB RAM
  на под при 1000 RPS. Не внедряй mesh, пока у тебя
  меньше 20 сервисов и нет реальной потребности.

  10. KONG, TRAEFIK, AMBASSADOR — СРАВНЕНИЕ
  Три популярных API Gateway. Каждый со своим характером.

  KONG:
    • Построен на NGINX + Lua.
    • Plugin-based архитектура. Сотни плагинов
      для auth, rate limiting, logging, transformations.
    • Admin REST API для динамической конфигурации.
    • Работает как standalone, так и как Kubernetes
      Ingress Controller.
    • Большое сообщество.
    • Enterprise-версия с поддержкой.
    • Когда выбирать: если нужна богатая
      плагинная экосистема и API management.

  TRAEFIK:
    • Написан на Go.
    • Автоматический service discovery. Видит Docker,
      Kubernetes, Consul, etcd и автоматически
      настраивает роутинг.
    • Встроенный Let's Encrypt.
    • Красивый dashboard.
    • Простой в настройке.
    • Отлично работает с Docker.
    • Когда выбирать: если нужна простота и
      автоматическое обнаружение сервисов.

  AMBASSADOR:
    • Построен на Envoy.
    • Kubernetes-native. Конфигурация через CRD
      (Custom Resource Definitions).
    • Declarative YAML.
    • Фокус на developer self-service.
    • Хорошо интегрируется с GitOps.
    • Commercial версия — Ambassador Edge Stack.
    • Когда выбирать: если ты уже в Kubernetes
      и хочешь Envoy-based gateway с декларативной
      конфигурацией.

  Сводная таблица:
    ┌──────────────┬──────────┬──────────┬──────────────┐
    │              │ Kong     │ Traefik  │ Ambassador   │
    ├──────────────┼──────────┼──────────┼──────────────┤
    │ База         │ NGINX    │ Go       │ Envoy        │
    │ Конфиг       │ REST API │ Auto +   │ CRD / YAML   │
    │              │          │ файлы    │              │
    │ K8s-native   │ Частично │ Да       │ Да           │
    │ Service      │ Плагины  │ Да       │ Через Envoy  │
    │ discovery    │          │          │              │
    │ Let's        │ Плагин   │ Встроен  │ Через Envoy  │
    │ Encrypt      │          │          │              │
    │ Сложность    │ Средняя  │ Низкая   │ Средняя      │
    │ Экосистема   │ Огромная │ Средняя  │ Растущая     │
    └──────────────┴──────────┴──────────┴──────────────┘

  11. ИНТЕГРАЦИЯ API GATEWAY С SERVICE DISCOVERY
  API Gateway не знает, где находятся сервисы. Он должен
  спросить у Service Discovery.

  Поток:
    1. Клиент → API Gateway: GET /api/orders
    2. API Gateway → Service Registry: "где order-service?"
    3. Registry → API Gateway: [10.0.0.5:8080, 10.0.0.6:8080]
    4. API Gateway → Instance 1: форвардит запрос
    5. Instance 1 → API Gateway: ответ
    6. API Gateway → Клиент: ответ

  Без service discovery API Gateway пришлось бы вручную
  конфигурировать с адресами всех сервисов. Каждый
  перезапуск — ручное обновление. Кошмар.

  С discovery gateway динамически получает актуальные
  адреса. Сервис упал — gateway узнаёт и перестаёт
  слать туда трафик. Новый инстанс — gateway видит его.

  В Kubernetes это работает из коробки. Ingress Controller
  (тот же Traefik, Kong, Ambassador) смотрит в Kubernetes
  API и автоматически создаёт маршруты для Service.

  12. SERVICE MESH — СЛЕДУЮЩИЙ УРОВЕНЬ
  Service Mesh — это инфраструктурный слой для управления
  east-west трафиком. Он вставляет sidecar-прокси рядом
  с каждым подом. Весь трафик между сервисами идёт через
  прокси.

  Что даёт mesh:
    • mTLS — взаимная аутентификация и шифрование
      между сервисами.
    • RETRIES, TIMEOUTS, CIRCUIT BREAKING — на уровне
      сети, без изменения кода.
    • TRAFFIC SPLITTING — canary, blue-green.
    • OBSERVABILITY — метрики, трейсинг, логи
      для каждого запроса.
    • DISCOVERY — sidecar знает адреса других сервисов.

  Популярные mesh:
    • ISTIO — самый мощный, самый сложный. Envoy sidecar.
    • LINKERD — проще, легче. Rust sidecar.
    • CILIUM SERVICE MESH — eBPF-based, без sidecar.
      Самый производительный.

  Когда НЕ нужен mesh:
    • Меньше 20 сервисов.
    • Нет требований к mTLS.
    • Нет команды, которая может поддерживать
      mesh-инфраструктуру.

  Если нужен только mTLS — cert-manager +
  NetworkPolicies дешевле.

  13. АНТИПАТТЕРНЫ

  13.1. HARDCODED IP-АДРЕСА
    Самая частая ошибка. Работает до первого перезапуска.
    Всегда используй discovery.
  13.2. API GATEWAY КАК ЕДИНАЯ ТОЧКА ОТКАЗА
    Если gateway упал — система недоступна. Нужно
    минимум 2-3 инстанса за load balancer.
  13.3. GATEWAY ДЛЯ EAST-WEST ТРАФИКА
    API Gateway не должен управлять внутренней
    коммуникацией. Для этого есть service mesh
    или client-side discovery.
  13.4. HEALTH CHECK, КОТОРЫЙ ВСЕГДА ВОЗВРАЩАЕТ 200
    Бесполезен. Должен проверять реальные зависимости.
  13.5. СЛИШКОМ КОРОТКИЙ TTL В DNS
    Нагрузка на DNS. Слишком длинный — stale данные.
    Балансируй.
  13.6. SERVICE MESH БЕЗ ПОТРЕБНОСТИ
    Sidecar'ы дорогие. Не внедряй mesh, пока не
    упрёшься в реальные проблемы.
  13.7. ЗАБЫЛИ ПРО HEALTH CHECKS
    Discovery отдаёт мёртвые инстансы. Клиенты
    получают ошибки.

  14. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Hardcoded IP — смерть. Service Discovery обязателен
      для любого динамического окружения.
  2.  Service Registry — центральный компонент. Должен
      быть highly available.
  3.  Client-side discovery — быстрее, но требует
      библиотек на каждом языке. Server-side — проще,
      но добавляет хоп.
  4.  Kubernetes даёт discovery из коробки через Service
      и DNS. Для большинства случаев этого достаточно.
  5.  Health checks — обязательны. Без них discovery
      отдаёт мёртвые инстансы.
  6.  API Gateway — входная дверь для north-south трафика.
      Auth, rate limiting, routing, SSL.
  7.  Service Mesh — для east-west трафика. mTLS,
      retries, observability.
  8.  Не путай edge и internal traffic. Это разные задачи
      с разными инструментами.
  9.  Kong — для богатой плагинной экосистемы. Traefik —
      для простоты и авто-discovery. Ambassador — для
      Kubernetes-native.
  10. Service Mesh нужен, когда у тебя 20+ сервисов
      и реальная потребность в mTLS и observability.
      Не раньше.
  11. API Gateway должен динамически получать адреса
      из Service Discovery. Ручная конфигурация — зло.
  12. Начинай с Kubernetes Service. Если мало — добавь
      API Gateway (Ingress). Если всё ещё мало — думай
      про Service Mesh.
*/
