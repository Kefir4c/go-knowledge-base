package flagconfig

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// 1. CONFIG & FLAGS: ОСНОВЫ
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:
1. Зачем управлять настройками?
Приложение должно быть гибким. Один и тот же бинарник должен уметь запускаться
на ноутбуке разработчика (порт 8080, локальная БД) и на сервере (порт 80, прод БД).
Настройки позволяют менять поведение программы без перекомпиляции.

2. Пакет flag:
- Встроен в стандартную библиотеку Go.
- Работает только с аргументами командной строки (при запуске: ./app -port=80).
- Типизация: Флаги строго типизированы (int, string, bool, duration).

3. Аргументы vs Флаги:
- Флаги: Имеют имя и значение (-port=8080).
- Аргументы (Args): Это "хвост" после всех флагов. Например, в команде
  `rm -rf /tmp`, "-rf" — это флаги, а "/tmp" — неименованный аргумент.
*/

// 1: Разница между flag.Int и flag.IntVar
// На собесе могут спросить: "Как прокинуть флаг сразу в готовую структуру?"
type Config struct {
	Port    int
	Timeout time.Duration
}

func ExampleBinding() {
	cfg := &Config{}

	// Var-вариация позволяет писать значение сразу в адрес переменной в структуре
	flag.IntVar(&cfg.Port, "Port", 8080, "API Port")
	flag.DurationVar(&cfg.Timeout, "Timeout", 30*time.Second, "Request timeout")

	// Важно: Parse должен быть один на программу (в рамках дефолтного набора)
	flag.Parse()
}

// --- ПРИМЕР 2: Custom Flag (Интерфейс flag.Value) ---
// Позволяет парсить сложные типы (например, слайс строк из одного флага).
type sliceValue []string

func (s *sliceValue) String() string {
	return strings.Join(*s, ",")
}

func (s *sliceValue) Set(value string) error {
	if value == "" {
		return errors.New("empty value")
	}
	*s = append(*s, value)
	return nil
}

func ExampleCustomType() {
	var userRoles sliceValue
	// flag.Var принимает интерфейс flag.Value
	flag.Var(&userRoles, "role", "User roles (can be used multiple times: -role=admin -role=editor)")

	flag.Parse()
	fmt.Println("Roles:", userRoles)
}

// --- ПРИМЕР 3: Изолированные наборы флагов (flag.FlagSet) ---
// Используется в утилитах типа 'git', где есть подкоманды (git push, git pull).
func ExampleSubcommands() {
	// Создаем отдельный набор для команды 'server'
	serverCmd := flag.NewFlagSet("server", flag.ExitOnError)
	serverPort := serverCmd.Int("port", 8080, "server port")

	// И для команды 'client'
	clientCmd := flag.NewFlagSet("client", flag.ExitOnError)
	clientTarget := clientCmd.String("target", "localhost", "target server")

	if len(os.Args) < 2 {
		fmt.Println("expected 'server' or 'client' subcommands")
		return
	}

	switch os.Args[1] {
	case "server":
		serverCmd.Parse(os.Args[2:])
		fmt.Printf("Starting server on %d\n", *serverPort)
	case "client":
		clientCmd.Parse(os.Args[2:])
		fmt.Printf("Client connecting to %s\n", *clientTarget)
	}
}

//2. ФАЙЛЫ, ENV И VIPER
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:
1. Почему файлы (YAML/JSON)?
Флаги хороши для 2-3 параметров. Если настроек 50 (БД, кеш, логи, лимиты, токены),
их проще хранить в структурированном файле. YAML сейчас — стандарт де-факто
из-за читаемости и поддержки комментариев.

2. Переменные окружения (ENV):
В Docker и Kubernetes это основной способ доставки секретов.
Настройки не должны "торчать" в файлах внутри образа.

3. Viper:
Самая популярная библиотека в Go для конфигов. Главная фишка — умеет объединять
все источники (defaults -> config file -> env -> flags) в одну структуру.
*/

// Структура нашего конфига (используем mapstructure для Viper)
type Config2 struct {
	Port    int    `mapstructure:"PORT"`
	DBUrl   string `mapstructure:"DB_URL"`
	Timeout string `mapstructure:"TIMEOUT"`
}

//1: Чтение JSON/YAML через Viper

func LoadConfig() (cfg Config2, err error) {
	// 1. Путь к файлу
	viper.AddConfigPath("./config")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// 2. Чтение ENV (очень важно для Docker)
	// Viper будет искать переменные типа APP_PORT, APP_DB_URL
	viper.SetEnvPrefix("APP")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return cfg, err
	}

	err = viper.Unmarshal(&cfg)
	return
}

// 2: Приоритетность (Precedence) своими руками
// Если не хочешь тащить Viper, важно понимать логику перекрытия:
func GetParam(flagValue string, envKey string, defaultValue string) string {
	// 1. Проверяем флаг
	if flagValue != "" {
		return flagValue
	}
	// 2. Проверяем ENV
	if env := os.Getenv(envKey); env != "" {
		return env
	}
	// 3. Возвращаем дефолт
	return defaultValue
}

//3: Работа с JSON напрямую (стандартная библиотека)

func LoadJSONManual() {
	data, _ := os.ReadFile("config.json")

	var cfg Config2
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatal(err)
	}
}

// 4: Вложенные структуры и мапинг
// В реальных проектах конфиг — это дерево.
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
	// Вложенная структура для БД
	DB DatabaseConfig `mapstructure:"database"`
}

type DatabaseConfig struct {
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
}

func LoadNestedConfig() (*ServerConfig, error) {
	v := viper.New() // Создаем отдельный экземпляр (лучше, чем глобальный)

	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.SetConfigType("yaml")

	// Установка дефолтов для вложенных полей
	v.SetDefault("database.name", "my_app_db")
	v.SetDefault("port", 8080)

	// Чтение переменных окружения для вложенных полей
	// Чтобы заменить database.user, нужно передать APP_DATABASE_USER
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err // Если файл есть, но он кривой — возвращаем ошибку
		}
		// Если файла нет — не страшно, поедем на дефолтах и ENV
	}

	var cfg ServerConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// 5: Валидация конфига при старте
// совет: Приложение должно "упасть" сразу, если конфиг битый (Fail Fast).
func ValidateConfig(cfg *ServerConfig) error {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port: %d", cfg.Port)
	}
	if cfg.DB.User == "" {
		return fmt.Errorf("database user is required")
	}
	return nil
}

// 6: Прямой доступ к значениям (без Unmarshal)
// Иногда нам нужно всего одно значение, и не хочется плодить структуры.
func QuickAccess() {
	viper.Set("api_key", "secret-token-123")

	key := viper.GetString("api_key")
	port := viper.GetInt("port")

	fmt.Printf("Key: %s, Port: %d\n", key, port)
}

// 3. DYNAMIC, REMOTE, VALIDATION
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. ПРИОРИТЕТНОСТЬ (PRECEDENCE):
   Разработчик должен четко понимать иерархию "перекрытия" настроек. Стандарт:
   Флаги > ENV-переменные > Удаленный конфиг (Consul) > Локальный файл > Дефолты.

2. ВАЛИДАЦИЯ (FAIL FAST):
   Проверка конфига должна происходить ДО запуска бизнес-логики. Если в конфиге
   ошибка (например, таймаут < 0), приложение обязано упасть с четким логом.
   Это предотвращает непредсказуемое поведение в проде.

3. ДИНАМИЧЕСКАЯ ПЕРЕЗАГРУЗКА (HOT RELOAD):
   Возможность менять настройки (например, LogLevel) без рестарта подов в K8s.
   - Механизм: FSNotify (слежка за файлом) или SIGHUP (сигнал от ОС).
   - Риски: Data Race. Доступ к объекту конфига должен быть потокобезопасным.

4. REMOTE CONFIG:
   В распределенных системах конфиги хранятся в Etcd или Consul. Это позволяет
   обновить настройку сразу на 100+ микросервисах одной кнопкой.
*/

// 1: Динамическая перезагрузка (Hot Reload) через Viper
func DynamicConfigReload() {
	v := viper.New()
	v.SetConfigName("cinfig")
	v.AddConfigPath(".")
	v.SetConfigType("yaml")

	v.WatchConfig() // Запускает фоновую горутину, следящую за файлом
	v.OnConfigChange(func(in fsnotify.Event) {
		fmt.Println("Config file changed")
		// Здесь можно обновить логгер или внутренние лимиты
		// ВАЖНО: доступ к переменным должен быть потокобезопасным (используй RWMutex)
	})

	v.ReadInConfig()
}

//2: Сверхсложная валидация (Validator + Custom Tags)

type AdvancedConfig struct {
	// Обязательное поле, валидный email, не менее 10 символов
	AdminEmail string `mapstructure:"admin_email" validate:"required,email"`
	// Число в диапазоне, полезно для лимитов
	MaxConnections int `mapstructure:"max_conn" validate:"required,gt=0,lte=1000"`
	// Кастомное правило: строка должна быть одним из вариантов
	Environment string `mapstructure:"env" validate:"oneof=dev staging prod"`
}

func (c *AdvancedConfig) Validate() error {
	validate := validator.New()
	return validate.Struct(c)
}

//3: Обработка ошибок "Тихого" перекрытия

func DebugConfigPrecedence(v *viper.Viper) {
	// Viper.AllSettings() покажет итоговый "пирог" после всех слияний

	settings := v.AllSettings()
	fmt.Printf("Final Resolved Settings: %+v\n", settings)

	// Проверка: откуда пришло конкретное значение?
	if v.InConfig("database.password") {
		fmt.Println("Warning: password is set in config file, not via ENV!")
	}
}

//4: Перезагрузка по сигналу ОС (SIGHUP)

type AppConfig struct{}

func HandleSIGHUP(cfg *AppConfig) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP)

	go func() {
		for range sigs {
			fmt.Println("SIGHUP received, reloading config...")
			// Логика перечитывания файла или обращения к Consul
			newCfg, err := ReloadFromDisk()
			if err == nil {
				*cfg = *newCfg // Атомарная замена (если структура небольшая)
			}
		}
	}()
}

// 5: Remote Config (Consul/Etcd)
func LoadRemoteConfig() {
	v := viper.New()
	// Работает с Consul, Etcd или Firestore
	// ВАЖНО: требует импорта анонимного пакета выше
	err := v.AddRemoteProvider("consul", "localhost:8500", "my-service-config")
	if err != nil {
		log.Fatalf("unable to connect to remote config: %v", err)
	}

	v.SetConfigType("json") // Формат данных в Consul
	err = v.ReadRemoteConfig()

	// Можно запустить бесконечный цикл мониторинга удаленного конфига
	v.WatchRemoteConfigOnChannel()
}

// 4.EXPERT ARCHITECTURE & SECURITY (12-FACTOR, REMOTE, SECRETS)
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. ПРИНЦИПЫ "12-FACTOR APP" (CONFIG):
   Конфигурация должна быть строго отделена от кода. Один и тот же бинарник
   запускается везде (dev, staging, prod), меняются только ENV.
   Смертный грех: if env == "prod" { ... } в коде.

2. РАЗДЕЛЕНИЕ ОКРУЖЕНИЙ:
   - Dev: Локальные конфиги, моки БД, Debug логи.
   - Staging: Максимально приближено к проду, но с тестовыми ключами.
   - Prod: Только зашифрованные секреты, минимальный лог-левел, жесткие лимиты.

3. SECURITY & ENCRYPTION:
   Чувствительные данные (пароли, приватные ключи) никогда не хранятся в
   открытом виде. Используются инструменты:
   - HashiCorp Vault (стандарт индустрии).
   - AWS Secrets Manager / GCP Secret Manager.
   - SOPs (для шифрования YAML файлов в Git).

4. INFRASTRUCTURE (CONSUL/ETCD):
   Централизованное хранилище. Позволяет реализовать "Dynamic Discovery":
   сервис А узнает адрес сервиса Б из Consul, а не из хардкода.
*/

// 2: Расшифровка секретов
func DecryptSecret(cryptoKey, encryptedB24 string) (string, error) {
	key := []byte(cryptoKey) // Ключ должен быть 16, 24 или 32 байта
	ciphertext, _ := base64.StdEncoding.DecodeString(encryptedB24)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Упрощенный пример для демонстрации концепции
	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}

// 2: Интеграция с Consul
func InitRemoteProvider() {
	v := viper.New()

	// Конфиг читается из Consul по ключу "config/my-service"
	err := v.AddRemoteProvider("consul", "consul-cluster.internal:8500", "config/my-service")
	if err != nil {
		log.Fatalf("Critical: failed to connect to Consul: %v", err)
	}

	v.SetConfigType("json")
	if err := v.ReadRemoteConfig(); err != nil {
		log.Fatalf("Critical: failed to read remote config: %v", err)
	}

	// Настраиваем периодический опрос (Polling) или Watch
	go func() {
		for {
			time.Sleep(time.Minute * 5)
			if err := v.WatchRemoteConfig(); err != nil {
				log.Printf("Error watching remote config: %v", err)
				continue
			}
		}
	}()
}

// 3: Паттерн "Atomic Config" (Senior Way)
// Вместо мьютексов для чтения конфига на каждом запросе (что дорого),
// используем atomic.Value. Это дает максимально быстрое чтение.

type Config3 struct {
	MaxRequests int
	Timeout     time.Duration
}

type ConfigManager struct {
	currentConfig atomic.Value
}

func (m *ConfigManager) Get() Config3 {
	return m.currentConfig.Load().(Config3)
}

func (m *ConfigManager) Reload(newCfg Config3) {
	m.currentConfig.Store(newCfg)
}

// 4: Graceful Degenerate (Умные дефолты)
// Экспертный подход: если Consul недоступен, не падаем, а берем
// последний успешно прочитанный конфиг из локального кэша.

func LoadWithFallback(remoteUrl string) *viper.Viper {
	v := viper.New()

	// 1. Пытаемся взять из Consul
	v.AddRemoteProvider("consul", remoteUrl, "service/config")
	err := v.ReadRemoteConfig()

	if err != nil {
		log.Printf("Consul unavailable: %v. Falling back to local cache...", err)
		// 2. Фолбэк на локальный файл, который мы сохранили при прошлом успешном запуске
		v.SetConfigFile("./cache/last_working_config.yaml")
		if err := v.ReadInConfig(); err != nil {
			log.Fatal("Critical: No config sources available")
		}
	} else {
		// Сохраняем "удачный" конфиг в кэш на будущее
		v.WriteConfigAs("./cache/last_working_config.yaml")
	}
	return v
}

// 5: Провайдер-интерфейс для тестов
// Сеньор никогда не завязывает бизнес-логику на глобальный Viper.
// Мы создаем интерфейс, чтобы в тестах легко подменить конфиг моком.

type ConfigProvider interface {
	GetInt(key string) int
	GetString(key string) string
}

type PaymentService struct {
	cfg ConfigProvider
}

func NewPaymentService(p ConfigProvider) *PaymentService {
	return &PaymentService{cfg: p}
}

func (s *PaymentService) Process() {
	limit := s.cfg.GetInt("payment.limit") // В тесте вернем 100, в проде - из Consul
	fmt.Println("Limit is:", limit)
}
