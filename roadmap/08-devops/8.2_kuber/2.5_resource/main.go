package main

/*
  УРОК 2.5: RESOURCE REQUESTS И LIMITS
  Каждый Pod в K8s потребляет CPU и память. Без ограничений
  один Pod может съесть всю ноду: OOM-killer убьёт соседей,
  CPU будет занят на 100%, остальные сервисы встанут.
  K8s даёт два механизма:
    requests — сколько Pod'у нужно (гарантия).
    limits   — максимум, что Pod может потребить (потолок).
  Requests влияют на планирование. Limits — на runtime.

  СОДЕРЖАНИЕ:
    1.  Зачем нужны requests и limits
    2.  Requests — гарантия
    3.  Limits — потолок
    4.  Что происходит при превышении
    5.  CPU throttling vs OOMKilled
    6.  QoS-классы
    7.  Единицы измерения
    8.  Как выбирать значения
    9.  GOMEMLIMIT для Go-сервисов
    10. kubectl top — мониторинг
    11. LimitRange и ResourceQuota
    12. Kubelet eviction
    13. Что НЕ надо лимитировать
    14. Антипаттерны
    15. Финальные выводы

  1. ЗАЧЕМ НУЖНЫ REQUESTS И LIMITS

  БЕЗ LIMITS:
    Один Pod с утечкой памяти съедает всю память ноды.
    Ядро запускает OOM-killer. Убивает случайные процессы.
    Соседние Pod'ы падают.
    Один Pod с бесконечным циклом занимает 100% CPU.
    Остальные Pod'ы тормозят.

  С LIMITS: Pod не превысит свой потолок. Соседи защищены.

  С REQUESTS: Scheduler не поставит Pod на ноду, где нет
  места. Pod гарантированно получит минимум.

  2. REQUESTS — ГАРАНТИЯ
  requests — минимальный объём, который Pod гарантированно получит.

  ЧТО ГАРАНТИРУЕТ:
    • Scheduler не поставит Pod на ноду, где нет requests.
    • При contention за CPU — Pod получит не меньше requests.
    • Pod получит хотя бы requests памяти.

  ПРИМЕР:
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"

  ВЛИЯНИЕ НА SCHEDULER:
    Scheduler складывает requests всех Pod'ов на ноде.
    Сумма не может превысить capacity ноды.

    Нода 4 CPU, 8 GiB.
      Pod A: 1 CPU, 2 GiB
      Pod B: 2 CPU, 4 GiB
      Pod C: 1 CPU, 2 GiB
    Сумма: 4 CPU, 8 GiB. Нода заполнена.
    Pod D с requests 1 CPU — Pending.

  ВАЖНО: requests — это резервирование, а не фактическое
  потребление. Pod может использовать меньше — ресурсы
  простаивают. Другие Pod'ы их не займут.

  3. LIMITS — ПОТОЛОК
  limits — максимум, который Pod может потребить.

  ПРИМЕР:
    resources:
      limits:
        cpu: "500m"
        memory: "256Mi"

  КЛЮЧЕВОЕ РАЗЛИЧИЕ CPU И ПАМЯТИ:
    ПАМЯТЬ (жёсткий лимит):
      Превышение → ядро убивает процесс. Exit 137. OOMKilled.
      Память нельзя «замедлить».

    CPU (мягкий лимит):
      Превышение → throttling. Pod работает медленнее,
      но не умирает. CPU — делимая величина.

  4. ЧТО ПРОИСХОДИТ ПРИ ПРЕВЫШЕНИИ

  ПАМЯТЬ:
    Pod использует 300 MiB при limits.memory 256 MiB.
    Ядро: OOM-killer убивает процесс.
    Exit Code: 137.
    Status: OOMKilled.
    RESTARTS растёт.

    В describe pod:
      Last State:   Terminated
      Reason:       OOMKilled
      Exit Code:    137

    В kubectl get pods:
      NAME    READY   STATUS    RESTARTS
      app-1   0/1     Running   3    ← счётчик растёт

  CPU:
    Pod использует 1.5 CPU при limits.cpu 1.
    Ядро: cgroup CPU quota. Pod замедляется.
    Метрика: container_cpu_cfs_throttled_seconds_total.
    Latency растёт, throughput падает.
    Но Pod продолжает работать.

  5. CPU THROTTLING VS OOMKILLED

  CPU THROTTLING:
    limits.cpu = 500m = 0.5 ядра.
    Период cgroup = 100ms.
    За 100ms Pod может использовать 50ms CPU.
    Использовал 60ms — заморозка на 40ms.
    КАК ВИДНО: latency растёт, throughput падает,
    CPU utilization ниже limits.
    ЧТО ДЕЛАТЬ: увеличить limits.cpu, оптимизировать код,
    масштабировать.

  OOMKILLED:
    Pod превысил limits.memory. Ядро не может «замедлить»
    память — только убить.
    КАК ВИДНО: RESTARTS растёт, describe показывает OOMKilled,
    exit code 137, логи обрываются на середине.
    ЧТО ДЕЛАТЬ: увеличить limits.memory, найти утечку,
    уменьшить GOMEMLIMIT, использовать pprof.

  6. QOS-КЛАССЫ
  K8s присваивает Pod'у один из трёх классов. Влияет на
  eviction при нехватке ресурсов на ноде.

  GUARANTEED (самый защищённый):
    requests == limits для всех ресурсов всех контейнеров.

    resources:
      requests: { cpu: "500m", memory: "256Mi" }
      limits:   { cpu: "500m", memory: "256Mi" }

    Убивают последними. Для критичных сервисов, БД, gateway.

  BURSTABLE (средний):
    requests и limits разные. Или задан только один.

    resources:
      requests: { cpu: "100m", memory: "128Mi" }
      limits:   { cpu: "500m", memory: "256Mi" }

    Убивают вторыми. Большинство сервисов.

  BESTEFFORT (самый незащищённый):
    Ни requests, ни limits.

    Убивают первыми. В проде — никогда.

  ПОСМОТРЕТЬ:
    kubectl get pod <name> -o jsonpath='{.status.qosClass}'

  ЗАЧЕМ ЭТО ЗНАТЬ:
    При нехватке памяти kubelet выселяет Pod'ы в порядке
    BestEffort → Burstable → Guaranteed.
    Внутри класса — по использованию относительно requests.
    Кто больше превышает — тот и выселяется.

  7. ЕДИНИЦЫ ИЗМЕРЕНИЯ

  CPU:
    1     = 1 ядро
    100m  = 0.1 ядра
    500m  = 0.5 ядра
    2     = 2 ядра

  MEMORY:
    128Mi = 128 MiB (1024²)
    1Gi   = 1 GiB (1024³)
    256M  = 256 MB (1000²)

    Всегда используй Mi и Gi — точнее.

  ПРИМЕРЫ:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "500m"
      memory: "256Mi"

  ЕСЛИ УКАЗАТЬ ТОЛЬКО ЧИСЛО БЕЗ ЕДИНИЦ:
    cpu: 1       = 1 ядро.
    memory: 128  = 128 байт (не 128Mi!). Всегда указывай Mi.

  8. КАК ВЫБИРАТЬ ЗНАЧЕНИЯ
  REQUESTS: минимум, который Pod'у нужен.
    Смотреть на реальное потребление под низкой нагрузкой. Плюс небольшой запас.

  LIMITS: максимум без вреда для ноды.
    Смотреть на пики под максимальной нагрузкой. Плюс запас на всплески.

  ПРАВИЛА:
    • requests.memory >= реального минимума.
    • limits.memory >= 1.5× реального пика.
    • requests.cpu — средний CPU.
    • limits.cpu — 2-3× requests для burstable.
    • Для критичных: requests == limits (Guaranteed).

  УЗНАТЬ РЕАЛЬНОЕ ПОТРЕБЛЕНИЕ:
    kubectl top pods -n demo
    # NAME    CPU(cores)  MEMORY(bytes)
    # app-1   15m         85Mi

    Или локально: wrk, hey, ab, pprof.

  9. GOMEMLIMIT ДЛЯ GO-СЕРВИСОВ
  ПРОБЛЕМА: Go-сервис растёт до 250 MiB при limits.memory
  256 MiB. GC редко вызывается. Всплеск → OOMKilled.

  РЕШЕНИЕ: GOMEMLIMIT — soft limit для GC.

  КАК СТАВИТЬ:
    GOMEMLIMIT = limits.memory × 0.8

    limits.memory = 256Mi
    GOMEMLIMIT    = 205MiB

  ПОЧЕМУ 80%: есть память вне heap (стеки, mmap).
  GC не успевает мгновенно.

  В DEPLOYMENT:
    env:
    - name: GOMEMLIMIT
      value: "200MiB"

  В КОДЕ:
    debug.SetMemoryLimit(200 * 1024 * 1024)

  ВАЖНО ПРО GOMAXPROCS:
    Go по умолчанию использует GOMAXPROCS = num CPU ноды.
    В контейнере с limits.cpu=1 на ноде с 16 ядрами Go
    создаст 16 потоков.

    Используй automaxprocs:
      import _ "go.uber.org/automaxprocs"

    Подстраивает GOMAXPROCS под limits.cpu.

  10. KUBECTL TOP — МОНИТОРИНГ
  Требует metrics-server.

  kubectl top pods -n demo
  # NAME    CPU(cores)  MEMORY(bytes)
  # app-1   15m         85Mi

  kubectl top nodes
  # NAME    CPU(cores)  CPU%  MEMORY(bytes)  MEMORY%
  # node-1  500m        12%   2Gi            25%

  СОРТИРОВКА:
    kubectl top pods --sort-by=cpu -n demo
    kubectl top pods --sort-by=memory -n demo

  С CONTAINERS:
    kubectl top pod <name> --containers -n demo

  ЧТО СМОТРЕТЬ:
    • CPU близко к limits → throttling.
    • Memory близко к limits → OOM risk.
    • Memory растёт монотонно → утечка.

  11. LIMITRANGE И RESOURCEQUOTA
  LIMITRANGE — defaults и максимумы на namespace.

    apiVersion: v1
    kind: LimitRange
    metadata:
      name: default-limits
      namespace: demo
    spec:
      limits:
      - default:            # если Pod не указал resources
          cpu: "500m"
          memory: "256Mi"
        defaultRequest:
          cpu: "100m"
          memory: "128Mi"
        max:                # максимум для одного Pod
          cpu: "2"
          memory: "2Gi"
        type: Container

  ЧТО ДЕЛАЕТ:
    • Если Pod не указал resources — получает default.
    • Если указал больше max — не создаётся.

  RESOURCEQUOTA — ограничивает сумму ресурсов в namespace.

    apiVersion: v1
    kind: ResourceQuota
    metadata:
      name: demo-quota
      namespace: demo
    spec:
      hard:
        requests.cpu: "10"
        requests.memory: 20Gi
        limits.cpu: "20"
        limits.memory: 40Gi
        pods: "50"

  ЗАЧЕМ:
    • Изоляция команд.
    • Защита от «шумного соседа».
    • Дефолты для неаккуратных разработчиков.

  12. KUBELET EVICTION
  Kubelet следит за ресурсами ноды и может выселять Pod'ы,
  когда ресурсы заканчиваются.

  НЕ ПУТАТЬ:
    • OOM-killer — ядро Linux при превышении limits.memory.
    • Eviction — kubelet при нехватке ресурсов на ноде.

  КОГДА СРАБАТЫВАЕТ:
    По дефолтным порогам:
      • memory.available < 100Mi
      • nodefs.available < 10%
      • imagefs.available < 15%
      • pid.available < 10%

  ПОРЯДОК ВЫСЕЛЕНИЯ:
    1. BestEffort Pod'ы (без requests/limits).
    2. Burstable Pod'ы, превышающие requests.
    3. Burstable Pod'ы, не превышающие requests.
    4. Guaranteed Pod'ы.

  ЧТО ПРОИСХОДИТ:
    • Pod получает статус Evicted.
    • Deployment создаёт новый на другой ноде.
    • В событиях: «The node was low on resource: memory».

  КАК ЗАЩИТИТЬСЯ:
    • requests == limits (QoS Guaranteed).
    • Мониторить node memory available.
    • Не перегружать ноды (requests ≤ 80% capacity).

  ПРОВЕРИТЬ СОБЫТИЯ:
    kubectl get events --field-selector reason=Evicted

  13. ЧТО НЕ НАДО ЛИМИТИРОВАТЬ
  CPU LIMITS — часто НЕ надо:

    Google, Shopify рекомендуют НЕ ставить limits.cpu:
      • Throttling непредсказуем. Pod работает медленнее,
        но не умирает. Latency растёт скачками.
      • Requests уже даёт гарантию.
      • При contention ядро само распределит CPU по requests.
      • Без limits Pod может использовать простаивающий CPU.

    ПРАКТИКА:
      resources:
        requests:
          cpu: "100m"       # гарантируем минимум
          memory: "128Mi"
        limits:
          # CPU limit НЕ ставим
          memory: "256Mi"   # memory limit обязателен

  ПАМЯТЬ — limits ОБЯЗАТЕЛЬНЫ:
    Без limits.memory Pod может съесть всю память ноды.
    OOM-killer убьёт кого угодно.

  14. АНТИПАТТЕРНЫ
  14.1. НЕ СТАВИТЬ RESOURCES.
    BestEffort QoS. Убивают первыми.

  14.2. requests = limits ДЛЯ ВСЕГО.
    Guaranteed для всех — избыточно, нет burst.

  14.3. limits МЕНЬШЕ requests.
    K8s не даст создать.

  14.4. limits БЕЗ REQUESTS.
    K8s приравняет requests = limits.

  14.5. СЛИШКОМ МАЛЕНЬКИЙ limits.memory ДЛЯ GO.
    Go съедает всё доступное. OOMKilled.

  14.6. GOMEMLIMIT = limits.memory.
    Без запаса. GC не успевает. OOM.

  14.7. CPU limits СЛИШКОМ МАЛЕНЬКИЕ.
    Постоянный throttling.

  14.8. НЕ СМОТРЕТЬ kubectl top.
    Лимиты — из головы.

  14.9. ОДИН REQUEST ДЛЯ DEV И PROD.
    В dev нагрузка меньше. В prod OOM.

  14.10. ИГНОРИРОВАТЬ THROTTLING.
    Pod работает медленно. Latency растёт.

  14.11. RESOURCEQUOTA БЕЗ LIMITRANGE.
    Квоты есть, но Pod'ы без resources не пройдут.

  14.12. CPU LIMITS В ПРОДЕ БЕЗ ПРИЧИНЫ.
    Throttling непредсказуем.

  14.13. ИГНОРИРОВАТЬ EVICTION СОБЫТИЯ.
    Pod'ы выселяются с нод. Смотреть events.

  15. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Requests — гарантия. Влияют на планирование.
      Limits — потолок. Влияют на runtime.
  2.  Память — жёсткий лимит. Превышение → OOMKilled.
      CPU — мягкий. Превышение → throttling.
  3.  QoS: Guaranteed (requests==limits), Burstable
      (разные), BestEffort (ничего).
  4.  Eviction порядок: BestEffort → Burstable → Guaranteed.
  5.  Единицы: CPU — m, memory — Mi, Gi.
  6.  requests.memory >= минимум,
      limits.memory >= 1.5× пик.
  7.  Для Go: GOMEMLIMIT = limits.memory × 0.8.
      automaxprocs для GOMAXPROCS.
  8.  kubectl top pods/nodes — реальное потребление.
  9.  LimitRange — defaults. ResourceQuota — сумма на namespace.
  10. Kubelet eviction при нехватке на ноде. По QoS.
  11. CPU limits в проде — часто НЕ надо. Memory limits обязательны.
  12. Антипаттерны: нет resources, limits < requests,
      GOMEMLIMIT = limits, нет запаса, CPU limits без причины.
*/
