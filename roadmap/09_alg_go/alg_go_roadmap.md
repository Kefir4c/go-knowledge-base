# ROADMAP АЛГОРИТМОВ И GO ЗАДАЧ (30 ДНЕЙ)

## GO-ЗАДАЧИ

##TIER 1: CONCURRENCY

- [ ] **Worker Pool** — N воркеров, M задач, результаты в канал (с поддержкой `context` для отмены)
- [ ] **Fan-in** — merge N каналов в один
- [ ] **Fan-out** — один канал в N обработчиков
- [ ] **Pipeline** — 3+ стадии через каналы
- [ ] **Graceful Shutdown** — полный цикл: `SIGTERM` -> `context` -> `srv.Shutdown` -> `wg.Wait` -> закрытие БД
- [ ] **Rate Limiter** — token bucket через `time.Ticker`
- [ ] **Semaphore** — ограничение параллелизма через буферизованный канал
- [ ] **Merge N каналов** — dynamic merge с `select`
- [ ] **Context Cancellation** — отмена долгой операции, если клиент ушёл (не путать с Timeout)
- [ ] **errgroup** — `golang.org/x/sync/errgroup` (WaitGroup с обработкой ошибок и отменой)
- [ ] **Retry с backoff** — экспоненциальная задержка + `context`
- [ ] **Timeout wrapper** — `context.WithTimeout` вокруг функции

##TIER 2: ПАТТЕРНЫ И СТРУКТУРЫ

- [ ] **Concurrent Map** — map + `sync.RWMutex` (или `sync.Map`, если знаешь когда его использовать)
- [ ] **Atomic counter** — `sync/atomic`
- [ ] **LRU Cache** — `container/list` + map
- [ ] **In-memory KV с TTL** — map + `time.AfterFunc` или горутина-чистильщик
- [ ] **Singleton** — `sync.Once`
- [ ] **Option pattern** — функциональные опции (как в gRPC)
- [ ] **sync.Pool** — переиспользование объектов, когда имеет смысл

##TIER 3: GOTCHAS И ЛОВУШКИ (НУЖНО ЗНАТЬ НАИЗУСТЬ)

- [ ] **Закрытый канал** — паника при записи, нулевое значение при чтении. Кто должен закрывать?
- [ ] **Nil-канал** — блокировка навсегда при чтении/записи
- [ ] **select + default** — неблокирующее чтение
- [ ] **Interface nil check** — почему `err != nil` может быть true, даже если внутри nil (тип + значение)
- [ ] **Slice underlying array** — почему `append` может изменить оригинальный массив (`a[1:3]`)
- [ ] **Map concurrent access** — почему нельзя писать в map из разных горутин (panic)
- [ ] **panic / recover** — когда использовать (и почему только в `defer`)

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
- [ ] **HTTP Client с Timeout** — `http.Client{Timeout: ...}` (частая ошибка — забывают)

### Строки
- [ ] **Reverse words** — "hello world" -> "world hello"
- [ ] **Compress string** — "aaabbc" -> "a3b2c1"
- [ ] **strings.Builder** — понимать, почему `+` в цикле — это O(n^2)

---
## ТОП-10 GO-ЗАДАЧ
1. **Worker Pool** — почти 
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

## ОБЯЗАТЕЛЬНЫЕ ТЕМЫ (13)

1. Big O + Go-специфика
2. Arrays + Hash map
3. Two pointers
4. Sliding window + Strings
5. Binary search
6. Stack
7. Linked lists
8. Trees + BST
9. Recursion + Backtracking
10. Graphs (BFS/DFS)
11. Topological sort + Intervals
12. Heap
13. DP (1D + база 2D) + Go-concurrency

---


## РАСПИСАНИЕ ПО ДНЯМ

### День 1: Big O
- Теория: O(1), O(log n), O(n), O(n log n), O(n²), O(2^n), amortized, space.
- Go: slice, map, sort, container/heap.
- Практика: оценить сложность 20 своих решений.

### День 2–3: Arrays + Hash map
- Теория: slice, map, prefix sum.
- Задачи: Two Sum, Contains Duplicate, Valid Anagram, Group Anagrams, Top K Frequent Elements, Product of Array Except Self.

### День 4: Two pointers
- Теория: сходящиеся указатели, fast/slow.
- Задачи: Valid Palindrome, Two Sum II, 3Sum.

### День 5: Sliding window + Strings
- Теория: фиксированное/переменное окно, rune vs byte, strings.Builder.
- Задачи: Longest Substring Without Repeating, Longest Repeating Character Replacement, Minimum Size Subarray Sum.

### День 6: Binary search
- Теория: классический, граница, по ответу.
- Задачи: Binary Search, Search in Rotated Sorted Array, Koko Eating Bananas, Search a 2D Matrix.

### День 8: Stack
- Теория: LIFO, monotonic stack.
- Задачи: Valid Parentheses, Min Stack, Daily Temperatures.

### День 9–10: Linked lists
- Теория: dummy node, Floyd's cycle detection.
- Задачи: Reverse Linked List, Merge Two Sorted Lists, Linked List Cycle, Remove Nth Node From End.

### День 11–12: Trees
- Теория: DFS (preorder/inorder/postorder), BFS (level order).
- Задачи: Maximum Depth, Invert Binary Tree, Same Tree, Level Order Traversal, Diameter of Binary Tree.

### День 13: BST
- Теория: свойства BST, валидация, поиск, вставка, удаление.
- Задачи: Validate BST, Lowest Common Ancestor of BST, Kth Smallest Element in BST.

### День 15–16: Recursion + Backtracking
- Теория: рекурсия, дерево вызовов, backtracking.
- Задачи: Subsets, Permutations, Combination Sum.

### День 17–18: Graphs (BFS/DFS на матрице)
- Теория: представление графа, BFS, DFS, visited, flood fill.
- Задачи: Number of Islands, Clone Graph, Rotting Oranges, Pacific Atlantic Water Flow.

### День 20: Intervals
- Теория: сортировка интервалов, пересечение, вставка.
- Задачи: Merge Intervals, Insert Interval, Non-overlapping Intervals.

### День 22: Heap
- Теория: min-heap, max-heap, container/heap в Go.
- Задачи: Kth Largest Element, Merge k Sorted Lists.

### День 23–24: DP 1D
- Теория: memoization, tabulation, состояния, переходы.
- Задачи: Climbing Stairs, House Robber, Coin Change, Longest Increasing Subsequence, *Unique Paths

### День 26–27: Go-concurrency
- Теория: горутины, каналы, select, sync, context, errgroup.
- Задачи: Worker Pool, Fan-in/Fan-out, Rate Limiter, Graceful Shutdown, Merge каналов, Pipeline.

### День 28: Greedy + Matrix
- Теория: локально оптимальный выбор, работа с матрицей in-place.
- Задачи: Maximum Subarray, Jump Game, Rotate Image.

### День 29: Strings финально
- Задачи: Longest Palindromic Substring, Encode/Decode Strings, Group Anagrams (повтор).