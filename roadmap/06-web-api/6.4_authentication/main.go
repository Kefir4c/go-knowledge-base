package authentication

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

/*
  АУТЕНТИФИКАЦИИ
  Этот раздел дополняет базовую теорию (сессии vs JWT, OAuth2, хеширование и т.п.)
  и раскрывает тему до уровня, достаточного для уверенного ответа на любые
  вопросы на собеседовании уровня Senior.

  1.  СТРУКТУРА JWT — РАЗБОР ПО КОСТОЧКАМ
  JWT состоит из трёх частей, разделённых точкой (.):

    Header.Payload.Signature

  Каждая часть — это Base64URL-кодированный JSON (без завершающих '=').

  • HEADER — содержит как минимум:
      {
        "alg": "HS256",   // алгоритм подписи (может быть "RS256", "ES256" и т.д.)
        "typ": "JWT"      // тип токена (обычно JWT)
      }
    Может содержать дополнительные поля, например, "kid" (key ID) для указания,
    какой ключ использовать при проверке подписи.

  • PAYLOAD — содержит утверждения (claims). Делится на три категории:
      - Registered claims (зарегистрированные): предопределённые, обязательные
        для совместимости. Основные: iss (издатель), sub (субъект, обычно user ID),
        aud (аудитория, кому предназначен токен), exp (время истечения, Unix),
        iat (время выпуска), nbf (не раньше чем), jti (уникальный идентификатор).
      - Public claims — определяются сообществом, регистрируются в IANA,
        например, "email", "name".
      - Private claims — пользовательские, например, "role": "admin".

    Важно: payload не зашифрован, только подписан. Любой, кто владеет токеном,
    может прочитать его содержимое (Base64URL декодируется без ключа).
    Поэтому никогда не кладите в JWT чувствительные данные (пароль, секреты).

  • SIGNATURE — создаётся путём хеширования строки "header.payload" с использованием
    секретного ключа (для HMAC) или приватного ключа (для RSA/ECDSA). Алгоритм
    определяется в header.

  Пример создания подписи (HS256):
    HMAC-SHA256(
      base64UrlEncode(header) + "." + base64UrlEncode(payload),
      secret
    )

  2.  АЛГОРИТМЫ ПОДПИСИ: HS256 vs RS256 vs ES256
  HMAC (HS256) — симметричный: один секрет для подписи и проверки.
    + Простота, высокая скорость.
    – Секрет должен быть надёжно сохранён на всех серверах, которые проверяют токен.
    – Нельзя использовать для микросервисов, если сервисы не доверяют друг другу
      (все знают секрет, могут генерировать токены).
    Подходит для монолитов или строго внутренних систем.

  RSA (RS256) — асимметричный: приватный ключ для подписи, публичный для проверки.
    + Безопаснее, можно раздавать публичный ключ проверяющим сервисам.
    + Проверка быстрее подписи.
    – Медленнее, чем HMAC (но для большинства задач это некритично).
    Используется в OpenID Connect (Google, Keycloak) — вы получаете публичный ключ
    по URL .well-known.

  ECDSA (ES256) — асимметричный на эллиптических кривых.
    + Ещё быстрее RSA и даёт меньший размер ключа и подписи.
    – Требует больше математики, но современные библиотеки поддерживают.

  На практике:
    - Для внутреннего API с одним сервером — HS256.
    - Для внешних клиентов и микросервисов — RS256 или ES256.

  3.  ACCESS И REFRESH ТОКЕНЫ — ПОЧЕМУ ДВА?
  Одиночный токен с большим сроком жизни — рискованно: если украден, злоумышленник
  имеет доступ долгое время. Короткий токен (5–15 минут) снижает риск, но требует
  частой переаутентификации.

  Решение: пара токенов.
    - Access — короткий, используется для доступа к ресурсам.
    - Refresh — долгий, используется только для получения нового access.

  Это позволяет:
    - Держать доступный токен "на коротком поводке".
    - При утечке access-токена злоумышленник ограничен 15 минутами.
    - При компрометации refresh-токена (если он украден) мы можем его отозвать,
      т.к. храним на сервере (или в чёрном списке).
    - Refresh-токен не передаётся при каждом запросе, что снижает риск перехвата.

  Плюс, refresh-токен может быть более защищён (httpOnly cookie, SameSite=Strict),
  а access — отправляться в заголовке Authorization.

  4.  РОТАЦИЯ REFRESH-ТОКЕНОВ (REFRESH TOKEN ROTATION) — ПОДРОБНО
  Базовая схема: при /refresh выдаём новый access, старый refresh остаётся.
  Недостаток: если refresh украден, злоумышленник может бесконечно обновлять access,
  пока не истечёт срок refresh (дни или недели).

  Ротация решает это:
    1. При каждом обновлении выдаётся НОВАЯ пара (access + refresh).
    2. Старый refresh аннулируется (удаляется из хранилища или добавляется в чёрный список).
    3. Если злоумышленник попытается использовать старый refresh, он получит ошибку
       (или ему выдадут новый, но при этом владелец потеряет доступ, что сигнализирует
       о компрометации).

  Усиленный вариант: "family" или "chain" — все refresh-токены одного пользователя
  объединены в семейство. Если один из них скомпрометирован, можно отозвать всю ветку.

  Реализация на сервере:
    - Хранить список активных refresh-токенов (в Redis или БД) с user ID и временем.
    - При /refresh проверять, что токен существует, удалять его и создавать новый.
    - При logout — удалять все токены пользователя.

  5.  ОТЗЫВ JWT (INVALIDATION) — МЕТОДЫ
  Так как JWT stateless, отзыв требует дополнительных механизмов.

  • Чёрный список (blacklist) — хранить идентификаторы (jti) или хеши токенов
    в быстром хранилище (Redis) до истечения их срока действия.
    При каждом запросе проверяем, не находится ли токен в чёрном списке.
    Минус: теряем stateless, увеличиваем задержку (один запрос в Redis).

  • Версионирование токенов — хранить у пользователя поле token_version.
    В JWT кладём версию. При смене пароля или logout увеличиваем версию,
    старые токены с меньшей версией считаются недействительными.
    Проверка: сравниваем версию в токене с актуальной из БД (но это уже не stateless).

  • Короткий срок жизни + ротация — самый простой подход, если не требуется
    мгновенный отзыв. При смене пароля аннулируем refresh-токены (удаляем из БД),
    а access-токены истекают через 5–15 минут.

  6.  OAuth 2.0 ГРАНТЫ — КОГДА КАКОЙ ИСПОЛЬЗОВАТЬ
  • Authorization Code Flow (с PKCE) — самый безопасный, рекомендуется для
    веб-приложений, мобильных и SPA (с PKCE).
      - Клиент перенаправляет пользователя на сервер авторизации.
      - После аутентификации сервер выдаёт код (authorization code) и перенаправляет
        обратно на клиент (через redirect_uri).
      - Клиент обменивает код на токены (access + refresh), используя client_secret
        (для конфиденциальных клиентов) или PKCE (для публичных).
      - Код действует несколько минут и не может быть использован повторно.

  • Implicit Flow — устарел, не рекомендуется (токен выдаётся в URL-фрагменте,
    подвержен перехвату). Заменён на Authorization Code + PKCE.

  • Resource Owner Password Credentials Flow — пользователь передаёт логин/пароль
    напрямую клиенту. Используется только для доверенных клиентов (официальные
    приложения). Не рекомендуется для сторонних.

  • Client Credentials Flow — для машинного доступа (сервер-сервер).
    Клиент использует свои учётные данные (client_id + client_secret) для получения
    токена, без участия пользователя. Используется для сервисных аккаунтов.

  • Device Code Flow — для устройств с ограниченным вводом (TV, IoT).
    Пользователь переходит по ссылке на другом устройстве и вводит код.

  7.  OPENID CONNECT (OIDC) — ПОЧЕМУ ЭТО НЕ ПРОСТО OAuth2?
  OAuth2 даёт доступ к ресурсам, но не говорит, кто именно авторизовался.
  OIDC добавляет уровень аутентификации:

    - ID Token — JWT, содержащий информацию о пользователе (sub, email, name, и т.д.)
      и метаданные (время выдачи, срок, nonce). Проверяется клиентом, чтобы убедиться,
      что аутентификация прошла успешно.

    - UserInfo Endpoint — GET-запрос с access-токеном возвращает расширенную
      информацию о пользователе (если ID Token недостаточно).

    - Валидация ID Token включает проверку issuer, аудитории, срока, подписи,
      и, если используется, nonce (для защиты от replay-атак).

  OIDC используется почти всеми крупными провайдерами (Google, Microsoft, Keycloak).
  На собеседовании часто спрашивают разницу между OAuth2 и OIDC.

  8.  БЕЗОПАСНОСТЬ ПРИ ХРАНЕНИИ ТОКЕНОВ НА КЛИЕНТЕ
  • localStorage/sessionStorage (SPA)
    + Простота, доступен для JavaScript.
    – Уязвим для XSS: любой скрипт может прочитать токен и отправить его на свой сервер.
    Рекомендация: использовать только для кратковременных токенов, реализовать
    Content Security Policy (CSP) для минимизации XSS.

  • httpOnly cookie
    + Недоступен для JavaScript → защита от XSS.
    – Уязвим для CSRF (нужны anti-CSRF токены или SameSite=Strict).
    – Cookie отправляется автоматически на все запросы к домену (может быть проблема
      с кросс-доменными API).

  • Сочетание: хранить access-токен в памяти (не в хранилище), refresh-токен в httpOnly
    cookie, и обновлять access через /refresh.

  • Для мобильных приложений: использовать безопасное хранилище (Keychain/Keystore).

  9.  ЗАЩИТА ОТ CSRF И XSS — ПРАКТИЧЕСКИЕ МЕРЫ
  CSRF (Cross-Site Request Forgery):
    - Используйте SameSite=Strict или SameSite=Lax для cookie с сессией/токеном.
    - Дополнительно: anti-CSRF токены, которые передаются в заголовках (например, X-CSRF-Token).
    - Проверка Referer/Origin заголовков на сервере.
    - Для SPA с JWT в заголовке Authorization CSRF не страшен (т.к. браузер не подставляет
      этот заголовок автоматически).

  XSS (Cross-Site Scripting):
    - Экранируйте вывод в HTML.
    - Используйте Content Security Policy (CSP) для ограничения выполнения скриптов.
    - Не храните токены в localStorage, предпочитайте httpOnly cookie (для access).
    - Используйте безопасные HTTP-заголовки (X-Content-Type-Options, X-Frame-Options).

  10. ХЕШИРОВАНИЕ ПАРОЛЕЙ — BCrypt / ARGON2 В ДЕТАЛЯХ
  BCrypt:
    - Вход: пароль, соль (автоматически генерируется), стоимость (cost factor).
    - Стоимость определяет количество раундов (2^cost), влияет на скорость.
      Рекомендуется cost=10-12 для современных серверов (проверка ~0.1-0.3 секунды).
    - Хранит соль и хеш в одном строковом поле (формат: $2a$10$salt...hash...).
    - Устойчив к атакам по словарю, т.к. медленный.
    - В Go: bcrypt.GenerateFromPassword(password, cost) и CompareHashAndPassword.

  Argon2:
    - Победитель конкурса Password Hashing Competition (2015).
    - Параметры: время (time), память (memory), параллелизм (threads), длина хеша.
    - Более устойчив к GPU-атакам (памятьозависимый).
    - Сложнее в настройке, но считается наилучшим выбором для нового кода.
    - В Go: golang.org/x/crypto/argon2.

  Важно: никогда не используйте быстрые алгоритмы (MD5, SHA1, SHA256 без соли).
  Они позволяют подбирать пароли со скоростью миллиарды попыток в секунду.

  11. УПРАВЛЕНИЕ СЕКРЕТАМИ И КЛЮЧАМИ
  Секреты для подписи JWT (accessSecret, refreshSecret) должны быть:
    - Достаточной длины (не менее 32 байт для HMAC, 2048 бит для RSA).
    - Генерироваться случайным образом (не слова из словаря).
    - Храниться в переменных окружения или секретных менеджерах (Hashicorp Vault, AWS Secrets Manager).
    - Регулярно ротироваться (но это сложно, т.к. все токены, подписанные старым секретом,
      станут невалидными). Для ротации можно использовать два секрета одновременно:
      старый для проверки, новый для подписи, затем переключиться.

  Публичные ключи (для RS256) можно распространять через JWKS (JSON Web Key Set)
  — стандартный формат для передачи ключей.

  12. МОНИТОРИНГ И ЛОГИРОВАНИЕ
  Важно логировать события аутентификации:
    - Успешные и неудачные попытки входа (email, IP, время).
    - Обновление токенов.
    - Выход из системы (logout).
    - Смена пароля.
    - Необычная активность (несколько входов с разных IP, подозрительные запросы).

  Логи должны быть защищены и не содержать конфиденциальных данных (паролей, токенов).
  Используйте структурированное логирование (JSON) для упрощения анализа.

  13. RATE LIMITING ДЛЯ ЭНДПОИНТОВ АУТЕНТИФИКАЦИИ
  Чтобы защититься от брутфорса, ограничивайте количество запросов к /login и /register.
  Например, не более 5 попыток в минуту с одного IP или по email.
  После превышения — возвращайте 429 Too Many Requests.
  Реализация: обычно через middleware с использованием Redis или in-memory счётчиков.

  14. ТИПИЧНЫЕ ОШИБКИ
  • Отсутствие проверки срока действия токена (exp) — это приводит к бесконечному
    использованию токенов.
  • Использование секрета "secret" или "password" — легко подобрать.
  • Хранение секрета в коде — утечка через репозиторий.
  • Непроверка алгоритма подписи (можно использовать "none" алгоритм, если разрешён).
    Всегда проверяйте, что алгоритм соответствует ожидаемому.
  • Игнорирование аудитории (aud) — токен может быть использован в другом сервисе.
    Всегда проверяйте aud на соответствие вашему приложению.
  • Доверие к переданному JWT без проверки подписи — это полностью ломает безопасность.
  • Использование одного и того же секрета для access и refresh — логически неправильно,
    и при утечке одного секрета компрометируются оба типа.

  15. ПЕРСПЕКТИВЫ И ТРЕНДЫ
  • Пассвордлесс (passwordless) — вход по e-mail/коду или с использованием WebAuthn.
  • Децентрализованные идентификаторы (DID) и верифицируемые credentials.
  • Zero Trust — постоянная проверка каждого запроса, даже внутри периметра.
  • Машинное обучение для обнаружения аномалий в поведении пользователей.
*/

//ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ДЛЯ BCrypt И JWT

// HashPassword — хеширует пароль через bcrypt с заданной стоимостью.
func HashPassword(pass string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), 10)
	return string(hash), err
}

// CheckPassword — сравнивает пароль с хешем.
func CheckPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GenerateJWT — создаёт JWT с пользовательскими данными.
func GenerateJWT(userID int, secret string, expiry time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"uid": userID,
		"exp": time.Now().Add(expiry).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseJWT — парсит и валидирует токен.
func ParseJWT(tokenString, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// ПРИМЕР 1: ХЕШИРОВАНИЕ ПАРОЛЕЙ С BCRYPT
/*
  Что показывает:
    - Как bcrypt генерирует хеш с солью.
    - Как сравнивать пароль с хешем.
    - Что один и тот же пароль даёт разные хеши (из-за соли).
    - Что проверка занимает время (замедляет перебор).

  Почему это важно:
    На собеседовании обязательно спросят про хеширование паролей. Вы должны
    объяснить, почему bcrypt/argon2, а не MD5/SHA1, и показать пример.

  Фишки:
    - Используем bcrypt.DefaultCost (10) — хороший баланс.
    - Обрабатываем ошибки.
    - Выводим хеши и время проверки.
*/

func primer1() {
	fmt.Println("\n=== ПРИМЕР 1: Хеширование паролей с bcrypt ===")

	password := "SoskaNaByblike337"

	// Хешируем
	hash1, err := HashPassword(password)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Пароль: %s\nХеш 1: %s\n", password, hash1)

	// Хешируем ещё раз — хеш другой из-за соли
	hash2, _ := HashPassword(password)
	fmt.Printf("Хеш 2: %s\n", hash2)

	// Проверяем
	err = CheckPassword(password, hash1)
	fmt.Printf("Проверка правильного пароля: %v\n", err == nil)

	err = CheckPassword("wrong", hash1)
	fmt.Printf("Проверка неправильного пароля: %v\n", err != nil)

	err = CheckPassword(hash2, hash1)
	fmt.Printf("Проверяем для хэша: %v\n", err != nil)

	// Замер времени (показывает, что bcrypt медленный)
	start := time.Now()
	HashPassword(password)
	fmt.Printf("Время хеширования: %v\n", time.Since(start))
}

// ПРИМЕР 2: ГЕНЕРАЦИЯ И ПРОВЕРКА JWT

/*
  Что показывает:
    - Как создать JWT с пользовательскими данными (uid) и сроком.
    - Как распарсить и проверить подпись.
    - Как извлечь данные из токена.

  Почему это важно:
    Понимание устройства JWT — база для любых систем с токенами.
    На собеседовании могут попросить написать генерацию и проверку.

  Фишки:
    - Используем HS256 (симметричный) для простоты.
    - Добавляем exp и iat.
    - Показываем, что токен с изменённой подписью не проходит проверку.
*/

func primer2() {
	fmt.Println("\n=== ПРИМЕР 2: Генерация и проверка JWT ===")

	secret := "my-secret-key"
	userID := 42
	expiry := 15 * time.Minute

	// Генерируем токен
	token, err := GenerateJWT(userID, secret, expiry)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Сгенерированный токен:\n%s\n", token)

	// Парсим и проверяем
	claims, err := ParseJWT(token, secret)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Проверка успешна. Claims: %v\n", claims)
	fmt.Printf("UserID: %v (тип %T)\n", claims["uid"], claims["uid"])

	// Попробуем с неправильным секретом
	_, err = ParseJWT(token, "wrong-secret")
	fmt.Printf("Проверка с неправильным секретом: %v\n", err)

	// Попробуем изменить токен (подпись не совпадёт)
	parts := strings.Split(token, ".")
	if len(parts) == 3 {
		fake := parts[0] + "." + parts[1] + "." + "fake"
		_, err = ParseJWT(fake, secret)
		fmt.Printf("Проверка поддельного токена: %v\n", err)
	}
}

// ПРИМЕР 3: MIDDLEWARE ДЛЯ ПРОВЕРКИ ТОКЕНА

/*
  Что показывает:
    - Как написать middleware, который проверяет токен в заголовке Authorization.
    - Как добавлять userID в контекст для последующих хендлеров.
    - Как возвращать 401 при отсутствии или невалидности токена.

  Почему это важно:
    Это стандартный паттерн для защиты эндпоинтов.
    На собеседовании часто просят реализовать проверку JWT в middleware.

  Фишки:
    - Извлечение токена из "Bearer <token>".
    - Использование context.WithValue для передачи данных.
    - Чистая обработка ошибок с понятными статусами.
*/

// Пример middleware, который проверяет токен и кладёт userID в контекст.
func AuthMiddlewares(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHandler := r.Header.Get("Authorization")
		if authHandler == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}
		parts := strings.SplitN(authHandler, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}
		claims, err := ParseJWT(parts[1], secret)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}
		// Извлекаем userID (может быть float64 из-за JSON)
		uid, ok := claims["uid"].(float64)
		if !ok {
			http.Error(w, "Invalid token claims", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), "userID", int(uid))
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// protectedHandler — пример защищённого эндпоинта.
func protectedHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": fmt.Sprintf("Hello user %d, you are authenticated!", userID),
		"user_id": userID,
	})
}

func primer3() {
	fmt.Println("\n=== ПРИМЕР 3: Middleware для проверки JWT ===")

	secret := "my-secret-key"
	mux := http.NewServeMux()

	// Защищённый эндпоинт
	mux.HandleFunc("GET /protected", AuthMiddlewares(secret, protectedHandler))

	// Также добавим эндпоинт для генерации токена (для теста)
	mux.HandleFunc("GET /generate", func(w http.ResponseWriter, r *http.Request) {
		token, _ := GenerateJWT(123, secret, 15*time.Minute)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": token})
	})

	addr := ":8090"
	srv := &http.Server{Addr: addr, Handler: mux}

	fmt.Printf("Запуск примера 3 на http://localhost%s\n", addr)
	fmt.Println("Эндпоинты:")
	fmt.Println("  GET /generate  — получить тестовый токен")
	fmt.Println("  GET /protected — защищённый (требует Bearer токен)")
	fmt.Println("Для остановки нажмите Enter")

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	fmt.Scanln()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

//ПРИМЕР 4: OAuth2 имитация

/*
  В реальном проекте вы бы делали HTTP-запросы к Google:
    - GET /auth/login -> редирект на https://accounts.google.com/o/oauth2/v2/auth
    - После подтверждения пользователь возвращается на /auth/callback с code
    - POST /oauth/token для обмена кода на токены
    - GET /oauth/userinfo для получения данных

  Здесь мы имитируем все шаги локально, чтобы показать логику.
*/

// OAuth2Config — настройки провайдера (имитация Google)
type OAuth2Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string // локальный эндпоинт для "подтверждения"
	TokenURL     string // локальный эндпоинт для обмена кода
	UserInfoURL  string // локальный эндпоинт для получения данных
}

// OAuth2State — хранилище состояний (для защиты от CSRF)
var oauthStates = struct {
	sync.RWMutex
	data map[string]string // state -> redirect_uri (или userID)
}{data: make(map[string]string)}

// generateState генерирует случайную строку для state
func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// OAuth2Handlers — обработчики для OAuth2 потока
type OAuth2Handlers struct {
	config    *OAuth2Config
	userStore *InMemoryUserRepo // используем из примера 4 (или свою реализацию)
	svc       *AuthService      // используем сервис из примера 4 для выдачи токенов
}

func NewOAuth2Handlers(cfg *OAuth2Config, userStore *InMemoryUserRepo, svc *AuthService) *OAuth2Handlers {
	return &OAuth2Handlers{config: cfg, userStore: userStore, svc: svc}
}

// Login — перенаправляет на страницу авторизации провайдера (имитация)
func (h *OAuth2Handlers) Login(w http.ResponseWriter, r *http.Request) {
	// Генерируем state и сохраняем
	state := generateState()
	oauthStates.Lock()
	oauthStates.data[state] = "default" // можно сохранить redirect_uri
	oauthStates.Unlock()

	// Строим URL авторизации (в реальности это внешний URL)
	// Здесь мы просто перенаправляем на локальный эндпоинт "авторизации"
	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&state=%s&scope=email profile",
		h.config.AuthURL, h.config.ClientID, h.config.RedirectURL, state)

	http.Redirect(w, r, authURL, http.StatusFound)
}

// Authorize — имитация страницы авторизации провайдера (пользователь подтверждает доступ)
func (h *OAuth2Handlers) Authorize(w http.ResponseWriter, r *http.Request) {
	// Получаем параметры
	redirectURI := r.URL.Query().Get("redirect_uri")
	state := r.URL.Query().Get("state")
	// В реальном проекте здесь бы была страница входа и подтверждения
	// Мы просто имитируем, что пользователь нажал "Разрешить"

	// Проверяем state (защита от CSRF)
	oauthStates.RLock()
	_, ok := oauthStates.data[state]
	oauthStates.RUnlock()
	if !ok {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}
	// Удаляем state после использования
	oauthStates.Lock()
	delete(oauthStates.data, state)
	oauthStates.Unlock()

	// Генерируем код авторизации (произвольная строка)
	code := base64.URLEncoding.EncodeToString([]byte(uuid.New().String()))

	// Перенаправляем обратно с кодом
	redirectURL := fmt.Sprintf("%s?code=%s&state=%s", redirectURI, code, state)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// Callback — обработка кода, обмен на токены
func (h *OAuth2Handlers) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "Missing code or state", http.StatusBadRequest)
		return
	}
	// Проверяем state (если не удалён ранее)
	oauthStates.RLock()
	_, ok := oauthStates.data[state]
	oauthStates.RUnlock()
	if !ok {
		// state может быть уже удалён, но если мы его удалили в Authorize,
		// то здесь он уже не существует; для демонстрации пропустим проверку
	}
	// В реальности здесь нужно отправить POST запрос к /token с code, client_id, client_secret
	// и получить access_token, refresh_token, id_token.

	// Имитируем получение токенов
	// Для простоты создадим пользователя в нашей системе (по email)
	// Предположим, что провайдер вернул email пользователя.
	// В реальности мы бы получили его из ID Token или UserInfo Endpoint.
	email := "oauth_user@example.com" // В реальности из ответа провайдера

	// Проверяем, существует ли пользователь; если нет — создаём
	user, ok := h.userStore.GetUserByEmail(email)
	if !ok {
		// Регистрируем пользователя с временным паролем (не используется)
		hashed, _ := bcrypt.GenerateFromPassword([]byte("oauth_tmp"), bcrypt.DefaultCost)
		user, _ = h.userStore.CreateUser(email, string(hashed))
	}

	// Генерируем свои access/refresh токены (как в примере 4)
	access, _ := h.svc.GenerateAccessToken(user.ID)
	refresh, _ := h.svc.GenerateRefreshToken(user.ID)

	// Возвращаем токены в JSON (или можно установить cookie)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token":  access,
		"refresh_token": refresh,
		"token_type":    "Bearer",
		"id_token":      "имитация ID Token", // обычно это JWT с данными пользователя
	})
}

// UserInfo — имитация эндпоинта получения информации о пользователе (для OIDC)
func (h *OAuth2Handlers) UserInfo(w http.ResponseWriter, r *http.Request) {
	// В реальности проверяем access_token и возвращаем данные
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"sub":   "12345",
		"email": "oauth_user@example.com",
		"name":  "OAuth User",
	})
}

// Запуск примера 5
func primer4() {
	fmt.Println("\n=== ПРИМЕР 5: OAuth2 / OpenID Connect (имитация) ===")

	// Настройки (как бы от Google)
	cfg := &OAuth2Config{
		ClientID:     "your-client-id",
		ClientSecret: "your-client-secret",
		RedirectURL:  "http://localhost:8091/auth/callback",
		AuthURL:      "http://localhost:8091/oauth/authorize", // локальная имитация
		TokenURL:     "http://localhost:8091/oauth/token",     // не используется в этой демо
		UserInfoURL:  "http://localhost:8091/oauth/userinfo",
	}

	// Хранилища и сервис (из примера 4)
	userRepo := NewInMemoryUserRepo()
	cfgSvc := &Config{
		AccessSecret:       "oauth-access-secret",
		RefreshSecret:      "oauth-refresh-secret",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		BcryptCost:         12,
	}
	refreshRepo := NewInMemoryRefreshRepo()
	svc := NewAuthService(cfgSvc, userRepo, refreshRepo)
	handlers := NewOAuth2Handlers(cfg, userRepo, svc)

	mux := http.NewServeMux()
	// Эндпоинты OAuth2
	mux.HandleFunc("GET /auth/login", handlers.Login)
	mux.HandleFunc("GET /oauth/authorize", handlers.Authorize) // имитация страницы провайдера
	mux.HandleFunc("GET /auth/callback", handlers.Callback)
	mux.HandleFunc("GET /oauth/userinfo", handlers.UserInfo)

	// Защищённый эндпоинт (для демонстрации)
	mux.HandleFunc("GET /protected", AuthMiddleware(svc, func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value("userID").(int)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Protected via OAuth2 login",
			"user_id": uid,
		})
	}))

	addr := ":8091"
	srv := &http.Server{Addr: addr, Handler: mux}

	fmt.Printf("OAuth2 сервер запущен на http://localhost%s\n", addr)
	fmt.Println("Эндпоинты:")
	fmt.Println("  GET /auth/login          - начало входа через Google (имитация)")
	fmt.Println("  GET /oauth/authorize     - страница подтверждения (локальная)")
	fmt.Println("  GET /auth/callback       - обработка кода (редирект)")
	fmt.Println("  GET /protected           - защищённый ресурс (токен из примера 4)")
	fmt.Println("Нажмите Enter для остановки")

	go srv.ListenAndServe()
	fmt.Scanln()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

// ПРИМЕР 6: ДВУХФАКТОРНАЯ АУТЕНТИФИКАЦИЯ (TOTP)

/*
  ПРИМЕР 6: TOTP (Google Authenticator)
  Что показывает:
    - Как генерировать секрет для пользователя.
    - Как создавать QR-код для сканирования в приложении.
    - Как проверять одноразовые коды.
    - Как интегрировать TOTP в процесс логина (после успешного ввода пароля).

  Почему это важно:
    Двухфакторная аутентификация — стандарт безопасности для серьёзных приложений.
    На собеседовании могут спросить, как реализовать 2FA.

  Фишки:
    - Используем библиотеку github.com/pquerna/otp/totp для генерации и проверки.
    - Храним секрет в БД (в примере — в памяти).
    - Генерируем QR-код в виде data URL (base64 PNG) для отображения в браузере.
    - Добавляем эндпоинт /setup-totp для активации и /verify-totp для проверки.
    - После проверки выдаём токены (как в примере 4).
*/

// UserTOTPSecret хранит TOTP секрет для пользователя
type UserTOTPSecret struct {
	Secret  string
	Enabled bool
}

var totpSecrets = struct {
	sync.RWMutex
	data map[int]UserTOTPSecret
}{data: make(map[int]UserTOTPSecret)}

// TOTPHandlers — обработчики для TOTP
type TOTPHandlers struct {
	userRepo *InMemoryUserRepo
	svc      *AuthService
}

func NewTOTPHandlers(userRepo *InMemoryUserRepo, svc *AuthService) *TOTPHandlers {
	return &TOTPHandlers{userRepo: userRepo, svc: svc}
}

// SetupTOTP — генерирует секрет и возвращает QR-код (для активации)
func (h *TOTPHandlers) SetupTOTP(w http.ResponseWriter, r *http.Request) {
	// Предполагаем, что пользователь уже аутентифицирован (по access токену)
	uid, ok := r.Context().Value("userID").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Генерируем секрет для пользователя
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "MyApp",
		AccountName: fmt.Sprintf("user-%d", uid),
	})
	if err != nil {
		http.Error(w, "Failed to generate TOTP secret", http.StatusInternalServerError)
		return
	}

	// Сохраняем секрет
	totpSecrets.Lock()
	totpSecrets.data[uid] = UserTOTPSecret{Secret: key.Secret(), Enabled: false}
	totpSecrets.Unlock()

	// Генерируем QR-код в виде PNG
	img, err := key.Image(200, 200)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		http.Error(w, "Failed to encode QR code", http.StatusInternalServerError)
		return
	}
	qrBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"secret": key.Secret(),
		"qr":     "data:image/png;base64," + qrBase64,
	})
}

// VerifyTOTP — проверяет код и активирует TOTP для пользователя
func (h *TOTPHandlers) VerifyTOTP(w http.ResponseWriter, r *http.Request) {
	uid, ok := r.Context().Value("userID").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		http.Error(w, "Missing code", http.StatusBadRequest)
		return
	}

	totpSecrets.RLock()
	secretData, exists := totpSecrets.data[uid]
	totpSecrets.RUnlock()
	if !exists {
		http.Error(w, "TOTP not set up for user", http.StatusBadRequest)
		return
	}

	// Проверяем код
	if !totp.Validate(req.Code, secretData.Secret) {
		http.Error(w, "Invalid TOTP code", http.StatusUnauthorized)
		return
	}

	// Активируем TOTP для пользователя
	totpSecrets.Lock()
	secretData.Enabled = true
	totpSecrets.data[uid] = secretData
	totpSecrets.Unlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "TOTP enabled successfully"})
}

// LoginWithTOTP — логин, требующий TOTP (после проверки пароля)
// В реальности вы бы вызывали этот эндпоинт после успешного ввода пароля,
// либо включали проверку TOTP в основной логин.
func (h *TOTPHandlers) LoginWitnTOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	user, ok := h.userRepo.GetUserByEmail(req.Email)
	if !ok {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Проверяем TOTP, если включён
	totpSecrets.RLock()
	secretDate, exists := totpSecrets.data[user.ID]
	totpSecrets.RUnlock()
	if exists && secretDate.Enabled {
		if !totp.Validate(req.Code, secretDate.Secret) {
			http.Error(w, "Invalid TOTP code", http.StatusUnauthorized)
			return
		} else {
			http.Error(w, "TOTP not enabled for this user", http.StatusBadRequest)
			return
		}

		// Выдаём токены
		access, _ := h.svc.GenerateAccessToken(user.ID)
		refresh, _ := h.svc.GenerateRefreshToken(user.ID)
		w.Header().Set("Conected-Tepe", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token":  access,
			"refresh_token": refresh,
			"token_type":    "Bearer",
		})
	}
}

// Запуск примера 5
func primer5() {
	fmt.Println("\n=== ПРИМЕР 6: TOTP (двухфакторная аутентификация) ===")

	userRepo := NewInMemoryUserRepo()
	cfgSvc := &Config{
		AccessSecret:       "totp-access-secret",
		RefreshSecret:      "totp-refresh-secret",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		BcryptCost:         12,
	}
	refreshRepo := NewInMemoryRefreshRepo()
	svc := NewAuthService(cfgSvc, userRepo, refreshRepo)
	handlers := NewTOTPHandlers(userRepo, svc)

	// Создаём тестового пользователя (для удобства)
	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	userRepo.CreateUser("test@example.com", string(hashed))

	mux := http.NewServeMux()
	// Эндпоинты для TOTP (требуют аутентификации)
	mux.HandleFunc("POST /setup-totp", AuthMiddleware(svc, handlers.SetupTOTP))
	mux.HandleFunc("POST /verify-totp", AuthMiddleware(svc, handlers.VerifyTOTP))

	// Также добавим эндпоинт для получения токена (для теста setup)
	mux.HandleFunc("GET /generate-token", func(w http.ResponseWriter, r *http.Request) {
		// Просто выдаём токен для тестового пользователя, чтобы можно было вызвать /setup-totp
		user, _ := userRepo.GetUserByEmail("test@example.com")
		access, _ := svc.GenerateAccessToken(user.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": access})
	})

	addr := ":8092"
	srv := &http.Server{Addr: addr, Handler: mux}

	fmt.Printf("TOTP сервер запущен на http://localhost%s\n", addr)
	fmt.Println("Эндпоинты:")
	fmt.Println("  GET /generate-token     - получить access токен для тестового пользователя")
	fmt.Println("  POST /setup-totp        - сгенерировать секрет и QR (требует Bearer токен)")
	fmt.Println("  POST /verify-totp       - проверить код и активировать TOTP (требует Bearer)")
	fmt.Println("  POST /login-totp        - логин с паролем и TOTP кодом (публичный)")
	fmt.Println("Нажмите Enter для остановки")

	go srv.ListenAndServe()
	fmt.Scanln()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
