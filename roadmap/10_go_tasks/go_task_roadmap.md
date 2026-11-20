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
- [ ] **Context Cancellation** — отмена долгой операции, если клиент ушёл (не путать с Timeout)
- [ ] **Timeout wrapper** — `context.WithTimeout` вокруг функции (per-task таймаут в worker pool)
- [ ] **Retry с backoff** — экспоненциальная задержка + `context`
- [ ] **Memory model / happens-before** — что гарантируют каналы, мьютексы, atomic; как горутины видят изменения друг друга
- [ ] **Goroutine leak** — типовые причины утечек (запись в канал без читателя, забытый cancel) и как их найти (`goleak`, pprof)

## TIER 2: ПАТТЕРНЫ И СТРУКТУРЫ

- [ ] **Concurrent Map** — map + `sync.RWMutex` (или `sync.Map`, если знаешь когда его использовать)
- [ ] **Atomic counter** — `sync/atomic`
- [ ] **LRU Cache** — `container/list` + map
- [ ] **In-memory KV с TTL** — map + `time.AfterFunc` или горутина-чистильщик
- [ ] **Singleton** — `sync.Once`
- [ ] **Option pattern** — функциональные опции (как в gRPC)
- [ ] **Generics** — базово: обобщённый `Filter`/`Map`/`Stack`, `comparable`, type constraints
- [ ] **sync.Pool** — концепция и когда имеет смысл (не реализация, достаточно статьи)

## TIER 3: GOTCHAS И ЛОВУШКИ (НУЖНО ЗНАТЬ НАИЗУСТЬ)

- [ ] **Закрытый канал** — паника при записи, нулевое значение при чтении. Кто должен закрывать?
- [ ] **Nil-канал** — блокировка навсегда при чтении/записи
- [ ] **select + default** — неблокирующее чтение
- [ ] **select при отмене** — продюсер пишет в канал через `select { case <-ctx.Done() ... }` против дедлока при fail-fast
- [ ] **Interface nil check** — почему `err != nil` может быть true, даже если внутри nil (тип + значение)
- [ ] **Slice underlying array** — почему `append` может изменить оригинальный массив (`a[1:3]`)
- [ ] **Map concurrent access** — почему нельзя писать в map из разных горутин (panic)
- [ ] **panic / recover** — когда использовать (и почему только в `defer`)
- [ ] **Loop variable capture** — захват переменной цикла в замыкании (до Go 1.22 и как сейчас)
- [ ] **defer в цикле** — накапливаются отложенные вызовы
- [ ] **Копирование мьютекса** — передача структуры со `sync.Mutex` по значению

## TIER 4: ОШИБКИ, HTTP И СТРОКИ

### Ошибки
- [ ] **Custom error** — структура с полями
- [ ] **Wrapping** — `fmt.Errorf` с `%w`
- [ ] **errors.Is / errors.As** — как проверять обёрнутые ошибки
- [ ] **Sentinel error** — `var ErrNotFound = errors.New(...)`

### HTTP / Context
- [ ] **HTTP сервер** — простой REST API
- [ ] **Middleware chain** — logging, recovery, auth (обёртки над `http.Handler`)
- [ ] **Graceful shutdown HTTP** — `srv.Shutdown(ctx)`
- [ ] **HTTP Server Timeouts** — ReadTimeout / WriteTimeout / IdleTimeout / ReadHeaderTimeout
- [ ] **HTTP Client с Timeout** — `http.Client{Timeout: ...}` (частая ошибка — забывают)

### Строки
- [ ] **Reverse words** — "hello world" -> "world hello"
- [ ] **Compress string** — "aaabbc" -> "a3b2c1"
- [ ] **strings.Builder** — понимать, почему `+` в цикле — это O(n^2)

## TIER 5: ТЕСТИРОВАНИЕ И ИНСТРУМЕНТЫ

- [ ] **Table-driven tests** — паттерн, стандарт на собесах
- [ ] **httptest** — тестирование HTTP-хендлеров без живого сервера
- [ ] **go test -race** — прогон конкурентного кода под race-детектором
- [ ] **Бенчмарки** — `BenchmarkXxx` и `b.ReportAllocs`
- [ ] **pprof** — где смотреть CPU/горутины, базовое чтение профиля

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
