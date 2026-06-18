package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Config содержит настройки для подсчёта
type Config struct {
	NumWorkers    int  // количество воркеров
	BufferSize    int  // размер буфера для сканера
	MinWordLength int  // минимальная длина слова
	IgnoreCase    bool // игнорировать регистр
	TopN          int  // количество слов в топе
}

// DefaultConfig возвращает настройки по умолчанию
func DefaultConfig() *Config {
	return &Config{
		NumWorkers:    runtime.NumCPU(),
		BufferSize:    5 * 1024 * 1024, // 5 MB
		MinWordLength: 1,
		IgnoreCase:    true,
		TopN:          10,
	}
}

// WordCounter — основной счётчик
type WordCounter struct {
	config *Config
	freq   map[string]int
	mu     sync.Mutex
	total  int
	lines  int
}

// NewWordCounter создаёт новый счётчик
func NewWordCounter(cfg *Config) *WordCounter {
	return &WordCounter{
		config: cfg,
		freq:   make(map[string]int),
	}
}

// ProcessFile обрабатывает файл
func (wc *WordCounter) ProcessFile(filename string) error {
	start := time.Now()
	log.Printf("Processing file: %s", filename)

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Каналы: строки на вход, результаты на выход
	lines := make(chan string, wc.config.NumWorkers*2)
	results := make(chan map[string]int, wc.config.NumWorkers)

	// Запускаем воркеров
	var wg sync.WaitGroup
	for i := 0; i < wc.config.NumWorkers; i++ {
		wg.Add(1)
		go wc.worker(lines, results, &wg)
	}

	// Читаем файл и отправляем строки в канал
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, wc.config.BufferSize)
	scanner.Buffer(buf, wc.config.BufferSize)

	lineCount := 0
	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
			lineCount++
		}
		close(lines)
	}()

	// Ждём завершения воркеров
	go func() {
		wg.Wait()
		close(results)
	}()

	// Собираем результаты
	for localFreq := range results {
		wc.mu.Lock()
		for word, count := range localFreq {
			wc.freq[word] += count
			wc.total += count
		}
		wc.mu.Unlock()
	}

	wc.lines = lineCount

	elapsed := time.Since(start)
	log.Printf("File processed in %v", elapsed)
	log.Printf("Total lines: %d", wc.lines)
	log.Printf("Total words: %d", wc.total)

	return nil
}

// worker — воркер, обрабатывающий строки
func (wc *WordCounter) worker(lines <-chan string, results chan<- map[string]int, wg *sync.WaitGroup) {
	defer wg.Done()

	// Локальная мапа для этого воркера
	localFreq := make(map[string]int)

	for line := range lines {
		words := splitIntoWords(line, wc.config.IgnoreCase, wc.config.MinWordLength)
		for _, word := range words {
			localFreq[word]++
		}
	}

	// Отправляем результаты
	results <- localFreq
}

// GetTopWords возвращает топ-N слов
func (wc *WordCounter) GetTopWords() []WordCount {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	// Превращаем map в слайс структур
	wordCounts := make([]WordCount, 0, len(wc.freq))
	for word, count := range wc.freq {
		wordCounts = append(wordCounts, WordCount{Word: word, Count: count})
	}

	// Сортируем по убыванию
	sort.Slice(wordCounts, func(i, j int) bool {
		if wordCounts[i].Count != wordCounts[j].Count {
			return wordCounts[i].Count > wordCounts[j].Count
		}
		return wordCounts[i].Word < wordCounts[j].Word
	})

	n := wc.config.TopN
	if n > len(wordCounts) {
		n = len(wordCounts)
	}
	return wordCounts[:n]
}

// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ

// WordCount структура для вывода
type WordCount struct {
	Word  string
	Count int
}

// splitIntoWords разбивает строку на слова
func splitIntoWords(line string, ignoreCase bool, minLen int) []string {
	if ignoreCase {
		line = strings.ToLower(line)
	}

	fields := strings.Fields(line)
	words := make([]string, 0, len(fields))

	for _, field := range fields {
		// Убираем пунктуацию в начале и конце
		word := strings.TrimFunc(field, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if len(word) >= minLen {
			words = append(words, word)
		}
	}

	return words
}

// ПРИМЕР ИСПОЛЬЗОВАНИЯ

func main() {
	cfg := DefaultConfig()
	cfg.NumWorkers = 4
	cfg.MinWordLength = 2
	cfg.TopN = 20

	counter := NewWordCounter(cfg)

	filename := "large_file.txt"
	if err := counter.ProcessFile(filename); err != nil {
		log.Fatalf("Error: %v", err)
	}

	topWords := counter.GetTopWords()
	fmt.Printf("\nTop %d most frequent words:\n", cfg.TopN)
	for i, wc := range topWords {
		fmt.Printf("%2d. %-15s %d\n", i+1, wc.Word, wc.Count)
	}
}
