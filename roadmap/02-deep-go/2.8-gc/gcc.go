package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"time"
)

//1. Полноценный мониторинг GC с выводом в таблицу

type GCMetrics struct {
	NumGC         uint32
	LastPause     time.Duration
	PauseTotal    time.Duration
	HeapAlloc     uint64
	HeapObjects   uint64
	NextGC        uint64
	LastPauseTime time.Time
}

func getGCMetrics() GCMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	lastIdx := (m.NumGC - 1) % 256
	return GCMetrics{
		NumGC:         m.NumGC,
		LastPause:     time.Duration(m.PauseNs[lastIdx]),
		PauseTotal:    time.Duration(m.PauseTotalNs),
		HeapAlloc:     m.HeapAlloc,
		HeapObjects:   m.HeapObjects,
		NextGC:        m.NextGC,
		LastPauseTime: time.Unix(0, int64(m.LastGC)),
	}
}

func primer1() {
	fmt.Printf("Initial GOGC = %d\n", debug.SetGCPercent(-1)) // -1 только читает
	fmt.Printf("GOMEMLIMIT = %v\n", debug.SetMemoryLimit(-1))

	go func() {
		var hold [][]byte
		for {
			hold := append(hold, make([]byte, 1<<20))
			if len(hold) > 100 {
				hold = nil
				runtime.GC()
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	tick := time.Tick(2 * time.Second)
	for range tick {
		m := getGCMetrics()
		fmt.Printf("GC #%-4d | last: %-8v | total: %-10v | heap: %-7.1fMB | objects: %d | next: %.1fMB\n",
			m.NumGC,
			m.LastPause.Truncate(time.Microsecond),
			m.PauseTotal.Truncate(time.Microsecond),
			float64(m.HeapAlloc)/1e6,
			m.HeapObjects,
			float64(m.NextGC)/1e6,
		)
	}
}

//2.Эксперимент с GOMEMLIMIT и GOGC – убиваем память

var (
	gogc      = flag.Int("gogc", 500, "GOGC (0=off)")
	memlimit  = flag.Int64("memlimit", 0, "GOMEMLIMIT (MB)")
	allocSize = flag.Int("size", 10, "MB per allocation")
	interval  = flag.Duration("interval", 5*time.Millisecond, "alloc interval")
)

func primer2() {
	flag.Parse()
	if *gogc >= 0 {
		debug.SetGCPercent(*gogc)
	}
	if *memlimit > 0 {
		debug.SetMemoryLimit(*memlimit << 20)
	}

	fmt.Printf("GOGC = %d, GOMEMLIMIT = %d MB\n", *gogc, *memlimit)

	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		log.Printf("Alloc = %.1f MB | Sys = %.1f MB | HeapObjects = %d | NumGC = %d",
			float64(m.Alloc)/1e6,
			float64(m.Sys)/1e6,
			m.HeapObjects,
			m.NumGC,
		)
	}

	var shunks [][]byte
	for {
		shunks = append(shunks, make([]byte, *allocSize<<20))
		time.Sleep(*interval)
	}
}

func primer3() {
	// Настраиваем сбор метрик
	samples := []metrics.Sample{
		{Name: "/gc/pauses:seconds"},
		{Name: "/sched/latencies:seconds"},
	}

	go func() {
		var leak [][]byte
		for {
			leak = append(leak, make([]byte, 1<<20))
			if len(leak) > 200 {
				leak = nil
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	for i := 0; i < 20; i++ {
		time.Sleep(1 * time.Second)
		metrics.Read(samples)

		// Суммируем все паузы GC за последнюю секунду
		var gcPauses float64

		for _, s := range samples[0].Value.Float64Histogram().Buckets {
			gcPauses += s
		}

		log.Printf(
			"GC pauses (total last sec): %.3f ms, scheduling latency (max): %.3f ms",
			gcPauses*1000,
			samples[1].Value.Float64Histogram(),
		)
	}
}

func primer4() {
	fmt.Println("=== GC Load Measurement ===")
	fmt.Println("Press Ctrl+C to stop")

	// Запускаем фоновую нагрузку
	go allocateMemory()

	// Измеряем GC CPU fraction каждые 2 секунды
	tick := time.NewTicker(2 * time.Second)
	for range tick.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// GCCPUFraction — доля процессорного времени, потраченная на GC с момента старта
		percent := m.GCCPUFraction * 100

		fmt.Printf(
			"GC CPU: %.2f%% | NumGC: %d | HeapAlloc: %.1f MB | PauseTotal: %v\n",
			percent,
			m.NumGC,
			float64(m.HeapAlloc)/1e6,
			time.Duration(m.PauseTotalNs),
		)

		// Если GC ещё не запускался, принудительно вызываем его один раз
		if m.NumGC == 0 {
			fmt.Println("  -> Forcing first GC...")
			runtime.GC()
		}
	}
}

func allocateMemory() {
	var keep [][]byte
	for {
		// Каждые 100 мс выделяем 5 MB
		keep = append(keep, make([]byte, 60<<20))
		time.Sleep(100 * time.Nanosecond)

		// Не даём памяти расти бесконечно
		if len(keep) > 30 {
			keep = nil
			runtime.GC()
		}
	}
}

func worker() {
	for {
		// Выделяем 5 МБ и тут же теряем ссылку
		_ = make([]byte, 5<<20)
		// Делаем полезную работу (например, сжигаем CPU)
		for i := 0; i < 1000; i++ {
			_ = i * i
		}
		// Если GC не успевает, немного замедлимся
		time.Sleep(100 * time.Microsecond)
	}
}

func primer5() {
	// Запускаем много горутин, которые постоянно выделяют и бросают память
	for i := 0; i < runtime.NumCPU()*2; i++ {
		go worker()
	}

	tick := time.NewTicker(1 * time.Second)
	for range tick.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		fmt.Printf("GC CPU fraction (total): %.2f%% | NumGC: %d | Live heap: %.1f MB\n",
			m.GCCPUFraction*100, m.NumGC, float64(m.HeapAlloc)/1e6)
	}
}

func main() {
	//primer1()
	//primer2()
	//primer3()
	//primer4()
	primer5()
}
