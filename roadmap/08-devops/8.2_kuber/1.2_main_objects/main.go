package main

/*
  УРОК 1.2: ОСНОВНЫЕ ОБЪЕКТЫ KUBERNETES
  В K8s всё — объект. Pod, Deployment, Service, ConfigMap —
  каждый описан в YAML, хранится в etcd, управляется через
  API Server. Понимание объектов — база для работы с K8s.

  Объекты делятся на группы:
    WORKLOADS       — что запускать (Pod, Deployment, StatefulSet).
    NETWORKING      — как общаться (Service, Ingress).
    CONFIG          — конфиги и секреты (ConfigMap, Secret).
    ISOLATION       — изоляция (Namespace).
    BATCH           — одноразовые задачи (Job, CronJob).

  СОДЕРЖАНИЕ:
    1.  Pod — минимальная единица
    2.  Deployment и ReplicaSet
    3.  StatefulSet
    4.  DaemonSet (кратко)
    5.  Job и CronJob
    6.  Service — стабильный доступ
    7.  Ingress
    8.  ConfigMap
    9.  Secret
    10. Namespace
    11. Как объекты связаны
    12. Иерархия объектов
    13. Финальные выводы

  1. POD — МИНИМАЛЬНАЯ ЕДИНИЦА
  Pod — то, что реально запускается в K8s. Не контейнер, а
  группа контейнеров, разделяющих network namespace и volumes.

  ЧТО ВНУТРИ:
    • Один или несколько контейнеров.
    • Общий сетевой namespace — один IP на Pod.
    • Общие volumes.
    • Один lifecycle — упал Pod, упали все контейнеры.

  ЗАЧЕМ ГРУППА, А НЕ ОДИН КОНТЕЙНЕР:
    Sidecar-паттерн. Рядом с основным контейнером — вспомогательный:
      • Log shipper (fluentbit).
      • Proxy (istio-proxy).
      • Metrics exporter.
      • Init-контейнер для подготовки.

  ПРИМЕР POD С ДВУМЯ КОНТЕЙНЕРАМИ:
    apiVersion: v1
    kind: Pod
    metadata:
      name: app
    spec:
      containers:
      - name: app
        image: my-app:1.0
        ports:
        - containerPort: 8080
      - name: log-sidecar
        image: fluentbit:latest
        volumeMounts:
        - name: logs
          mountPath: /var/log
      volumes:
      - name: logs
        emptyDir: {}

  Оба контейнера видят /var/log — логи пишутся в общий volume,
  sidecar их читает и отправляет.

  ЧЕГО НЕ ДЕЛАЮТ С POD'АМИ НАПРЯМУЮ:
    • Не создают Pod'ы руками в проде.
    • Не обновляют их.
    • Не управляют репликами.

    Pod — эфемерный. Если упал — он пропал. Deployment создаст
    новый.

  ПОДХОДЫ К ЖИЗНЕННОМУ ЦИКЛУ:
    Init-контейнеры — запускаются ДО основного, по порядку.
    Используются для: дождаться БД, применить миграции, подготовить файлы.

    Обычные контейнеры — работают параллельно.
    Ephemeral-контейнеры — временные, для отладки работающего Pod'а (kubectl debug).

  СТАТУСЫ POD:
    Pending     — создан, но ещё не назначен ноде.
    Running     — запущен, работает.
    Succeeded   — завершился с кодом 0.
    Failed      — завершился с ошибкой.
    Unknown     — нода потеряла связь с API Server.

  ЧТО ВАЖНО:
    Pod — не контейнер. Это группа контейнеров с общим IP.
    Pod эфемерный. Его не создают руками в проде.

  2. DEPLOYMENT И REPLICASET
  Deployment — основной способ запускать stateless-приложения.
  Управляет репликами Pod'ов.

  ИЕРАРХИЯ:
    Deployment
      └─ ReplicaSet
           ├─ Pod
           ├─ Pod
           └─ Pod

  Ты пишешь Deployment. K8s создаёт ReplicaSet. ReplicaSet
  создаёт Pod'ы.

  ЗАЧЕМ ДВА УРОВНЯ:
    • Deployment управляет версиями (rolling update).
    • ReplicaSet обеспечивает количество Pod'ов.
    • При обновлении Deployment создаёт НОВЫЙ ReplicaSet,
      старый остаётся для отката.

  ПРИМЕР:
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: app
    spec:
      replicas: 3
      selector:
        matchLabels:
          app: app
      template:
        metadata:
          labels:
            app: app
        spec:
          containers:
          - name: app
            image: my-app:1.0
            ports:
            - containerPort: 8080

  РАЗБОР:
    replicas: 3       — три Pod'а.
    selector          — какие Pod'ы считать «своими» (по labels).
    template          — шаблон для создания Pod'ов.
    labels.app: app   — должны совпадать с selector.

  АНТИПАТТЕРН: labels в template и selector не совпадают.

    Ошибка: «selector does not match template labels».
    ReplicaSet не понимает, какие Pod'ы его.

  СТРАТЕГИИ ОБНОВЛЕНИЯ:

    ROLLING UPDATE (дефолт):
      • Постепенно заменяет Pod'ы.
      • Всегда есть живые.
      • maxSurge: сколько можно добавить.
      • maxUnavailable: сколько можно убрать.

    RECREATE:
      • Убить все старые, создать новые.
      • Есть downtime.
      • Для несовместимых версий.

  КОМАНДЫ:
    kubectl get deployments
    kubectl describe deployment app
    kubectl scale deployment app --replicas=5
    kubectl set image deployment/app app=my-app:2.0
    kubectl rollout status deployment/app
    kubectl rollout undo deployment/app
    kubectl rollout history deployment/app

  ЧТО ВАЖНО :
    Deployment → ReplicaSet → Pod. Deployment управляет
    обновлениями, ReplicaSet — количеством. Rolling update
    через maxSurge и maxUnavailable.

  3. STATEFULSET
  StatefulSet — для приложений с состоянием. БД, Kafka,
  Elasticsearch, Zookeeper.

  ЧЕМ ОТЛИЧАЕТСЯ ОТ DEPLOYMENT:

    СТАБИЛЬНЫЕ ИМЕНА:
      Deployment:    app-6b9d8f7c4-x9k2p (случайные)
      StatefulSet:   postgres-0, postgres-1, postgres-2

    СТАБИЛЬНЫЕ VOLUMES:
      Каждый Pod получает свой PVC. При перезапуске — тот же volume.

    ПОРЯДОК ЗАПУСКА:
      StatefulSet: 0 → 1 → 2 (по одному).
      Deployment: все параллельно.

    ПОРЯДОК ОСТАНОВКИ:
      StatefulSet: 2 → 1 → 0 (обратный).

    HEADLESS SERVICE:
      Для StatefulSet используется clusterIP: None.
      DNS: postgres-0.postgres, postgres-1.postgres.

  ЗАЧЕМ:
    БД нужны стабильные имена, чтобы реплики знали адрес
    мастера. Volume должен пережить перезапуск Pod'а.
    Порядок запуска важен для инициализации.

  ПРИМЕР:
    apiVersion: apps/v1
    kind: StatefulSet
    metadata:
      name: postgres
    spec:
      serviceName: postgres
      replicas: 3
      selector:
        matchLabels:
          app: postgres
      template:
        metadata:
          labels:
            app: postgres
        spec:
          containers:
          - name: postgres
            image: postgres:16
            volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
      volumeClaimTemplates:
      - metadata:
          name: data
        spec:
          accessModes: ["ReadWriteOnce"]
          resources:
            requests:
              storage: 10Gi

  ЧТО ВАЖНО:
    StatefulSet — стабильные имена (pod-0), стабильные volumes,
    порядок запуска. Для БД. На практике чаще используют
    Managed Postgres (RDS, Cloud SQL), а не крутят
    StatefulSet руками.

  4. DAEMONSET (КРАТКО)
  DaemonSet запускает по одному Pod'у на каждой ноде.

  ЗАЧЕМ:
    • Лог-агенты (fluentbit, filebeat).
    • Мониторинг (node-exporter).
    • Network plugin (Calico, Cilium).
    • Storage plugin.

  ПРИМЕР:
    apiVersion: apps/v1
    kind: DaemonSet
    metadata:
      name: node-exporter
    spec:
      selector:
        matchLabels:
          app: node-exporter
      template:
        metadata:
          labels:
            app: node-exporter
        spec:
          containers:
          - name: node-exporter
            image: prom/node-exporter

  ЧТО ВАЖНО:
    Ты редко пишешь DaemonSet руками. Обычно их ставят через
    Helm-чарты (kube-prometheus-stack, calico).

  5. JOB И CRONJOB
  JOB — одноразовая задача. Должна завершиться.

  ЗАЧЕМ:
    • Миграции БД перед деплоем.
    • Одноразовые скрипты (seed, backup, cleanup).
    • Batch-обработка.

  ПРИМЕР:
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: migrate
    spec:
      backoffLimit: 3            # максимум 3 попытки
      ttlSecondsAfterFinished: 300   # удалить через 5 мин
      template:
        spec:
          restartPolicy: OnFailure
          containers:
          - name: migrate
            image: my-app:1.0
            command: ["/app/migrate", "up"]

  ЧТО ВАЖНО:
    restartPolicy: OnFailure — если упадёт, Job перезапустит.
    backoffLimit — сколько попыток.
    ttlSecondsAfterFinished — автоудаление после завершения.

  CRONJOB — задача по расписанию.

    apiVersion: batch/v1
    kind: CronJob
    metadata:
      name: backup
    spec:
      schedule: "0 3 * * *"      # каждый день в 3 утра
      jobTemplate:
        spec:
          template:
            spec:
              restartPolicy: OnFailure
              containers:
              - name: backup
                image: backup:1.0

  ЧТО ВАЖНО:
    Формат schedule — как в cron: минута час день месяц день_недели.
    concurrencyPolicy — что делать, если предыдущий Job ещё
    выполняется (Allow, Forbid, Replace).

  6. SERVICE — СТАБИЛЬНЫЙ ДОСТУП
  Pod'ы эфемерны. IP меняется при каждом пересоздании.
  Service даёт стабильный адрес.

  ТИПЫ:

    CLUSTERIP (дефолт):
      • Внутренний IP.
      • Доступен только внутри кластера.
      • Для общения сервисов.

    NODEPORT:
      • Открывает порт на каждой ноде (30000-32767).
      • Доступен снаружи по <NodeIP>:<port>.
      • Неудобно, но работает.

    LOADBALANCER:
      • Облачный балансировщик (ELB, ALB).
      • Публичный IP.
      • Дорого (обычно 1 LB = 1 сервис).

    EXTERNALNAME:
      • CNAME на внешний DNS.
      • Не проксирует трафик.

    HEADLESS (clusterIP: None):
      • Отдаёт IP всех Pod'ов.
      • Для StatefulSet.

  КАК РАБОТАЕТ CLUSTERIP:
    apiVersion: v1
    kind: Service
    metadata:
      name: app
    spec:
      type: ClusterIP
      selector:
        app: app
      ports:
      - port: 80             # порт Service
        targetPort: 8080     # порт Pod'а

  ЧТО ПРОИСХОДИТ:
    1. Service создаётся с виртуальным IP (10.96.x.x).
    2. kube-proxy на каждой ноде настраивает iptables.
    3. Трафик на ClusterIP перенаправляется на Pod'ы.
    4. Балансировка — round-robin.

  ENDPOINTS:
    Service находит Pod'ы по selector. Создаёт объект Endpoints
    со списком IP.

      kubectl get endpoints app
      # NAME   ENDPOINTS                          AGE
      # app    10.244.1.5:8080,10.244.2.7:8080   5s

    Если Endpoints пустой — selector не совпадает с labels.

  DNS ВНУТРИ КЛАСТЕРА:
    <service>.<namespace>.svc.cluster.local

    Пример: postgres.default.svc.cluster.local
    Внутри того же namespace: просто postgres.

  ЧТО ВАЖНО:
    Service — стабильный IP, балансировка, DNS. ClusterIP для
    внутреннего, LoadBalancer для внешнего. Headless для
    StatefulSet. Endpoints обновляются автоматически по selector.

  7. INGRESS
  Ingress — HTTP-роутинг снаружи. Один вход для множества
  сервисов.

  ЧЕМ ОТЛИЧАЕТСЯ ОТ SERVICE:
    Service LoadBalancer — 1 LB на сервис. Дорого.
    Ingress — 1 LB на весь кластер, роутинг по хостам/путям.

  ПРИМЕР:
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: app
      annotations:
        nginx.ingress.kubernetes.io/proxy-body-size: "10m"
    spec:
      ingressClassName: nginx
      rules:
      - host: api.example.com
        http:
          paths:
          - path: /orders
            pathType: Prefix
            backend:
              service:
                name: order-service
                port:
                  number: 80
          - path: /users
            pathType: Prefix
            backend:
              service:
                name: user-service
                port:
                  number: 80
      tls:
      - hosts:
        - api.example.com
        secretName: api-tls

  INGRESS CONTROLLER:
    Ingress — только правила. Чтобы они работали, нужен
    Ingress Controller: nginx-ingress, Traefik, HAProxy.

    В облаке (EKS, GKE) обычно уже стоит.

  ЧТО ВАЖНО:
    Ingress = HTTP-роутинг по хостам и путям. Нужен
    Ingress Controller. TLS через Secret. Один LB на весь кластер.

  8. CONFIGMAP
  ConfigMap — конфиги отдельно от образа.

  ЗАЧЕМ:
    Один образ — разные окружения. Не пересобирать для
    каждого env.

  ПРИМЕР:
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: app-config
    data:
      APP_ENV: "production"
      LOG_LEVEL: "info"
      DB_HOST: "postgres.default.svc.cluster.local"

  ИСПОЛЬЗОВАНИЕ КАК ENV:
    spec:
      containers:
      - name: app
        image: my-app:1.0
        envFrom:
        - configMapRef:
            name: app-config
        # или отдельные ключи:
        env:
        - name: APP_ENV
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: APP_ENV

  ИСПОЛЬЗОВАНИЕ КАК VOLUME:
    spec:
      containers:
      - name: app
        volumeMounts:
        - name: config
          mountPath: /etc/app
      volumes:
      - name: config
        configMap:
          name: app-config

    Файлы появятся в /etc/app/: APP_ENV, LOG_LEVEL, DB_HOST.

  ЧТО ВАЖНО:
    • ConfigMap для несекретных конфигов.
    • Можно как env, так и volume.
    • При volume — hot reload (файлы обновляются).
    • При env — нужен рестарт Pod'а.

  9. SECRET
  Secret — для паролей, токенов, сертификатов.

  ОТЛИЧИЕ ОТ CONFIGMAP:
    • Хранится в base64 (не шифрование!).
    • Можно настроить шифрование at rest (encryption config).
    • Отдельный RBAC.
    • Ограничение размера — 1 МБ.

  ПРИМЕР:
    apiVersion: v1
    kind: Secret
    metadata:
      name: app-secrets
    type: Opaque
    stringData:
      DB_USER: "app"
      DB_PASSWORD: "supersecret"
    # или:
    data:
      DB_USER: YXBw              # base64 от "app"
      DB_PASSWORD: c3VwZXJzZWNyZXQ=

  stringData — автоматически кодирует в base64.
  data — уже закодированные значения.

  ТИПЫ:
    Opaque             — обычный (дефолт).
    kubernetes.io/tls  — TLS-сертификат.
    kubernetes.io/dockerconfigjson — credentials для registry.

  ИСПОЛЬЗОВАНИЕ:
    spec:
      containers:
      - name: app
        envFrom:
        - secretRef:
            name: app-secrets

  ЧТО ВАЖНО НА СОБЕСЕ:
    Secret — base64, не шифрование. Для паролей. В проде —
    Sealed Secrets, External Secrets или Vault.

  10. NAMESPACE
  Namespace — логическая изоляция.

  ЗАЧЕМ:
    • Разделить команды (team-a, team-b).
    • Разделить окружения (dev, staging, prod).
    • Квоты и RBAC на namespace.

  ДЕФОЛТНЫЕ:
    default       — для всего, если не указано.
    kube-system   — системные компоненты K8s.
    kube-public   — публичные ресурсы.
    kube-node-lease — heartbeat нод.

  ЧТО ВНУТРИ NAMESPACE:
    • Pod, Deployment, StatefulSet.
    • Service, Ingress.
    • ConfigMap, Secret.
    • Job, CronJob.

  ЧТО ВНЕ NAMESPACE:
    • Node.
    • PersistentVolume.
    • StorageClass.
    • ClusterRole / ClusterRoleBinding.

  КОМАНДЫ:
    kubectl get namespaces
    kubectl create namespace app
    kubectl get pods -n app
    kubectl config set-context --current --namespace=app

  QUOTA НА NAMESPACE:
    apiVersion: v1
    kind: ResourceQuota
    metadata:
      name: app-quota
      namespace: app
    spec:
      hard:
        requests.cpu: "10"
        requests.memory: 20Gi
        limits.cpu: "20"
        limits.memory: 40Gi
        pods: "50"

  11. КАК ОБЪЕКТЫ СВЯЗАНЫ

  СХЕМА ТИПИЧНОГО ПРИЛОЖЕНИЯ:
    ┌──────────────────────────────────┐
    │ Namespace: production            │
    │                                  │
    │  Ingress (api.example.com)       │
    │     │                            │
    │     ▼                            │
    │  Service (app) ◄─────┐           │
    │     │                │           │
    │     ▼                │           │
    │  Endpoints ──────────┘           │
    │     │                            │
    │     ▼                            │
    │  Deployment (app)                │
    │     │                            │
    │     ▼                            │
    │  ReplicaSet                      │
    │     │                            │
    │     ├─ Pod ── uses ── ConfigMap  │
    │     ├─ Pod ── uses ── Secret     │
    │     └─ Pod                       │
    └──────────────────────────────────┘

  СВЯЗИ:
    Ingress → Service по имени.
    Service → Endpoints по selector.
    Endpoints → Pod по labels.
    Deployment → ReplicaSet по ownerReferences.
    ReplicaSet → Pod по ownerReferences.
    Pod → ConfigMap/Secret по именам.

  КЛЮЧЕВОЕ:
    Всё связано через labels и имена. Не через ID.

  12. ИЕРАРХИЯ ОБЪЕКТОВ

  УРОВНИ:
    Cluster-wide:   Namespace, Node, PersistentVolume,
                    StorageClass, ClusterRole.

    Namespaced:     Pod, Deployment, StatefulSet, DaemonSet,
                    Job, CronJob, Service, Ingress,
                    ConfigMap, Secret.

    Внутри Pod:     Containers, Volumes, InitContainers.

  ИЕРАРХИЯ WORKLOADS:
    Deployment → ReplicaSet → Pod → Containers
    StatefulSet → Pod → Containers
    DaemonSet → Pod (по одному на ноду)
    Job → Pod → Containers

  ВЛАДЕЛЬЦЫ (ownerReferences):
    Каждый объект знает, кто его создал.
    Pod знает, что его создал ReplicaSet.
    ReplicaSet знает, что его создал Deployment.
    При удалении родителя K8s каскадно удаляет детей.
    Удалил Deployment — убираются ReplicaSet и Pod'ы.

  13. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Pod — группа контейнеров с общим IP и volumes. Не
      контейнер. Эфемерный.
  2.  Deployment → ReplicaSet → Pod. Rolling update через
      Deployment. Для stateless.
  3.  StatefulSet — стабильные имена (pod-0), volumes,
      порядок запуска. Для БД.
  4.  DaemonSet — по одному Pod'у на ноду. Для логов,
      мониторинга, network plugin.
  5.  Job — одноразовая задача (миграции). CronJob — по расписанию.
  6.  Service — стабильный доступ. ClusterIP, NodePort,
      LoadBalancer, Headless. Endpoints по selector.
  7.  Ingress — HTTP-роутинг. Нужен Ingress Controller.
      TLS через Secret.
  8.  ConfigMap — конфиги. Secret — base64, для паролей.
  9.  Namespace — изоляция, квоты, RBAC.
  10. Объекты связаны через labels и имена, не через ID.
  11. Owner references → каскадное удаление. Удалил
      Deployment — убрались ReplicaSet и Pod'ы.
*/
