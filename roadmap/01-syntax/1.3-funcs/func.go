package funcs

import (
	"crypto/tls"
	"fmt"
	"time"
)

// ЗАДАЧА 1. MAP/FILTER/REDUCE ЧЕРЕЗ ДЖЕНЕРИКИ
// Плохо: для каждого типа своя функция
func mapInts(s []int, f func(int) int) []int {
	result := make([]int, len(s))
	for i, v := range s {
		result[i] = f(v)
	}
	return result
}

func mapStrings(s []string, f func(string) string) []string {
	result := make([]string, len(s))
	for i, v := range s {
		result[i] = f(v)
	}
	return result
}

// С дженериками (один раз для всех типов)
func mapGen[T any, R any](sl []T, f func(T) R) []R {
	result := make([]R, len(sl))
	for i, j := range sl {
		result[i] = f(j)
	}
	return result
}

// Filter оставляет только элементы, для которых f возвращает true
func filterGen[T any](slice []T, f func(T) bool) []T {
	var result []T
	for _, v := range slice {
		if f(v) {
			result = append(result, v)
		}
	}
	return result
}

// Reduce сворачивает слайс в одно значение
func reduce[T any, R any](sl []T, inital R, f func(R, T) R) R {
	result := inital

	for _, v := range sl {
		result = f(result, v)
	}
	return result
}

// ЗАДАЧА 2. КОНСТРУКТОР С FUNCTIONAL OPTIONS ДЛЯ SERVER
// ServerConfig структура конфигурации
type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	TLSConfig    *tls.Config
	MaxConn      int
}

// Server наша структура
type Server struct {
	config ServerConfig
	// другие поля (listener, logger...)
}

// Option функциональный тип-опция
type Option func(*ServerConfig)

// Конструктор с вариативными опциями
func NewServer(opts ...Option) *Server {
	// Стандартные значения (defaults)
	config := ServerConfig{
		Port:         8080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		MaxConn:      100,
	}

	// Применяем опции
	for _, opt := range opts {
		opt(&config)
	}

	return &Server{
		config: config,
	}
}

// Конкретные опции
func WithPort(port int) Option {
	return func(c *ServerConfig) {
		if port > 0 && port < 65536 {
			c.Port = port
		}
	}
}

func WithTimeouts(read, write time.Duration) Option {
	return func(c *ServerConfig) {
		if read > 0 {
			c.ReadTimeout = read
		}
		if write > 0 {
			c.WriteTimeout = write
		}
	}
}

func WithTLS(tlsConfig *tls.Config) Option {
	return func(c *ServerConfig) {
		c.TLSConfig = tlsConfig
	}
}

func WithMaxConn(maxConn int) Option {
	return func(c *ServerConfig) {
		if maxConn > 0 {
			c.MaxConn = maxConn
		}
	}
}

// 3.3. ПРОДВИНУТЫЙ МЕМОИЗАТОР (с TTL)
type CacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

func memoizeWithTTL[K comparable, V any](f func(K) V, ttl time.Duration) func(K) V {
	cache := make(map[K]CacheEntry[V])

	return func(key K) V {
		if entry, ok := cache[key]; ok {
			if time.Now().Before(entry.expiresAt) {
				fmt.Printf("Cache hit for %v\n", key)
				return entry.value
			}
			fmt.Printf("Cache expired for %v\n", key)
		}
		fmt.Printf("Computing for %v\n", key)
		result := f(key)
		cache[key] = CacheEntry[V]{
			value:     result,
			expiresAt: time.Now().Add(ttl),
		}
		return result
	}
}
