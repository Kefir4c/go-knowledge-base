package balance

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"

	pb "github.com/"
)

// КОНФИГУРАЦИЯ
const (
	totalRequests  = 100
	concurrency    = 10
	requestTimeout = 3 * time.Second
)

// ─── КЛИЕНТСКАЯ СТАТИСТИКА
type ClientStats struct {
	mu         sync.Mutex
	requests   int64
	errors     int64
	byServer   map[string]int64
	latencySum time.Duration
	latencyMax time.Duration
}

func (s *ClientStats) Record(serverID string, latency time.Duration, err error) {
	atomic.AddInt64((&s.requests), 1)
	if err != nil {
		atomic.AddInt64((&s.errors), 1)
		return
	}
	s.mu.Lock()
	s.byServer[serverID]++
	s.latencySum += latency
	if latency > s.latencyMax {
		s.latencyMax = latency
	}
	s.mu.Unlock()
}

func (s *ClientStats) Print() {
	s.mu.Lock()
	defer s.mu.Unlock()

	fmt.Println("СТАТИСТИКА КЛИЕНТА")
	fmt.Printf("Всего запросов:%d\n", atomic.LoadInt64(&s.requests))
	fmt.Printf("Ошибок:%d\n", atomic.LoadInt64(&s.errors))
	fmt.Printf("Средняя задержка:%.2fms\n", float64(s.latencySum.Milliseconds())/float64(atomic.LoadInt64(&s.requests)))
	fmt.Printf("Макс. задержка:%v\n", s.latencyMax)
	fmt.Println("\n РАСПРЕДЕЛЕНИЕ ПО СЕРВЕРАМ:")
	for server, count := range s.byServer {
		percent := float64(count) / float64(atomic.LoadInt64(&s.requests)) * 100
		fmt.Printf("%s → %d запросов (%.1f%%)\n", server, count, percent)
	}
}

func main() {
	// 1. НАСТРОЙКА РЕЗОЛВЕРА (локальные адреса)
	// В продакшене используй dns:///service-name:50051
	// Здесь — ручной резолвер для локальной разработки
	r := manual.NewBuilderWithScheme("manual")
	addrs := []resolver.Address{
		{Addr: "localhost:50051", ServerName: "srv-a"},
		{Addr: "localhost:50052", ServerName: "srv-b"},
		{Addr: "localhost:50053", ServerName: "srv-c"},
	}
	r.UpdateState(resolver.State{Addresses: addrs})
	resolver.Register(r)

	// 2. ПОДКЛЮЧЕНИЕ С ROUND_ROBIN
	conn, err := grpc.NewClient(r.Scheme()+":///",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	if err != nil {
		log.Fatalf("connect error: %v", err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)
	stats := &ClientStats{byServer: make(map[string]int64)}

	// 3. ПРОВЕРКА HEALTH (опционально)
	healthClient := healthpb.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	resp, _ := healthClient.Check(ctx, &healthpb.HealthCheckRequest{})
	cancel()
	log.Printf("Health check: %v", resp.Status)

	log.Printf("Запуск клиента с round_robin (%d запросов, %d параллельно)",
		totalRequests, concurrency)

	// 4. ОТПРАВКА ЗАПРОСОВ (параллельно)
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	startTime := time.Now()

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			defer cancel()

			userID := fmt.Sprintf("%d", (idx%5)+1)
			req := &pb.GetUserRequest{Id: userID}

			start := time.Now()
			resp, err := client.GetUser(ctx, req)
			latency := time.Since(start)

			if err != nil {
				stats.Record("", latency, err)
				log.Printf("[%d] error: %v", idx+1, err)
				return
			}

			stats.Record(userID, latency, nil)
			log.Printf("[%d] → %s (%v)", idx+1, resp.ProcessedBy, latency.Round(time.Millisecond))
		}(i)
	}
	wg.Wait()
	totalTime := time.Since(startTime)

	// 5. ВЫВОД СТАТИСТИКИ
	log.Printf("\nбщее время: %v", totalTime)
	stats.Print()

	// 6. ЗАВЕРШЕНИЕ ПО СИГНАЛУ
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Клиент завершён")
}
