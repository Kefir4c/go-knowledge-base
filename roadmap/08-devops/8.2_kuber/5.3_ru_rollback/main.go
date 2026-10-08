package main

/*
  УРОК 5.3: ROLLING UPDATES И ROLLBACK
  Обновление приложения — самая опасная операция в проде. Одно неверное движение — и сервис лёг.
  Rolling update решает это: Pod'ы заменяются постепенно, без downtime. Пока новые поднимаются, старые ещё работают.
  Если новая версия плохая — откат одной командой. Это не «фича Deployment», это его основная задача.
  Всё остальное — обёртка вокруг этого.

  СОДЕРЖАНИЕ:
    1.  Зачем rolling update
    2.  Что происходит при обновлении: полный путь
    3.  ReplicaSet — как Deployment версионирует Pod'ы
    4.  Стратегия RollingUpdate vs Recreate
    5.  maxSurge и maxUnavailable
    6.  Как выбрать maxSurge и maxUnavailable
    7.  kubectl set image — обновление образа
    8.  kubectl rollout status — следим за обновлением
    9.  kubectl rollout history — история ревизий
    10. kubectl rollout undo — откат
    11. revisionHistoryLimit и change-cause
    12. Zero-downtime деплой — полная картина
    13. Probes и graceful shutdown
    14. preStop hook и terminationGracePeriodSeconds
    15. Обновление ConfigMap и Secret
    16. Rollout restart — перезапуск без изменения YAML
	17. стратегии обновления
    18. Связь с Go
    19. Антипаттерны
    20. Финальные выводы

  1. ЗАЧЕМ ROLLING UPDATE
  ПРОБЛЕМА: обновление без стратегии — это downtime. Если убить все старые Pod'ы и создать новые:
    • Сервис недоступен, пока новые не поднимутся.
    • Пользователи получают 502/503.
    • При проблемах с новой версией — полный откат.

  РЕШЕНИЕ: rolling update. Pod'ы заменяются постепенно:
    • Всегда есть живые Pod'ы, которые обрабатывают трафик.
    • Новая версия проверяется на одном Pod'е, потом катится дальше.
    • Если что-то не так — обновление останавливается.
    • Откат — одной командой.

  ЧТО ЭТО ДАЁТ:
    • Zero-downtime деплой.
    • Возможность отката.
    • Проверка новой версии на реальном трафике.
    • Контроль скорости обновления.

  ВАЖНО: rolling update — не автоматическая магия. Он работает только если настроены probes,
  graceful shutdown и правильно выбраны maxSurge/maxUnavailable.
  Без этого будет downtime или обрывы запросов.

  2. ЧТО ПРОИСХОДИТ ПРИ ОБНОВЛЕНИИ: ПОЛНЫЙ ПУТЬ
  Ты запускаешь:
    kubectl set image deployment/app app=my-app:2.0.0 -n demo

  ПОШАГОВО:
    ШАГ 1:kubectl обновляет Deployment. API Server сохраняет новую версию template в etcd.
    ШАГ 2:Deployment Controller видит изменение. Создаёт НОВЫЙ ReplicaSet для версии v2.0.0. Старый RS (v1.0.0) пока с replicas: 3.
    ШАГ 3:Новый RS начинает создавать Pod'ы. С учётом maxSurge создаётся n новый Pod (v2.0.0). Pending → ContainerCreating → Running.
    ШАГ 4:Новый Pod проходит startupProbe и readinessProbe. Только после Ready, Service Controller добавляет его в Endpoints.
    ШАГ 5:Deployment уменьшает старый RS на 1. Один старый Pod получает SIGTERM, убирается из Endpoints, завершается (graceful shutdown).
    ШАГ 6:Повтор шагов 3-5, пока все Pod'ы не обновятся. Старый RS уменьшается до 0. Новый RS достигает replicas: 3.

  ВРЕМЕННАЯ ШКАЛА (3 реплики, maxSurge:1, maxUnavailable:0):
    t=0s    kubectl set image
    t=1s    Новый RS создан
    t=2s    Новый Pod v2 создан (Pending)
    t=5s    Новый Pod v2 Running
    t=10s   Новый Pod v2 Ready
    t=11s   Старый Pod v1 получил SIGTERM
    t=15s   Старый Pod v1 завершился
    t=16s   Новый Pod v2 #2 создан
    ...
    t=60s   Все 3 Pod'а обновлены

  ЧТО ВАЖНО: обновление — это не «удалить все, создать все». Это постепенный процесс с контролем на каждом шаге.

  3. REPLICASET — КАК DEPLOYMENT ВЕРСИОНИРУЕТ POD'Ы
  Deployment не создаёт Pod'ы напрямую. Он создаёт ReplicaSet, который создаёт Pod'ы.

  ИЕРАРХИЯ:
    Deployment (app)
      └─ ReplicaSet (app-abc123)  ← для v1.0.0
           ├─ Pod
           ├─ Pod
           └─ Pod

  ПРИ ОБНОВЛЕНИИ: Deployment создаёт НОВЫЙ ReplicaSet для v2.0.0. Старый RS (v1.0.0) уменьшается до 0,
  но НЕ удаляется. Оба RS сосуществуют во время обновления.

  СХЕМА ВО ВРЕМЯ ОБНОВЛЕНИЯ:
    ┌──────────────────────────────┐
    │       Deployment (app)       │
    │      image: v2.0.0           │
    └───────┬─────────────────┬────┘
            │                 │
            ▼                 ▼
    ┌───────────────┐  ┌───────────────┐
    │ RS v1.0.0     │  │ RS v2.0.0     │
    │ replicas: 2   │  │ replicas: 1   │
    └───────┬───────┘  └───────┬───────┘
            │                  │
        ┌───┴───┐          ┌───┴───┐
        ▼       ▼          ▼       ▼
      Pod v1  Pod v1     Pod v2   (создаётся)
      Ready   Ready      Ready

  ПОСЛЕ ОБНОВЛЕНИЯ: RS v1.0.0 (replicas:0), RS v2.0.0 (replicas:3).

  ЗАЧЕМ СТАРЫЙ RS ОСТАВЛЯТЬ: для отката. kubectl rollout undo мгновенно вернёт старый RS.
  Не надо пересобирать образ, не надо ждать. Просто вернуть replicas с 0 на 3.

  НА ПРАКТИКЕ: ReplicaSet — это версия. Один RS = одна версия template.
  Deployment хранит историю RS. Undo — переключение между RS.

  4. СТРАТЕГИЯ ROLLINGUPDATE VS RECREATE
  ROLLINGUPDATE (дефолт):
    • Постепенная замена Pod'ов.
    • Zero-downtime.
    • Всегда есть живые Pod'ы.
    • Контроль через maxSurge и maxUnavailable.
    • Для stateless-приложений.

    ПРИМЕР:
      spec:
        strategy:
          type: RollingUpdate
          rollingUpdate:
            maxSurge: 1
            maxUnavailable: 0

  RECREATE:
    • Убить все старые, создать все новые.
    • Есть downtime.
    • Нет живых Pod'ов в момент перехода.
    • Для несовместимых версий.

    ПРИМЕР:
      spec:
        strategy:
          type: Recreate

  КОГДА RECREATE:
    • Несовместимые версии (нельзя, чтобы v1 и v2 работали одновременно).
    • Миграции схемы БД, которые ломают старую версию.
    • Простой сервис в dev/staging, где downtime не критичен.

  ЧТО ПРОИСХОДИТ ПРИ RECREATE:
    1. Убиваются все Pod'ы старого ReplicaSet.
    2. Создаётся новый ReplicaSet.
    3. Запускаются все Pod'ы новой версии.

  АНТИПАТТЕРН: использовать Recreate для stateless без причины. RollingUpdate лучше.

  5. MAXSURGE И MAXUNAVAILABLE
  Эти два параметра определяют, как агрессивно катится обновление. Работают только при RollingUpdate.

  MAXSURGE — сколько Pod'ов можно создать СВЕРХ replicas.
    maxSurge: 1     → на 1 Pod больше в моменте.
    maxSurge: 30%   → на 30% больше (для 10 реплик = 3).
    maxSurge: 0     → не создавать лишних. Обновление медленнее.

  MAXUNAVAILABLE — сколько Pod'ов может быть недоступно.
    maxUnavailable: 0    → всегда все живы. Zero-downtime.
    maxUnavailable: 1    → на 1 Pod можно убрать.
    maxUnavailable: 30%  → на 30% можно убрать.

  ВАЖНО: хотя бы один из них должен быть > 0. Иначе обновление не сдвинется с места.

  ПРИМЕР (3 реплики, maxSurge:1, maxUnavailable:0):
    Старт: RS-v1 [3] RS-v2 [0] Всего: 3
    Шаг 1: RS-v1 [3] RS-v2 [1] Всего: 4 (surge)
    Шаг 2: RS-v1 [2] RS-v2 [1] Всего: 3 (убрали старый)
    Шаг 3: RS-v1 [2] RS-v2 [2] Всего: 4
    Шаг 4: RS-v1 [1] RS-v2 [2] Всего: 3
    Шаг 5: RS-v1 [1] RS-v2 [3] Всего: 4
    Шаг 6: RS-v1 [0] RS-v2 [3] Всего: 3

  ПРИМЕР (3 реплики, maxSurge:0, maxUnavailable:1):
    Старт: RS-v1 [3] RS-v2 [0] Всего: 3
    Шаг 1: RS-v1 [2] RS-v2 [0] Всего: 2 (убрали старый)
    Шаг 2: RS-v1 [2] RS-v2 [1] Всего: 3 (создали новый)
    Шаг 3: RS-v1 [1] RS-v2 [1] Всего: 2
    Шаг 4: RS-v1 [1] RS-v2 [2] Всего: 3
    Шаг 5: RS-v1 [0] RS-v2 [2] Всего: 2
    Шаг 6: RS-v1 [0] RS-v2 [3] Всего: 3

  ПРИМЕР (3 реплики, maxSurge:1, maxUnavailable:1):
    Старт: RS-v1 [3] RS-v2 [0] Всего: 3
    Шаг 1: RS-v1 [2] RS-v2 [1] Всего: 3 (одновременно)
    Шаг 2: RS-v1 [1] RS-v2 [2] Всего: 3
    Шаг 3: RS-v1 [0] RS-v2 [3] Всего: 3

  ЧТО ВАЖНО: maxSurge и maxUnavailable применяются к replicas.
  Если replicas=3, maxSurge=1 — можно максимум 4 Pod'а. Если replicas=10, maxSurge=30% — можно максимум 13 Pod'ов.

  6. КАК ВЫБРАТЬ MAXSURGE И MAXUNAVAILABLE

  ZERO-DOWNTIME (рекомендуется для прода):
    maxSurge: 1
    maxUnavailable: 0
    Всегда все Pod'ы живы. Создаётся один лишний, он становится Ready, потом убирается один старый. Медленнее, но безопаснее.
	Необходимо учесть, что если у нас Deployment имеет namespaces c resourceQuota, то в квоте необходимо учитывать
	maxSurge, иначе формально Deployment будет здоров, но новые поды не будут создавать, т.к. в текущей момент времени
	могут быть забиты все pod'ы по лимиту.

  БЫСТРОЕ ОБНОВЛЕНИЕ:
    maxSurge: 25%
    maxUnavailable: 25%
    Создаётся и убирается больше Pod'ов параллельно. Быстрее, но есть момент недоступности. Для dev/staging или некритичных сервисов.

  БЕЗ ЛИШНИХ РЕСУРСОВ:
    maxSurge: 0
    maxUnavailable: 1
    Не создаём сверх replicas. Убираем один, создаём один. Медленнее,
    но не нужно свободных ресурсов на нодах. Для случаев, когда нет запаса CPU/memory.

  НЕ РАБОТАЕТ:
    maxSurge: 0
    maxUnavailable: 0
    Нельзя ни создать, ни убить Pod. Обновление зависнет. Deployment Controller будет ждать, но ничего не произойдёт.

  ЧТО ВЫБРАТЬ ДЛЯ GO-СЕРВИСА:
    maxSurge: 1
    maxUnavailable: 0
    Go-сервис обычно лёгкий. Один лишний Pod — не проблема. Zero-downtime важнее.
    Если ресурсов мало — maxSurge: 0, maxUnavailable: 1, но тогда будет момент недоступности.

  ПРАВИЛО: для прода — maxUnavailable: 0. Для dev — можно пожертвовать доступностью ради скорости.

  7. KUBECTL SET IMAGE — ОБНОВЛЕНИЕ ОБРАЗА
  Основная команда для обновления версии:
    kubectl set image deployment/app app=my-app:2.0.0 -n demo

  РАЗБОР:
    deployment/app — что обновляем.
    app=my-app:2.0.0 — контейнер app, новый образ.
    -n demo — namespace.

  ЧТО ПРОИСХОДИТ:
    1. Deployment обновляется.
    2. Создаётся новый ReplicaSet.
    3. Запускается rolling update.

  ДРУГИЕ СПОСОБЫ ОБНОВИТЬ:
    • kubectl apply -f deployment.yaml (с новым образом).
    • kubectl edit deployment/app (вручную в редакторе).
    • kubectl patch deployment/app -p '{"spec":...}'.
    • Из CI/CD: kubectl set image.

  ЧТО ВАЖНО: set image — императивная команда. Она меняет Deployment в кластере.
  Если Deployment в git с другим образом — при следующем apply вернётся старое. В проде — обновляй через git + apply.

  В CI/CD:
    kubectl set image deployment/app \
      app=my-app:${GIT_SHA} -n demo
    kubectl rollout status deployment/app -n demo

  8. KUBECTL ROLLOUT STATUS — СЛЕДИМ ЗА ОБНОВЛЕНИЕМ
  После set image — следим за прогрессом:
    kubectl rollout status deployment/app -n demo

  ЧТО ПОКАЗЫВАЕТ:
    Waiting for deployment "app" rollout to finish:
      1 out of 3 new replicas have been updated...
    Waiting for deployment "app" rollout to finish:
      2 out of 3 new replicas have been updated...
    Waiting for deployment "app" rollout to finish:
      3 out of 3 new replicas have been updated...
    deployment "app" successfully rolled out

  Команда блокируется, пока обновление не завершится. В CI/CD — это критично:
  следующий шаг не начнётся, пока деплой не закончится.

  ФЛАГ --timeout:
    kubectl rollout status deployment/app --timeout=120s -n demo
    Если обновление не завершилось за 120 секунд — ошибка. В CI/CD — пайплайн упадёт. Можно откатить.

  ЧТО ВИДНО В ПРОЦЕССЕ:
    kubectl get pods -n demo -l app=app -w
    Видно, как появляются новые Pod'ы, старые Terminating.

  ЕСЛИ ЗАВИСЛО:
    Обычно из-за того, что новый Pod не проходит readinessProbe.
    kubectl describe pod <new-pod> -n demo
    kubectl logs <new-pod> -n demo

  9. KUBECTL ROLLOUT HISTORY — ИСТОРИЯ РЕВИЗИЙ
  K8s хранит историю ревизий Deployment. Каждое изменение template создаёт новую ревизию.

  ПОСМОТРЕТЬ ИСТОРИЮ:
    kubectl rollout history deployment/app -n demo

    # REVISION  CHANGE-CAUSE
    # 1         <none>
    # 2         <none>
    # 3         <none>

  CHANGE-CAUSE пустое, потому что мы не указали. Можно указать:
    kubectl annotate deployment/app \
      kubernetes.io/change-cause="update to v2.0.0" -n demo

  Или сразу при set image:
    kubectl set image deployment/app app=my-app:2.0.0 -n demo
    kubectl annotate deployment/app \
      kubernetes.io/change-cause="v2.0.0: fix login bug" -n demo

  ПОСМОТРЕТЬ КОНКРЕТНУЮ РЕВИЗИЮ:
    kubectl rollout history deployment/app --revision=2 -n demo
    Покажет template этой ревизии.

  СКОЛЬКО РЕВИЗИЙ ХРАНИТЬ:
    spec:
      revisionHistoryLimit: 10    # дефолт
    При каждом обновлении старая ревизия остаётся. Когда лимит достигнут — самая старая удаляется.
    Удаление ревизии = удаление старого ReplicaSet.

  ПРАВИЛО: для прода — 5-10. Для dev — можно 0-3. Всегда указывай change-cause.
  Это 5 секунд, но экономит часы при разборе инцидентов.

  10. KUBECTL ROLLOUT UNDO — ОТКАТ

  ОТКАТ НА ПРЕДЫДУЩУЮ ВЕРСИЮ:
    kubectl rollout undo deployment/app -n demo
    Мгновенно возвращает предыдущую ревизию. Создаётся новый ReplicaSet для старой версии.
    Текущий RS уменьшается до 0. Rolling update в обратную сторону.

  ОТКАТ К КОНКРЕТНОЙ РЕВИЗИИ:
    kubectl rollout undo deployment/app --to-revision=2 -n demo
    Возвращает конкретную ревизию (не обязательно предыдущую).

  КАК ЭТО РАБОТАЕТ ПОД КАПОТОМ:
    Старые ReplicaSet'ы остаются в кластере с replicas: 0. Undo просто возвращает нужный RS в active,
    а текущий уменьшает до 0. Ничего не пересобирается, не перекачивается. Мгновенно.

  ЧТО ВАЖНО:
    • Undo работает только если есть история ревизий.
    • Если revisionHistoryLimit: 0 — undo не сработает.
    • Undo — это тоже rolling update (постепенный).

  СЛЕДИТЬ ЗА ОТКАТОМ:
    kubectl rollout status deployment/app -n demo

  ПРИМЕР СЦЕНАРИЯ:
    1. v1.0.0 работает.
    2. Обновили до v2.0.0. Что-то сломалось.
    3. kubectl rollout undo deployment/app.
    4. v1.0.0 возвращается. Rolling update в обратную сторону.
    5. Через минуту всё работает как раньше.

  ВАЖНО: undo не исправляет баги. Он возвращает старую версию. Причину бага надо искать в логах, метриках, трейсах.
  Undo — это «остановить кровотечение», а не лечение.

  11. REVISIONHISTORYLIMIT И CHANGE-CAUSE
  revisionHistoryLimit — сколько старых ReplicaSet'ов хранить. Каждый RS = одна ревизия.
    • 10 — дефолт, разумно для большинства.
    • 5 — если экономишь ресурсы API Server.
    • 0 — отключить историю. Undo не работает.
    • 20+ — если часто катишь и нужна глубокая история.

  ЧТО ВАЖНО: ревизия — это не только образ. Это весь template. Изменение env, resources, probes — всё создаёт ревизию.
  АНТИПАТТЕРН: revisionHistoryLimit: 0 в проде. Тогда undo не работает. При проблемах — только новый деплой с исправлением.

  change-cause — по умолчанию пустое, непонятно, что менялось. Заполнить:
    kubectl annotate deployment/app \
      kubernetes.io/change-cause="v2.0.0: fix login bug" -n demo

  Теперь видно:
    # REVISION  CHANGE-CAUSE
    # 1         v1.0.0: initial
    # 2         v1.1.0: add metrics
    # 3         v2.0.0: fix login bug

  В CI/CD:
    CHANGE_CAUSE="${GIT_SHA}: ${COMMIT_MESSAGE}"
    kubectl annotate deployment/app \
      kubernetes.io/change-cause="${CHANGE_CAUSE}" -n demo

  12. ZERO-DOWNTIME ДЕПЛОЙ — ПОЛНАЯ КАРТИНА
  Zero-downtime — это совместная работа:
    • Deployment (rolling update).
    • ReplicaSet (правильное количество).
    • Probes (готовность Pod'а).
    • Service (endpoints обновляются).
    • Graceful shutdown (в Go-коде).
    • preStop hook (опционально).

  ПОШАГОВО ПРИ ОБНОВЛЕНИИ:
    1. Ты запускаешь kubectl set image.
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
    • Probes не настроены → трафик идёт до готовности, клиенты получают 500.
    • Readiness не проходит при shutdown → Service не успевает убрать Pod → часть запросов уходит в мёртвый.
    • Нет graceful shutdown → активные запросы обрываются.
    • terminationGracePeriodSeconds слишком мал → SIGKILL на середине shutdown.

  13. PROBES И GRACEFUL SHUTDOWN
  Без probes rolling update не работает как надо.

  LIVENESS PROBE: если падает — Pod перезапускается. Не должен проверять БД (иначе каскадные рестарты).

  READINESS PROBE: если падает — Pod убирается из Service Endpoints. Должен проверять зависимости (БД, Redis), но быстро.
  При shutdown — сразу false, чтобы убраться из Endpoints.

  STARTUP PROBE: для медленно стартующих приложений. Пока не прошёл — liveness и readiness не проверяются.
  Даёт время на инициализацию.

  ПРИМЕР КОНФИГА:
    livenessProbe:
      httpGet:
        path: /health
        port: 8080
      initialDelaySeconds: 10
      periodSeconds: 10
      failureThreshold: 3

    readinessProbe:
      httpGet:
        path: /ready
        port: 8080
      initialDelaySeconds: 3
      periodSeconds: 5
      failureThreshold: 2

    startupProbe:
      httpGet:
        path: /health
        port: 8080
      failureThreshold: 30
      periodSeconds: 3

  GRACEFUL SHUTDOWN В GO:
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

  ЧТО ЭТО ДАЁТ:
    • ready=false → readinessProbe падает → Pod убирается из Endpoints.
    • srv.Shutdown ждёт завершения активных запросов.
    • Через 20 секунд — принудительное завершение.

  14. PRESTOP HOOK И TERMINATIONGRACEPERIODSECONDS
  preStop — команда, которая выполняется ПЕРЕД отправкой SIGTERM основному процессу.

  ЗАЧЕМ:
    • Дать kube-proxy время обновить iptables.
    • Дать Service убрать Pod из Endpoints.
    • Дать load balancer'у перестать слать трафик.

  ПРОБЛЕМА, КОТОРУЮ РЕШАЕТ:
    Когда Pod получает SIGTERM, kubelet одновременно отправляет SIGTERM и начинает удалять Pod из Endpoints.
    Но обновление Endpoints не мгновенное. Есть окно, когда Pod уже получил SIGTERM, но трафик ещё идёт.
    Если Go-сервис сразу закрывает listener — запросы падают.

  РЕШЕНИЕ: preStop sleep 5. Pod получает preStop. Kubelet ждёт 5 секунд. За это время Endpoints обновляются.
  Потом SIGTERM. Go-сервис делает graceful shutdown.

  ПРИМЕР:
    lifecycle:
      preStop:
        exec:
          command: ["/bin/sh", "-c", "sleep 5"]
  terminationGracePeriodSeconds — сколько kubelet ждёт завершения Pod'а перед SIGKILL. Дефолт 30 секунд.

  ЧТО ПРОИСХОДИТ:
    1. Pod удаляется.
    2. Выполняется preStop hook.
    3. Отправляется SIGTERM.
    4. Kubelet ждёт terminationGracePeriodSeconds.
    5. Если Pod не завершился — SIGKILL.

  ЧТО ВАЖНО:
    • Время preStop + время shutdown < terminationGracePeriodSeconds.
    • Если preStop спит 10 секунд, а grace period 5 — preStop не успеет, SIGKILL.
    • Если Go-сервис делает shutdown 40 секунд, а grace period 30 — SIGKILL на середине.
  ПРАВИЛО: grace period = preStop + max(shutdown time) + запас. Для Go-сервисов обычно 30-60 секунд.

  15. ОБНОВЛЕНИЕ CONFIGMAP И SECRET
  ВАЖНО: Deployment не следит за ConfigMap и Secret.

  ЧТО ПРОИСХОДИТ:
    Ты обновляешь ConfigMap. Pod'ы не перезапускаются. ENV не обновляется (env читается при старте).
    Volume обновляется (kubelet синкает каждые 30-60 секунд). Но приложение может не перечитывать файлы.

  РЕШЕНИЕ: rollout restart.
    kubectl rollout restart deployment/app -n demo

  ЧТО ЭТО ДЕЛАЕТ: создаёт новый ReplicaSet с новым template. Template тот же, но добавляется аннотация с timestamp.
  Запускается rolling update. Pod'ы перезапускаются, читают новый ConfigMap.

  АЛЬТЕРНАТИВА: Reloader. Инструмент, который следит за ConfigMap/Secret и
  автоматически делает rollout restart при изменении. Не встроен в K8s, ставится отдельно.

  ВАЖНО:
    • Для ENV — только rollout restart.
    • Для VOLUME — файлы обновляются, но приложение должно перечитывать. Если не перечитывает — тоже rollout restart.
    • subPath mount — не обновляется вообще.

  16. ROLLOUT RESTART — ПЕРЕЗАПУСК БЕЗ ИЗМЕНЕНИЯ YAML

  Команда:
    kubectl rollout restart deployment/app -n demo

  ЧТО ДЕЛАЕТ: добавляет аннотацию kubectl.kubernetes.io/restartedAt с текущим timestamp в template.
  Это создаёт новую ревизию. Запускается rolling update. Pod'ы пересоздаются.

  ЗАЧЕМ:
    • Обновил ConfigMap/Secret — надо перечитать.
    • Подозрение на memory leak — перезапустить.
    • Просто хочу свежие Pod'ы.

  ЧТО ВАЖНО: это не «убить все Pod'ы». Это rolling update. Zero-downtime (если настроено). Старые Pod'ы уходят постепенно.

  В CI/CD: если ConfigMap в том же helm-релизе — rollout происходит автоматически (helm делает restart).
  Если ConfigMap отдельно — нужен явный restart.

17. СТРАТЕГИИ ОБНОВЛЕНИЯ — ПОЛНЫЙ ОБЗОР
  K8s Deployment нативно поддерживает только две стратегии: RollingUpdate и Recreate.
  Всё остальное — Blue-Green, Canary, A/B, Shadow — это паттерны поверх K8s.
  Они реализуются через Services, Ingress, Argo Rollouts, Flagger или самописные контроллеры.

  17.1. ROLLINGUPDATE (нативная)
    Что это: постепенная замена Pod'ов с контролем через maxSurge/maxUnavailable.
    Плюсы: zero-downtime, откат, встроено в Deployment.
    Минусы: старая и новая версии работают одновременно (API должен быть совместим).
    Когда: 90% случаев. Stateless-сервисы. Дефолт для прода.

  17.2. RECREATE (нативная)
    Что это: убить все старые Pod'ы, создать все новые.
    Плюсы: нет одновременной работы версий. Проще, чем RollingUpdate.
    Минусы: downtime на время перехода.
    Когда: несовместимые версии, миграции схемы БД, ломающие старую версию, dev/staging.

  17.3. BLUE-GREEN (паттерн)
    Что это: две среды. Blue — текущая версия, Green — новая. Трафик переключается с Blue на Green разом.
    Как делается: два Deployment (app-blue, app-green). Service селектит по label version: blue или green. Переключение — смена label в Service.
    Плюсы: мгновенный откат (просто вернуть label). Нет одновременной работы версий. Можно протестировать Green до переключения.
    Минусы: двойные ресурсы (два Deployment работают одновременно). Все пользователи переключаются разом.
    Когда: критичные сервисы, где нужен мгновенный откат и есть запас ресурсов.

    ПРИМЕР ПЕРЕКЛЮЧЕНИЯ:
      # Service сейчас смотрит на blue
      selector:
        app: app
        version: blue

      # Переключение на green — меняем selector
      kubectl patch service app -n demo \
        -p '{"spec":{"selector":{"version":"green"}}}'

      # Откат — вернуть blue
      kubectl patch service app -n demo \
        -p '{"spec":{"selector":{"version":"blue"}}}'

  17.4. CANARY (паттерн)
    Что это: новая версия получает малую долю трафика. Постепенно доля увеличивается: 5% → 25% → 50% → 100%.
    Как делается: два Deployment (app-stable, app-canary). Service или Ingress раскидывает трафик по весам.
    Argo Rollouts / Flagger автоматизируют процесс с проверкой метрик.
    Плюсы: реальный трафик на новой версии. Ограниченный радиус поражения при багах. Автоматический откат по метрикам.
    Минусы: сложнее настроить. Нужен контроль трафика на уровне Service/Ingress. Нужны метрики для автоматизации.
    Когда: важные релизы, где нельзя рисковать всеми пользователями сразу.

    ПРИМЕР С NGINX INGRESS:
      annotations:
        nginx.ingress.kubernetes.io/canary: "true"
        nginx.ingress.kubernetes.io/canary-weight: "10"

      10% трафика идёт на canary-версию, 90% — на stable.

  17.5. A/B TESTING (паттерн)
    Что это: две версии для разных групп пользователей. Не для деплоя, а для эксперимента. Цель — сравнить метрики: конверсию, retention, клики.
    Как делается: Ingress роутит по заголовкам, cookies, geo. Пользователь попадает в одну из версий и остаётся в ней.
    Плюсы: сравнение метрик между версиями на реальных пользователях.
    Минусы: сложность. Нужна аналитика и разделение пользователей.
    Когда: продуктовые эксперименты, тестирование UX-изменений.

  17.6. SHADOW (паттерн)
    Что это: новая версия получает копию реального трафика, но ответы игнорируются. Пользователь видит ответ старой версии.
    Как делается: прокси дублирует запросы. Обычно через Istio, Envoy, Linkerd.
    Плюсы: тест на реальном трафике без влияния на пользователей. Ловит баги до релиза.
    Минусы: сложно. Двойная нагрузка на БД и зависимости. Нужен service mesh.
    Когда: тестирование критичных изменений, миграции, новые алгоритмы.

  СРАВНЕНИЕ:
    RollingUpdate — K8s native, zero-downtime, 90% случаев.
    Recreate — K8s native, downtime, для несовместимых версий.
    Blue-Green — паттерн, мгновенный откат, двойные ресурсы.
    Canary — паттерн, ограниченный радиус, сложнее, метрики.
    A/B — паттерн, для экспериментов, не для деплоя.
    Shadow — паттерн, для тестов, двойная нагрузка.

  ИНСТРУМЕНТЫ:
    Argo Rollouts — CRD для Canary и Blue-Green. Автоматизация с проверкой метрик.
    Flagger — автоматизация Canary с метриками из Prometheus.
    Istio / Linkerd — service mesh, контроль трафика, Shadow.
    Nginx Ingress — canary через annotations, простые случаи.

  ПРАВИЛО ДЛЯ GO-РАЗРАБОТЧИКА:
    По умолчанию — RollingUpdate. Это встроено, просто, работает.
    Blue-Green — если нужен мгновенный откат и есть ресурсы.
    Canary — если релиз рискованный и нужен контроль.
    A/B и Shadow — редко, для специфических задач. Обычно настраивает платформенная команда.

  18. СВЯЗЬ С GO

  Что нужно от Go-разработчика для rolling update:
  1. STATELESS. Никаких сессий в памяти. Redis/JWT. Никаких локальных файлов. Всё в volume или S3. Любой Pod обработает любой запрос.
  2. GRACEFUL SHUTDOWN. Ловить SIGTERM. ready.Store(false) → убраться из Endpoints. srv.Shutdown(ctx) →
     завершить активные запросы. Закрыть соединения с БД. Выйти с кодом 0.
  3. PROBES. /health — liveness и startup. Не проверяет БД. /ready — readiness. Проверяет БД, Redis.
  4. RESOURCES. requests.cpu — для планирования. limits.memory — для OOMKilled. GOMEMLIMIT — на 10-20% меньше limits.memory.
  5. TERMINATIONGRACEPERIODSECONDS. Больше, чем preStop + shutdown time.

  ПРИМЕР ПОЛНОГО MAIN.GO:
    package main

    import (
        "context"
        "errors"
        "log/slog"
        "net/http"
        "os"
        "os/signal"
        "sync/atomic"
        "syscall"
        "time"
    )

    var ready atomic.Bool

    func main() {
        ctx, cancel := signal.NotifyContext(
            context.Background(),
            syscall.SIGTERM,
            syscall.SIGINT,
        )
        defer cancel()

        mux := http.NewServeMux()
        mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
        })
        mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
            if !ready.Load() {
                w.WriteHeader(http.StatusServiceUnavailable)
                return
            }
            w.WriteHeader(http.StatusOK)
        })

        srv := &http.Server{
            Addr:    ":8080",
            Handler: mux,
        }

        go func() {
            slog.Info("starting server")
            if err := srv.ListenAndServe(); err != nil &&
                !errors.Is(err, http.ErrServerClosed) {
                slog.Error("server error", "err", err)
                os.Exit(1)
            }
        }()

        ready.Store(true)
        slog.Info("ready")

        <-ctx.Done()
        slog.Info("shutdown signal received")
        ready.Store(false)

        shutdownCtx, shutdownCancel := context.WithTimeout(
            context.Background(), 20*time.Second)
        defer shutdownCancel()

        if err := srv.Shutdown(shutdownCtx); err != nil {
            slog.Error("shutdown error", "err", err)
        }
        slog.Info("shutdown complete")
    }

  ЧТО ЗДЕСЬ:
    • /health — всегда 200. Не проверяет БД.
    • /ready — 503 пока не готов, 200 после.
    • signal.NotifyContext — ловит SIGTERM.
    • ready.Store(false) — убирает из Endpoints.
    • srv.Shutdown — завершает активные запросы.
    • 20 секунд — таймаут shutdown.

  19. АНТИПАТТЕРНЫ
  19.1. НЕТ PROBES. Трафик идёт в Pod'ы, которые ещё не готовы. Клиенты получают 500 при обновлении.
  19.2. LIVENESS ПРОВЕРЯЕТ БД. Если БД упала — kubelet начнёт рестартить Pod'ы. Каскадные рестарты.
  19.3. НЕТ GRACEFUL SHUTDOWN. Активные запросы обрываются при удалении Pod'а. Пользователи получают ошибки.
  19.4. maxSurge: 0 И maxUnavailable: 0. Обновление не сдвинется с места.
  19.5. maxUnavailable: 1 БЕЗ ПРИЧИНЫ. Есть момент недоступности. Для прода — maxUnavailable: 0.
  19.6. RECREATE ДЛЯ STATELESS. Downtime там, где можно было обойтись.
  19.7. revisionHistoryLimit: 0. Undo не работает.
  19.8. LATEST В ОБРАЗЕ. Каждый pull может принести новую версию. Rolling update не сработает как ожидалось. Фиксируй тег или digest.
  19.9. ИЗМЕНИЛ CONFIGMAP — НЕ ПЕРЕЗАПУСТИЛ POD'Ы. Pod'ы не перечитают env. Нужен rollout restart.
  19.10. terminationGracePeriodSeconds: 5 ДЛЯ МЕДЛЕННЫХ СЕРВИСОВ. Go-сервис не успевает завершиться. SIGKILL.
  19.11. preStop sleep 60 ПРИ grace period 30. preStop не успеет, SIGKILL.
  19.12. ОБНОВЛЕНИЕ БЕЗ rollout status. Запустил и забыл. Ошибки видишь только от пользователей.
  19.13. UNDO БЕЗ ПОНИМАНИЯ ПРИЧИНЫ. Откатился, но баг остался. Надо искать причину, а не только откатывать.
  19.14. ROLLOUT RESTART ВМЕСТО ROLLING UPDATE. Restart не меняет версию, только пересоздаёт Pod'ы. Для обновления — set image или apply.

  20. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Rolling update — постепенная замена Pod'ов без downtime.
  2.  Deployment создаёт новый ReplicaSet для новой версии, старый оставляет для отката.
  3.  Стратегии: RollingUpdate (дефолт) и Recreate (с downtime).
  4.  maxSurge — сколько Pod'ов сверх replicas. maxUnavailable — сколько Pod'ов может быть недоступно.
  5.  Для zero-downtime: maxSurge: 1, maxUnavailable: 0.
  6.  Нельзя maxSurge: 0 и maxUnavailable: 0 одновременно.
  7.  kubectl set image — обновление образа.
  8.  kubectl rollout status — следить за прогрессом.
  9.  kubectl rollout history — история ревизий.
  10. kubectl rollout undo — откат (мгновенный).
  11. revisionHistoryLimit — сколько ревизий хранить.
  12. change-cause — аннотация для читаемой истории.
  13. Zero-downtime = rolling update + probes + graceful shutdown + Service.
  14. Probes: /health для liveness/startup, /ready для readiness.
  15. Graceful shutdown: ready.Store(false) → srv.Shutdown.
  16. preStop hook — дать Service обновить Endpoints.
  17. terminationGracePeriodSeconds > preStop + shutdown time.
  18. ConfigMap/Secret — только rollout restart.
  19. Go-сервис: stateless, graceful shutdown, probes, реалистичные resources.
*/
