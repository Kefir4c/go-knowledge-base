package main

import (
	"context"
	"sync"
	"time"
)

/*
Ленивое удаление (Lazy Deletion)
Самый простой способ. Мы не запускаем никаких таймеров. Мы храним время
истечения прямо в структуре значения. А когда клиент вызывает Get, мы
проверяем: «А не просрочился ли ключ?». Если да — удаляем и возвращаем пустоту.
Плюсы: Тратит 0% CPU в фоне, идеально для редких ключей.
Минусы: Если ключ записали и больше никогда не читали,
он будет вечно висеть в памяти (утечка).
*/
/*
type Item struct {
	value     interface{}
	ExpiresAt time.Time
}

type LazyCache struct {
	mu sync.RWMutex
	m  map[string]Item
}

func NewLazyCache() *LazyCache {
	return &LazyCache{m: make(map[string]Item)}
}

func (c *LazyCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = Item{
		value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (c *LazyCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, ok := c.m[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(item.ExpiresAt) {
		c.mu.Lock()
		delete(c.m, key)
		c.mu.Unlock()
		return nil, false
	}
	return item.value, true
}
*/

/*
Активное удаление через time.AfterFunc
time.AfterFunc — это встроенная функция Go, которая запускает колбэк через заданное время в отдельной горутине.
Когда мы делаем Set, мы говорим: "Через ttl секунд выполни удаление этого ключа".
Плюсы: Ключ удаляется точно в срок, память не утекает.
Минусы*: time.AfterFunc создает отдельную горутину на каждый таймер. Если у тебя 1 000 000 ключей,
у тебя будет 1 000 000 таймеров и горутин. Это убьет память и CPU. Этот способ годится только для небольшого количества ключей.
*/
/*
type AfterFuncCache struct {
	mu     sync.RWMutex
	m      map[string]interface{}
	timers map[string]*time.Timer
}

func NewAfterFuncCache() *AfterFuncCache {
	return &AfterFuncCache{
		m:      make(map[string]interface{}),
		timers: make(map[string]*time.Timer),
	}
}

func (c *AfterFuncCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if oldTimer, ok := c.timers[key]; ok {
		oldTimer.Stop()
	}
	c.m[key] = value

	c.timers[key] = time.AfterFunc(ttl, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.timers, key)
		delete(c.m, key)
	})
}
*/

/*
Фоновая горутина-чистильщик (Background Cleaner)
Вместо того чтобы создавать миллион таймеров, мы создаем один таймер (time.Ticker),
который раз в N секунд просыпается и проходит по всей мапе, удаляя просроченные ключи.
Плюсы: Всего одна горутина на весь сервис, не утекает память.
Минусы: Просроченный ключ может лежать в мапе до N секунд (пока чистильщик не проснется).
Если N = 1 минута, а TTL = 1 секунда, клиент может прочитать просроченный ключ.
*/
/*
type Item struct {
	Value     interface{}
	ExpiresAt time.Time
}

type ProdCache struct {
	mu sync.RWMutex
	m  map[string]Item
}

func NewProdCache(ctx context.Context) *ProdCache {
	c := &ProdCache{m: make(map[string]Item)}

	go c.cleanupLoop(ctx)
	return c
}

func (c *ProdCache) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for k, i := range c.m {
				if now.After(i.ExpiresAt) {
					delete(c.m, k)
				}
			}
			c.mu.Unlock()
		}
	}
}

func (c *ProdCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = Item{Value: value, ExpiresAt: time.Now().Add(ttl)}
}

func (c *ProdCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, ok := c.m[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(item.ExpiresAt) {
		c.mu.Lock()
		delete(c.m, key)
		c.mu.Unlock()
		return nil, false
	}
	return item.Value, true
}
*/

/*
Production-Ready (Комбинация Lazy + Cleaner)
Как делают в настоящем проде (и в Redis)? Комбинируют оба подхода!
При Get делаем ленивую проверку (чтобы никогда не отдать просроченный ключ).
В фоне работает чистильщик (чтобы удалять мусор, который никто не читает).
*/

type Item struct {
	Value     interface{}
	ExpiresAt time.Time
}

type ProdCache struct {
	mu sync.RWMutex
	m  map[string]Item
}

func NewProdCache(ctx context.Context) *ProdCache {
	c := &ProdCache{m: make(map[string]Item)}

	go c.cleanupLoop(ctx)
	return c
}

func (c *ProdCache) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for k, v := range c.m {
				if now.After(v.ExpiresAt) {
					delete(c.m, k)
				}
			}
			c.mu.Unlock()
		}
	}
}

func (c *ProdCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = Item{Value: value, ExpiresAt: time.Now().Add(ttl)}
}

func (c *ProdCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	item, ok := c.m[key]
	c.mu.Unlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(item.ExpiresAt) {
		c.mu.Lock()
		delete(c.m, key)
		c.mu.Unlock()
		return nil, false
	}
	return item.Value, true
}
