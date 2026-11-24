package main

import (
	"hash/fnv"
	"sync"
)

/*
Демонстрация проблемы (Data Race)
Покажем, почему обычная мапа не работает. (Это не решение, а постановка задачи).
*/
/*
func main() {
	m := make(map[int]int)
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			m[val] = val * 2
		}(i)
	}
	wg.Wait()
	fmt.Println(m)
}
*/

/*
Классическое решение (Map + sync.RWMutex)
Мы оборачиваем мапу в структуру и защищаем её мьютексом.
Почему RWMutex, а не Mutex?
RWMutex позволяет множеству горутин читать одновременно (RLock), но блокирует всё
при записи (Lock). Для кэшей и справочников это дает огромный прирост производительности.
*/
/*
type ConcurrentMap struct {
	mu sync.RWMutex
	m  map[string]int
}

func NewConcurrentMap() *ConcurrentMap {
	return &ConcurrentMap{
		m: make(map[string]int),
	}
}

func (c *ConcurrentMap) Set(key string, value int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = value
}

func (c *ConcurrentMap) Get(key string) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.m[key]
	return val, ok
}

func (c *ConcurrentMap) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
}

func main() {
	cm := NewConcurrentMap()
	var wg sync.WaitGroup

	for i := 0; i < 100000; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			cm.Set(fmt.Sprintf("key_%d", val), val)
		}(i)
	}

	wg.Wait()
	fmt.Println("Все данные записаны без паники!")
}
*/

/*
Встроенный sync.Map (Когда он реально нужен)
В Go 1.9 добавили sync.Map. Многие думают, что это "серебряная пуля" и используют везде. Это ошибка.
sync.Map оптимизирован для двух специфических случаев:
Когда ключи пишутся один раз, а читаются много раз (read-heavy).
Когда разные горутины работают с разными (непересекающимися) наборами ключей.
*/
/*
func main() {
	var sm sync.Map

	sm.Store("user_1", "Ivan")
	sm.Store("user_2", "Petr")

	if val, ok := sm.Load("user_1"); ok {
		fmt.Println(val)
	}

	actual, loaded := sm.LoadOrStore("user_1", "New name")
	fmt.Printf("Значение: %v, Было ли уже загружено: %v\n", actual, loaded)

	sm.Delete(("user_2"))

	sm.Range(func(key, value interface{}) bool {
		fmt.Println(key)
		return true
	})
}
*/

/*
Шардированная мапа (Sharded Map) — Уровень Middle+
Если у тебя 10 000 горутин, и они все пишут в map + RWMutex, они будут постоянно
блокироваться на одном мьютексе. Это называется lock contention (конкуренция за блокировку).
Разбить мапу на N частей (шардов). Каждая часть имеет свой мьютекс. Ключ хешируется, и мы попадаем
в нужный шард. Это позволяет горутинам писать параллельно, если их ключи попали в разные шарды.
*/
const shardCount = 32

type shard struct {
	mu sync.RWMutex
	m  map[string]int
}

type ShardedMap struct {
	shards [shardCount]shard
}

func NewShardedMap() *ShardedMap {
	sm := &ShardedMap{}
	for i := 0; i < shardCount; i++ {
		sm.shards[i].m = make(map[string]int)
	}
	return sm
}

// Хеш-функция для выбора шарда
func getShardIndex(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32()) % shardCount
}

func (sm *ShardedMap) Set(key string, value int) {
	idx := getShardIndex(key)
	sm.shards[idx].mu.Lock()
	defer sm.shards[idx].mu.Unlock()
	sm.shards[idx].m[key] = value
}

func (sm *ShardedMap) Get(key string) (int, bool) {
	idx := getShardIndex(key)
	sm.shards[idx].mu.RLock()
	defer sm.shards[idx].mu.RUnlock()
	val, ok := sm.shards[idx].m[key]
	return val, ok
}
