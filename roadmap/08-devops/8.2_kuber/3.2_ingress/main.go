package main

/*
  УРОК 3.2: INGRESS
  У тебя 20 микросервисов. Каждый торчит наружу через
  LoadBalancer? Это 20 публичных IP и $300-500/мес в AWS.
  Ingress решает это: ОДИН вход для всего кластера. Правила
  роутинга по хостам и путям. Один SSL-сертификат. Одна
  точка для auth и rate-limit.

  СОДЕРЖАНИЕ:
    1.  Зачем нужен Ingress
    2.  Ingress vs Service LoadBalancer
    3.  Ingress Controller — обязательный компонент
    4.  Базовая структура Ingress
    5.  Host-based routing
    6.  Path-based routing
    7.  Wildcard-хосты и default backend
    8.  TLS — HTTPS через Secret
    9.  Cert-manager — автоматические сертификаты
    10. Annotations — настройка контроллера
    11. Основные Ingress Controller'ы
    12. Gateway API — новое поколение
    13. Debugging Ingress
    14. Связь с Go
    15. Антипаттерны
    16. Финальные выводы

  1. ЗАЧЕМ НУЖЕН INGRESS
  Есть 20 микросервисов. Каждый — Service ClusterIP.
  Пользователь должен попадать в них снаружи.

  ВАРИАНТ 1: 20 LoadBalancer'ов.
    Каждый сервис — Service type: LoadBalancer.
    Каждый — публичный IP.
    Каждый — $15-25/мес.
    Итого: 20 × $20 = $400/мес только за LB.
    Плюс 20 SSL-сертификатов.
    Плюс 20 точек для auth.
    Кошмар.

  ВАРИАНТ 2: ОДИН Ingress.
    Один публичный IP.
    Правила роутинга: путь → Service.
    Один SSL-сертификат.
    Одна точка для auth и rate-limit.

    Итого: 1 × $20 = $20/мес. Экономия 95%.

  СХЕМА:
    Интернет
       │
       ▼
    ┌─────────────────┐
    │  LoadBalancer   │  1 публичный IP
    │  (Ingress       │
    │   Controller)   │
    └────────┬────────┘
             │ роутинг по правилам
    ┌────────┼────────┬
    ▼        ▼        ▼
    /orders  /users   /admin
    │        │        │
    ▼        ▼        ▼
    Service  Service  Service
    order    user     admin

  2. INGRESS VS SERVICE LOADBALANCER

  SERVICE TYPE: LOADBALANCER:
    • L4 (TCP/UDP).
    • Один LB = один Service.
    • Дорого.
    • Работает с любым протоколом (TCP, gRPC, WebSocket).
    • Нет роутинга по путям.

  INGRESS:
    • L7 (HTTP/HTTPS).
    • Один Ingress = много Service.
    • Дёшево.
    • Только HTTP/HTTPS.
    • Роутинг по хостам и путям.
    • TLS termination.
    • Rewrite, redirect, auth.

  КОГДА ЧТО ВЫБРАТЬ:
    • HTTP/HTTPS сервисы → Ingress.
    • gRPC, TCP, WebSocket → LoadBalancer.
    • Один сервис наружу → LoadBalancer проще.
    • 3+ сервиса наружу → Ingress.

  3. INGRESS CONTROLLER — ОБЯЗАТЕЛЬНЫЙ КОМПОНЕНТ
  Сам Ingress — это ТОЛЬКО правила. YAML-объект. Он не работает
  без Ingress Controller.

  INGRESS CONTROLLER — это Deployment, который:
    • Слушает API Server на изменения Ingress-объектов.
    • Настраивает nginx/Traefik/HAProxy по этим правилам.
    • Сам является Service type: LoadBalancer (или NodePort).
    • Обрабатывает трафик.

  БЕЗ КОНТРОЛЛЕРА:
    Ingress-объект создан. Никто его не читает.
    Правила не работают. Трафик не идёт.

  УСТАНОВКА NGINX INGRESS (Killercoda/minikube/kind):
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.11.2/deploy/static/provider/cloud/deploy.yaml
    kubectl wait --namespace ingress-nginx \
      --for=condition=ready pod \
      --selector=app.kubernetes.io/component=controller \
      --timeout=120s

  В ОБЛАКЕ (EKS, GKE, AKS) — Ingress Controller обычно нужно
  ставить отдельно.

  4. БАЗОВАЯ СТРУКТУРА INGRESS

  МИНИМАЛЬНЫЙ INGRESS:
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: app
      namespace: demo
    spec:
      ingressClassName: nginx
      rules:
      - host: api.example.com
        http:
          paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: app
                port:
                  number: 80

  РАЗБОР:
    ingressClassName: nginx       — какой Controller использовать.
    rules                         — список правил.
    host                          — домен для роутинга.
    paths                         — пути на этом домене.
    pathType                      — тип сопоставления.
    backend                       — Service и порт, куда направить.

  PATH TYPE:
    Prefix:
      /orders → /orders, /orders/123, /orders/anything.
      Самый частый.

    Exact:
      /orders → только /orders.

    ImplementationSpecific:
      Зависит от контроллера. Не используй без нужды.

  5. HOST-BASED ROUTING
  Разные домены → разные сервисы.

    api.example.com    → api-service
    admin.example.com  → admin-service
    www.example.com    → web-service

  ПРИМЕР:
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: multi-host
    spec:
      ingressClassName: nginx
      rules:
      - host: api.example.com
        http:
          paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: api-service
                port:
                  number: 80

      - host: admin.example.com
        http:
          paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: admin-service
                port:
                  number: 80

  КАК РАБОТАЕТ:
    Ingress Controller смотрит Host заголовок HTTP-запроса.
    api.example.com → api-service.
    admin.example.com → admin-service.

  БЕЗ HOST:
    Если host не указан — правило работает для любого домена.
    Полезно для dev: нет своего домена, стучишься по IP.

  6. PATH-BASED ROUTING
  Один домен → разные пути → разные сервисы.

    api.example.com/orders  → order-service
    api.example.com/users   → user-service
    api.example.com/payment → payment-service

  ПРИМЕР:
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: path-based
    spec:
      ingressClassName: nginx
      rules:
      - host: api.example.com
        http:
          paths:
          - path: /orders
            pathType: Prefix
            backend:
              service:
                name: order-service
                port:
                  number: 80
          - path: /users
            pathType: Prefix
            backend:
              service:
                name: user-service
                port:
                  number: 80

  ПОРЯДОК ПРАВИЛ ВАЖЕН:
    Более специфичные — раньше.

    ПРАВИЛЬНО:
      - path: /api/v2
      - path: /api

    НЕПРАВИЛЬНО:
      - path: /api
      - path: /api/v2      ← никогда не сработает

  REWRITE (nginx-ingress):

    Если нужно убрать префикс:
      annotations:
        nginx.ingress.kubernetes.io/rewrite-target: /

    Запрос /orders/123 → /123 в order-service.

  7. WILDCARD-ХОСТЫ И DEFAULT BACKEND
  WILDCARD — правило для всех поддоменов одного домена.

    - host: "*.example.com"
      http:
        paths:
        - path: /
          pathType: Prefix
          backend:
            service:
              name: app
              port:
                number: 80

  Сработает для:
    • api.example.com
    • admin.example.com
    • anything.example.com

  НЕ сработает для:
    • example.com (без поддомена).

  DEFAULT BACKEND — что отвечать, если ни одно правило
  не подошло.

    spec:
      defaultBackend:
        service:
          name: fallback
          port:
            number: 80

  Полезно для кастомной 404-страницы или единой точки входа
  для всех неизвестных доменов.

  8. TLS — HTTPS ЧЕРЕЗ SECRET
  Ingress умеет terminate SSL. Сертификат лежит в Secret.

  СОЗДАТЬ SECRET:
    kubectl create secret tls api-tls \
      --cert=tls.crt \
      --key=tls.key \
      -n demo

  Или манифестом:
    apiVersion: v1
    kind: Secret
    metadata:
      name: api-tls
      namespace: demo
    type: kubernetes.io/tls
    data:
      tls.crt: <base64-encoded-cert>
      tls.key: <base64-encoded-key>

  INGRESS С TLS:
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: app
    spec:
      ingressClassName: nginx
      tls:
      - hosts:
        - api.example.com
        secretName: api-tls
      rules:
      - host: api.example.com
        http:
          paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: app
                port:
                  number: 80

  ЧТО ПРОИСХОДИТ:
    • Ingress Controller слушает 443.
    • Клиент подключается по HTTPS.
    • SSL termination на Controller.
    • Внутри кластера трафик идёт по HTTP.

  9. CERT-MANAGER — АВТОМАТИЧЕСКИЕ СЕРТИФИКАТЫ
  Cert-manager автоматически получает Let's Encrypt сертификаты
  и обновляет их раз в 60 дней.

  УСТАНОВКА:
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.0/cert-manager.yaml

  ClusterIssuer (Let's Encrypt):
    apiVersion: cert-manager.io/v1
    kind: ClusterIssuer
    metadata:
      name: letsencrypt
    spec:
      acme:
        server: https://acme-v02.api.letsencrypt.org/directory
        email: admin@example.com
        privateKeySecretRef:
          name: letsencrypt-key
        solvers:
        - http01:
            ingress:
              class: nginx

  INGRESS С АВТО-СЕРТИФИКАТОМ:
    metadata:
      annotations:
        cert-manager.io/cluster-issuer: letsencrypt
    spec:
      tls:
      - hosts:
        - api.example.com
        secretName: api-tls    # cert-manager создаст сам

  ЧТО ПРОИСХОДИТ:
    1. Cert-manager видит аннотацию.
    2. Заказывает сертификат у Let's Encrypt.
    3. Проходит ACME challenge.
    4. Создаёт Secret api-tls.
    5. Раз в 60 дней — обновляет автоматически.

  10. ANNOTATIONS — НАСТРОЙКА КОНТРОЛЛЕРА
  Annotations управляют Ingress Controller.

  ПОПУЛЯРНЫЕ ДЛЯ NGINX:

    Таймауты:
      nginx.ingress.kubernetes.io/proxy-read-timeout: "60"
      nginx.ingress.kubernetes.io/proxy-send-timeout: "60"
      nginx.ingress.kubernetes.io/proxy-connect-timeout: "60"

    Размер тела:
      nginx.ingress.kubernetes.io/proxy-body-size: "10m"

    Redirect HTTP → HTTPS:
      nginx.ingress.kubernetes.io/ssl-redirect: "true"
      nginx.ingress.kubernetes.io/force-ssl-redirect: "true"

    Rate limiting:
      nginx.ingress.kubernetes.io/limit-rps: "10"
      nginx.ingress.kubernetes.io/limit-burst: "20"

    Basic auth:
      nginx.ingress.kubernetes.io/auth-type: basic
      nginx.ingress.kubernetes.io/auth-secret: basic-auth

    Rewrite:
      nginx.ingress.kubernetes.io/rewrite-target: /

    CORS:
      nginx.ingress.kubernetes.io/enable-cors: "true"
      nginx.ingress.kubernetes.io/cors-allow-origin: "*"

    gRPC:
      nginx.ingress.kubernetes.io/backend-protocol: "GRPC"

    WebSocket — работает из коробки. Таймауты те же
    (proxy-read-timeout).

  ПОЛНЫЙ ПРИМЕР С ANNOTATIONS:
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: app
      annotations:
        nginx.ingress.kubernetes.io/proxy-body-size: "10m"
        nginx.ingress.kubernetes.io/proxy-read-timeout: "120"
        nginx.ingress.kubernetes.io/ssl-redirect: "true"
        nginx.ingress.kubernetes.io/limit-rps: "100"
    spec:
      ingressClassName: nginx
      tls:
      - hosts:
        - api.example.com
        secretName: api-tls
      rules:
      - host: api.example.com
        http:
          paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: app
                port:
                  number: 80

  ВАЖНО ПРО GO:
    Таймауты Ingress влияют на долгие запросы. Если экспорт
    данных занимает 90 секунд, а proxy-read-timeout дефолт 60 —
    клиент получит 504. Ставь таймауты под свои worst-case.

  11. ОСНОВНЫЕ INGRESS CONTROLLER'Ы

  NGINX INGRESS:
    • Самый популярный.
    • Основан на nginx.
    • Богатые аннотации.
    • Подходит для большинства случаев.

  TRAEFIK:
    • Написан на Go.
    • Динамическая конфигурация через CRD.
    • Красивый dashboard.
    • Хорош для микросервисов.

  HAProxy INGRESS:
    • На основе HAProxy.
    • Высокая производительность.
    • Хорош для TCP.

  AWS LOAD BALANCER CONTROLLER:
    • Для EKS.
    • Создаёт ALB/NLB из Ingress-объектов.
    • Интеграция с IAM, ACM, WAF.

  ЧТО ВЫБРАТЬ:
    • Обычный кластер → nginx-ingress.
    • Kubernetes-native → Traefik.
    • EKS → AWS Load Balancer Controller.
    • Высокие нагрузки → HAProxy или Envoy.

  12. GATEWAY API — НОВОЕ ПОКОЛЕНИЕ
  Gateway API — эволюция Ingress. Разрабатывается в SIG-Network.

  ЧЕМ ОТЛИЧАЕТСЯ:
    • Роли разделены:
      - GatewayClass — какой контроллер.
      - Gateway — точка входа (LB, порты, TLS).
      - HTTPRoute — правила роутинга.

    • Явная поддержка L4 и L7.
    • Лучше для multi-tenant.
    • Разные команды управляют разными частями.

  ПРИМЕР:
    apiVersion: gateway.networking.k8s.io/v1
    kind: Gateway
    metadata:
      name: prod-gateway
    spec:
      gatewayClassName: nginx
      listeners:
      - name: https
        port: 443
        protocol: HTTPS
        tls:
          certificateRefs:
          - name: api-tls
    ---
    apiVersion: gateway.networking.k8s.io/v1
    kind: HTTPRoute
    metadata:
      name: app
    spec:
      parentRefs:
      - name: prod-gateway
      rules:
      - matches:
        - path:
            type: PathPrefix
            value: /orders
        backendRefs:
        - name: order-service
          port: 80

  СТАТУС В 2026: Gateway API стабилен (v1). Постепенно
  заменяет Ingress. Но Ingress всё ещё распространён.

  13. DEBUGGING INGRESS
  ПОШАГОВО:

    ШАГ 1: Контроллер установлен?
      kubectl get pods -n ingress-nginx
      # Все Running?

    ШАГ 2: Ingress создан?
      kubectl get ingress -n demo
      # ADDRESS заполнен?

    ШАГ 3: Endpoints заполнены?
      kubectl get endpoints -n demo
      # Пусто → selector Service не совпадает или Pod'ы не Ready.

    ШАГ 4: Логи контроллера.
      kubectl logs -n ingress-nginx \
        -l app.kubernetes.io/component=controller --tail=100

    ШАГ 5: Конфиг nginx внутри контроллера.
      kubectl exec -it -n ingress-nginx <controller-pod> -- \
        cat /etc/nginx/nginx.conf | grep -A 5 demo

  ТИПОВЫЕ ОШИБКИ:
    • 404 — правила не совпадают (host, path).
    • 502 — Service пустой (нет Ready Pod'ов).
    • 503 — Pod'ы падают.
    • 504 — таймаут, сервис не ответил.
    • ADDRESS пустой — LoadBalancer не создался.

  PORT-FORWARD (если нет LoadBalancer):
    kubectl port-forward -n ingress-nginx \
      svc/ingress-nginx-controller 8080:80

    curl -H "Host: api.example.com" http://localhost:8080/

  14. СВЯЗЬ С GO
  Go-сервис не знает про Ingress. Слушает порт 8080.

  ТАЙМАУТЫ:
    Ingress > Go-сервис. Иначе 504 раньше ответа.

    Ingress:       proxy-read-timeout: "120"
    Go-сервис:     WriteTimeout: 130 * time.Second

  HEADERS ПРОКСИРОВАНИЯ:
    Ingress прокидывает X-Real-IP, X-Forwarded-For,
    X-Forwarded-Proto. В Go читай:

      realIP := r.Header.Get("X-Real-IP")
      proto := r.Header.Get("X-Forwarded-Proto")

  WEBSOCKET — работает из коробки.

  gRPC — нужна аннотация:
    nginx.ingress.kubernetes.io/backend-protocol: "GRPC"

  15. АНТИПАТТЕРНЫ

  15.1. INGRESS БЕЗ CONTROLLER.
    Объект создан, но трафик не идёт.

  15.2. LOADBALANCER ДЛЯ КАЖДОГО СЕРВИСА.
    20 LB = $400/мес. Ingress = 1 LB.

  15.3. ПУТИ В НЕПРАВИЛЬНОМ ПОРЯДКЕ.
    /api перед /api/v2 → v2 не сработает.

  15.4. НЕТ TLS.
    HTTPS в 2026 обязателен.

  15.5. ТАЙМАУТЫ МЕНЬШЕ, ЧЕМ У СЕРВИСА.
    Клиент получит 504.

  15.6. TLS SECRET В ДРУГОМ NAMESPACE.
    Должен быть в том же namespace, что Ingress.

  15.7. HOST: "*" В ПРОДЕ.
    Работает для любого домена. Только конкретные.

  15.8. НЕТ ANNOTATIONS ДЛЯ BODY SIZE.
    Клиент загружает 20 МБ, дефолт 1 МБ → 413.

  15.9. INGRESS БЕЗ PATH TYPE.
    Дефолт — ImplementationSpecific.

  15.10. INGRESS БЕЗ HEALTHCHECK.
    Service пустой → 502.

  15.11. IGNORE ИМЯ КЛАССА.
    Без ingressClassName — дефолтный Controller.

  15.12. НЕ СМОТРЕТЬ ADDRESS В INGRESS.
    Если пусто — LB не создался.

  15.13. ОДИН INGRESS НА 100 ПРАВИЛ.
    Сложно поддерживать. Разделяй по сервисам.

  16. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Ingress — L7 HTTP-роутинг снаружи. Один вход для
      множества сервисов.
  2.  Ingress vs LoadBalancer: в 10-20 раз дешевле.
  3.  Ingress Controller обязателен. Без него не работает.
  4.  Структура: ingressClassName, rules, host, paths, backend.
  5.  Host-based — по домену. Path-based — по пути.
      Wildcard — для поддоменов. Default backend — fallback.
  6.  TLS через Secret типа kubernetes.io/tls.
  7.  Cert-manager — автосертификаты Let's Encrypt.
  8.  Annotations: таймауты, body-size, rewrite, rate-limit.
  9.  Controllers: nginx, Traefik, HAProxy, AWS LB Controller.
  10. Gateway API — новое поколение. Разделяет роли.
  11. Debugging: Controller → Ingress → Endpoints → логи.
  12. Таймауты Ingress > Go-сервис. Иначе 504.
  13. Антипаттерны: Ingress без Controller, LB для каждого
      сервиса, пути в неправильном порядке, нет TLS.
*/
