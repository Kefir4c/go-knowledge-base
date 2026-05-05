package errors

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
)

//1. ЧТО ТАКОЕ ОШИБКИ
/*
ЧТО ЭТО:
- Ошибка — это значение, которое указывает на проблему в выполнении операции.
- В Go ошибки представляются через интерфейс `error`.
- Это основной механизм обработки ошибок в языке.

ПОЧЕМУ ЭТО ВАЖНО:
- Позволяет писать надежный и предсказуемый код.
- Избегает паник в непредвиденных ситуациях.
- Реализует принцип "ошибки — это нормально".

ОСНОВНЫЕ ОПЕРАЦИИ:
- Создание: errors.New("message"), fmt.Errorf("format", ...)
- Обработка: if err != nil { ... }
- Возвращение: return nil, err

ПОДВОДНЫЕ КАМНИ В ДАННОМ БЛОКЕ:
- Нельзя игнорировать ошибки — это приводит к неожиданному поведению
- nil означает отсутствие ошибки
- Нельзя сравнивать ошибки через == в некоторых случаях (см. продвинутый уровень)
*/

func basicErr() {
	err := errors.New("error db")
	fmt.Println(err)

	if err != nil {
		fmt.Println("error:", err)
	}
}

func readFile(fileName string) ([]byte, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("file reading error %s: %w", fileName, err)
	}
	return data, nil
}

//2. СОЗДАНИЕ И ОБРАБОТКА ОШИБОК (WRAPPING)
/*
ТЕОРЕТИЧЕСКАЯ БАЗА (Для собеса):
- error в Go — это интерфейс, а не класс или исключение.
- Любой тип, у которого есть метод Error() string, автоматически считается ошибкой.
- Нулевое значение для интерфейса error — это nil. Если err == nil, значит операции прошла успешно.

Механика Wrapping (Оборачивания):
   - Появилась в Go 1.13.
   - Позволяет строить "стек" ошибок.
   - %w под капотом создает структуру, у которой есть метод Unwrap() error.

МЕХАНИЗМ ВОЗВРАТА:
- В Go принято возвращать ошибку последним аргументом.
- Мы не "выбрасываем" (throw) ошибки, а передаем их как обычные значения.

Когда использовать %w vs %v:
   - %w (Wrap): когда внешнему коду НУЖНО знать первопричину (например, "это была ошибка таймаута?").
   - %v (Value): когда мы хотим просто добавить текст, но СКРЫТЬ детали реализации (инкапсуляция ошибок).
*/

// 2.1. Использование интерфейса error и создание через fmt.Errorf
func processFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		// Создаем новую ошибку, добавляя контекст (имя файла),
		// чтобы в логах было понятно, где именно упало.
		return fmt.Errorf("failed to open config at %s: %v", path, err)
	}
	defer f.Close()
	return nil
}

// 2.2. Обработка сетевых запросов (базовый случай)
func fetchStatus(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("network call failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

// 2.3. ПЕРЕДАЧА ОШИБОК МЕЖДУ ГОРУТИНАМИ
/*
Проблема: горутина не может просто вернуть error в main.
Решение: используем каналы для доставки ошибок.
*/

func concurrentWorker(urls []string) error {
	errChan := make(chan error, len(urls))
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			if u == "" {
				errChan <- errors.New("empty url")
				return
			}
			errChan <- nil
		}(url)
	}

	// Ждем в отдельной горутине, чтобы не заблокировать чтение
	go func() {
		wg.Wait()
		close(errChan) // Закрываем, когда все закончили
	}()

	// Читаем все ошибки
	for err := range errChan {
		if err != nil {
			return err // Все еще возвращаем первую, но остальные не зависнут (благодаря буферу или закрытию)
		}
	}
	return nil
}
