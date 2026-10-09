package main

/*
  УРОК 8.3: HELM
  Ты написал Go-сервис, собрал образ, сделал Deployment, Service, ConfigMap, Secret, Ingress. Всё в одном YAML.
  А теперь нужно задеплоить это в dev, staging, prod. У каждого — свой namespace, свои ресурсы, свои домены, свои секреты.
  Копировать YAML и менять руками — путь в ад. Helm решает это: шаблоны + values. Один чарт — много окружений.
  Helm — это «пакетный менеджер для K8s». Как apt для Linux, но для YAML.

  СОДЕРЖАНИЕ:
    1.  Зачем нужен Helm
    2.  Chart, release, repo
    3.  Установка Helm
    4.  Структура чарта
    5.  Chart.yaml — метаданные
    6.  values.yaml — значения по умолчанию
    7.  templates/ — Go-шаблоны
    8.  Встроенные объекты: .Release, .Values, .Chart
    9.  _helpers.tpl — общие шаблоны
    10. Функции: default, quote, include, tpl
    11. Control flow: if, range, with
    12. Dependencies — subcharts
    13. Release: install, upgrade, rollback, uninstall
    14. Hooks: pre-install, post-upgrade
    15. Helm в CI/CD
    16. Helm vs Kustomize
    17. Практика: чарт для Go-сервиса
    18. Связь с Go
    19. Антипаттерны
    20. Финальные выводы

  1. ЗАЧЕМ НУЖЕН HELM
  ПРОБЛЕМА: у тебя Go-сервис. В K8s он требует 5-6 объектов: Deployment, Service, ConfigMap, Secret, Ingress, HPA.
  В dev — 2 реплики, домен dev.example.com, маленькие limits. В prod — 5 реплик, домен api.example.com, большие limits, TLS.
  Копировать YAML и менять руками — 3 версии файлов, которые разъезжаются. Опечатки в проде. Боль при откате.

  РЕШЕНИЕ: Helm.
    • Один набор шаблонов на все окружения.
    • Разные values.yaml для dev/staging/prod.
    • Одной командой — install, upgrade, rollback.
    • Вся история релизов хранится в K8s (Secret'ах).

  ЧТО ЭТО ДАЁТ:
    • Один чарт — много окружений.
    • Версионирование конфигурации.
    • Откат одной командой.
    • Зависимости (Postgres, Redis как subcharts).
    • Шаблонизация: не копипаста, а переиспользование.

  ВАЖНО: Helm — не единственный инструмент. Есть Kustomize (встроен в kubectl), Jsonnet, cdk8s.
  Но Helm — стандарт де-факто в K8s-мире. Большинство публичных чартов — в Helm.

  2. CHART, RELEASE, REPO
  CHART — пакет. Директория с шаблонами, values, метаданными.
    Пример: my-app/ — chart. Внутри: Chart.yaml, values.yaml, templates/.
    Аналогия: .deb или .rpm пакет. Или npm-пакет. Или Go-модуль.

  RELEASE — установленный экземпляр чарта в кластере.
    Пример: helm install my-app ./chart — создаёт release с именем my-app.
    Один чарт можно установить много раз с разными именами: my-app-dev, my-app-prod.

  REPO — репозиторий чартов.
    Пример: Bitnami, Prometheus Community, Grafana.
    helm repo add bitnami https://charts.bitnami.com/bitnami
    helm install postgres bitnami/postgresql

  КАК ЭТО РАБОТАЕТ ВМЕСТЕ:
    • Пишешь чарт для своего Go-сервиса.
    • Устанавливаешь его в кластер: helm install.
    • Helm рендерит шаблоны с values, применяет через kubectl.
    • Сохраняет release в Secret (sh.helm.release.v1.<name>.v<N>).
    • При upgrade — рендерит заново, применяет diff.
    • При rollback — берёт предыдущую версию release из Secret.

  3. УСТАНОВКА HELM

  macOS:
    brew install helm

  Linux:
    curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

  ПРОВЕРИТЬ:
    helm version

  ВАЖНО: helm 3 vs helm 2.
    Helm 2 использовал Tiller (серверную часть в кластере). Был небезопасен.
    Helm 3 — только клиент. Release хранится в Secret'ах. RBAC работает как обычно.
    В 2024 — только Helm 3. Helm 2 устарел.

  ОСНОВНЫЕ КОМАНДЫ:
    helm install <name> <chart>            — установить
    helm upgrade <name> <chart>            — обновить
    helm rollback <name> <revision>        — откатить
    helm uninstall <name>                  — удалить
    helm list                              — список релизов
    helm status <name>                     — статус релиза
    helm history <name>                    — история ревизий
    helm template <name> <chart>           — отрендерить локально
    helm lint <chart>                      — проверить чарт
    helm repo add <name> <url>             — добавить репо
    helm repo update                       — обновить репо

  4. СТРУКТУРА ЧАРТА

  Минимальный чарт:
    my-app/
    ├── Chart.yaml
    ├── values.yaml
    └── templates/
        ├── deployment.yaml
        ├── service.yaml
        └── _helpers.tpl

  Полная структура:
    my-app/
    ├── Chart.yaml              — метаданные чарта.
    ├── values.yaml             — значения по умолчанию.
    ├── values.schema.json      — JSON Schema для валидации.
    ├── .helmignore             — что не паковать.
    ├── charts/                 — subcharts (dependencies).
    ├── crds/                   — CRD для установки до шаблонов.
    └── templates/
        ├── NOTES.txt           — сообщение после install.
        ├── _helpers.tpl        — общие шаблоны.
        ├── deployment.yaml
        ├── service.yaml
        ├── configmap.yaml
        ├── secret.yaml
        ├── ingress.yaml
        ├── hpa.yaml
        └── tests/
            └── test-connection.yaml

  ВАЖНО:
    • Файлы, начинающиеся с _ — не рендерятся в манифесты. Только helpers.
    • NOTES.txt — выводится после install/upgrade.
    • tests/ — тесты, запускаются через helm test.

  СОЗДАТЬ ЧАРТ:
    helm create my-app
    helm lint my-app

  5. CHART.YAML — МЕТАДАННЫЕ
  Chart.yaml описывает чарт.

  МИНИМАЛЬНЫЙ:
    apiVersion: v2
    name: my-app
    description: My Go service
    type: application
    version: 0.1.0
    appVersion: "1.0.0"

  ПОЛЯ:
    apiVersion: v2               — формат Chart.yaml. Для Helm 3 — v2.
    name: my-app                 — имя чарта.
    type: application            — application или library.
    version: 0.1.0               — версия ЧАРТА (semver).
    appVersion: "1.0.0"          — версия приложения (строкой).
    dependencies:                — список зависимостей.
    maintainers / keywords / home / sources / icon.

  РАЗНИЦА version vs appVersion:
    version — версия ЧАРТА. Меняется при изменении шаблонов.
    appVersion — версия ПРИЛОЖЕНИЯ. Меняется при релизе Go-сервиса.
    В шаблонах можно использовать .Chart.AppVersion для тега образа.

  ПРИМЕР С ЗАВИСИМОСТЯМИ:
    apiVersion: v2
    name: my-app
    version: 0.1.0
    appVersion: "1.0.0"
    dependencies:
    - name: postgresql
      version: "15.x.x"
      repository: https://charts.bitnami.com/bitnami
      condition: postgresql.enabled

  condition — включает/выключает subchart по значению values.

  6. VALUES.YAML — ЗНАЧЕНИЯ ПО УМОЛЧАНИЮ
  values.yaml — дефолтные значения, которые подставляются в шаблоны.

  ПРИМЕР:
    replicaCount: 2

    image:
      repository: my-app
      tag: "1.0.0"
      pullPolicy: IfNotPresent

    service:
      type: ClusterIP
      port: 80
      targetPort: 8080

    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
      limits:
        cpu: "500m"
        memory: "256Mi"

    config:
      appEnv: production
      logLevel: info

    ingress:
      enabled: false
      className: nginx
      host: api.example.com
      tls: false

  КАК ИСПОЛЬЗУЕТСЯ:
    В шаблонах — через .Values.<path>. Например, .Values.replicaCount, .Values.image.tag.
    При install — можно переопределить: helm install my-app ./chart --set replicaCount=3.
    Или через файл: helm install my-app ./chart -f prod-values.yaml.

  ПРИОРИТЕТ VALUES (от высшего к низшему):
    1. --set (или --set-string, --set-file).
    2. -f prod-values.yaml (последний указанный — высший приоритет).
    3. values.yaml чарта.
    4. values.yaml subchart'ов.

  СОВЕТ: не переопределяй всё через --set. Для сложных values — отдельный файл -f.

  ПРИМЕР prod-values.yaml:
    replicaCount: 5
    image:
      tag: "2.0.0"
    resources:
      requests:
        cpu: "500m"
        memory: "512Mi"
    ingress:
      enabled: true
      host: api.example.com
      tls: true

  7. TEMPLATES/ — GO-ШАБЛОНЫ
  Шаблоны — это Go text/template с дополнительными функциями от Helm (Sprig).

  ПРОСТОЙ ШАБЛОН (templates/deployment.yaml):
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: {{ .Release.Name }}-app
      namespace: {{ .Release.Namespace }}
      labels:
        app: {{ .Release.Name }}
    spec:
      replicas: {{ .Values.replicaCount }}
      selector:
        matchLabels:
          app: {{ .Release.Name }}
      template:
        metadata:
          labels:
            app: {{ .Release.Name }}
        spec:
          containers:
          - name: app
            image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
            ports:
            - containerPort: {{ .Values.service.targetPort }}
            resources:
              {{- toYaml .Values.resources | nindent 14 }}

  ЧТО ЗДЕСЬ:
    {{ .Release.Name }} — имя релиза (из helm install my-app).
    {{ .Release.Namespace }} — namespace релиза.
    {{ .Values.replicaCount }} — значение из values.yaml.
    {{- toYaml ... | nindent 14 }} — объект в YAML с отступом.
    {{- ... }} — минус убирает перевод строки перед шаблоном.

  ЧТО ТАКОЕ {{- И -}}:
    {{- убирает whitespace ПЕРЕД шаблоном.
    -}} убирает whitespace ПОСЛЕ шаблона.
    Без них в YAML будут лишние пустые строки и сломанные отступы.

  ПРАВИЛО: в шаблонах активно используй {{- и -}} для контроля whitespace. Иначе YAML будет кривой.

  8. ВСТРОЕННЫЕ ОБЪЕКТЫ: .RELEASE, .VALUES, .CHART
  Helm передаёт в шаблоны несколько объектов.

  .Release — информация о релизе:
    .Release.Name          — имя релиза (my-app).
    .Release.Namespace     — namespace (demo).
    .Release.IsInstall     — true если install.
    .Release.IsUpgrade     — true если upgrade.
    .Release.Revision      — номер ревизии (1, 2, 3...).
    .Release.Service       — всегда "Helm".

  .Values — значения из values.yaml:
    .Values.replicaCount
    .Values.image.repository
    .Values.service.port

  .Chart — метаданные из Chart.yaml:
    .Chart.Name            — имя чарта.
    .Chart.Version         — версия чарта.
    .Chart.AppVersion      — версия приложения.

  .Capabilities — информация о кластере:
    .Capabilities.KubeVersion          — версия K8s.
    .Capabilities.APIVersions          — доступные API-версии.

    ПРИМЕР:
      {{- if .Capabilities.APIVersions.Has "networking.k8s.io/v1" }}
      apiVersion: networking.k8s.io/v1
      {{- end }}

  .Files — доступ к файлам чарта:
    {{ .Files.Get "config/something.conf" }}

  ПРАВИЛО: .Release.Name используй для имён объектов. Это позволяет ставить несколько релизов одного чарта в один namespace.

  9. _HELPERS.TPL — ОБЩИЕ ШАБЛОНЫ
  _helpers.tpl — файл с переиспользуемыми шаблонами. Не рендерится в манифесты, только определяет функции.

  ПРИМЕР:
    {{- define "my-app.name" -}}
    {{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
    {{- end }}

    {{- define "my-app.fullname" -}}
    {{- if .Values.fullnameOverride }}
    {{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
    {{- else }}
    {{- printf "%s-%s" .Release.Name (include "my-app.name" .) | trunc 63 | trimSuffix "-" }}
    {{- end }}
    {{- end }}

    {{- define "my-app.labels" -}}
    helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
    app.kubernetes.io/name: {{ include "my-app.name" . }}
    app.kubernetes.io/instance: {{ .Release.Name }}
    app.kubernetes.io/managed-by: {{ .Release.Service }}
    {{- end }}

    {{- define "my-app.selectorLabels" -}}
    app.kubernetes.io/name: {{ include "my-app.name" . }}
    app.kubernetes.io/instance: {{ .Release.Name }}
    {{- end }}

  КАК ИСПОЛЬЗУЕТСЯ:
    metadata:
      name: {{ include "my-app.fullname" . }}
      labels:
        {{- include "my-app.labels" . | nindent 4 }}

  ПОЧЕМУ include, А НЕ template:
    include возвращает строку, можно использовать с pipe (| nindent, | trim).
    template только вставляет, не возвращает.

  ПРАВИЛО: в _helpers.tpl определяй .fullname, .labels, .selectorLabels. Используй везде. Это стандарт Helm-чартов.

  10. ФУНКЦИИ: DEFAULT, QUOTE, INCLUDE, TPL
  Helm добавляет к Go-шаблонам библиотеку функций Sprig (более 100 функций).

  ОСНОВНЫЕ:
    default — значение по умолчанию:
      {{ .Values.image.tag | default .Chart.AppVersion }}

    quote — обернуть в кавычки:
      {{ .Values.image.tag | quote }}      # "1.0.0"

    include — вставить шаблон:
      {{ include "my-app.labels" . | nindent 4 }}

    tpl — рендерить строку как шаблон:
      {{ tpl .Values.someString . }}

    toYaml — объект в YAML:
      {{ toYaml .Values.resources | nindent 14 }}

    nindent — добавить отступ + перевод строки:
      {{ include "..." . | nindent 4 }}

    trunc — обрезать до N символов:
      {{ .Release.Name | trunc 63 }}

    contains — проверка подстроки:
      {{ if contains .Release.Name .Chart.Name }}

    ternary — тернарный оператор:
      {{ ternary "yes" "no" .Values.enabled }}

  ПРАВИЛО: активно используй default и quote. Первый — для fallback, второй — для защиты от YAML-инъекций (пароли, токены).

  11. CONTROL FLOW: IF, RANGE, WITH
  IF — условная логика:
    {{- if .Values.ingress.enabled }}
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    ...
    {{- end }}

  IF/ELSE:
    {{- if .Values.ingress.tls }}
    tls:
    - hosts:
      - {{ .Values.ingress.host }}
      secretName: {{ .Release.Name }}-tls
    {{- end }}

  WITH — смена контекста:
    {{- with .Values.nodeSelector }}
    nodeSelector:
      {{- toYaml . | nindent 8 }}
    {{- end }}

    Внутри with контекст . = .Values.nodeSelector.

  RANGE — цикл по списку:
    {{- range .Values.env }}
    - name: {{ .name }}
      value: {{ .value | quote }}
    {{- end }}

  RANGE ПО MAP:
    {{- range $key, $value := .Values.config }}
    {{ $key }}: {{ $value | quote }}
    {{- end }}

  ПЕРЕМЕННЫЕ:
    {{- $fullName := include "my-app.fullname" . }}
    name: {{ $fullName }}

  ВАЖНО: whitespace control. При range/if используй {{- и -}} постоянно, иначе YAML будет сломан.

  ПРАВИЛО: если что-то опционально (Ingress, HPA, ServiceAccount, NodeSelector) — оборачивай в if. Не рендери пустые секции.

  12. DEPENDENCIES — SUBCHARTS
  Твой чарт может зависеть от других чартов. Например, Postgres, Redis, MinIO.

  КАК ДОБАВИТЬ:
    В Chart.yaml:
      dependencies:
      - name: postgresql
        version: "15.x.x"
        repository: https://charts.bitnami.com/bitnami
        condition: postgresql.enabled

    Затем:
      helm dependency update ./my-app
      # Скачает subchart в charts/.

  КАК РАБОТАЕТ:
    • Subchart устанавливается вместе с основным.
    • Values subchart можно переопределить в values.yaml основного чарта.
    • Ключ — имя subchart.

    ПРИМЕР values.yaml:
      postgresql:
        enabled: true
        auth:
          username: app
          password: supersecret
          database: app
        primary:
          persistence:
            size: 10Gi

  ЧТО ЭТО ДАЁТ:
    • Не надо писать StatefulSet для Postgres руками.
    • Bitnami поддерживает актуальные чарты.
    • Обновление — через helm dependency update.

  МИНУСЫ:
    • Много «магии» в values.
    • Subchart может обновиться и сломать совместимость.

  ПРАВИЛО: для прода БД чаще выносят в Managed (RDS). Subchart — для dev/staging или когда Managed недоступен.

  13. RELEASE: INSTALL, UPGRADE, ROLLBACK, UNINSTALL

  УСТАНОВКА:
    helm install my-app ./chart -n demo --create-namespace
    helm install my-app ./chart -n demo -f prod-values.yaml
    helm install my-app ./chart -n demo --set replicaCount=3

    Что происходит:
      • Helm рендерит шаблоны.
      • Применяет манифесты.
      • Создаёт Secret с release-информацией.

  СПИСОК РЕЛИЗОВ:
    helm list -n demo
    helm list -A

  СТАТУС:
    helm status my-app -n demo

  ИСТОРИЯ:
    helm history my-app -n demo
    # REVISION  STATUS      CHART        APP VERSION  DESCRIPTION
    # 1         superseded  my-app-0.1.0 1.0.0        Install complete
    # 2         deployed    my-app-0.1.0 2.0.0        Upgrade complete

  ОБНОВЛЕНИЕ:
    helm upgrade my-app ./chart -n demo -f prod-values.yaml
    helm upgrade my-app ./chart -n demo --set image.tag=2.0.0

    Что происходит:
      • Helm рендерит заново.
      • Diff с текущим.
      • Применяет изменения.
      • Создаёт новую ревизию.

  UPGRADE --INSTALL:
    helm upgrade --install my-app ./chart -n demo
    Если релиза нет — install. Если есть — upgrade. Удобно в CI.

  ОТКАТ:
    helm rollback my-app 1 -n demo
    helm rollback my-app -n demo
    # Возвращает предыдущую.
    # Откат — это тоже upgrade. Создаёт новую ревизию с содержимым старой.

  УДАЛЕНИЕ:
    helm uninstall my-app -n demo

    Что происходит:
      • Удаляются все ресурсы из релиза.
      • Release-secret удаляется.
      • PVC, созданные через volumeClaimTemplates, ОСТАЮТСЯ.

  КАК HELM ОТСЛЕЖИВАЕТ РЕСУРСЫ:
    По labels: app.kubernetes.io/managed-by=Helm, meta.helm.sh/release-name=my-app.

  ПРАВИЛО: не редактируй ресурсы, созданные Helm, через kubectl edit. Это создаст drift. Все изменения — через values.yaml и helm upgrade.

  14. HOOKS: PRE-INSTALL, POST-UPGRADE
  Hooks — это ресурсы, которые выполняются в определённые моменты жизненного цикла релиза.

  ВИДЫ ХУКОВ:
    pre-install       — до установки.
    post-install      — после установки.
    pre-upgrade       — до обновления.
    post-upgrade      — после обновления.
    pre-rollback      — до отката.
    post-rollback     — после отката.
    pre-delete        — до удаления.
    post-delete       — после удаления.
    test              — при helm test.

  ПРИМЕР: JOB ДЛЯ МИГРАЦИЙ ПЕРЕД UPGRADE
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: {{ .Release.Name }}-migrate
      annotations:
        "helm.sh/hook": pre-upgrade
        "helm.sh/hook-delete-policy": before-hook-creation
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
          - name: migrate
            image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
            command: ["/server", "migrate"]

  ЧТО ЭТО ЗНАЧИТ:
    • Перед upgrade — запускается Job с миграциями.
    • Helm ждёт его успешного завершения.
    • Если Job падает — upgrade не начнётся.
    • После успеха Job удаляется.

  HOOK-DELETE-POLICY:
    before-hook-creation — удалить предыдущий hook перед новым.
    hook-succeeded — удалить после успеха.
    hook-failed — удалить после провала.

  ПРАВИЛО: миграции через pre-upgrade hook. Не в init-контейнере основного Deployment (3 реплики запустят 3 миграции).

  15. HELM В CI/CD

  Типичный pipeline:
    # 1. Проверить чарт
    helm lint ./chart
    # 2. Отрендерить локально (dry-run)
    helm template my-app ./chart -f prod-values.yaml > rendered.yaml
    # 3. Diff с текущим (плагин helm-diff)
    helm diff upgrade my-app ./chart -n demo -f prod-values.yaml
    # 4. Upgrade
    helm upgrade --install my-app ./chart -n demo -f prod-values.yaml --wait --atomic
    # 5. Проверить статус
    helm status my-app -n demo

  ФЛАГ --WAIT:
    Ждать, пока все Pod'ы станут Ready. Если не готовы за timeout — upgrade фейлится.

  ФЛАГ --TIMEOUT:
    helm upgrade --install --timeout=5m ...

  ФЛАГ --ATOMIC:
    Если upgrade падает — Helm автоматически откатывает.

  В GITOPS:
    ArgoCD, Flux умеют работать с Helm-чартами.
    ArgoCD рендерит чарт и применяет. Diff с git.
    Helm всё ещё нужен для локальной разработки и CI.

  ПРАВИЛО: в CI/CD — helm upgrade --install --atomic --wait. Атомарно, с ожиданием, с автооткатом при провале.

  16. HELM VS KUSTOMIZE
  Два инструмента для управления YAML. Решают разные задачи.

  HELM:
    • Шаблоны + values.
    • Мощная шаблонизация (Go templates, функции).
    • Релизы, откаты, hooks.
    • Публичные чарты (Bitnami, Prometheus Community).
    • Сложнее читать (Go templates).

  KUSTOMIZE:
    • Overlays поверх базовых YAML.
    • Без шаблонов — просто patch.
    • Встроен в kubectl (kubectl apply -k).
    • Проще читать (обычный YAML).
    • Нет релизов и откатов.

  КОГДА ЧТО:
    • Helm — если нужны шаблоны с логикой, релизы, публичные чарты.
    • Kustomize — если YAML почти одинаковый, отличаются только мелочи.
    • Комбинация — Helm для шаблонов + Kustomize для overlay.

  ПРАВИЛО: для Go-сервиса — Helm. Для инфры (Prometheus, Istio, cert-manager) — Helm-чарты. Для overlay dev/prod одного сервиса — можно Kustomize.

  17. ПРАКТИКА: ЧАРТ ДЛЯ GO-СЕРВИСА
  Соберём чарт для Go-сервиса

  СТРУКТУРА:
    my-app/
    ├── Chart.yaml
    ├── values.yaml
    ├── prod-values.yaml
    ├── .helmignore
    └── templates/
        ├── _helpers.tpl
        ├── NOTES.txt
        ├── deployment.yaml
        ├── service.yaml
        ├── configmap.yaml
        ├── secret.yaml
        └── ingress.yaml

  CHART.YAML:
    apiVersion: v2
    name: my-app
    description: My Go service
    type: application
    version: 0.1.0
    appVersion: "1.0.0"

  VALUES.YAML:
    replicaCount: 2

    image:
      repository: my-app
      tag: ""
      pullPolicy: IfNotPresent

    service:
      type: ClusterIP
      port: 80
      targetPort: 8080

    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
      limits:
        cpu: "500m"
        memory: "256Mi"

    config:
      appEnv: production
      logLevel: info

    secrets:
      dbUser: app
      dbPassword: ""

    ingress:
      enabled: false
      className: nginx
      host: api.example.com
      tls: false

  _HELPERS.TPL:
    {{- define "my-app.name" -}}
    {{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
    {{- end }}

    {{- define "my-app.fullname" -}}
    {{- if .Values.fullnameOverride }}
    {{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
    {{- else }}
    {{- printf "%s-%s" .Release.Name (include "my-app.name" .) | trunc 63 | trimSuffix "-" }}
    {{- end }}
    {{- end }}

    {{- define "my-app.labels" -}}
    helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
    app.kubernetes.io/name: {{ include "my-app.name" . }}
    app.kubernetes.io/instance: {{ .Release.Name }}
    app.kubernetes.io/managed-by: {{ .Release.Service }}
    {{- end }}

    {{- define "my-app.selectorLabels" -}}
    app.kubernetes.io/name: {{ include "my-app.name" . }}
    app.kubernetes.io/instance: {{ .Release.Name }}
    {{- end }}

  TEMPLATES/DEPLOYMENT.YAML:
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: {{ include "my-app.fullname" . }}
      namespace: {{ .Release.Namespace }}
      labels:
        {{- include "my-app.labels" . | nindent 4 }}
    spec:
      replicas: {{ .Values.replicaCount }}
      selector:
        matchLabels:
          {{- include "my-app.selectorLabels" . | nindent 6 }}
      template:
        metadata:
          labels:
            {{- include "my-app.selectorLabels" . | nindent 8 }}
        spec:
          terminationGracePeriodSeconds: 30
          containers:
          - name: app
            image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
            imagePullPolicy: {{ .Values.image.pullPolicy }}
            ports:
            - name: http
              containerPort: {{ .Values.service.targetPort }}
            envFrom:
            - configMapRef:
                name: {{ include "my-app.fullname" . }}-config
            - secretRef:
                name: {{ include "my-app.fullname" . }}-secret
            env:
            - name: GOMEMLIMIT
              value: "230MiB"
            resources:
              {{- toYaml .Values.resources | nindent 14 }}
            readinessProbe:
              httpGet:
                path: /ready
                port: {{ .Values.service.targetPort }}
              initialDelaySeconds: 3
              periodSeconds: 5
            livenessProbe:
              httpGet:
                path: /health
                port: {{ .Values.service.targetPort }}
              initialDelaySeconds: 10
              periodSeconds: 10

  TEMPLATES/SERVICE.YAML:
    apiVersion: v1
    kind: Service
    metadata:
      name: {{ include "my-app.fullname" . }}
      namespace: {{ .Release.Namespace }}
      labels:
        {{- include "my-app.labels" . | nindent 4 }}
    spec:
      type: {{ .Values.service.type }}
      selector:
        {{- include "my-app.selectorLabels" . | nindent 6 }}
      ports:
      - name: http
        port: {{ .Values.service.port }}
        targetPort: {{ .Values.service.targetPort }}

  TEMPLATES/CONFIGMAP.YAML:
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: {{ include "my-app.fullname" . }}-config
      namespace: {{ .Release.Namespace }}
      labels:
        {{- include "my-app.labels" . | nindent 4 }}
    data:
      APP_ENV: {{ .Values.config.appEnv | quote }}
      LOG_LEVEL: {{ .Values.config.logLevel | quote }}

  TEMPLATES/SECRET.YAML:
    apiVersion: v1
    kind: Secret
    metadata:
      name: {{ include "my-app.fullname" . }}-secret
      namespace: {{ .Release.Namespace }}
      labels:
        {{- include "my-app.labels" . | nindent 4 }}
    type: Opaque
    stringData:
      DB_USER: {{ .Values.secrets.dbUser | quote }}
      DB_PASSWORD: {{ .Values.secrets.dbPassword | quote }}

  TEMPLATES/INGRESS.YAML:
    {{- if .Values.ingress.enabled }}
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: {{ include "my-app.fullname" . }}
      namespace: {{ .Release.Namespace }}
      labels:
        {{- include "my-app.labels" . | nindent 4 }}
    spec:
      ingressClassName: {{ .Values.ingress.className }}
      rules:
      - host: {{ .Values.ingress.host }}
        http:
          paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ include "my-app.fullname" . }}
                port:
                  number: {{ .Values.service.port }}
      {{- if .Values.ingress.tls }}
      tls:
      - hosts:
        - {{ .Values.ingress.host }}
        secretName: {{ include "my-app.fullname" . }}-tls
      {{- end }}
    {{- end }}

  PROD-VALUES.YAML:
    replicaCount: 5

    image:
      tag: "2.0.0"

    resources:
      requests:
        cpu: "500m"
        memory: "512Mi"
      limits:
        cpu: "2"
        memory: "1Gi"

    config:
      appEnv: production
      logLevel: warn

    secrets:
      dbUser: app
      dbPassword: "prod-secret-123"

    ingress:
      enabled: true
      className: nginx
      host: api.example.com
      tls: true

  КОМАНДЫ:
    # Установить в dev
    helm install my-app-dev ./my-app -n dev --create-namespace
    # Установить в prod
    helm install my-app-prod ./my-app -n prod --create-namespace -f prod-values.yaml
    # Обновить prod
    helm upgrade my-app-prod ./my-app -n prod -f prod-values.yaml --atomic --wait
    # История
    helm history my-app-prod -n prod
    # Откат
    helm rollback my-app-prod 1 -n prod
    # Удалить
    helm uninstall my-app-dev -n dev

  18. СВЯЗЬ С GO

  Что нужно от Go-разработчика для Helm:
  1. ПОНИМАНИЕ YAML И K8S-ОБЪЕКТОВ.
     Helm — это шаблонизация YAML. Без знания K8s — никак.
  2. VERSIONING.
     appVersion в Chart.yaml = версия образа.
     Обновляешь Go-сервис — обновляешь appVersion.
  3. GOMEMLIMIT.
     В чарте ставится как env. Должен быть на 10-20% меньше limits.memory.
  4. /HEALTH И /READY.
     Используются в probes. Продумай их заранее.
  5. GRACEFUL SHUTDOWN.
     terminationGracePeriodSeconds в чарте > preStop + shutdown time.
  6. НЕ ХАРДКОДИТЬ КОНФИГИ.
     Всё через env. Конфиги — через ConfigMap/Secret в чарте.

  ПРАКТИЧЕСКИЙ СЦЕНАРИЙ:
    Ты добавил фичу. Выпустил версию 1.1.0.
      1. Собрать образ: docker build -t my-app:1.1.0 .
      2. Запушить в registry.
      3. helm upgrade my-app ./chart -n prod -f prod-values.yaml --set image.tag=1.1.0.
      4. Проверить: helm status, kubectl get pods.

  ВАЖНО: не меняй values.yaml руками в проде. Values — в git. Upgrade — через CI/CD.

  19. АНТИПАТТЕРНЫ
  19.1. ХАРДКОД В ШАБЛОНАХ. Всё через .Values.
  19.2. НЕТ _HELPERS.TPL. Дублирование .fullname и labels в каждом файле.
  19.3. values.yaml НА 2000 СТРОК. Раздели на чарты.
  19.4. helm upgrade БЕЗ --atomic. Провал — прод в полусломанном состоянии.
  19.5. helm upgrade БЕЗ --wait. Не ждём готовности Pod'ов.
  19.6. LATEST В ОБРАЗЕ. tag должен быть фиксирован.
  19.7. ИЗМЕНЕНИЕ РЕСУРСОВ ЧЕРЕЗ kubectl edit. Helm потеряет tracking. Drift.
  19.8. СЕКРЕТЫ В values.yaml В GIT. External Secrets или Sealed Secrets.
  19.9. МИГРАЦИИ В INIT-КОНТЕЙНЕРЕ ВМЕСТО HOOK. 3 реплики запустят 3 миграции.
  19.10. УДАЛЕНИЕ ЧАРТА БЕЗ ПОНИМАНИЯ, ЧТО PVC ОСТАНУТСЯ.
  19.11. ВСЁ ЧЕРЕЗ --set. Для сложных values — отдельный -f.
  19.12. НЕТ helm lint В CI.
  19.13. SUBCHART БЕЗ VERSION LOCK.
  19.14. ОДИН ЧАРТ НА ВСЮ ИНФРУ. Prometheus, Postgres, App — разделяй.
  19.15. НЕТ README ДЛЯ ЧАРТА.

  20. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Helm — пакетный менеджер для K8s. Chart + values → release.
  2.  Chart — пакет. Release — экземпляр чарта в кластере. Repo — репозиторий чартов.
  3.  Структура: Chart.yaml, values.yaml, templates/, _helpers.tpl.
  4.  Chart.yaml — метаданные. version (чарт) vs appVersion (приложение).
  5.  values.yaml — дефолтные значения. Переопределяются через -f или --set.
  6.  Templates — Go-шаблоны + Sprig функции. {{ .Values.x }}, {{ .Release.Name }}, {{ include "..." . }}.
  7.  _helpers.tpl — переиспользуемые шаблоны (.fullname, .labels, .selectorLabels).
  8.  Функции: default, quote, toYaml, nindent, trunc, contains, ternary.
  9.  Control flow: if, range, with. Whitespace control {{- и -}}.
  10. Dependencies — subcharts. Postgres, Redis через Bitnami.
  11. Release: install, upgrade, rollback, uninstall. История в Secret'ах.
  12. Hooks: pre-install, post-upgrade, pre-upgrade. Миграции — через pre-upgrade.
  13. CI/CD: helm lint → helm template → helm diff → helm upgrade --install --atomic --wait.
  14. Helm vs Kustomize: Helm для шаблонов и релизов, Kustomize для overlay.
  15. Go-разработчик: понимать YAML, K8s, versioning (appVersion), GOMEMLIMIT, probes, graceful shutdown.
*/
