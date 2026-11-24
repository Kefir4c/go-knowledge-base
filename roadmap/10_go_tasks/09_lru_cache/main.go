package main

import (
	"container/list"
	"sync"
	"time"
)

/*
Базовый LRU (без мьютексов, для понимания)
Сначала разберем, как работает container/list. Это двусвязный список из стандартной
библиотеки. Каждый элемент (*list.Element) хранит значение и ссылки на соседей.
Мы будем использовать map для поиска элемента за O(1) и list для хранения порядка (спереди — свежие, сзади — старые).
*/
/*
type LRUCache struct {
	capacity int
	items    map[int]*list.Element
	ll       *list.List
}

type entry struct {
	key   int
	value int
}

func constructor(cap int) *LRUCache {
	return &LRUCache{
		capacity: cap,
		items:    make(map[int]*list.Element),
		ll:       list.New(),
	}
}

func (c *LRUCache) Get(key int) int {
	if elem, ok := c.items[key]; ok {
		c.ll.MoveToFront(elem)
		return elem.Value.(*entry).value
	}
	return -1
}

func (c *LRUCache) Put(key int, value int) {
	if elem, ok := c.items[key]; ok {
		c.ll.MoveToFront(elem)
		elem.Value.(*entry).value = value
		return
	}
	if c.ll.Len() >= c.capacity {
		back := c.ll.Back()
		if back != nil {
			c.ll.Remove(back)
			delete(c.items, back.Value.(*entry).key)
		}
	}
	newElem := c.ll.PushFront(&entry{key, value})
	c.items[key] = newElem
}
*/

/*
Concurrency-Safe LRU (Для собеседования на Мидла)
В Go нельзя писать в map из разных горутин (будет panic: concurrent map writes). А http.Server в
Go обрабатывает каждый запрос в отдельной горутине. Значит, наш LRU должен быть потокобезопасным.
*/
/*
type LRUCache struct {
	mu       sync.Mutex
	capacity int
	items    map[int]*list.Element
	ll       *list.List
}

type entry struct {
	key int
	val int
}

func Constructor(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		items:    make(map[int]*list.Element),
		ll:       list.New(),
	}
}

func (c *LRUCache) Get(key int) int {
	c.mu.Lock()
	defer c.mu.Lock()

	if elem, ok := c.items[key]; ok {
		c.ll.MoveToFront(elem)
		return elem.Value.(*entry).val
	}
	return -1
}

func (c *LRUCache) Put(key, value int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.ll.MoveToFront(elem)
		elem.Value.(*entry).val = value
		return
	}

	if c.ll.Len() >= c.capacity {
		back := c.ll.Back()
		if back != nil {
			c.ll.Remove(back)
			delete(c.items, back.Value.(*entry).key)
		}
	}
	newElem := c.ll.PushFront(&entry{key, value})
	c.items[key] = newElem
}
*/

/*
Generic LRU (Для продвинутых)
Начиная с Go 1.18, в языке появились дженерики (Generics). В реальном проекте ты не захочешь
писать отдельный LRU для string->int, int->string и struct->struct. Ты напишешь один обобщенный.
*/
/*
type LRUCache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	items    map[K]*list.Element
	ll       *list.List
}

type entry[K comparable, V any] struct {
	key   K
	value V
}

func Constructor[K comparable, V any](capacity int) *LRUCache[K, V] {
	return &LRUCache[K, V]{
		capacity: capacity,
		items:    make(map[K]*list.Element),
		ll:       list.New(),
	}
}

func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.ll.MoveToFront(elem)
		return elem.Value.(*entry[K, V]).value, true
	}
	var zero V
	return zero, false
}

func (c *LRUCache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.ll.MoveToFront(elem)
		elem.Value.(*entry[K, V]).value = value
		return
	}

	if c.ll.Len() >= c.capacity {
		back := c.ll.Back()
		if back != nil {
			c.ll.Remove(back)
			delete(c.items, back.Value.(*entry[K, V]).key)
		}
	}

	newElem := c.ll.PushFront(&entry[K, V]{key: key, value: value})
	c.items[key] = newElem
}
*/
/*
LRU с TTL (Time To Live) — Production-Ready
Как добавить TTL (время жизни) для ключей?. Обычный LRU не удаляет элементы по времени,
только по нехватке места. Чтобы добавить TTL, нужно хранить время истечения и периодически чистить кэш.
*/
type entryWithTTL struct {
	key       string
	value     interface{}
	expiresAt time.Time
}

type TTL_LRUCache struct {
	mu       sync.Mutex
	capacity int
	ttl      time.Duration
	items    map[string]*list.Element
	ll       *list.List
}

func Constructor(capacity int, ttl time.Duration) *TTL_LRUCache {
	c := &TTL_LRUCache{
		capacity: capacity,
		ttl:      ttl,
		items:    make(map[string]*list.Element),
		ll:       list.New(),
	}
	go c.cleanupLoop()
	return c
}

func (c *TTL_LRUCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.Value.(*entryWithTTL).expiresAt) {
				c.ll.Remove(v)
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}

func (c *TTL_LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		e := elem.Value.(*entryWithTTL)

		if time.Now().After(e.expiresAt) {
			c.ll.Remove(elem)
			delete(c.items, key)
			return nil, false
		}
		c.ll.MoveToFront(elem)
		return e.value, true
	}
	return nil, false
}

func (c *TTL_LRUCache) Put(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiresAt := time.Now().Add(c.ttl)

	if elem, ok := c.items[key]; ok {
		e := elem.Value.(*entryWithTTL)
		e.value = value
		e.expiresAt = expiresAt
		c.ll.MoveToFront(elem)
		return
	}

	if c.ll.Len() >= c.capacity {
		back := c.ll.Back()
		if vack != nil {
			c.ll.Remove(back)
			delete(c.items, back.Value.(*entryWithTTL).key)
		}
	}

	newEntry := &entryWithTTL{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}
	newElem := c.ll.PushFront(newEntry)
	c.items[key] = newElem
}
