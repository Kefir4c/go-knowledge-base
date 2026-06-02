package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

// ============================================
// 1. ИНТЕРФЕЙС TaskStore
// ============================================

type TaskStore interface {
	Save(task string) error
	Get(id int) (string, error)
	List() ([]string, error)
	Delete(id int) error
}

// ============================================
// 2. РЕАЛЬНАЯ РЕАЛИЗАЦИЯ (InMemoryStore)
// ============================================

type InMemoryStore struct {
	tasks  map[int]string
	nextID int
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		tasks:  make(map[int]string),
		nextID: 1,
	}
}

func (s *InMemoryStore) Save(task string) error {
	s.tasks[s.nextID] = task
	s.nextID++
	return nil
}

func (s *InMemoryStore) Get(id int) (string, error) {
	task, ok := s.tasks[id]
	if !ok {
		return "", fmt.Errorf("task %d not found", id)
	}
	return task, nil
}

func (s *InMemoryStore) List() ([]string, error) {
	tasks := make([]string, 0, len(s.tasks))
	for i := 1; i < s.nextID; i++ {
		if task, ok := s.tasks[i]; ok {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

func (s *InMemoryStore) Delete(id int) error {
	if _, ok := s.tasks[id]; !ok {
		return fmt.Errorf("task %d not found", id)
	}
	delete(s.tasks, id)
	return nil
}

// ============================================
// 3. ЛОГИРУЮЩАЯ ОБЁРТКА (через эмбеддинг)
// ============================================

type LoggingTaskStore struct {
	TaskStore // эмбеддинг интерфейса!
	logger    *log.Logger
}

func NewLoggingTaskStore(store TaskStore, logger *log.Logger) *LoggingTaskStore {
	return &LoggingTaskStore{
		TaskStore: store,
		logger:    logger,
	}
}

// Переопределяем методы с логированием
func (l *LoggingTaskStore) Save(task string) error {
	l.logger.Printf("[INFO] Save task: %q", task)

	start := time.Now()
	err := l.TaskStore.Save(task)

	l.logger.Printf("[INFO] Save completed in %v, error: %v", time.Since(start), err)
	return err
}

func (l *LoggingTaskStore) Get(id int) (string, error) {
	l.logger.Printf("[INFO] Get task: id=%d", id)

	start := time.Now()
	task, err := l.TaskStore.Get(id)

	l.logger.Printf("[INFO] Get completed in %v, result: %q, error: %v", time.Since(start), task, err)
	return task, err
}

func (l *LoggingTaskStore) List() ([]string, error) {
	l.logger.Printf("[INFO] List all tasks")

	start := time.Now()
	tasks, err := l.TaskStore.List()

	l.logger.Printf("[INFO] List completed in %v, count: %d, error: %v", time.Since(start), len(tasks), err)
	return tasks, err
}

func (l *LoggingTaskStore) Delete(id int) error {
	l.logger.Printf("[INFO] Delete task: id=%d", id)

	start := time.Now()
	err := l.TaskStore.Delete(id)

	l.logger.Printf("[INFO] Delete completed in %v, error: %v", time.Since(start), err)
	return err
}

// ============================================
// 4. ПРИМЕР ИСПОЛЬЗОВАНИЯ
// ============================================

func main() {
	// Создаём логгер
	logger := log.New(os.Stdout, "[TASK] ", log.LstdFlags|log.Lmicroseconds)

	// Реальная реализация
	realStore := NewInMemoryStore()

	// Оборачиваем в логирующую
	store := NewLoggingTaskStore(realStore, logger)

	// Используем — всё логируется автоматически
	store.Save("Buy milk")
	store.Save("Write code")

	task, _ := store.Get(1)
	fmt.Println("Got task:", task)

	tasks, _ := store.List()
	fmt.Println("All tasks:", tasks)

	store.Delete(1)
	store.Delete(999) // ошибка — залогируется
}
