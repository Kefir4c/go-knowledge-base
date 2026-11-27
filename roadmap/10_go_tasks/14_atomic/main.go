package main

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

/*
Реализация своего Mutex (Spinlock)
Задача "Как бы ты реализовал sync.Mutex с нуля, используя только sync/atomic?"
Суть: Называется Spinlock (Спин-лок). Вместо того чтобы усыплять горутину
(как делает системный мьютекс), мы заставляем её крутиться в цикле и долбить
процессор, пытаясь захватить флаг (CAS). Работает быстро, но жрет CPU, если лок держат долго.
*/
/*
type SpinLock struct {
	flag atomic.Bool
}

func (s *SpinLock) Lock() {
	for {
		if s.flag.CompareAndSwap(false, true) {
			return
		}
	}
}

func (s *SpinLock) Unlock() {
	s.flag.Store(false)
}

func main() {
	var lock SpinLock
	var wg sync.WaitGroup
	count := 0

	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock.Lock()
			count++
			lock.Unlock()
		}()
	}
	wg.Wait()
	fmt.Println("count:", count)
}
*/

/*
Упаковка состояний (Bit Packing)
Задача: "У нас есть система, где нужно атомарно управлять 3 флагами (например:
isRunning, isPaused, hasError) и версией конфига. Как сделать это без мьютексов?"
Суть: Вместо 4 разных переменных мы используем одну 64-битную переменную (atomic.Uint64).
Флаги хранятся в младших битах, а версия — в старших. Мы меняем их атомарно с
помощью побитовых операций и CAS (чтобы избежать состояния гонки при обновлении).
*/
/*
type SystemState struct {
	data atomic.Uint64
}

func (s *SystemState) SetRunning(inRunning bool) {
	for {
		old := s.data.Load()
		var newVal uint64
		if inRunning {
			newVal = old | 1
		} else {
			newVal = old &^ 1
		}
		if s.data.CompareAndSwap(old, newVal) {
			return
		}
	}
}

func (s *SystemState) SetPaused(inPaused bool) {
	for {
		old := s.data.Load()
		var newVal uint64
		if inPaused {
			newVal = old | (1 << 1)
		} else {
			newVal = old &^ (1 << 1)
		}
		if s.data.CompareAndSwap(old, newVal) {
			return
		}
	}
}

func (s *SystemState) GetStatus() (isRunning, isPaused bool) {
	val := s.data.Load()
	isRunning = (val & 1) == 1
	isPaused = (val & (1 << 1)) == (1 << 1)
	return
}

func main() {
	var state SystemState
	state.SetRunning(true)
	state.SetPaused(true)

	running, paused := state.GetStatus()
	fmt.Printf("Running: %v, Paused: %v\n", running, paused)
}
*/

/*
Lock-Free Ring Buffer (Кольцевой буфер)
Задача: "Напиши кольцевой буфер фиксированного размера,
где Producer и Consumer работают параллельно без единого мьютекса".
Суть: В high-frequency trading (HFT) и логгерах мьютексы недопустимы из-за
задержек. Мы используем массив фиксированного размера и два atomic.Uint64
(head и tail). Producer двигает head, Consumer двигает tail.
*/
/*
type LockFreeRingBuffer struct {
	data     []int
	capacity uint64
	head     atomic.Uint64
	tail     atomic.Uint64
}

func NewLockFreeRingBuffer(size uint64) *LockFreeRingBuffer {
	return &LockFreeRingBuffer{
		data:     make([]int, size),
		capacity: size,
	}
}

func (l *LockFreeRingBuffer) Push(val int) bool {
	h := l.head.Load()
	t := l.tail.Load()

	if h-t >= l.capacity {
		return false
	}
	l.data[h&(l.capacity-1)] = val
	l.head.Store(h)
	return true
}

func (l *LockFreeRingBuffer) Pop() (int, bool) {
	h := l.head.Load()
	t := l.tail.Load()

	if h == t {
		return 0, false
	}

	val := l.data[t&(l.capacity-1)]
	l.tail.Add(1)
	return val, true
}

func main() {
	l := NewLockFreeRingBuffer(4)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= 100; i++ {
			for !l.Push(i) {
				// Ждем, пока буфер не освободится (spin)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= 10; i++ {
			val, ok := l.Pop()
			for !ok {
				runtime.Gosched()
				val, ok = l.Pop()
			}
			fmt.Printf("Consumed: %d\n", val)
		}
	}()
	wg.Wait()
}
*/

/*
Atomic Reference Counting (Управление жизненным циклом)
Задача:"У нас есть разделяемый ресурс (например, сетевое соединение). Несколько горутин им пользуются.
Как безопасно закрыть соединение, когда последняя горутина закончит работу, не используя sync.WaitGroup?"
Суть: Проблема WaitGroup в том, что если ты вызовешь wg.Add(1) после того, как горутина сделала wg.Wait(),
программа запаникует. Мы реализуем свой Reference Counting (Счетчик ссылок), как в C++ или Rust.
*/

type Resource struct {
	name     string
	refCount atomic.Int32
	closed   atomic.Bool
}

func NewResource(name string) *Resource {
	r := &Resource{name: name}
	r.refCount.Store(1)
	return r
}

func (r *Resource) Acquire() error {
	for {
		count := r.refCount.Load()
		if count == 0 || r.closed.Load() {
			return errors.New("resource is closed")
		}

		if r.refCount.CompareAndSwap(count, count+1) {
			return nil
		}
	}
}

func (r *Resource) Release() {
	newCount := r.refCount.Add(-1)
	if newCount == 0 {
		r.closed.Store(true)
		fmt.Printf("Ресурс '%s' закрыт, так как счетчик ссылок достиг 0\n", r.name)
	}
}

func main() {
	res := NewResource("DatabaseConnection")

	for i := 1; i <= 100; i++ {
		if err := res.Acquire(); err != nil {
			go func(id int) {
				fmt.Printf("Горутина %d использует ресурс\n", id)
				time.Sleep(100 * time.Millisecond)
				res.Release()
			}(i)
		}
	}
	res.Release()
	time.Sleep(500 * time.Millisecond)
}
