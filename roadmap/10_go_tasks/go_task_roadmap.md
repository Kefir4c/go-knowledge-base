# GO-ЗАДАЧИ

## TIER 1: CONCURRENCY

- [x] **Worker Pool** — N воркеров, M задач, результаты в канал (с поддержкой `context` для отмены)
- [x] **Fan-in** — merge N каналов в один
- [x] **Fan-out** — один канал в N обработчиков
- [x] **Pipeline** — 3+ стадии через каналы
- [x] **Graceful Shutdown** — полный цикл: `SIGTERM` -> `context` -> `srv.Shutdown` -> `wg.Wait` -> закрытие БД
- [x] **Rate Limiter** — token bucket через `time.Ticker` (потом взглянуть `golang.org/x/time/rate`)
- [x] **Semaphore** — ограничение параллелизма через буферизованный канал
- [x] **errgroup** — `golang.org/x/sync/errgroup` (WaitGroup с обработкой ошибок и отменой)
- [x] **Context Cancellation** — отмена долгой операции, если клиент ушёл (не путать с Timeout)
- [x] **Timeout wrapper** — `context.WithTimeout` вокруг функции (per-task таймаут в worker pool)
- [x] **Retry с backoff** — экспоненциальная задержка + `context`

## TIER 2: ПАТТЕРНЫ И СТРУКТУРЫ

- [ ] **LRU Cache** — `container/list` + map + `sync.Mutex`.
- [ ] **Concurrent Map** — map + `sync.RWMutex` (или `sync.Map`).
- [ ] **In-memory KV с TTL** — map + `time.AfterFunc` или горутина-чистильщик.
- [ ] **HTTP Middleware** — logging, recovery (panic), auth (обертки над `http.Handler`).
- [ ] **HTTP Сервис** — REST API + graceful shutdown + `ReadTimeout`/`WriteTimeout`.
- [ ] **Atomic counter** — `sync/atomic`.
- [ ] **Generics** — базово: обобщённый `Filter`/`Map`, `comparable`.

## TIER 3: GOTCHAS И ЛОВУШКИ (ЗНАТЬ НАИЗУСТЬ)

### Каналы
- [ ] **Закрытый канал** — паника при записи, нулевое значение при чтении. Кто должен закрывать?
- [ ] **Nil-канал** — блокировка навсегда при чтении/записи.
- [ ] **select + default** — неблокирующее чтение.
- [ ] **select при отмене** — продюсер пишет в канал через `select { case <-ctx.Done() ... }` против дедлока.

### Память и Интерфейсы
- [ ] **Interface nil check** — почему `err != nil` может быть true, даже если внутри nil (тип + значение).
- [ ] **Slice underlying array** — почему `append` может изменить оригинальный массив (`a[1:3]`).
- [ ] **Map concurrent access** — почему нельзя писать в map из разных горутин (panic).
- [ ] **Копирование мьютекса** — передача структуры со `sync.Mutex` по значению.

### Язык
- [ ] **panic / recover** — когда использовать (и почему только в `defer`).
- [ ] **Loop variable capture** — захват переменной цикла в замыкании (до Go 1.22 и как сейчас).
- [ ] **defer в цикле** — накапливаются отложенные вызовы.

## TIER 4: ТЕОРИЯ (ПРОЧИТАТЬ)
- [ ] **Memory model / happens-before** — что гарантируют каналы, мьютексы, atomic.
- [ ] **Goroutine leak** — типовые причины утечек и как их найти (`goleak`, `runtime.NumGoroutine`).
- [ ] **Ошибки: Wrapping + errors.Is/As** — как проверять обёрнутые ошибки.
- [ ] **sync.Pool** — концепция и когда имеет смысл.
---
## ТОП-10 GO-ЗАДАЧ
1. **Worker Pool** — почти всегда
2. **Fan-in / Fan-out** — очень часто
3. **Graceful Shutdown** — часто
4. **Rate Limiter** — часто
5. **LRU Cache** — часто
6. **Pipeline** — часто
7. **Retry с backoff** — иногда
8. **Concurrent Map** — иногда
9. **In-memory KV с TTL** — иногда
10. **HTTP middleware** — иногда

---
