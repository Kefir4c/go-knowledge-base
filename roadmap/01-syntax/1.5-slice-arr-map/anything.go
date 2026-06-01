package main

import (
	"container/list"
	"fmt"
	"runtime"
)

// «УТЕЧКА» BACKING-ARRAY ЧЕРЕЗ SUB-SLICE
// ❌ ПЛОХО: утечка памяти
func getFirst(s []string) []string {
	// Возвращаем под-слайс, но ссылаемся на ВЕСЬ огромный массив
	return s[:1] // утечка! весь s остаётся в памяти
}

// ✅ ХОРОШО: копируем нужные данные
func getFirstGood(s []string) []string {
	result := make([]string, 1)
	copy(result, s[:1])
	return result
}
func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Alloc = %v MB\n", m.Alloc/1024/1024)
}
func primer1() {
	// Создаём огромный слайс
	hugeSlice := make([]string, 1000000)
	for i := 0; i < len(hugeSlice); i++ {
		hugeSlice[i] = fmt.Sprintf("item_%d", i)
	}

	// ❌ Утечка: smallSlice ссылается на backing-array hugeSlice
	smallSlice := hugeSlice[:10]

	hugeSlice = nil // очищаем ссылку, но память НЕ освободится!

	// Память всё ещё занята, потому что smallSlice ссылается на тот же массив
	fmt.Println("first element:", smallSlice[0])

	// ✅ Правильно: копируем
	goodSlice := getFirstGood(make([]string, 1000000))
	fmt.Println("good slice:", goodSlice)

	// Демонстрация проблемы
	printMemUsage()
}

//LRU-КЭШ НА MAP + CONTAINER/LIST

type LRUCache struct {
	capacity int
	cache    map[int]*list.Element // ключ -> элемент в списке
	lruList  *list.List            // список: голова = старые, хвост = новые
}

// entry хранит ключ и значение
type entry struct {
	key   int
	value int
}

// New создаёт новый кэш
func New(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[int]*list.Element),
		lruList:  list.New(),
	}
}

// Get достаёт значение из кэша
func (c *LRUCache) Get(key int) (int, bool) {
	elem, ok := c.cache[key]
	if !ok {
		return 0, false
	}
	c.lruList.MoveToBack(elem)
	return elem.Value.(*entry).value, true
}

func (c *LRUCache) Put(key, value int) {
	// Если ключ уже есть — обновляем
	if elem, ok := c.cache[key]; ok {
		elem.Value.(*entry).value = value
		c.lruList.MoveToBack(elem)
		return
	}

	// Если переполнен — удаляем самый старый (из головы)
	if c.lruList.Len() >= c.capacity {
		oldest := c.lruList.Front()
		delete(c.cache, oldest.Value.(*entry).key)
		c.lruList.Remove(oldest)
	}

	// Добавляем новый
	c.cache[key] = c.lruList.PushBack(&entry{key: key, value: value})
}

func main() {
	cache := New(3)

	cache.Put(1, 10)
	cache.Put(2, 20)
	cache.Put(3, 30)
	fmt.Println("Самый старый -> самый новый:", getKeys(cache)) // [1 2 3]

	cache.Get(1)                                 // используем 1 → она становится новой
	fmt.Println("После Get(1):", getKeys(cache)) // [2 3 1]

	cache.Put(4, 40)                             // добавляем 4 → вытесняется 2 (самый старый)
	fmt.Println("После Put(4):", getKeys(cache)) // [3 1 4]
}

// Вспомогательная функция для вывода ключей в порядке от старых к новым
func getKeys(c *LRUCache) []int {
	keys := make([]int, 0, c.lruList.Len())
	for e := c.lruList.Front(); e != nil; e = e.Next() {
		keys = append(keys, e.Value.(*entry).key)
	}
	return keys
}
