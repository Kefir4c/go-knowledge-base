package main

/*
  УРОК 2.1: POD — ЧТО ЭТО
  Pod — минимальная единица в K8s. Не контейнер, не виртуалка,
  а группа контейнеров с общим сетевым namespace и общими
  volumes. Всё, что ты деплоишь в K8s, рано или поздно становится Pod'ом.
  Почему не просто «контейнер»? Потому что в реальных системах
  почти всегда рядом с приложением едет ещё что-то: лог-агент,
  прокси, метрики. Можно было бы запустить их отдельно, но
  тогда они не разделят сеть и данные. Pod решает это: несколько
  контейнеров внутри Pod'а видят друг друга по localhost и
  могут шарить volumes.

  СОДЕРЖАНИЕ:
    1.  Почему Pod, а не контейнер
    2.  Что общего у контейнеров в Pod'е
    3.  Один контейнер — самый частый случай
    4.  Sidecar — вспомогательный контейнер
    5.  Init-контейнеры — подготовка перед стартом
    6.  Ephemeral containers — отладка живого Pod'а
    7.  Жизненный цикл Pod'а
    8.  Статусы Pod'а
    9.  Restart policy
    10. Pod template и его роль
    11. Почему Pod не создают руками в проде
    12. Связь с Go
    13. Антипаттерны
    14. Финальные выводы

  1. ПОЧЕМУ POD, А НЕ КОНТЕЙНЕР
  В Docker контейнер — единица. В K8s единица — Pod.

  ПРИЧИНА: в реальном приложении рядом с основным процессом
  всегда едет что-то ещё.

  ПРИМЕРЫ:
    • Лог-агент (fluentbit, filebeat) читает логи из общей
      папки и отправляет в Elasticsearch.
    • Service mesh прокси (Envoy, istio-proxy) перехватывает
      трафик приложения.
    • Metrics exporter (node-exporter) отдаёт метрики по HTTP.
    • Backup-агент периодически снимает дампы.
    • Init-скрипт применяет миграции перед стартом.

  Если запустить всё это отдельными контейнерами — придётся
  настраивать сеть между ними, volumes, синхронизацию. K8s
  решает это через Pod: контейнеры в одном Pod'е автоматически
  делят network namespace и могут монтировать общие volumes.

  2. ЧТО ОБЩЕГО У КОНТЕЙНЕРОВ В POD'Е

  ТРИ ВЕЩИ ДЕЛЯТСЯ МЕЖДУ ВСЕМИ КОНТЕЙНЕРАМИ POD'А:

    NETWORK NAMESPACE:
      • Один IP на весь Pod.
      • Один набор портов.
      • Контейнеры видят друг друга через localhost.
      • Нельзя двум контейнерам слушать один порт.

    VOLUMES:
      • Можно смонтировать один volume в несколько контейнеров.
      • Один пишет — другой читает.
      • Пример: app пишет логи, fluentbit читает.

    LIFECYCLE:
      • Pod — единица. Упал Pod — упали все контейнеры.
      • Удаляется Pod — удаляются все контейнеры.
      • Нельзя убить один контейнер, оставив остальные.

  ЧТО НЕ ДЕЛИТСЯ:
    • Файловая система — у каждого контейнера своя.
    • Процессы — каждый контейнер изолирован.
    • PID namespace — по умолчанию у каждого свой.

  СХЕМА:
    ┌──────────────────────────────────────┐
    │              POD                     │
    │  IP: 10.244.1.5                      │
    │  ┌──────────────┐  ┌──────────────┐  │
    │  │   app        │  │  sidecar     │  │
    │  │  :8080       │  │  :9090       │  │
    │  └──────┬───────┘  └──────┬───────┘  │
    │         │                 │          │
    │         └────────┬────────┘          │
    │                  │                   │
    │         ┌────────▼────────┐          │
    │         │  shared volume  │          │
    │         └─────────────────┘          │
    └──────────────────────────────────────┘

  3. ОДИН КОНТЕЙНЕР — САМЫЙ ЧАСТЫЙ СЛУЧАЙ
  90% Pod'ов в проде — один контейнер. Go-сервис, nginx, postgres.

  ПРИМЕР МИНИМАЛЬНОГО POD'А:
    apiVersion: v1
    kind: Pod
    metadata:
      name: app
      labels:
        app: app
    spec:
      containers:
      - name: app
        image: my-app:1.0
        ports:
        - containerPort: 8080

  ЧТО ТУТ:
    • Один контейнер с именем app.
    • Образ my-app:1.0.
    • Слушает 8080 внутри Pod'а.
    • Pod получит IP от CNI (например, 10.244.1.5).

  ОБРАЩЕНИЕ:
    Из другого Pod'а: http://10.244.1.5:8080.
    Из другого контейнера того же Pod'а: http://localhost:8080.

  4. SIDECAR — ВСПОМОГАТЕЛЬНЫЙ КОНТЕЙНЕР
  Sidecar — контейнер, который живёт рядом с основным и делает
  что-то полезное.

  ТИПИЧНЫЕ SIDECAR'Ы:

    LOG SHIPPER:
      • Читает логи из общей папки.
      • Отправляет в централизованное хранилище.
      • Пример: fluentbit, filebeat, vector.

    PROXY:
      • Перехватывает трафик приложения.
      • Делает mTLS, retries, circuit breaking.
      • Пример: Envoy, istio-proxy.

    METRICS EXPORTER:
      • Слушает метрики приложения.
      • Отдаёт их в формате Prometheus.
      • Пример: nginx-exporter, postgres-exporter.

    BACKUP AGENT:
      • Периодически делает дампы данных.
      • Пишет в S3 или другой storage.

  ПРИМЕР POD'А С SIDECAR:
    apiVersion: v1
    kind: Pod
    metadata:
      name: app-with-logs
    spec:
      containers:
      - name: app
        image: my-app:1.0
        volumeMounts:
        - name: logs
          mountPath: /var/log/app
      - name: log-shipper
        image: fluentbit:latest
        volumeMounts:
        - name: logs
          mountPath: /var/log/app
          readOnly: true
      volumes:
      - name: logs
        emptyDir: {}

  РАЗБОР:
    • Оба контейнера видят /var/log/app.
    • app пишет туда логи.
    • log-shipper читает и отправляет.
    • emptyDir удаляется вместе с Pod'ом.

  ВАЖНО:
    Sidecar'ы не создаются вручную в манифесте Deployment
    (хотя можно). Обычно их подсовывают через:
      • Service mesh (Istio) — добавляет sidecar автоматически.
      • Mutating admission webhook.
      • Helm-чарты.

  В ЦЕЛОМ:
    Sidecar — паттерн, а не встроенная сущность K8s. Это просто
    контейнер в том же Pod'е, который делит с основным сеть
    и volumes.

  5. INIT-КОНТЕЙНЕРЫ — ПОДГОТОВКА ПЕРЕД СТАРТОМ
  Init-контейнеры запускаются ДО основных контейнеров. Каждый
  выполняется до завершения (exit 0). Только после этого
  запускается следующий init или основной контейнер.

  ЗАЧЕМ:
    • Дождаться готовности БД (retry в init).
    • Применить миграции.
    • Скачать конфиги из внешнего источника.
    • Подготовить файлы, права.
    • Сгенерировать сертификаты.

  ПРИМЕР:
    apiVersion: v1
    kind: Pod
    metadata:
      name: app
    spec:
      initContainers:
      - name: wait-for-db
        image: busybox:1.36
        command:
        - sh
        - -c
        - |
          until nc -z postgres 5432; do
            echo "waiting for postgres..."
            sleep 1
          done
      - name: migrate
        image: my-app:1.0
        command: ["/app/migrate", "up"]
      containers:
      - name: app
        image: my-app:1.0
        ports:
        - containerPort: 8080

  ЧТО ПРОИСХОДИТ:
    1. Pod создан, статус Init:0/2.
    2. Запускается wait-for-db. Ждёт БД.
    3. Завершился. Статус Init:1/2.
    4. Запускается migrate. Применяет миграции.
    5. Завершился. Статус Init:2/2.
    6. Запускается app. Статус Running.

  КЛЮЧЕВЫЕ СВОЙСТВА:
    • Запускаются по порядку. Второй после первого.
    • Если упал — Pod перезапускает его (по restartPolicy).
    • Должны завершиться с кодом 0.
    • Не работают во время жизни Pod'а. Только для подготовки.
    • Могут использовать другой образ (busybox, migrator).

  ПРЕИМУЩЕСТВО ПЕРЕД RETRY В КОДЕ:
    Разделение ответственности. Основной контейнер не думает
    про «дождаться БД». Init это делает. Код сервиса чище.

  В ЦЕЛОМ:
    Init-контейнер — для подготовки. Дождаться БД, миграции,
    скачать конфиг. Запускается до основного, один за другим,
    должен завершиться с 0.

  6. EPHEMERAL CONTAINERS — ОТЛАДКА ЖИВОГО POD'А
  Ephemeral container — временный контейнер, который добавляется
  в работающий Pod для отладки. Появился в K8s 1.16+.

  ЗАЧЕМ:
    • Pod на distroless — нет sh, ls, curl.
    • Хочется зайти внутрь, но передеплой = downtime.
    • Хочется посмотреть сеть, процессы, файлы.

  КАК ЗАПУСКАЕТСЯ:
    kubectl debug -it <pod> --image=alpine --target=app

  ЧТО ПРОИСХОДИТ:
    • В Pod добавляется контейнер alpine.
    • Он видит тот же network namespace, что и app.
    • Может делиться process namespace (--target).
    • После выхода — исчезает.

  ПРИМЕР СЕССИИ:
    kubectl debug -it app-abc123 --image=alpine --target=app

    # Внутри:
    ps aux                # видишь процессы app
    ip addr               # видишь сеть Pod'а
    wget -q -O - http://localhost:8080/health

  ОГРАНИЧЕНИЯ:
    • Только для отладки. Не для продакшн-нагрузки.
    • Нельзя указать в манифесте Deployment. Только руками.
    • Не переживают рестарт Pod'а.

	В ЦЕЛОМ:
    Ephemeral containers — для отладки Pod'ов без shell
    (distroless, scratch). Запускаются через kubectl debug.

  7. ЖИЗНЕННЫЙ ЦИКЛ POD'А
  У Pod'а есть фазы. Каждая — определённое состояние.

    PENDING:
      Pod создан, но ещё не запущен. Ждёт Scheduler,
      ждёт pull образа, ждёт init-контейнеры.

    RUNNING:
      Хотя бы один основной контейнер работает.

    SUCCEEDED:
      Все контейнеры завершились с 0. Для Job'ов.

    FAILED:
      Хотя бы один контейнер завершился с != 0.

    UNKNOWN:
      Не удаётся получить статус (обычно проблема с нодой).

  СХЕМА:
    Pending ──► Running ──► Succeeded
                    │
                    └────► Failed

  ВАЖНО:
    Pending ≠ проблема. Может быть нормально при старте.
    Pending долго (> 30 сек) — проблема. Смотреть describe.

  8. СТАТУСЫ POD'А
  Помимо фазы, есть состояние контейнеров.
  Running — контейнер работает.
  Waiting — ждёт чего-то (pull, init).
  Terminated — завершился (успех или ошибка).

  ТИПОВЫЕ «ПЛОХИЕ» СТАТУСЫ:

    CrashLoopBackOff:
      Контейнер падает, K8s его перезапускает с backoff.
      Причина: ошибка в коде, не хватает ENV, БД недоступна.

      Диагностика:
        kubectl describe pod <name>
        kubectl logs <name> --previous

    ImagePullBackOff:
      Не может скачать образ.
      Причина: опечатка в имени, тега нет, приватный registry
      без secret.

      Диагностика:
        kubectl describe pod <name>
        # Смотреть Events: "Failed to pull image".

    ErrImagePull:
      То же самое, что ImagePullBackOff, но первая попытка.

    OOMKilled:
      Превышен limits.memory. Ядро убило процесс.
      Причина: утечка, мало памяти в limits.

      Диагностика:
        kubectl describe pod <name>
        # Last State: Terminated, Reason: OOMKilled

    ContainerCreating:
      Контейнер создаётся. Нормально, если недолго.

    Pending:
      Pod не размещён. Scheduler не нашёл ноду.

      Причина: не хватает ресурсов, taints, nodeSelector.

      Диагностика:
        kubectl describe pod <name>
        # Events: "0/3 nodes are available: 3 Insufficient cpu".

  9. RESTART POLICY
  restartPolicy определяет, что делать при падении контейнера.

  ALWAYS (дефолт):
    Всегда перезапускать. Даже после успешного завершения.
    Для долгоживущих сервисов.

  ON-FAILURE:
    Перезапускать только при ненулевом exit-коде.
    Для Job'ов.

  NEVER:
    Не перезапускать.
    Для одноразовых задач.

  В DEPLOYMENT'Е:
    restartPolicy всегда Always. Менять нельзя. Если хочешь
    одноразовую задачу — используй Job, не Deployment.

  В POD'Е:
    Можно задать любой. Но обычно Pod'ы запускают через
    Deployment/StatefulSet/Job, а не напрямую.

  10. POD TEMPLATE И ЕГО РОЛЬ
  Pod template — это spec Pod'а внутри другого объекта
  (Deployment, StatefulSet, Job).

  ПРИМЕР:
    apiVersion: apps/v1
    kind: Deployment
    spec:
      template:
        metadata:
          labels:
            app: app
        spec:
          containers:
          - name: app
            image: my-app:1.0

  ЧТО ЭТО ЗНАЧИТ:
    Deployment не создаёт Pod напрямую. Он создаёт ReplicaSet,
    который создаёт Pod'ы по шаблону template.

  ВАЖНО:
    • Изменение template → новая ревизия Deployment → rolling
      update.
    • Template описывает желаемое состояние Pod'а.
    • Все Pod'ы Deployment создаются из одного template.
    • Запущенные Pod'ы нельзя редактировать напрямую (только
      через template + rollout).

  11. ПОЧЕМУ POD НЕ СОЗДАЮТ РУКАМИ В ПРОДЕ

  В K8s можно создать Pod напрямую:
    apiVersion: v1
    kind: Pod
    metadata:
      name: app
    spec:
      containers:
      - name: app
        image: my-app:1.0

  Но в проде так не делают. Причины:
    • Pod не перезапустится при падении (только если укажешь
      restartPolicy, но всё равно уступает Deployment).
    • Pod не обновится при новой версии образа.
    • Pod не масштабируется.
    • Pod не раскидывается по нодам.
    • Удалил Pod — второй не появится.

  Правильный подход:
    Deployment → ReplicaSet → Pod'ы.
    StatefulSet → Pod'ы для stateful.
    DaemonSet → по одному Pod'у на ноду.
    Job → одноразовые Pod'ы.

  Создавать Pod напрямую — только для отладки или обучения.

  В ЦЕЛОМ:
    Pod'ы в проде создаются через контроллеры (Deployment,
    StatefulSet). Голый Pod — не управляется, не перезапускается,
    не обновляется.

  12. СВЯЗЬ С GO
  Go-сервис в Pod'е — это один контейнер.

  ЧТО ДЕЛАЕТ GO-РАЗРАБОТЧИК:
    • Пишет Dockerfile для сервиса.
    • В манифесте указывает образ, env, resources, probes.
    • Init-контейнер для миграций (тоже на Go, тот же бинарник).
    • Читает env (DB_HOST, REDIS_HOST) через os.Getenv.
    • Отдаёт /health для liveness, /ready для readiness.
    • Graceful shutdown по SIGTERM (см. урок про graceful shutdown).

  ТИПИЧНЫЙ MAНИФЕСТ:
    spec:
      initContainers:
      - name: migrate
        image: my-app:1.0
        command: ["/app/migrate", "up"]
        envFrom:
        - configMapRef:
            name: app-config
      containers:
      - name: app
        image: my-app:1.0
        command: ["/app", "server"]
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: app-config
        - secretRef:
            name: app-secrets
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "500m"
            memory: "256Mi"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080

  ЧТО ЗДЕСЬ GO-СПЕЦИФИКА:
    • Один бинарник — разные команды (server, migrate, worker).
    • Probes указывают на твои эндпоинты.
    • Env читается через os.Getenv.
    • Graceful shutdown работает через SIGTERM (kubelet
      посылает его при удалении Pod'а).

  POD И SIGTERM:
    При удалении Pod'а kubelet:
      1. Убирает Pod из Service endpoints (readiness падает).
      2. Посылает SIGTERM основным контейнерам.
      3. Ждёт terminationGracePeriodSeconds (дефолт 30).
      4. Если не завершился — SIGKILL.

    Твой Go-сервис должен:
      • Ловить SIGTERM через signal.NotifyContext.
      • Завершить активные запросы.
      • Закрыть соединения с БД.
      • Выйти с 0.

  13. АНТИПАТТЕРНЫ

  13.1. СОЗДАВАТЬ POD НАПРЯМУЮ В ПРОДЕ.
    Не перезапустится, не обновится, не отскейлится. Только
    через Deployment/StatefulSet/Job.

  13.2. НЕСКОЛЬКО КОНТЕЙНЕРОВ БЕЗ ПРИЧИНЫ.
    Sidecar нужен, если есть общая задача. Просто «два контейнера
    в одном Pod'е» — антипаттерн.

  13.3. ЛОГИ В ФАЙЛ ВМЕСТО STDOUT.
    K8s собирает stdout/stderr автоматически. Писать в файл —
    усложнять себе жизнь.

  13.4. НЕТ RESOURCES REQUESTS.
    Scheduler не знает, куда ставить Pod. BestEffort QoS —
    первый на eviction.

  13.5. PROBES ПРОВЕРЯЮТ БД.
    Liveness не должен проверять БД. Если БД упала — kubelet
    начнёт рестартить Pod'ы. БД от этого не поднимется.

  13.6. НЕТ PROBES.
    K8s не знает, готов ли Pod. Трафик идёт в мёртвый.

  13.7. INIT-КОНТЕЙНЕР, КОТОРЫЙ НИКОГДА НЕ ЗАВЕРШАЕТСЯ.
    Init должен завершиться. Если он висит — Pod застрянет
    в Init:0/N.

  13.8. EPHEMERAL CONTAINERS В МАНИФЕСТЕ.
    Их нельзя указать в Deployment. Только руками через
    kubectl debug.

  13.9. IGNORE ТЕРМИНАЦИИ.
    Если Go-сервис не ловит SIGTERM — kubelet ждёт 30 секунд
    и убивает SIGKILL. Соединения рвутся.

  13.10. ПОДМЕНА DEPLOYMENT НА POD.
    Если Pod с restartPolicy: Always — это не Deployment.
    Нет rolling update, нет истории ревизий.

  13.11. VOLUME В ПАМЯТИ ДЛЯ ДАННЫХ.
    emptyDir удаляется с Pod'ом. Для persistent-данных — PVC.

  13.12. IMAGE LATEST.
    Каждый pull может принести новую версию. Ломает
    воспроизводимость. Фиксируй тег или digest.

  14. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Pod — минимальная единица K8s. Группа контейнеров с
      общим IP, network namespace и volumes.
  2.  Почему Pod, а не контейнер: рядом с приложением нужен
      лог-агент, прокси, метрики.
  3.  Один контейнер в Pod'е — 90% случаев. Sidecar — когда
      нужен вспомогательный процесс.
  4.  Init-контейнеры — подготовка до старта основного
      (дождаться БД, миграции).
  5.  Ephemeral containers — для отладки через kubectl debug.
  6.  Фазы Pod'а: Pending, Running, Succeeded, Failed, Unknown.
  7.  Статусы контейнеров: Running, Waiting, Terminated.
      Плохие: CrashLoopBackOff, ImagePullBackOff, OOMKilled, Pending.
  8.  restartPolicy: Always, OnFailure, Never. В Deployment'е всегда Always.
  9.  Pod'ы в проде создаются через контроллеры (Deployment,
      StatefulSet, Job), не напрямую.
  10. Pod template — шаблон Pod'а внутри Deployment/StatefulSet.
  11. Для Go-сервиса: один контейнер, init для миграций,
      probes на /health и /ready, graceful shutdown по SIGTERM.
  12. kubelet посылает SIGTERM при удалении Pod'а. Ждёт
      terminationGracePeriodSeconds (дефолт 30). Если не
      завершился — SIGKILL.
  13. Антипаттерны: Pod напрямую в проде, несколько
      контейнеров без причины, probes проверяют БД, нет
      resources, image latest.
*/
