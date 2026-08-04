package cacheaside

import (
	"context"
	"errors"
	"fmt"
	"internal/singleflight"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 7.1. КЭШИРОВАНИЕ (CACHE-ASIDE)
0. ВВЕДЕНИЕ: ЗАЧЕМ НУЖНО КЭШИРОВАНИЕ?

Кэширование — это один из самых эффективных способов повышения производительности
и снижения нагрузки на основное хранилище данных (базу данных). Redis, будучи
высокоскоростным in-memory хранилищем, идеально подходит для кэширования.

Однако просто добавить Redis в архитектуру недостаточно — важно правильно выбрать
стратегию кэширования. Cache-Aside (или Lazy Loading) — это наиболее популярный
и гибкий паттерн, который подходит для большинства сценариев.

1. ЧТО ТАКОЕ CACHE-ASIDE? (ПОДРОБНО)

1.1. Определение
Cache-Aside — это паттерн, при котором приложение самостоятельно управляет кэшем:
оно сначала проверяет наличие данных в кэше, и только при отсутствии (cache miss)
обращается к основному хранилищу, а затем помещает полученные данные в кэш.

Этот паттерн называют также «Lazy Loading», потому что данные загружаются в кэш
только в момент первого обращения, а не заранее.

1.2. Схема работы (пошагово)
1. Клиент отправляет запрос на получение данных.
2. Приложение вычисляет ключ для доступа к кэшу (обычно на основе идентификатора).
3. Приложение выполняет GET по ключу в Redis.
   - Если ключ существует (cache hit):
     a) Десериализует данные (если они хранятся в сериализованном виде).
     b) Возвращает данные клиенту.
   - Если ключ отсутствует (cache miss):
     a) Приложение обращается к основной БД (SELECT).
     b) Получает данные из БД.
     c) Сериализует данные (если необходимо).
     d) Сохраняет их в Redis с заданным TTL (SET ... EX).
     e) Возвращает данные клиенту.

1.3. Преимущества Cache-Aside
- Простота реализации — не требует сложной инфраструктуры.
- Экономия памяти — кэшируются только те данные, которые реально запрашиваются.
- Устойчивость к сбоям — если Redis недоступен, приложение продолжает работать через БД.
- Гибкость — можно легко изменять TTL, политики инвалидации и стратегии сериализации.
- Подходит для read-heavy приложений, где данные редко меняются.

1.4. Недостатки и ограничения
- Первый запрос всегда медленный (cache miss) — это приводит к дополнительной задержке.
- Возможна несогласованность данных (stale data) между кэшем и БД.
- При высокой нагрузке множество одновременных промахов могут создать «эффект стаи»
  (thundering herd), когда сотни запросов одновременно обращаются к БД.
- Требует продуманной стратегии инвалидации, чтобы избежать устаревших данных.
- Дополнительная сложность в поддержке согласованности при обновлениях.

2. РАЗНОВИДНОСТИ CACHE-ASIDE

2.1. Стандартный (синхронный) Cache-Aside
   - Самый простой вариант: GET → при промахе SELECT → SET.
   - Весь процесс выполняется синхронно в одном потоке.
   - Подходит для большинства случаев, когда нагрузка не слишком высокая.

2.2. Cache-Aside с защитой от thundering herd (singleflight / mutex)
   - При множестве одновременных запросов к одному ключу только один выполняет
     загрузку из БД, остальные ждут его результат.
   - Реализуется через sync.Once, singleflight, или распределённую блокировку.
   - Это предотвращает избыточные запросы к БД и снижает нагрузку на неё.

2.3. Асинхронный Cache-Aside (write-behind)
   - При промахе данные загружаются из БД и сразу возвращаются клиенту,
     а сохранение в кэш выполняется асинхронно в фоновом потоке.
   - Это уменьшает время ответа на первый запрос, но может привести к тому,
     что следующий запрос снова обратится к БД, если асинхронная запись не завершилась.
   - Также требует механизма повторных попыток для гарантированной записи.

2.4. Refresh-ahead (предварительное обновление)
   - При обращении к ключу, у которого TTL истекает (например, через 80% от TTL),
     приложение инициирует асинхронное обновление данных из БД в фоне,
     чтобы поддерживать кэш актуальным.
   - Снижает вероятность cache miss для часто запрашиваемых ключей.

3. СТРАТЕГИИ ИНВАЛИДАЦИИ КЭША

Инвалидация — это процесс удаления или обновления устаревших данных в кэше.
Правильная инвалидация критически важна для поддержания согласованности данных.

3.1. Инвалидация при обновлении (invalidate on write)
   - При обновлении данных в БД приложение удаляет соответствующий ключ из кэша.
   - Следующий запрос загрузит свежие данные из БД и снова поместит их в кэш.
   - Преимущество: простота, гарантирует актуальность данных.
   - Недостаток: дополнительный запрос к БД при следующем обращении.

3.2. Обновление кэша при записи (write-through)
   - При обновлении данных в БД приложение также обновляет кэш (SET).
   - Кэш всегда содержит актуальные данные (сразу после обновления).
   - Преимущество: последующие запросы получают свежие данные из кэша.
   - Недостаток: дополнительная задержка на запись (два вызова: БД + Redis).

3.3. TTL-based инвалидация (автоматическое истечение)
   - Данные автоматически удаляются из кэша по истечении заданного TTL.
   - Не требует явной инвалидации, упрощает код.
   - Недостаток: данные могут быть устаревшими, пока не истекут TTL.
   - Часто комбинируется с активной инвалидацией для критичных данных.

3.4. Версионирование данных (versioning)
   - В кэше хранятся данные вместе с версией (или timestamp).
   - При обновлении данных в БД версия увеличивается.
   - При чтении из кэша проверяется версия: если она устарела, данные перезагружаются.
   - Позволяет обновлять кэш без удаления, но требует дополнительных проверок.

4. ОБРАБОТКА ОШИБОК И FALLBACK СТРАТЕГИИ

4.1. Ошибки Redis
   - Если Redis недоступен (сетевой сбой, таймаут, падение), приложение должно
     переключаться на БД (fallback).
   - Важно не допустить каскадных отказов: если Redis не отвечает, не следует
     долго ждать — используйте таймауты (ReadTimeout, WriteTimeout).
   - Логируйте ошибки для диагностики.

4.2. Ошибки БД
   - Если загрузка из БД завершилась ошибкой, не кэшируйте ошибку.
   - Верните ошибку клиенту, возможно с fallback-значением (если допустимо).

4.3. Таймауты и retry
   - Установите разумные таймауты для операций с Redis и БД.
   - Для временных сбоев используйте механизм повторов (retry) с экспоненциальной задержкой.
   - Но будьте осторожны: бесконечные retry могут усугубить проблему.

4.4. Circuit breaker
   - При множественных ошибках Redis (или БД) можно использовать паттерн Circuit Breaker,
     чтобы временно отключать проблемный компонент и выполнять fallback.
   - Реализуется через библиотеки (например, go-resiliency, hystrix-go).

5. МЕТРИКИ И МОНИТОРИНГ КЭША

Для оценки эффективности кэширования важно собирать и анализировать метрики:

5.1. Основные метрики
   - Hit ratio: процент успешных обращений к кэшу (hits / (hits + misses)).
   - Miss ratio: процент промахов.
   - Время ответа (latency): среднее время для hit и miss.
   - Количество ошибок (error rate): ошибки Redis и БД.
   - TTL истечение: сколько ключей истекло по TTL.

5.2. Анализ метрик
   - Низкий hit ratio (< 80%) указывает на неэффективное кэширование.
   - Высокая задержка при miss может быть вызвана медленной БД или сетью.
   - Рост error rate требует немедленного вмешательства.

5.3. Инструменты мониторинга
   - Prometheus + Grafana для сбора и визуализации метрик.
   - Встроенные счетчики в приложении (с экспортом в мониторинг).
   - Логирование cache miss для выявления «горячих» ключей.

6. СРАВНЕНИЕ С ДРУГИМИ ПАТТЕРНАМИ КЭШИРОВАНИЯ

6.1. Read-Through
   - Приложение обращается к абстракции кэша (прокси), которая сама загружает данные
     из БД при промахе.
   - В отличие от Cache-Aside, логика загрузки инкапсулирована в кэш-прокси,
     что упрощает клиентский код.
   - Недостаток: прокси становится единой точкой отказа.

6.2. Write-Through
   - При записи данные обновляются одновременно в БД и в кэше.
   - Обеспечивает согласованность, но увеличивает задержку записи.
   - Часто используется вместе с Read-Through.

6.3. Write-Behind (Write-Back)
   - При записи данные обновляются только в кэше, а затем асинхронно записываются в БД.
   - Очень высокая производительность записи, но риск потери данных при сбое.
   - Требует механизма репликации и восстановления.

6.4. Refresh-Ahead
   - Данные обновляются в фоне до истечения TTL, чтобы всегда быть актуальными.
   - Снижает задержки для часто запрашиваемых ключей.

7. ПРОДВИНУТЫЕ ТЕХНИКИ ДЛЯ CACHE-ASIDE

7.1. Использование singleflight для защиты от thundering herd
   - Пакет golang.org/x/sync/singleflight гарантирует, что для одного ключа
     одновременно выполняется только один запрос к БД.
   - Все остальные запросы ждут результат первого.
   - Это эффективно предотвращает «эффект стаи» при кэш-промахе.

7.2. Распределённая блокировка при обновлении
   - Если несколько экземпляров приложения одновременно пытаются обновить один
     и тот же ключ, можно использовать Redis-блокировку (SetNX) для синхронизации.
   - Это предотвращает дублирующие запросы к БД и гарантирует, что обновление
     выполняется только одним инстансом.

7.3. Версионирование данных
   - Хранение версии (или timestamp) вместе с данными позволяет обнаруживать
     устаревшие данные и обновлять их без удаления ключа.
   - Например, при обновлении данных в БД версия инкрементируется, и при чтении
     сравнивается с версией в кэше.

7.4. Асинхронное обновление (refresh-ahead)
   - При обращении к ключу, у которого TTL истекает, можно инициировать фоновое
     обновление данных, не заставляя клиента ждать.
   - Это особенно полезно для данных с высокой частотой запросов.

7.5. Сериализация и сжатие
   - Для уменьшения размера данных в кэше используйте эффективные форматы сериализации
     (Protobuf, MessagePack) и сжатие (gzip, zstd).
   - Это снижает потребление памяти и сетевой трафик.

7.6. Ключи и префиксы
   - Используйте единый формат ключей (например, "service:entity:id").
   - Префиксы помогают группировать ключи и облегчают инвалидацию.

8. BEST PRACTICES ДЛЯ ПРОДАКШНА

1. Устанавливайте TTL с запасом: данные не должны истекать слишком часто,
   но и не должны храниться слишком долго (риск устаревания).
   - Для статичных данных (справочники) — 1–24 часа.
   - Для часто меняющихся — 1–5 минут.
   - Используйте разные TTL для разных типов данных.

2. Используйте singleflight или mutex для защиты от thundering herd.
3. Настройте таймауты (ReadTimeout, WriteTimeout) для Redis и БД.
4. Логируйте cache miss для анализа (но не слишком много, чтобы не забивать логи).
5. Инвалидируйте кэш при обновлении данных (delete ключа).
6. Для критичных данных используйте версионирование или write-through.
7. Мониторьте hit ratio и алертьте при его падении ниже 80–90%.
8. При падении Redis используйте fallback на БД.
9. Используйте контекст с таймаутом для отмены долгих операций.
10. Рассмотрите использование Redis Cluster для масштабирования кэша.

9. СВЯЗЬ С GO (go-redis)

В Go паттерн Cache-Aside реализуется в слое репозитория или сервиса.
Примерная структура:

    func (r *UserRepository) GetUser(ctx context.Context, id int) (*User, error) {
        key := fmt.Sprintf("user:%d", id)
        // 1. Проверяем кэш
        cached, err := r.redis.Get(ctx, key).Result()
        if err == nil {
            var user User
            if err := json.Unmarshal([]byte(cached), &user); err == nil {
                return &user, nil
            }
        }
        if !errors.Is(err, redis.Nil) {
            // Логируем ошибку Redis, но не прерываем
            log.Printf("Redis error: %v", err)
        }
        // 2. Загружаем из БД
        user, err := r.db.GetUser(ctx, id)
        if err != nil {
            return nil, err
        }
        // 3. Сохраняем в кэш (с TTL)
        data, _ := json.Marshal(user)
        go func() {
            // асинхронно, чтобы не задерживать ответ
            ctxBg := context.Background()
            r.redis.Set(ctxBg, key, data, r.ttl).Err()
        }()
        return user, nil
    }

Важно:
- Используйте контекст с таймаутом (context.WithTimeout) для ограничения времени.
- Обрабатывайте redis.Nil как cache miss, а не как ошибку.
- При ошибке Redis не прерывайте выполнение — переходите к БД.
- Рассмотрите использование singleflight для защиты от thundering herd.

10. ТИПИЧНЫЕ ОШИБКИ И КАК ИХ ИЗБЕЖАТЬ

"Кэширую всё подряд без TTL"
 Память переполнится. Устанавливайте TTL для каждого ключа.

"Игнорирую ошибки Redis, но не делаю fallback"
 Приложение упадёт. Всегда предусматривайте fallback на БД.

"Кэширую ошибки (например, nil вместо данных)"
 Это может привести к тому, что клиенты будут получать ошибки, даже когда данные есть.
 Кэшируйте только успешные результаты.

"Не инвалидирую кэш при обновлении"
 Данные в кэше устаревают, пользователи видят старую информацию.

"Использую слишком большой TTL"
 Данные устаревают, но долго живут в кэше, занимая память.

"Не мониторю hit ratio"
 Не понимаю, эффективно ли кэширование.

11. ИТОГИ

Cache-Aside — это фундаментальный паттерн кэширования, который сочетает простоту
реализации, гибкость и высокую эффективность. Он подходит для большинства
read-heavy приложений и является де-факто стандартом при работе с Redis.

Правильная реализация Cache-Aside требует учёта множества факторов: TTL, инвалидация,
защита от thundering herd, fallback при ошибках, мониторинг и метрики.
Сочетание этих техник позволяет достичь высокой производительности и надёжности.

В следующем разделе мы рассмотрим код продакшен-примеров, реализующих все эти аспекты.
*/

var (
	rdb    *redis.Client
	ctx    = context.Background()
	logger = log.Default()
	db     = &sync.Map{} // имитация БД
	sf     singleflight.Group
)

func init() {
	rdb = redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatalf("Redis не отвечает: %v", err)
	}
	// Заполняем БД
	for i := 1; i <= 100; i++ {
		db.Store(i, fmt.Sprintf(`{"id":%d,"name":"user_%d"}`, i, i))
	}
}

func main() {
	fmt.Println("=== CACHE-ASIDE===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. БАЗОВЫЙ CACHE-ASIDE (GET → MISS → DB → SET)
func primer1() {
	fmt.Println("--- 1. Базовый Cache-Aside ---")

	userID := 1
	key := fmt.Sprintf("user:%d", userID)

	cached, err := rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		logger.Printf("Cache miss для %s", key)
		// Загружаем из БД
		val, ok := db.Load(userID)
		if !ok {
			fmt.Println("Пользователь не найден")
			return
		}
		data := val.(string)
		// Сохраняем в кэш
		rdb.Set(ctx, key, data, 5*time.Minute)
		fmt.Printf("Загружено из БД: %s\n", data)
	} else if err != nil {
		logger.Printf("Ошибка Redis: %v", err)
		// Fallback на БД
		val, _ := db.Load(userID)
		fmt.Printf("Fallback из БД: %s\n", val.(string))
	} else {
		fmt.Printf("Cache hit: %s\n", cached)
	}
	rdb.Del(ctx, key)
}

// 2. ЗАЩИТА ОТ THUNDERING HERD (SINGLEFLIGHT)
func primer2() {
	fmt.Println("\n--- 2. Cache-Aside с singleflight ---")

	getUser := func(id int) (string, error) {
		key := fmt.Sprintf("userID:%d", id)

		// Проверяем кэш
		cahced, err := rdb.Get(ctx, key).Result()
		if err == nil {
			return cahced, nil
		}
		if !errors.Is(err, redis.Nil) {
			logger.Printf("Redis error: %v", err)
		}

		// singleflight
		result, err, _ := sf.Do(key, func() (any, error) {
			logger.Printf("Загрузка из БД для %d (singleflight)", id)
			time.Sleep(100 * time.Millisecond) // имитация запроса
			val, ok := db.Load(key)
			if !ok {
				return "", fmt.Errorf("user %d not found", id)
			}
			data := val.(string)
			// Сохраняем в кэш (синхронно, т.к. мы уже внутри singleflight)
			rdb.Set(ctx, key, data, 5*time.Minute)
			return data, nil
		})
		if err != nil {
			return "", err
		}
		return result.(string), nil
	}

	// 10 горутин одновременно запрашивают одного пользователя
	id := 101
	db.Store(id, `{"id":101,"name":"singleflight_user"}`)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := getUser(id)
			if err != nil {
				logger.Printf("Ошибка: %v", err)
			} else {
				fmt.Printf("Получено: %s\n", data)
			}
		}()
	}
	wg.Wait()
	rdb.Del(ctx, fmt.Sprintf("user:%d", id))
	db.Delete(id)
}

// 3. АСИНХРОННАЯ ЗАПИСЬ В КЭШ (WRITE-BEHIND)
func primer3() {
	fmt.Println("\n--- 3. Асинхронная запись (write-behind) ---")

	type CacheJob struct {
		Key   string
		Value string
		TTL   time.Duration
	}
	jobs := make(chan CacheJob, 100)

	// Воркер для асинхронной записи
	go func() {
		for job := range jobs {
			ctxBg := context.Background()
			err := rdb.Set(ctxBg, job.Key, job.Value, job.TTL).Err()
			if err != nil {
				logger.Printf("Ошибка асинхронной записи: %v", err)
			}
		}
	}()

	getUser := func(id int) (string, error) {
		key := fmt.Sprintf("user:%d", id)
		cached, err := rdb.Get(ctx, key).Result()
		if err == nil {
			return cached, nil
		}
		if !errors.Is(err, redis.Nil) {
			logger.Printf("Redis error: %v", err)
		}
		// Загружаем из БД
		val, ok := db.Load(id)
		if !ok {
			return "", fmt.Errorf("user %d not found", id)
		}
		data := val.(string)
		// Отправляем в очередь асинхронной записи (не ждём)
		jobs <- CacheJob{Key: key, Value: data, TTL: 5 * time.Minute}
		return data, nil
	}

	id := 202
	db.Store(id, `{"id":202,"name":"async_user"}`)
	data, _ := getUser(id)
	fmt.Printf("Получено из БД: %s (кэш будет обновлён асинхронно)\n", data)
	time.Sleep(200 * time.Millisecond) // даём время воркеру
	cached, _ := rdb.Get(ctx, fmt.Sprintf("user:%d", id)).Result()
	fmt.Printf("Кэш после асинхронной записи: %s\n", cached)
	rdb.Del(ctx, fmt.Sprintf("user:%d", id))
	db.Delete(id)
	close(jobs)
}

// 4. ИНВАЛИДАЦИЯ КЭША ПРИ ОБНОВЛЕНИИ
func primer4() {
	fmt.Println("\n--- 4. Инвалидация при обновлении данных ---")

	updateUser := func(id int, newName string) error {
		// Обновляем в БД
		db.Store(id, fmt.Sprintf(`{"id":%d,"name":"%s"}`, id, newName))
		// Удаляем ключ из кэша
		key := fmt.Sprintf("user:%d", id)
		err := rdb.Del(ctx, key).Err()
		if err != nil {
			logger.Printf("Ошибка инвалидации: %v", err)
		}
		return nil
	}

	getUser := func(id int) (string, error) {
		key := fmt.Sprintf("user:%d", id)
		cached, err := rdb.Get(ctx, key).Result()
		if err == nil {
			return cached, nil
		}
		if !errors.Is(err, redis.Nil) {
			return "", err
		}
		val, ok := db.Load(id)
		if !ok {
			return "", fmt.Errorf("user %d not found", id)
		}
		data := val.(string)
		rdb.Set(ctx, key, data, 5*time.Minute)
		return data, nil
	}

	id := 303
	db.Store(id, `{"id":303,"name":"old_name"}`)
	// Заполняем кэш
	getUser(id)
	// Обновляем (кэш будет удалён)
	updateUser(id, "new_name")
	// Следующий запрос загрузит свежие данные
	data, _ := getUser(id)
	fmt.Printf("После обновления: %s\n", data)
	rdb.Del(ctx, fmt.Sprintf("user:%d", id))
	db.Delete(id)
}

// 5. ВЕРСИОНИРОВАННЫЙ КЭШ (ОБНОВЛЕНИЕ БЕЗ УДАЛЕНИЯ)
func primer5() {
	fmt.Println("\n--- 5. Версионированный кэш ---")

	type VersionedData struct {
		Version int
		Data    string
	}

	// Храним версию в отдельном ключе или в JSON (здесь упрощённо)
	versionMap := &sync.Map{}

	getUser := func(id int) (string, error) {
		key := fmt.Sprintf("user:%d", id)
		verKey := key + ":ver"

		// Читаем данные и версию из кэша
		cached, err := rdb.Get(ctx, key).Result()
		if err == nil {
			ver, _ := rdb.Get(ctx, verKey).Int64()
			dbVer, _ := versionMap.Load(id)
			if dbVer != nil && dbVer.(int) == int(ver) {
				return cached, nil
			}
		}
		// Загружаем из БД
		val, ok := db.Load(id)
		if !ok {
			return "", fmt.Errorf("user %d not found", id)
		}
		data := val.(string)
		// Увеличиваем версию
		newVer := 1
		v, ok := versionMap.Load(id)
		if ok {
			newVer = v.(int) + 1
		}
		versionMap.Store(key, newVer)
		// Сохраняем в кэш
		rdb.Set(ctx, key, data, 5*time.Minute)
		rdb.Set(ctx, verKey, newVer, 5*time.Minute)
		return data, nil
	}
	id := 404
	db.Store(id, `{"id":404,"name":"v1_data"}`)
	versionMap.Store(id, 1)

	// Первый запрос
	data, _ := getUser(id)
	fmt.Printf("Первая загрузка: %s\n", data)

	// Обновляем версию в БД (имитация изменения данных)
	db.Store(id, `{"id":404,"name":"v2_data"}`)
	versionMap.Store(id, 2)

	// Второй запрос — увидит старую версию в кэше, но перезагрузит, т.к. версия изменилась
	data, _ = getUser(id)
	fmt.Printf("После обновления версии: %s\n", data)

	rdb.Del(ctx, fmt.Sprintf("user:%d", id), fmt.Sprintf("user:%d:ver", id))
	db.Delete(id)
	versionMap.Delete(id)
}

// 6. ПРЕДВАРИТЕЛЬНАЯ ЗАГРУЗКА (WARM-UP)
func primer6() {
	fmt.Println("\n--- 6. Предварительная загрузка (warm-up) ---")

	// При старте приложения загружаем горячие ключи
	warmupKeys := []string{"config", "feature_flags", "popular_items"}
	for _, key := range warmupKeys {
		val, ok := db.Load(key)
		if ok {
			rdb.Set(ctx, key, val.(string), 10*time.Minute)
		}
	}
	fmt.Println("Warmup завершён")

	// Проверяем
	for _, k := range warmupKeys {
		val, _ := rdb.Get(ctx, k).Result()
		fmt.Printf("%s: %s\n", k, val)
	}
	rdb.Del(ctx, warmupKeys...)
}

// 7. FALLBACK ПРИ ОШИБКЕ REDIS
func primer7() {
	fmt.Println("\n--- 7. Fallback на БД при ошибке Redis ---")

	// Функция проверки доступности Redis
	isRedisAvailable := func() bool {
		ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			logger.Printf("Redis недоступен: %v", err)
			return false
		}
		return true
	}

	getUser := func(id int) (string, error) {
		key := fmt.Sprintf("user:%d", id)

		// 1. Пытаемся получить из Redis
		if isRedisAvailable() {
			cached, err := rdb.Get(ctx, key).Result()
			if err == nil {
				return cached, nil
			}
			if !errors.Is(err, redis.Nil) {
				logger.Printf("Ошибка Redis, переключаемся на БД: %v", err)
				// Здесь явный fallback
				val, ok := db.Load(id)
				if !ok {
					return "", fmt.Errorf("user %d not found", id)
				}
				return val.(string), nil
			}
		}

		// 2. Redis недоступен или кэш-промах — идём в БД
		logger.Printf("Redis недоступен или cache miss, используем БД")
		val, ok := db.Load(id)
		if !ok {
			return "", fmt.Errorf("user %d not found", id)
		}
		data := val.(string)

		// 3. Если Redis стал доступен, пробуем записать (но не блокируем)
		if isRedisAvailable() {
			go func() {
				ctxBg := context.Background()
				_ = rdb.Set(ctxBg, key, data, 5*time.Minute).Err()
			}()
		}
		return data, nil
	}

	id := 505
	db.Store(id, `{"id":505,"name":"fallback_user"}`)

	// Имитация недоступности Redis (устанавливаем очень короткий таймаут)
	// В реальности это может быть сетевой сбой
	rdb.Options().ReadTimeout = 1 * time.Millisecond

	data, err := getUser(id)
	if err != nil {
		logger.Printf("Ошибка: %v", err)
	} else {
		fmt.Printf("Получено через fallback: %s\n", data)
	}

	// Восстанавливаем нормальный таймаут
	rdb.Options().ReadTimeout = 2 * time.Second
	rdb.Del(ctx, fmt.Sprintf("user:%d", id))
	db.Delete(id)
}

// 8. МЕТРИКИ КЭША (HIT/MISS RATIO)
func primer8() {
	fmt.Println("\n--- 8. Сбор метрик кэша ---")

	type Metrics struct {
		mu     sync.Mutex
		hits   int
		misses int
		errors int
	}
	m := &Metrics{}

	// Обёртка для кэширования с метриками
	getUser := func(id int) (string, error) {
		key := fmt.Sprintf("user:%d", id)
		cached, err := rdb.Get(ctx, key).Result()
		if err == nil {
			m.mu.Lock()
			m.hits++
			m.mu.Unlock()
			return cached, nil
		}
		if errors.Is(err, redis.Nil) {
			m.mu.Lock()
			m.misses++
			m.mu.Unlock()
			// Загружаем из БД
			val, ok := db.Load(id)
			if !ok {
				return "", fmt.Errorf("user %d not found", id)
			}
			data := val.(string)
			rdb.Set(ctx, key, data, 5*time.Minute)
			return data, nil
		}
		m.mu.Lock()
		m.errors++
		m.mu.Unlock()
		return "", err
	}

	// Имитация запросов
	for i := 0; i < 20; i++ {
		id := rand.Intn(100) + 1
		getUser(id)
	}
	fmt.Printf("Hits: %d, Misses: %d, Errors: %d\n", m.hits, m.misses, m.errors)
	fmt.Printf("Hit ratio: %.2f%%\n", float64(m.hits)/float64(m.hits+m.misses)*100)
	rdb.FlushDB(ctx)
}
