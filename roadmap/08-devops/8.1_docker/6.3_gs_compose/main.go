package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

/*
  УРОК 6.3: GRACEFUL SHUTDOWN НА УРОВНЕ COMPOSE
  docker compose stop — команда, которая кажется простой.
  Но за ней стоит сложный механизм: сигналы, таймауты, порядок
  остановки. Если не понимать — при каждом деплое будет
  потеря данных, оборванные соединения, "Killed" в логах.
  Особенно это важно для сервисов с состоянием: БД, брокеров,
  Go-сервисов с открытыми соединениями. Правильная остановка —
  это не «пусть падает», а «дай процессу завершить начатое».

  СОДЕРЖАНИЕ:
    1.  Проблема: жёсткая остановка
    2.  Что происходит при docker compose stop
    3.  Сигналы: SIGTERM, SIGKILL, SIGINT, SIGHUP
    4.  stop_signal: какой сигнал послать
    5.  stop_grace_period: сколько ждать
    6.  PID 1 в контейнере
    7.  Shell-форма vs exec-форма ENTRYPOINT
    8.  Порядок остановки в Compose
    9.  Docker stop vs kill vs down
    10. Своё время для разных сервисов
    11. Как проверить, что shutdown работает
    12. Антипаттерны
    13. Финальные выводы

  1. ПРОБЛЕМА: ЖЁСТКАЯ ОСТАНОВКА
  Ты делаешь `docker compose stop` или `docker compose down`.
  Что происходит с твоим приложением?

  Наивное ожидание:
    Приложение получает сигнал «остановись».
    Дочитывает запросы.
    Закрывает соединения.
    Сохраняет данные.
    Выходит чисто.

  Реальность (если ничего не настроено):
    Docker посылает SIGTERM.
    Ждёт 10 секунд.
    Если процесс не завершился — SIGKILL.
    Процесс умирает мгновенно.
    Данные могут быть потеряны.

  ЧТО ЭТО ЗНАЧИТ НА ПРАКТИКЕ:

    Postgres:
      • Убит посреди записи WAL.
      • Нужно восстановление при следующем старте.
      • Медленный рестарт.

    Kafka:
      • Не закоммичены offset'ы.
      • Консьюмеры перечитают с начала.
      • Дубли сообщений.

    Go-сервис с HTTP:
      • Активные запросы оборваны.
      • Клиенты получают ECONNRESET.
      • Логи не дописаны.

    Go-сервис с БД:
      • Незакоммиченные транзакции потеряны.
      • Соединения с БД висят до таймаута.
      • Счётчики соединений в БД растут.

  ПРАВИЛЬНЫЙ ПОДХОД: настроить compose так, чтобы у процесса
  было время завершиться. Это делается через два параметра:
  `stop_signal` и `stop_grace_period`.

  2. ЧТО ПРОИСХОДИТ ПРИ DOCKER COMPOSE STOP
  Пошагово:
    Шаг 1: Compose отправляет сигнал stop_signal (по умолчанию
           SIGTERM) процессу с PID 1 в контейнере.

    Шаг 2: Процесс получает сигнал.
           • Если он его ловит — начинает graceful shutdown.
           • Если не ловит — ядро убивает процесс (default
             handler для SIGTERM — termination).

    Шаг 3: Compose ждёт stop_grace_period (по умолчанию 10
           секунд).

    Шаг 4: Если процесс всё ещё жив — SIGKILL.
           SIGKILL нельзя поймать. Процесс умирает мгновенно.

    Шаг 5: Контейнер переходит в состояние Exited.

  ВРЕМЕННАЯ ШКАЛА:
    t=0s      SIGTERM послан
    t=0-10s   ждём (grace period)
    t=10s     SIGKILL, если процесс не завершился

  ЧТО ВИДНО В ЛОГАХ:
    Если shutdown сработал:
      $ docker compose logs app
      {"msg":"SIGTERM received"}
      {"msg":"shutting down gracefully"}
      {"msg":"stopped cleanly"}

    Если shutdown не сработал:
      $ docker compose logs app
      (пусто про shutdown)
      # Процесс убит через 10 секунд.

    $ docker compose ps
    # Exited (137)    ← 128+9 = SIGKILL

  ВАЖНО ПРО EXIT CODE:
    0     — успешное завершение.
    143   — 128+15 = SIGTERM (процесс сам обработал и вышел).
    137   — 128+9 = SIGKILL (принудительно убит).
    1-125 — код ошибки приложения.

  3. СИГНАЛЫ: SIGTERM, SIGKILL, SIGINT, SIGHUP
  Сигнал — асинхронное уведомление от ядра процессу.

  SIGTERM (15) — «вежливая просьба завершиться»:
    • Можно перехватить.
    • Можно отложить.
    • Можно обработать (graceful shutdown).
    • Это дефолт для Docker stop.

  SIGKILL (9) — «немедленно умереть»:
    • НЕЛЬЗЯ перехватить.
    • НЕЛЬЗЯ отложить.
    • Ядро убивает процесс мгновенно.
    • Не даёт ни шанса на cleanup.

  SIGINT (2) — «Ctrl+C»:
    • Аналог SIGTERM, но обычно от интерактивного терминала.
    • Можно перехватить.
    • Реже используется в контейнерах.

  SIGHUP (1) — «терминал отключился»:
    • Исторически — для reload конфигов.
    • Можно перехватить.
    • Редко в контейнерах.

  SIGQUIT (3) — «выход с core dump»:
    • Используется nginx для graceful shutdown.
    • Можно перехватить.
    • Опционально в других сервисах.

  ЧТО ВЫБРАТЬ В COMPOSE:
    По умолчанию — SIGTERM. Это стандарт, работает для 99%
    сервисов.

    Особые случаи:
      • Nginx исторически — SIGQUIT.
      • Некоторые legacy-сервисы — свой сигнал.

  ПРОВЕРКА, КАКОЙ СИГНАЛ ЖДЁТ ПРОЦЕСС:
    В документации образа. Или через `docker inspect`:
      docker inspect nginx:1.27-alpine -f '{{.Config.StopSignal}}'
      # (пусто) — дефолт SIGTERM.

  4. STOP_SIGNAL: КАКОЙ СИГНАЛ ПОСЛАТЬ
  Параметр `stop_signal` в compose задаёт, какой сигнал
  Compose посылает первым.

  СИНТАКСИС:
    services:
      app:
        stop_signal: SIGTERM    # дефолт
        # или
        stop_signal: SIGINT
        stop_signal: SIGQUIT
        stop_signal: SIGHUP
        stop_signal: SIGUSR1

  КОГДА МЕНЯТЬ:
    SIGTERM (дефолт):
      • Go-сервисы, Java, Python, Node.js — все современные
        рантаймы умеют ловить SIGTERM.
      • Оставляй как есть.

    SIGQUIT:
      • Nginx: graceful shutdown (finish requests, close listen
        socket).
      • Некоторые другие сервисы, если документация требует.

    SIGINT:
      • Если приложение по какой-то причине ловит SIGINT, а не
        SIGTERM.

    SIGUSR1:
      • Кастомные сценарии. Редко.

  ПРИМЕР ДЛЯ NGINX:
    services:
      nginx:
        image: nginx:1.27-alpine
        stop_signal: SIGQUIT

    Что происходит: nginx получает SIGQUIT, завершает текущие
    запросы, перестаёт принимать новые, закрывается корректно.

  ПРИМЕР ДЛЯ GO-СЕРВИСА:
    services:
      app:
        stop_signal: SIGTERM    # явно, хотя это дефолт

    В Go-коде:
      ctx, cancel := signal.NotifyContext(
          context.Background(),
          syscall.SIGTERM, syscall.SIGINT)
      defer cancel()

      <-ctx.Done()
      // graceful shutdown

  ВАЖНО: stop_signal не гарантирует, что процесс его
  перехватит. Если процесс не зарегистрировал обработчик —
  ядро применит дефолтное действие. Для SIGTERM это
  termination (процесс умирает).

  5. STOP_GRACE_PERIOD: СКОЛЬКО ЖДАТЬ
  Параметр `stop_grace_period` задаёт, сколько Compose ждёт
  после сигнала, прежде чем послать SIGKILL.

  ПО УМОЛЧАНИЮ: 10 секунд.

  СИНТАКСИС:
    services:
      app:
        stop_grace_period: 30s

    Единицы: s, m. Можно комбинировать: 1m30s.

  ЧТО ЭТО ЗНАЧИТ:
    t=0s      SIGTERM
    t=0-30s   grace period (процесс завершается)
    t=30s     SIGKILL (если процесс не завершился)

  КАК ВЫБРАТЬ ЗНАЧЕНИЕ:
    Зависит от worst-case времени graceful shutdown:

    Быстрые сервисы (nginx, redis):
      5-10 секунд.

    Сервисы с короткими запросами (API):
      15-30 секунд.

    Сервисы с длинными запросами (экспорт данных, отчёты):
      60-120 секунд.

    БД (postgres, mysql):
      30-60 секунд (flush WAL, checkpoint).

    Kafka:
      30-60 секунд (commit offsets, close).

  ПРИМЕРЫ:
    services:
      nginx:
        stop_grace_period: 10s

      app:
        stop_grace_period: 30s

      postgres:
        stop_grace_period: 60s

      migrate:
        stop_grace_period: 1m30s

  ЧТО ПРОИСХОДИТ, ЕСЛИ GRACE PERIOD МАЛ:
    Приложение не успевает завершить работу.
    SIGKILL убивает посреди операции.
    Результат: потеря данных, обрыв соединений.

  ЧТО ПРОИСХОДИТ, ЕСЛИ GRACE PERIOD ВЕЛИК:
    Ничего страшного. Если приложение завершилось за 2 секунды,
    Compose не ждёт все 30. Ждёт ровно столько, сколько нужно.
    Grace period — это верхняя граница, а не фиксированное время.

  ПРАВИЛО: ставь с запасом. Лучше 30 секунд, а завершится за 5,
  чем 5 секунд и убит на середине.

  6. PID 1 В КОНТЕЙНЕРЕ
  В контейнере первый процесс (PID 1) — это твой ENTRYPOINT.
  Не systemd, не init. Просто твой бинарник или скрипт.

  КОГДА PID 1 = ТВОЙ БИНАРНИК (exec-форма):

    ENTRYPOINT ["/app"]

    Что происходит при SIGTERM:
      • Сигнал идёт прямо в /app (PID 1).
      • Приложение ловит его.
      • Делает graceful shutdown.
      • Выходит.

  КОГДА PID 1 = SHELL (shell-форма):
    ENTRYPOINT /app

    Что происходит при SIGTERM:
      • Сигнал идёт в /bin/sh (PID 1).
      • Shell НЕ пробрасывает сигнал дочернему /app.
      • /app продолжает работать.
      • Через stop_grace_period — SIGKILL для shell.
      • /app умирает вместе с ним.
      • Graceful shutdown не срабатывает.

  ЭТО ПЕРВАЯ ГЛАВНАЯ ПРИЧИНА сломанного shutdown.

  ПРОВЕРКА:
    docker compose exec app ps aux
    # PID 1: /app         ← правильно (exec-форма)
    # PID 1: /bin/sh -c /app    ← неправильно (shell-форма)

  ИСПРАВЛЕНИЕ: только exec-форма в ENTRYPOINT.

    # Плохо.
    ENTRYPOINT /app

    # Хорошо.
    ENTRYPOINT ["/app"]

  Или если нужен entrypoint-скрипт для setup:
    ENTRYPOINT ["/bin/sh", "-c", "setup && exec /app"]

    Ключевое слово `exec` заменяет shell на /app. Тогда PID 1
    = /app, и сигнал доходит.

  7. SHELL-ФОРМА VS EXEC-ФОРМА ENTRYPOINT
  Развёрнуто — почему это критично.

  SHELL-ФОРМА (без квадратных скобок):
    ENTRYPOINT /app

    Разворачивается в:
      /bin/sh -c "/app"

    Процессы в контейнере:
      PID 1: /bin/sh -c /app
      PID 7: /app

    SIGTERM от Docker идёт в PID 1 = /bin/sh.
    Shell в состоянии wait() ждёт завершения /app.
    Shell НЕ пробрасывает SIGTERM.
    /app не получает сигнал.
    Через grace period — SIGKILL всё убивает.

  EXEC-ФОРМА (с квадратными скобками):

    ENTRYPOINT ["/app"]

    Процессы в контейнере:
      PID 1: /app

    SIGTERM от Docker идёт в PID 1 = /app.
    /app ловит сигнал.
    Graceful shutdown работает.

  РАЗНИЦА — ОДНА ПАРА КВАДРАТНЫХ СКОБОК.

  ПРОВЕРКА НА ПРАКТИКЕ:
    docker run --rm -d --name t alpine sleep 60
    docker exec t ps aux
    # PID 1 = sleep 60    ← exec-форма

    docker run --rm -d --name t2 alpine sh -c "sleep 60"
    docker exec t2 ps aux
    # PID 1 = /bin/sh -c sleep 60
    # PID 7 = sleep 60    ← shell-форма

  ОСОБЕННОСТЬ: некоторые shell'и (bash, dash) умеют exec
  для последней команды. Но не полагайся на это. Всегда
  используй явную exec-форму.

  КОМБИНАЦИЯ С CMD:
    ENTRYPOINT ["/app"]
    CMD ["--config", "/etc/app/config.yaml"]

    Что происходит: PID 1 = /app --config /etc/app/config.yaml.
    Сигнал доходит. Всё правильно.

  8. ПОРЯДОК ОСТАНОВКИ В COMPOSE
  Когда ты делаешь `docker compose down`, Compose останавливает
  сервисы в обратном порядке от старта.

  ПРАВИЛО: сервисы без зависимостей останавливаются первыми.
  Зависимые — последними.
  ПРИМЕР:
    services:
      postgres:
      migrate:
        depends_on:
          postgres:
      app:
        depends_on:
          migrate:

    Старт:     postgres → migrate → app
    Остановка: app → migrate → postgres

  ЧТО ЭТО ДАЁТ:
    • app завершается первым, пока postgres ещё жив.
    • Все соединения с БД корректно закрываются.
    • postgres останавливается последним, уже без клиентов.
    • Никаких обрывов.

  ЕСЛИ БЫ БЫЛО НАОБОРОТ:
    • postgres остановился.
    • app продолжает работать, но БД недоступна.
    • Ошибки в логах, retry, задержки.

  СИГНАЛ ОСТАНОВКИ ПО ЦЕПОЧКЕ:
    Compose посылает SIGTERM каждому сервису **параллельно**,
    но с учётом порядка. App получает сигнал первым, потом
    через несколько секунд — migrate, потом — postgres.

    Фактически Compose ждёт, пока зависимые сервисы остановятся,
    прежде чем послать сигнал зависимостям.

  ЭТО АВТОМАТИЧЕСКИ. Не нужно ничего настраивать. Достаточно
  правильно указать depends_on.

  9. DOCKER STOP VS KILL VS DOWN
  Три команды, разные эффекты.

  DOCKER COMPOSE STOP:
    docker compose stop

    • Посылает SIGTERM всем сервисам.
    • Ждёт stop_grace_period.
    • Если не завершились — SIGKILL.
    • Контейнеры остаются (не удаляются).
    • Сеть остаётся.
    • Volumes остаются.
    • Можно снова `docker compose start`.

    Используй для: временной остановки.

  DOCKER COMPOSE KILL:
    docker compose kill

    • Посылает SIGKILL сразу.
    • Без grace period.
    • Без SIGTERM.
    • Мгновенно.
    • Контейнеры остаются.

    Используй для: когда stop завис и не помогает.

  DOCKER COMPOSE DOWN:
    docker compose down

    • Посылает SIGTERM всем сервисам.
    • Ждёт stop_grace_period.
    • SIGKILL, если не успели.
    • Удаляет контейнеры.
    • Удаляет сеть.
    • Volumes остаются (кроме `-v`).

    Используй для: завершения работы проекта.

  ЧТО ВАЖНО:
    `stop` и `down` соблюдают graceful shutdown.
    `kill` — нет.

    В проде никогда не используй `kill` без веской причины.

  ВРЕМЯ ОСТАНОВКИ:
    Обычно занимает 1-5 секунд, если shutdown быстрый.
    До stop_grace_period, если shutdown медленный.
    Ровно stop_grace_period, если shutdown завис.

  ПРОВЕРКА:
    time docker compose stop
    # real    0m0.153s    ← быстро, shutdown сработал
    # real    0m10.004s   ← shutdown не сработал, ждали grace period

  10. СВОЁ ВРЕМЯ ДЛЯ РАЗНЫХ СЕРВИСОВ
  Разные сервисы имеют разное время shutdown. Настрой
  индивидуально.

  ПРИМЕР КОМПЛЕКСНОГО СТЕКА:
    services:
      nginx:
        image: nginx:1.27-alpine
        stop_grace_period: 10s
        stop_signal: SIGQUIT

      app:
        image: my-app:1.0
        stop_grace_period: 30s
        stop_signal: SIGTERM

      worker:
        image: my-worker:1.0
        stop_grace_period: 60s
        stop_signal: SIGTERM

      postgres:
        image: postgres:16-alpine
        stop_grace_period: 60s
        stop_signal: SIGTERM

      redis:
        image: redis:7-alpine
        stop_grace_period: 30s
        stop_signal: SIGTERM

      kafka:
        image: apache/kafka:3.7.0
        stop_grace_period: 90s
        stop_signal: SIGTERM

  ЛОГИКА:
    nginx: SIGQUIT, 10 секунд — finish requests.
    app: SIGTERM, 30 секунд — close DB connections.
    worker: SIGTERM, 60 секунд — finish current job.
    postgres: SIGTERM, 60 секунд — checkpoint, flush WAL.
    redis: SIGTERM, 30 секунд — save RDB.
    kafka: SIGTERM, 90 секунд — commit offsets, close.

  В ОДНОМ МЕСТЕ — ЧЕРЕЗ YAML ANCHORS (если хочется):
    x-common-stop: &common-stop
      stop_grace_period: 30s
      stop_signal: SIGTERM

    services:
      app:
        <<: *common-stop

      worker:
        <<: *common-stop
        stop_grace_period: 60s

  11. КАК ПРОВЕРИТЬ, ЧТО SHUTDOWN РАБОТАЕТ
  ТРИ СПОСОБА.

  СПОСОБ 1: ЗАМЕР ВРЕМЕНИ
    time docker compose stop app

    Если заняло 0-2 секунды — shutdown сработал.
    Если заняло ровно stop_grace_period — shutdown завис,
    SIGKILL пришёл по таймауту.
    Если заняло дольше 30 секунд — что-то не так.

  СПОСОБ 2: ЛОГИ
    docker compose logs app --tail 5

    Ищи строки вроде:

      {"msg":"shutting down"}
      {"msg":"stopped cleanly"}

    Если их нет — процесс убит SIGKILL, shutdown не сработал.

  СПОСОБ 3: EXIT CODE
    docker compose ps -a | Select-String app
    # Exited (0)       ← чистый выход
    # Exited (143)     ← SIGTERM, но обработан
    # Exited (137)     ← SIGKILL, принудительно убит

    Если 137 — shutdown не сработал. Смотри shell-форму
    ENTRYPOINT, отсутствие обработчика сигналов.

  ПРОВЕРКА PID 1:
    docker compose exec app ps aux
    # PID 1 = /app         ← exec-форма, ок
    # PID 1 = /bin/sh      ← shell-форма, плохо

  ПРОВЕРКА НАСТРОЕК:
    docker inspect <container> -f '{{.Config.StopSignal}}'
    docker inspect <container> -f '{{.HostConfig.StopTimeout}}'

  12. АНТИПАТТЕРНЫ
  12.1. SHELL-ФОРМА ENTRYPOINT.
    ENTRYPOINT /app
    # PID 1 = /bin/sh, SIGTERM не доходит до /app.
  12.2. НЕТ ОБРАБОТЧИКА SIGTERM В КОДЕ.
    Даже с exec-формой ENTRYPOINT, если приложение не ловит
    SIGTERM — оно умрёт по дефолтному обработчику.
  12.3. STOP_GRACE_PERIOD = 0.
    Если поставить 0 — Compose сразу посылает SIGKILL.
    Shutdown не успеет.
  12.4. СЛИШКОМ МАЛЕНЬКИЙ STOP_GRACE_PERIOD.
    Приложение завершается за 5 секунд, а grace period 3.
    SIGKILL на середине shutdown.
  12.5. ЗАБЫЛИ STOP_SIGNAL ДЛЯ NGINX.
    Nginx любит SIGQUIT для graceful. С SIGTERM работает,
    но менее изящно.
  12.6. ПОРЯДОК ОСТАНОВКИ ЧЕРЕЗ ЗАВИСИМОСТИ НЕПРАВИЛЬНЫЙ.
    App и Postgres без depends_on. Останавливаются
    параллельно. App не успевает закрыть соединения.
  12.7. ИСПОЛЬЗОВАНИЕ `KILL` ВМЕСТО `STOP`.
    docker compose kill — принудительно. Никакого graceful.
  12.8. ЗАБЫЛИ HEALTHCHECK ДЛЯ APP.
    Нет способа проверить, что app готов до отправки трафика
    (в k8s особенно).
  12.9. ВСЁ В ОДНОМ SERVER С GRACE 10S.
    Postgres нуждается в 30-60. Nginx — 10. Нельзя одно
    значение для всех.
  12.10. НЕ ЛОГИРОВАТЬ SHUTDOWN.
    Без логов «shutting down» невозможно понять, сработал ли
    graceful или SIGKILL.
  12.11. ЗАБЫТЬ ПРО STOP_GRACE_PERIOD В ПРОДЕ.
    В dev по умолчанию 10s ок. В проде под нагрузкой
    приложение не успевает.
  12.12. STOP_GRACE_PERIOD БОЛЬШЕ DEPLOY TIMEOUT.
    CI/CD ждёт 30 секунд на деплой. Grace period 60 секунд.
    Деплой падает по таймауту до завершения shutdown.

  13. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  docker compose stop по умолчанию: SIGTERM → 10 секунд → SIGKILL.
  2.  SIGTERM можно перехватить. SIGKILL — нельзя.
  3.  stop_signal задаёт сигнал. Дефолт SIGTERM, для nginx — SIGQUIT.
  4.  stop_grace_period задаёт время между SIGTERM и SIGKILL.
      Дефолт 10 секунд.
  5.  Exec-форма ENTRYPOINT обязательна. Shell-форма ломает
      доставку сигнала.
  6.  PID 1 в контейнере = ENTRYPOINT. Это не systemd.
  7.  Порядок остановки — обратный порядку запуска. По
      depends_on. Автоматически.
  8.  stop / down соблюдают graceful shutdown. kill — нет.
  9.  Exit code 137 = SIGKILL, 143 = SIGTERM, 0 = чисто.
  10. Своё время для каждого сервиса:
      nginx: 10s, app: 30s, postgres: 60s, kafka: 90s.
  11. Проверка: time docker compose stop, docker compose
      logs, exit code, ps aux.
  12. Антипаттерны: shell-форма ENTRYPOINT, нет обработчика
      SIGTERM, grace period 0 или слишком маленький, kill
      вместо stop, забыли stop_signal для nginx.
*/

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cmd := "server"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "server":
		// Обычный сервер. Shutdown за ~5 секунд.
		runServer(logger, 5*time.Second)
	case "server-slow":
		// Медленный shutdown. 25 секунд.
		runServer(logger, 25*time.Second)
	case "server-no-grace":
		// Без обработки SIGTERM. Умрёт по дефолтному handler'у.
		runServerNoGrace(logger)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

// runServer — правильный сервер с graceful shutdown.
func runServer(logger *slog.Logger, shutdownDelay time.Duration) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "pid=%d time=%s\n", os.Getpid(),
			time.Now().Format(time.RFC3339))
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}

	// signal.NotifyContext — правильный способ ловить SIGTERM.
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	go func() {
		logger.Info("server started", "addr", srv.Addr, "pid", os.Getpid())
		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("SIGTERM received, graceful shutdown started",
		"delay", shutdownDelay)

	// Даём время завершить текущие запросы.
	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(), shutdownDelay+5*time.Second)
	defer cancelShutdown()

	// Эмулируем длинную работу (например, закрытие БД-соединений,
	// commit транзакций, flush буферов).
	time.Sleep(shutdownDelay)

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("stopped cleanly")
}

// runServerNoGrace — БЕЗ обработки SIGTERM.
// Демонстрация: default handler убивает процесс.
func runServerNoGrace(logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}

	logger.Info("server-no-grace started",
		"addr", srv.Addr, "pid", os.Getpid())

	// Никакого signal.Notify. SIGTERM убьёт процесс мгновенно.
	if err := srv.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "err", err)
		os.Exit(1)
	}
}
