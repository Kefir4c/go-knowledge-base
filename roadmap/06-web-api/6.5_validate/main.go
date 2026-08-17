package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/locales/eu"
	"github.com/go-playground/locales/ru"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	ru_translations "github.com/go-playground/validator/v10/translations/ru"
)

/*
  УРОК 6.5 — ВАЛИДАЦИЯ ВХОДЯЩИХ ДАННЫХ В GO

  Валидация — это не просто «проверка данных», это краеугольный камень
  безопасности, надёжности и пользовательского опыта. Пропустив валидацию,
  вы рискуете получить SQL-инъекции, XSS, паники, мусор в БД, DoS-атаки
  и разгневанных пользователей.

  1. ИСТОРИЧЕСКИЙ КОНТЕКСТ: ПОЧЕМУ В СТАНДАРТНОЙ БИБЛИОТЕКЕ НЕТ ВАЛИДАЦИИ?
  Go изначально проектировался как минималистичный язык с упором на
  простоту и композируемость. В стандартной библиотеке есть только
  базовые инструменты: пакеты для работы с JSON, HTTP, но нет встроенного
  механизма валидации структур. Почему?

    • Философия Go: «предоставлять строительные блоки, а не готовые решения».
    • Валидация — это предметная область, зависящая от бизнес-логики,
      поэтому её выносят на уровень приложения.
    • Сообщество создало де-факто стандарт — go-playground/validator,
      который активно поддерживается и используется в таких фреймворках,
      как Gin, Echo, Fiber.

  Итог: вы НЕ должны писать валидацию вручную через if-ы — это ведёт
  к дублированию, ошибкам и трудно поддерживаемому коду.

  2. ВНУТРЕННЕЕ УСТРОЙСТВО GO-PLAYGROUND/VALIDATOR
  Библиотека использует рефлексию (reflect) для анализа тегов структур
  во время выполнения. Это позволяет ей быть гибкой и универсальной.

  Ключевые компоненты:
    • Validate — основной объект, содержащий кеш структур (для производительности).
    • Теги (struct tags) — описывают правила в виде строк, например
      `validate:"required,min=8,email"`.
    • Валидаторы — функции, которые проверяют конкретное правило.
      Встроенных около 50, плюс можно добавлять свои.

  Важно: библиотека кеширует информацию о структурах при первом вызове
  validate.Struct(), поэтому повторные вызовы работают быстро.
  Однако при использовании в высоконагруженных системах рекомендуется
  один раз создать экземпляр валидатора и использовать его как синглтон —
  мы так и делаем в примерах.

  3. ПОДРОБНЫЙ РАЗБОР ТЕГОВ (БОЛЕЕ 50 ВСТРОЕННЫХ ПРАВИЛ)
  Приведём только самые важные и часто используемые теги. Полный список
  есть в документации: https://pkg.go.dev/github.com/go-playground/validator/v10

  ┌─────────────────┬───────────────────────────────────────────────────────┐
  │ Тег             │ Описание                                              │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ required        │ Поле обязательно, не должно быть нулевым значением    │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ min, max        │ Для чисел: минимальное/максимальное значение.         │
  │                 │ Для строк: минимальная/максимальная длина.            │
  │                 │ Для слайсов/массивов: минимальное/макс. кол-во        │
  │                 │ Пример: `min=8, max=20`                               │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ len             │ Точная длина (для строк, слайсов, массивов, карт)     │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ eq, ne, gt, lt, │ Сравнение с числом или строкой:                       │
  │ gte, lte        │ eq=5 — равно 5, ne=5 — не равно 5, gt=0 — >0 и т.д.   │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ email           │ Проверка на корректный email (по RFC 5322)            │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ url             │ Проверка на корректный URL                            │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ uuid, uuid3,    │ Проверка на UUID разных версий                        │
  │ uuid4, uuid5    │                                                       │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ alphanum        │ Только буквы (латиница) и цифры                       │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ alpha           │ Только буквы (латиница)                               │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ numeric         │ Только цифры (число, но в виде строки)                │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ number          │ Число (с возможным знаком и десятичной точкой)        │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ oneof           │ Одно из перечисленных значений (через пробел)         │
  │                 │ `oneof=male female other`                             │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ eqfield         │ Поле должно быть равно другому полю (подтверждение)   │
  │ nefield         │ Поле не должно быть равно другому полю                │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ contains        │ Строка должна содержать подстроку                     │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ startsWith,     │ Строка должна начинаться/заканчиваться на подстроку   │
  │ endsWith        │                                                       │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ ip, ipv4, ipv6  │ Проверка IP-адреса (версий 4/6)                       │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ dns             │ Проверка DNS-имени (домена)                           │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ json            │ Строка должна быть валидным JSON                      │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ base64          │ Строка должна быть валидным Base64                    │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ datetime        │ Строка должна соответствовать формату даты/времени    │
  │                 │ `datetime=2006-01-02`                                 │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ unique          │ Элементы в слайсе/массиве должны быть уникальными     │
  ├─────────────────┼───────────────────────────────────────────────────────┤
  │ iscolor         │ Цвет в формате hex, rgb, hsl и т.д.                   │
  └─────────────────┴───────────────────────────────────────────────────────┘

  Комбинировать теги можно через запятую, порядок не важен.
  Если нужно исключить поле из валидации при нулевом значении —
  используется тег `omitempty` (тогда валидация применяется только если
  поле не пустое).

  4. КАСТОМНЫЕ ВАЛИДАТОРЫ — КОГДА И КАК ИХ ПИСАТЬ
  Встроенных тегов не всегда хватает. Типичные сценарии:

    • Пароль должен содержать хотя бы одну цифру, одну заглавную и одну
      строчную букву (business logic).
    • Проверка, что email не занят в БД (запрос к хранилищу).
    • Даты: не в прошлом, не в будущем, в пределах определённого возраста.
    • Зависимая валидация: если поле A == "something", то поле B обязательно.

  Регистрация кастомного валидатора:
    validate.RegisterValidation("имя_правила", func(fl validator.FieldLevel) bool {
        // fl.Field() — отражает значение поля
        // fl.GetStruct() — возвращает всю структуру (чтобы сравнивать поля)
        // возвращаем true — если валидно, false — иначе
    })

  Важно: кастомный валидатор выполняется каждый раз при вызове validate.Struct().
  Если он делает запрос к БД или внешнему API, это может быть дорого.
  Для таких случаев используйте кеширование или отложенную валидацию.

  5. ЛОКАЛИЗАЦИЯ ОШИБОК — ПОЧЕМУ ЭТО ВАЖНО И КАК ЭТО РАБОТАЕТ
  По умолчанию ошибки возвращаются на английском, например:
    "Email is a required field"
    "Password must be at least 8 characters"

  Для русскоязычных пользователей или для мультиязычных API нужно
  выдавать сообщения на языке пользователя.

  Библиотека go-playground/validator работает с universal-translator
  для перевода сообщений. Переводы уже подготовлены для многих языков
  в пакетах go-playground/locales/*.

  Алгоритм:
    1. Создаём экземпляр нужной локали (ru.New()).
    2. Регистрируем переводчик в universal-translator.
    3. Регистрируем стандартные переводы ошибок через
       ru_translations.RegisterDefaultTranslations(validate, translator).
    4. При получении ошибок вызываем ve.Translate(translator).

  Можно также переопределить отдельные сообщения:
    translator.Add("error-message-key", "ваш текст", false)

  В примерах мы используем русский переводчик, что делает сообщения
  понятными и профессиональными.

  6. ВАЛИДАЦИЯ ВЛОЖЕННЫХ СТРУКТУР, СЛАЙСОВ, КАРТ
  • Вложенные структуры валидируются рекурсивно, если поле имеет тег
    `validate` и структура не является указателем на нулевое значение.
    Для указателей: если указатель nil, то поле считается нулевым,
    если не стоит required, валидация пропускается.

  • Слайсы: валидация применяется к каждому элементу, если тип элемента
    — структура. Можно также использовать тег `unique` для проверки
    уникальности элементов.

  • Карты: валидация поддерживается, но только для значений, если они
    структуры. Также есть теги для проверки ключей и значений (но редко
    используются).

  7. ВАЛИДАЦИЯ С УЧЁТОМ КОНТЕКСТА (CONTEXT-AWARE VALIDATION)
  Иногда правила валидации зависят от роли пользователя, окружения или
  других динамических факторов. Например, администратор может создавать
  пользователей без подтверждения email, а обычный пользователь — нет.

  Решения:
    1. Использовать разные DTO для разных ролей.
    2. Использовать кастомные валидаторы, которые принимают контекст
       (например, через глобальные переменные или через параметры функции).
    3. Использовать валидацию на уровне бизнес-логики (после базовой
       валидации) — это часто самый гибкий способ.

  В go-playground/validator нет прямой поддержки контекста, но вы можете
  передавать данные через кастомные структуры или использовать замыкания.

  8. ПРОИЗВОДИТЕЛЬНОСТЬ И ОПТИМИЗАЦИЯ
  • Создавайте один экземпляр валидатора на всё приложение (синглтон).
  • Библиотека кеширует информацию о структурах, поэтому повторные
    вызовы validate.Struct() работают быстро.
  • Используйте `validate.StructPartial()` или `validate.StructExcept()`
    для валидации только части полей (если нужно).
  • Для больших структур с множеством полей используйте `omitempty`,
    чтобы пропускать нулевые значения.
  • Кастомные валидаторы с доступом к БД или внешним API могут стать
    узким местом — кешируйте результаты или делайте их асинхронными.

  9. ИНТЕГРАЦИЯ С ВЕБ-ФРЕЙМВОРКАМИ
  • Gin: использует go-playground/validator как стандарт, можно
    переопределить валидатор через binding.Validator.
  • Echo: также интегрирует валидатор, но требует настройки.
  • Fiber: аналогично.

  В примерах мы используем чистый net/http для демонстрации, но принцип
  тот же: декодируем JSON в структуру, затем вызываем validate.Struct().

  10. ОБРАБОТКА ОШИБОК И ЛОГИРОВАНИЕ
  Всегда возвращайте структурированный ответ с ошибками (например, JSON
  вида {"errors": {"field": "message"}}). Это облегчает интеграцию с
  фронтендом.

  Логируйте ошибки валидации на сервере (особенно если они связаны с
  подозрительными данными) — это помогает обнаруживать атаки или баги
  на клиенте.

  Не показывайте пользователю внутренние детали (например, названия полей
  из структуры Go), используйте человекочитаемые имена — для этого и
  существует RegisterTagNameFunc.

  11. ЧАСТЫЕ ОШИБКИ И АНТИ-ПАТТЕРНЫ (КРАСНЫЕ ФЛАГИ)
   Валидация только на клиенте — злоумышленник может обойти её.
   Использование валидации как единственной защиты от XSS/SQL-инъекций —
     нужны ещё экранирование и параметризованные запросы.
   Игнорирование ошибок валидации (не обрабатывать их).
   Слишком строгая валидация, которая мешает пользователям (например,
     не разрешать буквы ё, пробелы в имени).
   Хранение паролей в открытом виде даже после валидации — валидация
     не отменяет хеширование.
   Валидация больших файлов в памяти — использовать ограничения и
     стриминг.

  12. ТЕСТИРОВАНИЕ ВАЛИДАЦИИ
  Пишите юнит-тесты для валидации, проверяя как корректные, так и
  некорректные данные. Используйте табличные тесты.

  Для кастомных валидаторов пишите отдельные тесты, проверяя их логику
  изолированно.

  Также полезно тестировать интеграцию с HTTP (например, через httptest),
  чтобы убедиться, что ошибки возвращаются в правильном формате.

  13. ВАЛИДАЦИЯ И БЕЗОПАСНОСТЬ (ДОПОЛНИТЕЛЬНО)
  Валидация помогает предотвратить:
    • DoS-атаки через слишком длинные строки (используйте ограничения max).
    • Инъекции через поля, которые потом подставляются в запросы
      (хотя основная защита — параметризация).
    • Паники при неожиданных типах данных.

  Однако валидация НЕ заменяет:
    • Экранирование при выводе в HTML (защита от XSS).
    • Проверку прав доступа (авторизацию).
    • Шифрование и хеширование чувствительных данных.

  Всегда комбинируйте валидацию с другими защитными механизмами.

  14. СРАВНЕНИЕ С АЛЬТЕРНАТИВАМИ
  • Go-playground/validator — самый популярный, активно поддерживается,
    гибкий, с широкой экосистемой.
  • Go-validator (asaskevich/govalidator) — устаревший, не использует
    теги структур, менее удобен.
  • Собственная валидация через if-ы — много кода, трудно поддерживать.
  • Использование JSON Schema — подход для API-документации, но не
    интегрируется напрямую с Go-структурами.

  На сегодняшний день go-playground/validator — золотой стандарт.

  15. ПРИМЕРЫ СЛОЖНЫХ СЦЕНАРИЕВ (ЗАВИСИМАЯ ВАЛИДАЦИЯ)
  • Если поле Country == "US", то поле State обязательно.
  • Если пользователь старше 18 лет, то поле ParentEmail не обязательно.
  • Если поле PaymentMethod == "card", то поля CardNumber и CVV обязательны.

  Это можно реализовать через кастомные валидаторы, которые имеют доступ
  ко всей структуре через fl.GetStruct().

  Также можно использовать тег `dive` для валидации элементов внутри
  слайсов и карт, а тег `keys` — для валидации ключей карты.

  16. ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ (BEST PRACTICES)
  • Определяйте DTO (Data Transfer Objects) для каждого входящего запроса.
  • Используйте теги json для маппинга, а validate — для валидации.
  • Регистрируйте кастомные валидаторы как можно раньше (в init() или main()).
  • Всегда возвращайте пользователю список ошибок с человекочитаемыми
    сообщениями.
  • Не смешивайте валидацию с бизнес-логикой — валидация должна быть
    на уровне контроллера или сервиса.
  • Для сложных правил используйте кастомные валидаторы, а не засоряйте
    структуру сложными тегами.
  • Используйте omitempty для необязательных полей.
  • Для дат используйте тип time.Time и кастомный декодер или парсите
    отдельно.
  • Пишите юнит-тесты для валидации.
*/

//ВСПОМОГАТЕЛЬНЫЙ КОД

var (
	validate   *validator.Validate
	translator ut.Translator
	once       sync.Once
)

// initValidator инициализирует валидатор и переводчик один раз.
func initValidator() {
	once.Do(func() {
		validate = validator.New()

		// Используем JSON-имена для полей в ошибках
		validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := fld.Tag.Get("json")
			if name == "" {
				return fld.Name
			}
			if commaIdx := strings.Index(name, ","); commaIdx != -1 {
				name = name[:commaIdx]
			}
			return name
		})

		// Регистрируем кастомные валидаторы
		registerCustomValidators(validate)

		// Настраиваем русский переводчик
		ruLocale := ru.New()
		euLocale := eu.New()
		uni := ut.New(ruLocale, euLocale)
		var found bool
		translator, found = uni.GetTranslator("ru")
		if !found {
			log.Fatal("Russian translator not found")
		}
		if err := ru_translations.RegisterDefaultTranslations(validate, translator); err != nil {
			log.Fatal(err)
		}
	})
}

// registerCustomValidators добавляет пользовательские правила.
func registerCustomValidators(v *validator.Validate) {
	v.RegisterValidation("strong_password", func(fl validator.FieldLevel) bool {
		pw := fl.Field().String()
		if len(pw) < 8 {
			return false
		}
		hasDigit := regexp.MustCompile(`[0-9]`).MatchString(pw)
		hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(pw)
		hasLower := regexp.MustCompile(`[a-z]`).MatchString(pw)
		return hasDigit && hasUpper && hasLower
	})

	// Правило: дата не в будущем (для дня рождения)
	v.RegisterValidation("not_future", func(fl validator.FieldLevel) bool {
		t, ok := fl.Field().Interface().(time.Time)
		if !ok {
			return false
		}
		return !t.After(time.Now())
	})

	// Правило: email уникален (имитация проверки в БД)
	v.RegisterValidation("unique_email", func(fl validator.FieldLevel) bool {
		email := fl.Field().String()
		// Здесь реальный запрос к БД
		existing := map[string]bool{
			"existing@example.com": true,
			"admin@example.com":    true,
		}
		return !existing[email]
	})

	// Правило: если одно поле заполнено, то другое обязательно (зависимость)
	v.RegisterValidation("required_if_field", func(fl validator.FieldLevel) bool {
		// сложный пример: мы не можем легко получить другое поле через тег,
		// поэтому оставим как демонстрацию — будем использовать отдельную функцию.
		return true
	})
}

// ValidateStruct — универсальная обёртка, возвращает map[field]message
func ValidateStruct(s interface{}) (map[string]string, error) {
	initValidator()
	err := validate.Struct(s)
	if err == nil {
		return nil, nil
	}
	verr, ok := err.(validator.ValidationErrors)
	if !ok {
		return nil, err
	}
	result := make(map[string]string)
	for _, e := range verr {
		// Поле уже берётся из JSON-тега благодаря RegisterTagNameFunc
		result[e.Field()] = e.Translate(translator)
	}
	return result, nil
}

// ПРИМЕР 1: БАЗОВАЯ ВАЛИДАЦИЯ (ОСНОВНЫЕ ТЕГИ)
/*
  ЗАЧЕМ:
    Овладеть основами — обязательные поля, проверка email, длина, диапазон
    чисел, ограничение допустимых значений (oneof). Без этого невозможно
    построить ни один надёжный API.

  ФИШКИ:
    • Теги required, email, min, max, gte, lte, oneof.
    • Автоматический перевод ошибок на русский (через translator).
    • Использование JSON-имён полей в сообщениях об ошибках.
    • Отображение структурированного списка ошибок (map[field]message).
*/

type UserBasicDTO struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=32"`
	Age      int    `json:"age" validate:"required,gte=18,lte=99"`
	Gender   string `json:"gender" validate:"required,oneof=male female other"`
}

func primer1() {
	fmt.Println("\n=== ПРИМЕР 1: Базовая валидация ===")
	valid := UserBasicDTO{
		Email:    "test@primer.com",
		Password: "Pepsi221",
		Age:      25,
		Gender:   "male",
	}
	errs, _ := ValidateStruct(valid)
	fmt.Printf("Валидный: ошибок %d\n", len(errs))

	invalid := UserBasicDTO{Email: "not-email", Password: "short", Age: 16, Gender: "alien"}
	errs, _ = ValidateStruct(invalid)
	fmt.Println("Ошибки невалидного:")
	for f, m := range errs {
		fmt.Printf("  %s: %s\n", f, m)
	}
}

// ПРИМЕР 2: КАСТОМНЫЕ ВАЛИДАТОРЫ (ПАРОЛЬ, УНИКАЛЬНОСТЬ, ДАТА)
/*
  ЗАЧЕМ:
    Бизнес-правила часто выходят за рамки стандартных тегов. Например,
    сложные требования к паролю, проверка уникальности email в БД,
    валидация дат (не в прошлом/будущем). Без кастомных валидаторов
    пришлось бы писать много повторяющегося кода.

  ФИШКИ:
    • Регистрация своих валидаторов через validate.RegisterValidation.
    • Использование регулярных выражений для проверки пароля (strong_password).
    • Проверка даты с помощью time.Time и сравнение с текущей датой (not_future).
    • Имитация запроса к БД (unique_email) — в реальном проекте здесь будет
      обращение к репозиторию.
*/

type UserCustomDTO struct {
	Email     string    `json:"email" validate:"required,email,unique_email"`
	Password  string    `json:"password" validate:"required,min=8,strong_password"`
	BirthDate time.Time `json:"birth_date" validate:"required,not_future"`
	Age       int       `json:"age" validate:"required,gte=18,lte=99"`
}

func primer2() {
	fmt.Println("\n=== ПРИМЕР 2: Кастомные валидаторы ===")

	valid := UserCustomDTO{
		Email:     "existing@example.com",
		Password:  "StrongPwd123",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Age:       30,
	}
	errs, _ := ValidateStruct(valid)
	fmt.Printf("Валидный: ошибок %d\n", len(errs))

	invalid := UserCustomDTO{
		Email:     "existing@example.com",         // нарушает unique_email
		Password:  "weak",                         // min и strong_password
		BirthDate: time.Now().Add(24 * time.Hour), // not_future
		Age:       17,                             // gte=18
	}
	errs, _ = ValidateStruct(invalid)
	fmt.Println("Ошибки невалидного:")
	for f, m := range errs {
		fmt.Printf("  %s: %s\n", f, m)
	}
}

// ПРИМЕР 3: ВЛОЖЕННЫЕ СТРУКТУРЫ И СЛАЙСЫ
/*
  ЗАЧЕМ:
    Реальные данные почти всегда содержат массивы и вложенные объекты
    (адрес, теги, список позиций заказа). Нужно уметь валидировать
    каждый элемент и каждое поле.

  ФИШКИ:
    • Тег dive — говорит валидатору заглянуть внутрь структуры или слайса.
    • Тег unique — проверяет уникальность элементов в слайсе.
    • Можно комбинировать: `unique,dive,alphanum` — уникальность и каждый
      элемент состоит только из букв/цифр.
    • Вложенные структуры валидируются рекурсивно, если поле имеет тег validate.
*/

type Address struct {
	Street  string `json:"street" validate:"required"`
	City    string `json:"city" validate:"required"`
	ZipCode string `json:"zip_code" validate:"required,len=6"`
	Country string `json:"country" validate:"required,oneof=US CA GB"`
}

type UserNestedDTO struct {
	Email    string   `json:"email" validate:"required,email"`
	Password string   `json:"password" validate:"required,min=8"`
	Address  Address  `json:"address" validate:"required,dive"` // dive — валидируем вложенную структуру
	Tags     []string `json:"tags" validate:"required,min=1,max=5,unique,dive,alphanum"`
}

func primer3() {
	fmt.Println("\n=== ПРИМЕР 3: Вложенные структуры и слайсы ===")

	valid := UserNestedDTO{
		Email:    "test@example.com",
		Password: "12345678",
		Address: Address{
			Street:  "123 Main St",
			City:    "New York",
			ZipCode: "100001",
			Country: "US",
		},
		Tags: []string{"go", "programming"},
	}
	errs, _ := ValidateStruct(valid)
	fmt.Printf("Валидный: ошибок %d\n", len(errs))

	invalid := UserNestedDTO{
		Email:    "test@example.com",
		Password: "12345678",
		Address: Address{
			Street:  "", // required
			City:    "New York",
			ZipCode: "123", // len=6
			Country: "RU",  // oneof
		},
		Tags: []string{"go", "go!", "go!"}, // повтор и alphanum нарушен
	}
	errs, _ = ValidateStruct(invalid)
	fmt.Println("Ошибки невалидного:")
	for f, m := range errs {
		fmt.Printf("  %s: %s\n", f, m)
	}
}

// ПРИМЕР 4: ЛОКАЛИЗАЦИЯ ОШИБОК НА РУССКИЙ
/*
  ЗАЧЕМ:
    Пользователи должны понимать, что именно не так с их запросом.
    Сообщения на родном языке повышают UX и уменьшают количество
    повторных обращений в поддержку.

  ФИШКИ:
    • Подключение пакетов go-playground/locales и universal-translator.
    • Регистрация стандартных переводов через ru_translations.
    • Использование e.Translate(translator) при формировании ошибок.
    • Можно переопределять отдельные сообщения через translator.Add().
*/
func primer4() {
	fmt.Println("\n=== ПРИМЕР 4: Локализация на русский ===")

	// Вся локализация уже настроена в initValidator()
	invalid := UserBasicDTO{
		Email:    "not-email",
		Password: "short",
		Age:      16,
		Gender:   "alien",
	}
	errs, _ := ValidateStruct(invalid)
	fmt.Println("Русские сообщения:")
	for f, m := range errs {
		fmt.Printf("  %s: %s\n", f, m)
	}
}

// ПРИМЕР 5: ЗАВИСИМЫЕ ПОЛЯ (ПОДТВЕРЖДЕНИЕ ПАРОЛЯ)
/*
  ЗАЧЕМ:
    При регистрации или смене пароля пользователь должен ввести его дважды,
    чтобы исключить опечатки. eqfield — простой и элегантный способ проверить
    совпадение двух полей.

  ФИШКИ:
    • Тег eqfield — сравнивает значение текущего поля со значением указанного поля.
    • Не требует кастомного кода — всё делается тегами.
    • Если поля не совпадают, генерируется понятная ошибка.
*/

type UserWithConfirmDTO struct {
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=8"`
	PasswordConfirm string `json:"password_confirm" validate:"required,eqfield=Password"`
}

func primer5() {
	fmt.Println("\n=== ПРИМЕР 5: Подтверждение пароля (eqfield) ===")

	valid := UserWithConfirmDTO{
		Email:           "test@primer.com",
		Password:        "PupuPipi",
		PasswordConfirm: "PupuPipi",
	}
	errs, _ := ValidateStruct(valid)
	fmt.Printf("Валидный: ошибок %d\n", len(errs))

	invalid := UserWithConfirmDTO{
		Email:           "test@primer.com",
		Password:        "12345678",
		PasswordConfirm: "wrong",
	}
	errs, _ = ValidateStruct(invalid)
	fmt.Println("Ошибки (пароли не совпадают):")
	for f, m := range errs {
		fmt.Printf("  %s: %s\n", f, m)
	}
}

// ПРИМЕР 6: КОНТЕКСТНО-ЗАВИСИМАЯ ВАЛИДАЦИЯ (РОЛЬ)
/*
  ЗАЧЕМ:
    В зависимости от роли пользователя (админ, обычный пользователь)
    требования к данным могут меняться. Например, админу можно не вводить
    номер телефона, а обычному пользователю — обязательно.

  ФИШКИ:
    • Кастомный валидатор, который обращается к глобальному контексту.
    • Гибкость: можно передавать любые параметры (роль, окружение, настройки).
    • Альтернатива — использовать разные DTO для разных ролей, но контекстный
      подход позволяет избежать дублирования структур.
    • В реальных проектах контекст передаётся через значение в запросе (например, через middleware).
*/

// Глобальный контекст для демонстрации
var validationContext = struct {
	IsAdmin bool
}{}

type UserWithContextDTO struct {
	Email string `json:"email" validate:"required,email"`
	Phone string `json:"phone" validate:"required_if_is_admin"` // кастомный тег
}

func primer6() {
	fmt.Println("\n=== ПРИМЕР 6: Контекстная валидация (роль) ===")

	// Регистрируем кастомный валидатор (вызываем вручную, т.к. initValidator уже выполнен)
	validate.RegisterValidation("required_if_is_admin", func(fl validator.FieldLevel) bool {
		if validationContext.IsAdmin {
			return fl.Field().String() != ""
		}
		return true
	})
	validationContext.IsAdmin = false
	user := UserWithContextDTO{Email: "test@example.com", Phone: ""}
	errs, _ := ValidateStruct(user)
	fmt.Printf("Обычный пользователь без телефона: ошибок %d\n", len(errs))

	// Сценарий: админ — телефон обязателен
	validationContext.IsAdmin = true
	errs, _ = ValidateStruct(user)
	fmt.Println("Админ без телефона: ошибки:")
	for f, m := range errs {
		fmt.Printf("  %s: %s\n", f, m)
	}

	// Админ с телефоном
	user.Phone = "+123456789"
	errs, _ = ValidateStruct(user)
	fmt.Printf("Админ с телефоном: ошибок %d\n", len(errs))

	// Сбрасываем
	validationContext.IsAdmin = false
}

// ПРИМЕР 7: ЧАСТИЧНАЯ ВАЛИДАЦИЯ (PATCH)
/*
  ЗАЧЕМ:
    При частичном обновлении (PATCH-запрос) клиент присылает только те поля,
    которые хочет изменить. Нужно валидировать только переданные поля,
    а отсутствующие не проверять.

  ФИШКИ:
    • Тег omitempty — поле пропускается, если оно нулевое.
    • validate.StructPartial() — валидация только указанных полей.
    • Очень полезно для экономии ресурсов и удобства клиента.
    • В примере показано, как валидировать только Email или только Password.
*/

type UserPatchDTO struct {
	Email    string `json:"email" validate:"omitempty,email"`
	Password string `json:"password" validate:"omitempty,min=8"`
	Age      int    `json:"age" validate:"omitempty,gte=18,lte=99"`
}

// ValidatePartial — валидирует только указанные поля.
func ValidatePartial(s interface{}, fields ...string) (map[string]string, error) {
	initValidator()
	err := validate.StructPartial(s, fields...)
	if err == nil {
		return nil, nil
	}
	verr, ok := err.(validator.ValidationErrors)
	if !ok {
		return nil, err
	}
	result := make(map[string]string)
	for _, e := range verr {
		result[e.Field()] = e.Translate(translator)
	}
	return result, nil
}

func primer7() {
	fmt.Println("\n=== ПРИМЕР 7: Частичная валидация (PATCH) ===")

	user := UserPatchDTO{
		Email:    "not-email",
		Password: "short",
		Age:      16,
	}

	// Валидируем только Email
	errs, _ := ValidatePartial(user, "Email")
	fmt.Println("Валидация только Email:")
	for f, m := range errs {
		fmt.Printf("  %s: %s\n", f, m)
	}

	// Валидируем только Password
	errs, _ = ValidatePartial(user, "Password")
	fmt.Println("Валидация только Password:")
	for f, m := range errs {
		fmt.Printf("  %s: %s\n", f, m)
	}

	// Валидируем все поля (с omitempty для отсутствующих)
	errs, _ = ValidateStruct(user)
	fmt.Println("Валидация всех полей (omitempty пропускает нулевые):")
	if len(errs) == 0 {
		fmt.Println("  Все поля валидны (или пропущены)")
	} else {
		for f, m := range errs {
			fmt.Printf("  %s: %s\n", f, m)
		}
	}
}

// ПРИМЕР 8: ПОЛНЫЙ HTTP-ЭНДПОИНТ (РЕГИСТРАЦИЯ) С ВАЛИДАЦИЕЙ
/*
  ЗАЧЕМ:
    Это конечный результат — полноценный REST эндпоинт для регистрации,
    который включает все описанные техники: декодирование JSON, валидацию
    всех полей, обработку дат, возврат структурированных ошибок и успешный
    ответ.

  ФИШКИ:
    • Полный цикл: JSON → структура → валидация → ответ.
    • Отдельная валидация даты (парсинг строки в time.Time).
    • Возврат ошибок в едином формате с полем "errors" (маппинг поле → сообщение).
    • Использование всех тегов из предыдущих примеров: required, email,
      min, strong_password, eqfield, gte, lte.
    • Обработка ошибок декодирования JSON (400 Bad Request).
    • Функция sendJSONError для единообразного ответа.
    • Graceful shutdown.
*/

type RegisterRequest struct {
	Email           string `json:"email" validate:"required,email,unique_email"`
	Password        string `json:"password" validate:"required,min=8,strong_password"`
	PasswordConfirm string `json:"password_confirm" validate:"required,eqfield=Password"`
	BirthDate       string `json:"birth_date" validate:"required"` // парсим отдельно
	Age             int    `json:"age" validate:"required,gte=18,lte=120"`
	AgreeToTerms    bool   `json:"agree_to_terms" validate:"required"`
}

type RegisterResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message,omitempty"`
	Errors  map[string]string `json:"errors,omitempty"`
}

func sendJSONError(w http.ResponseWriter, status int, message string, errors map[string]string) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(RegisterResponse{
		Success: false,
		Message: message,
		Errors:  errors,
	})
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	const layout = "2006-01-02"
	initValidator()

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, http.StatusBadRequest, "Invalid JSON format", nil)
		return
	}

	// Парсим дату рождения
	var birthDate time.Time
	if req.BirthDate != "" {
		var err error
		birthDate, err = time.Parse(layout, req.BirthDate)
		if err != nil {
			sendJSONError(w, http.StatusBadRequest, "Invalid birth_date format (YYYY-MM-DD)", nil)
			return
		}
		if birthDate.After(time.Now()) {
			sendJSONError(w, http.StatusBadRequest, "Birth date cannot be in the future", nil)
			return
		}
	}

	// Валидируем основные поля (без BirthDate, т.к. мы её уже проверили)
	type CoreDTO struct {
		Email           string `json:"email" validate:"required,email,unique_email"`
		Password        string `json:"password" validate:"required,min=8,strong_password"`
		PasswordConfirm string `json:"password_confirm" validate:"required,eqfield=Password"`
		Age             int    `json:"age" validate:"required,gte=18,lte=99"`
		AgreeToTerms    bool   `json:"agree_to_terms" validate:"required"`
	}
	core := CoreDTO{
		Email:           req.Email,
		Password:        req.Password,
		PasswordConfirm: req.PasswordConfirm,
		Age:             req.Age,
		AgreeToTerms:    req.AgreeToTerms,
	}

	errs, err := ValidateStruct(core)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "Validation error", nil)
		return
	}
	if len(errs) > 0 {
		sendJSONError(w, http.StatusBadRequest, "Validation failed", errs)
		return
	}

	// Всё ок — регистрируем пользователя (имитация сохранения в БД)
	// Здесь вы бы создавали запись в БД и отправляли подтверждение на email.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(RegisterResponse{
		Success: true,
		Message: "User registered successfully",
	})
}

func primer8() {
	fmt.Println("\n=== ПРИМЕР 8: HTTP-сервер с валидацией (продакшен) ===")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", RegisterHandler)

	addr := ":8096"
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Printf("Сервер запущен на http://localhost%s\n", addr)
	fmt.Println("Эндпоинт: POST /register")
	fmt.Println("Пример запроса:")
	fmt.Println(`{
  "email": "new@example.com",
  "password": "StrongPwd123",
  "password_confirm": "StrongPwd123",
  "birth_date": "1990-01-01",
  "age": 30,
  "agree_to_terms": true
}`)
	fmt.Println("Нажмите Enter для остановки")

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	fmt.Scanln()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	fmt.Println("Сервер остановлен")
}

func main() {
	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}
