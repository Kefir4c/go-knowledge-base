# Kubernetes Roadmap

## Что ты будешь уметь после этого роадмапа:
- Объяснить архитектуру K8s: control plane, worker nodes, kubelet, API server.
- Понимать Pod, Deployment, Service, Ingress.
- Настроить probes, ресурсы, ConfigMap, Secret.
- Объяснить, чем K8s отличается от Compose.
- Уметь работать с `kubectl` на базовом уровне.
- Знать про namespaces, rolling updates, HPA.
- Диагностировать типовые проблемы (CrashLoopBackOff, OOMKilled).

## 🟢 БЛОК 1: АРХИТЕКТУРА
### 1.1. Control plane vs Worker nodes
**Логика:** K8s — кластер из нод. Control plane управляет, worker nodes запускают приложения.
**Что учить:**
- Control plane: API Server, etcd, Scheduler, Controller Manager.
- Worker node: kubelet, kube-proxy, container runtime (containerd).
- Что происходит при `kubectl apply`: API server → etcd → scheduler → kubelet → контейнер.
  **Связь с Go:** kubelet и API server написаны на Go. `client-go` — библиотека для работы с K8s API из Go.

### 1.2. Основные объекты (обзор)
**Логика:** В K8s всё — объект в API. Понимать, что для чего.
**Что учить:**
- **Pod** — минимальная единица, один или несколько контейнеров.
- **Deployment** — управляет репликами Pod'ов.
- **ReplicaSet** — обеспечивает нужное число Pod'ов (создаётся Deployment'ом).
- **Service** — стабильный сетевой доступ к Pod'ам.
- **Ingress** — HTTP-роутинг снаружи.
- **ConfigMap / Secret** — конфиги и секреты.
- **Namespace** — логическая изоляция.
- **Job / CronJob** — одноразовые и по расписанию задачи.
- **StatefulSet** — для stateful-приложений (БД).

### 1.3. kubectl — базовые команды
**Логика:** kubectl — основной инструмент. 90% работы — 15 команд.
**Что учить:** `kubectl get/describe/logs/exec/apply/delete`, `port-forward`, `scale`, `rollout status/undo/history`. `kubectl get all -n <ns>`. `kubectl config use-context`, `kubectx`/`kubens`.
**Связь с Go:** Из Go-сервиса работа с K8s API — через `client-go`. Для отладки — `kubectl exec -it <pod> -- sh`.

## 🟡 БЛОК 2: POD И РАБОЧИЕ НАГРУЗКИ
### 2.1. Pod — что это
**Логика:** Pod — минимальная единица. Внутри — один или несколько контейнеров, разделяющих network namespace и volumes.
**Что учить:**
- Почему Pod, а не просто контейнер: разделяемая сеть, общие volumes.
- **Sidecar** — вспомогательный контейнер рядом (лог-агент, proxy).
- **Init-контейнеры** — запускаются до основного, для подготовки (например, дождаться БД).
- **Ephemeral containers** — для отладки работающего Pod'а.
  **Связь с Go:** Go-сервис обычно = один контейнер в Pod'е. Init-контейнеры пишут на Go для миграций.

### 2.2. Deployment и ReplicaSet
**Логика:** Deployment — основной способ запускать stateless-приложения. Управляет версиями и репликами.
**Что учить:**
- Иерархия: Deployment → ReplicaSet → Pod.
- `replicas`, `selector`, `template`.
- Стратегия **RollingUpdate**: `maxSurge`, `maxUnavailable`.
- Стратегия **Recreate** (для несовместимых версий, downtime).
- История ревизий и `kubectl rollout undo`.
  **Связь с Go:** Go-сервис деплоится через Deployment. Zero-downtime обновление работает благодаря probes.

### 2.3. StatefulSet и Job
**Логика:** Не всё stateless. Для БД и одноразовых задач — свои контроллеры.
**Что учить:**
- **StatefulSet**: стабильные имена (`db-0`, `db-1`), стабильные volumes, порядок запуска. Для Postgres, Kafka, Elasticsearch.
- **Job**: одноразовая задача (миграции). **CronJob**: по расписанию (бэкапы).
- На практике БД чаще выносят в облачный Managed Postgres, а не крутят StatefulSet руками.
  **Связь с Go:** Миграции в K8s запускают через Job, а не в основном контейнере.

### 2.4. Probes — liveness, readiness, startup
**Логика:** K8s должен знать, жив ли Pod и готов ли принимать трафик.
**Что учить:**
- **livenessProbe** — если падает, Pod перезапускается.
- **readinessProbe** — если падает, Pod убирается из Service endpoints.
- **startupProbe** — для медленно стартующих приложений.
- Типы: `httpGet`, `tcpSocket`, `exec`.
- Параметры: `initialDelaySeconds`, `periodSeconds`, `failureThreshold`.
- Антипаттерн: проверять БД в liveness (каскадные рестарты).
  **Связь с Go:** Go-сервис отдаёт `/health` для liveness, `/ready` для readiness. В distroless probe задаётся в манифесте, не в Dockerfile.

### 2.5. Resource requests и limits
**Логика:** Без лимитов один Pod может съесть всю ноду. Requests — гарантия, limits — потолок.
**Что учить:**
- `requests` — сколько нужно для планирования.
- `limits` — максимальный потолок.
- QoS-классы: **Guaranteed**, **Burstable**, **BestEffort**.
- **OOMKilled** при превышении limits.
- `kubectl top pod/node` — потребление ресурсов.
  **Связь с Go:** `GOMEMLIMIT` должен быть на 10-20% меньше `limits.memory`. Иначе OOM-kill.

## 🟠 БЛОК 3: СЕТИ И ДОСТУП
### 3.1. Service — типы
**Логика:** Pod'ы эфемерны, IP меняются. Service — стабильная точка доступа.
**Что учить:**
- **ClusterIP** — внутренний, только внутри кластера (дефолт).
- **NodePort** — открывает порт на каждой ноде (30000-32767).
- **LoadBalancer** — облачный балансировщик (ELB, ALB).
- **ExternalName** — CNAME на внешний DNS.
- **Headless Service** (`clusterIP: None`) — для StatefulSet, отдаёт IP всех Pod'ов.
- DNS внутри кластера: `<service>.<namespace>.svc.cluster.local`.
  **Связь с Go:** Go-сервис ходит к БД через `postgres.default.svc.cluster.local:5432` или просто `postgres:5432` (в том же namespace).

### 3.2. Ingress
**Логика:** Один вход снаружи для множества сервисов. HTTP-роутинг по путям и хостам.
**Что учить:**
- **Ingress** — правила роутинга (какой путь → какой Service).
- **Ingress Controller** — реализация (nginx, Traefik, HAProxy).
- TLS через секрет с сертификатом.
- Path-based и host-based роутинг.
- Annotations для настройки контроллера (таймауты, body size).
  **Связь с Go:** Обычно Ingress → Go-сервис через Service. Таймауты и body-size настраиваются в annotations.

## 🔴 БЛОК 4: КОНФИГИ И СЕКРЕТЫ
### 4.1. ConfigMap
**Логика:** Конфиги — отдельно от образа. Один образ — разные окружения.
**Что учить:**
- Создание: `kubectl create configmap`, из файла, из литералов.
- Использование: env или volume mount.
- Hot reload: mount обновляется, env — нет (нужен рестарт).
- Обновление и rollout.

### 4.2. Secret
**Логика:** Для паролей, токенов, сертификатов.
**Что учить:**
- Хранятся в **base64** (не шифрование!).
- Типы: Opaque, tls, docker-registry.
- Использование как env или volume.
- `kubectl create secret generic/docker-registry/tls`.
- Для прода — Sealed Secrets, External Secrets, Vault.
  **Связь с Go:** Go-сервис читает секреты из env или из `/etc/secrets/` (volume).

## 🟣 БЛОК 5: ЭКСПЛУАТАЦИЯ
### 5.1. Namespaces
**Логика:** Логическая изоляция. Команды, окружения, проекты.
**Что учить:**
- Дефолтные: `default`, `kube-system`, `kube-public`.
- Ресурсы в namespace: Pod, Service, ConfigMap, Secret.
- Ресурсы вне namespace: Node, ClusterRole.
- **ResourceQuota** — квоты на namespace.
- `kubectl config set-context --current --namespace=X`.

### 5.2. HPA (Horizontal Pod Autoscaler)
**Логика:** Автоматическое изменение числа реплик по метрикам.
**Что учить:**
- Требует **metrics-server**.
- Метрики: CPU, память (по умолчанию), кастомные (через Prometheus).
- `minReplicas`, `maxReplicas`, `targetCPUUtilizationPercentage`.
- Требует `requests.cpu` в Pod template.
  **Связь с Go:** Go-сервис должен быть stateless, чтобы HPA работал. Сессии — в Redis, не в памяти.

### 5.3. Rolling updates и rollback
**Логика:** Обновление без downtime.
**Что учить:**
- `kubectl set image deployment/app app=my-app:2.0`.
- `kubectl rollout status/undo/history`.
- Параметры `maxSurge`, `maxUnavailable`.
- Что происходит с Pod'ами при обновлении.
- Проблема: старая и новая версии работают одновременно → API должен быть совместим.

### 5.4. Observability
**Логика:** Без мониторинга K8s — чёрный ящик.
**Что учить:**
- Логи: `kubectl logs`, `kubectl logs -l app=foo --all-containers`.
- Метрики: metrics-server, Prometheus.
- Трейсинг: Jaeger, Tempo.
- Events: `kubectl get events`, `kubectl describe`.

## 🔵 БЛОК 6: ДИАГНОСТИКА
### 6.1. Типовые проблемы
**Логика:** 90% времени в K8s — понять, почему Pod не работает.
**Что учить:**
- **CrashLoopBackOff** — контейнер падает и перезапускается. Смотреть `kubectl logs --previous`.
- **ImagePullBackOff** — образ не тянется. Проверить имя, тег, secret для registry.
- **OOMKilled** — превышен `limits.memory`. Увеличить limit или GOMEMLIMIT.
- **Pending** — Pod не может запланироваться. Проверить `requests` и ресурсы нод.
- **Evicted** — нода выселяет Pod (диск/память). Смотреть `kubectl describe node`.

### 6.2. Инструменты диагностики
**Что учить:**
- `kubectl describe pod` — почему не стартует.
- `kubectl logs --previous` — логи упавшего контейнера.
- `kubectl get events --sort-by=.metadata.creationTimestamp`.
- `kubectl exec -it <pod> -- sh` — зайти внутрь.
- `kubectl debug` — ephemeral container.
- `k9s` — TUI для быстрой навигации.

## КЛЮЧЕВЫЕ ВЫВОДЫ
1. **Control plane** (API Server, etcd, Scheduler, Controller Manager) управляет кластером. **Worker nodes** (kubelet, kube-proxy, containerd) запускают Pod'ы.
2. **Pod** — минимальная единица. Один или несколько контейнеров, общая сеть и volumes.
3. **Deployment** — для stateless, **StatefulSet** — для БД, **Job** — одноразовые задачи.
4. **Service** — стабильный доступ. ClusterIP, NodePort, LoadBalancer, Headless.
5. **Ingress** — HTTP-роутинг снаружи. Один вход для множества сервисов.
6. **Probes:** liveness (жив), readiness (готов), startup (стартовал). Без них K8s не знает состояние.
7. **Requests** — гарантия ресурсов, **limits** — потолок. При превышении — OOMKilled.
8. **ConfigMap** — конфиги, **Secret** — секреты (base64, не шифрование).
9. **HPA** — автоскейл по метрикам. Требует stateless-приложения.
10. **Rolling update** — обновление без downtime. `kubectl rollout undo` — откат.
11. **Namespaces** — изоляция.
12. **Диагностика:** describe, logs, events, exec, debug.
13. **Compose vs K8s:** Compose — один хост, K8s — кластер. Compose для dev, K8s для прода.
14. **БД в K8s:** через StatefulSet + PVC, но на практике чаще — Managed Postgres в облаке.
15. **На собесе:** достаточно понимать архитектуру, основные объекты, probes, resources и уметь диагностировать CrashLoopBackOff.