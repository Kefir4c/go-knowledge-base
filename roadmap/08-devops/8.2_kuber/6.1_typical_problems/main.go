package main

/*
  УРОК 6.1: ТИПОВЫЕ ПРОБЛЕМЫ
  90% времени в K8s — понять, почему Pod не работает. Приложение не стартует, трафик не идёт,
  Pod висит в Pending, контейнер падает в CrashLoopBackOff.
  Хорошая новость: типовых проблем не так много, и у каждой есть чёткий алгоритм диагностики.
  Выучив 6-8 основных статусов, ты покрываешь 95% инцидентов.
  Плохая новость: без системного подхода легко потеряться. Поэтому главный инструмент — kubectl describe pod, а не гадание.

  СОДЕРЖАНИЕ:
    1.  Почему это важно
    2.  Фазы Pod'а и статусы контейнеров
    3.  kubectl describe — главный инструмент
    4.  CrashLoopBackOff
    5.  ImagePullBackOff и ErrImagePull
    6.  OOMKilled
    7.  Pending
    8.  Evicted
    9.  ContainerCreating — долго висит
    10. Terminating — застрял
    11. CreateContainerConfigError
    12. FailedMount и FailedAttachVolume
    13. RunContainerError
    14. Unhealthy — probes не проходят
    15. NotReady — нода недоступна
    16. Init:Error и Init:CrashLoopBackOff
    17. Системный подход к диагностике
    18. Инструменты: describe, logs, events, debug, k9s
    19. Связь с Go
    20. Антипаттерны
    21. Финальные выводы

  1. ПОЧЕМУ ЭТО ВАЖНО
  В K8s ты редко пишешь YAML руками. Чаще ты разбираешься, почему то, что должно работать, не работает.
  Pod не запустился — почему? Приложение упало — почему? Трафик не идёт — почему?

  ТИПОВЫЕ СИТУАЦИИ:
    • Развернул Deployment, Pod'ы висят в Pending.
    • Обновил образ, Pod'ы падают в CrashLoopBackOff.
    • Приложение работало, потом начало OOMKilled.
    • Нода ушла в NotReady, Pod'ы Evicted.
    • Readiness probe не проходит, трафик не идёт.

  2. ФАЗЫ POD'А И СТАТУСЫ КОНТЕЙНЕРОВ

  ФАЗЫ POD'А (высокоуровневые):
    Pending    — Pod создан, но ещё не запущен. Ждёт Scheduler, тянет образ, работает init.
    Running    — хотя бы один основной контейнер работает.
    Succeeded  — все контейнеры завершились с кодом 0. Для Job.
    Failed     — хотя бы один контейнер завершился с != 0.
    Unknown    — не удаётся получить статус (обычно проблема с нодой).

  СТАТУСЫ КОНТЕЙНЕРОВ:
    Waiting     — ждёт чего-то (pull образа, init, ресурсы).
    Running     — работает.
    Terminated  — завершился (успех или ошибка).

  КАК ЭТО ОТОБРАЖАЕТСЯ В kubectl get pods:
    Pending            — Pod не размещён.
    ContainerCreating  — kubelet создаёт контейнер.
    Running            — работает.
    CrashLoopBackOff   — падает и перезапускается.
    ImagePullBackOff   — образ не тянется.
    ErrImagePull       — первая попытка скачать образ не удалась.
    Completed          — Job завершился успешно.
    Error              — Job завершился с ошибкой.
    Terminating        — удаляется.
    Evicted            — выселен с ноды.
    OOMKilled          — убит по памяти.
    Init:0/2           — init-контейнеры ещё работают.

  ЧТО ВАЖНО: статус в kubectl get — это не фаза, а более детальный признак. Он показывает, что именно происходит с контейнером.
  Если статус тебя смущает — иди в describe. Там всё подробно.

  3. KUBECTL DESCRIBE — ГЛАВНЫЙ ИНСТРУМЕНТ

  Когда Pod не работает — первое действие:
    kubectl describe pod <name> -n <ns>

  ЧТО ВЫВОДИТ:
    • Метаданные (labels, annotations, node).
    • Spec (образ, env, resources, probes).
    • Status (фаза, IP, условия).
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

  ПРАВИЛО: если что-то не работает — describe. Если describe не хватает — logs. Если и этого мало — exec/debug внутрь.

  4. CRASHLOOPBACKOFF

  СИМПТОМ:
    kubectl get pods -n demo
    # NAME          READY   STATUS             RESTARTS   AGE
    # app-abc123    0/1     CrashLoopBackOff   5          3m

  ЧТО ЗНАЧИТ: контейнер стартует, падает, K8s его перезапускает с экспоненциальным backoff (10s, 20s, 40s, 80s, max 5m).

  ПОЧЕМУ ЭТО ПРОИСХОДИТ:
    • Паника в коде при инициализации.
    • Не хватает обязательного ENV.
    • БД/Redis недоступны (неправильный DSN или контейнер ещё не поднялся).
    • Неправильные права (permission denied).
    • Порт занят (address already in use).
    • Неправильная команда в command/args.
    • Миграции падают при старте.

  ДИАГНОСТИКА:
    1. kubectl describe pod <name> -n demo.
       Смотреть Last State: Terminated, Reason, Exit Code.
       Смотреть Events внизу.

    2. kubectl logs <name> -n demo --previous.
       Логи упавшего контейнера. Обычный logs показывает текущий (перезапущенный) контейнер.

    3. kubectl logs <name> -n demo --tail=200.
       Логи текущего контейнера.

  ЧТО СМОТРЕТЬ В ЛОГАХ:
    panic: runtime error: ...          — паника Go.
    connection refused                 — БД недоступна.
    dial tcp: lookup postgres         — DNS не резолвится.
    permission denied                  — права.
    bind: address already in use      — порт занят.
    missing required env: DB_HOST      — нет ENV.

  ЧАСТАЯ ПРИЧИНА В GO:
    • Паника при инициализации (nil pointer, отсутствует обязательный ENV).
    • Не обработали ошибку при подключении к БД.
    • os.Exit(1) без логирования причины.

  РЕШЕНИЕ:
    • Валидировать ENV при старте. Падать с понятной ошибкой, а не с panic.
    • Retry подключения к БД (init-контейнер или в коде).
    • Читать логи предыдущего контейнера (--previous).

  ВАЖНО: CrashLoopBackOff — не диагноз, а симптом. Причина — в логах и Events.

  5. IMAGEPULLBACKOFF И ERRIMAGEPULL

  СИМПТОМ:
    kubectl get pods -n demo
    # NAME          READY   STATUS             RESTARTS   AGE
    # app-abc123    0/1     ImagePullBackOff   0          2m
    # app-def456    0/1     ErrImagePull       0          30s

  ЧТО ЗНАЧИТ:
    ErrImagePull — первая попытка скачать образ не удалась.
    ImagePullBackOff — K8s повторяет с backoff (10s, 20s, 40s, max 5m).

  ПРИЧИНЫ:
    • Опечатка в имени образа или тега.
    • Тега нет в registry (docker pull my-app:1.0 — 404).
    • Приватный registry без imagePullSecret.
    • Неправильный imagePullSecret (не тот namespace, истёк токен).
    • Сеть: нода не может достучаться до registry.
    • Rate limit от registry (Docker Hub).
    • Registry требует аутентификации, а secret не указан.

  ДИАГНОСТИКА:
    kubectl describe pod <name> -n demo

    В Events:
      Failed to pull image "my-app:1.0": rpc error: code = Unknown desc = ...
      Error: ErrImagePull
      Error: ImagePullBackOff

  ТИПОВЫЕ СООБЩЕНИЯ И ЧТО ЗНАЧАТ:
    "manifest unknown"              — тега нет в registry.
    "unauthorized"                  — нет imagePullSecret или неправильный.
    "no such host"                  — DNS до registry не резолвится.
    "connection refused"            — registry недоступен.
    "toomanyrequests"               — rate limit Docker Hub.
    "x509: certificate signed by unknown authority" — TLS-проблема.

  РЕШЕНИЕ:
    • Проверить образ локально: docker pull my-app:1.0.
    • Создать imagePullSecret: kubectl create secret docker-registry reg-cred --docker-server=... --docker-username=... --docker-password=...
    • Указать в Pod: imagePullSecrets: - name: reg-cred.
    • Или привязать к ServiceAccount: kubectl patch sa default -p '{"imagePullSecrets":[{"name":"reg-cred"}]}'.
    • Проверить, что нода может достучаться до registry (curl, ping).

  ВАЖНО: если образ в Docker Hub и rate limit — использовать зеркало или authenticated pull.

  6. OOMKILLED

  СИМПТОМ:
    kubectl get pods -n demo
    # NAME          READY   STATUS      RESTARTS   AGE
    # app-abc123    0/1     OOMKilled   3          5m

  Или Pod Running, но приложение падает с exit code 137.

  ЧТО ЗНАЧИТ: контейнер превысил limits.memory. Ядро убило процесс (OOM Killer). Exit code 137 = 128 + 9 (SIGKILL).

  ПОЧЕМУ ПРОИСХОДИТ:
    • Утечка памяти в коде.
    • limits.memory слишком маленький для реальной нагрузки.
    • Всплеск нагрузки (много параллельных запросов → много памяти).
    • Go-сервис не знает про limits (нет GOMEMLIMIT).
    • Большой heap из-за GC-пауз.

  ДИАГНОСТИКА:
    kubectl describe pod <name> -n demo

    В Last State:
      Reason: OOMKilled
      Exit Code: 137

    kubectl top pod <name> -n demo
    # Показывает текущее потребление.

    kubectl get pod <name> -n demo -o yaml | grep -A 5 lastState

  ЧТО СМОТРЕТЬ:
    • Restart count растёт.
    • Memory usage в Grafana приближается к limits.
    • Логи: пусто (процесс убит, не успел залогировать).
    • В describe — Last State: OOMKilled.

  РЕШЕНИЕ:
    • Увеличить limits.memory.
    • Настроить GOMEMLIMIT (для Go) на 10-20% меньше limits.
    • Проверить утечки (pprof, heap dump).
    • Оптимизировать код (меньше аллокаций, sync.Pool).
    • Проверить, нет ли бесконечного роста slice/map.

  ВАЖНО ДЛЯ GO: GOMEMLIMIT — soft limit для GC. Если GOMEMLIMIT=230MiB, а limits.memory=256Mi, GC начнёт агрессивнее работать при 230Mi, не давая процессу дойти до 256Mi.
  Без GOMEMLIMIT Go видит только физическую память ноды (например, 16GB) и не знает про cgroup limit. Heap растёт до 256Mi, ядро убивает.

  ПРИМЕР:
    env:
    - name: GOMEMLIMIT
      value: "230MiB"
    resources:
      limits:
        memory: "256Mi"

  7. PENDING

  СИМПТОМ:
    kubectl get pods -n demo
    # NAME          READY   STATUS    RESTARTS   AGE
    # app-abc123    0/1     Pending   0          5m

  ЧТО ЗНАЧИТ: Pod создан, но Scheduler не может найти подходящую ноду.

  ПРИЧИНЫ:
    • Нехватка ресурсов: requests.cpu/memory больше, чем свободно на нодах.
    • nodeSelector не совпадает ни с одной нодой.
    • Taints на всех нодах, у Pod нет tolerations.
    • Affinity правила не выполняются.
    • PVC не создан / не привязан (для StatefulSet).
    • PodAntiAffinity не даёт разместить два Pod'а на одной ноде.
    • Все ноды в NotReady.

  ДИАГНОСТИКА:
    kubectl describe pod <name> -n demo

    В Events:
      Warning  FailedScheduling  scheduler
      0/3 nodes are available: 3 Insufficient cpu.
      # или
      0/3 nodes are available: 1 node(s) had taint {key: value},
      that the pod didn't tolerate.
      # или
      0/3 nodes are available: 3 node(s) didn't match Pod's node affinity.

  ЧТО СМОТРЕТЬ:
    • Сколько ресурсов просит Pod (requests.cpu, requests.memory).
    • Сколько доступно на нодах: kubectl describe node <name>.
    • Какие taints: kubectl get nodes -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints.
    • Есть ли PVC и привязан ли он: kubectl get pvc -n demo.
    • Affinity правила: kubectl get pod <name> -o yaml | grep -A 10 affinity.

  РЕШЕНИЕ:
    • Уменьшить requests.cpu/memory.
    • Добавить ноды (Cluster Autoscaler).
    • Исправить nodeSelector/affinity.
    • Добавить tolerations.
    • Разобраться с PVC (StorageClass, доступность зоны).
    • Убрать PodAntiAffinity, если он слишком строгий.

  ВАЖНО: Pending ≠ проблема. Может быть нормально при старте (образ тянется, init-контейнер работает).
  Pending долго (>30 секунд) — проблема. Смотреть describe.

  8. EVICTED

  СИМПТОМ:
    kubectl get pods -n demo
    # NAME          READY   STATUS    RESTARTS   AGE
    # app-abc123    0/1     Evicted   0          10m

  ЧТО ЗНАЧИТ: Pod выселен с ноды. Не потому что он плохой, а потому что нода под давлением.

  ПРИЧИНЫ:
    • MemoryPressure на ноде — нода заканчивает память.
    • DiskPressure — нода заканчивает диск.
    • PIDPressure — слишком много процессов.
    • Kubelet выбирает Pod'ы с низким QoS и выселяет.

  ПОРЯДОК ВЫСЕЛЕНИЯ:
    1. BestEffort — нет requests/limits.
    2. Burstable — requests < limits.
    3. Guaranteed — requests == limits (последние).

  ДИАГНОСТИКА:
    kubectl describe pod <name> -n demo
    # В Status: Reason: Evicted
    # В Message: The node was low on resource: memory.

    kubectl describe node <node>
    # Смотреть Conditions: MemoryPressure, DiskPressure.
    # Смотреть Allocated resources.

  РЕШЕНИЕ:
    • Настроить requests/limits, чтобы QoS был Guaranteed или Burstable.
    • Почистить диск на нодах (docker system prune, удалить логи).
    • Увеличить ресурсы нод.
    • Добавить ноды.

  ВАЖНО: Evicted — проблема уровня ноды, не Pod'а. Смотреть describe node, а не describe pod.

  9. CONTAINERCREATING — ДОЛГО ВИСИТ

  СИМПТОМ:
    kubectl get pods -n demo
    # NAME          READY   STATUS              RESTARTS   AGE
    # app-abc123    0/1     ContainerCreating   0          5m

  ЧТО ЗНАЧИТ: Pod размещён, kubelet создаёт контейнер. Обычно быстро (секунды). Если висит минуты — проблема.

  ПРИЧИНЫ:
    • Образ тянется долго (большой образ, медленный registry).
    • Volume не может примонтироваться (PVC, ConfigMap, Secret).
    • CNI не может выделить IP.
    • Init-контейнер долго работает.

  ДИАГНОСТИКА:
    kubectl describe pod <name> -n demo
    # Смотреть Events.

  ТИПОВЫЕ EVENTS:
    Pulling image "my-app:1.0" — тянет образ.
    FailedMount — volume не монтируется.
    FailedAttachVolume — не может прикрепить диск.

  РЕШЕНИЕ:
    • Проверить размер образа (multi-stage, distroless).
    • Проверить PVC/StorageClass.
    • Проверить CNI (Calico, Cilium).

  10. TERMINATING — ЗАСТРЯЛ

  СИМПТОМ:
    kubectl get pods -n demo
    # NAME          READY   STATUS        RESTARTS   AGE
    # app-abc123    0/1     Terminating   0          15m

  ЧТО ЗНАЧИТ: Pod удаляется, но не может завершиться. Обычно из-за finalizers.

  ПРИЧИНЫ:
    • Finalizers на Pod'е (например, от операторов).
    • Процесс не отвечает на SIGTERM.
    • terminationGracePeriodSeconds слишком длинный.
    • kubelet не может связаться с container runtime.

  ДИАГНОСТИКА:
    kubectl get pod <name> -n demo -o yaml | grep finalizers

    kubectl describe pod <name> -n demo

  РЕШЕНИЕ:
    • Дождаться terminationGracePeriodSeconds.
    • Удалить finalizer: kubectl patch pod <name> -p '{"metadata":{"finalizers":[]}}' --type=merge.
    • Проверить container runtime на ноде.

  ВАЖНО: force delete (kubectl delete pod --force --grace-period=0) — опасно, может оставить ресурсы на ноде.

  11. CREATECONTAINERCONFIGERROR

  СИМПТОМ:
    kubectl get pods -n demo
    # NAME          READY   STATUS                       RESTARTS   AGE
    # app-abc123    0/1     CreateContainerConfigError   0          2m

  ЧТО ЗНАЧИТ: контейнер не может быть создан из-за ошибки в конфигурации.

  ПРИЧИНЫ:
    • ConfigMap или Secret не найден.
    • Secret есть, но нет нужного ключа.
    • Неправильное имя volume.
    • Projected volume с ошибкой.
    • imagePullSecrets не найден.

  ДИАГНОСТИКА:
    kubectl describe pod <name> -n demo
    # В Events: Error: secret "app-secrets" not found.
    # Или: couldn't find key DB_PASSWORD in Secret demo/app-secrets.

  РЕШЕНИЕ:
    • Создать ConfigMap/Secret.
    • Проверить ключи.
    • Проверить имена volume.

  ВАЖНО: это ошибка в момент создания контейнера. Pod не запустится, пока ConfigMap/Secret не появятся.

  12. FAILEDMOUNT И FAILEDATTACHVOLUME

  СИМПТОМ: Pod висит в ContainerCreating, в Events — FailedMount.

  ПРИЧИНЫ:
    • PVC не привязан.
    • StorageClass недоступен.
    • Volume не может быть прикреплён к ноде (зональность в облаке).
    • NFS недоступен.
    • ConfigMap/Secret не найден.
    • CSI-драйвер не работает.

  ДИАГНОСТИКА:
    kubectl describe pod <name> -n demo
    # Events: Unable to attach or mount volumes: ...

    kubectl get pvc -n demo
    # STATUS: Pending — PVC не привязан.

    kubectl describe pvc <name> -n demo
    # Events: waiting for first consumer to be created.

  РЕШЕНИЕ:
    • Проверить StorageClass: kubectl get storageclass.
    • Проверить, что PVC создан и привязан.
    • Проверить зону ноды и зону диска (в AWS/GCP).
    • Проверить CSI-драйвер.

  ВАЖНО: FailedAttachVolume — в облаке часто из-за того, что диск в одной зоне, а Pod запланирован в другой.

  13. RUNCONTAINERERROR

  СИМПТОМ:
    kubectl get pods -n demo
    # NAME          READY   STATUS              RESTARTS   AGE
    # app-abc123    0/1     RunContainerError   0          1m

  ЧТО ЗНАЧИТ: контейнер создан, но не может запуститься.

  ПРИЧИНЫ:
    • Команда (command) не найдена.
    • Бинарник отсутствует в образе.
    • Нет прав на выполнение.
    • Ошибка в entrypoint.

  ДИАГНОСТИКА:
    kubectl describe pod <name> -n demo
    # Events: Error: failed to start container: ...

  РЕШЕНИЕ:
    • Проверить command/args в манифесте.
    • Проверить, что бинарник есть в образе (docker run --rm image ls /app).
    • Проверить права.

  ВАЖНО: RunContainerError встречается реже, чем CrashLoopBackOff. Обычно из-за неправильной команды.

  14. UNHEALTHY — PROBES НЕ ПРОХОДЯТ

  СИМПТОМ:
    kubectl get pods -n demo
    # NAME          READY   STATUS    RESTARTS   AGE
    # app-abc123    0/1     Running   3          5m

  STATUS Running, но READY 0/1. Это значит: контейнер работает, но readinessProbe падает.

  ПРИЧИНЫ:
    • /ready возвращает не 200.
    • Порт неправильный.
    • Path неправильный.
    • Приложение не успевает стартовать (initialDelaySeconds слишком маленький).
    • Приложение не может достучаться до БД и потому не ready.
    • livenessProbe падает → Pod перезапускается.

  ДИАГНОСТИКА:
    kubectl describe pod <name> -n demo
    # Events: Readiness probe failed: ...
    # Events: Liveness probe failed: ...

    kubectl logs <name> -n demo
    # Логи покажут, что приложение делает при probe.

    kubectl exec -it <name> -n demo -- curl localhost:8080/ready

  РЕШЕНИЕ:
    • Проверить, что эндпоинт отвечает.
    • Увеличить initialDelaySeconds.
    • Настроить startupProbe для медленных приложений.
    • Разделить liveness и readiness (liveness не проверяет БД).

  ВАЖНО ДЛЯ GO: readiness должен проверять зависимости (БД, Redis), liveness — только себя. Если liveness проверяет БД и БД упала — kubelet начнёт перезапускать Pod'ы, что не поможет.

  15. NOTREADY — НОДА НЕДОСТУПНА

  СИМПТОМ:
    kubectl get nodes
    # NAME     STATUS     ROLES    AGE   VERSION
    # node-1   NotReady   worker   10d   v1.28.0

  ЧТО ЗНАЧИТ: kubelet на ноде не отвечает или нода под давлением.

  ПРИЧИНЫ:
    • kubelet упал.
    • Container runtime (containerd) упал.
    • Сеть на ноде проблемы.
    • Диск полный (DiskPressure).
    • Память кончилась (MemoryPressure).
    • Проблемы с сертификатами.

  ДИАГНОСТИКА:
    kubectl describe node <name>
    # Смотреть Conditions: Ready, MemoryPressure, DiskPressure, PIDPressure.
    # Смотреть Events.

    На ноде (ssh):
      systemctl status kubelet
      systemctl status containerd
      journalctl -u kubelet -n 100
      df -h

  РЕШЕНИЕ:
    • Перезапустить kubelet: systemctl restart kubelet.
    • Перезапустить containerd.
    • Почистить диск.
    • Проверить сеть.

  ЧТО ПРОИСХОДИТ С POD'АМИ:
    Через 40 секунд нода помечается NotReady.
    Через 5 минут Node Controller выселяет Pod'ы.
    Deployment создаёт новые на других нодах.

  ВАЖНО: NotReady — проблема уровня ноды. Смотреть describe node, не describe pod.

  16. INIT:ERROR И INIT:CRASHLOOPBACKOFF

  СИМПТОМ:
    kubectl get pods -n demo
    # NAME          READY   STATUS                RESTARTS   AGE
    # app-abc123    0/1     Init:Error            0          2m
    # app-def456    0/1     Init:CrashLoopBackOff 3          5m

  ЧТО ЗНАЧИТ: один из init-контейнеров падает. Основной контейнер не запустится, пока init не завершится успешно.

  ПРИЧИНЫ:
    • Init-контейнер не может подключиться к БД (для миграций).
    • Неправильная команда в init.
    • Нет прав.
    • Долгий wait, а timeout истёк.

  ДИАГНОСТИКА:
    kubectl describe pod <name> -n demo
    # Смотреть Events, Last State init-контейнера.

    kubectl logs <name> -n demo -c <init-container-name>
    kubectl logs <name> -n demo -c <init-container-name> --previous

  РЕШЕНИЕ:
    • Исправить init-контейнер.
    • Проверить команду.
    • Проверить зависимости (БД, DNS).

  ВАЖНО: init-контейнеры запускаются последовательно. Если первый упал — второй не запустится. Основной контейнер не запустится вообще.

  17. СИСТЕМНЫЙ ПОДХОД К ДИАГНОСТИКЕ
  Когда Pod не работает — иди по шагам.

  ШАГ 1: kubectl get pods -n demo
    Что видим? Pending, Running, CrashLoopBackOff, ImagePullBackOff?

  ШАГ 2: kubectl describe pod <name> -n demo
    Смотрим Events внизу. Обычно там ответ.

  ШАГ 3: kubectl logs <name> -n demo
    Если контейнер запускался — смотрим логи.

  ШАГ 4: kubectl logs <name> -n demo --previous
    Если контейнер перезапускался — смотрим логи предыдущего.

  ШАГ 5: kubectl get events -n demo --sort-by=.metadata.creationTimestamp
    Если describe не помог — смотрим все события.

  ШАГ 6: kubectl exec -it <name> -n demo -- sh
    Если контейнер Running, но что-то не так — заходим внутрь.

  ШАГ 7: kubectl debug -it <name> --image=alpine -n demo
    Если контейнер distroless (нет sh) — debug.

  ШАГ 8: kubectl describe node <node>
    Если проблема на уровне ноды (Evicted, NotReady).

  ПРАВИЛО: 90% проблем решаются на шагах 1-3. Остальные 10% — на шагах 4-8.

  18. ИНСТРУМЕНТЫ: DESCRIBE, LOGS, EVENTS, DEBUG, K9S
  kubectl describe — главный инструмент.
    kubectl describe pod <name>
    kubectl describe node <name>
    kubectl describe deployment <name>

  kubectl logs — логи.
    kubectl logs <name> -n demo
    kubectl logs <name> -n demo --previous
    kubectl logs -l app=app -n demo --all-containers

  kubectl get events — события.
    kubectl get events -n demo --sort-by=.metadata.creationTimestamp
    kubectl get events -n demo --field-selector type=Warning

  kubectl debug — отладка distroless.
    kubectl debug -it <name> --image=alpine --target=app -n demo

  kubectl exec — зайти внутрь.
    kubectl exec -it <name> -n demo -- sh

  k9s — TUI для K8s. Быстрая навигация.
    k9s -n demo
    # :pods, :deploy, :events, :nodes — быстрые команды.

  stern — логи нескольких Pod'ов сразу.
    stern app -n demo
    stern -l app=app -n demo --tail=50

  ПРАВИЛО: k9s + stern + kubectl describe покрывают 99% диагностики.

  19. СВЯЗЬ С GO

  Что нужно от Go-разработчика, чтобы избежать типовых проблем:

  1. ВАЛИДАЦИЯ ENV ПРИ СТАРТЕ.
     Не паниковать с nil pointer. Падать с понятной ошибкой.

     ПРИМЕР:
       func loadConfig() (Config, error) {
           dbHost := os.Getenv("DB_HOST")
           if dbHost == "" {
               return Config{}, errors.New("DB_HOST is required")
           }
           return Config{DBHost: dbHost}, nil
       }

  2. RETRY ПОДКЛЮЧЕНИЯ К БД.
     БД может стартовать медленнее. Не падать с первой попытки.

     ПРИМЕР:
       func connectDB(ctx context.Context) (*sql.DB, error) {
           var db *sql.DB
           var err error
           for i := 0; i < 10; i++ {
               db, err = sql.Open("postgres", dsn)
               if err == nil {
                   if err = db.PingContext(ctx); err == nil {
                       return db, nil
                   }
               }
               time.Sleep(2 * time.Second)
           }
           return nil, err
       }

  3. GOMEMLIMIT.
     Установить на 10-20% меньше limits.memory.

     ПРИМЕР:
       env:
       - name: GOMEMLIMIT
         value: "230MiB"
       resources:
         limits:
           memory: "256Mi"

  4. GRACEFUL SHUTDOWN.
     Ловить SIGTERM, завершать активные запросы, закрывать соединения.

     ПРИМЕР:
       ctx, cancel := signal.NotifyContext(
           context.Background(), syscall.SIGTERM)
       defer cancel()

       <-ctx.Done()
       srv.Shutdown(shutdownCtx)

  5. ЛОГИ В STDOUT.
     Не в файл. K8s соберёт сам.

     ПРИМЕР:
       slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

  6. HEALTH И READY.
     /health — всегда 200. /ready — проверяет зависимости.

  7. НЕ ЛОГИРОВАТЬ СЕКРЕТЫ.
     Пароли, токены, PII — никогда.

  ЧТО ЧАСТО ЛОМАЕТСЯ В GO-СЕРВИСАХ В K8S:
    • Паника при старте из-за отсутствующего ENV.
    • OOMKilled из-за отсутствия GOMEMLIMIT.
    • Обрыв запросов без graceful shutdown.
    • Readiness не проходит, потому что БД медленно стартует.

  ПРАВИЛО: Go-сервис в K8s должен быть «хорошим гражданином»: валидировать конфиг, retry, graceful shutdown, GOMEMLIMIT, stdout logs, probes.

  20. АНТИПАТТЕРНЫ
  20.1. СМОТРЕТЬ ТОЛЬКО GET PODS, НЕ ИДТИ В DESCRIBE. 90% ответов — в Events.
  20.2. НЕ СМОТРЕТЬ ЛОГИ ПРЕДЫДУЩЕГО КОНТЕЙНЕРА. --previous — обязателен для CrashLoopBackOff.
  20.3. ПАНИКА ВМЕСТО ОШИБКИ ПРИ ВАЛИДАЦИИ ENV. Понятная ошибка > panic.
  20.4. НЕТ RETRY ПОДКЛЮЧЕНИЯ К БД. БД стартует медленнее — сервис падает.
  20.5. НЕТ GOMEMLIMIT. OOMKilled при limits.memory, хотя heap мог бы жить.
  20.6. НЕТ GRACEFUL SHUTDOWN. Запросы обрываются при rolling update.
  20.7. ЛОГИ В ФАЙЛ. kubectl logs пусто.
  20.8. LIVENESS ПРОВЕРЯЕТ БД. Каскадные рестарты при недоступности БД.
  20.9. READINESS НЕ ПРОВЕРЯЕТ НИЧЕГО. Трафик идёт в Pod, который не готов.
  20.10. requests.cpu БЕЗ limits.memory. Scheduler не знает реальных потребностей.
  20.11. IMAGE LATEST. Каждый pull — новая версия.
  20.12. НЕТ REQUESTS. BestEffort QoS — первый на Evicted.
  20.13. ИГНОРИРОВАТЬ EVENTS. Events отвечают быстрее, чем логи.
  20.14. FORCE DELETE POD'ОВ. Оставляет ресурсы на ноде.
  20.15. НЕ СМОТРЕТЬ DESCRIBE NODE. Evicted/NotReady — проблема ноды, а не Pod'а.
  20.16. СЕКРЕТЫ В ЛОГАХ. Даже в debug.
  20.17. ИГНОРИРОВАТЬ GOMEMLIMIT. Go не знает про cgroup limits.

  21. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  90% времени в K8s — диагностика. Умение быстро разобраться — ключевой навык.
  2.  kubectl describe — главный инструмент. Events внизу отвечают на большинство вопросов.
  3.  CrashLoopBackOff — контейнер падает. Логи: kubectl logs --previous.
  4.  ImagePullBackOff — образ не тянется. Проверить имя, тег, imagePullSecret.
  5.  OOMKilled — превышен limits.memory. Exit 137. GOMEMLIMIT для Go.
  6.  Pending — Scheduler не может разместить. Проверить requests, taints, nodeSelector, PVC.
  7.  Evicted — нода под давлением. Смотреть describe node, не describe pod.
  8.  ContainerCreating долго — volume не монтируется или образ тянется.
  9.  Terminating застрял — finalizers или процесс не отвечает на SIGTERM.
  10. CreateContainerConfigError — ConfigMap/Secret не найден.
  11. FailedMount — PVC/StorageClass проблема.
  12. RunContainerError — неправильная команда.
  13. Unhealthy — probes не проходят. Проверить /health и /ready.
  14. NotReady — нода недоступна. kubelet, containerd, диск, сеть.
  15. Init:Error — init-контейнер упал. Логи init-контейнера.
  16. Системный подход: get pods → describe pod → logs → logs --previous → events → exec/debug.
  17. Инструменты: describe, logs, events, debug, k9s, stern.
  18. Go-сервис: валидация ENV, retry, GOMEMLIMIT, graceful shutdown, stdout logs, probes.
*/
