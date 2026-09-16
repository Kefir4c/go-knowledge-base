package baseimage

/*
  УРОК 2.2: СРАВНЕНИЕ БАЗОВЫХ ОБРАЗОВ
  Размер образа — это не просто цифра в docker images. Это скорость
  pull (а значит — скорость деплоя и горизонтального масштабирования),
  стоимость хранения в registry и, что критичнее всего, — площадь
  атаки. Чем больше в образе лишнего, тем больше уязвимостей.
  Для Go-разработчика выбор базового образа — это не «поставить
  что поменьше». Это осознанный trade-off между размером, удобством
  отладки, совместимостью и безопасностью. В этой теме мы разберём
  четыре основных варианта: ubuntu, alpine, distroless и scratch.

  СОДЕРЖАНИЕ:
    1.  Зачем вообще выбирать базовый образ
    2.  ubuntu — полный Linux
    3.  alpine — популярный минимализм
    4.  gcr.io/distroless — золотая середина
    5.  scratch — абсолютный ноль
    6.  Сводная таблица характеристик
    7.  Что такое musl libc и почему это важно
    8.  CA-сертификаты и tzdata
    9.  Non-root пользователь
    10. Выбор для Go: dev, staging, prod
    11. Отладка distroless и scratch
    12. Антипаттерны
    13. Финальные выводы

  1. ЗАЧЕМ ВООБЩЕ ВЫБИРАТЬ БАЗОВЫЙ ОБРАЗ
  Базовый образ — это фундамент твоего контейнера. Он определяет:

    • РАЗМЕР. Влияет на скорость pull. Alpine (5 МБ) качается
      за секунды, Ubuntu (80 МБ) — за минуты. В Kubernetes
      при скейле это разница между «поднялось быстро» и «поднялось
      через минуту».

    • СТОИМОСТЬ REGISTRY. Хранить 100 образов по 800 МБ — дорого.
      Хранить 100 образов по 15 МБ — дёшево.

    • ATTACK SURFACE. Каждый пакет в образе — потенциальная
      уязвимость (CVE). Чем меньше пакетов, тем меньше CVE.
      Ubuntu содержит сотни пакетов. Scratch — ноль.

    • СОВМЕСТИМОСТЬ. musl libc (alpine) vs glibc (ubuntu) —
      разные стандартные библиотеки. CGO-бинарники могут
      не работать на musl.

    • УДОБСТВО ОТЛАДКИ. Есть shell — можно зайти через
      docker exec. Нет shell — только debug-образ или nsenter.

  ПРАВИЛО: базовый образ — это часть архитектуры приложения.
  Менять его — это рефакторинг, а не «поправить одну строчку
  в Dockerfile».

  2. UBUNTU — ПОЛНЫЙ LINUX
  Ubuntu — классический дистрибутив Linux. Полноценная ОС со
  всеми утилитами.

    FROM ubuntu:22.04

  РАЗМЕР: 80 МБ. Это базовый образ без ничего. Каждый apt-get
  install добавляет сверху.

  ПЛЮСЫ:
    • ВСЁ ЕСТЬ. bash, curl, wget, ps, top, apt — всё из коробки.
    • ЗНАКОМО. Большинство разработчиков знают Ubuntu.
    • СОВМЕСТИМОСТЬ. glibc — стандарт, всё работает.
    • APT. Огромный репозиторий пакетов.
    • ДОКУМЕНТАЦИЯ. Все туториалы в интернете — для Ubuntu/Debian.

  МИНУСЫ:
    • БОЛЬШОЙ. 80 МБ базы. С зависимостями — 200-500 МБ.
    • МНОГО CVE. Каждый пакет — потенциальная уязвимость.
      Регулярные обновления — обязательны.
    • МЕДЛЕННЫЙ PULL. 80 МБ качается долго, особенно при скейле.
    • ИЗБЫТОЧНОСТЬ. 95% пакетов в рантайме не нужны.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Legacy-проекты, где важна совместимость.
    • Нужны специфичные системные пакеты, которых нет в alpine.
    • CGO-бинарники со сложными зависимостями.
    • Разработка, где нужно много инструментов.

  ДЛЯ GO — практически никогда. Go-бинарник статический, ему
  Ubuntu не нужен.

  3. ALPINE — ПОПУЛЯРНЫЙ МИНИМАЛИЗМ
  Alpine — минималистичный Linux. 5 МБ базы, musl libc, busybox.

    FROM alpine:3.19

  РАЗМЕР: 5 МБ. С пакетами — 15-30 МБ.

  ПЛЮСЫ:
    • МАЛЕНЬКИЙ. 5 МБ vs 80 МБ у Ubuntu. В 16 раз меньше.
    • ЕСТЬ SHELL. busybox ash — можно зайти через docker exec.
    • APK. Пакетный менеджер. Много пакетов, но меньше, чем в apt.
    • БЕЗОПАСНЕЕ. Меньше пакетов — меньше CVE.
    • ПОПУЛЯРЕН. Большинство Go-туториалов используют alpine.

  МИНУСЫ:
    • MUSL LIBC. Не glibc. CGO-бинарники могут не работать.
      Race detector в Go не работает на musl.
    • BUSYBOX ASH. Не bash. Скрипты с bash-синтаксисом
      могут не работать.
    • DNS RESOLVER. Другой резолвер. В больших k8s-кластерах
      бывают проблемы с CoreDNS.
    • МЕНЬШЕ ПАКЕТОВ. Не всё есть в apk.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Dev/staging — есть shell для отладки.
    • Нужны сторонние утилиты (curl, jq, psql).
    • CGO_ENABLED=0 (статический бинарник) — тогда musl не проблема.
    • Нужен entrypoint-скрипт на shell.

  ДЛЯ GO — хороший вариант для dev/staging. Для строгого прода
  лучше distroless.

  4. GCR.IO/DISTROLESS — ЗОЛОТАЯ СЕРЕДИНА
  Distroless — образы от Google. Нет shell, нет package manager.
  Только runtime, ca-certs, tzdata и nonroot-пользователь.

    FROM gcr.io/distroless/static-debian12:nonroot

  РАЗМЕР: 2 МБ. С бинарником — 10-15 МБ.

  ЧТО ВНУТРИ:
    • ca-certificates — HTTPS работает из коробки.
    • tzdata — time.LoadLocation работает.
    • /etc/passwd с nonroot (UID 65532).
    • /etc/ssl — сертификаты.
    • Больше ничего.

  ПЛЮСЫ:
    • МАЛЕНЬКИЙ. 2 МБ базы. Меньше alpine.
    • БЕЗОПАСНЫЙ. Нет shell — нечего запускать. Нет пакетов —
      нечего обновлять. CVE почти не накапливаются.
    • NONROOT ПО УМОЛЧАНИЮ. С тегом :nonroot — UID 65532.
    • CA-CERTS И TZDATA. Не надо тащить руками.
    • МЕНЬШЕ CVE. Реально меньше, чем у alpine.

  МИНУСЫ:
    • НЕТ SHELL. Зайти через docker exec нельзя.
    • НЕТ CURL. Healthcheck через curl не работает.
    • НЕТ ENTRYPOINT-СКРИПТОВ. Только exec-форма.
    • ОТЛАДКА СЛОЖНЕЕ. Нужен debug-образ или nsenter.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Прод для Go-сервисов — идеальный выбор.
    • CGO_ENABLED=0 — обязательно (static).
    • Нужны HTTPS и таймзоны — включено.
    • Строгие требования к безопасности.

  ВАРИАНТЫ:
    • static-debian12:nonroot — для static Go-бинарников. 2 МБ.
    • base-debian12:nonroot — для CGO (glibc). 20 МБ.
    • cc-debian12:nonroot — для C++.
    • static-debian12:debug — с busybox для отладки. Не для прода.

  ДЛЯ GO — это лучший выбор для прода. Точка.

  5. SCRATCH — АБСОЛЮТНЫЙ НОЛЬ
  scratch — пустой образ. Вообще ничего.

    FROM scratch
    COPY --from=builder /out/server /server
    ENTRYPOINT ["/server"]

  РАЗМЕР: 0 МБ. С бинарником — 5-10 МБ.

  ЧТО ВНУТРИ: только твой бинарник. Всё.

  ПЛЮСЫ:
    • МИНИМАЛЬНО ВОЗМОЖНЫЙ РАЗМЕР. 5-10 МБ с бинарником.
    • НУЛЕВАЯ ПЛОЩАДЬ АТАКИ. Нечего эксплуатировать.
    • ИДЕАЛЕН ДЛЯ STATIC GO. CGO_ENABLED=0 — работает.
    • НЕТ CVE. Вообще нечего сканировать.

  МИНУСЫ:
    • НЕТ CA-CERTS. HTTPS-запросы падают. Надо копировать руками.
    • НЕТ TZDATA. time.LoadLocation падает. Надо копировать руками.
    • НЕТ /ETC/PASSWD. USER appuser не работает. Только UID.
    • НЕТ SHELL. Отладка — только через nsenter или debug.
    • НЕТ /TMP. Иногда нужно --tmpfs /tmp.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Абсолютный минимум.
    • CGO_ENABLED=0 (static binary).
    • Не нужны HTTPS-запросы к внешним API.
    • Не нужны таймзоны.
    • UID фиксирован числом, не именем.

  КОГДА НЕ ИСПОЛЬЗОВАТЬ:
    • Нужны HTTPS-запросы.
    • Нужны таймзоны.
    • Нужны сторонние утилиты.

  В 90% случаев distroless лучше scratch. Distroless уже
  содержит ca-certs и tzdata. Не надо тащить руками.

  6. СВОДНАЯ ТАБЛИЦА ХАРАКТЕРИСТИК
  ┌────────────┬──────────┬────────┬────────────┬──────────────────┐
  │ Образ      │ Размер   │ Shell  │ libc       │ CVE (примерно)   │
  ├────────────┼──────────┼────────┼────────────┼──────────────────┤
  │ ubuntu     │ 80 МБ    │ bash   │ glibc      │ 100+             │
  │ alpine     │ 5 МБ     │ ash    │ musl       │ 10-30            │
  │ distroless │ 2 МБ     │ нет    │ glibc      │ 0-5              │
  │ scratch    │ 0 МБ     │ нет    │ нет        │ 0                │
  └────────────┴──────────┴────────┴────────────┴──────────────────┘

  С БИНАРНИКОМ GO:
  ┌────────────┬──────────────────┬──────────┬─────────────────────┐
  │ Образ      │ Итоговый размер  │ Отладка  │ Сценарий            │
  ├────────────┼──────────────────┼──────────┼─────────────────────┤
  │ ubuntu     │ 100-150 МБ       │ легко    │ legacy, CGO         │
  │ alpine     │ 15-30 МБ         │ легко    │ dev/staging         │
  │ distroless │ 10-15 МБ         │ сложно   │ прод Go             │
  │ scratch    │ 5-10 МБ          │ сложно   │ минимализм          │
  └────────────┴──────────────────┴──────────┴─────────────────────┘

  7. ЧТО ТАКОЕ MUSL LIBC И ПОЧЕМУ ЭТО ВАЖНО
  libc — стандартная библиотека C. Она даёт базовые функции:
  malloc, printf, open, read, socket, DNS-резолвинг.

  ДВА ВАРИАНТА:
    • GLIBC. GNU C Library. Стандарт на Ubuntu, Debian, CentOS.
      Большая, но полная.

    • MUSL LIBC. Альтернатива. В Alpine. Маленькая, но с нюансами.

  ПОЧЕМУ ЭТО ВАЖНО ДЛЯ GO:
    Go с CGO_ENABLED=0 — компилируется статически. libc не нужна.
    Работает везде. Это путь для scratch и distroless.

    Go с CGO_ENABLED=1 — линкуется с libc. На alpine — с musl,
    на ubuntu — с glibc. Бинарник не переносим между ними.

  ПРОБЛЕМЫ С MUSL:
    • DNS. Другой резолвер. В k8s бывают проблемы с поиском
      сервисов через CoreDNS. Иногда нужно копировать
      /etc/resolv.conf или ставить nss-dns.

    • RACE DETECTOR. go test -race не работает на musl.

    • НЕКОТОРЫЕ БИБЛИОТЕКИ. Ищут glibc-специфичные вещи.
      Приходится ставить libc6-compat.

  ПРАВИЛО ДЛЯ GO: CGO_ENABLED=0. Тогда musl vs glibc не важно.
  Бинарник работает на любом Linux.

  8. CA-СЕРТИФИКАТЫ И TZDATA
  Эти две вещи часто забывают, а потом ловят странные баги.

  CA-СЕРТИФИКАТЫ:
    Нужны для HTTPS-запросов к внешним API (Stripe, AWS, etc).

    Без них:
      x509: certificate signed by unknown authority

    Где есть:
      • ubuntu, alpine, distroless — из коробки.
      • scratch — НЕТ. Копируй руками.

    Копирование в scratch:
      FROM alpine:3.19 AS certs
      RUN apk add --no-cache ca-certificates

      FROM scratch
      COPY --from=certs /etc/ssl/certs/ca-certificates.crt \
                          /etc/ssl/certs/ca-certificates.crt
      COPY --from=builder /out/server /server
      ENTRYPOINT ["/server"]

  TZDATA:
    Нужны для time.LoadLocation("Europe/Moscow").

    Без них:
      error: unknown time zone Europe/Moscow

    Где есть:
      • ubuntu, alpine, distroless — из коробки.
      • scratch — НЕТ. Копируй руками.

    Копирование в scratch:
      COPY --from=certs /usr/share/zoneinfo /usr/share/zoneinfo

  ВЫВОД: если используешь scratch — не забудь ca-certs и tzdata.
  Или используй distroless — там всё уже есть.

  9. NON-ROOT ПОЛЬЗОВАТЕЛЬ
  По умолчанию контейнер работает под root. Это плохо для
  безопасности: если сервис пробьют — злоумышленник root.

  КАК СДЕЛАТЬ NON-ROOT:

    В UBUNTU / ALPINE:
      RUN adduser -D -u 1000 appuser
      USER appuser

    В DISTROLESS:
      FROM gcr.io/distroless/static-debian12:nonroot
      USER nonroot:nonroot

      UID 65532 уже создан. Просто используй.

    В SCRATCH:
      USER 1000:1000

      Только UID. /etc/passwd нет — имени не существует.

      Или скопировать /etc/passwd:

      COPY --from=builder /etc/passwd /etc/passwd
      COPY --from=builder /etc/group /etc/group
      USER appuser

  ВАЖНО: COPY --chown для файлов.

    Если копируешь файлы под root, а потом переключаешься на
    USER 1000, файлы будут root:root. Приложение не сможет
    их читать.

      COPY --from=builder --chown=1000:1000 /out/server /server

  ПРАВИЛО: всегда non-root в проде. distroless даёт это
  из коробки.

  10. ВЫБОР ДЛЯ GO: DEV, STAGING, PROD

  DEV / STAGING:
    Рекомендация: alpine.

    Почему: нужен shell для отладки, есть apk для установки
    утилит, маленький (5 МБ базы). Musl не проблема, если
    CGO_ENABLED=0.

  PROD:
    Рекомендация: distroless.

    Почему: 2 МБ базы, ca-certs и tzdata внутри, nonroot по
    умолчанию, минимальный attack surface, CVE почти нет.

  АБСОЛЮТНЫЙ МИНИМУМ:
    Рекомендация: scratch.

    Почему: 0 МБ базы, только бинарник. Но: нет ca-certs,
    нет tzdata, нет /etc/passwd. Всё вручную.

  LEGACY / CGO:
    Рекомендация: ubuntu или debian-slim.

    Почему: glibc, полный набор пакетов. Но 80+ МБ.

  СХЕМА ВЫБОРА:
    ┌──────────────────────────────────────────┐
    │ Нужен shell для отладки?                 │
    │   Да → alpine                            │
    │   Нет → нужно HTTPS и таймзоны?          │
    │     Да → distroless                      │
    │     Нет → scratch                        │
    └──────────────────────────────────────────┘

  11. ОТЛАДКА DISTROLESS И SCRATCH
  Проблема: нет shell. Зайти через docker exec нельзя.

  РЕШЕНИЯ:
  11.1. DEBUG-ОБРАЗ
    Собирай два образа из одного Dockerfile:
      FROM alpine:3.19 AS debug
      RUN apk add --no-cache curl strace tcpdump jq
      COPY --from=builder /out/server /app/server
      ENTRYPOINT ["/app/server"]

      FROM gcr.io/distroless/static-debian12:nonroot AS prod
      COPY --from=builder /out/server /server
      ENTRYPOINT ["/server"]
    Debug — для отладки, prod — для прода.

  11.2. DOCKER CP
    Скопировать файлы из контейнера без shell:
      docker cp <id>:/app/logs ./logs
      docker cp ./config.yaml <id>:/app/
    Работает и для distroless.

  11.3. NSENTER
    Зайти в namespace контейнера через хост:
      PID=$(docker inspect -f '{{.State.Pid}}' <id>)
      nsenter -t $PID -n -p -m
    Работает даже для scratch. Требует root на хосте.

  11.4. KUBECTL DEBUG
    В Kubernetes — ephemeral containers:
      kubectl debug -it <pod> --image=alpine
    Отдельный контейнер в том же pod. Видит те же volumes
    и network.

  12. АНТИПАТТЕРНЫ

  12.1. UBUNTU ДЛЯ GO-СЕРВИСА.
    Go-бинарник статический. Ubuntu не нужна. 80 МБ базы — зря.
  12.2. ALPINE ДЛЯ ПРОДА БЕЗ CGO_ENABLED=0.
    CGO-бинарник на musl может не работать. Всегда CGO_ENABLED=0.
  12.3. SCRATCH БЕЗ CA-CERTS.
    HTTPS-запросы падают. Копируй ca-certificates.
  12.4. SCRATCH БЕЗ TZDATA.
    time.LoadLocation падает. Копируй zoneinfo.
  12.5. DISTROLESS БЕЗ :nonroot.
    Работает под root. Теряется половина смысла distroless.
  12.6. RUN ОТ ROOT, USER ПОСЛЕ.
    Файлы root:root. Приложение не читает. Используй --chown.
  12.7. latest В БАЗОВОМ ОБРАЗЕ.
    Меняется без предупреждения. Фиксируй версию (3.19, не latest).
  12.8. UBUNTU ДЛЯ CGO БЕЗ ПРИЧИНЫ.
    Debian-slim меньше и тоже glibc.
  12.9. DISTROLESS БЕЗ DEBUG-ОБРАЗА.
    Попадёшь в инцидент — зайти нельзя. Всегда имей debug.
  12.10. ОТСУТСТВИЕ HEALTHCHECK.
    Оркестратор не знает, готов ли сервис. Для distroless —
    через подкоманду в бинарнике.

  13. ФИНАЛЬНЫЕ ВЫВОДЫ

  1.  Базовый образ — часть архитектуры. Выбирай осознанно.
  2.  Ubuntu (80 МБ) — для legacy и CGO. Для Go — не нужна.
  3.  Alpine (5 МБ) — для dev/staging. Shell, apk, musl libc.
      CGO_ENABLED=0 решает проблему musl.
  4.  Distroless (2 МБ) — золотая середина для прода.
      Без shell, с ca-certs и tzdata, nonroot по умолчанию.
  5.  Scratch (0 МБ) — абсолютный минимум. Всё вручную:
      ca-certs, tzdata, /etc/passwd. В 90% случаев distroless лучше.
  6.  MUSL vs GLIBC: для Go с CGO_ENABLED=0 не важно.
      Static binary работает на любом Linux.
  7.  CA-CERTS и TZDATA — частые забытые вещи. В distroless
      они есть. В scratch — копируй руками.
  8.  NON-ROOT — обязательно. distroless:nonroot — простое решение.
  9.  COPY --CHOWN — иначе файлы root:root и приложение не читает.
  10. DEV: alpine. PROD: distroless. МИНИМАЛИЗМ: scratch. LEGACY: ubuntu.
  11. DEBUG-ОБРАЗ из того же Dockerfile — обязателен для
      distroless и scratch.
  12. Docker cp, nsenter, kubectl debug — способы отладки без shell.
  13. На собесе: «Для Go используем distroless для прода
      и alpine для dev. CGO_ENABLED=0 даёт static binary,
      который работает на distroless. Nonroot по умолчанию.
      ca-certs и tzdata внутри. Минимальный attack surface,
      почти нет CVE. Для отладки — debug-образ на alpine».
*/
