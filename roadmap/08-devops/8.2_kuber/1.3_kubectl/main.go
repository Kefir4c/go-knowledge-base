package main

/*
  УРОК 1.3: KUBECTL — БАЗОВЫЕ КОМАНДЫ
  kubectl — твой главный инструмент для работы с K8s. Через него
  ты создаёшь объекты, смотришь состояние, отлаживаешь, масштабируешь.
  Хорошая новость: 90% работы — 15 команд. Освоив их, ты покрываешь
  всё, что нужно в ежедневной работе.

  СОДЕРЖАНИЕ:
    1.  Синтаксис и философия kubectl
    2.  Контексты и namespace — где ты работаешь
    3.  get — основной просмотр
    4.  describe — детальная диагностика
    5.  logs — логи
    6.  exec — зайти внутрь
    7.  apply и delete — создание и удаление
    8.  rollout — обновления и откат
    9.  scale — масштабирование
    10. port-forward — локальный доступ
    11. Полезные флаги и форматы вывода
    12. Events и диагностика проблем
    13. Связь с Go
    14. Шпаргалка
    15. Финальные выводы

  1. СИНТАКСИС И ФИЛОСОФИЯ KUBECTL

  ОБЩИЙ СИНТАКСИС:
    kubectl <команда> <ресурс> [имя] [флаги]

  ПРИМЕРЫ:
    kubectl get pods
    kubectl describe pod app-abc123
    kubectl logs app-abc123
    kubectl apply -f deployment.yaml
    kubectl delete deployment app

  РЕСУРСЫ В КОРОТКОЙ И ДЛИННОЙ ФОРМЕ:
    pod         po
    deployment  deploy
    service     svc
    namespace   ns
    configmap   cm
    secret
    statefulset sts
    daemonset   ds
    replicaset  rs
    job
    cronjob     cj
    ingress     ing
    node        no
    persistentvolumeclaim pvc

    Короткие формы экономят время:
      kubectl get po       # вместо pods
      kubectl get deploy   # вместо deployments
      kubectl get svc      # вместо services

  ЧТО ТАКОЕ «РЕСУРС»:
    Любой объект K8s. Pod, Service, Deployment, ConfigMap —
    всё это ресурсы. kubectl работает с ними единообразно.

  ОСНОВНЫЕ КОМАНДЫ (запомнить):
    get         — список или один объект.
    describe    — детальная информация + события.
    logs        — логи контейнера.
    exec        — выполнить команду в Pod'е.
    apply       — создать или обновить из YAML.
    delete      — удалить.
    rollout     — управление обновлениями.
    scale       — изменить число реплик.
    port-forward — проброс порта.
    config      — работа с конфигами (контексты).

  2. КОНТЕКСТЫ И NAMESPACE — ГДЕ ТЫ РАБОТАЕШЬ
  KUBECONFIG — файл конфигурации. Обычно ~/.kube/config.

  Контекст = кластер + пользователь + namespace.

  КОМАНДЫ ДЛЯ КОНТЕКСТОВ:
    # Посмотреть текущий контекст.
    kubectl config current-context

    # Список всех контекстов.
    kubectl config get-contexts

    # Переключиться на другой контекст.
    kubectl config use-context production

    # Текущий namespace по умолчанию.
    kubectl config view --minify --output 'jsonpath={..namespace}'

    # Установить namespace по умолчанию в текущем контексте.
    kubectl config set-context --current --namespace=demo

  УДОБНЫЕ УТИЛИТЫ:

    KUBECTX — быстрое переключение контекстов.

      kubectx                       # список
      kubectx production            # переключиться

    KUBENS — быстрое переключение namespace.

      kubens                        # список
      kubens demo                   # переключиться

    Устанавливать через brew/apt/github releases. Очень удобно,
    когда работаешь с несколькими кластерами.

  NAMESPACE В КОМАНДАХ:
    Можно указывать флагом -n:

      kubectl get pods -n production
      kubectl get pods --namespace=production

    Или использовать текущий (через kubens или config set-context).

  ПРАВИЛО:
    Всегда явно указывай namespace. Иначе работаешь в default,
    а там обычно ничего нет — и ты непонятно почему «не видишь
    свои Pod'ы».

  3. GET — ОСНОВНОЙ ПРОСМОТР
  Самый частый вызов. Показывает список объектов.

  БАЗОВЫЕ ПРИМЕРЫ:
    kubectl get pods
    kubectl get pods -n production
    kubectl get deployments
    kubectl get services
    kubectl get configmaps
    kubectl get secrets

  ВСЁ СРАЗУ:
    kubectl get all -n production

    Покажет Pod'ы, Service'ы, Deployment'ы, ReplicaSet'ы.
    Не покажет ConfigMap, Secret, Ingress — их надо отдельно.

  ОДИН ОБЪЕКТ:
    kubectl get pod app-abc123 -n production
    kubectl get deployment app -n production

  РАСШИРЕННЫЙ ВЫВОД:
    # С IP и нодой.
    kubectl get pods -o wide

    # Пример вывода:
    # NAME       READY   STATUS    RESTARTS   AGE   IP           NODE
    # app-abc    1/1     Running   0          5m    10.244.1.5   node-1
    # app-def    1/1     Running   0          5m    10.244.2.7   node-2

    Колонки IP и NODE — самые полезные. Сразу видно,
    куда Scheduler раскидал Pod'ы.

  ФИЛЬТР ПО LABELS:
    kubectl get pods -l app=app
    kubectl get pods -l 'app in (app,api)'

  ФИЛЬТР ПО STATUS:
    kubectl get pods --field-selector status.phase=Running
    kubectl get pods --field-selector status.phase=Pending

  СЛЕЖЕНИЕ В РЕАЛЬНОМ ВРЕМЕНИ:
    kubectl get pods -w

    Полезно при деплое: видишь, как Pod'ы появляются.

  СОРТИРОВКА:
    kubectl get pods --sort-by=.metadata.creationTimestamp

  ЧТО ЗНАЧАТ КОЛОНКИ:
    NAME       — имя Pod'а.
    READY      — сколько контейнеров готовы (1/1, 2/2).
    STATUS     — Running, Pending, CrashLoopBackOff, Completed.
    RESTARTS   — сколько раз перезапускался. Рост — плохой знак.
    AGE        — сколько живёт.

  ЧТО СМОТРЕТЬ ПЕРВЫМ ДЕЛОМ:
    1. STATUS = Running? Если Pending — Scheduler не может
       разместить.
    2. READY = 1/1? Если 0/1 — readinessProbe не проходит.
    3. RESTARTS растёт? Что-то падает.

  4. DESCRIBE — ДЕТАЛЬНАЯ ДИАГНОСТИКА
  describe — главный инструмент отладки. Показывает всё о
  объекте + события внизу.

  БАЗОВЫЕ ПРИМЕРЫ:
    kubectl describe pod app-abc123
    kubectl describe deployment app
    kubectl describe service app
    kubectl describe node node-1

  ЧТО ВЫВОДИТ:
    • Метаданные (labels, annotations).
    • Spec (что должно быть).
    • Status (что есть).
    • Events (что происходило).

  СЕКЦИЯ EVENTS — САМОЕ ВАЖНОЕ:

    Events:
      Type     Reason            From       Message
      ----     ------            ----       -------
      Warning  FailedScheduling  scheduler  0/3 nodes are available:
                                             3 Insufficient cpu
      Normal   Scheduled         scheduler  Successfully assigned
      Normal   Pulling           kubelet    Pulling image "my-app:1.0"
      Normal   Pulled            kubelet    Successfully pulled image
      Normal   Created           kubelet    Created container app
      Normal   Started           kubelet    Started container app

  ЧТО ЗДЕСЬ ЧИТАТЬ:
    • FailedScheduling — Pod не может разместиться. Причина:
      не хватает ресурсов, taints, nodeSelector.
    • ImagePullBackOff — образ не тянется. Опечатка в имени,
      приватный registry без secret.
    • CrashLoopBackOff — контейнер падает и перезапускается.
    • OOMKilled — превышен limits.memory.
    • Unhealthy — probes не проходят.

  ЧТО ЕЩЁ ПОЛЕЗНО:
    В describe pod видно:
      • Какой node выбрал Scheduler.
      • Какие probes настроены.
      • Какие env variables.
      • Какие volumes примонтированы.
      • Какие limits/requests.

    В describe node видно:
      • Сколько ресурсов всего.
      • Сколько занято.
      • Какие Pod'ы работают.
      • Taints и conditions.

  СИНТАКСИС С LABELS:
    kubectl describe pods -l app=app
    kubectl describe pods -l app=app -n production

  5. LOGS — ЛОГИ
  Логи — то, что приложение пишет в stdout/stderr.

  БАЗОВЫЕ ПРИМЕРЫ:
    kubectl logs app-abc123
    kubectl logs app-abc123 -n production
    kubectl logs deployment/app              # лог одного из Pod'ов
    kubectl logs -l app=app                  # логи всех Pod'ов

  ФЛАГИ:
    -f, --follow              — следить в реальном времени.
    --tail=100                — последние 100 строк.
    --since=1h                — за последний час.
    --timestamps              — с таймстампами.
    -c <container>            — конкретный контейнер (если их несколько).
    --previous                — логи ПРЕДЫДУЩЕГО контейнера (если упал).

  ПРИМЕРЫ:
    # Последние 100 строк.
    kubectl logs --tail=100 app-abc123

    # Следить в реальном времени.
    kubectl logs -f app-abc123

    # Логи упавшего контейнера.
    kubectl logs --previous app-abc123

    # Логи конкретного контейнера в Pod'е с несколькими.
    kubectl logs app-abc123 -c sidecar

  ЧТО ВАЖНО:
    Если Pod упал и был перезапущен — логи старого контейнера
    уже недоступны через обычный logs. Только через --previous.

    Если контейнер пишет в файл внутри Pod'а, а не в stdout —
    kubectl logs не покажет. Нужен sidecar или exec.

  ДЛЯ МНОГИХ POD'ОВ:
    kubectl logs -l app=app --all-containers --prefix

    Выведет логи всех Pod'ов с префиксами имён.

  6. EXEC — ЗАЙТИ ВНУТРЬ
  exec запускает команду внутри работающего контейнера.

  ОСНОВНОЙ СИНТАКСИС:
    kubectl exec -it <pod> -- <команда>
    kubectl exec -it <pod> -n <ns> -- <команда>

  ПРИМЕРЫ:
    # Интерактивный shell.
    kubectl exec -it app-abc123 -- sh

    # Если есть bash.
    kubectl exec -it app-abc123 -- bash

    # Одна команда.
    kubectl exec app-abc123 -- ls /app
    kubectl exec app-abc123 -- cat /etc/config/app.yaml

    # Конкретный контейнер в Pod'е с несколькими.
    kubectl exec -it app-abc123 -c app -- sh

    # Через deployment (выберет один из Pod'ов).
    kubectl exec -it deployment/app -- sh

  ЧТО ПОЛЕЗНО ДЕЛАТЬ ВНУТРИ:
    # Проверить переменные окружения.
    env | grep DB_

    # Проверить DNS.
    nslookup postgres.demo.svc.cluster.local
    ping postgres

    # Проверить доступ к другому сервису.
    wget -q -O - http://app.demo.svc.cluster.local/health

    # Посмотреть сетевые интерфейсы.
    ip addr

    # Посмотреть процессы.
    ps aux

  ВАЖНО ПРО DISTROLESS:
    В distroless нет sh, bash, ls. exec не сработает. Решения:

      • Ephemeral containers: kubectl debug.
      • Debug-образ с тем же бинарником + shell.
      • nsenter на ноде (сложно).

    Команда debug:
      kubectl debug -it app-abc123 --image=alpine --target=app

    Поднимает временный контейнер alpine в том же Pod'е,
    делит namespace с app.

  7. APPLY И DELETE — СОЗДАНИЕ И УДАЛЕНИЕ
  APPLY — основной способ создавать объекты. Декларативный:
  «сделай так, чтобы было по этому YAML».

  БАЗОВЫЕ ПРИМЕРЫ:
    kubectl apply -f deployment.yaml
    kubectl apply -f ./k8s/                       # вся папка
    kubectl apply -f https://example.com/app.yaml # из URL
    kubectl apply -k ./overlays/prod/             # kustomize

  ЧТО ДЕЛАЕТ APPLY:
    1. Читает YAML.
    2. Смотрит, есть ли такой объект в кластере.
    3. Если нет — создаёт.
    4. Если есть — обновляет (патчит то, что отличается).

  ПРЕИМУЩЕСТВО ПЕРЕД CREATE:
    apply идемпотентен. Можно запускать много раз — результат
    один. create упадёт, если объект уже есть.

  DELETE — удаление объектов.

    kubectl delete -f deployment.yaml
    kubectl delete pod app-abc123
    kubectl delete deployment app
    kubectl delete pods -l app=app              # все с label
    kubectl delete pods --all -n demo           # все в namespace
    kubectl delete namespace demo               # всё содержимое

  ОСТОРОЖНО:
    kubectl delete namespace demo удалит ВСЁ в namespace.
    Не делай это в проде без понимания.

  ДРУГИЕ КОМАНДЫ:
    kubectl edit deployment app -n demo
      Открывает YAML в редакторе. Сохранил — применилось.

    kubectl patch deployment app -n demo \
      -p '{"spec":{"replicas":5}}'
      Патч конкретного поля.

    kubectl create deployment app --image=my-app:1.0
      Императивное создание (без YAML).

  ПРАВИЛО: используй apply + YAML. Всё остальное — редко.

  8. ROLLOUT — ОБНОВЛЕНИЯ И ОТКАТ
  rollout — управление версиями Deployment.

  ОСНОВНЫЕ КОМАНДЫ:
    # Запустить обновление образа.
    kubectl set image deployment/app app=my-app:2.0.0 -n demo

    # Посмотреть статус обновления.
    kubectl rollout status deployment/app -n demo

    # История ревизий.
    kubectl rollout history deployment/app -n demo

    # История конкретной ревизии.
    kubectl rollout history deployment/app --revision=2 -n demo

    # Откат на предыдущую версию.
    kubectl rollout undo deployment/app -n demo

    # Откат на конкретную ревизию.
    kubectl rollout undo deployment/app --to-revision=2 -n demo

    # Приостановить обновление (можно менять параметры).
    kubectl rollout pause deployment/app -n demo

    # Продолжить.
    kubectl rollout resume deployment/app -n demo

    # Перезапустить все Pod'ы (без изменения YAML).
    kubectl rollout restart deployment/app -n demo

  КОГДА ИСПОЛЬЗУЕТСЯ ПЕРЕЗАПУСК:
    Изменил ConfigMap или Secret — Pod'ы не перечитают
    автоматически (если env). `rollout restart` пересоздаёт
    Pod'ы, они читают новые значения.

  КАК РАБОТАЕТ ROLLING UPDATE:
    Deployment создаёт НОВЫЙ ReplicaSet.
    Постепенно увеличивает его реплики.
    Постепенно уменьшает старый ReplicaSet.
    Контроль через maxSurge и maxUnavailable.

    Если новая версия падает — rollout status покажет
    ошибку. Откат — kubectl rollout undo.

  ЧТО ВАЖНО НА СОБЕСЕ:
    Знать: set image, rollout status, rollout undo.
    Понимать: при обновлении есть новая и старая версии
    одновременно. API должен быть совместим.

  9. SCALE — МАСШТАБИРОВАНИЕ
  scale — изменить количество реплик.

  ПРИМЕРЫ:
    # Вручную.
    kubectl scale deployment/app --replicas=5 -n demo

    # Через HPA — автоматически (см. отдельный урок).
    # HPA сам меняет replicas.

  ПРОВЕРКА:
    kubectl get deployment app -n demo
    # NAME   READY   UP-TO-DATE   AVAILABLE   AGE
    # app    5/5     5            5           5m

    kubectl get pods -n demo -l app=app
    # 5 Pod'ов.

  SCALE ДО 0:
    kubectl scale deployment/app --replicas=0 -n demo

    Все Pod'ы удалятся. Deployment остаётся. Можно вернуть:

    kubectl scale deployment/app --replicas=3 -n demo

  10. PORT-FORWARD — ЛОКАЛЬНЫЙ ДОСТУП
  port-forward пробрасывает порт из Pod'а или Service на
  localhost. Для локальной отладки.

  СИНТАКСИС:
    kubectl port-forward <pod|svc> <local-port>:<remote-port>

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

  ОГРАНИЧЕНИЯ:
    port-forward работает через kubectl, значит через API Server.
    Не подходит для высоких нагрузок. Только для отладки.

  11. ПОЛЕЗНЫЕ ФЛАГИ И ФОРМАТЫ ВЫВОДА

  ФОРМАТЫ ВЫВОДА:
    -o wide       — расширенный (IP, NODE).
    -o yaml       — полный YAML объекта.
    -o json       — JSON.
    -o name       — только имена.
    -o jsonpath   — извлечь конкретное поле.
    -o custom-columns — свои колонки.

  ПРИМЕРЫ:
    # Полный YAML запущенного объекта.
    kubectl get deployment app -o yaml

    # Только имена Pod'ов.
    kubectl get pods -o name
    # pod/app-abc123
    # pod/app-def456

    # IP конкретного Pod'а.
    kubectl get pod app-abc123 -o jsonpath='{.status.podIP}'

    # Свои колонки.
    kubectl get pods -o custom-columns=\
    NAME:.metadata.name,\
    IP:.status.podIP,\
    NODE:.spec.nodeName

  ФИЛЬТРЫ И ЯРЛЫКИ:
    -l <selector>           — фильтр по labels.
    --field-selector        — фильтр по полям.
    -n <namespace>          — namespace.
    -A, --all-namespaces    — по всем namespace.

  ПРИМЕРЫ:
    # Все Pod'ы во всех namespace.
    kubectl get pods -A

    # Все Pod'ы с label app=app.
    kubectl get pods -l app=app

    # Все Running Pod'ы.
    kubectl get pods --field-selector status.phase=Running

  ШАБЛОНИРОВАНИЕ:
    --dry-run=client        — не отправлять в API Server.
    --dry-run=server        — отправить, но не сохранять.
    -o yaml > out.yaml      — сохранить манифест.

  КЛАССИЧЕСКИЙ ПРИЁМ:
    # Сгенерировать YAML шаблон Deployment без применения.
    kubectl create deployment app --image=my-app:1.0 \
      --dry-run=client -o yaml > deployment.yaml

    # Отредактировать и применить.
    kubectl apply -f deployment.yaml

  12. EVENTS И ДИАГНОСТИКА ПРОБЛЕМ
  События — то, что происходило в кластере. Помогают понять,
  почему что-то не работает.

  КОМАНДЫ:
    # Все события в namespace.
    kubectl get events -n demo

    # С сортировкой по времени.
    kubectl get events -n demo --sort-by=.metadata.creationTimestamp

    # Только предупреждения.
    kubectl get events -n demo --field-selector type=Warning

    # С фильтром по объекту.
    kubectl get events -n demo --field-selector involvedObject.name=app-abc123

  ТИПИЧНЫЕ ПРОБЛЕМЫ И ЧТО СМОТРЕТЬ:
    CrashLoopBackOff:
      kubectl describe pod <name>
      kubectl logs <name> --previous

    ImagePullBackOff:
      kubectl describe pod <name>
      # Смотреть events: "Failed to pull image".

    Pending:
      kubectl describe pod <name>
      # Смотреть events: "0/3 nodes available".

    OOMKilled:
      kubectl describe pod <name>
      # Смотреть Last State: Reason: OOMKilled.
      # Увеличить limits.memory.

    Unhealthy:
      kubectl describe pod <name>
      # Смотреть probes и их ошибки.

  ПОЛЕЗНЫЕ КОМАНДЫ:
    # Топ по CPU и памяти.
    kubectl top pods -n demo
    kubectl top nodes

    # API resources — что доступно в кластере.
    kubectl api-resources

    # Версии API.
    kubectl api-versions

    # Информация о кластере.
    kubectl cluster-info

    # Логи API Server (если есть доступ к ноде).

  13. СВЯЗЬ С GO
  kubectl — для ручной работы. Для автоматизации — client-go.
  CLIENT-GO — библиотека для работы с K8s API из Go.

  ПРИМЕР:
    import (
        "context"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/client-go/kubernetes"
        "k8s.io/client-go/rest"
    )

    func main() {
        // Конфиг из ~/.kube/config или из Pod'а.
        config, _ := rest.InClusterConfig()

        clientset, _ := kubernetes.NewForConfig(config)

        // Получить список Pod'ов в namespace.
        pods, _ := clientset.CoreV1().
            Pods("demo").
            List(context.Background(), metav1.ListOptions{})

        for _, pod := range pods.Items {
            fmt.Println(pod.Name, pod.Status.Phase)
        }
    }

  IN-CLUSTER CONFIG:
    Когда Pod работает внутри кластера, client-go сам находит
    API Server через service account:

      • Переменные KUBERNETES_SERVICE_HOST/PORT.
      • Токен в /var/run/secrets/kubernetes.io/serviceaccount/.

  ЗАЧЕМ НУЖНО:
    • Написание операторов (custom controllers).
    • Автоматизация деплоя.
    • Сбор информации о кластере из сервиса.
    • Интеграции (CI/CD, monitoring).

  14. ШПАРГАЛКА

  СМОТРЕТЬ:
    kubectl get pods -n <ns>
    kubectl get pods -o wide -n <ns>
    kubectl get pods -w -n <ns>
    kubectl get all -n <ns>
    kubectl get events -n <ns> --sort-by=.metadata.creationTimestamp

  ДИАГНОСТИКА:
    kubectl describe pod <name> -n <ns>
    kubectl logs <pod> -n <ns>
    kubectl logs <pod> --previous -n <ns>
    kubectl logs -f <pod> -n <ns>

  ЗАЙТИ ВНУТРЬ:
    kubectl exec -it <pod> -n <ns> -- sh
    kubectl debug -it <pod> --image=alpine -n <ns>

  СОЗДАТЬ/УДАЛИТЬ:
    kubectl apply -f <file>.yaml
    kubectl delete -f <file>.yaml
    kubectl delete namespace <ns>

  ОБНОВИТЬ:
    kubectl set image deployment/<name> <c>=<image>:<tag> -n <ns>
    kubectl rollout status deployment/<name> -n <ns>
    kubectl rollout undo deployment/<name> -n <ns>

  МАСШТАБ:
    kubectl scale deployment/<name> --replicas=5 -n <ns>

  ЛОКАЛЬНО:
    kubectl port-forward svc/<name> 8080:80 -n <ns>
    kubectl port-forward pod/<name> 5432:5432 -n <ns>

  КОНТЕКСТЫ:
    kubectl config current-context
    kubectl config use-context <name>
    kubectl config set-context --current --namespace=<ns>
    kubectx <name>          # утилита
    kubens <name>           # утилита

  15. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  90% работы — 15 команд: get, describe, logs, exec, apply,
      delete, rollout, scale, port-forward, config.
  2.  kubectl get pods -o wide — базовый просмотр. IP и NODE сразу видны.
  3.  kubectl describe — главный инструмент отладки. Events
      внизу покажут, что не так.
  4.  kubectl logs — логи. --previous для упавших контейнеров.
      -f для follow.
  5.  kubectl exec -it <pod> -- sh — зайти внутрь. Не работает
      на distroless — используй kubectl debug.
  6.  kubectl apply -f — декларативно. Идемпотентно. Всегда
      используй YAML, не create с флагами.
  7.  kubectl rollout — обновления. set image, status, undo, restart.
  8.  kubectl scale — масштабирование. Для stateless.
  9.  kubectl port-forward — локальный доступ. Для отладки.
  10. Контексты: kubectl config + kubectx/kubens. Всегда явно
      указывай namespace.
  11. Клиент для автоматизации — client-go. Для операторов и скриптов.
  12. Классический приём: --dry-run=client -o yaml для
      генерации шаблонов.
  13. На собесе: знать get, describe, logs, exec, apply,
      rollout. Уметь объяснить, как диагностировать
      CrashLoopBackOff.
  14. Не заучивай всё. Держи шпаргалку. 90% команд повторяются.
  15. kubectl — навык. Через неделю работы все команды
      запомнятся сами. Не старайся заучить сейчас.
*/
