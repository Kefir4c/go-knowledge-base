package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

/*
 ТЕОРИЯ

1. Паттерны поведения при падении мастера

Graceful Degradation | Сервис продолжает работать, но с ограниченным функционалом (например, только чтение) | Когда чтение критично, запись можно отложить |
Circuit Breaker | Отключает запросы к мастеру после N ошибок, чтобы не нагружать его | Для защиты от каскадных отказов |
Retry with Backoff | Повторные попытки с экспоненциальной задержкой | Временные сбои (failover занимает 10-30 секунд) |
Read-Only Mode | Все запросы направляются только на реплики | Когда мастер недоступен более 1 минуты |
Failover Aware | Сервис знает о состоянии кластера и переключается автоматически | При использовании Patroni или etcd |

2. Коды ошибок PostgreSQL (важно!)

| Код | Сообщение | Что делать |
|-----|-----------|------------|
| `57P01` | admin_shutdown | Мастер перезагружается — повторить |
| `57P02` | crash_shutdown | Мастер упал — переключиться на реплику |
| `53300` | too many connections | Подождать и повторить |
| `08006` | connection failure | Соединение потеряно — переподключиться |
| `40001` | serialization failure | Повторить транзакцию |

3. Лучшие практики

- Всегда использовать **контексты с таймаутами**.
- Использовать **пулы соединений** (pgxpool) для мастера и реплик.
- Настраивать **health check** (ping) каждые 5-10 секунд.
- Логировать **все ошибки** с уровнем WARN/ERROR.
- Использовать **структурированные логи** (slog) для трейсинга.

 ПРАКТИКА (ГОТОВЫЙ КОД ДЛЯ ПРОДАКШЕНА)
*/

// 1. КОНФИГУРАЦИЯ

type DBConfig struct {
	MasterDSN                 string
	ReplicaDSNs               []string
	MaxRetries                int
	RetryDelay                time.Duration
	HealthCheckPeriod         time.Duration
	CircuitBreakerMaxFailures int
	CircuitBreakerTimeout     time.Duration
}

func LoadDBConfig() DBConfig {
	return DBConfig{
		MasterDSN:                 getEnv("MASTER_DSN", "postgres://user:pass@localhost:5432/mydb"),
		ReplicaDSNs:               strings.Split(getEnv("REPLICA_DSNS", "postgres://user:pass@localhost:5433/mydb,postgres://user:pass@localhost:5434/mydb"), ","),
		MaxRetries:                5,
		RetryDelay:                200 * time.Millisecond,
		HealthCheckPeriod:         5 * time.Second,
		CircuitBreakerMaxFailures: 3,
		CircuitBreakerTimeout:     30 * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// 2. МЕНЕДЖЕР ПОДКЛЮЧЕНИЙ (MASTER + REPLICAS)

type DBManager struct {
	config         DBConfig
	master         *pgxpool.Pool
	replicas       []*pgxpool.Pool
	mu             sync.RWMutex
	isMasterAlive  bool
	isReadOnly     bool
	replicaIndex   int
	circuitBreaker *CircuitBreaker
	healthCtx      context.Context
	healthCancel   context.CancelFunc
}

func NewDBManager(cfg DBConfig) (*DBManager, error) {
	// Подключаемся к мастеру
	master, err := pgxpool.New(context.Background(), cfg.MasterDSN)
	if err != nil {
		return nil, fmt.Errorf("master connection: %w", err)
	}

	// Подключаемся к репликам
	var replicas []*pgxpool.Pool
	for _, dsn := range cfg.ReplicaDSNs {
		if dsn == "" {
			continue
		}
		pool, err := pgxpool.New(context.Background(), dsn)
		if err != nil {
			log.Printf("⚠️ Replica connection failed: %v", err)
			continue
		}
		replicas = append(replicas, pool)
	}

	cb := NewCircuitBreaker(cfg.CircuitBreakerMaxFailures, cfg.CircuitBreakerTimeout)

	healthCtx, healthCancel := context.WithCancel(context.Background())

	return &DBManager{
		config:         cfg,
		master:         master,
		replicas:       replicas,
		isMasterAlive:  true,
		isReadOnly:     false,
		circuitBreaker: cb,
		healthCtx:      healthCtx,
		healthCancel:   healthCancel,
	}, nil
}

func (m *DBManager) Close() {
	m.healthCancel()
	if m.master != nil {
		m.master.Close()
	}
	for _, p := range m.replicas {
		p.Close()
	}
}

// 3. HEALTH CHECK

func (m *DBManager) StartHealthCheck() {
	go func() {
		ticker := time.NewTicker(m.config.HealthCheckPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-m.healthCtx.Done():
				log.Println("Health check stopped")
				return
			case <-ticker.C:
				m.checkHealth()
			}
		}
	}()
}

func (m *DBManager) checkHealth() {
	// Проверка мастера
	err := m.master.Ping(context.Background())
	if err != nil {
		log.Printf("❌ Master is DOWN: %v", err)
		m.mu.Lock()
		if m.isMasterAlive {
			m.isMasterAlive = false
			m.isReadOnly = true
			log.Println("⚠️ Switching to READ-ONLY mode")
		}
		m.mu.Unlock()
	} else {
		m.mu.Lock()
		if !m.isMasterAlive {
			log.Println("✅ Master is UP again! Switching back to normal mode")
			m.isMasterAlive = true
			m.isReadOnly = false
		}
		m.mu.Unlock()
	}

	// Проверка реплик
	for i, pool := range m.replicas {
		err := pool.Ping(context.Background())
		if err != nil {
			log.Printf("❌ Replica %d is DOWN: %v", i, err)
		} else {
			log.Printf("✅ Replica %d is UP", i)
		}
	}
}

// 4. CIRCUIT BREAKER

type CircuitBreaker struct {
	failures    int
	maxFailures int
	state       string // "closed", "open", "half-open"
	mu          sync.Mutex
	lastFail    time.Time
	timeout     time.Duration
}

func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures: maxFailures,
		state:       "closed",
		timeout:     timeout,
	}
}

func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == "open" {
		if time.Since(cb.lastFail) > cb.timeout {
			cb.state = "half-open"
			cb.failures = 0
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}

	err := fn()
	if err != nil {
		cb.failures++
		cb.lastFail = time.Now()
		if cb.failures >= cb.maxFailures {
			cb.state = "open"
			log.Printf("Circuit breaker opened after %d failures", cb.failures)
		}
		return err
	}

	// Success
	cb.failures = 0
	cb.state = "closed"
	return nil
}

// 5. ЗАПИСЬ (MASTER) С RETRY И CIRCUIT BREAKER

func (m *DBManager) isMasterError(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "57P01", "57P02", "53300", "08006":
			return true
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "server closed the connection") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "too many connections") ||
		strings.Contains(msg, "terminating connection")
}

func (m *DBManager) Write(ctx context.Context, query string, args ...interface{}) error {
	return m.circuitBreaker.Call(func() error {
		var lastErr error
		for attempt := 0; attempt < m.config.MaxRetries; attempt++ {
			// Проверяем статус мастера
			m.mu.RLock()
			alive := m.isMasterAlive
			m.mu.RUnlock()

			if !alive {
				log.Printf("⚠️ Master is down, attempt %d/%d, waiting for failover...", attempt+1, m.config.MaxRetries)
				time.Sleep(time.Duration(1<<attempt) * 2 * time.Second)
				continue
			}

			// Выполняем запрос
			_, err := m.master.Exec(ctx, query, args...)
			if err == nil {
				return nil
			}

			if m.isMasterError(err) {
				log.Printf("❌ Master error: %v, attempt %d/%d", err, attempt+1, m.config.MaxRetries)
				m.mu.Lock()
				m.isMasterAlive = false
				m.isReadOnly = true
				m.mu.Unlock()
				time.Sleep(time.Duration(1<<attempt) * 3 * time.Second)
				continue
			}

			// Нефатальная ошибка
			if attempt < m.config.MaxRetries-1 {
				time.Sleep(m.config.RetryDelay)
				continue
			}
			lastErr = err
		}
		return fmt.Errorf("failed after %d attempts: %w", m.config.MaxRetries, lastErr)
	})
}

// 6. ЧТЕНИЕ (REPLICAS) С БАЛАНСИРОВКОЙ

func (m *DBManager) Read(ctx context.Context, query string, args ...interface{}) pgx.Row {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.replicas) == 0 {
		return m.master.QueryRow(ctx, query, args...)
	}

	// Round-robin
	m.mu.Lock()
	idx := m.replicaIndex % len(m.replicas)
	m.replicaIndex++
	m.mu.Unlock()

	return m.replicas[idx].QueryRow(ctx, query, args...)
}

func (m *DBManager) ReadMany(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.replicas) == 0 {
		return m.master.Query(ctx, query, args...)
	}

	// Случайная реплика
	idx := time.Now().UnixNano() % int64(len(m.replicas))
	return m.replicas[idx].Query(ctx, query, args...)
}

func (m *DBManager) IsMasterAlive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isMasterAlive
}

func (m *DBManager) IsReadOnly() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isReadOnly
}

// 7. БИЗНЕС-СЕРВИС (NOTIFICATION SERVICE)

type Notification struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type NotificationService struct {
	db *DBManager
}

func NewNotificationService(db *DBManager) *NotificationService {
	return &NotificationService{db: db}
}

func (s *NotificationService) Create(ctx context.Context, userID int64, message string) error {
	if s.db.IsReadOnly() {
		log.Println("⚠️ Master is down, write operation rejected")
		return fmt.Errorf("master unavailable: write operation rejected")
	}
	return s.db.Write(ctx,
		"INSERT INTO notifications (user_id, message, status, created_at) VALUES ($1, $2, $3, $4)",
		userID, message, "pending", time.Now(),
	)
}

func (s *NotificationService) GetByUser(ctx context.Context, userID int64) ([]Notification, error) {
	rows, err := s.db.ReadMany(ctx,
		"SELECT id, user_id, message, status, created_at FROM notifications WHERE user_id = $1 ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		// Fallback на мастер, если реплики недоступны
		if s.db.IsMasterAlive() {
			log.Println("Replicas unavailable, falling back to master for read")
			rows, err = s.db.master.Query(ctx,
				"SELECT id, user_id, message, status, created_at FROM notifications WHERE user_id = $1 ORDER BY created_at DESC",
				userID,
			)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	defer rows.Close()

	var items []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Message, &n.Status, &n.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	return items, nil
}

// 8. HTTP HANDLERS

type CreateRequest struct {
	UserID  int64  `json:"user_id"`
	Message string `json:"message"`
}

func setupRouter(svc *NotificationService) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		alive := svc.db.IsMasterAlive()
		readonly := svc.db.IsReadOnly()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "ok",
			"master_alive": alive,
			"read_only":    readonly,
		})
	})

	mux.HandleFunc("GET /notifications", func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.URL.Query().Get("user_id")
		if userIDStr == "" {
			http.Error(w, "missing user_id", http.StatusBadRequest)
			return
		}
		var userID int64
		if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
			http.Error(w, "invalid user_id", http.StatusBadRequest)
			return
		}

		notifs, err := svc.GetByUser(r.Context(), userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(notifs)
	})

	mux.HandleFunc("POST /notifications", func(w http.ResponseWriter, r *http.Request) {
		var req CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.UserID <= 0 || req.Message == "" {
			http.Error(w, "user_id and message required", http.StatusBadRequest)
			return
		}

		err := svc.Create(r.Context(), req.UserID, req.Message)
		if err != nil {
			if strings.Contains(err.Error(), "master unavailable") {
				http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"ok"}`))
	})

	return mux
}

// 9. GRACEFUL SHUTDOWN

func gracefulShutdown(server *http.Server, db *DBManager) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	db.Close()
	log.Println("Shutdown complete")
}

// 10. MAIN

func main() {
	cfg := LoadDBConfig()
	log.Println("Starting DB Manager...")
	db, err := NewDBManager(cfg)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	defer db.Close()

	db.StartHealthCheck()

	svc := NewNotificationService(db)
	mux := setupRouter(svc)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Println("Server listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	gracefulShutdown(server, db)
}
