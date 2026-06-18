package __4_ibo

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

/*
ТЕОРИЯ: ПАКЕТ IO

1. БАЗОВЫЕ ИНТЕРФЕЙСЫ

io.Reader — интерфейс для чтения данных.
  type Reader interface {
      Read(p []byte) (n int, err error)
  }
  • Читает до len(p) байт в p.
  • Возвращает количество прочитанных байт n (0..len(p)).
  • Если данные закончились — возвращает io.EOF.
  • Может прочитать меньше, чем len(p) — это нормально.

io.Writer — интерфейс для записи данных.
  type Writer interface {
      Write(p []byte) (n int, err error)
  }
  • Записывает до len(p) байт из p.
  • Возвращает количество записанных байт n (0..len(p)).
  • Может записать меньше, чем len(p) — это нормально.

io.Closer — интерфейс для закрытия ресурсов.
  type Closer interface {
      Close() error
  }

io.Seeker — интерфейс для перемещения по данным.
  type Seeker interface {
      Seek(offset int64, whence int) (int64, error)
  }

КОМБО-ИНТЕРФЕЙСЫ:
  type ReadCloser interface { Reader; Closer }
  type WriteCloser interface { Writer; Closer }
  type ReadWriteCloser interface { Reader; Writer; Closer }
  type ReadSeeker interface { Reader; Seeker }
  type WriteSeeker interface { Writer; Seeker }

ЧТО РЕАЛИЗУЕТ IO.READER:
  • os.File
  • strings.Reader, bytes.Reader
  • net.Conn
  • http.Response.Body
  • gzip.Reader

ЧТО РЕАЛИЗУЕТ IO.WRITER:
  • os.File
  • bytes.Buffer
  • net.Conn
  • http.ResponseWriter
  • gzip.Writer

2. ВАЖНЫЕ ФУНКЦИИ ПАКЕТА IO

func Copy(dst Writer, src Reader) (written int64, err error)
  • Копирует данные из src в dst.
  • Использует буфер 32KB внутри.
  • Возвращает количество скопированных байт.
  • Это самый эффективный способ копирования.

func CopyBuffer(dst Writer, src Reader, buf []byte) (written int64, err error)
  • То же, что Copy, но с кастомным буфером.

func CopyN(dst Writer, src Reader, n int64) (written int64, err error)
  • Копирует ровно n байт. Если меньше — ошибка.

func ReadAll(r Reader) ([]byte, error)
  • Читает всё до io.EOF в один слайс.
  • ⚠️ ОПАСНО: загружает всё в память! Только для маленьких данных.

func ReadFull(r Reader, buf []byte) (n int, err error)
  • Читает ровно len(buf) байт.
  • Если прочитано меньше — возвращает io.ErrUnexpectedEOF.

func ReadAtLeast(r Reader, buf []byte, min int) (n int, err error)
  • Читает минимум min байт.

func WriteString(w Writer, s string) (n int, err error)
  • Записывает строку в Writer (без преобразования в []byte).

3. СТАНДАРТНЫЕ РЕАЛИЗАЦИИ В ПАКЕТЕ IO

func LimitReader(r Reader, n int64) Reader
  • Ограничивает чтение n байтами.
  • После чтения n байт возвращает io.EOF.

func MultiReader(readers ...Reader) Reader
  • Объединяет несколько читателей в один.
  • Читает последовательно: первый, второй, третий...

func TeeReader(r Reader, w Writer) Reader
  • Читает из r и одновременно пишет в w.
  • Полезно для дублирования данных (логирование, кэширование).

var Discard Writer = devNull(0)
  • Приёмник, который ничего не делает с данными (как /dev/null).

func MultiWriter(writers ...Writer) Writer
  • Записывает данные во все writers одновременно.
  • Полезно для отправки данных в несколько мест (файл + консоль).

ТЕОРИЯ: ПАКЕТ BUFIO

1. BUFIO.READER — БУФЕРИЗИРОВАННОЕ ЧТЕНИЕ

bufio.Reader — обёртка над io.Reader с внутренним буфером.

СОЗДАНИЕ:
  reader := bufio.NewReader(r)
  reader := bufio.NewReaderSize(r, 1024*1024) // 1MB буфер

МЕТОДЫ:
  func (b *Reader) Read(p []byte) (n int, err error)
  func (b *Reader) ReadByte() (byte, error)
  func (b *Reader) ReadRune() (r rune, size int, err error)
  func (b *Reader) ReadString(delim byte) (string, error)
  func (b *Reader) ReadLine() (line []byte, isPrefix bool, err error)
  func (b *Reader) ReadBytes(delim byte) ([]byte, error)
  func (b *Reader) Reset(r io.Reader)
  func (b *Reader) Buffered() int // сколько байт в буфере
  func (b *Reader) Peek(n int) ([]byte, error) // заглянуть в буфер без чтения

2. BUFIO.SCANNER — ПОСТРОЧНОЕ ЧТЕНИЕ (САМЫЙ ПОПУЛЯРНЫЙ)

БАЗОВОЕ ИСПОЛЬЗОВАНИЕ:
  scanner := bufio.NewScanner(reader)
  for scanner.Scan() {
      line := scanner.Text()
      // обрабатываем line
  }
  if err := scanner.Err(); err != nil {
      return err
  }

⚠️ ВАЖНЫЕ ОГРАНИЧЕНИЯ:
  1. ПО УМОЛЧАНИЮ БУФЕР 64KB!
     Если строка длиннее → ошибка "bufio.Scanner: token too long"
     РЕШЕНИЕ: scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB

  2. Text() ВОЗВРАЩАЕТ КОПИЮ СТРОКИ.
     Для 1 млн строк создаст 1 млн копий в памяти.

  3. Scan() ВОЗВРАЩАЕТ false ПРИ EOF ИЛИ ОШИБКЕ.
     Всегда проверяй scanner.Err() после цикла.

МЕТОДЫ SCANNER:
  func (s *Scanner) Scan() bool
  func (s *Scanner) Text() string
  func (s *Scanner) Bytes() []byte
  func (s *Scanner) Err() error
  func (s *Scanner) Buffer(buf []byte, max int)
  func (s *Scanner) Split(split SplitFunc)

КАСТОМНЫЙ SPLIT:
  scanner.Split(bufio.ScanWords) // разбить по словам
  scanner.Split(bufio.ScanRunes) // разбить по рунам
  scanner.Split(bufio.ScanLines) // разбить по строкам (по умолчанию)
  scanner.Split(bufio.ScanBytes) // разбить по байтам

3. BUFIO.WRITER — БУФЕРИЗИРОВАННАЯ ЗАПИСЬ

СОЗДАНИЕ:
  writer := bufio.NewWriter(w)
  writer := bufio.NewWriterSize(w, 1024*1024) // 1MB буфер

МЕТОДЫ:
  func (b *Writer) Write(p []byte) (n int, err error)
  func (b *Writer) WriteString(s string) (n int, err error)
  func (b *Writer) WriteByte(c byte) error
  func (b *Writer) WriteRune(r rune) (size int, err error)
  func (b *Writer) Flush() error // сбросить буфер на диск

⚠️ ВАЖНО:
  • Всегда вызывай Flush() в конце (через defer)!
  • Без Flush() данные могут потеряться.
  • Буфер сбрасывается автоматически при заполнении.

ТЕОРИЯ: ПАКЕТ OS

1. ОСНОВНЫЕ ТИПЫ

type File struct { ... }           // открытый файл
type FileInfo interface { ... }   // информация о файле
type DirEntry interface { ... }   // запись в директории (Go 1.16+)

ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ:
  var Stdin  *File // стандартный ввод (io.Reader)
  var Stdout *File // стандартный вывод (io.Writer)
  var Stderr *File // стандартный вывод ошибок (io.Writer)

2. ОТКРЫТИЕ И СОЗДАНИЕ ФАЙЛОВ

func Open(name string) (*File, error)
  • Открывает файл для чтения (O_RDONLY).
  • Если файла нет — ошибка.

func Create(name string) (*File, error)
  • Создаёт файл (O_RDWR|O_CREATE|O_TRUNC).
  • Если файл существует — перезаписывает.

func OpenFile(name string, flag int, perm FileMode) (*File, error)
  • Гибкое открытие с флагами и правами.

ФЛАГИ (комбинируются через |):
  os.O_RDONLY   // только чтение
  os.O_WRONLY   // только запись
  os.O_RDWR     // чтение и запись
  os.O_CREATE   // создать если не существует
  os.O_APPEND   // дописывать в конец
  os.O_TRUNC    // очистить при открытии
  os.O_EXCL     // с O_CREATE — ошибка если файл существует
  os.O_SYNC     // синхронная запись

ПРАВА ДОСТУПА (восьмеричные):
  0644  // -rw-r--r-- (владелец读写, остальные读)
  0755  // -rwxr-xr-x (владелец все, остальные读+执行)
  0600  // -rw------- (только владелец)
  0444  // -r--r--r-- (только чтение)

3. ОСНОВНЫЕ ФУНКЦИИ РАБОТЫ С ФАЙЛАМИ

func ReadFile(name string) ([]byte, error) // Go 1.16+
  • Читает весь файл в память.
  • ⚠️ ОПАСНО: загружает всё в память! Только для маленьких файлов.

func WriteFile(name string, data []byte, perm FileMode) error // Go 1.16+
  • Записывает данные в файл.
  • Если файл существует — перезаписывает.

func Stat(name string) (FileInfo, error)
  • Возвращает информацию о файле.
  • Для символических ссылок — переходит по ссылке.

func Lstat(name string) (FileInfo, error)
  • Возвращает информацию о файле (НЕ переходит по ссылкам).

func Remove(name string) error
  • Удаляет файл или пустую директорию.

func RemoveAll(path string) error
  • Удаляет файл или директорию рекурсивно (аналог rm -rf).

4. РАБОТА С ДИРЕКТОРИЯМИ

func ReadDir(name string) ([]DirEntry, error) // Go 1.16+
  • Читает содержимое директории.

func Mkdir(name string, perm FileMode) error
  • Создаёт одну директорию.

func MkdirAll(path string, perm FileMode) error
  • Создаёт директорию рекурсивно (аналог mkdir -p).

func Getwd() (dir string, err error)
  • Возвращает текущую рабочую директорию.

func Chdir(dir string) error
  • Меняет текущую рабочую директорию.

func TempDir() string
  • Возвращает системную временную директорию (/tmp, %TEMP%).

func MkdirTemp(dir, pattern string) (string, error) // Go 1.16+
  • Создаёт уникальную временную директорию.

func CreateTemp(dir, pattern string) (*File, error) // Go 1.16+
  • Создаёт уникальный временный файл.

5. ИНФОРМАЦИЯ О ФАЙЛЕ (FILEINFO)

МЕТОДЫ FILEINFO:
  Name() string       // имя файла
  Size() int64        // размер в байтах
  Mode() FileMode     // права доступа
  ModTime() time.Time // время модификации
  IsDir() bool        // является ли директорией
  Sys() interface{}   // системная информация

6. ПЕРЕМЕННЫЕ ОКРУЖЕНИЯ

func Getenv(key string) string
  • Возвращает значение или пустую строку.

func LookupEnv(key string) (string, bool)
  • Возвращает значение и флаг существования (предпочтительный способ).

func Setenv(key, value string) error
  • Устанавливает переменную окружения.

func Unsetenv(key string) error
  • Удаляет переменную окружения.

func Environ() []string
  • Возвращает все переменные в формате KEY=value.

7. ОШИБКИ И ИХ ПРОВЕРКА

func IsNotExist(err error) bool
  • Проверяет, вызвана ли ошибка отсутствием файла.

func IsExist(err error) bool
  • Проверяет, вызвана ли ошибка существованием файла.

func IsPermission(err error) bool
  • Проверяет, вызвана ли ошибка недостатком прав.

func IsTimeout(err error) bool
  • Проверяет, вызвана ли ошибка таймаутом.

8. МЕТОДЫ *FILE (САМЫЕ ВАЖНЫЕ)

func (f *File) Read(b []byte) (n int, err error)      // io.Reader
func (f *File) Write(b []byte) (n int, err error)     // io.Writer
func (f *File) Close() error                          // io.Closer
func (f *File) Seek(offset int64, whence int) (int64, error) // io.Seeker
func (f *File) Stat() (FileInfo, error)
func (f *File) Sync() error // сброс на диск (fsync)
func (f *File) Truncate(size int64) error
func (f *File) Chmod(mode FileMode) error
func (f *File) Chdir() error

9. ТИПИЧНЫЕ ОШИБКИ И ИХ РЕШЕНИЯ

ОШИБКА 1: "too many open files"
  Причина: забыли закрыть файл
  Решение: всегда defer file.Close()

ОШИБКА 2: "bufio.Scanner: token too long"
  Причина: строка > 64KB
  Решение: scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

ОШИБКА 3: "no such file or directory"
  Причина: файл не существует
  Решение: проверить существование через os.IsNotExist(err)

ОШИБКА 4: "permission denied"
  Причина: недостаточно прав
  Решение: проверить права доступа, запустить от администратора

ОШИБКА 5: "cannot delete file" (Windows)
  Причина: файл открыт и не закрыт
  Решение: defer file.Close() перед удалением

ОШИБКА 6: "memory exhausted" при чтении
  Причина: os.ReadFile для большого файла
  Решение: использовать bufio.Scanner или io.Copy

*/

// 1. СКАЧИВАНИЕ ФАЙЛА ПО HTTP
func primer1(url, dest string) error {
	// 1. HTTP запрос
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// 2. Создаём файл
	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	// 3. Копируем данные
	written, err := io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	log.Printf("Downloaded %d bytes to %s", written, dest)
	return nil
}

// 2. ЧТЕНИЕ БОЛЬШОГО CSV
func readCSV(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Увеличиваем буфер для строк > 64KB (продакшен-вариант)
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, 0, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Парсим CSV
		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			log.Printf("Line %d: skip (too few fields)", lineNum)
			continue
		}

		// Обрабатываем данные
		log.Printf("Line %d: %s, %s", lineNum, fields[0], fields[1])
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	log.Printf("Total lines: %d", lineNum)
	return nil
}

// 3. ЛОГГЕР С РОТАЦИЕЙ
type logger struct {
	file     *os.File
	mu       sync.Mutex
	maxSize  int64
	filename string
}

func newLogger(filename string, maxSizeMb int) (*logger, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 6444)
	if err != nil {
		return nil, err
	}
	return &logger{
		file:     file,
		maxSize:  int64(maxSizeMb) * 1024 * 1024,
		filename: filename,
	}, nil
}

func (l *logger) Write(data []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	info, err := l.file.Stat()
	if err != nil {
		return 0, err
	}

	// Ротация
	if info.Size() > l.maxSize {
		l.file.Close()
		os.Rename(l.filename, l.filename+"old")
		l.file, err = os.Create(l.filename)
		if err != nil {
			return 0, err
		}
	}
	return l.file.Write(data)
}

func (l *logger) Close() error {
	return l.file.Close()
}

// 4. КОПИРОВАНИЕ ФАЙЛА
func primer4(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	return nil
}

// 5. КОПИРОВАНИЕ ДИРЕКТОРИИ
func primer5(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Относительный путь
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Путь назначения
		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		// Копируем файл
		return copyFile(path, dstPath)
	})
}

// 6. GREP
func primer6(filename, pattern string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(line, pattern) {
			fmt.Printf("%d: %s\n", lineNum, line)
		}
	}
	return scanner.Err()
}

// 7. WC -L
func primer7(filename string) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	count := 0
	for {
		_, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		count++
	}
	return count, nil
}

// 8. READJSON
func readJSON(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	dec := json.NewDecoder(file)

	// Читаем '['
	if _, err := dec.Token(); err != nil {
		return err
	}

	for dec.More() {
		var data map[string]interface{}
		if err := dec.Decode(&data); err != nil {
			return err
		}
		// Обрабатываем объект
		fmt.Printf("Object: %+v\n", data)
	}

	// Читаем ']'
	if _, err := dec.Token(); err != nil {
		return err
	}

	return nil
}

// 9. ОБХОД ДИРЕКТОРИИ И ПОДСЧЁТ ФАЙЛОВ
func primer9(dir string) (int, error) {
	count := 0
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

// 10. ТАЙМАУТ НА ЧТЕНИЕ ИЗ ФАЙЛА
func primer10(filename string, timeout time.Duration) ([]byte, error) {
	done := make(chan struct{})
	var data []byte
	var err error

	go func() {
		data, err = os.ReadFile(filename)
		close(done)
	}()

	select {
	case <-done:
		return data, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("read timeout after %v", timeout)
	}
}
