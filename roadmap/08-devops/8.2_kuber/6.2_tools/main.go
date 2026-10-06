package main

/*
  УРОК 6.2: ИНСТРУМЕНТЫ ДИАГНОСТИКИ
  Диагностика в K8s — это не гадание, а набор инструментов. Ты идёшь по цепочке: get pods → describe → logs → events → exec/debug.
  Каждый инструмент отвечает на свой вопрос. describe говорит, что не так с Pod'ом. logs — что говорит приложение. events — что говорит K8s.
  exec и debug пускают внутрь. k9s и stern ускоряют навигацию. Без инструментов ты слепой, с ними — видишь всю картину.
  ВАЖНО: конкретные статусы Pod'ов (CrashLoopBackOff, OOMKilled, Pending) — в уроке 6.1.
  Здесь — про инструменты: чем смотреть, как смотреть, что смотреть.

  СОДЕРЖАНИЕ:
    1.  Зачем нужны инструменты
    2.  kubectl describe — главный инструмент
    3.  kubectl logs — логи (краткий обзор)
    4.  kubectl get events — события K8s
    5.  kubectl exec — зайти внутрь
    6.  kubectl debug — ephemeral containers
    7.  kubectl top — ресурсы
    8.  kubectl get -o yaml/json/jsonpath
    9.  kubectl port-forward — локальный доступ
    10. kubectl auth can-i — RBAC диагностика
    11. kubectl api-resources и api-versions
    12. kubectl rollout — диагностика деплоя
    13. kubectl diff — что изменится
    14. kubectl explain — справка по полям
    15. k9s — TUI для K8s
    16. stern — логи нескольких Pod'ов
    17. Связь с Go
    18. Антипаттерны
    19. Финальные выводы

  1. ЗАЧЕМ НУЖНЫ ИНСТРУМЕНТЫ
  ПРОБЛЕМА: в K8s легко потеряться. Pod'ов десятки, нод несколько, статусов десятки. Где смотреть? Что смотреть? В каком порядке?

  РЕШЕНИЕ: системный подход. Каждый инструмент отвечает на свой вопрос.
    • describe — что не так с объектом? (Events, Status, Conditions).
    • logs — что говорит приложение? (stdout/stderr).
    • events — что говорит K8s? (Scheduler, kubelet, Controller).
    • exec — что внутри контейнера? (файлы, процессы, сеть).
    • debug — то же, но для distroless.
    • top — сколько ресурсов ест?
    • get -o yaml — что реально в объекте?
    • port-forward — достучаться локально.

  ПОРЯДОК ДИАГНОСТИКИ:
    1. kubectl get pods — что вообще происходит? Какой статус?
    2. kubectl describe pod — почему? Events внизу.
    3. kubectl logs / logs --previous — если контейнер запускался.
    4. kubectl get events — если describe не хватило.
    5. kubectl exec / debug — если контейнер Running, но что-то не так.
    6. kubectl describe node — если проблема на уровне ноды.

  ПРАВИЛО: 90% проблем решаются на шагах 1-3. Остальные 10% — на шагах 4-6.

  2. KUBECTL DESCRIBE — ГЛАВНЫЙ ИНСТРУМЕНТ
  Основная команда:
    kubectl describe pod <name> -n <ns>

  ЧТО ВЫВОДИТ:
    • Метаданные (name, namespace, node, IP, labels, annotations).
    • Spec (образ, env, resources, probes, volumes).
    • Status (фаза, conditions, container statuses).
    • Секцию Events в конце — самое важное.

  СЕКЦИЯ EVENTS — ЭТО ЛОГИ K8S О ПОДЕ:
    Events:
      Type     Reason            From       Message
      ----     ------            ----       -------
      Normal   Scheduled         scheduler  Successfully assigned demo/app to node-1
      Normal   Pulling           kubelet    Pulling image "my-app:1.0"
      Warning  Failed            kubelet    Failed to pull image "my-app:1.0": ...
      Warning  FailedScheduling  scheduler  0/3 nodes are available: 3 Insufficient cpu
      Warning  BackOff           kubelet    Back-off restarting failed container
      Warning  Unhealthy         kubelet    Readiness probe failed: ...

  ЧТО ЧИТАТЬ:
    • FailedScheduling — Pod не размещён. Причина: ресурсы, taints, nodeSelector.
    • Failed (pull) — образ не тянется. Опечатка, нет secret.
    • BackOff — контейнер падает. Смотреть логи.
    • Unhealthy — probe не проходит.
    • OOMKilling — превышен limits.memory.

  ЧТО ЕЩЁ СМОТРЕТЬ В DESCRIBE:
    • Node — на какой ноде Pod.
    • Containers → State → Last State — причина последнего перезапуска (OOMKilled, Error, Completed).
    • Conditions — Ready, ContainersReady, PodScheduled.
    • QoS Class — Guaranteed, Burstable, BestEffort.
    • Volumes — что примонтировано.

  DESCRIBE ДЛЯ РАЗНЫХ ОБЪЕКТОВ:
    kubectl describe pod <name> -n demo
    kubectl describe node <node>
    kubectl describe deployment <name> -n demo
    kubectl describe service <name> -n demo
    kubectl describe pvc <name> -n demo
    kubectl describe hpa <name> -n demo
    kubectl describe ingress <name> -n demo

  ФИЛЬТР ПО LABEL:
    kubectl describe pods -l app=app -n demo
    kubectl describe pods -l app=app --field-selector status.phase=Running -n demo

  ПРАВИЛО: describe — первое, что надо смотреть, когда что-то не так. 90% ответов там.

  3. KUBECTL LOGS — ЛОГИ (КРАТКИЙ ОБЗОР)
  Базовый доступ:
    kubectl logs <pod> -n demo

  ОСНОВНЫЕ ФЛАГИ:
    -f, --follow            — следить в реальном времени.
    --tail=100              — последние 100 строк.
    --since=1h              — за последний час.
    --timestamps            — добавить timestamp.
    -c <container>          — конкретный контейнер.
    --all-containers        — все контейнеры Pod'а.
    --prefix                — префикс с именем Pod'а.
    --previous              — логи предыдущего (упавшего) контейнера.

  КЛЮЧЕВЫЕ КОМАНДЫ ДЛЯ ДИАГНОСТИКИ:
    kubectl logs <pod> -n demo --previous
    kubectl logs <pod> -n demo -c <container> --previous
    kubectl logs -l app=app -n demo --all-containers --prefix
    kubectl logs <pod> -n demo --tail=500 --timestamps

  --previous — ЭТО КРИТИЧНО:
    Если Pod упал и был перезапущен — обычный logs покажет логи нового контейнера. Старые логи (того, что упал) — только через --previous.

  ЧТО ВАЖНО: K8s собирает только stdout/stderr. Если приложение пишет в файл — kubectl logs ничего не покажет.

  ПОДРОБНЕЕ О ЛОГАХ — В УРОКЕ 5.4 (Observability).

  4. KUBECTL GET EVENTS — СОБЫТИЯ K8S
  Events — это встроенный механизм K8s. API Server генерирует события о состоянии объектов.

  ПОСМОТРЕТЬ ВСЕ СОБЫТИЯ:
    kubectl get events -n demo

  С СОРТИРОВКОЙ ПО ВРЕМЕНИ:
    kubectl get events -n demo --sort-by=.metadata.creationTimestamp

  ТОЛЬКО ПРЕДУПРЕЖДЕНИЯ:
    kubectl get events -n demo --field-selector type=Warning

  ПО КОНКРЕТНОМУ ОБЪЕКТУ:
    kubectl get events -n demo --field-selector involvedObject.name=app-abc123

  WATCH В РЕАЛЬНОМ ВРЕМЕНИ:
    kubectl get events -n demo -w

  ПО ВСЕМ NAMESPACE:
    kubectl get events -A --sort-by=.metadata.creationTimestamp

  ЧТО ПОПАДАЕТ В EVENTS:
    • FailedScheduling — Pod не размещён.
    • Pulling, Pulled — kubelet тянет образ.
    • Created, Started — контейнер создан и запущен.
    • Unhealthy — probe не прошёл.
    • BackOff — контейнер падает и перезапускается.
    • OOMKilling — kubelet убил Pod по памяти.
    • Evicted — Pod выселен с ноды.

  ВАЖНО: events живут ~1 час. Для истории — event-exporter в Loki/ES. Или смотри сразу.
  ПРАВИЛО: если describe не помог — get events. Иногда события от Scheduler или Node Controller видны только там.

  5. KUBECTL EXEC — ЗАЙТИ ВНУТРЬ
  Основная команда:
    kubectl exec -it <pod> -n <ns> -- sh

  ЧТО ДЕЛАЕТ: запускает команду внутри работающего контейнера.

  ПРИМЕРЫ:
    # Интерактивный shell.
    kubectl exec -it app-abc123 -n demo -- sh
    kubectl exec -it app-abc123 -n demo -- bash

    # Одна команда.
    kubectl exec app-abc123 -n demo -- ls /app
    kubectl exec app-abc123 -n demo -- cat /etc/config/app.yaml

    # Конкретный контейнер в Pod'е с несколькими.
    kubectl exec -it app-abc123 -n demo -c app -- sh

    # Через deployment.
    kubectl exec -it deployment/app -n demo -- sh

  ЧТО ПОЛЕЗНО ДЕЛАТЬ ВНУТРИ:
    # Проверить env.
    env | grep DB_

    # Проверить DNS.
    nslookup postgres.demo.svc.cluster.local
    ping postgres

    # Проверить доступ к другому сервису.
    wget -q -O - http://app.demo.svc.cluster.local/health
    curl -v http://app.demo.svc.cluster.local/health

    # Посмотреть сетевые интерфейсы.
    ip addr
    netstat -tlnp

    # Посмотреть процессы.
    ps aux
    top

    # Посмотреть файлы.
    ls -la /etc/secrets/
    cat /etc/secrets/DB_PASSWORD

  ВАЖНО ПРО DISTROLESS:
    В distroless нет sh, bash, ls, curl. exec не сработает. Решения:
      • Ephemeral containers: kubectl debug.
      • Debug-образ с тем же бинарником + shell.
      • nsenter на ноде (сложно).

  ПРАВИЛО: exec — для живого контейнера. Если контейнер падает — exec не поможет, смотри логи.

  6. KUBECTL DEBUG — EPHEMERAL CONTAINERS
  ПРОБЛЕМА: контейнер на distroless (нет shell). Как зайти внутрь?

  РЕШЕНИЕ: ephemeral containers. Временный контейнер, добавляется в работающий Pod.

  ОСНОВНАЯ КОМАНДА:
    kubectl debug -it <pod> --image=alpine --target=<container> -n demo

  ЧТО ПРОИСХОДИТ:
    • В Pod добавляется контейнер alpine.
    • Он видит тот же network namespace, что и target.
    • Может делиться process namespace (--target).
    • После выхода — исчезает.

  ПРИМЕР СЕССИИ:
    kubectl debug -it app-abc123 --image=alpine --target=app -n demo

    # Внутри:
    ps aux                # видишь процессы app
    ip addr               # видишь сеть Pod'а
    wget -q -O - http://localhost:8080/health
    ls /proc/1/root/      # доступ к файлам app

  ДРУГИЕ РЕЖИМЫ DEBUG:
    # Копия Pod'а с изменённым образом.
    kubectl debug app-abc123 -n demo --copy-to=debug-pod --image=alpine

    # Debug ноды (запускает Pod на ноде с host namespaces).
    kubectl debug node/node-1 -it --image=alpine

    # Debug с общим process namespace (видно процессы target).
    kubectl debug -it app-abc123 --image=alpine --target=app --share-processes -n demo

  ОГРАНИЧЕНИЯ:
    • Только для отладки. Не для продакшн-нагрузки.
    • Нельзя указать в манифесте Deployment. Только руками.
    • Не переживают рестарт Pod'а.
    • Не все CNI поддерживают (нужен process namespace sharing).

  ПРАВИЛО: если distroless или нет shell — debug. Если есть shell — exec.

  7. KUBECTL TOP — РЕСУРСЫ
  Показывает текущее потребление CPU и memory.

  ОСНОВНЫЕ КОМАНДЫ:
    kubectl top nodes
    kubectl top pods -n demo
    kubectl top pods -n demo --containers
    kubectl top pods -n demo --sort-by=cpu
    kubectl top pods -n demo --sort-by=memory

  ПРИМЕР:
    kubectl top pods -n demo
    # NAME          CPU(cores)   MEMORY(bytes)
    # app-abc123    45m          120Mi
    # app-def456    38m          118Mi

  ЧТО ЗНАЧАТ ЕДИНИЦЫ:
    CPU в millicores — 1000m = 1 CPU. 50m = 0.05 CPU.
    Memory в Mi/Gi — 128Mi = 128 мебибайт.

  ТРЕБУЕТ: metrics-server. Без него — "error: Metrics API not available".

  ЧТО ПОЛЕЗНО СМОТРЕТЬ:
    • Топ по CPU — кто жрёт больше всех.
    • Топ по memory — кто может упасть в OOMKilled.
    • --containers — разбивка по контейнерам в Pod'е (если есть sidecar).

  ПРАВИЛО: top — для быстрой проверки. Для истории и графиков — Prometheus + Grafana.

  8. KUBECTL GET -O YAML/JSON/JSONPATH
  Иногда describe не показывает всё. Тогда — смотрим сырой объект.

  ПОЛНЫЙ YAML:
    kubectl get pod app-abc123 -n demo -o yaml
    kubectl get deployment app -n demo -o yaml

  ПОЛНЫЙ JSON:
    kubectl get pod app-abc123 -n demo -o json

  КОНКРЕТНОЕ ПОЛЕ (JSONPATH):
    # IP Pod'а.
    kubectl get pod app-abc123 -n demo -o jsonpath='{.status.podIP}'

    # Нода Pod'а.
    kubectl get pod app-abc123 -n demo -o jsonpath='{.spec.nodeName}'

    # Last State.
    kubectl get pod app-abc123 -n demo -o jsonpath='{.status.containerStatuses[*].lastState}'

    # Restart count.
    kubectl get pod app-abc123 -n demo -o jsonpath='{.status.containerStatuses[*].restartCount}'

  СВОИ КОЛОНКИ (CUSTOM-COLUMNS):
    kubectl get pods -n demo -o custom-columns=\
    NAME:.metadata.name,\
    IP:.status.podIP,\
    NODE:.spec.nodeName,\
    STATUS:.status.phase

  ЧТО ПОЛЕЗНО ИСКАТЬ ЧЕРЕЗ JSONPATH:
    • lastState.terminated.reason — почему упал (OOMKilled, Error).
    • lastState.terminated.exitCode — код выхода (137 = OOMKilled).
    • status.conditions — Ready, ContainersReady.
    • spec.containers[*].image — какой образ.

  ПРИМЕР ДЛЯ ДИАГНОСТИКИ OOMKILLED:
    kubectl get pod app-abc123 -n demo \
      -o jsonpath='{.status.containerStatuses[*].lastState.terminated.reason}'
    # OOMKilled

  ПРАВИЛО: jsonpath и custom-columns — для скриптов и быстрого извлечения полей. Для чтения глазами — yaml.

  9. KUBECTL PORT-FORWARD — ЛОКАЛЬНЫЙ ДОСТУП
  Пробрасывает порт из Pod'а или Service на localhost.

  СИНТАКСИС:
    kubectl port-forward <pod|svc> <local-port>:<remote-port> -n <ns>

  ПРИМЕРЫ:
    # От Pod'а.
    kubectl port-forward app-abc123 8080:8080 -n demo

    # От Service (выберет случайный Pod).
    kubectl port-forward svc/app 8080:80 -n demo

    # От Deployment.
    kubectl port-forward deployment/app 8080:8080 -n demo

  ЧТО ПРОИСХОДИТ:
    Локальный порт 8080 открывается на твоей машине.
    Трафик идёт через API Server в kubelet в Pod.
    Открой http://localhost:8080 — увидишь свой сервис.

  ЗАЧЕМ НУЖНО:
    • Локально тестировать API без внешнего Ingress.
    • Подключиться к БД внутри кластера.
    • Отлаживать сервис, который слушает порт.
    • Подключиться psql'ом к Postgres в Pod'е.

  ПРИМЕР ДЛЯ БД:
    kubectl port-forward svc/postgres 5432:5432 -n demo
    psql -h localhost -p 5432 -U app -d app

  ПРИМЕР ДЛЯ PROMETHEUS:
    kubectl port-forward svc/prometheus 9090:9090 -n monitoring
    # Открыть http://localhost:9090

  ОГРАНИЧЕНИЯ:
    port-forward работает через kubectl, значит через API Server.
    Не подходит для высоких нагрузок. Только для отладки.

  ПРАВИЛО: port-forward — для локальной отладки. В проде — Ingress и Service.

  10. KUBECTL AUTH CAN-I — RBAC ДИАГНОСТИКА
  Проверяет, может ли пользователь или ServiceAccount выполнить действие.

  ОСНОВНЫЕ КОМАНДЫ:
    kubectl auth can-i get pods -n demo
    kubectl auth can-i create deployments -n demo
    kubectl auth can-i delete secrets -n demo

  ДЛЯ ДРУГОГО ПОЛЬЗОВАТЕЛЯ:
    kubectl auth can-i list pods --as=alice@example.com -n demo
    # yes

    kubectl auth can-i list pods --as=alice@example.com -n default
    # no

  ДЛЯ SERVICEACCOUNT:
    kubectl auth can-i list pods --as=system:serviceaccount:demo:app-sa -n demo

  СПИСОК ВСЕХ ПРАВ:
    kubectl auth can-i --list -n demo

  ЗАЧЕМ НУЖНО:
    • Проверить RBAC-политику.
    • Понять, почему Pod не может обратиться к API Server.
    • Отладить права ServiceAccount.

  ПРИМЕР ПРОБЛЕМЫ:
    Pod не может читать ConfigMap. Проверяем:
      kubectl auth can-i get configmaps --as=system:serviceaccount:demo:app-sa -n demo
      # no
    Значит, Role/RoleBinding не настроены для этого SA.

  ПРАВИЛО: когда что-то «не имеет прав» — auth can-i. Быстрее, чем читать YAML ролей.

  11. KUBECTL API-RESOURCES И API-VERSIONS
  api-resources — список всех типов объектов в кластере.

  КОМАНДА:
    kubectl api-resources

  ЧТО ПОКАЗЫВАЕТ:
    NAME          SHORTNAMES   APIVERSION   NAMESPACED   KIND
    pods          po           v1           true         Pod
    deployments   deploy       apps/v1      true         Deployment
    services      svc          v1           true         Service

  ПОЛЕЗНЫЕ ФЛАГИ:
    kubectl api-resources --namespaced=true       # только namespaced
    kubectl api-resources --namespaced=false      # только cluster-wide
    kubectl api-resources --verbs=list,get        # по глаголам
    kubectl api-resources | grep monitoring       # поиск CRD

  api-versions — список всех API-версий.

  КОМАНДА:
    kubectl api-versions

  ЧТО ПОКАЗЫВАЕТ:
    apps/v1
    batch/v1
    autoscaling/v2
    networking.k8s.io/v1
    monitoring.coreos.com/v1
    ...

  ЗАЧЕМ НУЖНО:
    • Понять, какие CRD установлены (Prometheus Operator, Argo, Istio).
    • Проверить, поддерживается ли нужная API-версия.
    • Отладить «no matches for kind» в YAML.

  ПРИМЕР ПРОБЛЕМЫ:
    YAML падает с "no matches for kind ServiceMonitor". Проверяем:
      kubectl api-resources | grep ServiceMonitor
    Если пусто — Prometheus Operator не установлен.

  ПРАВИЛО: перед написанием YAML с CRD — проверь, что CRD есть в кластере.

  12. KUBECTL ROLLOUT — ДИАГНОСТИКА ДЕПЛОЯ
  Когда Deployment не катится или завис — смотрим rollout.

  СТАТУС ОБНОВЛЕНИЯ:
    kubectl rollout status deployment/app -n demo

  ИСТОРИЯ:
    kubectl rollout history deployment/app -n demo
    kubectl rollout history deployment/app --revision=2 -n demo

  ПРИЧИНА ЗАВИСАНИЯ:
    kubectl describe deployment app -n demo
    # Смотреть Conditions: Progressing, Available.
    # Смотреть Events.

  ПРОВЕРИТЬ REPLICASET'Ы:
    kubectl get rs -n demo -l app=app
    # Старый RS с replicas:0, новый с replicas:N.

  ЧТО СМОТРЕТЬ В describe deployment:
    Replicas: 3 desired | 2 updated | 3 total | 2 available | 1 unavailable
    # 1 unavailable — значит один Pod не поднимается.

    Conditions:
      Type           Status  Reason
      Available      True    MinimumReplicasAvailable
      Progressing    False   ProgressDeadlineExceeded
    # ProgressDeadlineExceeded — обновление зависло.

  ЧТО ПРОВЕРИТЬ ПРИ ЗАВИСАНИИ:
    1. Новый Pod вообще создаётся?
    2. Pod Running или Pending?
    3. Readiness probe проходит?
    4. В логах нового Pod'а что?

  ПРАВИЛО: если rollout status висит — describe deployment + логи нового Pod'а. Обычно причина в readiness probe.

  13. KUBECTL DIFF — ЧТО ИЗМЕНИТСЯ
  Показывает, что изменится при apply, без применения.

  КОМАНДА:
    kubectl diff -f deployment.yaml

  ЧТО ПОКАЗЫВАЕТ:
    Diff между текущим состоянием в кластере и YAML-файлом.

  ЗАЧЕМ НУЖНО:
    • Проверить, что apply не сломает прод.
    • Понять, почему apply не меняет то, что ожидалось.
    • Увидеть drift между git и кластером.

  ПРИМЕР:
    kubectl diff -f deployment.yaml
    # diff -u -N /tmp/LIVE-xxx /tmp/MERGED-xxx
    # --- /tmp/LIVE-xxx
    # +++ /tmp/MERGED-xxx
    # -  replicas: 3
    # +  replicas: 5

  ТРЕБУЕТ: права на server-side dry-run.
  ПРАВИЛО: в CI/CD — всегда diff перед apply. Это спасает от неожиданных изменений.

  14. KUBECTL EXPLAIN — СПРАВКА ПО ПОЛЯМ
  Показывает документацию по полям объектов.

  КОМАНДЫ:
    kubectl explain pod
    kubectl explain pod.spec
    kubectl explain pod.spec.containers
    kubectl explain pod.spec.containers.resources
    kubectl explain deployment.spec.strategy

  С РЕКУРСИЕЙ:
    kubectl explain pod --recursive

  ЧТО ПОКАЗЫВАЕТ:
    KIND:     Pod
    VERSION:  v1
    DESCRIPTION: ...
    FIELDS:
      apiVersion  <string>
      kind        <string>
      metadata    <Object>
      spec        <Object>

  ЗАЧЕМ НУЖНО:
    • Не помнишь все поля наизусть.
    • Проверить синтаксис поля.
    • Понять, что делает поле.
    • Не гуглить, а смотреть из терминала.

  ПРАВИЛО: не помнишь поле — explain. Быстрее, чем документация K8s.

  15. K9S — TUI ДЛЯ K8S
  k9s — это интерактивный терминальный UI для K8s.

  УСТАНОВКА:
    brew install k9s       # macOS
    apt install k9s        # Linux (или из github releases)

  ЗАПУСК:
    k9s
    k9s -n demo
    k9s --context production

  ОСНОВНЫЕ ВОЗМОЖНОСТИ:
    • Навигация по объектам стрелками.
    • Просмотр describe, logs, yaml прямо в UI.
    • Exec внутрь контейнера.
    • Фильтрация по namespace, label.
    • Быстрое переключение между ресурсами.

  ГОРЯЧИЕ КЛАВИШИ:
    :pods           — перейти к Pod'ам.
    :deploy         — к Deployment'ам.
    :svc            — к Service'ам.
    :events         — к событиям.
    :nodes          — к нодам.
    /pattern        — фильтр по имени.
    0               — все namespace.
    d               — describe.
    l               — logs.
    s               — shell.
    y               — yaml.
    Ctrl+D          — удалить.
    Esc             — назад.

  ЗАЧЕМ НУЖНО:
    • Быстрая навигация без kubectl-команд.
    • Одновременный просмотр нескольких ресурсов.
    • Быстрый доступ к логам и shell.
    • Удобно для дежурного (oncall).

  МИНУСЫ:
    • Не для скриптов (это UI).
    • Не для CI/CD.
    • Требует привычки.

  ПРАВИЛО: k9s + kubectl + stern покрывают 99% диагностики. k9s для быстрой навигации, kubectl — для точных команд.

  16. STERN — ЛОГИ НЕСКОЛЬКИХ POD'ОВ
  ПРОБЛЕМА: 20 Pod'ов в Deployment'е. Как смотреть логи всех сразу?
  kubectl logs -l app=app — работает, но неудобно: логи разных Pod'ов перемешаны, нет цветов, нет фильтра.

  РЕШЕНИЕ: stern.

  УСТАНОВКА:
    brew install stern       # macOS
    # или из github releases

  ОСНОВНЫЕ КОМАНДЫ:
    stern app -n demo
    stern -l app=app -n demo
    stern deployment/app -n demo

  ОСНОВНЫЕ ФЛАГИ:
    --tail=50               — последние 50 строк.
    --since=1h              — за последний час.
    --timestamps            — с timestamp.
    --exclude-container     — исключить контейнер.
    --container             — только конкретный контейнер.
    --color=always          — цветной вывод.

  ПРИМЕРЫ:
    stern -l app=app -n demo --tail=50
    stern deployment/app -n demo --since=10m
    stern app -n demo --exclude-container=istio-proxy

  ЧТО ПОЛЕЗНО:
    • Каждый Pod — свой цвет. Легко отличить.
    • Префикс с именем Pod'а.
    • Автоматически подхватывает новые Pod'ы при rolling update.
    • Фильтр по regex.

  ПРАВИЛО: для логов нескольких Pod'ов — stern. Для одного — kubectl logs.

  17. СВЯЗЬ С GO
  Что нужно от Go-разработчика для удобной диагностики:

  1. ЛОГИ В STDOUT/STDERR.
     Не в файл. K8s соберёт сам. kubectl logs и stern будут работать.

  2. СТРУКТУРИРОВАННЫЕ ЛОГИ.
     JSON через log/slog. Легко парсить и фильтровать.

     ПРИМЕР:
       slog.Info("request handled",
           "method", r.Method,
           "path", r.URL.Path,
           "status", status,
           "duration_ms", duration.Milliseconds(),
       )

  3. TRACE_ID В ЛОГАХ.
     Связать лог с трейсом. Добавлять trace_id в каждый лог.

  4. /HEALTH И /READY.
     Для probes. /health — для liveness, /ready — для readiness.

  5. GRACEFUL SHUTDOWN.
     Ловить SIGTERM. Завершать активные запросы. Закрывать соединения.

  6. ВАЛИДАЦИЯ ENV ПРИ СТАРТЕ.
     Не паниковать с nil pointer. Падать с понятной ошибкой.

     ПРИМЕР:
       if os.Getenv("DB_HOST") == "" {
           return errors.New("DB_HOST is required")
       }

  7. RETRY ПОДКЛЮЧЕНИЯ К БД.
     БД может стартовать медленнее. Не падать с первой попытки.

  ПРИМЕР ДЛЯ ОТЛАДКИ ЧЕРЕЗ DEBUG:
    Если Go-бинарник в distroless (нет shell):
      kubectl debug -it app-abc123 --image=alpine --target=app -n demo
      # Внутри:
      ls /proc/1/root/app      # файлы app
      cat /proc/1/root/etc/secrets/DB_PASSWORD
      wget -q -O - http://localhost:8080/health

  ЧТО ЧАСТО ЛОМАЕТСЯ:
    • Паника при старте → CrashLoopBackOff.
    • OOMKilled из-за отсутствия GOMEMLIMIT.
    • Readiness не проходит, потому что БД медленно стартует.
    • Логи в файл, а не в stdout → kubectl logs пусто.
    • Нет graceful shutdown → запросы обрываются.

  ПРАВИЛО: Go-сервис должен быть «хорошим гражданином» K8s: stdout-логи, probes, graceful shutdown, GOMEMLIMIT, валидация ENV.

  18. АНТИПАТТЕРНЫ
  18.1. СМОТРЕТЬ ТОЛЬКО GET PODS, НЕ ИДТИ В DESCRIBE. 90% ответов — в describe.
  18.2. НЕ СМОТРЕТЬ ЛОГИ ПРЕДЫДУЩЕГО КОНТЕЙНЕРА. --previous — обязателен для упавших Pod'ов.
  18.3. ИГНОРИРОВАТЬ EVENTS. Иногда Scheduler/Node Controller пишут только туда.
  18.4. ИСПОЛЬЗОВАТЬ EXEC НА DISTROLESS. Не сработает. Нужен debug.
  18.5. ЗАБЫТЬ ПРО PORT-FORWARD. Проще, чем настраивать Ingress для отладки.
  18.6. НЕ ИСПОЛЬЗОВАТЬ AUTH CAN-I. Гадать про RBAC вместо проверки.
  18.7. НЕ ПРОВЕРЯТЬ API-RESOURCES. YAML падает с "no matches for kind".
  18.8. СМОТРЕТЬ ROLLOUT STATUS БЕЗ DESCRIBE. Зависло — смотри describe deployment.
  18.9. НЕ ИСПОЛЬЗОВАТЬ DIFF ПЕРЕД APPLY. Неожиданные изменения в проде.
  18.10. ГУГЛИТЬ ПОЛЯ ВМЕСТО EXPLAIN. explain pod.spec.containers.resources — быстрее.
  18.11. НЕ ИСПОЛЬЗОВАТЬ K9S. Тратить время на kubectl-команды.
  18.12. KUBECTL LOGS ДЛЯ 20 POD'ОВ. stern удобнее.
  18.13. ЗАБЫТЬ ПРО JSONPATH. Извлечение полей руками через yaml — медленно.
  18.14. ИСПОЛЬЗОВАТЬ K9S В CI/CD. Это UI, для скриптов — kubectl.
  18.15. СМОТРЕТЬ ЛОГИ В ФАЙЛАХ. K8s собирает stdout. Логи в файлах недоступны.
  18.16. НЕ СМОТРЕТЬ LAST STATE В DESCRIBE. Там причина последнего перезапуска.
  18.17. FORCE DELETE POD'ОВ. Оставляет ресурсы на ноде.

  19. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Диагностика в K8s — системный подход. Каждый инструмент отвечает на свой вопрос.
  2.  Порядок: get pods → describe → logs → logs --previous → events → exec/debug.
  3.  kubectl describe — главный инструмент. Events внизу отвечают на большинство вопросов.
  4.  kubectl logs — stdout/stderr. --previous для упавших контейнеров. -l для нескольких Pod'ов.
  5.  kubectl get events — события K8s. Живут ~1 час. Смотреть при FailedScheduling, Evicted.
  6.  kubectl exec — зайти внутрь. Не работает на distroless.
  7.  kubectl debug — ephemeral containers. Для distroless.
  8.  kubectl top — CPU/memory. Требует metrics-server.
  9.  kubectl get -o yaml/json/jsonpath — сырые объекты и извлечение полей.
  10. kubectl port-forward — локальный доступ к Pod/Service.
  11. kubectl auth can-i — проверка RBAC.
  12. kubectl api-resources — список типов объектов. Проверка CRD.
  13. kubectl rollout — статус и история деплоя.
  14. kubectl diff — что изменится при apply.
  15. kubectl explain — справка по полям.
  16. k9s — TUI. Быстрая навигация. Не для скриптов.
  17. stern — логи нескольких Pod'ов сразу. Цветной вывод.
  18. Go-сервис: stdout-логи, slog JSON, /health, /ready, graceful shutdown, валидация ENV, retry.
*/
