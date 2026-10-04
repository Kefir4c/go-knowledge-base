package main

/*
  УРОК 4.2: SECRET
  Пароли, токены, сертификаты, SSH-ключи — не должны лежать в
  git, попадать в логи и быть видимыми любому, у кого есть
  доступ к кластеру. ConfigMap для этого не подходит.
  Secret решает это: отдельный объект K8s для чувствительных
  данных. Отдельный RBAC. Опционально — шифрование at rest.

  СОДЕРЖАНИЕ:
    1.  Зачем нужен Secret
    2.  Base64 — это НЕ шифрование
    3.  Типы Secret
    4.  Создание через kubectl
    5.  Создание через манифест
    6.  stringData vs data
    7.  Использование как env
    8.  Использование как volume
    9.  Image pull secrets
    10. TLS secrets
    11. Обновление и rollout
    12. Шифрование at rest
    13. Внешние системы для прода
    14. RBAC и защита
    15. Связь с Go
    16. Антипаттерны
    17. Финальные выводы

  1. ЗАЧЕМ НУЖЕН SECRET

  ПРОБЛЕМА: если пароли положить в ConfigMap:
    • Видны любому, у кого есть доступ к ConfigMap.
    • RBAC не отличается от обычных конфигов.
    • Легко случайно показать в kubectl describe.
    • Нет шифрования at rest по умолчанию.
    • Попадают в логи без предупреждения.

  РЕШЕНИЕ: Secret.
    Отдельный тип объекта. Отдельный RBAC. Шифрование at rest
    (настраивается). Меньше шансов случайно слить.

  ВАЖНО ПОНИМАТЬ:
    Secret — это не «шифрование из коробки». По умолчанию
    данные в Secret хранятся в etcd как base64. Это НЕ
    шифрование.

    Реальная защита:
      • RBAC — ограничить доступ.
      • Encryption at rest — шифровать данные в etcd.
      • External Secrets — не хранить секреты в кластере.

  2. BASE64 — ЭТО НЕ ШИФРОВАНИЕ
  Когда создаёшь Secret, K8s кодирует значения в base64.

  ПРИМЕР:
    stringData:
      DB_PASSWORD: "supersecret"

  Становится:
    data:
      DB_PASSWORD: c3VwZXJzZWNyZXQ=

  ЧТО ЭТО ЗНАЧИТ:
    Кодирование — обратимое преобразование. Любой, кто видит
    base64, декодирует его одной командой:

      echo "c3VwZXJzZWNyZXQ=" | base64 -d
      # supersecret

  ПОЧЕМУ ТАК СДЕЛАНО:
    • Base64 — способ передать бинарные данные в текстовом
      формате (YAML, JSON).
    • Не как защита.

  ЧТО ЗАЩИЩАЕТ НА САМОМ ДЕЛЕ:
    • RBAC — ограничивает, кто может читать Secret.
    • Encryption at rest — шифрует данные в etcd.
    • Внешние системы (Vault, External Secrets) — секреты
      вообще не лежат в кластере.

  ВЫВОД: никогда не полагайся на base64 как на защиту.
  Думай о Secret как о «помеченном контейнере для
  чувствительных данных», а не как о сейфе.

  3. ТИПЫ SECRET

  OPAQUE (по умолчанию):
    Обычный key-value. Для паролей, токенов, ключей API.

    type: Opaque
    stringData:
      DB_PASSWORD: "supersecret"
      API_KEY: "sk-1234..."

  KUBERNETES.IO/TLS:
    Для TLS-сертификатов. Обязательно два ключа: tls.crt,
    tls.key.

    type: kubernetes.io/tls
    data:
      tls.crt: <base64-cert>
      tls.key: <base64-key>

    Используется в Ingress для HTTPS.

  KUBERNETES.IO/DOCKERCONFIGJSON:
    Для доступа к приватному Docker registry. Содержит JSON
    с credentials.

    type: kubernetes.io/dockerconfigjson
    data:
      .dockerconfigjson: <base64-json>

    Используется в Pod'ах через imagePullSecrets.

  KUBERNETES.IO/BASIC-AUTH:
    Для basic-auth. Два ключа: username, password.

  KUBERNETES.IO/SSH-AUTH:
    Для SSH-ключей. Один ключ: ssh-privatekey.

  KUBERNETES.IO/SERVICE-ACCOUNT-TOKEN:
    Автоматически создаётся для ServiceAccount. Содержит
    токен для доступа к API Server.

  4. СОЗДАНИЕ ЧЕРЕЗ KUBECTL

  OPAQUE — ИЗ ЛИТЕРАЛОВ:
    kubectl create secret generic app-secrets \
      --from-literal=DB_USER=app \
      --from-literal=DB_PASSWORD=supersecret \
      -n demo

  OPAQUE — ИЗ ФАЙЛОВ:
    kubectl create secret generic app-secrets \
      --from-file=./secrets/api-key.txt \
      -n demo
    Ключ = имя файла. Значение = содержимое.

  OPAQUE — ИЗ ENV-ФАЙЛА:
    kubectl create secret generic app-secrets \
      --from-env-file=.env.secrets \
      -n demo

  TLS:
    kubectl create secret tls api-tls \
      --cert=./tls.crt \
      --key=./tls.key \
      -n demo

    Создаёт Secret типа kubernetes.io/tls.

  DOCKER REGISTRY:
    kubectl create secret docker-registry reg-cred \
      --docker-server=registry.example.com \
      --docker-username=user \
      --docker-password=pass \
      -n demo

    Создаёт kubernetes.io/dockerconfigjson.

  МИНУС ИМПЕРАТИВНОГО ПОДХОДА:
    • Не воспроизводится из git.
    • При пересоздании кластера — заново руками.
    • В проде — только манифесты + external secrets.

  5. СОЗДАНИЕ ЧЕРЕЗ МАНИФЕСТ

  ПРАВИЛЬНЫЙ ПОДХОД В ПРОДЕ:
    apiVersion: v1
    kind: Secret
    metadata:
      name: app-secrets
      namespace: demo
      labels:
        app: app
    type: Opaque
    stringData:
      DB_USER: "app"
      DB_PASSWORD: "supersecret"
      API_KEY: "sk-1234567890abcdef"

  ВАЖНО:
    • Не коммить манифесты с реальными секретами в git.
    • Использовать sealed-secrets или external-secrets.
    • Или зашифрованный git (SOPS, git-crypt).

  6. STRINGDATA VS DATA
  K8s поддерживает два поля для значений.

  STRINGDATA — открытый текст:
    stringData:
      DB_PASSWORD: "supersecret"

    Удобно писать. K8s автоматически кодирует в base64 и
    кладёт в data. НЕ сохраняется в объекте.

  DATA — уже base64:
    data:
      DB_PASSWORD: c3VwZXJzZWNyZXQ=

    Нужно самому закодировать. Полезно, если данные уже
    в base64.

  РЕКОМЕНДАЦИЯ: всегда используй stringData. Меньше ошибок.

  ПРОВЕРКА:
    kubectl get secret app-secrets -o jsonpath='{.stringData}'
    # пусто (stringData не сохраняется)

    kubectl get secret app-secrets -o jsonpath='{.data.DB_PASSWORD}'
    # c3VwZXJzZWNyZXQ=

  7. ИСПОЛЬЗОВАНИЕ КАК ENV

  ENVFROM — все ключи:
    spec:
      containers:
      - name: app
        image: my-app:1.0
        envFrom:
        - secretRef:
            name: app-secrets

    Все ключи Secret → env.

  ENV — выборочно:
    spec:
      containers:
      - name: app
        env:
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: app-secrets
              key: DB_PASSWORD
    Только указанные ключи.

  ЧТО ВАЖНО ПОМНИТЬ ПРО ENV:
    • Env не обновляется при изменении Secret. Нужен рестарт.
    • Env видно в `kubectl describe pod` (в открытом виде).
    • Env передаётся дочерним процессам.
    • Env может утечь в crash-дампы.

  КОГДА ENV ПОДХОДИТ:
    • Малые сервисы.
    • Секреты нужны только при старте.
    • Простота важнее безопасности.

  8. ИСПОЛЬЗОВАНИЕ КАК VOLUME
  Secret монтируется в Pod как файлы.
    spec:
      containers:
      - name: app
        volumeMounts:
        - name: secrets
          mountPath: /etc/secrets
          readOnly: true
      volumes:
      - name: secrets
        secret:
          secretName: app-secrets
          defaultMode: 0400

  КАЖДЫЙ КЛЮЧ = ФАЙЛ:
    /etc/secrets/DB_USER          → app
    /etc/secrets/DB_PASSWORD      → supersecret
    /etc/secrets/API_KEY          → sk-1234...

  ПРАВА ДОСТУПА:
    defaultMode: 0400 — только владелец читает.

  ВЫБОРОЧНО:
    volumes:
    - name: secrets
      secret:
        secretName: app-secrets
        items:
        - key: DB_PASSWORD
          path: db_password.txt
    Монтирует только выбранные ключи.

  ВАЖНО ПРО ОБНОВЛЕНИЕ:
    Volume обновляется автоматически при изменении Secret.
    Задержка 30-60 сек (kubelet sync period).
    НО приложение должно перечитывать файлы.

  SUBPATH:
    subPath mount НЕ обновляется. Частая ловушка.

  9. IMAGE PULL SECRETS
  Для скачивания образов из приватного registry нужны
  credentials.

  СОЗДАТЬ SECRET:
    kubectl create secret docker-registry reg-cred \
      --docker-server=registry.example.com \
      --docker-username=user \
      --docker-password=pass \
      -n demo

  ИСПОЛЬЗОВАТЬ В POD:
    spec:
      imagePullSecrets:
      - name: reg-cred
      containers:
      - name: app
        image: registry.example.com/my-app:1.0

  KUBELET САМ ИСПОЛЬЗУЕТ:
    При скачивании образа kubelet берёт credentials из Secret.

  АВТОМАТИЧЕСКИ ДЛЯ SERVICEACCOUNT:
    kubectl patch sa default -n demo \
      -p '{"imagePullSecrets":[{"name":"reg-cred"}]}'

    Тогда все Pod'ы автоматически получат доступ.

  10. TLS SECRETS
  Для HTTPS-сертификатов.

  СОЗДАНИЕ:
    kubectl create secret tls api-tls \
      --cert=tls.crt \
      --key=tls.key \
      -n demo

  СТРУКТУРА:
    type: kubernetes.io/tls
    data:
      tls.crt: <base64-cert>
      tls.key: <base64-key>

  ИСПОЛЬЗОВАНИЕ В INGRESS:
    spec:
      tls:
      - hosts:
        - api.example.com
        secretName: api-tls

  ИСПОЛЬЗОВАНИЕ В GO:
    http.ListenAndServeTLS(":443",
      "/etc/tls/tls.crt",
      "/etc/tls/tls.key",
      handler)

  11. ОБНОВЛЕНИЕ И ROLLOUT

  ОБНОВИТЬ SECRET:
    kubectl patch secret app-secrets -n demo \
      -p '{"stringData":{"DB_PASSWORD":"newpassword"}}'

  ЧТО ПРОИСХОДИТ С POD'АМИ:
    ENV: не обновляется. Нужен rollout restart.

      kubectl rollout restart deployment/app -n demo

    VOLUME: файлы обновляются автоматически. Приложение
    должно перечитывать.

  SUBPATH: не обновляется никогда.

  АВТОМАТИЗАЦИЯ:
    • Reloader (stakater/reloader) — следит за Secret
      и перезапускает Deployment.
    • Не встроено в K8s.

  12. ШИФРОВАНИЕ AT REST
  По умолчанию Secret хранится в etcd в base64. Если у
  злоумышленника есть доступ к etcd — он читает все секреты.

  ENCRYPTION AT REST — шифрование данных в etcd.
  КАК НАСТРОИТЬ:

    Создать EncryptionConfiguration:
      apiVersion: apiserver.config.k8s.io/v1
      kind: EncryptionConfiguration
      resources:
      - resources:
        - secrets
        providers:
        - aescbc:
            keys:
            - name: key1
              secret: <base64-32-byte-key>
        - identity: {}

    Указать в API Server флагом:
      --encryption-provider-config=/etc/kubernetes/encryption.yaml

  ПОСЛЕ ЭТОГО:
    Новые Secret шифруются. Старые — пересоздать:

      kubectl get secrets -A -o json | kubectl replace -f -

  В ОБЛАКЕ:
    EKS, GKE, AKS — часто включено по умолчанию.

  13. ВНЕШНИЕ СИСТЕМЫ ДЛЯ ПРОДА

  ПРОБЛЕМА K8S SECRET:
    • Хранится в git (если манифест в git).
    • Хранится в etcd.
    • Нет ротации.
    • Нет аудита.

  РЕШЕНИЯ:

    SEALED SECRETS:
      • Секрет шифруется на клиенте.
      • В git — зашифрованный манифест.
      • В кластере — SealedSecret CRD.
      • Контроллер расшифровывает и создаёт обычный Secret.

    EXTERNAL SECRETS OPERATOR:
      • Секреты хранятся в Vault/AWS Secrets Manager.
      • Оператор синхронизирует их в K8s.
      • Ротация — автоматическая.

    VAULT (HashiCorp):
      • Полноценное хранилище секретов.
      • Ротация, аудит, доступ по ролям.

    SOPS + AGE:
      • Шифрование секретов в git.
      • Расшифровка при apply.

  ЧТО ВЫБРАТЬ:
    • Небольшой проект → Sealed Secrets.
    • Облако → External Secrets Operator.
    • Enterprise → Vault.

  ПРАВИЛО: K8s Secret — это НЕ хранилище секретов. Это
  способ передать секрет из внешней системы в Pod.

  14. RBAC И ЗАЩИТА
  SECRET ЗАЩИЩЁН ЧЕРЕЗ RBAC.

    По умолчанию разработчики имеют доступ к Secret в
    своём namespace.

    Можно ограничить:
      apiVersion: rbac.authorization.k8s.io/v1
      kind: Role
      metadata:
        name: secret-reader
        namespace: demo
      rules:
      - apiGroups: [""]
        resources: ["secrets"]
        verbs: ["get"]
        resourceNames: ["app-secrets"]

  ПРОВЕРИТЬ:
    kubectl auth can-i get secrets -n demo
    # yes/no

  AUDIT LOGGING:
    K8s может логировать доступ к Secret. Настраивается
    в API Server.

  ПРАВИЛА:
    • Не давай всем доступ к Secret.
    • Используй отдельные Secret на сервис.
    • Не логируй Secret.
    • Не передавай Secret через env в прод.

  15. СВЯЗЬ С GO

  ЧЕРЕЗ ENV:

    func loadSecrets() Config {
        return Config{
            DBPassword: os.Getenv("DB_PASSWORD"),
            APIKey:     os.Getenv("API_KEY"),
        }
    }

  ЧЕРЕЗ ФАЙЛ:
    func loadSecretsFromFiles() (Config, error) {
        dbPass, err := os.ReadFile("/etc/secrets/DB_PASSWORD")
        if err != nil {
            return Config{}, err
        }
        apiKey, err := os.ReadFile("/etc/secrets/API_KEY")
        if err != nil {
            return Config{}, err
        }
        return Config{
            DBPassword: strings.TrimSpace(string(dbPass)),
            APIKey:     strings.TrimSpace(string(apiKey)),
        }, nil
    }

  КОГДА ЧТО:
    ENV:
      • Простые секреты.
      • Один раз при старте.

    FILE:
      • Большие секреты (сертификаты, ключи).
      • Ротация без рестарта.
      • Не показывать в describe pod.

  ПРАВИЛА ДЛЯ GO:
    • Не логируй секреты. Даже в debug.
    • Не передавай через командную строку.
    • Не показывай в HTTP-ответах ошибок.
    • Валидируй, что секреты загружены — падай, если нет.

  ПРИМЕР ВАЛИДАЦИИ:
    func loadConfig() (Config, error) {
        dbPass := os.Getenv("DB_PASSWORD")
        if dbPass == "" {
            return Config{}, errors.New("DB_PASSWORD is required")
        }
        apiKey := os.Getenv("API_KEY")
        if apiKey == "" {
            return Config{}, errors.New("API_KEY is required")
        }
        return Config{
            DBPassword: dbPass,
            APIKey:     apiKey,
        }, nil
    }

  HOT RELOAD:
    Если нужен — читай из файла, следи через fsnotify.
    Но проще — rollout restart.

  16. АНТИПАТТЕРНЫ

  16.1. СЕКРЕТЫ В GIT В ОТКРЫТОМ ВИДЕ.
    Даже в приватном репо — плохо. История остаётся.

  16.2. СЕКРЕТЫ В CONFIGMAP.
    Нет RBAC, нет шифрования.

  16.3. ENV В ПРОДЕ ДЛЯ СЕКРЕТОВ.
    Видны в kubectl describe pod.

  16.4. ОДИН SECRET НА ВСЁ.
    Все сервисы имеют доступ ко всему.

  16.5. СЕКРЕТ В ЛОГАХ.
    Случайно залогировал env или config.

  16.6. НЕТ ШИФРОВАНИЯ ETCD.
    Доступ к etcd = все секреты.

  16.7. НЕТ РОТАЦИИ.
    Пароли не меняются годами.

  16.8. SUBPATH ДЛЯ SECRET.
    Не обновляется. Hot reload не работает.

  16.9. СЕКРЕТ В КОМАНДНОЙ СТРОКЕ.
    args: ["--password=secret"]. Видно в ps aux.

  16.10. HTTP ОТВЕТЫ С СЕКРЕТАМИ.
    В ошибках, debug-эндпоинтах.

  16.11. СЕКРЕТ ЧЕРЕЗ ARG В DOCKERFILE.
    ARG виден в docker history.

  16.12. SEALED SECRETS БЕЗ ОТДЕЛЬНОГО РЕПО.
    Ключ расшифровки в том же репо. Смысла нет.

  17. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Secret — объект для паролей, токенов, сертификатов.
  2.  Base64 — это НЕ шифрование. Любой декодирует.
  3.  Типы: Opaque, kubernetes.io/tls, docker-registry,
      basic-auth, ssh-auth.
  4.  stringData — открытый текст при создании. data — base64.
  5.  Использование: env или volume.
  6.  env — проще, но не обновляется, видно в describe pod.
  7.  volume — обновляется автоматически, права 0400.
  8.  Image pull secrets — для приватных registry.
  9.  TLS secrets — для HTTPS. Используются в Ingress и Go-серверах.
  10. Обновление: env — только rollout restart.
      volume — автоматически.
  11. Encryption at rest — шифрование etcd.
  12. Для прода: Sealed Secrets, External Secrets Operator, Vault, SOPS.
  13. RBAC защищает Secret. Ограничивай доступ.
  14. В Go: env через os.Getenv, файл через os.ReadFile.
      Валидируй при старте. Не логируй.
  15. Антипаттерны: секреты в git, в ConfigMap, env в проде,
      subPath, нет ротации, нет шифрования etcd.
*/
