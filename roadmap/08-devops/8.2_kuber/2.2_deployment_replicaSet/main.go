package main

/*
  УРОК 2.2: DEPLOYMENT И REPLICASET
  Ты никогда не создаёшь Pod'ы руками в проде. Pod — эфемерный:
  упал — исчез, второго не появится. Pod не обновляется, не
  масштабируется, не раскидывается по нодам. Для всего этого нужен контроллер.
  Deployment — основной контроллер для stateless-приложений.
  Через него ты описываешь желаемое состояние: «хочу 3 реплики
  my-app:1.0». K8s сам добивается этого состояния и удерживает
  его.

  СОДЕРЖАНИЕ:
    1.  Зачем Deployment, если есть Pod
    2.  ReplicaSet — что это
    3.  Иерархия Deployment → ReplicaSet → Pod
    4.  Структура манифеста Deployment
    5.  selector, labels, template — критическая связка
    6.  Стратегия RollingUpdate
    7.  maxSurge и maxUnavailable
    8.  Стратегия Recreate
    9.  История ревизий и откат
    10. Zero-downtime деплой — как это работает
    11. Полезные команды
    12. Связь с Go
    13. Антипаттерны
    14. Финальные выводы

  1. ЗАЧЕМ DEPLOYMENT, ЕСЛИ ЕСТЬ POD

  POD НАПРЯМУЮ — плохо. Почему:
    • Не перезапустится при падении.
    • Не обновится при новой версии образа.
    • Не масштабируется.
    • Не раскидывается по нодам.
    • Не откатится при ошибке.

  Пример: создал Pod с nginx:1.27. Хочешь 3 реплики — надо
  вручную создавать ещё два. Упал один — вручную создавать
  заново. Вышла 1.28 — вручную удалять и создавать новые.

  DEPLOYMENT РЕШАЕТ ВСЁ ЭТО. Ты пишешь:
    replicas: 3
    image: my-app:1.0

  K8s сам:
    • Держит 3 реплики.
    • Перезапускает упавшие.
    • Раскидывает по нодам.
    • Обновляет версии без downtime.
    • Откатывает при ошибке.

  Правило: в проде Pod'ы всегда создаются через контроллеры.
  Deployment — для stateless. StatefulSet — для stateful.
  Job — для одноразовых.

  2. REPLICASET — ЧТО ЭТО
  ReplicaSet — это контроллер, который следит за количеством
  Pod'ов. Создаётся автоматически Deployment'ом.

  ЧТО ДЕЛАЕТ:
    • Смотрит: сколько Pod'ов с моими labels сейчас?
    • Если меньше нужного — создаёт новые.
    • Если больше — убивает лишние.

  ПРИМЕР:
    ReplicaSet хочет 3 Pod'а.
    Сейчас 2 (один упал).
    ReplicaSet создаёт новый.
    Через секунду снова 3.

  ЗАЧЕМ ОТДЕЛЬНЫЙ ОБЪЕКТ:
    • ReplicaSet можно использовать напрямую (редко).
    • Deployment использует ReplicaSet для версионирования.
    • При обновлении Deployment создаёт НОВЫЙ ReplicaSet,
      старый оставляет для отката.

  НЕ ИСПОЛЬЗУЙ REPLICASET НАПРЯМУЮ.
    Он не умеет rolling update. Только Deployment.

  3. ИЕРАРХИЯ DEPLOYMENT → REPLICASET → POD
  Ты создаёшь Deployment.
  Deployment создаёт ReplicaSet.
  ReplicaSet создаёт Pod'ы.

  СХЕМА:
    ┌─────────────────────────────────┐
    │       Deployment (app)          │
    │  replicas: 3, image: v1.0       │
    └───────────────┬─────────────────┘
                    │ создаёт
                    ▼
    ┌─────────────────────────────────┐
    │    ReplicaSet (app-abc123)      │
    │  (соответствует версии v1.0)    │
    └───────────────┬─────────────────┘
                    │ создаёт
        ┌───────────┼───────────┐
        ▼           ▼           ▼
    ┌────────┐  ┌────────┐  ┌────────┐
    │ Pod 1  │  │ Pod 2  │  │ Pod 3  │
    │ v1.0   │  │ v1.0   │  │ v1.0   │
    └────────┘  └────────┘  └────────┘

  ПРИ ОБНОВЛЕНИИ ДО v2.0:
    Deployment создаёт НОВЫЙ ReplicaSet для v2.0.
    Старый ReplicaSet (v1.0) уменьшается до 0, но не удаляется.
    Постепенно Pod'ы v1.0 заменяются на v2.0.

  СХЕМА ПОСЛЕ ОБНОВЛЕНИЯ:
    ┌─────────────────────────────────┐
    │       Deployment (app)          │
    │  image: v2.0                    │
    └───────┬─────────────────┬───────┘
            │                 │
            ▼                 ▼
    ┌───────────────┐  ┌───────────────┐
    │ RS v1.0       │  │ RS v2.0       │
    │ replicas: 0   │  │ replicas: 3   │
    └───────────────┘  └───────┬───────┘
                               │
                    ┌──────────┼──────────┐
                    ▼          ▼          ▼
                ┌────────┐ ┌────────┐ ┌────────┐
                │ Pod 1  │ │ Pod 2  │ │ Pod 3  │
                │ v2.0   │ │ v2.0   │ │ v2.0   │
                └────────┘ └────────┘ └────────┘

  ЗАЧЕМ СТАРЫЙ RS ОСТАВЛЯТЬ:
    Для отката. kubectl rollout undo мгновенно вернёт старый
    ReplicaSet. Не надо пересобирать, не надо ждать.

  4. СТРУКТУРА МАНИФЕСТА DEPLOYMENT

  Минимальный Deployment:
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: app
      labels:
        app: app
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

  РАЗБОР ПОЛЕЙ:
    apiVersion: apps/v1       — версия API (для Deployment — apps/v1).
    kind: Deployment          — тип объекта.
    metadata.name             — имя.
    metadata.labels           — метки самого Deployment.

    spec.replicas             — сколько Pod'ов.
    spec.selector             — по каким labels находить «свои» Pod'ы.
    spec.template             — шаблон Pod'а.
    spec.template.metadata.labels — labels Pod'ов.
    spec.template.spec        — spec Pod'а (containers, volumes и т.д.).

  ОБРАТИ ВНИМАНИЕ:
    labels в template ДОЛЖНЫ совпадать с selector.matchLabels.
    Иначе Deployment не поймёт, какие Pod'ы его.

  5. SELECTOR, LABELS, TEMPLATE — КРИТИЧЕСКАЯ СВЯЗКА
  Это самая частая причина ошибок у новичков.

  ПРАВИЛЬНО:
    spec:
      selector:
        matchLabels:
          app: app          # ← эти labels
      template:
        metadata:
          labels:
            app: app        # ← должны совпадать

  Deployment ищет Pod'ы по selector.matchLabels. Если labels
  в template не совпадают — ReplicaSet создаёт Pod'ы, но не
  считает их «своими». Получается бесконечный цикл создания.

  ОШИБКА:
    selector:
      matchLabels:
        app: my-app
    template:
      metadata:
        labels:
          app: app          # ← не совпадает

  Compose выдаст ошибку: «selector does not match template labels».

  СОВЕТ: используй один и тот же label (`app: <name>`) везде.
  Не изобретай разные имена.

  ЛУЧШИЙ ПОДХОД:
    metadata:
      name: app
    spec:
      selector:
        matchLabels:
          app: app          # имя сервиса
      template:
        metadata:
          labels:
            app: app        # то же самое

  ДОПОЛНИТЕЛЬНЫЕ LABELS:
    Можно добавлять в template больше labels, чем в selector.
    Например:
      selector:
        matchLabels:
          app: app
      template:
        metadata:
          labels:
            app: app
            version: v1     # ← дополнительная метка для Service

  Service может выбрать только Pod'ы с `version: v1` (или без —
  зависит от selector).

  6. СТРАТЕГИЯ ROLLINGUPDATE
  RollingUpdate — дефолтная стратегия. Обновление без downtime.

  КАК РАБОТАЕТ:
    1. Создаётся новый ReplicaSet с новой версией.
    2. Увеличивает replicas нового RS на 1.
    3. Ждёт, пока новый Pod станет Ready.
    4. Уменьшает replicas старого RS на 1.
    5. Повторяет, пока все Pod'ы не обновятся.

  ПРИМЕР ПОШАГОВО (3 реплики):
    Старт:               RS-v1 [3] RS-v2 [0]
    Шаг 1:               RS-v1 [3] RS-v2 [1]  (новый Pod стартует)
    Шаг 2 (после Ready): RS-v1 [2] RS-v2 [1]  (убрали старый)
    Шаг 3:               RS-v1 [2] RS-v2 [2]
    Шаг 4:               RS-v1 [1] RS-v2 [2]
    Шаг 5:               RS-v1 [1] RS-v2 [3]
    Шаг 6:               RS-v1 [0] RS-v2 [3]  (всё обновилось)

  ВСЁ ВРЕМЯ ДОСТУПНЫ:
    Минимум maxUnavailable живых Pod'ов. Если maxUnavailable=0
    и replicas=3 — всегда 3+ живых.

  ЕСЛИ НОВАЯ ВЕРСИЯ ПАДАЕТ:
    Новый Pod не становится Ready. Rolling update зависает.
    Дальнейшие шаги не выполняются.
    Старые Pod'ы продолжают работать.
    Ты видишь проблему через `kubectl rollout status` или
    `kubectl get pods`.

    Откат: kubectl rollout undo.

  НАСТРОЙКА:
    spec:
      strategy:
        type: RollingUpdate
        rollingUpdate:
          maxSurge: 1
          maxUnavailable: 0

  7. MAXSURGE И MAXUNAVAILABLE
  Эти два параметра определяют, как агрессивно катится
  обновление.

  MAXSURGE — сколько Pod'ов можно создать СВЕРХ replicas.
    maxSurge: 1     → на 1 Pod больше в моменте.
    maxSurge: 30%   → на 30% больше (для 10 реплик = 3).
    maxSurge: 0     → не создавать лишних. Обновление медленнее.

  MAXUNAVAILABLE — сколько Pod'ов может быть недоступно.
    maxUnavailable: 0    → всегда все живы. Zero-downtime.
    maxUnavailable: 1    → на 1 Pod можно убрать.
    maxUnavailable: 30%  → на 30% можно убрать.

  КОМБИНАЦИИ:
    ZERO-DOWNTIME (рекомендуется для прода):
      maxSurge: 1
      maxUnavailable: 0

      Всегда все Pod'ы живы. Создаётся один лишний, он
      становится Ready, потом убирается один старый.

    БЫСТРОЕ ОБНОВЛЕНИЕ:
      maxSurge: 25%
      maxUnavailable: 25%

      Создаётся и убирается больше Pod'ов параллельно.
      Быстрее, но есть момент недоступности.

    БЕЗ ЛИШНИХ РЕСУРСОВ:
      maxSurge: 0
      maxUnavailable: 1

      Не создаём сверх replicas. Убираем один, создаём один.
      Медленнее, но не нужно свободных ресурсов на нодах.

  ЧТО ВЫБРАТЬ ДЛЯ GO-СЕРВИСА:
    maxSurge: 1
    maxUnavailable: 0

    Потому что Go-сервис обычно лёгкий, и один лишний Pod
    не проблема. Zero-downtime важнее.

  ВАЖНО:
    Не ставь maxUnavailable: 0 и maxSurge: 0 одновременно.
    Обновление не сдвинется с места — нельзя ни создать,
    ни убить Pod.

  8. СТРАТЕГИЯ RECREATE
  Recreate — убить все старые, создать все новые.

  КОГДА ИСПОЛЬЗУЕТСЯ:
    • Несовместимые версии (нельзя, чтобы старые и новые
      Pod'ы работали одновременно).
    • Миграции схемы БД, которые ломают старую версию.
    • Простой сервис, где downtime не критичен.

  ПРИМЕР:
    spec:
      strategy:
        type: Recreate

  ЧТО ПРОИСХОДИТ:
    1. Убиваются все Pod'ы старого ReplicaSet.
    2. Создаётся новый ReplicaSet.
    3. Запускаются все Pod'ы новой версии.

  DOWNTIME: есть. На время между 1 и 3 сервис недоступен.

  ДЛЯ СТАТЕЙНЫХ ПРИЛОЖЕНИЙ:
    Если версия меняет схему БД — только Recreate. Иначе
    старая версия может испортить данные, которые новая
    уже мигрировала.

  АНТИПАТТЕРН:
    Использовать Recreate для stateless-сервисов без причины.
    RollingUpdate лучше.

  9. ИСТОРИЯ РЕВИЗИЙ И ОТКАТ
  K8s хранит историю ревизий Deployment. Каждое изменение
  template создаёт новую ревизию.

  ПОСМОТРЕТЬ ИСТОРИЮ:
    kubectl rollout history deployment/app

    # REVISION  CHANGE-CAUSE
    # 1         <none>
    # 2         <none>
    # 3         <none>

  CHANGE-CAUSE пустое, потому что мы не указали. Можно указать:
    kubectl annotate deployment/app \
      kubernetes.io/change-cause="update to v2.0.0"

  ОТКАТ:
    kubectl rollout undo deployment/app
    Мгновенно возвращает предыдущую ревизию.

  ОТКАТ К КОНКРЕТНОЙ РЕВИЗИИ:
    kubectl rollout undo deployment/app --to-revision=1

  КАК ЭТО РАБОТАЕТ ПОД КАПОТОМ:
    Старые ReplicaSet'ы остаются в кластере с replicas: 0.
    Undo просто возвращает нужный RS в active, а текущий
    уменьшает до 0.

    Ничего не пересобирается, не перекачивается. Мгновенно.

  СКОЛЬКО РЕВИЗИЙ ХРАНИТЬ:
    spec:
      revisionHistoryLimit: 10    # дефолт 10, старые удаляются

  УДАЛИТЬ ИСТОРИЮ:
    Удаление Deployment удаляет все его RS.

  10. ZERO-DOWNTIME ДЕПЛОЙ — КАК ЭТО РАБОТАЕТ

  Zero-downtime деплой — это совместная работа:
    • Deployment (rolling update).
    • ReplicaSet (правильное количество).
    • Probes (готовность Pod'а).
    • Service (endpoints обновляются).
    • Graceful shutdown (в Go-коде).

  ПОШАГОВО ПРИ ОБНОВЛЕНИИ:
    1. Ты запускаешь kubectl set image deployment/app ... .
    2. Deployment создаёт новый ReplicaSet.
    3. Новый RS создаёт Pod v2.
    4. Kubelet запускает контейнер в Pod'е.
    5. Startup probe проверяет, что Pod запустился.
    6. Readiness probe проверяет, что Pod готов.
    7. Service добавляет Pod в Endpoints (трафик пошёл).
    8. Deployment уменьшает старый RS на 1.
    9. Старый Pod получает SIGTERM.
    10. preStop hook (если есть) выполняется.
    11. Go-сервис делает graceful shutdown.
    12. Pod убирается из Endpoints.
    13. Pod удаляется.
    14. Возврат к шагу 3 для следующего Pod'а.

  ЧТО ВАЖНО ДЛЯ GO:
    • Порт 8080 — kubelet шлёт probes.
    • /health для liveness и startup.
    • /ready для readiness.
    • SIGTERM ловится через signal.NotifyContext.
    • srv.Shutdown завершает активные запросы.
    • terminationGracePeriodSeconds >= времени shutdown.

  ЕСЛИ ЧТО-ТО НЕ ТАК:
    • Probes не настроены → трафик идёт до готовности,
      клиенты получают 500.
    • Readiness не проходит при shutdown → Service не
      успевает убрать Pod → часть запросов уходит в мёртвый.
    • Нет graceful shutdown → активные запросы обрываются.

  ИДЕАЛЬНАЯ КОМБИНАЦИЯ:
    Deployment:
      maxSurge: 1
      maxUnavailable: 0
      terminationGracePeriodSeconds: 30

    Pod:
      startupProbe:    /health, period 2s, failure 30
      livenessProbe:   /health, period 10s
      readinessProbe:  /ready,  period 5s
      preStop:         sleep 5

    Go-код:
      signal.NotifyContext(SIGTERM)
      ready.Store(false) — сразу убирает из Service
      srv.Shutdown(20s)

  11. ПОЛЕЗНЫЕ КОМАНДЫ

  СМОТРЕТЬ:
    kubectl get deployments
    kubectl get deployments -o wide
    kubectl describe deployment app
    kubectl get replicasets
    kubectl get pods -l app=app

  МАСШТАБ:
    kubectl scale deployment/app --replicas=5

  ОБНОВИТЬ ОБРАЗ:
    kubectl set image deployment/app app=my-app:2.0.0

  СТАТУС ОБНОВЛЕНИЯ:
    kubectl rollout status deployment/app

  ИСТОРИЯ:
    kubectl rollout history deployment/app

  ОТКАТ:
    kubectl rollout undo deployment/app
    kubectl rollout undo deployment/app --to-revision=2

  ПАУЗА ОБНОВЛЕНИЯ:
    kubectl rollout pause deployment/app
    kubectl rollout resume deployment/app

  ПЕРЕЗАПУСК (без изменения YAML):
    kubectl rollout restart deployment/app

    Полезно после изменения ConfigMap/Secret — Pod'ы
    не перечитают env автоматически.

  ПРИМЕРЫ ОПИСАНИЯ:
    kubectl describe deployment app

    Смотреть:
      • Replicas (desired / updated / total / available).
      • Conditions (Available, Progressing).
      • Strategy (RollingUpdate / Recreate).
      • Events (внизу).

  12. СВЯЗЬ С GO
  Go-сервис деплоится через Deployment. Всё, что нужно
  сделать Go-разработчику:

  1. DOCKERFILE:
     FROM golang:1.22-alpine AS builder
     WORKDIR /src
     COPY . .
     RUN CGO_ENABLED=0 go build -o /out/server .

     FROM gcr.io/distroless/static-debian12:nonroot
     COPY --from=builder /out/server /server
     USER nonroot:nonroot
     EXPOSE 8080
     ENTRYPOINT ["/server"]

  2. DEPLOYMENT:
     С образом my-app:1.0.0, ресурсами, probes, env.

  3. PROBES:
     /health — для liveness и startup.
     /ready — для readiness (проверяет БД).

  4. GRACEFUL SHUTDOWN:
     signal.NotifyContext + srv.Shutdown + ready.Store(false).

  5. terminationGracePeriodSeconds:
     Больше, чем максимальное время shutdown.

  ЧТО РЕАЛЬНО ВАЖНО В КОДЕ:
    • Разделение /health и /ready — liveness не проверяет БД.
    • ready = atomic.Bool — переключается мгновенно.
    • При SIGTERM: ready=false, потом srv.Shutdown.
    • Общее время shutdown < terminationGracePeriodSeconds.

  ПРИМЕР КОДА:
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

  13. АНТИПАТТЕРНЫ

  13.1. SELECTOR НЕ СОВПАДАЕТ С TEMPLATE.LABELS.
    Deployment не находит свои Pod'ы. Бесконечное создание
    новых. Compose ругнётся.

  13.2. maxUNAVAILABLE: 0 И maxSURGE: 0.
    Обновление не сдвинется. Нельзя ни создать, ни убить Pod.

  13.3. НЕТ PROBES.
    Трафик идёт в Pod'ы, которые ещё не готовы. Клиенты
    получают 500 при обновлении.

  13.4. НЕТ terminationGracePeriodSeconds ДЛЯ МЕДЛЕННЫХ СЕРВИСОВ.
    Go-сервис не успевает завершиться. SIGKILL на середине shutdown.

  13.5. RECREATE БЕЗ ПРИЧИНЫ.
    Downtime там, где можно было обойтись.

  13.6. REVISIONHISTORYLIMIT: 0.
    Нельзя откатиться. Undo не работает.

  13.7. LATEST В ОБРАЗЕ.
    Каждый pull может принести новую версию. Rolling update
    не сработает как ожидалось. Фиксируй тег или digest.

  13.8. НЕТ RESOURCES REQUESTS.
    Scheduler не знает, куда ставить Pod. BestEffort QoS.

  13.9. ИЗМЕНИЛ CONFIGMAP — НЕ ПЕРЕЗАПУСТИЛ POD'Ы.
    Deployment не следит за ConfigMap. Pod'ы не перечитают
    env. Нужно kubectl rollout restart.

  13.10. ИСПОЛЬЗОВАТЬ REPLICASET НАПРЯМУЮ.
    Нет rolling update. Только для специфических случаев.

  13.11. ОДИН РЕПЛИКА В ПРОДЕ.
    Downtime при обновлении. Не нужен zero-downtime.

  13.12. МЕНЯТЬ POD'Ы ЧЕРЕЗ kubectl EDIT.
    Изменения в Pod'е не сохраняются. При пересоздании —
    потеряются. Всё — через Deployment.

  14. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Deployment — основной контроллер для stateless-приложений.
      Держит реплики, обновляет, откатывает.
  2.  ReplicaSet — обеспечивает количество Pod'ов. Создаётся
      Deployment'ом автоматически.
  3.  Иерархия: Deployment → ReplicaSet → Pod. При обновлении
      создаётся новый RS, старый оставляется для отката.
  4.  Обязательно: selector.matchLabels == template.metadata.labels.
  5.  Стратегия RollingUpdate — дефолт. Zero-downtime.
      Recreate — для несовместимых версий.
  6.  maxSurge — сколько можно создать сверх replicas.
      maxUnavailable — сколько можно убрать.
  7.  Для zero-downtime: maxSurge: 1, maxUnavailable: 0.
  8.  История ревизий хранится в K8s. Откат — kubectl rollout undo (мгновенный).
  9.  Zero-downtime = Deployment + probes + graceful shutdown + Service.
  10. Probes разделяются: /health для liveness/startup,
      /ready для readiness.
  11. Go-сервис при SIGTERM: ready.Store(false) →
      srv.Shutdown. Общее время < terminationGracePeriodSeconds.
  12. Команды: set image, rollout status, rollout undo,
      rollout restart, scale.
  13. Антипаттерны: несовпадение selector/labels, maxSurge=0
      и maxUnavailable=0, нет probes, нет graceful shutdown,
      latest в образе, ConfigMap без restart.
*/
