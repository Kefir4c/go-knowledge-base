package main

/*
  УРОК 4.1: CONFIGMAP
  Образ — неизменяемый артефакт. Один и тот же образ должен
  работать в dev, staging и prod. Меняются только конфиги:
  адрес БД, уровень логов, фича-флаги. Если запекать конфиги
  в образ — придётся собирать три образа, что противоречит
  идее «build once, run anywhere».
  ConfigMap решает это: конфиги живут отдельно от образа.
  Меняешь ConfigMap — Pod'ы получают новые значения. Один
  образ — много окружений.

  СОДЕРЖАНИЕ:
    1.  Зачем нужен ConfigMap
    2.  Что можно и что нельзя хранить
    3.  Создание через kubectl
    4.  Создание через манифест
    5.  Использование как env
    6.  Использование как volume
    7.  env vs volume — разница в hot reload
    8.  Обновление ConfigMap и rollout
    9.  Immutable ConfigMap
    10. Ограничения и лимиты
    11. ConfigMap vs Secret
    12. Полезные команды
    13. Связь с Go
    14. Антипаттерны
    15. Финальные выводы

  1. ЗАЧЕМ НУЖЕН CONFIGMAP

  ПРОБЛЕМА: конфиги в образе.

    Если запечь config.yaml в образ — придётся собирать
    отдельный образ для каждого окружения. Один и тот же код
    в dev и prod, но образы разные. Это противоречит идее
    immutable infrastructure.

    Плюс секреты в образе — вообще катастрофа. Любой, у кого
    есть доступ к registry, увидит пароли.

  РЕШЕНИЕ: ConfigMap.

    Конфиги живут в K8s как отдельный объект.
    Pod'ы читают их при старте.
    Один образ — разные ConfigMap для разных окружений.

  СХЕМА:
    ┌────────────────┐       ┌──────────────────┐
    │  Образ my-app  │       │   ConfigMap      │
    │  (одинаковый)  │       │  (dev / staging  │
    │                │       │   / prod)        │
    └────────┬───────┘       └────────┬─────────┘
             │                        │
             └──────────┬─────────────┘
                        │
                        ▼
              ┌───────────────┐
              │  Pod          │
              │  + env        │
              │  + volume     │
              └───────────────┘

  ЧТО МОЖНО НЕ ПЕРЕСОБИРАТЬ ОБРАЗ:

    • Адрес БД.
    • Уровень логов.
    • Таймауты.
    • Фича-флаги.
    • URLs внешних сервисов.
    • Лимиты, размеры буферов.

  2. ЧТО МОЖНО И ЧТО НЕЛЬЗЯ ХРАНИТЬ

  МОЖНО:
    • Key-value пары (строки).
    • Целые файлы (nginx.conf, app.yaml).
    • Многострочные тексты.
    • Бинарные данные через binaryData (base64).

  НЕЛЬЗЯ (или не надо):
    • Секреты (пароли, токены) — только Secret.
    • Большие файлы (> 1 МБ) — лимит etcd.
    • Изменяемые в рантайме данные — ConfigMap для конфигов,
      не для данных.

  ФОРМАТ:
    data:
      KEY: "value"                  # строки
      config.yaml: |                # многострочный
        server:
          port: 8080
      binaryData:                   # бинарные данные
        logo.png: "iVBORw0KGgo..."  # base64

  3. СОЗДАНИЕ ЧЕРЕЗ KUBECTL

  ИЗ ЛИТЕРАЛОВ:
    kubectl create configmap app-config \
      --from-literal=APP_ENV=production \
      --from-literal=LOG_LEVEL=info \
      -n demo

    Создаёт ConfigMap с двумя ключами.

  ИЗ ФАЙЛА:
    kubectl create configmap app-config \
      --from-file=config.yaml \
      -n demo

    Ключ = имя файла (config.yaml). Значение = содержимое.

  С ИЗМЕНЁННЫМ КЛЮЧОМ:
    kubectl create configmap app-config \
      --from-file=app.yaml=./configs/prod.yaml \
      -n demo

    Ключ app.yaml, значение — из файла prod.yaml.

  ИЗ ПАПКИ:
    kubectl create configmap app-config \
      --from-file=./configs/ \
      -n demo

    Каждый файл в папке → отдельный ключ.

  ИЗ ENV-ФАЙЛА:
    kubectl create configmap app-config \
      --from-env-file=.env \
      -n demo

    Каждая строка KEY=value → ключ в ConfigMap.

  МИНУС ЭТОГО СПОСОБА:
    Императивный. ConfigMap не описан в git. При пересоздании
    кластера — надо создавать заново вручную.

    В проде — только манифесты.

  4. СОЗДАНИЕ ЧЕРЕЗ МАНИФЕСТ
  ПРАВИЛЬНЫЙ СПОСОБ — декларативный.

    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: app-config
      namespace: demo
      labels:
        app: app
    data:
      # Простые пары.
      APP_ENV: "production"
      LOG_LEVEL: "info"
      DB_HOST: "postgres.demo.svc.cluster.local"
      DB_PORT: "5432"

      # Многострочный конфиг.
      app.yaml: |
        server:
          port: 8080
          read_timeout: 10s
          write_timeout: 30s
        log:
          level: info
          format: json

  ПРИМЕНИТЬ:
    kubectl apply -f configmap.yaml

  ПРЕИМУЩЕСТВА:
    • В git — история изменений.
    • Воспроизводимость — применил к новому кластеру, всё
      работает.
    • Code review.
    • CI/CD автоматизация.

  5. ИСПОЛЬЗОВАНИЕ КАК ENV
  ДВА СПОСОБА передать ConfigMap в env:

  СПОСОБ 1: envFrom — все ключи сразу.
    spec:
      containers:
      - name: app
        image: my-app:1.0
        envFrom:
        - configMapRef:
            name: app-config

    Все ключи из ConfigMap → env переменные.

    В Go:
      env := os.Getenv("APP_ENV")    // "production"
      log := os.Getenv("LOG_LEVEL")  // "info"

  СПОСОБ 2: env — выборочно.
    spec:
      containers:
      - name: app
        image: my-app:1.0
        env:
        - name: APP_ENV
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: APP_ENV
        - name: LOG_LEVEL
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: LOG_LEVEL

    Только указанные ключи. Остальные игнорируются.

  КОГДА ЧТО:
    envFrom:
      • Много ключей.
      • Все нужны.
      • Проще.

    env:
      • Мало ключей.
      • Не все нужны.
      • Нужен префикс (можно через prefix: "").

  ПРЕФИКС:
    envFrom:
    - configMapRef:
        name: app-config
      prefix: APP_

    Тогда APP_ENV вместо ENV.

  ПРОБЛЕМА ENV:
    env устанавливается один раз при старте контейнера.
    Изменение ConfigMap не влияет на env работающего Pod'а.
    Нужен рестарт.

  6. ИСПОЛЬЗОВАНИЕ КАК VOLUME
  ConfigMap можно смонтировать в Pod как файлы.

    spec:
      containers:
      - name: app
        image: my-app:1.0
        volumeMounts:
        - name: config
          mountPath: /etc/app
          readOnly: true
      volumes:
      - name: config
        configMap:
          name: app-config

  ЧТО ПРОИСХОДИТ:
    Каждый ключ ConfigMap становится файлом в /etc/app.

    /etc/app/APP_ENV       → содержимое "production"
    /etc/app/LOG_LEVEL     → содержимое "info"
    /etc/app/app.yaml      → содержимое многострочного

  В Go:
    data, _ := os.ReadFile("/etc/app/app.yaml")

  ВЫБОРОЧНО ТОЛЬКО НЕКОТОРЫЕ КЛЮЧИ:
    volumes:
    - name: config
      configMap:
        name: app-config
        items:
        - key: app.yaml
          path: config.yaml
        - key: LOG_LEVEL
          path: log_level.txt

    В /etc/app будут только config.yaml и log_level.txt.

  РЕЖИМ ДОСТУПА:
    volumes:
    - name: config
      configMap:
        name: app-config
        defaultMode: 0644    # права на файлы

  7. ENV VS VOLUME — РАЗНИЦА В HOT RELOAD
  Ключевое отличие.

  ENV:
    • Устанавливается один раз при старте.
    • Изменение ConfigMap НЕ влияет на работающий Pod.
    • Нужен рестарт: kubectl rollout restart deployment/app.
    • Для immutable конфигов.

  VOLUME:
    • Монтируется как файлы.
    • Kubelet обновляет файлы при изменении ConfigMap.
    • Задержка 30-60 секунд (kubelet sync period).
    • Приложение должно перечитывать файлы.
    • Для конфигов, которые могут меняться в рантайме.

  ЧТО ЗНАЧИТ «ПРИЛОЖЕНИЕ ДОЛЖНО ПЕРЕЧИТЫВАТЬ»:
    K8s обновляет файл на диске. Но твоё приложение читает
    конфиг один раз при старте — и держит в памяти.

    Чтобы hot reload работал:
      • Приложение должно перечитывать файл.
      • Варианты:
        - По SIGHUP (сигнал).
        - По таймеру (каждые N секунд).
        - По inotify (изменение файла).

    Большинство Go-сервисов не умеют hot reload. Проще
    сделать rollout restart.

  ЧТО ОБНОВЛЯЕТСЯ, ЧТО НЕТ:
    subPath mount — НЕ обновляется. K8s не обновляет файлы,
    смонтированные через subPath.

    Обычный mount — обновляется.
    Это частая ловушка.

  8. ОБНОВЛЕНИЕ CONFIGMAP И ROLLOUT
  ОБНОВИТЬ CONFIGMAP:

    Вариант 1: kubectl edit.
      kubectl edit configmap app-config -n demo

    Вариант 2: apply из манифеста.
      Изменил configmap.yaml → kubectl apply -f configmap.yaml

    Вариант 3: kubectl create configmap с --dry-run=client -o yaml.
      Генерирует манифест, потом apply.

  ЧТО ПРОИСХОДИТ С POD'АМИ:
    ENV: ничего. Нужен rollout restart.

      kubectl rollout restart deployment/app -n demo

    VOLUME: файлы обновляются автоматически. Если приложение
    перечитывает — увидит новое. Если нет — тоже нужен
    restart.

  ROLLOUT RESTART:
    Создаёт новые Pod'ы с новым ConfigMap. Постепенно
    заменяет старые (rolling update).
    Применяется только к Deployment. Для StatefulSet — тоже
    работает. Для Pod'ов напрямую — только delete.

  АВТОМАТИЧЕСКИЙ ROLLOUT (продвинутое):
    Некоторые инструменты (Reloader, kube-applier) умеют
    перезапускать Deployment при изменении ConfigMap.

    В K8s из коробки этого нет.

  9. IMMUTABLE CONFIGMAP
  ConfigMap можно сделать неизменяемым.

    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: app-config
    immutable: true
    data:
      APP_ENV: "production"

  ЧТО ЭТО ДАЁТ:
    • Нельзя изменить (только удалить и создать заново).
    • Kubelet не следит за изменениями — меньше нагрузки
      на API Server в больших кластерах.
    • Защита от случайного изменения в проде.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Стабильные конфиги, которые не меняются без релиза.
    • Большие кластеры (тысячи Pod'ов) — экономия на watch.

  КАК ИЗМЕНИТЬ:
    Удалить и создать заново.

  10. ОГРАНИЧЕНИЯ И ЛИМИТЫ

  РАЗМЕР:
    Максимум 1 МБ на ConfigMap.

    Причина — etcd. ConfigMap хранится в etcd, а etcd не
    любит большие значения.
    Решение: разбить на несколько ConfigMap.

  КОЛИЧЕСТВО КЛЮЧЕЙ:
    Формально не ограничено. Но на практике — чем больше,
    тем хуже.
    Лучше один ConfigMap на сервис, не один на всё.

  ИМЕНА КЛЮЧЕЙ:
    Должны быть валидными переменными окружения для env.
    Для volume — любые, но избегай пробелов и спецсимволов.

  11. CONFIGMAP VS SECRET
  ┌────────────────────┬───────────────┬───────────────────┐
  │                    │ ConfigMap     │ Secret            │
  ├────────────────────┼───────────────┼───────────────────┤
  │ Данные             │ Обычные       │ base64 (не шифр)  │
  │ Назначение         │ Конфиги       │ Пароли, токены    │
  │ RBAC               │ Обычный       │ Отдельный         │
  │ Encryption at rest │ Нет           │ Настраивается     │
  │ Показ в логах      │ Не критично   │ Опасно            │
  │ Размер             │ 1 МБ          │ 1 МБ              │
  └────────────────────┴───────────────┴───────────────────┘

  ГЛАВНОЕ: ConfigMap для не-секретных данных. Secret для
  всего, что не должно попасть в логи или git.

  ПРИМЕРЫ CONFIGMAP:
    • APP_ENV.
    • LOG_LEVEL.
    • DB_HOST.
    • DB_PORT.
    • URLs.
    • Таймауты.

  ПРИМЕРЫ SECRET:
    • DB_PASSWORD.
    • API_KEY.
    • JWT_SECRET.
    • TLS-сертификаты.

  12. ПОЛЕЗНЫЕ КОМАНДЫ

  СОЗДАТЬ:
    kubectl create configmap app-config --from-literal=KEY=value
    kubectl create configmap app-config --from-file=config.yaml
    kubectl apply -f configmap.yaml

  СМОТРЕТЬ:
    kubectl get configmap -n demo
    kubectl get configmap app-config -n demo -o yaml
    kubectl describe configmap app-config -n demo

  РЕДАКТИРОВАТЬ:
    kubectl edit configmap app-config -n demo

  УДАЛИТЬ:
    kubectl delete configmap app-config -n demo
    kubectl delete -f configmap.yaml

  ПЕРЕЗАПУСТИТЬ POD'Ы:
    kubectl rollout restart deployment/app -n demo

  ПРОВЕРИТЬ В POD'Е:
    # Env.
    kubectl exec -it app-xxx -n demo -- env | grep APP_

    # Volume.
    kubectl exec -it app-xxx -n demo -- ls /etc/app/
    kubectl exec -it app-xxx -n demo -- cat /etc/app/app.yaml

  13. СВЯЗЬ С GO
  Два способа читать конфиг в Go.

  ЧЕРЕЗ ENV:
    func loadConfig() Config {
        return Config{
            AppEnv:    os.Getenv("APP_ENV"),
            LogLevel:  os.Getenv("LOG_LEVEL"),
            DBHost:    os.Getenv("DB_HOST"),
            DBPort:    os.Getenv("DB_PORT"),
            DBUser:    os.Getenv("DB_USER"),
            DBPassword: os.Getenv("DB_PASSWORD"),   // из Secret
        }
    }

  ЧЕРЕЗ ФАЙЛ:
    import "gopkg.in/yaml.v3"

    type Config struct {
        Server struct {
            Port         int    `yaml:"port"`
            ReadTimeout  string `yaml:"read_timeout"`
            WriteTimeout string `yaml:"write_timeout"`
        } `yaml:"server"`
        Log struct {
            Level  string `yaml:"level"`
            Format string `yaml:"format"`
        } `yaml:"log"`
    }

    func loadConfigFromFile(path string) (Config, error) {
        data, err := os.ReadFile(path)
        if err != nil {
            return Config{}, err
        }
        var cfg Config
        if err := yaml.Unmarshal(data, &cfg); err != nil {
            return Config{}, err
        }
        return cfg, nil
    }

    cfg, err := loadConfigFromFile("/etc/app/app.yaml")

  КОГДА ЧТО:
    ENV:
      • Мало простых параметров.
      • Один сервис — несколько параметров.
      • Нет сложной структуры.

    Файл:
      • Сложная структура (вложенные поля).
      • Конфиг для нескольких сервисов.
      • Возможность hot reload через перечитывание файла.
  ПРАКТИКА: оба.

    Простые параметры — через env.
    Сложный конфиг — через файл.

  ПРАВИЛО:
    • НИКОГДА не хардкодь URL БД, пороги, настройки.
    • Всё — из env или файла.
    • Дефолты в коде на случай отсутствия env.
    • Валидация при старте — падать, если конфиг неверный.

  ПРИМЕР С ДЕФОЛТАМИ:
    func env(key, fallback string) string {
        if v := os.Getenv(key); v != "" {
            return v
        }
        return fallback
    }

    cfg := Config{
        AppEnv:   env("APP_ENV", "development"),
        LogLevel: env("LOG_LEVEL", "info"),
        DBHost:   env("DB_HOST", "localhost"),
        DBPort:   env("DB_PORT", "5432"),
    }

  14. АНТИПАТТЕРНЫ

  14.1. СЕКРЕТЫ В CONFIGMAP.
    Пароли в открытом виде. Только Secret.

  14.2. КОНФИГ В ОБРАЗЕ.
    Образ для каждого окружения. Нарушает immutable.
    ConfigMap снаружи.

  14.3. ИМПЕРАТИВНОЕ СОЗДАНИЕ.
    kubectl create configmap без манифеста. Не воспроизводится.

  14.4. ОДИН БОЛЬШОЙ CONFIGMAP.
    100 ключей на всё. Сложно поддерживать. По сервисам.

  14.5. SUBPATH МОНТИРОВАНИЕ.
    Файлы не обновляются при изменении ConfigMap.

  14.6. НЕТ ROLLOUT RESTART ПОСЛЕ ОБНОВЛЕНИЯ.
    ConfigMap обновился, Pod'ы работают со старым env.

  14.7. IMMUTABLE БЕЗ ПРИЧИНЫ.
    Хочешь горячо менять — не делай immutable.

  14.8. CONFIGMAP > 1 МБ.
    Не создастся. etcd не примет.

  14.9. CONFIGMAP ДЛЯ ДАННЫХ.
    ConfigMap — для конфигов. Данные — в volumes/БД.

  14.10. ЖДАТЬ HOT RELOAD ОТ ENV.
    Env не обновляется. Только volume. И то нужен
    reload в коде.

  14.11. НЕТ ВАЛИДАЦИИ КОНФИГА.
    Опечатка в env → сервис стартует с неверным конфигом.
    Валидируй при старте, падай с ошибкой.

  14.12. CONFIGMAP В ДРУГОМ NAMESPACE.
    Должен быть в том же namespace, что Pod.

  15. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  ConfigMap — конфиги отдельно от образа. Один образ —
      много окружений.
  2.  Создание: декларативно через манифест. Не через
      kubectl create.
  3.  Использование: env (envFrom/env), volume (файлы).
  4.  ENV: не обновляется при изменении ConfigMap. Нужен
      rollout restart.
  5.  VOLUME: обновляется автоматически. Приложение должно
      перечитывать файл.
  6.  subPath mount — не обновляется. Частая ловушка.
  7.  Immutable: нельзя изменить, только удалить и создать.
      Экономия на watch в больших кластерах.
  8.  Размер: 1 МБ максимум. Больше — разбить.
  9.  ConfigMap для не-секретных данных. Secret для паролей.
  10. В Go: env через os.Getenv, файл через yaml.Unmarshal.
      Дефолты на случай отсутствия.
  11. Валидируй конфиг при старте. Падай с понятной ошибкой.
  12. Антипаттерны: секреты в ConfigMap, конфиг в образе,
      subPath, нет rollout после обновления, один большой
      ConfigMap.
*/
