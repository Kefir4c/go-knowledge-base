package main

/*
  УРОК 5.1: NAMESPACES
  Кластер K8s — это не только контейнеры. Это ещё и организация:
  команды, окружения, проекты. Namespace — механизм логической
  изоляции. Один кластер делится на изолированные сегменты, где
  у каждой команды свои ресурсы, свои квоты, свои права.
  Без namespace всё лежит в одной куче. Разработчики видят чужие
  Secret, Pod'ы конфликтуют по именам, никто не знает, чьи это
  Deployment'ы. В большой компании это хаос.

  СОДЕРЖАНИЕ:
    1.  Зачем нужны Namespaces
    2.  Что внутри, что снаружи
    3.  Дефолтные Namespace
    4.  Создание и удаление
    5.  Переключение между Namespace
    6.  DNS между Namespace
    7.  Cross-namespace коммуникация
    8.  ResourceQuota — квоты
    9.  LimitRange — defaults
    10. RBAC на уровне Namespace
    11. ServiceAccount per namespace
    12. NetworkPolicy между namespace
    13. Multi-tenancy паттерны
    14. Cost allocation через namespace
    15. Удаление и каскад
    16. Debugging зависшего namespace
    17. Best practices
    18. Антипаттерны
    19. Финальные выводы

  1. ЗАЧЕМ НУЖНЫ NAMESPACES

  ПРОБЛЕМА: всё в одном кластере в одной куче.
    • Все Pod'ы в одном namespace — имена конфликтуют.
    • Все Secret видны всем — нет изоляции.
    • Нет квот — один сервис может занять все ресурсы.
    • Нет RBAC на границе — нельзя дать доступ «только к своим».

  РЕШЕНИЕ: Namespace.
    Namespace — это логическая изоляция внутри одного кластера.
    Не физическая. Все Pod'ы на тех же нодах, но:
      • Свои имена (можно создать app в namespace A и app в B).
      • Свои RBAC (разработчик A не видит namespace B).
      • Свои квоты (команда A ограничена 10 CPU, команда B — 20).
      • Свои Secret, ConfigMap, Service.

  ВАЖНО:
    Namespace — это НЕ физическая изоляция. Если хочется
    физической — отдельные кластеры. Namespace — для организации
    и мягкой изоляции.

  КОГДА ИСПОЛЬЗУЮТ:
    • Окружения: dev, staging, prod в одном кластере.
    • Команды: team-a, team-b, team-c.
    • Проекты: project-x, project-y.
    • Multi-tenant: SaaS с разными клиентами.

  2. ЧТО ВНУТРИ, ЧТО СНАРУЖИ
  НЕ ВСЕ объекты K8s находятся в namespace.

  ВНУТРИ namespace (namespaced):
    • Pod.
    • Deployment, StatefulSet, DaemonSet, ReplicaSet.
    • Service, Ingress.
    • ConfigMap, Secret.
    • Job, CronJob.
    • ServiceAccount.
    • PersistentVolumeClaim (PVC).
    • Role, RoleBinding.
    • NetworkPolicy.
    • HorizontalPodAutoscaler.

  СНАРУЖИ namespace (cluster-wide):
    • Node — физические машины. Не принадлежат namespace.
    • PersistentVolume (PV) — ресурс кластера.
    • StorageClass — шаблон для PV.
    • ClusterRole, ClusterRoleBinding — права на весь кластер.
    • Namespace — сам объект.
    • CustomResourceDefinition (CRD).
    • IngressClass.
    • PriorityClass.
    • MutatingWebhookConfiguration.
    • ValidatingWebhookConfiguration.

  ПОЧЕМУ ЭТО ВАЖНО:
    • kubectl get pods -n demo — Pod'ы конкретного namespace.
    • kubectl get nodes — ноды всегда cluster-wide.
    • kubectl get pv — PV всегда cluster-wide.
    • kubectl get pvc -n demo — PVC конкретного namespace.

  ЗАЧЕМ РАЗДЕЛЕНИЕ:
    • Node — физическая инфраструктура. Не может принадлежать
      одному namespace.
    • PV — общий ресурс. Много PVC из разных namespace могут
      ссылаться на один PV (обычно не могут, но концептуально).
    • ClusterRole — права на уровне кластера.

  3. ДЕФОЛТНЫЕ NAMESPACE
  В новом кластере уже есть 4 namespace.

  DEFAULT:
    Твой namespace по умолчанию. Если не указывать `-n`, все
    команды работают здесь.
    Плохо: засоряешь default. Хорошо: создать свой.

  KUBE-SYSTEM:
    Системные компоненты K8s: CoreDNS, kube-proxy, metrics-server.
    НЕ трогай. НЕ создавай тут свои ресурсы.

    Посмотреть что там:
      kubectl get pods -n kube-system
      # coredns-xxx
      # kube-proxy-xxx
      # metrics-server-xxx

  KUBE-PUBLIC:
    Публичные ресурсы, видные всем. Обычно пусто.

  KUBE-NODE-LEASE:
    Heartbeat нод. Служебный. Один Lease-объект на ноду.

  ПОСМОТРЕТЬ ВСЕ:
    kubectl get namespaces
    # NAME              STATUS   AGE
    # default           Active   10d
    # kube-node-lease   Active   10d
    # kube-public       Active   10d
    # kube-system       Active   10d

  4. СОЗДАНИЕ И УДАЛЕНИЕ

  ЧЕРЕЗ KUBECTL:
    kubectl create namespace demo

  ЧЕРЕЗ МАНИФЕСТ:
    apiVersion: v1
    kind: Namespace
    metadata:
      name: demo
      labels:
        env: dev
        team: backend
        cost-center: cc-1234

    kubectl apply -f namespace.yaml

  УДАЛЕНИЕ:
    kubectl delete namespace demo

  ЧТО ПРОИСХОДИТ ПРИ УДАЛЕНИИ:
    Удаляется ВСЁ содержимое namespace:
      • Все Pod'ы.
      • Все Service'ы.
      • Все ConfigMap, Secret.
      • Все Deployment'ы и т.д.
    Это опасно в проде. Двойная проверка перед delete.

  LABELS НА NAMESPACE:

    Полезно для RBAC и политик:
      labels:
        team: backend
        env: production
        cost-center: cc-1234

    По labels можно:
      • Фильтровать namespace.
      • Применять NetworkPolicy.
      • Считать cost allocation.

  ANNOTATIONS:

    Для документации и интеграций:
      annotations:
        owner: alice@example.com
        slack-channel: "#team-backend"
        description: "Backend services for prod"

  5. ПЕРЕКЛЮЧЕНИЕ МЕЖДУ NAMESPACE

  В КОМАНДАХ:
    kubectl get pods -n demo
    kubectl get pods --namespace=demo
    kubectl get pods -A              # все namespace

  ТЕКУЩИЙ NAMESPACE ПО УМОЛЧАНИЮ:
    kubectl config set-context --current --namespace=demo
    Теперь все команды без `-n` работают в demo.

  ПОСМОТРЕТЬ ТЕКУЩИЙ:
    kubectl config view --minify --output 'jsonpath={..namespace}' # demo

  УТИЛИТА KUBENS:
    kubens              # список namespace
    kubens demo         # переключиться
    Удобнее, чем kubectl config. Устанавливается отдельно.

  ПРАВИЛО:
    Всегда явно указывай `-n <namespace>`. Даже если
    переключил default. Меньше ошибок в командной строке.

  6. DNS МЕЖДУ NAMESPACE
  Service в K8s получает DNS-имя по формату:

    <service>.<namespace>.svc.cluster.local

  ПРИМЕРЫ:
    postgres.default.svc.cluster.local
    postgres.demo.svc.cluster.local
    api.production.svc.cluster.local

  КОРОТКИЕ ФОРМЫ:
    Из того же namespace:
      postgres

    Из другого namespace:
      postgres.demo
      postgres.demo.svc
      postgres.demo.svc.cluster.local

  ПРИМЕР В GO:
    // Тот же namespace.
    dsn := "postgres://user:pass@postgres:5432/app"

    // Другой namespace.
    dsn := "postgres://user:pass@postgres.production:5432/app"

  ВАЖНО:
    Короткая форма работает только внутри namespace. Из другого
    — указывай namespace.

  7. CROSS-NAMESPACE КОММУНИКАЦИЯ
  По умолчанию — все namespace видят друг друга по DNS.

  ПРИМЕР:
    Pod в namespace app-backend.
    Service postgres в namespace app-data.

    Из app-backend:
      ping postgres.app-data

    Работает. K8s не блокирует трафик между namespace по
    умолчанию.

  КАК ОГРАНИЧИТЬ:
    NetworkPolicy. По умолчанию — все со всеми.
    Чтобы ограничить — нужна политика default deny.

  КАК ОБЩАТЬСЯ:
    Из Pod'а в namespace A к Service в namespace B:
      1. Использовать полное DNS-имя:
         service-b.namespace-b.svc.cluster.local.
      2. Настроить NetworkPolicy, чтобы разрешить трафик.
      3. Service в B должен существовать.

  ПРИМЕР С GO:
    // Обращение из namespace backend к postgres в namespace data.
    dsn := "postgres://user:pass@postgres.data:5432/app"

    Или полностью:
    dsn := "postgres://user:pass@postgres.data.svc.cluster.local:5432/app"

  ВАЖНО:
    Cross-namespace — это не запрещено, но и не всегда нужно.
    Часто разные namespace используются для изоляции и не должны
    общаться напрямую.

  8. RESOURCEQUOTA — КВОТЫ
  ResourceQuota ограничивает сумму ресурсов в namespace.

  ПРИМЕР:
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
        services: "20"
        persistentvolumeclaims: "10"
        secrets: "30"
        configmaps: "30"

  ЧТО ЭТО ЗНАЧИТ:
    В namespace нельзя создать:
      • Pod'ов с суммой requests.cpu > 10.
      • Pod'ов с суммой requests.memory > 20Gi.
      • Больше 50 Pod'ов.
      • Больше 20 Service'ов.
      • И т.д.

  ЧТО ПРОИСХОДИТ ПРИ ПРЕВЫШЕНИИ:
    kubectl apply -f deployment.yaml
    # Error: pods "app" is forbidden: exceeded quota
    Создание падает. Нельзя превысить.

  ПРОВЕРИТЬ:
    kubectl get resourcequota -n demo
    # NAME         AGE   REQUEST                     LIMIT
    # demo-quota   5m    requests.cpu: 4/10          limits.cpu: 6/20
    #                    requests.memory: 8Gi/20Gi   limits.memory: 12Gi/40Gi
    #                    pods: 3/50                  ...

  ВАЖНО:
    Если ResourceQuota установлена — все Pod'ы в namespace
    должны указывать requests и limits. Иначе K8s не может
    посчитать сумму.
    Иначе ошибка: "must specify limits.cpu".

  ОБХОД ЧЕРЕЗ LIMITRANGE:
    LimitRange автоматически ставит defaults, если Pod их не
    указал. Тогда ResourceQuota работает и все довольны.

  9. LIMITRANGE — DEFAULTS
  LimitRange задаёт defaults и максимумы для отдельного Pod.

  ПРИМЕР:
    apiVersion: v1
    kind: LimitRange
    metadata:
      name: defaults
      namespace: demo
    spec:
      limits:
      - type: Container
        default:
          cpu: "500m"
          memory: "256Mi"
        defaultRequest:
          cpu: "100m"
          memory: "128Mi"
        max:
          cpu: "2"
          memory: "2Gi"
        min:
          cpu: "50m"
          memory: "64Mi"

  ЧТО ДЕЛАЕТ:
    • Pod без resources → получает default.
    • Pod с resources > max → не создаётся.
    • Pod с resources < min → не создаётся.

  ЗАЧЕМ ВМЕСТЕ С RESOURCEQUOTA:
    ResourceQuota требует, чтобы у Pod'ов были requests.
    LimitRange автоматически их ставит. Работают в паре.

  10. RBAC НА УРОВНЕ NAMESPACE
  Role и RoleBinding действуют в пределах namespace.

  РОЛЬ — набор правил:
    apiVersion: rbac.authorization.k8s.io/v1
    kind: Role
    metadata:
      name: pod-reader
      namespace: demo
    rules:
    - apiGroups: [""]
      resources: ["pods"]
      verbs: ["get", "list", "watch"]

  БИНДИНГ — привязка к пользователю/SA:
    apiVersion: rbac.authorization.k8s.io/v1
    kind: RoleBinding
    metadata:
      name: pod-reader-binding
      namespace: demo
    subjects:
    - kind: User
      name: alice@example.com
      apiGroup: rbac.authorization.k8s.io
    - kind: ServiceAccount
      name: app-sa
      namespace: demo
    roleRef:
      kind: Role
      name: pod-reader
      apiGroup: rbac.authorization.k8s.io

  ЧТО ЭТО ДАЁТ:
    • User alice@example.com может get/list/watch pods
      только в namespace demo.
    • В других namespace — не может.

  ПРОВЕРИТЬ:
    kubectl auth can-i list pods \
      --as=alice@example.com -n demo
    # yes

    kubectl auth can-i list pods \
      --as=alice@example.com -n default
    # no

  CLUSTERROLE — то же, но для всего кластера:
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: cluster-pod-reader
    rules:
    - apiGroups: [""]
      resources: ["pods"]
      verbs: ["get", "list"]

    ClusterRoleBinding — привязка к пользователю для всех
    namespace.

  ПРАВИЛО:
    Для команды на своём namespace — Role + RoleBinding.
    Для админа кластера — ClusterRole + ClusterRoleBinding.

  11. SERVICEACCOUNT PER NAMESPACE
  Каждый namespace имеет свой ServiceAccount `default`.
  ServiceAccount — это идентичность Pod'а.

  ЧТО ЭТО ЗНАЧИТ:
    Когда Pod делает запрос к API Server, он использует
    ServiceAccount своего namespace.
    Разные namespace = разные ServiceAccount = разные права.

  ПРИМЕР:
    В namespace demo есть SA `app-sa`.
    RoleBinding привязывает Role к `app-sa`.
    Pod в namespace demo с `serviceAccountName: app-sa`
    имеет права из Role.

  ПРОВЕРИТЬ:
    kubectl get sa -n demo
    # NAME      SECRETS   AGE
    # default   0         5m
    # app-sa    0         1m

  ИСПОЛЬЗОВАНИЕ В POD:
    spec:
      serviceAccountName: app-sa
      containers:
      ...

  ЗАЧЕМ ОТДЕЛЬНЫЙ SA:
    • Принцип минимальных привилегий.
    • Один SA — один сервис.
    • Разные права для разных сервисов.

  12. NETWORKPOLICY МЕЖДУ NAMESPACE
  По умолчанию — все Pod'ы в кластере могут общаться со всеми.

  NetworkPolicy ограничивает трафик. Работает на уровне
  namespace.

  DEFAULT DENY В NAMESPACE:
    apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: default-deny
      namespace: demo
    spec:
      podSelector: {}
      policyTypes:
      - Ingress
      - Egress

  ЧТО ДЕЛАЕТ:
    Запрещает ВСЁ входящее и исходящее в namespace demo.
    После этого — открывать точечно.

  РАЗРЕШИТЬ ТРАФИК ИЗ ДРУГОГО NAMESPACE:
    apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: allow-from-frontend
      namespace: demo
    spec:
      podSelector:
        matchLabels:
          app: backend
      policyTypes:
      - Ingress
      ingress:
      - from:
        - namespaceSelector:
            matchLabels:
              name: frontend
        ports:
        - protocol: TCP
          port: 8080

  ЧТО ЭТО ДАЁТ:
    Pod'ы в namespace demo с label app=backend принимают
    трафик только из namespace frontend на порт 8080.

  ВАЖНО:
    NetworkPolicy требует CNI с поддержкой: Calico, Cilium.
    Стандартный kubenet не поддерживает.

  ПРАВИЛО:
    Для multi-tenancy — default deny + точечные разрешения.
    Для простых случаев — оставить по умолчанию.

  13. MULTI-TENANCY ПАТТЕРНЫ
  ТРИ МОДЕЛИ ИЗОЛЯЦИИ:

    SHARED CLUSTER, SHARED NAMESPACE:
      Все в одном namespace. Мягкая изоляция через labels.
      Для маленьких команд.

    SHARED CLUSTER, SEPARATE NAMESPACE:
      Каждая команда — свой namespace. Обычная модель.
      Для средних компаний.

    SEPARATE CLUSTER:
      Каждая команда — свой кластер. Максимальная изоляция.
      Для enterprise, compliance.

  ПАТТЕРН ДЛЯ КОМАНД:
    namespace: team-a
    namespace: team-b
    namespace: team-c
    Каждый со своими RBAC, квотами, labels.

  ПАТТЕРН ДЛЯ ОКРУЖЕНИЙ:
    namespace: app-dev
    namespace: app-staging
    namespace: app-prod
    Один кластер — три окружения.

  ПАТТЕРН ДЛЯ ПРОЕКТОВ:
    namespace: project-x
    namespace: project-y
    Каждый проект изолирован.

  ПРАВИЛО:
    Namespace — не граница безопасности. Это граница
    организации. Для настоящей изоляции — отдельные кластеры.

  14. COST ALLOCATION ЧЕРЕЗ NAMESPACE
  Namespace — единица распределения затрат.

  ЧТО НУЖНО:
    • Labels на namespace (team, cost-center, env).
    • ResourceQuota (знаем лимиты).
    • Метрики реального потребления (kubectl top, Prometheus).
    • Инструменты (Kubecost, OpenCost).

  ПРИМЕР LABELS:
    apiVersion: v1
    kind: Namespace
    metadata:
      name: team-a-prod
      labels:
        team: team-a
        env: prod
        cost-center: cc-1234
        owner: alice@example.com

  ЧТО ЭТО ДАЁТ:
    • Можно посчитать затраты на команду.
    • Видно, кто жрёт больше всего ресурсов.
    • Легче принимать решения о бюджете.

    Kubecost автоматически считает затраты на namespace.
    Смотрит на requests/limits и реальное потребление.
    Показывает дашборд по namespace.

  15. УДАЛЕНИЕ И КАСКАД

  УДАЛЕНИЕ:
    kubectl delete namespace demo

  ЧТО УДАЛЯЕТСЯ:
    Всё, что было в namespace. Каскадно.

  ЧТО НЕ УДАЛЯЕТСЯ:
    • Node — физические машины.
    • PersistentVolume (PV) — остаётся, если не был
      привязан через StorageClass с reclaimPolicy: Delete.
    • ClusterRole, ClusterRoleBinding.

  ПРОДОЛЖИТЕЛЬНОСТЬ:
    Удаление namespace — асинхронное. Может занять минуты,
    если внутри много ресурсов.
    Пока удаляется — статус Terminating.

  ЗАВИСШИЙ NAMESPACE:
    Иногда namespace висит в Terminating. Причины:
      • Финалайзеры на объектах.
      • Cluster-scoped ресурсы, привязанные к namespace.

  ОСТОРОЖНО:
    Удаление namespace в проде = потеря всего. Всегда
    проверяй дважды.

  16. DEBUGGING ЗАВИСШЕГО NAMESPACE

  СИМПТОМ:
    kubectl get ns
    # NAME    STATUS        AGE
    # demo    Terminating   30m

    Namespace висит в Terminating.

  ПРИЧИНЫ:
    • Финалайзеры не убрались.
    • Cluster-scoped ресурс в namespace.
    • API Server проблемы.

  ДИАГНОСТИКА:
    # Смотрим, что в namespace.
    kubectl api-resources --verbs=list --namespaced -o name \
      | xargs -n 1 kubectl get --show-kind --ignore-not-found -n demo

    # Смотрим финалайзеры namespace.
    kubectl get ns demo -o yaml | grep finalizers

  РЕШЕНИЕ (ОСТОРОЖНО):
    # Убираем финалайзеры через API.
    kubectl get ns demo -o json | jq '.spec.finalizers = []' \
      | kubectl replace --raw "/api/v1/namespaces/demo/finalize" -f -

  ЕСЛИ НЕ ПОМОГАЕТ:
    Проверить, есть ли cluster-scoped ресурсы, привязанные
    к namespace. Их надо удалить вручную.

  17. BEST PRACTICES

  ИСПОЛЬЗУЙ NAMESPACE ДЛЯ:
    • Окружений: dev, staging, prod.
    • Команд: team-a, team-b.
    • Проектов: project-x, project-y.
    • Multi-tenant SaaS.

  ИМЕНОВАНИЕ:
    • Короткое, понятное.
    • Без спецсимволов (только латиница, цифры, дефис).
    • С префиксом по назначению: dev-team-a, prod-team-b.

  ПРАВИЛА:
    1. Не работай в default. Создай свой.
    2. Ставь labels (team, env, cost-center).
    3. Ставь ResourceQuota.
    4. Ставь LimitRange.
    5. Настраивай RBAC на namespace.
    6. Явно указывай -n в командах.
    7. Не удаляй namespace в проде без двойной проверки.
    8. Для multi-tenancy — NetworkPolicy default deny.

  ОКРУЖЕНИЯ В ОДНОМ КЛАСТЕРЕ:
    dev, staging, prod в одном кластере — ок, но:
      • Изоляция через namespace.
      • Отдельные RBAC.
      • Разные NetworkPolicy.
      • Prod обычно отдельный кластер (изоляция отказов).

  18. АНТИПАТТЕРНЫ

  18.1. ВСЁ В DEFAULT.
    Разработка, тесты, prod — всё в одном namespace.

  18.2. NAMESPACE БЕЗ RESOURCEQUOTA.
    Один сервис съедает все ресурсы кластера.

  18.3. NAMESPACE БЕЗ RBAC.
    Все видят всё. Разработчик может удалить prod.

  18.4. НЕТ LABELS.
    Не видно, чей namespace. Не работает cost allocation.

  18.5. ОДИН NAMESPACE НА КОМАНДУ И ПРОЕКТ.
    team-a и team-b в одном namespace — конфликты.

  18.6. PROD И DEV В ОДНОМ NAMESPACE.
    Один кривой kubectl apply — снёс prod.

  18.7. УДАЛЕНИЕ NAMESPACE В ПРОДЕ БЕЗ ПРОВЕРКИ.
    Каскадное удаление всего.

  18.8. ИМЕНА С ПРОБЕЛАМИ.
    K8s не даст создать.

  18.9. ИГНОРИРОВАНИЕ TERMINATING.
    Namespace висит в Terminating.

  18.10. НЕТ LIMITRANGE ПРИ RESOURCEQUOTA.
    Pod'ы без resources не создаются.

  18.11. HARDCODED NAMESPACE В МАНИФЕСТАХ.
    Один манифест для dev и prod. Namespace хардкод.

  18.12. CLUSTERROLE ДЛЯ РАЗРАБОТЧИКА.
    Разработчик видит весь кластер.

  18.13. NO NETWORKPOLICY ДЛЯ MULTI-TENANCY.
    Все namespace видят друг друга.

  19. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Namespace — логическая изоляция внутри кластера. Не физическая.
  2.  Дефолтные: default, kube-system, kube-public, kube-node-lease.
  3.  Внутри namespace: Pod, Service, ConfigMap, Secret,
      Deployment, Job, PVC, NetworkPolicy.
  4.  Снаружи: Node, PV, StorageClass, ClusterRole,
      ClusterRoleBinding, CRD.
  5.  Переключение: kubectl config set-context --current
      --namespace=X или kubens X.
  6.  DNS: <service>.<namespace>.svc.cluster.local.
  7.  Cross-namespace коммуникация работает по умолчанию.
      Ограничивается через NetworkPolicy.
  8.  ResourceQuota — квоты на namespace (CPU, memory, pods).
  9.  LimitRange — defaults и max для Pod'ов.
  10. RBAC: Role + RoleBinding для namespace.
      ClusterRole + ClusterRoleBinding для кластера.
  11. ServiceAccount — идентичность Pod'а, своя в каждом namespace.
  12. Multi-tenancy: shared cluster + separate namespace — обычная модель.
  13. Cost allocation через labels на namespace + Kubecost.
  14. Удаление namespace — каскадное.
  15. Debugging зависшего namespace — финалайзеры.
  16. Best practices: не работай в default, ставь labels,
      ResourceQuota, LimitRange, RBAC, NetworkPolicy.
  17. Антипаттерны: всё в default, нет квот, нет RBAC,
      prod и dev в одном namespace, ClusterRole для
      разработчика, нет NetworkPolicy.
*/
