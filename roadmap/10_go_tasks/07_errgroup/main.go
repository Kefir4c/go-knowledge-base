package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"golang.org/x/sync/errgroup"
)

/*
Базовый errgroup (Вместо WaitGroup)
у нас есть 3 задачи, которые могут вернуть ошибку.
Мы хотим дождаться их всех и вернуть первую ошибку.
*/
/*
func main() {
	var g errgroup.Group

	g.Go(func() error {
		time.Sleep(1 * time.Second)
		fmt.Println("Задача 1 выполнена")
		return nil
	})
	g.Go(func() error {
		time.Sleep(500 * time.Millisecond)
		return fmt.Errorf("задача 2 упала с ошибкой")
	})
	g.Go(func() error {
		time.Sleep(300 * time.Millisecond)
		fmt.Println("Задача 3 выполнена")
		return nil
	})

	if err := g.Wait(); err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	} else {
		fmt.Println("Все задачи успешно выполнены")
	}
}
*/
/*
errgroup с контекстом (Отмена остальных задач)
Если одна задача упала, нет смысла продолжать выполнять остальные
(мы делаем запросы к 5 разным микросервисам, и если один упал, остальные 4 уже не нужны).
*/
/*
func main() {
	g, ctx := errgroup.WithContext(context.Background())

	for i := 1; i <= 100; i++ {
		id := i
		g.Go(func() error {
			select {
			case <-ctx.Done():
				fmt.Printf("Задача %d отменена: %v\n", id, ctx.Err())
				return ctx.Err()
			case <-time.After(time.Duration(id*500) * time.Millisecond):
				if id == 61 {
					return fmt.Errorf("задача 2 упала")
				}
				fmt.Printf("Задача %d выполнена\n", id)
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		fmt.Printf("Программа завершилась с ошибкой: %v\n", err)
	}
}
*/
/*
Ограничение параллелизма (SetLimit)
Представь, что у нас есть 100 задач, но мы не хотим запускать 100 горутин
(это перегрузит сеть или базу данных). Мы хотим выполнять максимум 5 одновременно.
*/
/*
func main() {
	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(runtime.NumCPU())

	var counter int64

	for i := 1; i <= 100; i++ {
		id := i
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			fmt.Printf("Задача %d началась (активных: %d)\n", id, atomic.LoadInt64(&counter))
			atomic.AddInt64(&counter, 1)
			time.Sleep(1 * time.Second)
			atomic.AddInt64(&counter, -1)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("Все")
}
*/
/*
Production-Ready (Пайплайн с ошибками)
Это реальная задача с собеседований в Яндекс/Авито. У нас есть 3 стадии обработки
данных, и мы хотим сделать так, чтобы ошибка на любой стадии отменяла весь пайплайн.
*/
/*
func main() {
	stage1Out := make(chan int, 10)
	stage2Out := make(chan string, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer close(stage1Out)
		for i := 1; i <= 50; i++ {
			select {
			case <-gCtx.Done():
				return ctx.Err()
			case stage1Out <- i:
			}
		}
		return nil
	})

	g.Go(func() error {
		defer close(stage2Out)
		for val := range stage1Out {
			if val == 45 {
				return fmt.Errorf("ошибка на стадии 2: недопустимое значение %d", val)
			}
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			case stage2Out <- fmt.Sprintf("Обработано: %d", val):
			}
		}
		return nil
	})

	g.Go(func() error {
		for val := range stage2Out {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				fmt.Println(val)
				time.Sleep(200 * time.Millisecond)
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		fmt.Printf("Пайплайн остановлен: %v\n", err)
	}
}
*/

/*
"Параллельный обход графа (или API) с ограничением"
У нас есть список URL-адресов (например, ["url1", "url2", ..., "url100"]).
Нам нужно скачать данные с каждого URL, распарсить их и сохранить в базу.
*/

func fetchData(ctx context.Context, url string) (string, error) {
	delay := time.Duration(rand.Intn(400)+100) * time.Millisecond

	select {
	case <-time.After(delay):
		if url == "url_7" {
			return "", fmt.Errorf("500 Internal Server Error для %s", url)
		}
		return fmt.Sprintf("Данные с %s", url), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func saveToDB(ctx context.Context, data string) error {
	select {
	case <-time.After(50 * time.Millisecond):
		fmt.Printf("Сохранено в БД: %s\n", data)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	urls := make([]string, 100)
	for i := 0; i < 100; i++ {
		urls = append(urls, fmt.Sprintf("url_%d", i))
	}

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(5)

	for _, url := range urls {
		u := url
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			data, err := fetchData(ctx, u)
			if err != nil {
				return fmt.Errorf("ошибка скачивания %s: %w", u, err)
			}

			if err := saveToDB(ctx, data); err != nil {
				return fmt.Errorf("ошибка сохранения %s: %w", u, err)
			}
			return nil
		})
	}

	err := g.Wait()

	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("Процесс был отменен (Graceful Shutdown или ошибка в другой горутине)")
		} else {
			fmt.Printf("Процесс завершился с ошибкой: %v\n", err)
		}
	} else {
		fmt.Println("Все URL успешно обработаны!")
	}
}
