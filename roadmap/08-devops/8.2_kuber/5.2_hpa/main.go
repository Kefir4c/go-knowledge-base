package main

/*
  УРОК 5.2: HPA (HORIZONTAL POD AUTOSCALER)
  HPA — механизм автоматического масштабирования количества реплик Pod'ов на основе метрик.
  Если нагрузка растёт — HPA добавляет Pod'ы. Если падает — убирает.
  Без HPA ты либо держишь избыточные реплики (дорого), либо получаешь перегрузку при пиках (плохо для пользователей).
  HPA — не «поставил и забыл». Это сложная система, которая требует:
    • metrics-server (или другой источник метрик).
    • requests.cpu в Pod template (обязательно!).
    • Stateless-приложение (иначе сессии теряются).
    • Понимание, когда скейлить вверх, а когда — вниз.

  СОДЕРЖАНИЕ:
    1.  Зачем нужен HPA
    2.  Три вида автомасштабирования: HPA, VPA, Cluster Autoscaler
    3.  metrics-server — обязательное условие
    4.  Как работает HPA: control loop и формула
    5.  Типы метрик: Resource, Custom, External
    6.  API-версии: autoscaling/v1, v2
    7.  Манифест HPA v2 — полный разбор
    8.  targetCPUUtilizationPercentage и target
    9.  minReplicas, maxReplicas — границы
    10. Почему requests.cpu обязателен
    11. Behavior: scaleUp и scaleDown политики
    12. Stabilization window — защита от флаппинга
    13. HPA и Deployment: как они связаны
    14. HPA и Service: endpoints обновляются
    15. Кастомные метрики через Prometheus Adapter
    16. Внешние метрики (External Metrics) и KEDA
    17. Диагностика HPA: почему не скейлит
    18. HPA vs VPA vs Cluster Autoscaler
    19. Связь с Go
    20. Антипаттерны
    21. Финальные выводы

  1. ЗАЧЕМ НУЖЕН HPA
  ПРОБЛЕМА: нагрузка неравномерна. Ночью — 10 запросов в секунду, днём — 1000, в чёрную пятницу — 10000.
  Если держать фиксированное число реплик:
    • По пиковой нагрузке — 90% времени переплата за ресурсы.
    • По средней — падение при пиках.
    • По ночной — падение днём.

  РЕШЕНИЕ: HPA. Он автоматически меняет replicas в Deployment/StatefulSet/ReplicaSet на основе метрик.
  Нагрузка растёт → HPA добавляет Pod'ы. Нагрузка падает → HPA убирает Pod'ы.

  ЧТО ЭТО ДАЁТ:
    • Экономия: платишь только за нужные ресурсы.
    • Устойчивость: пики обрабатываются.
    • Автоматизация: не надо руками менять replicas.

  ВАЖНО: HPA работает только если приложение stateless. Если сессии в памяти Pod'а — при скейле вниз пользователи теряют сессии.
  Решение: сессии в Redis, sticky sessions на Ingress (костыль), или stateless JWT.

  2. ТРИ ВИДА АВТОМАСШТАБИРОВАНИЯ: HPA, VPA, CLUSTER AUTOSCALER
  HPA (Horizontal Pod Autoscaler):
    • Меняет КОЛИЧЕСТВО Pod'ов.
    • Горизонтальное масштабирование.
    • Реагирует на метрики (CPU, memory, custom).
    • Работает на уровне Deployment/StatefulSet.

  VPA (Vertical Pod Autoscaler):
    • Меняет requests/limits Pod'ов.
    • Вертикальное масштабирование.
    • Не увеличивает количество Pod'ов.
    • Требует перезапуска Pod'ов (обычно).
    • Не встроен в K8s, ставится отдельно.

  CLUSTER AUTOSCALER:
    • Добавляет/убирает НОДЫ в кластере.
    • Работает на уровне облачного провайдера (AWS, GCP, Azure).
    • Если Pod'ы не могут запланироваться (Pending) — добавляет ноду.
    • Если ноды простаивают — убирает.
    • Не встроен в K8s, ставится отдельно.

  КОМБИНАЦИЯ В ПРОДЕ: HPA + Cluster Autoscaler — классика. HPA создаёт Pod'ы → Cluster Autoscaler добавляет ноды, если Pod'ы не влезают.
  НЕ ИСПОЛЬЗУЙ HPA И VPA ВМЕСТЕ НА ОДНИХ МЕТРИКАХ. Они будут конфликтовать: VPA меняет requests, HPA смотрит на utilization относительно requests.
  Можно HPA на CPU + VPA на memory — но осторожно.

  3. METRICS-SERVER — ОБЯЗАТЕЛЬНОЕ УСЛОВИЕ
  HPA не работает без источника метрик. По умолчанию K8s НЕ умеет собирать метрики сам. Нужен metrics-server.

  METRICS-SERVER — что это:
    • Лёгкий агрегатор метрик.
    • Собирает CPU и memory с kubelet каждой ноды.
    • Отдаёт через Metrics API.
    • Не хранит историю (только текущие значения).
    • Не заменяет Prometheus.

  ПРОВЕРИТЬ, УСТАНОВЛЕН ЛИ:
    kubectl get deployment metrics-server -n kube-system
    kubectl top nodes
    kubectl top pods -n demo

  ЕСЛИ НЕ УСТАНОВЛЕН:
    kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

  ЧТО ДАЁТ METRICS-SERVER:
    • kubectl top pods/nodes.
    • HPA на CPU и memory.
    • VPA (если установлен).

  ЧЕГО НЕ ДАЁТ:
    • Историю метрик (только текущие).
    • Кастомные метрики (нужен Prometheus Adapter).
    • Алерты (нужен Prometheus + Alertmanager).

  В ОБЛАКЕ: EKS, GKE, AKS — metrics-server обычно предустановлен. В minikube — включается аддоном: minikube addons enable metrics-server.
  ВАЖНО: Без metrics-server HPA будет показывать <unknown> в TARGETS и не сможет скейлить.

  4. КАК РАБОТАЕТ HPA: CONTROL LOOP И ФОРМУЛА
  HPA — это контроллер. Он работает в цикле.

  ЦИКЛ HPA:
    1. Каждые 15 секунд (по умолчанию) HPA опрашивает метрики.
    2. Считает desired replicas по формуле.
    3. Сравнивает с текущим replicas.
    4. Если отличается — обновляет replicas в Deployment.

  ФОРМУЛА (для CPU utilization):
    desiredReplicas = ceil[currentReplicas * (currentMetricValue / desiredMetricValue)]

  ПРИМЕР: Текущих реплик: 3. Текущий CPU utilization: 80%. Целевой CPU utilization: 50%.
    desiredReplicas = ceil[3 * (80 / 50)] = ceil[4.8] = 5. HPA увеличит replicas с 3 до 5.

  ДРУГОЙ ПРИМЕР: Текущих реплик: 5. Текущий CPU: 20%. Целевой: 50%.
    desiredReplicas = ceil[5 * (20 / 50)] = ceil[2] = 2. HPA уменьшит replicas с 5 до 2.

  ЧТО ЗНАЧИТ utilization: utilization = (фактическое потребление CPU) / (requests.cpu) * 100%.
  Если requests.cpu = 100m, а Pod ест 80m: utilization = 80%.

  ВАЖНО: HPA смотрит на СРЕДНЕЕ по всем Pod'ам. Если у одного Pod'а 90%, а у другого 10% — среднее 50%. HPA не будет скейлить.
  ЧТО ЕЩЁ УЧИТЫВАЕТСЯ:
    • Не все Pod'ы готовы (readiness).
    • Не все Pod'ы running.
    • Учитываются только Ready Pod'ы.
    • Если Pod'ов меньше minReplicas — HPA скейлит до min.

  5. ТИПЫ МЕТРИК: RESOURCE, CUSTOM, EXTERNAL
  HPA v2 поддерживает три типа метрик.

  RESOURCE:
    • CPU, memory.
    • Источник: metrics-server.
    • Работает из коробки.
    • Самый простой и популярный.

    ПРИМЕР:
      metrics:
      - type: Resource
        resource:
          name: cpu
          target:
            type: Utilization
            averageUtilization: 70

  CUSTOM:
    • Метрики из Prometheus, Datadog, etc.
    • Источник: Prometheus Adapter, Custom Metrics API.
    • Требует установки адаптера.
    • Примеры: RPS, latency, queue size.

    ПРИМЕР:
      metrics:
      - type: Pods
        pods:
          metric:
            name: http_requests_per_second
          target:
            type: AverageValue
            averageValue: "100"

  EXTERNAL:
    • Метрики вне K8s: очередь в SQS, метрики БД.
    • Источник: External Metrics API.
    • Требует адаптера.
    • Примеры: длина очереди в RabbitMQ, лаг Kafka.

    ПРИМЕР:
      metrics:
      - type: External
        external:
          metric:
            name: sqs_queue_length
          target:
            type: AverageValue
            averageValue: "1000"

  ЧТО ИСПОЛЬЗОВАТЬ:
    • CPU — для большинства случаев.
    • Memory — осторожно (memory не освобождается быстро).
    • Custom — для точного скейла по нагрузке.
    • External — для очередей, event-driven.

  6. API-ВЕРСИИ: AUTOSCALING/V1, V2
  HPA менялся со временем.

  autoscaling/v1:
    • Только CPU.
    • Простой манифест.
    • Устарел, но поддерживается.

  autoscaling/v2beta1, v2beta2:
    • Добавлены memory, custom, external.
    • Устарели.

  autoscaling/v2:
    • Актуальная версия.
    • CPU + memory + custom + external.
    • behavior для тонкой настройки.
    • Используй только её.

  ПРИМЕР v1 (не используй):
    apiVersion: autoscaling/v1
    kind: HorizontalPodAutoscaler
    spec:
      scaleTargetRef:
        apiVersion: apps/v1
        kind: Deployment
        name: app
      minReplicas: 2
      maxReplicas: 10
      targetCPUUtilizationPercentage: 70

  ПРАВИЛО: всегда autoscaling/v2.

  7. МАНИФЕСТ HPA V2 — ПОЛНЫЙ РАЗБОР

  БАЗОВЫЙ HPA НА CPU:
    apiVersion: autoscaling/v2
    kind: HorizontalPodAutoscaler
    metadata:
      name: app
      namespace: demo
    spec:
      scaleTargetRef:
        apiVersion: apps/v1
        kind: Deployment
        name: app
      minReplicas: 2
      maxReplicas: 10
      metrics:
      - type: Resource
        resource:
          name: cpu
          target:
            type: Utilization
            averageUtilization: 70

  РАЗБОР:
    scaleTargetRef:
      Что масштабировать. apiVersion, kind, name. Обычно Deployment.
    minReplicas:
      Минимум Pod'ов. HPA не уйдёт ниже. Если 0 — может до 0 (нужен special feature).
    maxReplicas:
      Максимум Pod'ов. HPA не уйдёт выше.
    metrics:
      Список метрик. Может быть несколько. HPA выберет максимум из всех расчётов.

  HPA С НЕСКОЛЬКИМИ МЕТРИКАМИ:
    metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80

  HPA посчитает desired для каждой метрики и выберет МАКСИМУМ. Так безопаснее — не будет ситуации, когда CPU в норме, а память кончается.

  Типы target:
    Utilization — процент от requests.
    AverageValue — среднее значение на Pod.
    Value — общее значение.

  8. TARGETCPUUTILIZATIONPERCENTAGE И TARGET
  В v1: targetCPUUtilizationPercentage: 70. Это процент от requests.cpu.
  В v2:
    metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70

  ЧТО ЗНАЧИТ 70%:
    Если requests.cpu = 100m: 70% = 70m. HPA держит среднее потребление около 70m на Pod.
    Если Pod'ов 3, каждый ест 70m — всё ок. Если каждый ест 100m — HPA добавит Pod'ов. Если каждый ест 30m — HPA уберёт Pod'ов.

  КАКОЙ ПРОЦЕНТ СТАВИТЬ:
    • 50-70% — типично для CPU.
    • 80-90% — если хочешь экономить, но риск.
    • <50% — избыточно, переплата.

  ВАЖНО: HPA не работает, если utilization < 10% или > 100% (по умолчанию tolerance 0.1). Это защита от флаппинга.
  TOLERANCE: HPA не скейлит, если текущее значение близко к целевому. По умолчанию tolerance = 0.1 (10%).
  Если target = 70%, HPA не будет реагировать, пока utilization не выйдет за пределы 60-80%.

  9. MINREPLICAS, MAXREPLICAS — ГРАНИЦЫ
  MINREPLICAS:
    • Минимум Pod'ов.
    • HPA не уйдёт ниже, даже если нагрузка нулевая.
    • Зачем: отказоустойчивость. Даже ночью нужно 2 Pod'а, чтобы пережить падение одного.
    • Обычно 2-3.

  MAXREPLICAS:
    • Максимум Pod'ов.
    • HPA не уйдёт выше, даже если нагрузка огромная.
    • Зачем: защита от бесконечного роста. Ограничение бюджета и ресурсов кластера.
    • Обычно 10-50.

  ЧТО ЕСЛИ MAXREPLICAS ДОСТИГНУТ: HPA перестаёт добавлять Pod'ы. Если нагрузка продолжает расти — сервис деградирует.
  Нужно: увеличить maxReplicas, оптимизировать код, добавить Cluster Autoscaler (если не хватает нод).

  ЧТО ЕСЛИ MINREPLICAS = 0: HPA может убрать все Pod'ы. Сервис недоступен. Используется для CronJob-like сервисов,
  очередей (нет сообщений — нет Pod'ов). Требует специальной настройки (feature gate).

  ПРАВИЛО: minReplicas >= 2 для прода. maxReplicas — с запасом, но не бесконечно.

  10. ПОЧЕМУ REQUESTS.CPU ОБЯЗАТЕЛЕН
  HPA смотрит на utilization = потребление / requests.
  ЕСЛИ REQUESTS НЕ УКАЗАН: utilization не определён. HPA не может посчитать. Показывает <unknown>. НЕ СКЕЙЛИТ.

  ПРИМЕР:
    Без requests:
      resources:
        limits:
          cpu: "500m"
    HPA не работает.

    С requests:
      resources:
        requests:
          cpu: "100m"
        limits:
          cpu: "500m"
    HPA работает.

  ВАЖНО: requests.cpu должен быть реалистичным. Если занизить — HPA будет скейлить слишком агрессивно.
  Если завысить — HPA будет скейлить слишком поздно.

  КАК ВЫБРАТЬ REQUESTS:
    1. Запусти сервис под нагрузкой.
    2. Посмотри kubectl top pods.
    3. Возьми среднее потребление.
    4. requests = среднее + 20-30%.
    5. limits = requests * 2-3.

  ПРАВИЛО: Без requests.cpu HPA — бесполезен.

  11. BEHAVIOR: SCALEUP И SCALEDOWN ПОЛИТИКИ
  HPA v2 позволяет тонко настроить поведение.
  ЗАЧЕМ:
    • Быстро скейлить вверх при пиках.
    • Медленно скейлить вниз, чтобы не флаппить.
    • Ограничить скорость изменений.

  ПРИМЕР:
    behavior:
      scaleUp:
        stabilizationWindowSeconds: 0
        policies:
        - type: Percent
          value: 100
          periodSeconds: 15
        - type: Pods
          value: 4
          periodSeconds: 15
        selectPolicy: Max
      scaleDown:
        stabilizationWindowSeconds: 300
        policies:
        - type: Percent
          value: 10
          periodSeconds: 60

  РАЗБОР:
    scaleUp:
      • stabilizationWindowSeconds: 0 — реагировать сразу.
      • policies: не больше 100% или 4 Pod'ов за 15 секунд.
      • selectPolicy: Max — выбирать максимальное изменение.
    scaleDown:
      • stabilizationWindowSeconds: 300 — ждать 5 минут перед уменьшением.
      • policies: не больше 10% за 60 секунд.

  ЧТО ЭТО ДАЁТ:
    • Быстрый рост при пиках.
    • Медленное снижение — защита от флаппинга.
    • Контроль скорости изменений.

  ПРАВИЛО: В проде всегда настраивай behavior. Дефолты могут быть слишком агрессивными.

  12. STABILIZATION WINDOW — ЗАЩИТА ОТ ФЛАППИНГА

  ФЛАППИНГ — это когда HPA постоянно то добавляет, то убирает Pod'ы.
  ПРИЧИНА: Нагрузка колеблется: 60% → 80% → 60% → 80%. HPA: 3 Pod'а → 5 Pod'ов → 3 Pod'а → 5 Pod'ов.
  ПОСЛЕДСТВИЯ:
    • Постоянные перезапуски.
    • Нестабильность.
    • Лишняя нагрузка на API Server.
    • Пользователи страдают.

  РЕШЕНИЕ: stabilization window.
  КАК РАБОТАЕТ: HPA смотрит на рекомендации за последние N секунд. Берёт МАКСИМУМ для scaleUp и МИНИМУМ для scaleDown.

  ДЛЯ SCALEUP: stabilizationWindowSeconds: 0. Реагировать сразу. Пики требуют быстрой реакции.
  ДЛЯ SCALEDOWN: stabilizationWindowSeconds: 300. Ждать 5 минут. Если нагрузка стабильно низкая — уменьшить.

  ПРАВИЛО: scaleUp: 0-30 секунд. scaleDown: 300-600 секунд.

  13. HPA И DEPLOYMENT: КАК ОНИ СВЯЗАНЫ
  HPA не создаёт Pod'ы напрямую. HPA меняет replicas в Deployment.

  СХЕМА:
    ┌─────────────────────────────────────┐
    │  HPA                                │
    │  minReplicas: 2, maxReplicas: 10    │
    │  targetCPU: 70%                     │
    └───────────────┬─────────────────────┘
                    │ меняет replicas
                    ▼
    ┌─────────────────────────────────────┐
    │  Deployment (app)                   │
    │  replicas: 3 → 5                    │
    └───────────────┬─────────────────────┘
                    │ создаёт
                    ▼
    ┌─────────────────────────────────────┐
    │  ReplicaSet                         │
    └───────────────┬─────────────────────┘
                    │ создаёт
                    ▼
    ┌─────────────────────────────────────┐
    │  Pod'ы                              │
    └─────────────────────────────────────┘

  ЧТО ВАЖНО:
    • HPA и Deployment должны быть в одном namespace.
    • scaleTargetRef указывает на Deployment.
    • HPA не работает с голыми Pod'ами (только контроллеры).
    • HPA работает с Deployment, StatefulSet, ReplicaSet.

  КОНФЛИКТ: Если ты вручную меняешь replicas, а HPA тоже — они будут конфликтовать. HPA перезапишет твоё значение.
  Не меняй replicas вручную, если есть HPA.

  14. HPA И SERVICE: ENDPOINTS ОБНОВЛЯЮТСЯ
  Когда HPA добавляет Pod'ы:
    1. ReplicaSet создаёт Pod.
    2. Pod проходит readinessProbe.
    3. Service Controller добавляет Pod в Endpoints.
    4. Трафик начинает идти на новый Pod.

  Когда HPA убирает Pod'ы:
    1. HPA уменьшает replicas.
    2. ReplicaSet удаляет Pod.
    3. Pod получает SIGTERM.
    4. Pod убирается из Endpoints.
    5. Трафик перестаёт идти на Pod.
    6. Pod завершается.

  ВАЖНО: Без readinessProbe HPA может добавить Pod, который ещё не готов. Трафик пойдёт в мёртвый Pod. Всегда настраивай readinessProbe.
  ВАЖНО: Без graceful shutdown при удалении Pod'а активные запросы обрываются. Всегда настраивай graceful shutdown.

  15. КАСТОМНЫЕ МЕТРИКИ ЧЕРЕЗ PROMETHEUS ADAPTER
  CPU — не всегда лучшая метрика.
  ПРОБЛЕМА: Go-сервис может есть мало CPU, но обрабатывать много запросов. Или наоборот — ждать БД, есть мало CPU, но latency растёт.
  РЕШЕНИЕ: кастомные метрики.

  PROMETHEUS ADAPTER:
    • Устанавливается в кластер.
    • Читает метрики из Prometheus.
    • Отдаёт их через Custom Metrics API.
    • HPA может их использовать.

  ПРИМЕР МЕТРИКИ: http_requests_per_second, http_request_duration_seconds, queue_size.

  МАНИФЕСТ HPA:
    metrics:
    - type: Pods
      pods:
        metric:
          name: http_requests_per_second
        target:
          type: AverageValue
          averageValue: "100"

  ЧТО ЭТО ЗНАЧИТ: Держать 100 RPS на Pod. Если RPS растёт — HPA добавляет Pod'ов.

  ПЛЮСЫ:
    • Точнее, чем CPU.
    • Реагирует на реальную нагрузку.
    • Не зависит от эффективности кода.

  МИНУСЫ:
    • Сложнее настроить.
    • Нужен Prometheus + Adapter.
    • Задержка метрик (Prometheus scrape interval).

  КОГДА ИСПОЛЬЗОВАТЬ: CPU не коррелирует с нагрузкой, event-driven архитектура, очереди (Kafka, RabbitMQ, SQS).

  16. ВНЕШНИЕ МЕТРИКИ (EXTERNAL METRICS) И KEDA
  Метрики вне K8s.
  ПРИМЕРЫ: длина очереди в SQS, лаг в Kafka, количество сообщений в RabbitMQ, метрики облачного балансировщика.

  МАНИФЕСТ:
    metrics:
    - type: External
      external:
        metric:
          name: sqs_queue_length
          selector:
            matchLabels:
              queue: orders
        target:
          type: AverageValue
          averageValue: "1000"

  ЧТО ЭТО ЗНАЧИТ: Держать 1000 сообщений на Pod. Если очередь растёт — HPA добавляет Pod'ов.
  КОГДА ИСПОЛЬЗОВАТЬ: Worker'ы, которые читают из очереди, event-driven архитектура, batch-обработка.
  ВАЖНО: External Metrics API должен быть настроен. Обычно через Prometheus Adapter или KEDA.

  KEDA:
    • Kubernetes Event-Driven Autoscaling.
    • Упрощает external metrics.
    • Поддерживает SQS, Kafka, RabbitMQ, Redis и др.
    • Может скейлить до 0.

  17. ДИАГНОСТИКА HPA: ПОЧЕМУ НЕ СКЕЙЛИТ

  СМОТРЕТЬ СТАТУС:
    kubectl get hpa -n demo
    # NAME   REFERENCE         TARGETS         MINPODS   MAXPODS   REPLICAS   AGE
    # app    Deployment/app    45%/70%         2         10        3          5m

  ЧТО ЗНАЧАТ КОЛОНКИ:
    TARGETS: текущее/целевое. Если <unknown> — нет метрик. Если 45%/70% — всё ок, HPA не скейлит.

  ЕСЛИ <unknown>:
    • metrics-server не установлен.
    • requests.cpu не указан.
    • Pod'ы не Ready.
    • Метрика недоступна.

    Проверить: kubectl top pods -n demo; kubectl describe hpa app -n demo.

  ЕСЛИ НЕ СКЕЙЛИТ, ХОТЯ НАГРУЗКА ЕСТЬ:
    1. requests.cpu указан?
    2. utilization в пределах tolerance?
    3. maxReplicas достигнут?
    4. stabilization window?
    5. Метрика правильная?

  DESCRIBE HPA:
    kubectl describe hpa app -n demo
    Смотреть: Conditions, Events, Current Metrics, ScalingActive.

  ТИПОВЫЕ ОШИБКИ:
    "failed to get cpu utilization: missing request for cpu" → Не указан requests.cpu.
    "failed to get metrics: no metrics returned" → metrics-server не работает.
    "the HPA was unable to compute the replica count" → Проблема с метрикой.

  18. HPA VS VPA VS CLUSTER AUTOSCALER
  ┌──────────────────┬──────────────┬──────────────┬─────────────────┐
  │                  │ HPA          │ VPA          │ Cluster         │
  │                  │              │              │ Autoscaler      │
  ├──────────────────┼──────────────┼──────────────┼─────────────────┤
  │ Что меняет       │ Кол-во Pod'ов│ Requests/    │ Кол-во нод      │
  │                  │              │ limits       │                 │
  │ Направление      │ Горизонтально│ Вертикально  │ Горизонтально   │
  │ Уровень          │ Deployment   │ Pod          │ Cluster         │
  │ Требует          │ metrics-     │ metrics-     │ Cloud provider  │
  │                  │ server       │ server       │                 │
  │ Перезапуск       │ Нет          │ Да (обычно)  │ Нет             │
  │ Встроен в K8s    │ Да           │ Нет          │ Нет             │
  │ Сценарий         │ Нагрузка     │ Оптимизация  │ Нехватка нод    │
  │                  │ растёт       │ ресурсов     │                 │
  └──────────────────┴──────────────┴──────────────┴─────────────────┘

  КОМБИНАЦИЯ: HPA + Cluster Autoscaler — классика. HPA создаёт Pod'ы → Cluster Autoscaler добавляет ноды.

  19. СВЯЗЬ С GO
  HPA требует, чтобы Go-сервис был stateless.
  ЧТО ЭТО ЗНАЧИТ:
    • Нет сессий в памяти Pod'а.
    • Нет локальных файлов с данными.
    • Нет привязки к конкретному Pod'у.
    • Любой Pod может обработать любой запрос.

  ЕСЛИ СЕССИИ В ПАМЯТИ: HPA добавляет Pod → новый Pod не знает сессий. HPA убирает Pod → сессии теряются. Пользователи разлогиниваются.
  РЕШЕНИЕ: Сессии в Redis. JWT (stateless). Sticky sessions (костыль).

  GRACEFUL SHUTDOWN:
    При удалении Pod'а kubelet посылает SIGTERM. Go-сервис должен:
      • Поймать SIGTERM.
      • Завершить активные запросы.
      • Закрыть соединения.
      • Выйти с 0.
    Без этого HPA будет обрывать запросы.

  ПРИМЕР:
    var ready atomic.Bool

    func main() {
        ctx, cancel := signal.NotifyContext(
            context.Background(), syscall.SIGTERM)
        defer cancel()

        go func() {
            ready.Store(true)
            srv.ListenAndServe()
        }()

        <-ctx.Done()
        ready.Store(false)

        shutdownCtx, cancelShutdown := context.WithTimeout(
            context.Background(), 20*time.Second)
        defer cancelShutdown()
        srv.Shutdown(shutdownCtx)
    }

  READINESS PROBE: HPA учитывает только Ready Pod'ы. Если readinessProbe не проходит — Pod не учитывается. Всегда настраивай readinessProbe.

  RESOURCES:
    requests.cpu — обязателен для HPA.
    limits.cpu — опционален, но рекомендуется.
    GOMEMLIMIT — должен быть на 10-20% меньше limits.memory.

  ПРИМЕР:
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
      limits:
        cpu: "500m"
        memory: "256Mi"
    GOMEMLIMIT=230MiB
  ЧТО ВАЖНО В КОДЕ: Stateless. Graceful shutdown. Readiness probe. Реалистичные requests.

  20. АНТИПАТТЕРНЫ
  20.1. HPA БЕЗ REQUESTS.CPU. HPA не работает. <unknown> в TARGETS.
  20.2. HPA БЕЗ METRICS-SERVER. HPA не работает. <unknown> в TARGETS.
  20.3. HPA НА STATEFUL-ПРИЛОЖЕНИИ. Сессии теряются. Пользователи разлогиниваются.
  20.4. MINREPLICAS: 1. Нет отказоустойчивости. Упал Pod — сервис недоступен.
  20.5. MAXREPLICAS: 1000. Кластер не выдержит. Или бюджет.
  20.6. НЕТ READINESSPROBE. HPA добавляет Pod'ы, которые не готовы. Трафик идёт в мёртвые Pod'ы.
  20.7. НЕТ GRACEFUL SHUTDOWN. HPA убирает Pod'ы, запросы обрываются.
  20.8. АГРЕССИВНЫЙ SCALEDOWN. Флаппинг. Постоянные перезапуски.
  20.9. HPA НА CPU БЕЗ ПРИЧИНЫ. CPU не коррелирует с нагрузкой. HPA скейлит не туда.
  20.10. HPA И VPA НА ОДНИХ МЕТРИКАХ. Конфликт. Непредсказуемое поведение.
  20.11. РУЧНОЕ ИЗМЕНЕНИЕ REPLICAS ПРИ HPA. HPA перезапишет. Конфликт.
  20.12. HPA БЕЗ BEHAVIOR. Дефолты могут быть слишком агрессивными.
  20.13. HPA НА LATENCY. Latency растёт → HPA добавляет Pod'ы →
         latency растёт ещё больше (проблема в БД, а не в Pod'ах). Latency — плохая метрика для HPA.
  20.14. HPA И CLUSTER AUTOSCALER БЕЗ СОГЛАСОВАНИЯ. HPA создаёт Pod'ы, Cluster Autoscaler не успевает добавлять ноды. Pod'ы в Pending.

  21. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  HPA — автоматическое изменение количества Pod'ов на основе метрик.
  2.  Требует metrics-server и requests.cpu.
  3.  Формула: desiredReplicas = ceil[currentReplicas * (currentMetricValue / desiredMetricValue)].
  4.  Метрики: Resource (CPU, memory), Custom (Prometheus), External (SQS, Kafka).
  5.  API: autoscaling/v2 — актуальная версия.
  6.  minReplicas >= 2 для прода.
  7.  maxReplicas — с запасом, но не бесконечно.
  8.  targetCPUUtilizationPercentage: 50-70% — типично.
  9.  Behavior: scaleUp быстро, scaleDown медленно.
  10. Stabilization window: 0 для scaleUp, 300+ для scaleDown.
  11. HPA меняет replicas в Deployment.
  12. HPA и Service: endpoints обновляются автоматически.
  13. Кастомные метрики — через Prometheus Adapter.
  14. External метрики — через KEDA или Prometheus Adapter.
  15. Диагностика: kubectl get hpa, describe hpa, top pods.
  16. HPA + Cluster Autoscaler — классика для прода.
  17. Go-сервис должен быть stateless, с graceful shutdown, readinessProbe и requests.cpu.
  18. Антипаттерны: без requests, без metrics-server, minReplicas: 1, нет readinessProbe, нет graceful shutdown,
      агрессивный scaleDown, HPA на stateful, HPA и VPA на одних метриках.
*/
