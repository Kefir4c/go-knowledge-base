package __6_orm_gorm_go

import (
	"errors"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

/*
УРОК 5.6: ORM (GORM) — РАСШИРЕННАЯ ТЕОРИЯ
1. ВНУТРЕННЕЕ УСТРОЙСТВО GORM: КАК ЭТО РАБОТАЕТ ПОД КАПОТОМ

GORM использует рефлексию (reflect) для анализа структур во время выполнения.
Это позволяет:
  - Маппить имена полей на колонки в БД
  - Определять типы данных
  - Строить SQL-запросы автоматически

Как это выглядит внутри:
  1. GORM анализирует структуру (reflect.TypeOf)
  2. Строит карту полей → колонки
  3. Генерирует SQL на основе этой карты
  4. Выполняет запрос и сканирует результат обратно в структуру

Цена рефлексии:
  - Медленнее, чем ручной SQL (на 10-30% в простых случаях)
  - На сложных запросах разрыв может быть больше
  - В высоконагруженных системах это заметно

2. МИГРАЦИИ В GORM VS ОТДЕЛЬНЫЕ МИГРАЦИИ (GOOSE, GOLANG-MIGRATE)

GORM.AutoMigrate() — удобно для разработки, но опасно для продакшена:

  Плюсы:
    - Автоматически создаёт таблицы и индексы
    - Добавляет новые колонки
    - Изменяет типы (иногда)

  Минусы (почему не стоит использовать в продакшене):
    - Не удаляет колонки (только добавляет)
    - Может потерять данные при изменении типов
    - Нет контроля над версиями схемы
    - Не поддерживает сложные изменения (переименование таблиц, перенос данных)

  Лучшая практика:
    - В разработке: GORM.AutoMigrate() (быстро прототипировать)
    - В продакшене: отдельные миграции (goose, golang-migrate)
    - Миграции должны быть идемпотентными и обратно-совместимыми

3. SOFT DELETE — КАК ЭТО РАБОТАЕТ И КОГДА ИСПОЛЬЗОВАТЬ

Soft Delete — это паттерн, при котором записи не удаляются физически,
а помечаются как удалённые (поле DeletedAt).

  Преимущества:
    - Данные можно восстановить
    - Можно сохранить историю
    - Легче откатить ошибку

  Недостатки:
    - Таблица растёт
    - Нужно всегда добавлять WHERE deleted_at IS NULL
    - Уникальные индексы становятся сложнее

  Как это работает в GORM:
    type User struct {
        ID        uint
        Name      string
        DeletedAt gorm.DeletedAt `gorm:"index"`
    }

    db.Delete(&User{}, id)  // UPDATE users SET deleted_at = NOW() WHERE id = ?
    db.Find(&users)         // SELECT * FROM users WHERE deleted_at IS NULL

    // Чтобы найти удалённые записи:
    db.Unscoped().Find(&deletedUsers)

  Когда использовать:
    ✅ Данные, которые могут понадобиться для аудита
    ✅ Конфигурации, настройки (чтобы не потерять при откате)
    ✅ Пользователи (если нужно восстановить аккаунт)

  Когда НЕ использовать:
    ❌ Логи, события (лучше физическое удаление или партиционирование)
    ❌ Временные данные (сессии, кэш)
    ❌ Когда важна производительность (лишнее условие в каждом запросе)

4. ASSOCIATIONS (СВЯЗИ) — КАК ЭТО РАБОТАЕТ И КОГДА ИСПОЛЬЗОВАТЬ

GORM поддерживает три типа связей:

  4.1. Belongs To (Принадлежит)
    User belongs to Company → User имеет CompanyID и Company

  4.2. Has Many (Имеет много)
    User has many Notifications → User имеет Notifications (слайс)

  4.3. Many To Many (Многие ко многим)
    User has many Roles and Role has many Users → через связующую таблицу

  Пример Many-to-Many:
    type User struct {
        ID    uint
        Name  string
        Roles []Role `gorm:"many2many:user_roles;"`
    }

    type Role struct {
        ID   uint
        Name string
    }

  При использовании связей GORM может генерировать запросы разными способами:
    - Preload — загружает связанные данные через отдельные запросы (JOIN)
    - Joins — делает INNER JOIN (для фильтрации)
    - Association — методы для управления связями (Append, Replace, Clear)

5. N+1 ПРОБЛЕМА — ПОЧЕМУ ОНА ВОЗНИКАЕТ И КАК ЕЁ ОБНАРУЖИТЬ

N+1 проблема возникает, когда GORM делает:
  - 1 запрос для получения списка записей (SELECT * FROM users)
  - N запросов для получения связанных данных для каждой записи (SELECT * FROM notifications WHERE user_id = ?)

  Почему это плохо:
    - При 100 пользователях — 101 запрос
    - При 1000 — 1001 запрос
    - Это убивает производительность БД и сети

  Как обнаружить:
    1. Включить логирование SQL (ShowSQL)
    2. Посчитать количество запросов в логах
    3. Использовать EXPLAIN ANALYZE

  Как исправить:
    - Использовать Preload (JOIN + один запрос)
    - Использовать Joins (INNER JOIN + SELECT)
    - Использовать отдельный запрос для всех связанных данных (ручной сбор)

  Сравнение подходов (для 10 пользователей):

    | Подход               | Запросов | Время (ms) |
    |----------------------|----------|------------|
    | Без Preload (N+1)    | 11       | 50-100     |
    | Preload              | 2        | 10-20      |
    | Joins                | 1        | 5-10       |
    | sqlc (ручной SQL)    | 1        | 2-5        |

6. ТРАНЗАКЦИИ В GORM

GORM поддерживает транзакции двумя способами:

  6.1. Автоматические (Callback)
    db.Create(&user)  // одна операция — одна транзакция

  6.2. Ручные (Manual)
    tx := db.Begin()
    if err := tx.Create(&user).Error; err != nil {
        tx.Rollback()
        return err
    }
    if err := tx.Create(&notification).Error; err != nil {
        tx.Rollback()
        return err
    }
    tx.Commit()

  Советы по транзакциям:
    - Всегда проверяй ошибки после каждой операции
    - Используй defer для Rollback (на случай паники)
    - Держи транзакции короткими

7. ЛОГИРОВАНИЕ SQL В GORM (КАК ВИДЕТЬ, ЧТО ГЕНЕРИРУЕТСЯ)

  Способ 1: Config.Logger
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
        Logger: logger.Default.LogMode(logger.Info),
    })

  Способ 2: Custom Logger
    db.Logger = logger.New(
        log.New(os.Stdout, "\r\n", log.LstdFlags),
        logger.Config{
            SlowThreshold: time.Second,
            LogLevel:      logger.Info,
        },
    )

  Способ 3: DryRun (генерирует SQL без выполнения)
    stmt := db.Session(&gorm.Session{DryRun: true}).First(&user, 1).Statement
    sql := stmt.SQL.String()
    log.Println(sql)

8. КОГДА ИСПОЛЬЗОВАТЬ GORM — РЕАЛЬНЫЕ СЦЕНАРИИ

   Использовать GORM:

    - Админки, блоги, простые CRUD-сервисы
    - Прототипирование и MVP
    - Проекты с одной базой данных (нет сложных запросов)
    - Команда, где разработчики не знают SQL
    - Внутренние инструменты (не high-load)

   НЕ использовать GORM:

    - High-load сервисы (10k+ RPS)
    - Микросервисы с интенсивными запросами к БД
    - Системы с аналитикой и сложными JOIN
    - Когда нужен полный контроль над запросами
    - Когда важно минимизировать latency

   Комбинированный подход (best practice):
    - GORM для простых CRUD (админка, настройки)
    - sqlc или ручной SQL для сложных запросов (отчёты, аналитика)
    - GORM для миграций в разработке, goose для продакшена

9. АЛЬТЕРНАТИВЫ GORM (СРАВНЕНИЕ)

  | Инструмент  | Когда использовать                     | Производительность | Удобство |
  |-------------|----------------------------------------|--------------------|----------|
  | GORM        | Прототипы, CRUD, админки               | 🟡 Средняя         | 🟢 Высокое |
  | sqlc        | Стабильные запросы, high-load          | 🟢 Высокая         | 🟡 Среднее |
  | Squirrel    | Динамические запросы (фильтры)         | 🟢 Высокая         | 🟢 Высокое |
  | Ent         | GraphQL-подобные запросы, codegen      | 🟢 Высокая         | 🟡 Среднее |
  | Handwritten | Полный контроль, сложные запросы       | 🟢 Высокая         | 🔴 Низкое  |
*/
// 1. МОДЕЛИ (STRUCTS)

// User — модель пользователя
type User struct {
	ID        uint           `gorm:"primaryKey"`
	Name      string         `gorm:"size:255;not null"`
	Email     string         `gorm:"size:255;uniqueIndex;not null"`
	Password  string         `gorm:"size:255;not null"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"` // Soft Delete (gorm.DeletedAt — специальный тип)

	// Ассоциации: у пользователя есть уведомления (One-to-Many)
	Notifications []Notification `gorm:"foreignKey:UserID"`
}

// Notification — модель уведомления
type Notification struct {
	ID        uint       `gorm:"primaryKey"`
	UserID    uint       `gorm:"not null"`
	Message   string     `gorm:"type:text;not null"`
	Status    string     `gorm:"type:varchar(20);default:'pending'"`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
	SentAt    *time.Time `gorm:"default:null"`

	// Ассоциация: уведомление принадлежит пользователю (Many-to-One)
	User User `gorm:"foreignKey:UserID"`
}

// 2. HOOKS (ЖИЗНЕННЫЙ ЦИКЛ МОДЕЛИ)

// BeforeCreate — вызывается перед созданием записи
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	log.Printf("[HOOK] BeforeCreate: %s", u.Name)
	// Например, можно хешировать пароль
	// u.Password = hashPassword(u.Password)
	return nil
}

// AfterCreate — вызывается после создания записи
func (u *User) AfterCreate(tx *gorm.DB) (err error) {
	log.Printf("[HOOK] AfterCreate: User %s created (ID: %d)", u.Name, u.ID)
	return nil
}

// BeforeUpdate — вызывается перед обновлением
func (u *User) BeforeUpdate(tx *gorm.DB) (err error) {
	log.Printf("[HOOK] BeforeUpdate: User %s", u.Name)
	return nil
}

// AfterFind — вызывается после загрузки из БД
func (u *User) AfterFind(tx *gorm.DB) (err error) {
	log.Printf("[HOOK] AfterFind: User %s loaded", u.Name)
	return nil
}

// BeforeDelete — вызывается перед удалением (Soft Delete)
func (u *User) BeforeDelete(tx *gorm.DB) (err error) {
	log.Printf("[HOOK] BeforeDelete: User %s", u.Name)
	return nil
}

// 3. CRUD (СОЗДАНИЕ, ЧТЕНИЕ, ОБНОВЛЕНИЕ, УДАЛЕНИЕ)

// UserRepository — репозиторий для работы с пользователями
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create — создание пользователя
func (r *UserRepository) Create(name, email, password string) (*User, error) {
	user := &User{
		Name:     name,
		Email:    email,
		Password: password, // в реальном проекте хешируй!
	}
	result := r.db.Create(user)
	if result.Error != nil {
		return nil, result.Error
	}
	return user, nil
}

// GetByID — получение пользователя по ID
func (r *UserRepository) GetByID(id uint) (*User, error) {
	var user User
	result := r.db.First(&user, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

// GetByEmail — получение пользователя по email
func (r *UserRepository) GetByEmail(email string) (*User, error) {
	var user User
	result := r.db.Where("email = ?", email).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

// Update — обновление пользователя
func (r *UserRepository) Update(id uint, name, email string) error {
	result := r.db.Model(&User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name":  name,
		"email": email,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete — мягкое удаление (Soft Delete)
func (r *UserRepository) Delete(id uint) error {
	result := r.db.Delete(&User{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// List — список пользователей с пагинацией
func (r *UserRepository) List(page, pageSize int) ([]User, int64, error) {
	var users []User
	var total int64

	// Считаем общее количество
	r.db.Model(&User{}).Count(&total)

	offset := (page - 1) * pageSize
	result := r.db.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&users)
	if result.Error != nil {
		return nil, 0, result.Error
	}

	return users, total, nil
}

// 4. АССОЦИАЦИИ (СВЯЗИ)

// CreateUserWithNotifications — создание пользователя с уведомлениями
func (r *UserRepository) CreateUserWithNotifications(user *User, messages []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		for _, msg := range messages {
			notif := &Notification{
				UserID:  user.ID,
				Message: msg,
			}
			if err := tx.Create(notif).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetUserWithNotifications — получение пользователя с его уведомлениями
func (r *UserRepository) GetUserWithNotifications(id uint) (*User, error) {
	var user User
	// Preload — загружает связанные уведомления (JOIN)
	result := r.db.Preload("Notifications").First(&user, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

// 5. N+1 ПРОБЛЕМА (ДЕМОНСТРАЦИЯ И РЕШЕНИЕ)

//	ПЛОХО: N+1 проблема
//
// Запрос 1: SELECT * FROM users (N пользователей)
// Запросы N: SELECT * FROM notifications WHERE user_id = ? (для каждого пользователя)
func (r *UserRepository) ListUsersWithNotificationsBad() ([]User, error) {
	var users []User
	if err := r.db.Find(&users).Error; err != nil {
		return nil, err
	}

	// Для каждого пользователя загружаем уведомления отдельным запросом!
	for i := range users {
		var notifs []Notification
		if err := r.db.Where("user_id = ?", users[i].ID).Find(&notifs).Error; err != nil {
			return nil, err
		}
		users[i].Notifications = notifs
	}
	return users, nil
}

// ХОРОШО: используем Preload (один запрос + JOIN)
func (r *UserRepository) ListUsersWithNotificationsGood() ([]User, error) {
	var users []User
	// Preload — GORM загрузит все уведомления одним запросом
	err := r.db.Preload("Notifications").Find(&users).Error
	return users, err
}

// ЕЩЁ ЛУЧШЕ: Preload с условиями
func (r *UserRepository) ListUsersWithPendingNotifications() ([]User, error) {
	var users []User
	err := r.db.Preload("Notifications", "status = ?", "pending").Find(&users).Error
	return users, err
}

// 6. ЛОГИРОВАНИЕ SQL (ЧТОБЫ ВИДЕТЬ, ЧТО ГЕНЕРИРУЕТСЯ)

// SetupLogger — настройка логгера GORM (показывает SQL-запросы)
func SetupLogger() logger.Interface {
	return logger.New(
		log.New(log.Writer(), "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second, // логировать запросы > 1 сек
			LogLevel:                  logger.Info, // логировать все запросы
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)
}

// 7. MAIN (ПРИМЕР ИСПОЛЬЗОВАНИЯ)

func main() {
	// 1. Подключение к БД
	dsn := "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable TimeZone=UTC"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: SetupLogger(), // логируем запросы
	})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// 2. Автоматическая миграция (создание таблиц)
	err = db.AutoMigrate(&User{}, &Notification{})
	if err != nil {
		log.Fatal("Failed to migrate:", err)
	}
	log.Println(" Tables migrated")

	// 3. Создаём репозиторий
	repo := NewUserRepository(db)

	// 4. Пример: создание пользователя
	user, err := repo.Create("Alice", "alice@mail.com", "secret123")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf(" User created: %+v", user)

	// 5. Пример: создание уведомлений для пользователя
	msgs := []string{
		"Welcome to the service!",
		"Please verify your email",
		"Your profile is 80% complete",
	}
	if err := repo.CreateUserWithNotifications(user, msgs); err != nil {
		log.Fatal(err)
	}
	log.Printf(" Notifications created for user %s", user.Name)

	// 6. Пример: получение пользователя с уведомлениями (без N+1)
	userWithNotifs, err := repo.GetUserWithNotifications(user.ID)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf(" User with notifications: %s (%d notifs)", userWithNotifs.Name, len(userWithNotifs.Notifications))

	// 7. Пример: список пользователей с уведомлениями (Preload)
	users, err := repo.ListUsersWithNotificationsGood()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf(" All users with notifications: %d users", len(users))

	// 8. Пример: удаление пользователя (Soft Delete)
	if err := repo.Delete(user.ID); err != nil {
		log.Fatal(err)
	}
	log.Printf(" User %s soft-deleted", user.Name)

	// 9. Пример: восстановление пользователя (Unscoped)
	var deletedUser User
	err = db.Unscoped().First(&deletedUser, user.ID).Error
	if err == nil {
		log.Printf(" User is soft-deleted, can be restored: %+v", deletedUser)
		// Восстановление: db.Unscoped().Model(&User{}).Where("id = ?", user.ID).Update("deleted_at", nil)
	}
}
