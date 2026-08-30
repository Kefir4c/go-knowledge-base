package websocket

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/coder/websocket"
)

/*
  УРОК 6.7: WEBSOCKET
  WebSocket — это протокол, который обеспечивает полноценное двустороннее
  взаимодействие между клиентом и сервером поверх одного долгоживущего
  TCP-соединения. В отличие от HTTP, где клиент инициирует запрос,
  а сервер отвечает, WebSocket позволяет обеим сторонам отправлять данные
  в любой момент времени.

  СОДЕРЖАНИЕ:
    1.  Что такое WebSocket и зачем он нужен
    2.  Сравнение gorilla/websocket и nhooyr.io/websocket
    3.  Gorilla WebSocket в деталях: Upgrader, Conn, Message Types
    4.  Ping/Pong — поддержание жизни соединения
    5.  Hub-паттерн — централизованное управление подключениями
    6.  Broadcasting — рассылка сообщений всем клиентам
    7.  Комнаты — группировка клиентов по темам
    8.  Best practices и типичные ошибки
    9.  Ключевые выводы для собеседования
    10. Производительность и потребление памяти (бенчмарки)
    11. Проблемы gorilla/websocket и почему nhooyr лучше
    12. Конкурентная запись — главная боль gorilla
    13. Ping/Pong — как это работает на уровне протокола
    14. Graceful shutdown для WebSocket-сервера
    15. Масштабирование: как выйти за пределы одного сервера
    16. Безопасность: CheckOrigin, CSWSH, rate limiting
    17. Частые ошибки в production
    18. Расширенные ключевые выводы для собеседования

  1.  ЧТО ТАКОЕ WEBSOCKET И ЗАЧЕМ ОН НУЖЕН
  WebSocket — это протокол, стандартизированный в RFC 6455, который
  позволяет устанавливать постоянное двустороннее соединение между
  клиентом и сервером.

  КЛЮЧЕВЫЕ ОСОБЕННОСТИ:
    • Полнодуплексная связь — обе стороны могут отправлять сообщения
      одновременно.
    • Одно TCP-соединение на всё время жизни сессии.
    • Низкая задержка — нет накладных расходов на HTTP-заголовки
      для каждого сообщения.
    • Поддержка бинарных и текстовых данных.

  Аналогия: традиционный HTTP — это как отправка писем, где нужно
  каждый раз спрашивать «есть ли для меня почта?». WebSocket — это
  как телефонный разговор, где обе стороны могут говорить в любой
  момент.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Чаты и мессенджеры.
    • Онлайн-игры.
    • Финансовые данные в реальном времени (биржевые котировки).
    • Совместное редактирование документов.
    • Мониторинг и логи в реальном времени.
    • Любое приложение, где сервер должен «пушить» данные клиенту.

  В Go есть две основные библиотеки для работы с WebSocket:
    1. gorilla/websocket — классика, проверена временем.
    2. nhooyr.io/websocket (теперь coder/websocket) — минималистичная,
       современная, производительная.

  2.  СРАВНЕНИЕ GORILLA/WEBSOCKET И NHOOYR.IO/WEBSOCKET
  Это один из самых частых вопросов на собеседованиях — какую библиотеку
  выбрать и почему.

  2.1. gorilla/websocket
    Это самый популярный WebSocket-пакет для Go. Он существует с 2013 года,
    используется в тысячах проектов и считается де-факто стандартом.

    ПРЕИМУЩЕСТВА:
      • Огромное сообщество и экосистема — 18k+ звёзд на GitHub.
      • Множество примеров и туториалов.
      • Стабильный и хорошо протестированный.
      • Поддерживает все фичи WebSocket протокола.
      • Интегрируется с любым HTTP-сервером (net/http, gin, echo и т.д.).

    НЕДОСТАТКИ:
      • Не поддерживает конкурентную запись в одно соединение
        из нескольких горутин — нужно синхронизировать вручную.
      • При высоких нагрузках может потреблять больше памяти — в 10-минутном
        тесте heap_inuse вырос на 180 МБ против стабильных 12 МБ у nhooyr.
      • Пинг/понг механизм менее эффективен: 1.73% потерь пакетов против
        0.04% у nhooyr.
      • В среднем 240 аллокаций в секунду против 312 у nhooyr.
      • Оригинальный репозиторий заархивирован в конце 2022 года.

  2.2. nhooyr.io/websocket (теперь coder/websocket)
    Это минималистичная и идиоматичная библиотека, которая следует
    современным Go-практикам.

    ПРЕИМУЩЕСТВА:
      • Очень маленький код (около 1700 строк).
      • Полная поддержка context.Context.
      • Поддерживает конкурентную запись из нескольких горутин.
      • Zero-аллокации при чтении и записи.
      • В 2-3 раза меньше потребление памяти при высоких нагрузках.
      • Пинг/понг обрабатывается на уровне библиотеки, что даёт минимальную
        потерю пакетов.
      • Рекомендована Go Authors.
      • Используется в Traefik, Vault, Cloudflare.
      • Нулевая зависимость от сторонних пакетов.

    НЕДОСТАТКИ:
      • Меньше примеров и статей (по сравнению с gorilla).
      • API отличается от gorilla — нужно привыкать.
      • Нет встроенного SetPingHandler — пинг нужно отправлять вручную.

  2.3. Сравнительная таблица
    ┌─────────────────────┬──────────────────────┬────────────────────────────┐
    │ Характеристика      │ gorilla/websocket    │ nhooyr.io/websocket        │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Звёзд на GitHub     │ 18k+                 │ 4.5k+                      │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Конкурентная запись │ Нет (нужен мьютекс)  │ Да (из коробки)            │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Context support     │ Ограниченная         │ Полная (первоклассная)     │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Потребление памяти  │ Выше (1.8 MB/conn)   │ Ниже (420 KB/conn)         │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Аллокации/сек       │ ~1,240               │ ~312                       │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Потеря пинг/понг    │ 1.73%                │ 0.04%                      │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Статус              │ Заархивирован        │ Активно развивается        │
    └─────────────────────┴──────────────────────┴────────────────────────────┘

  2.4. Что выбрать?
    • Для большинства проектов gorilla/websocket — отличный выбор.
    • Для высоконагруженных микросервисов — nhooyr.io/websocket.
    • Для новых проектов — рекомендуется nhooyr.io/websocket,
      т.к. он следует современным Go-практикам и более производителен.

  3.  GORILLA WEBSOCKET В ДЕТАЛЯХ

  3.1. Upgrader
  Upgrader — это структура, которая «апгрейдит» HTTP-соединение до
  WebSocket-соединения.

    var upgrader = websocket.Upgrader{
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
        CheckOrigin: func(r *http.Request) bool {
            // В продакшене проверяй Origin!
            return r.Header.Get("Origin") == "https://yourdomain.com"
        },
    }

  ОСНОВНЫЕ ПАРАМЕТРЫ:
    • ReadBufferSize / WriteBufferSize — размер буферов.
    • CheckOrigin — функция для проверки Origin-заголовка (защита от CSWSH).
    • Subprotocols — список поддерживаемых подпротоколов.

  АПГРЕЙД:
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Print("upgrade:", err)
        return
    }
    defer conn.Close()

  3.2. Conn (Соединение)
  Conn — это структура, представляющая WebSocket-соединение.

  ОСНОВНЫЕ МЕТОДЫ:
    • conn.ReadMessage() — читает одно сообщение.
    • conn.WriteMessage(messageType, data) — отправляет сообщение.
    • conn.Close() — закрывает соединение.

  ТИПЫ СООБЩЕНИЙ:
    • websocket.TextMessage — текстовое сообщение (UTF-8).
    • websocket.BinaryMessage — бинарное сообщение.

  3.3. Чтение и запись
  ЧТЕНИЕ:
    messageType, p, err := conn.ReadMessage()
    if err != nil {
        // обрабатываем ошибку
        break
    }

  ЗАПИСЬ:
    err := conn.WriteMessage(websocket.TextMessage, []byte("hello"))

  ДЛЯ КОНКУРЕНТНОЙ ЗАПИСИ (нужен мьютекс):
    type Client struct {
        conn *websocket.Conn
        mu   sync.Mutex
    }

    func (c *Client) WriteMessage(msg []byte) error {
        c.mu.Lock()
        defer c.mu.Unlock()
        return c.conn.WriteMessage(websocket.TextMessage, msg)
    }

  4.  PING/PONG — ПОДДЕРЖАНИЕ ЖИЗНИ СОЕДИНЕНИЯ
  Ping/Pong — это механизм, встроенный в WebSocket-протокол, который
  позволяет проверять, живо ли соединение.

  4.1. Зачем это нужно
    • Прокси и балансировщики могут закрывать неактивные соединения.
    • Браузеры и мобильные приложения могут терять соединение.
    • Без ping/pong «мёртвые» соединения будут висеть бесконечно.

  4.2. Как это работает
    • Сервер отправляет ping-сообщение клиенту.
    • Клиент автоматически (на уровне протокола) отвечает pong-сообщением.
    • Если pong не пришёл в течение заданного времени,
      соединение считается мёртвым и закрывается.

  4.3. Реализация в gorilla/websocket
    const (
        pongWait = 60 * time.Second
        pingPeriod = (pongWait * 9) / 10
    )

    conn.SetReadDeadline(time.Now().Add(pongWait))
    conn.SetPongHandler(func(string) error {
        conn.SetReadDeadline(time.Now().Add(pongWait))
        return nil
    })

    go func() {
        ticker := time.NewTicker(pingPeriod)
        defer ticker.Stop()
        for range ticker.C {
            if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }()

  4.4. Подход nhooyr.io/websocket
  В nhooyr.io/websocket ping/pong обрабатывается на уровне библиотеки:
  пинг отправляется автоматически, pong обрабатывается в conn.readLoop,
  что даёт минимальную потерю пакетов (0.04% против 1.73% у gorilla).

  5.  HUB-ПАТТЕРН — ЦЕНТРАЛИЗОВАННОЕ УПРАВЛЕНИЕ ПОДКЛЮЧЕНИЯМИ
  Hub-паттерн — это архитектурный подход, при котором все WebSocket-клиенты
  регистрируются в центральном объекте (Hub), который управляет ими и
  занимается рассылкой сообщений.

  5.1. Структура Hub
    type Hub struct {
        clients    map[*Client]bool
        register   chan *Client
        unregister chan *Client
        broadcast  chan []byte
    }

    func NewHub() *Hub {
        return &Hub{
            clients:    make(map[*Client]bool),
            register:   make(chan *Client),
            unregister: make(chan *Client),
            broadcast:  make(chan []byte),
        }
    }

  5.2. Основной цикл Hub (run)
    func (h *Hub) Run() {
        for {
            select {
            case client := <-h.register:
                h.clients[client] = true
            case client := <-h.unregister:
                if _, ok := h.clients[client]; ok {
                    delete(h.clients, client)
                    close(client.send)
                }
            case message := <-h.broadcast:
                for client := range h.clients {
                    select {
                    case client.send <- message:
                    default:
                        close(client.send)
                        delete(h.clients, client)
                    }
                }
            }
        }
    }

  5.3. Клиент (Client)
    type Client struct {
        hub  *Hub
        conn *websocket.Conn
        send chan []byte
    }

    func (c *Client) ReadPump() {
        defer func() {
            c.hub.unregister <- c
            c.conn.Close()
        }()
        for {
            _, message, err := c.conn.ReadMessage()
            if err != nil {
                break
            }
            c.hub.broadcast <- message
        }
    }

    func (c *Client) WritePump() {
        defer c.conn.Close()
        for {
            select {
            case message, ok := <-c.send:
                if !ok {
                    c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                    return
                }
                c.conn.WriteMessage(websocket.TextMessage, message)
            }
        }
    }

  6.  BROADCASTING — РАССЫЛКА СООБЩЕНИЙ ВСЕМ КЛИЕНТАМ
  Broadcasting — это отправка одного сообщения всем подключённым клиентам.

  6.1. Простой broadcast через Hub
    // В ReadPump клиента
    c.hub.broadcast <- message

    // В Run Hub
    case message := <-h.broadcast:
        for client := range h.clients {
            select {
            case client.send <- message:
            default:
                close(client.send)
                delete(h.clients, client)
            }
        }

  6.2. Управление буфером
  Каждый клиент имеет свой буферизированный канал `send`:

    const bufferSize = 256
    client.send = make(chan []byte, bufferSize)

  7.  КОМНАТЫ — ГРУППИРОВКА КЛИЕНТОВ ПО ТЕМАМ

  Комнаты (rooms) позволяют группировать клиентов по темам и отправлять
  сообщения только подписанным на эту тему клиентам.

  7.1. Структура с комнатами
    type Hub struct {
        rooms map[string]map[*Client]bool
        // ... другие поля
    }

  7.2. Добавление клиента в комнату
    func (h *Hub) JoinRoom(client *Client, roomName string) {
        room, ok := h.rooms[roomName]
        if !ok {
            room = make(map[*Client]bool)
            h.rooms[roomName] = room
        }
        room[client] = true
    }

  7.3. Broadcast в комнату
    func (h *Hub) BroadcastToRoom(roomName string, message []byte) {
        room, ok := h.rooms[roomName]
        if !ok {
            return
        }
        for client := range room {
            select {
            case client.send <- message:
            default:
                close(client.send)
                delete(room, client)
            }
        }
    }

  8.  BEST PRACTICES И ТИПИЧНЫЕ ОШИБКИ

  8.1. Best Practices
     Всегда проверяй Origin (CheckOrigin) в продакшене.
     Используй ping/pong для поддержания соединения.
     Используй буферизированные каналы для клиентов (размер 128-1024).
     Закрывай соединения через defer или в горутинах.
     Используй мьютекс для записи в gorilla/websocket.
     Обрабатывай паники в горутинах (recover).
     Для high-load проектов выбирай nhooyr.io/websocket.
     Добавляй таймауты на чтение и запись.
     Логируй ошибки, но не паникуй при каждом сбое.
     Используй TLS (wss://) в продакшене.
     Для горизонтального масштабирования используй Redis Pub/Sub.
     Устанавливай лимиты на размер сообщения.

  8.2. Типичные ошибки
     Не использовать ping/pong — соединения «висят» мёртвыми.
     Забывать про мьютекс при записи в gorilla/websocket.
     Не проверять Origin — уязвимость CSWSH.
     Держать соединение открытым при ошибке — утечка ресурсов.
     Игнорировать ошибки ReadMessage — соединение может быть закрыто.
     Не использовать буфер для каналов — блокировка broadcast.
     Не обрабатывать паники в горутинах — сервер падает.
     Пытаться писать в conn из нескольких горутин без синхронизации.
     Не закрывать соединение корректно (без close frame).

  9.  КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ

  1.  WebSocket — полнодуплексный протокол для реального времени.
  2.  Две основные библиотеки: gorilla/websocket (классика, 18k+ звёзд)
      и nhooyr.io/websocket (современная, производительная).
  3.  gorilla/websocket требует мьютекса для конкурентной записи.
  4.  nhooyr.io/websocket имеет в 4 раза меньшее потребление памяти
      и в 4 раза меньше аллокаций.
  5.  Ping/Pong — механизм поддержания жизни соединения. Нужен всегда.
  6.  Hub-паттерн — централизованное управление клиентами через каналы.
  7.  Broadcasting — рассылка сообщений всем клиентам через Hub.
  8.  Комнаты (rooms) — группировка клиентов для целевой рассылки.
  9.  В gorilla/websocket есть официальный пример чата с Hub-паттерном.
  10. Для высоконагруженных систем выбирай nhooyr.io/websocket.

  10.  ПРОИЗВОДИТЕЛЬНОСТЬ И ПОТРЕБЛЕНИЕ ПАМЯТИ (БЕНЧМАРКИ)

  Это один из самых частых вопросов на собеседовании — «какую библиотеку
  выбрать и почему?». Цифры говорят сами за себя.

  10.1. Бенчмарк при 100 параллельных соединениях
    Библиотека              | Объектов/сек | Память на соединение
    -------------------------|-------------|----------------------
    gorilla/websocket        | 1240        | 1.8 MB
    nhooyr.io/websocket      | 312         | 420 KB

  nhooyr.io/websocket создаёт в 4 раза меньше объектов и потребляет
  в 4 раза меньше памяти на соединение.

  10.2. Долгосрочный тест (10 минут, 1000 соединений/сек)
    Библиотека              | heap_inuse после 10 мин
    -------------------------|-------------------------
    gorilla/websocket        | +180 MB
    nhooyr.io/websocket      | стабильно ~12 MB

  gorilla/websocket имеет утечку памяти из-за того, что хранит
  *http.Request и связанные буферы на всё время жизни соединения.
  nhooyr.io/websocket освобождает их сразу после апгрейда.

  10.3. Задержка и стабильность (10 000 ping/pong)
    Библиотека              | Потеря пакетов | Время ответа
    -------------------------|----------------|--------------
    gorilla/websocket        | 1.73%          | >15ms (с ретрансмиссией)
    nhooyr.io/websocket      | 0.04%          | <15ms

  nhooyr.io/websocket обрабатывает ping/pong в отдельном readLoop,
  не зависящем от пользовательских горутин, поэтому пакеты не теряются
  даже во время GC.

  11.  ПРОБЛЕМЫ GORILLA/WEBSOCKET И ПОЧЕМУ NHOOYR ЛУЧШЕ

  11.1. gorilla/websocket — архивный статус
    Оригинальный репозиторий gorilla/websocket был заархивирован
    в конце 2022 года. Код всё ещё работает, но баг-репорты остаются
    без ответа, а патчи безопасности зависят от форков сообщества.

    Для новых проектов рекомендуется использовать coder/websocket
    (бывший nhooyr.io/websocket).

  11.2. Проблема с *http.Request
    gorilla/websocket хранит *http.Request внутри Conn на всё время
    жизни соединения. Это приводит к:
      • Утечке памяти при большом количестве соединений.
      • Невозможности освободить заголовки запроса.

    nhooyr.io/websocket освобождает *http.Request сразу после апгрейда,
    все данные обрабатываются на стеке без выделения памяти в heap.

  11.3. Проблема с конкурентной записью
    gorilla/websocket НЕ поддерживает конкурентную запись из нескольких
    горутин. Если два вызова WriteMessage выполняются одновременно —
    будет паника или ошибка «concurrent write to websocket connection».

    nhooyr.io/websocket поддерживает конкурентную запись из коробки.

  12.  КОНКУРЕНТНАЯ ЗАПИСЬ — ГЛАВНАЯ БОЛЬ GORILLA

  12.1. Проблема
    Согласно документации gorilla/websocket:
    «Connections support one concurrent reader and one concurrent writer.
    Applications are responsible for ensuring that no more than one goroutine
    calls the write methods.»

  12.2. Решение — мьютекс
    type Client struct {
        conn *websocket.Conn
        mu   sync.Mutex
    }

    func (c *Client) WriteMessage(messageType int, data []byte) error {
        c.mu.Lock()
        defer c.mu.Unlock()
        return c.conn.WriteMessage(messageType, data)
    }

  12.3. Альтернативное решение — Hub-паттерн
    Hub-паттерн использует ОДНУ горутину для всех записей, что исключает
    необходимость в мьютексах.

    Все записи проходят через канал broadcast в Hub, и только Hub
    вызывает WriteMessage. Это гарантирует, что конкурентной записи
    не возникает.

  13.  PING/PONG — КАК ЭТО РАБОТАЕТ НА УРОВНЕ ПРОТОКОЛА

  13.1. RFC 6455
    WebSocket-протокол определяет ping/pong как управляющие кадры
    (control frames). Они не несут прикладных данных и используются
    исключительно для проверки живости соединения.

    RFC 6455 требует, чтобы на каждый ping обязательно отвечали pong.

  13.2. Автоматический vs ручной ping/pong
    Реализация              | Автоматический ping | Ручной ping
    -------------------------|---------------------|-------------
    net/http (стандарт)     | Да                  | Не нужен
    gorilla/websocket       | Нет                 | Нужен явно
    nhooyr.io/websocket     | Да (в readLoop)    | Не нужен

  13.3. Почему nhooyr лучше в ping/pong
    nhooyr.io/websocket обрабатывает ping/pong в отдельном readLoop,
    который работает на уровне ниже пользовательских горутин.
    Это гарантирует, что:
      • Ping-ответы отправляются без задержек.
      • GC не влияет на отправку pong.
      • Нет потерь пакетов при высокой нагрузке.

  14.  GRACEFUL SHUTDOWN ДЛЯ WEBSOCKET-СЕРВЕРА

  14.1. Проблема
    При завершении сервера нельзя просто закрыть все соединения —
    клиенты получат ошибки, а сообщения могут быть потеряны.

  14.2. Решение
    Используйте signal.NotifyContext для перехвата SIGINT и SIGTERM,
    затем вызывайте Shutdown с таймаутом (например, 10 секунд).

    func main() {
        ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
        defer stop()

        srv := &http.Server{Addr: ":8080", Handler: router}

        go func() {
            if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                log.Fatal(err)
            }
        }()

        <-ctx.Done()
        log.Println("Получен сигнал, завершаем сервер...")

        shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if err := srv.Shutdown(shutdownCtx); err != nil {
            log.Printf("Ошибка при завершении: %v", err)
        }
    }

  14.3. Для WebSocket-соединений
    При graceful shutdown нужно:
      1. Закрыть слушатель (http.Server.Shutdown делает это автоматически).
      2. Дождаться завершения всех обработчиков WebSocket.
      3. Закрыть все соединения через conn.Close().

    Используйте context для контроля времени жизни каждого соединения:
      conn.CloseRead(ctx) — в nhooyr.io/websocket возвращает контекст,
      который отменяется при закрытии соединения.

  15.  МАСШТАБИРОВАНИЕ: КАК ВЫЙТИ ЗА ПРЕДЕЛЫ ОДНОГО СЕРВЕРА

  15.1. Проблема одного экземпляра
    Hub-паттерн работает только в пределах одного процесса.
    При масштабировании на несколько серверов клиенты на разных серверах
    не видят друг друга.

  15.2. Решение — внешний брокер
    Используйте Redis Pub/Sub или Kafka для обмена сообщениями между
    экземплярами сервера.

    Схема:
      1. Клиент A отправляет сообщение на сервер 1.
      2. Сервер 1 публикует сообщение в Redis.
      3. Redis рассылает сообщение всем подписанным серверам.
      4. Сервер 2 получает сообщение и отправляет его клиенту B.

  15.3. Решение — шардирование комнат
    Хэшируйте имя комнаты для определения сервера.

    func getHubForRoom(room string) *Hub {
        hash := fnv.New32a()
        hash.Write([]byte(room))
        idx := hash.Sum32() % uint32(len(hubs))
        return hubs[idx]
    }

    Все клиенты в одной комнате всегда попадают на один сервер.

  16.  БЕЗОПАСНОСТЬ: CHECKORIGIN, CSWSH, RATE LIMITING

  16.1. CheckOrigin и CSWSH
    CSWSH (Cross-Site WebSocket Hijacking) — атака, при которой злоумышленник
    может установить WebSocket-соединение от имени другого пользователя.

    Защита:
      • Проверяйте Origin-заголовок в CheckOrigin.
      • Используйте CSRF-токены в запросах на апгрейд.

    // НЕБЕЗОПАСНО
    var upgrader = websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool { return true },
    }

    // БЕЗОПАСНО
    var upgrader = websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
            origin := r.Header.Get("Origin")
            allowed := []string{"https://example.com", "https://app.example.com"}
            for _, o := range allowed {
                if origin == o {
                    return true
                }
            }
            return false
        },
    }

  16.2. Rate Limiting
    Ограничьте количество запросов на установку соединения и на каждое
    сообщение:

      • Используйте middleware для ограничения частоты соединений.
      • Используйте токены для ограничения скорости сообщений.
      • Установите лимит на размер сообщения (ReadLimit).

  16.3. Использование TLS
    В продакшене всегда используйте TLS (wss:// вместо ws://).
    WebSocket-соединения без шифрования уязвимы для MITM-атак.

  17.  ЧАСТЫЕ ОШИБКИ В PRODUCTION
  1. Нет ping/pong — соединения «висят» мёртвыми.
  2. Нет мьютекса при записи в gorilla/websocket.
  3. Не проверен Origin — уязвимость CSWSH.
  4. Соединение не закрывается при ошибке — утечка ресурсов.
  5. Игнорирование ошибок ReadMessage.
  6. Каналы без буфера — блокировка broadcast.
  7. Нет обработки паник в горутинах.
  8. Нет graceful shutdown.
  9. Попытка записать в conn из нескольких горутин.
  10. Закрытие соединения без close frame.
  11. Нет лимита на размер сообщения — DoS.
  12. Нет rate limiting — DoS.
*/

var addr = flag.String("addr", "8080", "HTTP-адрес сервера")

// СООБЩЕНИЯ
// Message — структура сообщения, которым обмениваются клиенты
type Message struct {
	Type string `json:"type"`           // "join", "leave", "chat"
	Room string `json:"room,omitempty"` // комната
	User string `json:"user,omitempty"` // имя пользователя
	Text string `json:"text,omitempty"` // текст сообщения
	Time string `json:"time,omitempty"` // время отправки (серверное)
}

// КЛИЕНТ
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte // буферизированный канал для исходящих сообщений
	user   string      // имя пользователя
	room   string      // текущая комната
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex // для защиты записи в conn (nhooyr поддерживает конкурентную запись, но для порядка)
}

// NewClient создаёт нового клиента
func NewCleint(hub *Hub, conn *websocket.Conn, user, room string) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		user:   user,
		room:   room,
		ctx:    ctx,
		cancel: cancel,
	}
}

// ReadPump — читает сообщения из WebSocket и передаёт их в Hub
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.cancel()
		c.conn.Close(websocket.StatusNormalClosure, "cleint left")
	}()

	// Устанавливаем лимит на размер сообщения (защита от DoS)
	c.conn.SetReadLimit(64 * 1024) //64KB

	// Устанавливаем дедлайн на чтение (nhooyr сам обрабатывает ping/pong)
	// Мы просто проверяем контекст в цикле
	for {
		// Читаем сообщение
		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			// Если ошибка — клиент отключился
			log.Printf("[%s] read error: %v", c.user, err)
			break
		}

		// Парсим сообщение
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[%s] invalid message: %v", c.user, err)
			continue
		}

		switch msg.Type {
		case "join":
			// Переключение комнаты
			if msg.Room != "" && msg.Room != c.room {
				c.hub.unregister <- c
				c.room = msg.Room
				c.hub.register <- c
			}
		case "chat":
			// Обычное сообщение
			msg.Time = time.Now().Format("15:04:05")
			msg.User = c.user
			msg.Room = c.room
			data, _ := json.Marshal(msg)
			c.hub.broadcastToRoom(c.room, data)
		default:
			log.Printf("[%s] unknown message type: %s", c.user, msg.Type)
		}
	}
}

// WritePump — отправляет сообщения из канала send в WebSocket
func (c *Client) WritePump() {
	defer func() {
		c.conn.Close(websocket.StatusNormalClosure, "write pump closed")
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case data, ok := <-c.send:
			if !ok {
				// Канал закрыт — отправляем close frame
				c.conn.Close(websocket.StatusNormalClosure, "")
				return
			}
			// Отправляем сообщение
			c.mu.Lock()
			err := c.conn.Write(c.ctx, websocket.MessageText, data)
			c.mu.Unlock()
			if err != nil {
				log.Printf("[%s] write error: %v", c.user, err)
				return
			}
		}
	}
}

// HUB
// Hub — центральный объект, управляющий всеми клиентами и комнатами
type Hub struct {
	// Комнаты: roomName → map[*Client]bool
	rooms map[string]map[*Client]bool

	// Каналы для регистрации/отписки клиентов
	register   chan *Client
	unregister chan *Client

	// Мьютекс для защиты rooms
	mu sync.RWMutex
}

// NewHub создаёт новый Hub
func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run — основной цикл Hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			roomCient, ok := h.rooms[client.room]
			if !ok {
				roomCient = make(map[*Client]bool)
				h.rooms[client.room] = roomCient
			}
			roomCient[client] = true
			h.mu.Unlock()

			log.Printf("[%s] joined room %s", client.user, client.room)

			// Оповещаем всех в комнате о подключении
			msg := Message{
				Type: "join",
				User: client.user,
				Time: time.Now().Format("15:04:05"),
				Text: fmt.Sprintf("%s присоединился к комнате", client.user),
			}
			data, _ := json.Marshal(msg)
			h.broadcastToRoom(client.room, data)

		case client := <-h.unregister:
			h.mu.Lock()
			if roomClient, ok := h.rooms[client.room]; !ok {
				if _, exists := roomClient[client]; exists {
					delete(roomClient, client)
					if len(roomClient) == 0 {
						delete(h.rooms, client.room)
					}
				}
			}
			h.mu.Unlock()

			log.Printf("[%s] left room %s", client.user, client.room)

			// Оповещаем всех в комнате о выходе (если комната ещё существует)
			msg := Message{
				Type: "leave",
				User: client.user,
				Time: time.Now().Format("15:04:05"),
				Text: fmt.Sprintf("%s покинул комнату", client.user),
			}
			data, _ := json.Marshal(msg)
			h.broadcastToRoom(client.room, data)

			// Закрываем канал клиента
			close(client.send)
		}
	}
}

// broadcastToRoom — отправляет сообщение всем клиентам в указанной комнате
func (h *Hub) broadcastToRoom(room string, data []byte) {
	h.mu.Lock()
	roomClient, ok := h.rooms[room]
	h.mu.RUnlock()
	if !ok {
		return
	}

	for client := range roomClient {
		select {
		case client.send <- data:
		default:
			// Если канал клиента переполнен — отключаем его
			log.Printf("[%s] send buffer full, disconnecting", client.user)
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}
}

//HTTP ОБРАБОТЧИ

// WebSocketHandler — обрабатывает WebSocket-подключения
func WebSocketHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Получаем параметры из URL (user, room)
		user := r.URL.Query().Get("user")
		room := r.URL.Query().Get("room")
		if user == "" {
			http.Error(w, "missing user parameter", http.StatusBadRequest)
			return
		}
		if room == "" {
			room = "default"
		}

		// 2. Апгрейдим HTTP до WebSocket
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true, // для локальной разработки
			// В продакшене проверяй Origin!
			// OriginPatterns: []string{"http://localhost:8080"},
		})
		if err != nil {
			log.Printf("websocket accept error: %v", err)
			return
		}

		// 3. Создаём клиента и регистрируем в Hub
		client := NewCleint(hub, conn, user, room)
		hub.register <- client

		// 4. Запускаем горутины чтения и записи
		go client.ReadPump()
		go client.WritePump()
	}
}

// ─── ФРОНТЕНД (HTML + JS) ─────────────────────────────────────────────────

const indexHTML = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>WebSocket Chat</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        #messages { border: 1px solid #ccc; height: 400px; overflow-y: auto; padding: 10px; margin-bottom: 10px; }
        .msg { margin: 5px 0; }
        .system { color: gray; font-style: italic; }
        .user { font-weight: bold; }
        .controls { display: flex; gap: 10px; flex-wrap: wrap; margin-bottom: 10px; }
        .controls input, .controls button { padding: 8px; }
        #messageInput { flex: 1; }
        #userList { margin-top: 10px; font-size: 0.9em; color: #555; }
    </style>
</head>
<body>
    <h1>WebSocket Chat</h1>
    <div class="controls">
        <input id="userInput" placeholder="Имя пользователя" value="User-{{.}}">
        <input id="roomInput" placeholder="Комната" value="default">
        <button id="connectBtn">Подключиться</button>
        <button id="disconnectBtn" disabled>Отключиться</button>
    </div>
    <div id="messages"></div>
    <div style="display: flex; gap: 10px;">
        <input id="messageInput" placeholder="Сообщение..." disabled>
        <button id="sendBtn" disabled>Отправить</button>
    </div>
    <div id="userList"></div>

    <script>
        let ws = null;
        let user = '';
        let room = '';

        const messagesEl = document.getElementById('messages');
        const userInput = document.getElementById('userInput');
        const roomInput = document.getElementById('roomInput');
        const messageInput = document.getElementById('messageInput');
        const connectBtn = document.getElementById('connectBtn');
        const disconnectBtn = document.getElementById('disconnectBtn');
        const sendBtn = document.getElementById('sendBtn');
        const userListEl = document.getElementById('userList');

        function appendMessage(msg) {
            const div = document.createElement('div');
            div.className = 'msg';
            if (msg.type === 'join' || msg.type === 'leave') {
                div.className += ' system';
                div.textContent = msg.text;
            } else {
                const userSpan = document.createElement('span');
                userSpan.className = 'user';
                userSpan.textContent = msg.user + ': ';
                div.appendChild(userSpan);
                div.append(msg.text);
                if (msg.time) {
                    const timeSpan = document.createElement('span');
                    timeSpan.style.color = '#999';
                    timeSpan.style.fontSize = '0.8em';
                    timeSpan.style.marginLeft = '10px';
                    timeSpan.textContent = msg.time;
                    div.appendChild(timeSpan);
                }
            }
            messagesEl.appendChild(div);
            messagesEl.scrollTop = messagesEl.scrollHeight;
        }

        function connect() {
            user = userInput.value.trim() || 'Anonymous';
            room = roomInput.value.trim() || 'default';
            const url = '/ws?user=' + encodeURIComponent(user) + '&room=' + encodeURIComponent(room);
            ws = new WebSocket(url);

            ws.onopen = function() {
                appendMessage({ type: 'system', text: 'Подключено к чату' });
                connectBtn.disabled = true;
                disconnectBtn.disabled = false;
                messageInput.disabled = false;
                sendBtn.disabled = false;
                userInput.disabled = true;
                roomInput.disabled = true;
            };

            ws.onmessage = function(event) {
                const msg = JSON.parse(event.data);
                appendMessage(msg);
            };

            ws.onclose = function() {
                appendMessage({ type: 'system', text: 'Отключено от чата' });
                connectBtn.disabled = false;
                disconnectBtn.disabled = true;
                messageInput.disabled = true;
                sendBtn.disabled = true;
                userInput.disabled = false;
                roomInput.disabled = false;
                ws = null;
            };

            ws.onerror = function(err) {
                console.error('WebSocket error:', err);
            };
        }

        function disconnect() {
            if (ws) {
                ws.close();
                ws = null;
            }
        }

        function sendMessage() {
            const text = messageInput.value.trim();
            if (!text || !ws) return;
            const msg = {
                type: 'chat',
                text: text
            };
            ws.send(JSON.stringify(msg));
            messageInput.value = '';
        }

        connectBtn.addEventListener('click', connect);
        disconnectBtn.addEventListener('click', disconnect);
        sendBtn.addEventListener('click', sendMessage);
        messageInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') sendMessage();
        });

        // Автоматическое подключение при загрузке
        // window.onload = connect;
    </script>
</body>
</html>
`

// IndexHandler — отдаёт HTML-страницу
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

//MAIN

func main() {
	flag.Parse()

	// Создаём Hub и запускаем его в отдельной горутине
	hub := NewHub()
	go hub.Run()

	// Настраиваем HTTP-роутер
	http.HandleFunc("/", IndexHandler)
	http.HandleFunc("/ws", WebSocketHandler(hub))

	// Создаём HTTP-сервер
	srv := &http.Server{
		Addr:         *addr,
		Handler:      nil, // используем default ServeMux
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Запускаем HTTP-сервер в отдельной горутине
	go func() {
		log.Printf("🚀 HTTP сервер запущен на %s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP сервер ошибка: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Получен сигнал завершения, начинаем graceful shutdown...")

	// Контекст для graceful shutdown (30 секунд)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Закрываем HTTP-сервер
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Ошибка при завершении HTTP-сервера: %v", err)
	}

	log.Println("Сервер остановлен")
}
