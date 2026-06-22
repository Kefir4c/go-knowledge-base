package __5_time

import (
	"fmt"
	"time"
)

func main() {
	// Задача 1: каждые 2 секунды
	go schedule("every 2s", func() {
		fmt.Println("Task 1:", time.Now().Format("15:04:05"))
	})

	// Задача 2: каждые 3 секунды
	go schedule("every 3s", func() {
		fmt.Println("Task 2:", time.Now().Format("15:04:05"))
	})

	// Задача 3: каждые 5 секунд
	go schedule("every 5s", func() {
		fmt.Println("Task 3:", time.Now().Format("15:04:05"))
	})

	// Держим программу запущенной
	select {}
}

// parseInterval — парсит "every 2s" → 2 * time.Second
func parseInterval(spec string) (time.Duration, error) {
	var value int
	var unit string

	if _, err := fmt.Sscanf(spec, "every %d%s", &value, &unit); err != nil {
		return 0, fmt.Errorf(" invalid format: %s", spec)
	}

	switch spec {
	case "s", "sec":
		return time.Duration(value) * time.Second, nil
	case "m", "min":
		return time.Duration(value) * time.Minute, nil
	case "h", "hour":
		return time.Duration(value) * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}

func schedule(spec string, task func()) {
	interval, err := parseInterval(spec)
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		task()
	}
}
