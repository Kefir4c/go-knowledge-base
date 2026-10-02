package main

/*
  УРОК 3.1: SERVICE — ТИПЫ
  Pod'ы эфемерны. Упал — пересоздался с новым именем и новым
  IP. Deployment держит их количество, но не даёт стабильный
  адрес. Если один Pod'у нужно обратиться к другому — он не
  может захардкодить IP: тот изменится через минуту.
  Service решает эту проблему. Это стабильная точка доступа
  к группе Pod'ов. Он даёт:
    • Постоянный IP (ClusterIP).
    • Постоянное DNS-имя.
    • Балансировку между Pod'ами.
    • Автоматическое обновление при изменении Pod'ов.

  СОДЕРЖАНИЕ:
    1.  Зачем нужен Service
    2.  Как Service находит Pod'ы
    3.  ClusterIP — дефолт
    4.  NodePort — порт на каждой ноде
    5.  LoadBalancer — облачный балансировщик
    6.  ExternalName — CNAME
    7.  Headless Service — для StatefulSet
    8.  DNS внутри кластера
    9.  Endpoints — сердце Service
    10. kube-proxy — как это работает
    11. sessionAffinity — sticky sessions
    12. Полезные команды
    13. Связь с Go
    14. Антипаттерны
    15. Финальные выводы

  1. ЗАЧЕМ НУЖЕН SERVICE

  ПРОБЛЕМА:
    Deployment с 3 репликами. У каждого Pod свой IP.
    Pod'ы постоянно пересоздаются. IP меняются.

    Как одному сервису обратиться к другому?
    Плохо: захардкодить IP Pod'а. Через минуту он изменится.
    Плохо: захардкодить IP ноды. Pod'ы распределены по нодам.

  РЕШЕНИЕ: Service даёт СТАБИЛЬНЫЙ адрес для группы Pod'ов.
    Клиент обращается к Service.
    Service знает актуальный список Pod'ов.
    Service балансирует трафик.

  СХЕМА:
    ┌──────────────┐
    │   Клиент     │
    └──────┬───────┘
           │ http://app:80
           ▼
    ┌──────────────┐
    │   Service    │  ClusterIP: 10.96.0.5
    │   app        │  DNS: app.default.svc.cluster.local
    └──────┬───────┘
           │ балансирует между
    ┌──────┼──────┬
    ▼      ▼      ▼
    ┌───┐ ┌───┐ ┌───┐
    │Pod│ │Pod│ │Pod│
    └───┘ └───┘ └───┘

  Что это даёт:
    • Клиент не думает про IP Pod'ов.
    • Pod'ы могут пересоздаваться — Service обновит список.
    • Балансировка автоматом.
    • DNS-имя вместо IP.

  2. КАК SERVICE НАХОДИТ POD'Ы
  Через SELECTOR — по labels.

  ПРИМЕР:
    apiVersion: v1
    kind: Service
    metadata:
      name: app
    spec:
      selector:
        app: app                # ищет Pod'ы с label app=app
      ports:
      - port: 80
        targetPort: 8080

  И Pod'ы:
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: app
    spec:
      template:
        metadata:
          labels:
            app: app           # ← этот label
        spec:
          containers:
          - name: app
            ports:
            - containerPort: 8080

  ЧТО ПРОИСХОДИТ:
    Service смотрит все Pod'ы в namespace.
    Находит те, у кого label `app=app`.
    Создаёт объект Endpoints со списком их IP.
    kube-proxy на каждой ноде настраивает iptables
    для балансировки на эти IP.

  ВАЖНО:
    • Service и Deployment не связаны напрямую.
    • Связь только через labels и selector.
    • Если selector не совпадает с labels — Endpoints пустой.

  3. CLUSTERIP — ДЕФОЛТ
  ClusterIP — тип по умолчанию. Даёт виртуальный IP внутри кластера.

  ПРИМЕР:
    apiVersion: v1
    kind: Service
    metadata:
      name: app
    spec:
      type: ClusterIP       # дефолт, можно не писать
      selector:
        app: app
      ports:
      - port: 80            # порт Service
        targetPort: 8080    # порт Pod'а

  ЧТО ЭТО ДАЁТ:
    • Виртуальный IP: 10.96.x.x (ClusterIP).
    • DNS-имя: app.default.svc.cluster.local.
    • Доступен только внутри кластера.
    • Балансировка между Pod'ами.

  КОГДА ИСПОЛЬЗУЕТСЯ:
    • Общение между микросервисами.
    • Backend — для API-gateway.
    • Postgres, Redis, Kafka — для внутренних клиентов.
    • 90% всех Service в кластере.

  ЧТО НЕЛЬЗЯ:
    • Обратиться из интернета — IP только внутри.
    • Обратиться с хоста — нужен port-forward или Ingress.

  ПРОВЕРИТЬ:
    kubectl get service app
    # NAME   TYPE        CLUSTER-IP     PORT(S)
    # app    ClusterIP   10.96.140.23   80/TCP

    kubectl get endpoints app
    # NAME   ENDPOINTS                       AGE
    # app    10.244.1.5:8080,10.244.2.7:8080   ...

  4. NODEPORT — ПОРТ НА КАЖДОЙ НОДЕ
  NodePort открывает порт на каждой ноде кластера. Трафик
  с <NodeIP>:<NodePort> идёт в Service.

  ПРИМЕР:
    apiVersion: v1
    kind: Service
    metadata:
      name: app
    spec:
      type: NodePort
      selector:
        app: app
      ports:
      - port: 80
        targetPort: 8080
        nodePort: 30080     # 30000-32767

  ЧТО ЭТО ДАЁТ:
    • Всё, что даёт ClusterIP.
    • Плюс порт 30080 на каждой ноде.
    • Доступ снаружи: http://<NodeIP>:30080.

  КОГДА ИСПОЛЬЗУЕТСЯ:
    • Быстрый доступ к сервису без Ingress.
    • Отладка, dev-кластеры.
    • On-prem, где нет облачного LB.

  ПРОБЛЕМЫ:
    • Все ноды открывают порт — attack surface.
    • Ограниченный диапазон (30000-32767).
    • Неудобно в проде (клиенту надо знать IP ноды).
    • Не масштабируется — если нод 50, у всех открыт порт.

  ПРОВЕРИТЬ:
    kubectl get svc app
    # NAME   TYPE       CLUSTER-IP     PORT(S)
    # app    NodePort   10.96.140.23   80:30080/TCP

    # Снаружи:
    curl http://<NodeIP>:30080

  5. LOADBALANCER — ОБЛАЧНЫЙ БАЛАНСИРОВЩИК
  LoadBalancer создаёт облачный балансировщик (AWS ELB, GCP LB,
  Azure LB) и даёт публичный IP.

  ПРИМЕР:
    apiVersion: v1
    kind: Service
    metadata:
      name: app
    spec:
      type: LoadBalancer
      selector:
        app: app
      ports:
      - port: 80
        targetPort: 8080

  ЧТО ЭТО ДАЁТ:
    • Виртуальный IP (ClusterIP).
    • Публичный IP от облака.
    • DNS-имя облачного LB.
    • Балансировка + SSL termination (если настроить).

  КОГДА ИСПОЛЬЗУЕТСЯ:
    • Публичный сервис в облаке.
    • Когда Ingress не подходит (TCP, gRPC, не-HTTP).

  ПРОБЛЕМЫ:
    • Дорого: 1 LB = 1 IP = ~$15-25/мес в AWS.
    • 10 сервисов = 10 LB = $200/мес.
    • Медленно создаётся (1-3 минуты).
    • Не для внутренних сервисов — они не должны торчать.

  ПРАВИЛО:
    В проде наружу торчит ОДИН LoadBalancer (или Ingress).
    Внутри — ClusterIP.
    20 сервисов с LoadBalancer — это дикий перерасход.

  ПРОВЕРИТЬ:
    kubectl get svc app
    # NAME   TYPE           CLUSTER-IP     EXTERNAL-IP
    # app    LoadBalancer   10.96.140.23   1.2.3.4

  6. EXTERNALNAME — CNAME НА ВНЕШНИЙ DNS
  ExternalName не проксирует трафик. Просто создаёт DNS-запись
  (CNAME) на внешний адрес.

  ПРИМЕР:
    apiVersion: v1
    kind: Service
    metadata:
      name: external-db
    spec:
      type: ExternalName
      externalName: db.example.com

  ЧТО ЭТО ДАЁТ:
    • DNS-имя внутри кластера: external-db.default.svc.cluster.local.
    • Оно резолвится в db.example.com.
    • kube-proxy НЕ участвует — резолвинг на уровне DNS.

  КОГДА ИСПОЛЬЗУЕТСЯ:
    • Доступ к внешней БД из кластера через привычное DNS-имя.
    • Миграция: сервис временно смотрит на внешнюю БД,
      потом переключается на внутреннюю.
    • Без хардкода внешнего URL в коде.

  ЧТО ВАЖНО:
    • Не балансирует трафик.
    • Не проксирует.
    • Только DNS.
    • Работает с TCP, но healthcheck нет.

  7. HEADLESS SERVICE — ДЛЯ STATEFULSET
  Headless Service — Service без ClusterIP. `clusterIP: None`.

  ЧТО ЭТО ДАЁТ:
    Вместо одного виртуального IP — DNS отдаёт IP каждого Pod'а.

  ПРИМЕР:
    apiVersion: v1
    kind: Service
    metadata:
      name: postgres
    spec:
      clusterIP: None
      selector:
        app: postgres
      ports:
      - port: 5432

  ЧТО ЭТО ДАЁТ:
    DNS:
      postgres.default.svc.cluster.local         → все IP
      postgres-0.postgres.default.svc.cluster.local → IP postgres-0
      postgres-1.postgres.default.svc.cluster.local → IP postgres-1

  КОГДА ИСПОЛЬЗУЕТСЯ:
    • StatefulSet (Postgres, Kafka, MongoDB, ES).
    • Когда клиент должен знать конкретный Pod.
    • Для service discovery по репликам.

  ЗАЧЕМ НЕ ОБЫЧНЫЙ:
    Обычный Service балансирует между Pod'ами. Для БД это плохо —
    клиент не знает, к какому подключился. Мастер-реплика
    требуют явных адресов.

  8. DNS ВНУТРИ КЛАСТЕРА
  В K8s работает CoreDNS. Каждый Service получает DNS-запись.

  ФОРМАТ:
    <service>.<namespace>.svc.cluster.local

  ПРИМЕРЫ:
    postgres.default.svc.cluster.local
    postgres.production.svc.cluster.local
    app.demo.svc.cluster.local

  КОРОТКИЕ ФОРМЫ:
    В том же namespace:
      postgres

    В другом namespace:
      postgres.other-namespace

    Полный FQDN:
      postgres.other-namespace.svc.cluster.local

  ПРИМЕР В GO:
    // Обращение к БД в том же namespace.
    dsn := "postgres://user:pass@postgres:5432/app"

    // Обращение в другой namespace.
    dsn := "postgres://user:pass@postgres.production:5432/app"

  ПРОВЕРИТЬ:
    kubectl run tmp --rm -it --image=busybox -n default -- sh
    # Внутри:
    nslookup postgres
    nslookup app.production
    nslookup app.production.svc.cluster.local

  9. ENDPOINTS — СЕРДЦЕ SERVICE
  Endpoints — объект, который хранит список IP Pod'ов для Service.

  КАК СОЗДАЁТСЯ:
    Service Controller (встроенный контроллер K8s) следит
    за Service и Pod'ами.
    Находит Pod'ы по selector.
    Создаёт Endpoints со списком их IP.

  ПРИМЕР:
    kubectl get endpoints app -n default
    # NAME   ENDPOINTS                                AGE
    # app    10.244.1.5:8080,10.244.2.7:8080,...     5m

  ЕСЛИ ENDPOINTS ПУСТОЙ:
    Service не находит Pod'ы.
    Причины:
      • selector не совпадает с labels Pod'ов.
      • Pod'ы не Ready (readiness не проходит).
      • Pod'ы в другом namespace.

  ПРОВЕРИТЬ ПОДРОБНО:
    kubectl describe service app
    # Selector: app=app
    # Endpoints: 10.244.1.5:8080,10.244.2.7:8080
    # ...

    kubectl get endpointslices -l kubernetes.io/service-name=app
    # EndpointSlice — новый формат (K8s 1.21+).

  10. KUBE-PROXY — КАК ЭТО РАБОТАЕТ
  kube-proxy — сетевой агент на каждой ноде. Отвечает за
  реализацию Service.

  ЧТО ДЕЛАЕТ:
    • Следит за Service и Endpoints в API Server.
    • Настраивает правила iptables (или IPVS).
    • Маршрутизирует трафик с ClusterIP на конкретные Pod'ы.
    • Делает балансировку между Pod'ами.

  ПРИМЕР:
    Service my-app с ClusterIP 10.96.0.5.
    3 Pod'а с IP: 10.244.1.5, 10.244.2.7, 10.244.3.9.

    Когда Pod A обращается к 10.96.0.5:8080:
      • Трафик перехватывается kube-proxy на ноде.
      • Правила iptables перенаправляют его на один из Pod'ов.
      • Выбор — round-robin (по умолчанию).

    Это НЕ прокси в классическом смысле. kube-proxy только
    настраивает правила ядра, сам трафик через него не идёт.

  ДВА РЕЖИМА:
    iptables (дефолт):
      • Правила ядра.
      • Быстрый, но не масштабируется на тысячи сервисов.

    IPVS (для больших кластеров):
      • Балансировщик на уровне ядра Linux.
      • Лучше масштабируется.

  ЧТО ЭТО ЗНАЧИТ НА ПРАКТИКЕ:
    • ClusterIP — виртуальный. Его нет на интерфейсах нод.
    • iptables перехватывает трафик на него.
    • Балансировка происходит на уровне ядра.

  11. SESSIONAFFINITY — STICKY SESSIONS
  По умолчанию Service балансирует round-robin. Каждый запрос
  может попасть на другой Pod.
  Иногда нужно, чтобы один клиент всегда попадал на один Pod.
  Для этого — sessionAffinity.

  ПРИМЕР:
    apiVersion: v1
    kind: Service
    metadata:
      name: app
    spec:
      selector:
        app: app
      sessionAffinity: ClientIP        # sticky по IP клиента
      sessionAffinityConfig:
        clientIP:
          timeoutSeconds: 10800        # 3 часа
      ports:
      - port: 80
        targetPort: 8080

  ЧТО ЭТО ДАЁТ:
    Клиент с одного IP всегда попадает на один и тот же Pod.

  КОГДА ИСПОЛЬЗУЕТСЯ:
    • Legacy-приложения с сессией в памяти Pod'а.
    • WebSocket без sticky-балансировщика.

  ЧЕГО НЕ ДЕЛАТЬ:
    • Не полагайся на sessionAffinity для stateful-приложений.
    • Сессия должна быть в Redis или БД, а не в Pod'е.
    • sessionAffinity — костыль, а не решение.

  ПОЧЕМУ ПЛОХО:
    • Нагрузка распределяется неравномерно.
    • При падении Pod'а сессия теряется.
    • Не масштабируется — новый Pod простаивает.

  ПРАВИЛЬНО:
    • Храни сессию в Redis.
    • Любой Pod обработает любой запрос.
    • Нет sticky — балансировка честная.

  12. ПОЛЕЗНЫЕ КОМАНДЫ

  СМОТРЕТЬ:
    kubectl get svc
    kubectl get svc -o wide
    kubectl get svc -A                      # все namespace
    kubectl describe svc app

  ENDPOINTS:
    kubectl get endpoints app
    kubectl get endpointslices -l kubernetes.io/service-name=app

  DNS:
    kubectl run tmp --rm -it --image=busybox -- sh
    # nslookup app
    # nslookup app.default
    # nslookup app.default.svc.cluster.local

  PORT-FORWARD:
    kubectl port-forward svc/app 8080:80
    # Открыть http://localhost:8080

  ИЗМЕНИТЬ ТИП:
    kubectl patch svc app -p '{"spec":{"type":"NodePort"}}'

  УДАЛИТЬ:
    kubectl delete svc app
    kubectl delete -f service.yaml

  13. СВЯЗЬ С GO
  Go-сервис ходит к другому сервису через DNS.

  ПРИМЕР КОДА:
    func connectToDB() (*sql.DB, error) {
        // DNS-имя — из переменной окружения.
        host := os.Getenv("DB_HOST")   // "postgres"
        port := os.Getenv("DB_PORT")   // "5432"

        dsn := fmt.Sprintf(
            "postgres://%s:%s@%s:%s/%s?sslmode=disable",
            user, password, host, port, dbname,
        )
        return sql.Open("pgx", dsn)
    }

  В DEPLOYMENT:
    env:
    - name: DB_HOST
      value: postgres
    - name: DB_PORT
      value: "5432"

  В CLUSTER:
    Service postgres с ClusterIP.
    CoreDNS резолвит postgres → ClusterIP.
    kube-proxy направляет на Pod'ы.

  ПРАВИЛА ДЛЯ GO:
    • Не хардкодь IP. Только DNS-имена.
    • Используй короткое имя (postgres), если в том же namespace.
    • Полное имя (postgres.production), если в другом.
    • Retry подключения при старте — Service может
      стартовать раньше Pod'ов.

  14. АНТИПАТТЕРНЫ

  14.1. ХАРДКОД IP POD'А.
    IP меняется. Только DNS-имена Service.

  14.2. LOADBALANCER ДЛЯ ВНУТРЕННИХ СЕРВИСОВ.
    Каждому внутреннему сервису LB — $15-25/мес.
    Только Ingress наружу, ClusterIP внутри.

  14.3. SELECTOR НЕ СОВПАДАЕТ С LABELS.
    Endpoints пустой. Service не находит Pod'ы.

  14.4. NODEPORT В ПРОДЕ.
    Все ноды открывают порт. Attack surface.
    Или Ingress, или LoadBalancer.

  14.5. SERVICE БЕЗ READINESS.
    Трафик идёт в Pod'ы, которые не готовы.
    Клиенты получают 500.

  14.6. HEADLESS ДЛЯ STATELESS.
    Не нужен. Только для StatefulSet.

  14.7. EXTERNALNAME ДЛЯ ВНУТРЕННИХ.
    Внутренние — ClusterIP. ExternalName только для
    действительно внешних.

  14.8. ОДИН SERVICE НА ВСЁ.
    Один Service = один сервис. Не смешивай разные порты
    в одном Service.

  14.9. ИГНОРИРОВАТЬ ENDPOINTS.
    Пустой Endpoints — симптом проблемы. Смотреть первым.

  14.10. SERVICE TYPE: LOADBALANCER БЕЗ CLOUD.
    В on-prem не создастся. EXTERNAL-IP будет <pending>.

  14.11. SESSIONAFFINITY КАК РЕШЕНИЕ.
    Костыль. Сессия в Redis, а не sticky.

  15. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Service — стабильная точка доступа к Pod'ам. Даёт
      постоянный IP, DNS, балансировку.
  2.  Service находит Pod'ы по selector (labels).
  3.  ClusterIP — дефолт. Внутренний. 90% Service.
  4.  NodePort — порт на каждой ноде. Для отладки, dev.
  5.  LoadBalancer — облачный балансировщик. Дорого.
      Только для публичных сервисов.
  6.  ExternalName — CNAME на внешний DNS. Без проксирования.
  7.  Headless Service — `clusterIP: None`. Для StatefulSet.
      DNS отдаёт IP каждого Pod'а.
  8.  DNS: `<service>.<namespace>.svc.cluster.local`.
      Коротко: `postgres` в том же namespace.
  9.  Endpoints — список IP Pod'ов. Обновляется автоматически.
  10. kube-proxy настраивает iptables/IPVS для балансировки.
  11. sessionAffinity — sticky sessions. Костыль, лучше
      хранить сессию в Redis.
  12. В Go: DNS-имена в env, не хардкод IP. Retry при старте.
  13. Антипаттерны: хардкод IP, LoadBalancer для внутренних,
      NodePort в проде, нет readiness, пустой Endpoints,
      sessionAffinity как решение.
*/
