package main

/*
  УРОК 1.1: CONTROL PLANE VS WORKER NODES
  Kubernetes — это не «Docker, но сложнее». Это распределённая
  система управления контейнерами на кластере машин. Чтобы
  понимать K8s, нужно понимать, из чего он состоит: какие
  компоненты работают, что каждый делает, как они общаются.
  Без этой базы всё остальное — Pod, Deployment, Service —
  превращается в магические YAML-файлы, которые «почему-то
  работают» или «почему-то не работают». А на собеседовании
  первый вопрос почти всегда: «Расскажи, как устроен K8s».

  СОДЕРЖАНИЕ:
    1.  Что такое кластер
    2.  Control plane — мозг кластера
    3.  API Server — единственная точка входа
    4.  etcd — база состояния кластера
    5.  Scheduler — куда поставить Pod
    6.  Controller Manager — обеспечивает желаемое состояние
    7.  Worker node — где работают приложения
    8.  kubelet — агент на каждой ноде
    9.  kube-proxy — сеть и балансировка
    10. Container runtime — containerd
    11. Что происходит при kubectl apply — полный путь
    12. Что происходит, когда Pod падает
    13. Отказоустойчивость control plane
    14. Связь с Go
    15. Антипаттерны
    16. Финальные выводы

  1. ЧТО ТАКОЕ КЛАСТЕР
  Кластер K8s — это набор машин (нод), объединённых в одну
  систему. Каждая нода — это физический сервер или VM.

  В кластере есть два типа нод:
    CONTROL PLANE — управляет кластером. Не запускает твои
                    приложения.
    WORKER NODES  — запускают твои приложения (Pod'ы).

  МИНИМАЛЬНЫЙ КЛАСТЕР:
    ┌──────────────────────┐
    │  Control plane (1)   │
    └──────────┬───────────┘
               │
    ┌──────────┼──────────┐
    ▼          ▼          ▼
  ┌─────┐   ┌─────┐   ┌─────┐
  │Node1│   │Node2│   │Node3│
  └─────┘   └─────┘   └─────┘
   worker    worker    worker

  В проде control plane обычно тоже состоит из 3 или 5 нод —
  для отказоустойчивости. Worker nodes — от 2 до сотен, в
  зависимости от нагрузки.

  В DEV / MINIKUBE:
    Одна нода одновременно control plane и worker. Это удобно
    для локальной разработки, но не для прода.

  2. CONTROL PLANE — МОЗГ КЛАСТЕРА
  Control plane — это набор процессов, которые принимают
  решения о кластере: где запускать Pod'ы, сколько реплик,
  какие сервисы доступны.

  ЧЕТЫРЕ ОСНОВНЫХ КОМПОНЕНТА:
    ┌────────────────────────────────────────────┐
    │              CONTROL PLANE                 │
    │                                            │
    │  ┌──────────────┐  ┌─────────────────────┐ │
    │  │  API Server  │◄─┤     kubectl         │ │
    │  └───────┬──────┘  └─────────────────────┘ │
    │          │                                 │
    │  ┌───────▼──────┐  ┌─────────────────────┐ │
    │  │     etcd     │  │     Scheduler       │ │
    │  └──────────────┘  └─────────────────────┘ │
    │                                            │
    │  ┌────────────────────────────────────────┐│
    │  │        Controller Manager              ││
    │  └────────────────────────────────────────┘│
    └────────────────────────────────────────────┘

  API SERVER — единственная точка входа. Все команды (kubectl,
  контроллеры, kubelet) идут через него.

  ETCD — база данных. Хранит всё состояние кластера.

  SCHEDULER — решает, на какую ноду поставить Pod.

  CONTROLLER MANAGER — следит за тем, чтобы реальное состояние
  совпадало с желаемым.

  3. API SERVER — ЕДИНСТВЕННАЯ ТОЧКА ВХОДА
  API Server — центральный компонент. Все взаимодействия с
  кластером идут через него. Никто не общается с etcd напрямую
  (кроме API Server).

  ЧТО ОН ДЕЛАЕТ:
    • Принимает REST-запросы (HTTP/HTTPS).
    • Аутентифицирует и авторизует.
    • Валидирует объекты (схема, admission controllers).
    • Сохраняет в etcd.
    • Отдаёт watch-стримы другим компонентам.
    • Работает как API для kubectl, контроллеров, kubelet.

  ЧТО ЕМУ ОТПРАВЛЯЮТ:
    kubectl apply -f pod.yaml
      → API Server: «создай Pod»
      → валидация
      → сохранение в etcd
      → ответ клиенту

  ЧТО ЕМУ ОТПРАВЛЯЮТ КОМПОНЕНТЫ:

    Scheduler:
      «Есть нераспределённый Pod. Я решил, что он должен быть
      на Node2. Обнови запись».

    kubelet:
      «Я — Node1. Мой Pod упал. Обнови статус».

    Controller Manager:
      «Deployment хочет 3 реплики, а есть 2. Создай ещё одну».

  ВСЁ ИДЁТ ЧЕРЕЗ API SERVER. Это ключевая идея. Никакого
  прямого общения между компонентами. Все через API.

  ПОЧЕМУ ЭТО ХОРОШО:
    • Единая точка входа — проще контролировать.
    • Легко добавить аутентификацию, авторизацию, аудит.
    • Компоненты не знают друг о друге — только об API.
    • Можно заменить любой компонент, кроме API Server.

  4. ETCD — БАЗА СОСТОЯНИЯ КЛАСТЕРА
  etcd — распределённое key-value хранилище. Все объекты K8s
  (Pod, Deployment, Service, ConfigMap, Secret) хранятся там.

  ЧТО ХРАНИТСЯ:
    • Состояние всех объектов.
    • Метаданные (labels, annotations).
    • Секреты (в etcd, часто с шифрованием на уровне диска).
    • События (events).
    • Служебные данные (leases, IP-адреса).

  ХАРАКТЕРИСТИКИ:
    • Распределённый. Обычно 3 или 5 нод.
    • Консистентный. Использует протокол Raft.
    • Только API Server общается с ним напрямую.
    • Все данные в одном месте — точка отказа (поэтому 3-5 нод).

  ВАЖНО ПРО РАЗМЕР:
    etcd хранит всё состояние в памяти + на диске. Обычно
    рекомендуют не раздувать его больше 8 ГБ. Для больших
    кластеров — мониторинг, компакция.

  ЧТО ЭТО ЗНАЧИТ НА ПРАКТИКЕ:
    • Если etcd потерян — весь кластер потерян.
    • Бэкапы etcd — критичны (etcdctl snapshot save).
    • Не кладите в ConfigMap огромные файлы — это etcd.
    • Secrets в etcd — base64, не шифрование. Для настоящей
      защиты — encryption at rest + external secrets manager.

  ПОЧЕМУ НЕЛЬЗЯ ОБЩАТЬСЯ С ETCD НАПРЯМУЮ:
    • Нет авторизации на уровне K8s.
    • Нет admission-валидации.
    • Нет версионирования API.
    • Компоненты не знают про state machine K8s.

  API Server — это фасад над etcd с правилами K8s.

  5. SCHEDULER — КУДА ПОСТАВИТЬ POD
  Scheduler решает, на какую ноду поставить Pod.

  КОГДА ОН РАБОТАЕТ:
    1. API Server создал Pod со статусом Pending.
    2. У Pod'а нет привязки к ноде (nodeName пустой).
    3. Scheduler видит такой Pod.
    4. Выбирает подходящую ноду.
    5. Обновляет Pod (проставляет nodeName).
    6. kubelet на этой ноде видит Pod → запускает.

  КАК ВЫБИРАЕТ:
    ДВА ЭТАПА:

    FILTERING (отбор):
      • Какие ноды вообще подходят?
      • Учитывает: CPU, память, requests, taints, nodeSelector,
        affinity.
      • Ноды, не прошедшие фильтр, отбрасываются.

    SCORING (ранжирование):
      • Среди прошедших — какая лучше?
      • Учитывает: load balancing, распространение Pod'ов,
        affinity.
      • Выбирается нода с наибольшим score.

  ЧТО ВЛИЯЕТ НА ВЫБОР:
    • resource requests (CPU, memory) — хватит ли на ноде.
    • nodeSelector — конкретные labels ноды.
    • nodeAffinity / podAffinity — правила размещения.
    • taints и tolerations — какие Pod'ы могут жить на ноде.
    • resource limits.

  ЧТО НЕ ДЕЛАЕТ SCHEDULER:
    • Не запускает контейнеры. Это делает kubelet.
    • Не следит за Pod'ом после запуска. Это kubelet.
    • Не решает про scaling. Это контроллеры.

  ЕСЛИ SCHEDULER НЕ МОЖЕТ РАЗМЕСТИТЬ POD:
    Pod остаётся в статусе Pending. В `kubectl describe pod`
    видно причину: «0/3 nodes available: 3 Insufficient cpu».

    Типовые причины:
      • Не хватает ресурсов (requests больше, чем есть на нодах).
      • nodeSelector не совпадает ни с одной нодой.
      • taints без tolerations.

  6. CONTROLLER MANAGER — ОБЕСПЕЧИВАЕТ ЖЕЛАЕМОЕ СОСТОЯНИЕ
  Controller Manager — это набор контроллеров. Каждый следит за
  своим типом объекта и приводит реальное состояние к желаемому.

  КЛЮЧЕВАЯ ИДЕЯ K8S — ДЕКЛАРАТИВНОСТЬ:
    Ты пишешь «хочу 3 реплики Pod'а my-app».
    Контроллер следит: «сейчас 3? Ок. Упал один? Создаю новый».

    Ты пишешь «хочу Service с таким selector».
    Контроллер следит: «есть Pod'ы с такими labels? Добавляю
    их в endpoints».

  ВСТРОЕННЫЕ КОНТРОЛЛЕРЫ:

    DEPLOYMENT CONTROLLER:
      • Следит за Deployment.
      • Создаёт ReplicaSet.
      • Обновляет версии (rolling update).

    REPLICASET CONTROLLER:
      • Следит за количеством Pod'ов.
      • Если Pod упал — создаёт новый.
      • Если Pod'ов больше — убивает лишний.

    SERVICE CONTROLLER:
      • Следит за Service.
      • Создаёт Endpoints из Pod'ов по selector.

    NODE CONTROLLER:
      • Следит за нодами.
      • Если нода не отвечает — помечает NotReady.
      • Потом выселяет Pod'ы (eviction).

    JOB CONTROLLER:
      • Следит за Job.
      • Запускает Pod'ы, повторяет при падении.

    NAMESPACE CONTROLLER:
      • Удаляет всё содержимое при удалении namespace.

  ПРИМЕР РАБОТЫ:
    Ты создал Deployment с replicas: 3.
    1. Deployment controller создаёт ReplicaSet.
    2. ReplicaSet controller создаёт 3 Pod'а.
    3. Scheduler раскидывает Pod'ы по нодам.
    4. kubelet запускает контейнеры.
    5. Один Pod упал (OOMKilled).
    6. ReplicaSet controller видит: «2 из 3».
    7. Создаёт новый Pod.
    8. Scheduler распределяет, kubelet запускает.

  ВСЁ ЭТО АВТОМАТИЧЕСКИ. Ты не делаешь ничего — только следишь
  за декларацией.

  7. WORKER NODE — ГДЕ РАБОТАЮТ ПРИЛОЖЕНИЯ
  Worker node — машина, на которой реально крутятся твои
  контейнеры.

  ТРИ КОМПОНЕНТА НА КАЖДОЙ НОДЕ:
    ┌─────────────────────────────────────┐
    │           WORKER NODE               │
    │                                     │
    │  ┌──────────┐  ┌──────────────────┐ │
    │  │ kubelet  │  │  kube-proxy      │ │
    │  └──────────┘  └──────────────────┘ │
    │                                     │
    │ ┌──────────────────────────────────┐│
    │ │   containerd (container runtime) ││
    │ └──────────────────────────────────┘│
    │                                     │
    │  ┌────────┐ ┌────────┐ ┌────────┐   │
    │  │Pod app │ │Pod app │ │Pod app │   │
    │  └────────┘ └────────┘ └────────┘   │
    └─────────────────────────────────────┘

  8. KUBELET — АГЕНТ НА КАЖДОЙ НОДЕ
  kubelet — главный процесс на каждой worker node. Это агент,
  который связывает ноду с control plane.

  ЧТО ДЕЛАЕТ:
    • Следит за Pod'ами, назначенными на эту ноду.
    • Запускает контейнеры через container runtime.
    • Мониторит состояние контейнеров.
    • Выполняет probes (liveness, readiness).
    • Отправляет статусы в API Server.
    • Реагирует на изменения (Pod обновили — kubelet видит).

  КАК ПОЛУЧАЕТ ИНФОРМАЦИЮ:
    • Через watch к API Server: «дай мне все Pod'ы с
      spec.nodeName == меня».
    • API Server стримит обновления.
    • kubelet реагирует сразу.

  ЧТО НЕ ДЕЛАЕТ KUBELET:
    • Не решает, куда поставить Pod (это Scheduler).
    • Не управляет Service (это kube-proxy).
    • Не общается с etcd.

  ЕСЛИ KUBELET УПАЛ:
    • Нода помечается NotReady через N секунд.
    • Pod'ы на этой ноде помечаются.
    • Через 5 минут Node Controller выселяет Pod'ы.
    • Deployment создаёт новые на других нодах.

  ВАЖНО ПРО PROBES:
    kubelet сам выполняет probes. Каждые N секунд делает
    HTTP-запрос или exec в контейнер.

    Если livenessProbe падает N раз — kubelet перезапускает
    контейнер.
    Если readinessProbe падает — kubelet помечает Pod
    NotReady. Service исключает его из endpoints.

  9. KUBE-PROXY — СЕТЬ И БАЛАНСИРОВКА
  kube-proxy — сетевой агент на каждой ноде. Отвечает за
  реализацию Service.

  ЧТО ДЕЛАЕТ:
    • Следит за Service и Endpoints в API Server.
    • Настраивает правила iptables (или IPVS).
    • Маршрутизирует трафик с ClusterIP на конкретные Pod'ы.
    • Делает балансировку между Pod'ами Service'а.

  ПРИМЕР:
    Есть Service my-app с ClusterIP 10.96.0.5.
    Есть 3 Pod'а с IP: 10.244.1.5, 10.244.2.7, 10.244.3.9.

    Когда Pod A обращается к 10.96.0.5:8080:
      • Трафик перехватывается kube-proxy на ноде.
      • Правила iptables перенаправляют его на один из Pod'ов.
      • Выбор — round-robin (по умолчанию).

    Это НЕ прокси в классическом смысле. kube-proxy только
    настраивает правила ядра, сам трафик через него не идёт.

  ДВА РЕЖИМА:
    iptables — дефолт. Правила ядра. Быстрый, но не
      масштабируется на тысячи сервисов.

    IPVS — балансировщик на уровне ядра Linux. Быстрее,
      лучше масштабируется. Используется в больших кластерах.

  ЧТО НЕ ДЕЛАЕТ KUBE-PROXY:
    • Не управляет сетью между Pod'ами (это CNI).
    • Не даёт Pod'ам IP-адреса (это CNI).
    • Не реализует network policies (это CNI).

  10. CONTAINER RUNTIME — CONTAINERD
  Container runtime — то, что реально запускает контейнеры.

  ИСТОРИЯ:
    Раньше K8s использовал Docker (dockershim).
    С 1.24 dockershim удалён.
    Сейчас — containerd или CRI-O.

  CONTAINERD — дефолт:
    • Лёгкий, быстрый.
    • Управляет контейнерами через CRI (Container Runtime
      Interface).
    • kubelet общается с containerd через CRI.
    • containerd вызывает runc для создания контейнера.

  ЦЕПОЧКА:
    kubelet → CRI → containerd → runc → контейнер

  CRI (Container Runtime Interface):
    Стандартный API между kubelet и runtime. Любой runtime,
    реализующий CRI, может работать с K8s. Это позволяет
    менять containerd на CRI-O или другой.

  ЧТО ЭТО ЗНАЧИТ НА ПРАКТИКЕ:
    • Образы — это OCI-образы (см. урок про Docker).
    • Docker CLI на ноде может не быть.
    • Отладка — через crictl или containerd CLI.
    • Dockerfile, который ты писал, работает без изменений.

  11. ЧТО ПРОИСХОДИТ ПРИ KUBECTL APPLY — ПОЛНЫЙ ПУТЬ
  Ты написал:
    kubectl apply -f deployment.yaml

  ПОШАГОВО:

    ШАГ 1: kubectl → API Server
      kubectl отправляет HTTP-запрос на API Server.
      Аутентификация (сертификат, токен).
      Авторизация (RBAC: может ли этот пользователь создавать
      Deployment в этом namespace).

    ШАГ 2: API Server валидирует
      Проверяет схему YAML.
      Вызывает admission controllers (например, ResourceQuota,
      PodSecurityPolicy).
      Если всё ок — сохраняет в etcd.

    ШАГ 3: Deployment Controller видит новый Deployment
      Controller Manager следит за Deployment через watch.
      Видит новый — создаёт ReplicaSet.

    ШАГ 4: ReplicaSet Controller видит новый ReplicaSet
      Создаёт N Pod'ов с нужными labels.
      Pod'ы попадают в etcd со статусом Pending.

    ШАГ 5: Scheduler видит Pending Pod'ы
      Для каждого Pod'а:
        • Фильтрует ноды.
        • Ранжирует.
        • Выбирает одну.
        • Обновляет Pod (spec.nodeName = node-X).

    ШАГ 6: kubelet на выбранной ноде видит свой Pod
      Через watch видит, что на него назначили Pod.
      Вызывает containerd: «запусти контейнер».
      containerd → runc → контейнер запущен.
      kubelet обновляет статус Pod'а в API Server: Running.

    ШАГ 7: kube-proxy настраивает сеть
      Если Pod попал в Service (по labels) — kube-proxy
      добавляет его в правила iptables.

    ШАГ 8: Service Controller создаёт Endpoints
      Видит, что есть Pod с нужными labels.
      Создаёт Endpoints объект со списком IP Pod'ов.

  ВСЁ ЭТО ЗА СЕКУНДЫ. Автоматически. Ты написал один YAML.

  ВРЕМЕННАЯ ШКАЛА:
    t=0ms   kubectl apply
    t=100ms API Server сохранил в etcd
    t=200ms Deployment Controller создал ReplicaSet
    t=300ms ReplicaSet Controller создал Pod
    t=400ms Scheduler назначил ноду
    t=500ms kubelet начал запуск
    t=2s    контейнер запущен
    t=3s    Pod в статусе Running

  12. ЧТО ПРОИСХОДИТ, КОГДА POD ПАДАЕТ
  Демонстрация декларативности K8s.

  СЦЕНАРИЙ:
    Deployment с replicas: 3.
    Все 3 Pod'а на Node1, Node2, Node3.

    t=0s   Pod на Node1 упал (OOMKilled).
    t=1s   kubelet на Node1 сообщает в API Server: «Pod упал».
    t=2s   ReplicaSet Controller видит: «2 живых из 3».
    t=3s   Создаёт новый Pod.
    t=4s   Scheduler назначает ноду.
    t=5s   kubelet запускает.
    t=10s  Pod Running.

  Всё автоматически. Никто не пишет `docker run` руками.

  ДРУГОЙ СЦЕНАРИЙ — НОДА УПАЛА:
    t=0s    Node1 полностью недоступна.
    t=40s   Node Controller помечает Node1 NotReady.
    t=5m    Node Controller выселяет Pod'ы.
    t=5m    Deployment видит: «не хватает реплик».
    t=5m    Создаёт Pod'ы на других нодах.
    t=5m30s Pod'ы Running на Node2, Node3.

  Downtime для клиентов: пока Service видел живые Pod'ы —
  он их использовал. Как только новые поднялись — трафик пошёл
  на них. Пользователь ничего не заметил.

  13. ОТКАЗОУСТОЙЧИВОСТЬ CONTROL PLANE
  Control plane — критичная часть. Если он упал — кластером
  нельзя управлять.

  ЧТО ПРОИСХОДИТ, ЕСЛИ API SERVER УПАЛ:
    • kubectl не работает.
    • Новые Pod'ы не создаются.
    • Существующие Pod'ы продолжают работать (kubelet их не убивает).
    • Service'ы работают (kube-proxy уже настроил правила).

  То есть приложение продолжает работать, но кластер нельзя
  изменить. Это не downtime, но и не полная функциональность.

  ЧТО ПРОИСХОДИТ, ЕСЛИ ETCD УПАЛ:
    Всё плохо. etcd — источник истины. Без него API Server
    не работает. Кластер мёртв.

  ПОЭТОМУ — 3 ИЛИ 5 НОД CONTROL PLANE:
    3 ноды: терпимо 1 отказ.
    5 нод: терпимо 2 отказа.

    Работает через Raft: большинство (quorum) должно быть
    доступно. Для 3 нод — минимум 2. Для 5 — минимум 3.

  ЕСЛИ ЕСТЬ 3 НОДЫ И 2 УПАЛИ:
    Raft теряет quorum. etcd становится read-only.
    Нельзя создавать/обновлять объекты.
    Существующие Pod'ы работают.

  ПРАКТИКА:
    В облаке (EKS, GKE, AKS) control plane — managed. Ты его
    не видишь. AWS/Google/Azure следят за отказоустойчивостью.

    В self-hosted (kubeadm) — ты сам поднимаешь 3 или 5 нод
    control plane.

  14. СВЯЗЬ С GO
  K8s написан на Go. Многие компоненты ты можешь читать
  и понимать.

  ЧТО НАПИСАНО НА GO:

    • API Server.
    • kubelet.
    • Scheduler.
    • Controller Manager.
    • kube-proxy.
    • kubectl.
    • Helm.
    • Prometheus (мониторинг K8s).
    • Traefik (ingress controller).

  КАК ЭТО ПОЛЕЗНО GO-РАЗРАБОТЧИКУ:

    CLIENT-GO — библиотека для работы с API K8s:
      import "k8s.io/client-go/kubernetes"

      config, _ := rest.InClusterConfig()
      clientset, _ := kubernetes.NewForConfig(config)

      pods, _ := clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{})

    Используется для:
      • Написания операторов (custom controllers).
      • Автоматизации деплоя.
      • Сбора информации о кластере из сервиса.

  IN-CLUSTER CONFIG:
    Когда Pod работает внутри кластера, client-go
    автоматически находит API Server через service account:

      • Переменные окружения KUBERNETES_SERVICE_HOST/PORT.
      • Токен в /var/run/secrets/kubernetes.io/serviceaccount/.

    Ничего не нужно настраивать — работает из коробки.

  ОПЕРАТОРЫ — что это:
    Оператор = custom controller на Go. Следит за custom
    resource (CRD) и делает что-то в кластере.

    Примеры:
      • Postgres Operator — управляет кластерами Postgres.
      • Prometheus Operator — управляет Prometheus.
      • Istio Operator — управляет service mesh.

    Пишутся через client-go + controller-runtime.

    Не обязательно уметь писать операторы. Но знать, что
    K8s — это Go-проект, и что client-go существует —
    полезно. На собесе может всплыть.

  15. АНТИПАТТЕРНЫ
  15.1. ОБЩЕНИЕ КОМПОНЕНТОВ НАПРЯМУЮ.
    Все должны ходить через API Server. Это архитектура K8s.
  15.2. ПИСАТЬ В ETCD НАПРЯМУЮ.
    Никогда. Только через API Server.
  15.3. ХРАНИТЬ СОСТОЯНИЕ В POD'Е.
    Pod эфемерный. Состояние — в volume или внешней БД.
  15.4. ПИСАТЬ ЛОГИ В ФАЙЛ ВНУТРИ POD'А.
    Логи — в stdout/stderr. K8s сам их собирает.
  15.5. ДУМАТЬ, ЧТО KUBELET УБИВАЕТ ПОДЫ ПРИ ПАДЕНИИ API SERVER.
    Нет. Kubelet продолжает работать, Pod'ы живут.
  15.6. ИГНОРИРОВАТЬ RESOURCE REQUESTS.
    Без requests Scheduler не знает, куда ставить Pod.
    BestEffort QoS — первый кандидат на eviction.
  15.7. ОДНА НОДА CONTROL PLANE В ПРОДЕ.
    Упала — кластер мёртв. Нужно 3 или 5.
  15.8. БЭКАПЫ ETCD НЕ ДЕЛАЮТСЯ.
    Потеря etcd = потеря кластера.
  15.9. ДУМАТЬ, ЧТО CONTAINERD = DOCKER.
    Docker — это CLI + runtime + оркестрация.
    containerd — только runtime. У Docker CLI на ноде может
    не быть.
  15.10. ЗАПУСКАТЬ DOCKER CLI НА НОДЕ.
    Не нужно. Отладка — через crictl или kubectl.

  16. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  K8s — кластер из нод. Два типа: control plane и worker.
  2.  Control plane — мозг. API Server, etcd, Scheduler,
      Controller Manager.
  3.  Worker node — где работают Pod'ы. kubelet, kube-proxy, containerd.
  4.  API Server — единственная точка входа. Всё идёт через
      него. Никаких прямых коммуникаций.
  5.  etcd — база состояния. 3 или 5 нод для отказоустойчивости.
  6.  Scheduler — решает, на какую ноду поставить Pod. Два
      этапа: filtering и scoring.
  7.  Controller Manager — набор контроллеров. Следят за
      объектами и приводят реальное состояние к желаемому.
  8.  kubelet — агент на каждой ноде. Запускает контейнеры,
      выполняет probes, отправляет статусы.
  9.  kube-proxy — сеть и балансировка. iptables или IPVS.
  10. Container runtime — containerd или CRI-O. Общается с
      kubelet через CRI.
  11. При `kubectl apply` — цепочка: API Server → etcd →
      Controller → Scheduler → kubelet → containerd → Pod.
  12. Всё декларативно. Ты пишешь желаемое состояние —
      контроллеры добиваются его.
  13. Control plane в проде — 3 или 5 нод. Отказ 1-2 нод
      не выводит кластер из строя.
  14. K8s написан на Go. client-go — библиотека для работы
      с API из Go. Операторы — custom controllers.
  15. Антипаттерны: прямое общение с etcd, состояние в Pod,
      одна нода control plane в проде, нет бэкапов etcd.
*/
