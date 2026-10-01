package main

/*
  УРОК 2.3: STATEFULSET И JOB
  Deployment — для stateless. Но не всё в системе stateless.
  Есть БД, брокеры, хранилища — им нужны стабильные имена,
  стабильные диски, порядок запуска. Для них — StatefulSet.
  Есть одноразовые задачи: миграции, бэкапы, сиды. Они не
  должны крутиться вечно, как Deployment. Для них — Job и CronJob.
  Эта тема — про контроллеры, которые закрывают оставшиеся
  20% сценариев: stateful-приложения и batch-задачи.

  СОДЕРЖАНИЕ:
    1.  Зачем StatefulSet, если есть Deployment
    2.  Что такое стабильность в StatefulSet
    3.  Стабильные имена (ordinal)
    4.  Стабильные volumes (volumeClaimTemplates)
    5.  Порядок запуска и остановки
    6.  Headless Service — обязательный спутник
    7.  PVC retention — что происходит при удалении
    8.  Когда StatefulSet не нужен (Managed Postgres)
    9.  Job — одноразовая задача
    10. Job: parallelism, completions, backoffLimit
    11. CronJob — задача по расписанию
    12. concurrencyPolicy — что делать с перекрытием
    13. init-контейнер vs Job для миграций
    14. Связь с Go
    15. Антипаттерны
    16. Финальные выводы

  1. ЗАЧЕМ STATEFULSET, ЕСЛИ ЕСТЬ DEPLOYMENT

  Deployment создаёт Pod'ы с СЛУЧАЙНЫМИ именами:
    app-6b9d8f7c4-x9k2p
    app-6b9d8f7c4-p8m3n
    app-6b9d8f7c4-q7w5t

  Pod'ы взаимозаменяемы. Упал один — ReplicaSet создаёт новый
  с новым именем. IP тоже меняется. Volumes общие или
  эфемерные.

  ДЛЯ STATELESS ЭТО ИДЕАЛЬНО:
    • HTTP-сервис: любой Pod обработает запрос.
    • Worker: любая задача на любом Pod'е.
    • NoSQL с репликацией: любой узел может ответить.

  НО ДЛЯ БД ЭТО КАТАСТРОФА:
    • Postgres: реплика должна знать адрес мастера. Если у
      мастера имя меняется при рестарте — реплики теряют связь.
    • Kafka: брокеры обмениваются данными по стабильным
      адресам. Меняющееся имя = потеря данных.
    • Elasticsearch: ноды образуют кластер по именам.
      Случайные имена — кластер не соберётся.

  STATEFULSET ДАЁТ ИМ ТО, ЧТО НУЖНО:
    • Стабильные имена: postgres-0, postgres-1, postgres-2.
    • Стабильные volumes: каждый Pod со своим PVC.
    • Стабильный DNS: postgres-0.postgres, postgres-1.postgres.
    • Порядок запуска: 0 → 1 → 2.
    • Порядок остановки: 2 → 1 → 0.

  ПРАВИЛО: StatefulSet — для приложений, которым нужна
  идентичность. Deployment — для тех, кому всё равно.

  2. ЧТО ТАКОЕ СТАБИЛЬНОСТЬ В STATEFULSET
  ТРИ КЛЮЧЕВЫХ ГАРАНТИИ:

    ИДЕНТИЧНОСТЬ:
      У Pod'а постоянное имя. Упал postgres-1 — пересоздаётся
      с именем postgres-1. Не postgres-abc123.

    ДАННЫЕ:
      У Pod'а постоянный PVC. postgres-0 всегда монтирует
      свой volume. При пересоздании — тот же диск.

    СЕТЬ:
      У Pod'а стабильный DNS. postgres-0.postgres всегда
      резолвится в конкретный Pod.

  СХЕМА:
    Deployment:                    StatefulSet:
    ┌────────────────┐             ┌────────────────┐
    │ app-abc-x9k2p  │             │ postgres-0     │
    │ app-abc-p8m3n  │             │ postgres-1     │
    │ app-abc-q7w5t  │             │ postgres-2     │
    └────────────────┘             └────────────────┘
    Случайные имена                Стабильные имена
    Взаимозаменяемы                Каждый уникален

  ЧТО ЭТО ДАЁТ:
    Postgres master на postgres-0.
    Реплика на postgres-1 знает: «мастер — postgres-0.postgres».
    Если postgres-0 упал — пересоздался с тем же именем.
    Реплика догоняет его. Никаких настроек.

  3. СТАБИЛЬНЫЕ ИМЕНА (ORDINAL)
  Имена Pod'ов StatefulSet — ordinal от 0 до N+1:
    postgres-0
    postgres-1
    postgres-2
  ordinal = индекс. Постоянный. Не меняется.

  ЧТО ЭТО ДАЁТ:
    • Ты можешь явно обращаться к конкретному Pod'у:
      psql -h postgres-0.postgres
    • Логика приложения может использовать ordinal:
      «я postgres-0 — значит, я мастер».
    • DNS резолвится в конкретный Pod.

  ОБРАТИ ВНИМАНИЕ:
    Даже если Pod упал и его пересоздали — ordinal сохраняется.
    postgres-1 остаётся postgres-1. Всегда.

  АНТИПАТТЕРН:
    Завязываться на ordinal в логике приложения — так себе.
    Но для БД — часто единственный способ.

  4. СТАБИЛЬНЫЕ VOLUMES (VOLUME CLAIM TEMPLATES)
  В Deployment — один общий PVC или emptyDir.
  В StatefulSet — каждому Pod свой PVC.

  ПРИМЕР:
    spec:
      volumeClaimTemplates:
      - metadata:
          name: data
        spec:
          accessModes: ["ReadWriteOnce"]
          resources:
            requests:
              storage: 10Gi

  ЧТО ПРОИСХОДИТ:
    StatefulSet создаёт PVC для каждого Pod:
      data-postgres-0
      data-postgres-1
      data-postgres-2

  Каждый PVC привязан к своему Pod'у. Postgres-0 всегда
  монтирует data-postgres-0.

  ЧТО ЭТО ДАЁТ:
    • Упал postgres-1 — пересоздался, монтирует тот же
      data-postgres-1.
    • Данные не теряются.
    • Каждый Pod пишет в свой диск (для репликации).

  ВАЖНО:
    PVC НЕ удаляются при удалении Pod'а. Даже при удалении
    StatefulSet. Их надо удалять вручную.
    Это защита от случайной потери данных.

  5. ПОРЯДОК ЗАПУСКА И ОСТАНОВКИ
  StatefulSet запускает Pod'ы ПОСЛЕДОВАТЕЛЬНО, не параллельно.

  ЗАПУСК:
    Шаг 1: создаётся postgres-0.
    Шаг 2: ждёт, пока postgres-0 станет Ready.
    Шаг 3: создаётся postgres-1.
    Шаг 4: ждёт Ready.
    Шаг 5: создаётся postgres-2.
    ...

  ОСТАНОВКА:
    Обратный порядок: 2 → 1 → 0.
    Шаг 1: удаляется postgres-2.
    Шаг 2: ждёт, пока удалится.
    Шаг 3: удаляется postgres-1.
    ...

  ЗАЧЕМ ЭТО:
    Для БД это критично. Реплика не должна стартовать раньше
    мастера. Иначе она не знает, откуда синхронизироваться.
    Каждый следующий Pod может рассчитывать, что предыдущий
    уже работает.

  СКОЛЬКО ЖДЁТ:
    По умолчанию — бесконечно. Пока Pod не станет Ready.

    Можно настроить:
      spec:
        podManagementPolicy: Parallel

    Тогда Pod'ы стартуют параллельно. Для некоторых приложений
    (например, Zookeeper) это правильно.
    Но для Postgres — Sequential (дефолт). Критично.

  ПРИМЕР ПОШАГОВО:
    kubectl get pods -w -l app=postgres

    # postgres-0   0/1  Pending       — создаётся
    # postgres-0   0/1  ContainerCreating
    # postgres-0   1/1  Running       — готов
    # postgres-1   0/1  Pending       — только теперь
    # ...

  6. HEADLESS SERVICE — ОБЯЗАТЕЛЬНЫЙ СПУТНИК
  StatefulSet требует Headless Service (clusterIP: None).

  ЗАЧЕМ:
    Обычный Service даёт один ClusterIP и балансирует между
    Pod'ами. Для БД это плохо — клиент не знает, к какому
    Pod'у подключился.
    Headless Service не даёт ClusterIP. Вместо этого DNS
    возвращает IP каждого Pod'а отдельно.

  ПРИМЕР:
    apiVersion: v1
    kind: Service
    metadata:
      name: postgres
    spec:
      clusterIP: None        # headless
      selector:
        app: postgres
      ports:
      - port: 5432

  ЧТО ЭТО ДАЁТ:
    DNS-записи:
      postgres.demo.svc.cluster.local        → все IP Pod'ов
      postgres-0.postgres.demo.svc.cluster.local  → IP postgres-0
      postgres-1.postgres.demo.svc.cluster.local  → IP postgres-1
      postgres-2.postgres.demo.svc.cluster.local  → IP postgres-2

  КАК ИСПОЛЬЗУЕТСЯ:
    • Приложение может обратиться к конкретному Pod'у:
      psql -h postgres-0.postgres
    • Реплика знает адрес мастера: postgres-0.postgres.
    • Service discovery внутри кластера через DNS.

  ОБЯЗАТЕЛЬНО ЛИ:
    Формально — serviceName в StatefulSet обязателен. Но можно
    указать обычный Service. Правда, теряются DNS-записи
    отдельных Pod'ов.
    На практике — всегда Headless.

  7. PVC RETENTION — ЧТО ПРОИСХОДИТ ПРИ УДАЛЕНИИ
  ЭТО ВАЖНО. PVC ведут себя иначе, чем Pod'ы.

  ЧТО ПРОИСХОДИТ ПРИ УДАЛЕНИИ STATEfulSET:
    StatefulSet удаляется. Pod'ы удаляются. НО PVC ОСТАЮТСЯ.
    data-postgres-0
    data-postgres-1
    data-postgres-2
    Это защита от случайной потери данных.

  ЧТО ПРОИСХОДИТ ПРИ УДАЛЕНИИ POD'А:
    Pod удаляется. PVC остаётся.
    Если Pod пересоздаётся (например, StatefulSet восстанавливает) —
    монтирует тот же PVC.

  КАК УДАЛИТЬ PVC:
    Вручную:
      kubectl delete pvc data-postgres-0 -n demo

    Или через политику:
      spec:
        persistentVolumeClaimRetentionPolicy:
          whenDeleted: Delete    # при удалении StatefulSet
          whenScaled: Retain     # при уменьшении replicas

    Значения:
      • Retain (дефолт) — оставить.
      • Delete — удалить.

  НА ПРАКТИКЕ:
    Всегда Retain. Случайно удалить StatefulSet — не потерять
    данные. Удалить PVC руками, когда уверен.

  ЕСЛИ НУЖНО ПЕРЕИСПОЛЬЗОВАТЬ PVC:
    После удаления StatefulSet нужно вручную удалить PVC,
    иначе новый StatefulSet не создаст свой (имя занято).

  8. КОГДА STATEFULSET НЕ НУЖЕН (MANAGED POSTGRES)

  ВАЖНАЯ ПРАВДА:
    StatefulSet — это сложно. Репликация, failover, backup,
    обновления — всё на тебе. Для БД это ад.

  ЧТО ДЕЛАЮТ В ПРОДЕ:
    БД выносят в Managed-сервисы:
      • AWS RDS (Postgres, MySQL, Aurora).
      • Google Cloud SQL.
      • Azure Database.
      • MongoDB Atlas.
      • Aiven (Kafka, Postgres, Redis).

  ЧТО MANAGED ДАЁТ:
    • Автоматический failover.
    • Репликация из коробки.
    • Backup и restore.
    • Обновления и патчи.
    • Мониторинг.
    • Масштабирование.

  ЧТО НЕ ДАЁТ:
    • Контроль над версией (ограниченный набор).
    • Кастомизация конфигов (ограниченная).
    • Дешевле — обычно дороже.

  ДЛЯ GO:
    «В большинстве случаев — Managed (RDS, Cloud SQL).
    StatefulSet уместен, когда нужен полный контроль над
    конфигами или on-prem. Но его эксплуатация — задача
    отдельной команды».

  КОГДА STATEFULSET ОПРАВДАН:
    • On-prem, где нет Managed.
    • Кастомные конфиги (специфичные тюнинги).
    • Специфичные расширения БД.
    • Kafka/Elasticsearch в больших объёмах, где Managed дорог.
    • Обучение и пет-проекты.

  9. JOB — ОДНОРАЗОВАЯ ЗАДАЧА
  Job запускает Pod'ы до успешного завершения. Не держит их
  живыми постоянно.

  ЗАЧЕМ:
    • Миграции БД.
    • Одноразовый seed данных.
    • Backup.
    • Cleanup старых записей.
    • Batch-обработка.

  ПРИМЕР:
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: app-migrate
    spec:
      backoffLimit: 3               # максимум 3 попытки
      ttlSecondsAfterFinished: 300  # удалить через 5 минут
      template:
        spec:
          restartPolicy: OnFailure  # перезапускать при ошибке
          containers:
          - name: migrate
            image: my-app:1.0
            command: ["/server", "migrate"]

  КЛЮЧЕВЫЕ ПОЛЯ:
    backoffLimit:
      Сколько раз повторить при падении.
      Дефолт 6. После — Job помечается Failed.

    ttlSecondsAfterFinished:
      Сколько секунд хранить завершённый Job.
      После — удаляется автоматически.

    restartPolicy:
      OnFailure — перезапускать при != 0.
      Never — не перезапускать (для специфичных случаев).

  ЖИЗНЕННЫЙ ЦИКЛ:
    Создан → Pod запущен → успех → Job Completed.
    Или: попытка → провал → retry → ... → backoffLimit → Failed.

  СТАТУС:
    kubectl get jobs -n demo
    # NAME           COMPLETIONS   DURATION   AGE
    # app-migrate    1/1           15s        30s

    COMPLETIONS 1/1 — успех.
    Если 0/1 и не растёт — Job застрял.

  ЛОГИ:
    kubectl logs job/app-migrate -n demo

  КОГДА ИСПОЛЬЗУЕТСЯ ДЛЯ МИГРАЦИЙ:
    В паре с Deployment:
      • Job выполняется ДО старта Deployment.
      • В Deployment — init-контейнер, который ждёт Job.
    Или:
      • Deployment сам запускает миграции при старте (в коде).
      • Но это хуже — миграции могут запуститься параллельно
        в нескольких Pod'ах.

  10. JOB: PARALLELISM, COMPLETIONS, BACKOFFLIMIT
  Job умеет запускать несколько Pod'ов параллельно.

  ОБЫЧНЫЙ JOB (1 Pod):
    spec:
      template:
        ...

    Запускает 1 Pod до успеха.

  JOB С COMPLETIONS (N раз):
    spec:
      completions: 5
      template:
        ...

    Запускает 5 успешных завершений (не одновременно).

  JOB С PARALLELISM (параллельно):
    spec:
      completions: 10
      parallelism: 3
      template:
        ...

    Запускает до 3 Pod'ов одновременно.
    Всего нужно 10 успешных завершений.

  ПРИМЕРЫ ИСПОЛЬЗОВАНИЯ:
    • Бэкап БД: completions: 1, parallelism: 1.
    • Обработка 1000 файлов: completions: 100, parallelism: 10.
      Каждый Pod берёт свою порцию (через индекс или очередь).

  ИНДЕКСЫ:
    Job передаёт Pod'ам переменные:
      JOB_COMPLETION_INDEX  — индекс Pod'а (0..N-1).

    Приложение может использовать для шардирования:
      shard = JOB_COMPLETION_INDEX
      for i := shard; i < total; i += N {
          processItem(i)
      }

  backoffLimit:
    Сколько неудачных попыток до Failed.
    Каждая попытка — новый Pod.
    Между попытками — экспоненциальный backoff.

  11. CRONJOB — ЗАДАЧА ПО РАСПИСАНИЮ
  CronJob — Job, который запускается по расписанию.

  ПРИМЕР:
    apiVersion: batch/v1
    kind: CronJob
    metadata:
      name: backup
    spec:
      schedule: "0 3 * * *"          # каждый день в 3:00
      jobTemplate:
        spec:
          template:
            spec:
              restartPolicy: OnFailure
              containers:
              - name: backup
                image: backup:1.0

  FORMAT SCHEDULE (как в cron):
    ┌───────────── минута (0-59)
    │ ┌───────────── час (0-23)
    │ │ ┌───────────── день месяца (1-31)
    │ │ │ ┌───────────── месяц (1-12)
    │ │ │ │ ┌───────────── день недели (0-6, 0 = воскресенье)
    │ │ │ │ │
    * * * * *
  ПРИМЕРЫ:
    "0 3 * * *"        — каждый день в 3 утра.
    "/5 * * * *"      — каждые 5 минут.
"0 /2 * * *"      — каждые 2 часа.
"0 0 * * 0"        — воскресенье в полночь.
"0 9-17 * * 1-5"   — 9-17 по будням.

АНАЛОГИЯ:
CronJob = Job + расписание.
Каждое срабатывание создаёт новый Job.

ХРАНЕНИЕ ИСТОРИИ:
spec:
successfulJobsHistoryLimit: 3
failedJobsHistoryLimit: 1
Сколько завершённых Job'ов хранить. Старые удаляются.

ПРИМЕРЫ ИСПОЛЬЗОВАНИЯ:
• Бэкапы БД (ночь).
• Cleanup старых записей.
• Отчёты (утром).
• Синхронизация с внешними системами.
• Reindex Elasticsearch.

ЧТО ВАЖНО ПОМНИТЬ:
CronJob использует UTC по умолчанию. Указывать timezone:

spec:
timeZone: "Europe/Moscow"    # K8s 1.27+

12. CONCURRENCYPOLICY — ЧТО ДЕЛАТЬ С ПЕРЕКРЫТИЕМ
Если предыдущий Job ещё работает, а время следующего уже
наступило — что делать?

ALLOW (дефолт):
Запустить новый Job параллельно.
Может быть несколько Job'ов одновременно.

FORBID:
Пропустить новое срабатывание.
Если предыдущий Job не завершился — новый не создастся.
Используется для бэкапов: не хотим два одновременно.

REPLACE:
Убить предыдущий Job и запустить новый.
Для задач, где важна свежесть данных.

ПРИМЕР:
spec:
schedule: "/5 * * * *"
concurrencyPolicy: Forbid

ЧТО ВЫБРАТЬ:
• Бэкап — Forbid. Два бэкапа одновременно — плохо.
• Cleanup — Forbid. Не имеет смысла параллельно.
• Отчёты — Allow. Могут работать параллельно.
• Синхронизация — Replace. Свежие данные важнее.

13. INIT-КОНТЕЙНЕР VS JOB ДЛЯ МИГРАЦИЙ
Есть два способа запускать миграции в K8s. У каждого свои
плюсы и минусы.

ВАРИАНТ 1: INIT-КОНТЕЙНЕР В DEPLOYMENT
Deployment:
initContainers:
- name: migrate
image: my-app:1.0
command: ["/server", "migrate"]
containers:
- name: app
...

ПЛЮСЫ:
• Просто — всё в одном манифесте.
• Автоматически перед стартом.
• Не нужно ничего запускать вручную.

МИНУСЫ:
• Несколько реплик Deployment запустят миграции
ПАРАЛЛЕЛЬНО. Может быть race condition.
• Если init упадёт — Pod не запустится.
• При каждом рестарте Pod'а миграции запускаются снова.

ВАРИАНТ 2: ОТДЕЛЬНЫЙ JOB

Job:
name: app-migrate
...

Deployment:
...

ПЛЮСЫ:
• Гарантированно один раз.
• Можно запускать вручную: kubectl apply -f job.yaml.
• Логи отдельно.
• Не блокирует Deployment.

МИНУСЫ:
• Нужно запускать явно перед деплоем.
• В CI это отдельный шаг.

РЕКОМЕНДАЦИЯ:
Для прода — Job. В CI pipeline:
kubectl apply -f k8s/migrate-job.yaml
kubectl wait --for=condition=complete job/app-migrate --timeout=120s
kubectl apply -f k8s/deployment.yaml
Для dev/staging — можно init-контейнер, если реплика одна.

ЧТО ВАЖНО ПРО МИГРАЦИИ:
• Должны быть идемпотентны.
• Должны быть обратно совместимы (старая версия + новая
схема должны работать вместе во время rolling update).
• Должны логировать результат.

СВЯЗЬ С GO:
Один бинарник — разные команды:
/server             — запускает HTTP-сервер.
/server migrate     — применяет миграции.
/server seed        — загружает тестовые данные.

В Job'е: command: ["/server", "migrate"].
В Deployment: command: ["/server"].

14. СВЯЗЬ С GO
ЧТО КАСАЕТСЯ GO-РАЗРАБОТЧИКА:

STATEFULSET — как использовать:
• Если ты разворачиваешь Postgres/Kafka/ES — пиши StatefulSet.
• В 90% случаев это делает DevOps или Managed-сервис.
• Ты как Go-разработчик подключаешься к нему через Headless
Service: `postgres-0.postgres.demo.svc.cluster.local`.

JOB — как использовать:
• Миграции: отдельный Job в CI перед деплоем.
• Один бинарник, команда migrate.
• Ждёт завершения БД (retry в коде).
• Идемпотентные миграции (goose, migrate, atlas).

CRONJOB — как использовать:
• Если у тебя сервис делает периодические задачи —
вынеси в CronJob.
• Каждое срабатывание — отдельный процесс.
• Не надо держать долгоживущий Pod ради того, чтобы
раз в час что-то делать.

ПРИМЕР GO-КОДА ДЛЯ MIGRATE:
func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := runMigrate(); err != nil {
			log.Fatal(err)
		}
		return
	}
	runServer()
}
func runMigrate() error {
	// Подключение к БД с retry.
	db, err := connectDB()
	if err != nil {
		return err
	}
	defer db.Close()

	// Применение миграций (goose, migrate).
	if err := goose.Up(db, "migrations"); err != nil {
		return err
	}
	return nil
}

ЧТО ЗАПУСКАЕТ В K8S:
Job:
command: ["/server", "migrate"]

ЧТО ВАЖНО В КОДЕ:
• Идемпотентность. Если миграция уже применена — не падать.
• Retry подключения. БД может стартовать медленно.
• Exit 0 при успехе. Иначе Job перезапустится.

15. АНТИПАТТЕРНЫ

15.1. ИСПОЛЬЗОВАТЬ DEPLOYMENT ДЛЯ БД.
Postgres, Kafka, ES — только StatefulSet. Deployment
даст случайные имена и эфемерные диски.

15.2. STATEFULSET БЕЗ HEADLESS SERVICE.
Формально работает, но нет DNS-записей Pod'ов. Реплика
не знает адрес мастера.

15.3. ИСПОЛЬЗОВАТЬ ОБЫЧНЫЙ SERVICE ВМЕСТО HEADLESS.
Балансировка между Pod'ами БД — плохо. Клиент не знает,
к какому подключился.

15.4. УДАЛИТЬ STATEFULSET БЕЗ ПОНИМАНИЯ PVC.
PVC остаются. Занимают место. Новый StatefulSet не может
создать PVC с тем же именем.

15.5. STATEFULSET С МАЛЕНЬКИМ PVC.
Postgres 10 ГБ? Через месяц — кончилось место. Сразу
считай с запасом.

15.6. STATEFULSET НА NODE С ОДНИМ ДИСКОМ.
Несколько Pod'ов на одной ноде — contention за диск.
Для БД нужны отдельные ноды или intiaffinity.

15.7. JOB БЕЗ backoffLimit.
Дефолт 6. Если миграция падает по причине (не retry) —
будет 6 попыток. Может забить логи.

15.8. JOB БЕЗ ttlSecondsAfterFinished.
Завершённые Job'ы копятся. Засоряют кластер.

15.9. CRONJOB С concurrencyPolicy: Allow ДЛЯ БЭКАПА.
Два бэкапа одновременно — плохо. Forbid.

15.10. ЗАПУСКАТЬ МИГРАЦИИ В INIT-КОНТЕЙНЕРЕ С 3 РЕПЛИКАМИ.
Три Pod'а одновременно запустят миграции. Race condition.

15.11. НЕИДЕМПОТЕНТНЫЕ МИГРАЦИИ.
При retry могут испортить данные. Всегда идемпотентные.

15.12. МИГРАЦИИ, ЛОМАЮЩИЕ СТАРУЮ ВЕРСИЮ.
Во время rolling update работают и старая, и новая версии.
Миграции должны быть обратно совместимы.

15.13. CUSTOM TIMEOUT ДЛЯ JOB БЕЗ activeDeadlineSeconds.
Если Job застрянет — он никогда не завершится. Ставь
activeDeadlineSeconds.

15.14. STATEFULSET С replicas: 0 В ПРОДЕ.
Возможно, ты забыл вернуть. Или оставил случайно.
Мониторинг покажет.

16. ФИНАЛЬНЫЕ ВЫВОДЫ
1.  StatefulSet — для stateful-приложений: БД, Kafka,
Elasticsearch, Zookeeper.
2.  Три гарантии StatefulSet:
• Стабильные имена (postgres-0, postgres-1).
• Стабильные volumes (PVC на каждого Pod'а).
• Стабильный DNS (postgres-0.postgres).
3.  Порядок запуска: 0 → 1 → 2.
Порядок остановки: 2 → 1 → 0.
4.  Headless Service обязателен. clusterIP: None.
DNS-записи для каждого Pod'а.
5.  PVC остаются при удалении StatefulSet. Защита от потери данных.
6.  В 90% случаев БД — Managed (RDS, Cloud SQL, Atlas).
StatefulSet — когда нужен полный контроль или on-prem.
7.  Job — одноразовая задача. backoffLimit, ttlSecondsAfterFinished.
Для миграций, seed, backup.
8.  completions — сколько раз, parallelism — сколько параллельно.
9.  CronJob — Job по расписанию. Формат как в cron.
10. concurrencyPolicy: Allow, Forbid, Replace.
11. Для миграций в проде — отдельный Job в CI, не
init-контейнер. Идемпотентные миграции.
12. Один Go-бинарник — команды server, migrate, seed.
В Job'е: command: ["/server", "migrate"].
13. Антипаттерны: Deployment для БД, StatefulSet без
Headless, удаление PVC без понимания, миграции в
init с 3 репликами, неидемпотентные миграции.
*/
