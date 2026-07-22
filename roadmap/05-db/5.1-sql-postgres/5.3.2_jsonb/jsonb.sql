/*
ШАГ 3.2: JSONB И GIN-ИНДЕКСЫ — ПОЛНЫЙ КУРС ДЛЯ MIDDLE/SENIOR

ЧАСТЬ 1. ТЕОРИЯ

1.1. ВНУТРЕННЕЕ УСТРОЙСТВО JSONB

Как JSONB хранится на диске?
  - JSONB хранится в бинарном формате (разработан специально для PostgreSQL).
  - При вставке JSONB парсится и нормализуется:
    * Удаляются пробелы и лишние символы.
    * Дублирующиеся ключи схлопываются (остаётся последний).
    * Ключи сортируются (для быстрого бинарного поиска).
    * Числа преобразуются в бинарное представление.
  - Бинарное представление позволяет извлекать значения без повторного парсинга (в отличие от JSON).

1.1.1.
Добавлю важный нюанс про сжатие:
JSONB, как и любой большой объект в PostgreSQL, может быть сжат через механизм TOAST (The Oversized-Attribute Storage Technique).
Если JSONB-документ превышает ~2 КБ, он автоматически сжимается и хранится отдельно от основной строки таблицы.

Как это влияет на производительность:
Чтение сжатого JSONB требует распаковки, что добавляет накладные расходы.
Если ты часто читаешь большие JSONB-документы целиком, это может быть медленно.

Для часто читаемых больших документов хранить их можно в несжатом виде,
изменяя параметры хранения колонки (ALTER TABLE ... SET STORAGE EXTERNAL)

1.2. JSON vs JSONB — ПОЛНОЕ СРАВНЕНИЕ

  Характеристика          | JSON                     | JSONB
  ------------------------|--------------------------|------------------------
  Хранение                | Текст (как есть)         | Бинарное, нормализованное
  Сохранение порядка ключей| Да                       | Нет
  Поддержка индексов      | Нет (только full scan)   | Да (GIN, BTREE на выражения)
  Скорость вставки        | Быстрее (меньше обработки)| Медленнее (парсинг + нормализация)
  Скорость поиска         | Медленнее (постоянно парсится)| Быстрее (бинарный поиск)
  Поддержка операторов    | Только ->, ->>           | Все операторы
  Размер на диске         | Меньше (если сжимается)  | Обычно больше (из-за нормализации)
  Использование           | Архивы, логи             | OLTP, частые запросы

1.3. ОПЕРАТОРЫ РАБОТЫ С JSONB

  ->        — получить значение как JSON (сохраняет тип)
  ->>       — получить значение как текст (SQL текст)
  #>        — получить вложенный объект по пути (массив ключей)
  #>>       — получить вложенный объект как текст
  @>        — содержит ли левый JSON правый (проверка вхождения)
  <@        — является ли левый JSON подмножеством правого
  ?         — существует ли ключ (для объектов) или элемент (для массивов)
  ?|        — существует ли хотя бы один из ключей
  ?&        — существуют ли все ключи
  ||        — объединение двух JSONB объектов
  -         — удаление ключа (или элемента массива)
  #-        — удаление по пути

1.4. ВНУТРЕННЕЕ УСТРОЙСТВО GIN-ИНДЕКСА

  GIN (Generalized Inverted Index) — это индекс, который строит обратный индекс
  для составных значений (как поисковый движок).

  Как работает GIN для JSONB:
    - При создании индекса USING GIN (settings) PostgreSQL анализирует все
      JSONB-документы и строит индекс по ключам и значениям.
    - Для каждого ключа/значения хранится список ID строк (TID), где они встречаются.
    - При поиске @> (содержит) индекс быстро находит все строки, которые
      содержат указанный JSON-объект.

  Два типа GIN-индексов для JSONB:
    1. USING GIN (settings) — стандартный, поддерживает все операторы.
    2. USING GIN (settings jsonb_path_ops) — оптимизирован для @>,
       занимает меньше места, но не поддерживает ?, ?|, ?&.

  Когда использовать jsonb_path_ops:
    - Если используются только запросы @> (проверка вхождения).
    - Если важна экономия места на диске.
    - Если не нужны операторы существования ключей (?).
    * Размер индекса с jsonb_path_ops обычно на 30-50% меньше, чем с обычным GIN.
    * Скорость поиска по @> с jsonb_path_ops может быть выше на 20-30%.

1.5. ИНДЕКСАЦИЯ КОНКРЕТНЫХ ПОЛЕЙ

  Помимо GIN можно создавать обычные BTREE-индексы на конкретные поля:

  CREATE INDEX idx_email ON notification_settings ((settings->>'email'));
  CREATE INDEX idx_language ON notification_settings ((settings->>'language'));

  Это полезно, если:
    - Поле часто используется в WHERE с оператором =.
    - Нужна уникальность (UNIQUE INDEX).
    - Нужна сортировка (ORDER BY).

1.5.1
ИНДЕКСАЦИЯ КОНКРЕТНЫХ ПОЛЕЙ — добавим нюанс про NULL и COALESCE
Добавлю важный нюанс:
Если у тебя есть поле, которое часто бывает NULL, и ты ищешь по нему,
индекс будет работать только для ненулевых значений.

Как обойти:
-- Индекс, который учитывает NULL
CREATE INDEX idx_email_not_null ON notification_settings ((settings->>'email')) WHERE settings->>'email' IS NOT NULL;

Или использовать COALESCE для подстановки значения по умолчанию:
CREATE INDEX idx_email_coalesce ON notification_settings ((COALESCE(settings->>'email', '')));

1.6. ВАЛИДАЦИЯ JSONB СХЕМЫ

  Можно добавить CHECK-ограничение для валидации структуры JSONB:

  ALTER TABLE notification_settings
  ADD CONSTRAINT valid_settings CHECK (
    jsonb_typeof(settings) = 'object' AND
    (NOT settings ? 'email' OR jsonb_typeof(settings->'email') = 'boolean') AND
    (NOT settings ? 'channels' OR jsonb_typeof(settings->'channels') = 'object')
  );

  Также можно использовать расширение pg_jsonschema (требует установки).

1.7. ПОЛНОТЕКСТОВЫЙ ПОИСК В JSONB

  Можно комбинировать GIN с полнотекстовым поиском:

  CREATE INDEX idx_fts ON notifications USING GIN (
    to_tsvector('english',
      COALESCE(data->>'subject', '') || ' ' || COALESCE(data->>'body', '')
    )
  );

1.8. ПРОИЗВОДИТЕЛЬНОСТЬ JSONB

  Преимущества:
    - Гибкость схемы (не нужен ALTER TABLE для новых полей).
    - Быстрый поиск по индексам.
    - Компактное хранение (бинарное).

  Недостатки:
    - Нет строгой типизации (ошибки на уровне приложения).
    - Медленнее, чем обычные колонки (на 20-50%).
    - Больший размер на диске (по сравнению с простыми колонками).
    - Сложнее оптимизировать (требует понимания GIN).

1.9. КОГДА ИСПОЛЬЗОВАТЬ JSONB

  ✅ Хорошо:
    - Гибкая схема (настройки, метаданные).
    - Атрибуты, которые редко используются в WHERE.
    - Хранение внешних данных (API-ответы).
    - Логи, события, аудит.
    - Прототипирование (быстрая смена структуры).
    - JSONB отлично подходит для реализации EAV-подобных моделей, когда у разных сущностей разные атрибуты.

  ❌ Плохо:
    - Часто обновляемые поля (накладные расходы).
    - Поля, которые часто используются в JOIN (лучше отдельная таблица).
    - Высоконагруженные запросы с фильтрацией по JSONB (медленнее, чем обычные колонки).
    - Когда нужна строгая типизация.

1.10. СВЯЗЬ С GO

  В Go работа с JSONB реализуется через стандартный пакет encoding/json:

    type Settings struct {
        Email    bool     `json:"email"`
        Push     bool     `json:"push"`
        Channels Channels `json:"channels,omitempty"`
    }

  При вставке:
    data, _ := json.Marshal(settings)
    db.Exec(ctx, "INSERT INTO table (settings) VALUES ($1::JSONB)", data)

  При чтении:
    var raw json.RawMessage
    row.Scan(&raw)
    json.Unmarshal(raw, &settings)
*/

--ЧАСТЬ 2. ПРАКТИКА (ОСНОВНАЯ)

-- 2.1. ПОДГОТОВКА ДАННЫХ

DROP TABLE IF EXISTS notification_settings CASCADE;

CREATE TABLE notification_settings (
    id BIGSERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    settings JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Вставляем тестовые данные с разными настройками
INSERT INTO notification_settings (user_id, settings) VALUES
    (1, '{
        "email": true,
        "push": true,
        "sms": false,
        "channels": {
            "telegram": true,
            "whatsapp": false
        },
        "quiet_hours": {
            "start": "22:00",
            "end": "08:00"
        },
        "tags": ["urgent", "important"]
    }'),
    (2, '{
        "email": false,
        "push": true,
        "sms": true,
        "channels": {
            "telegram": false,
            "whatsapp": true
        },
        "language": "ru",
        "tags": ["regular"]
    }'),
    (3, '{
        "email": true,
        "push": false,
        "sms": false,
        "channels": {
            "telegram": true,
            "whatsapp": true
        },
        "timezone": "UTC+3",
        "tags": ["urgent", "regular", "promo"]
    }'),
    (4, '{
        "email": true,
        "push": true,
        "sms": true,
        "channels": {
            "telegram": false,
            "whatsapp": false
        },
        "quiet_hours": {
            "start": "23:00",
            "end": "07:00"
        },
        "tags": ["promo"]
    }'),
    (5, '{}'::JSONB);  -- пустые настройки (по умолчанию)

-- 2.2. ОПЕРАТОРЫ -> И ->>
-- 2.2.1. -> возвращает JSON (сохраняет тип)
SELECT 
    user_id,
    settings -> 'email' AS email_json,
    settings -> 'channels' AS channels_json,
    settings -> 'tags' AS tags_json
FROM notification_settings;

-- 2.2.2. ->> возвращает TEXT (SQL строка)
SELECT
    user_id,
    settings ->> 'email' AS email_text,
    settings ->> 'language' AS language_text
FROM notification_settings;

-- 2.2.3. Сравнение -> и ->>
SELECT
    user_id,
    settings -> 'email' AS email_json,    -- true (булево значение в JSON)
    settings ->> 'email' AS email_text    -- 'true' (строка)
FROM notification_settings;

-- 2.2.4. Извлечение из вложенного объекта (-> и ->> цепочкой)
SELECT
    user_id,
    settings -> 'channels' -> 'telegram' AS telegram_json,
    settings -> 'channels' ->> 'telegram' AS telegram_text,
    settings -> 'quiet_hours' ->> 'start' AS quiet_start
FROM notification_settings;

-- 2.3. ОПЕРАТОРЫ #> И #>> (ПУТИ)
-- 2.3.1. #> извлекает вложенный объект по пути (массив ключей)
SELECT
    user_id,
    settings #> '{channels, telegram}' AS telegram_json,
    settings #> '{quiet_hours, start}' AS quiet_start_json
FROM notification_settings;

-- 2.3.2. #>> извлекает как текст
SELECT
    user_id,
    settings #>> '{channels, telegram}' AS telegram_text,
    settings #>> '{quiet_hours, start}' AS quiet_start_text
FROM notification_settings;

-- 2.4. ОПЕРАТОР @> (СОДЕРЖИТ)
-- 2.4.1. Проверка наличия конкретного JSON объекта
SELECT
    user_id,
    settings
FROM notification_settings
WHERE settings @> '{"email": true}';

-- 2.4.2. Проверка наличия вложенного объекта
SELECT
    user_id,
    settings
FROM notification_settings
WHERE settings @> '{"channels": {"telegram": true}}';

-- 2.4.3. Проверка наличия массива с элементом
SELECT
    user_id,
    settings
FROM notification_settings
WHERE settings @> '"{tags": ["urgent"]}';

-- 2.4.4. Проверка нескольких условий в одном @>
SELECT
    user_id,
    settings
FROM notification_settings
WHERE settings @> '{"email": true, "push": true}';

-- 2.4.5. Обратный оператор <@ (является подмножеством)
SELECT
    user_id,
    settings
FROM notification_settings
WHERE '{"email": true, "push": true}'::JSONB <@ settings;
-- Найдёт только тех, у кого есть и email, и push (среди других полей)

-- 2.5. ОПЕРАТОРЫ ? (СУЩЕСТВОВАНИЕ КЛЮЧА)
-- 2.5.1. ? — существует ли ключ в объекте
SELECT
    user_id,
    settings
FROM notification_settings
WHERE settings ? 'language';

-- 2.5.2. ? — существует ли элемент в массиве
SELECT
    user_id,
    settings
FROM notification_settings
WHERE settings -> 'tags' ? 'urgent';

-- 2.5.3. ?| — существует ли хотя бы один ключ из списка
SELECT
    user_id,
    settings
FROM notification_settings
WHERE settings ?| ARRAY['language', 'timezone'];

-- 2.5.4. ?& — существуют ли все ключи из списка
SELECT
    user_id,
    settings
FROM notification_settings
WHERE  settings ?& ARRAY['email', 'push', 'channels'];

-- 2.6. GIN-ИНДЕКСЫ

-- 2.6.1. Создаём GIN-индекс на всё JSONB поле
CREATE INDEX idx_notification_settings_gin ON notification_settings USING GIN (settings);

-- 2.6.2. Более компактный индекс (jsonb_path_ops) — только для @>
CREATE INDEX idx_notification_settings_gin_path_ops ON notification_settings USING GIN (settings jsonb_path_ops);

CREATE INDEX idx_notification_settings_email ON notification_settings ((settings ->> 'email'));

CREATE INDEX idx_notification_settings_telegram ON notification_settings ((settings #> '{channels, telegram}'));

-- 2.6.4. Демонстрация работы индекса
EXPLAIN(ANALYZE, BUFFERS)
SELECT user_id, settings
FROM notification_settings
WHERE settings @> '{"email": true}';
-- Должен быть Bitmap Index Scan или Index Scan


-- 2.6.5. Индекс не используется, если функция на JSONB
EXPLAIN (ANALYZE, BUFFERS)
SELECT user_id, settings
FROM notification_settings
WHERE settings->>'email' = 'true';
-- Seq Scan! (потому что используется ->>, а не @>)

-- 2.6.6. Чтобы использовать индекс, создаём выражение
CREATE INDEX idx_notification_settings_email_text ON notification_settings ((settings->>'email'));
EXPLAIN (ANALYZE, BUFFERS)
SELECT user_id, settings
FROM notification_settings
WHERE settings->>'email' = 'true';
-- Теперь используется Index Scan!

-- 2.7. РАБОТА С JSONB (ОБНОВЛЕНИЕ, УДАЛЕНИЕ, ДОБАВЛЕНИЕ)
-- 2.7.1. Добавление нового поля
UPDATE notification_settings
SET settings = settings || '{"push": true}'::JSONB
WHERE user_id = 5;

-- 2.7.2. Обновление существующего поля
UPDATE notification_settings
SET settings = settings || '{"email": false}'::JSONB
WHERE user_id = 2;

-- 2.7.3. Обновление вложенного поля (используем jsonb_set)
UPDATE notification_settings
SET settings = jsonb_set(settings, '{channels, telegram}, 'true'::JSONB')
WHERE user_Id = 1;

-- 2.7.4. Обновление нескольких полей через jsonb_set
UPDATE notification_settings
SET settings = jsonb_set(
    jsonb_set(settings, '{quiet_hours, start}', '"21:00"'::JSONB),
    '{quiet_hours, end}', '"06:00"'::JSONB
)
WHERE user_id = 1;

-- 2.7.5. Удаление ключа (оператор -)
UPDATE notification_settings
SET settings = settings - 'language'      -- удаляем поле language
WHERE user_id = 2;

-- 2.7.6. Удаление вложенного ключа (#-)
UPDATE notification_settings
SET settings = settings #- '{channels, whatsapp}'
WHERE user_id = 1;

-- 2.7.7. Добавление элемента в массив (используем ||)
UPDATE notification_settings
SET settings = jsonb_set(
    settings,
    '{tags}',
    (settings -> 'tags') || '["critical"]'::JSONB
)
WHERE user_id = 1;

-- 2.7.8. Удаление элемента из массива (через -)
UPDATE notification_settings
SET settings = jsonb_set(
    settings,
    '{tags}',
    (settings -> 'tags') - 'regulaer'::TEXT:JSONB
)
WHERE user_id = 2;

-- 2.8. СЛОЖНЫЕ ЗАПРОСЫ С JSONB
-- 2.8.1. Поиск по нескольким условиям (AND)
SELECT user_id, settings
FROM notification_settings
WHERE settings @> '{"email": true}'
    AND settings @> '{"push": true}'
    AND settings @> '{"channels" : {"telegram": true}}';

-- 2.8.2. Поиск с OR (используем ? или OR)
SELECT user_id, settings
FROM notification_settings
WHERE settings ? 'language'
   OR settings ? 'timezone';    

-- 2.8.3. Поиск по вложенному полю с сравнением   
SELECT user_id, settings
FROM notification_settings
WHERE (settings #>> '{quiet_hours, start}') < '23:00';

-- 2.8.4. Агрегация: сколько пользователей имеют каждый тип уведомления
SELECT
    'email' AS notification_type,
    COUNT(*) FILTER (WHERE settings @> '{"email": true}') AS enabled
FROM notification_settings
UNION ALL
SELECT
    'push' AS notification_type,
    COUNT(*) FILTER (WHERE settings @> '{"push": true}') AS enabled
FROM notification_settings
UNION ALL
SELECT
    'sms' AS notification_type,
    COUNT(*) FILTER (WHERE settings @> '{"sms": true}') AS enabled
FROM notification_settings;

-- 2.8.5. Получение всех уникальных тегов
SELECT DISTINCT jsonb_array_elements(settings->'tags') AS tag
FROM notification_settings
WHERE settings ? 'tags';

-- 2.8.6. Получение пользователей с конкретным тегом (через jsonb_array_elements)
SELECT DISTINCT user_id, settings
FROM notification_settings,
     jsonb_array_elements_text(settings->'tags') AS tag
WHERE tag = 'urgent';

-- 2.8.7. Пользователи с минимальными настройками (только email)
SELECT user_id, settings
FROM notification_settings
WHERE settings = '{"email": true}'::JSONB;

--СЛОЖНЫЕ ПРИМЕРЫ ДЛЯ JSONB
-- 1. КОМБИНИРОВАННЫЙ ПОИСК С РАЗВОРАЧИВАНИЕМ МАССИВОВ
-- Найти пользователей, у которых есть тег 'urgent' И включен email И
-- language = 'ru' (если поле language есть)
SELECT DISTINCT ns.user_id, ns.settings
FROM notification_settings ns,
    jsonb_array_elements_text(ns.settings ->'tags') AS tag
WHERE ns.settings @> '{"email": true}'
    AND (ns.settings ->>'language') = 'true'
    AND tag = 'urgent';

-- 2. ОБНОВЛЕНИЕ С УСЛОВИЕМ: добавить тег 'vip' только для пользователей,
-- у которых суммарное количество тегов < 5, и они имеют email=true
WITH target_users AS(
    SELECT user_id, settings
    FROM notification_settings
    WHERE settings @> '{"email": true}'
        AND jsonb_array_length(settings -> 'tags') < 5
)    
UPDATE notification_settings
SET settings = jconb_set(
    settings,
    '{tags}',
    (settings ->> 'tags') || '["vip"]'::JSONB,
    true
)
FROM target_users
WHERE notification_settings.user_id = target_users.user_id
    AND NOT (settings ->> 'tags' ? 'vip');
-- Пояснение: CTE, jsonb_array_length, условное добавление без дубликатов.

-- 3. АНАЛИТИКА: количество пользователей по комбинациям (email, push, sms)
-- и среднее количество тегов
SELECT
    (settings ->> 'email')::boolean AS email,
    (settings ->> 'push')::boolean AS push,
    (settings ->> 'sms')::boolean AS sms,
    COUNT(*) AS user_count,
    AVG(jsonb_array_length(settings -> 'tags'))::int AS avg_tags
FROM notification_settings
WHERE settings ? 'email'
  AND settings ? 'push'
  AND settings ? 'sms'
GROUP BY email, push, sms
ORDER BY user_count DESC;  

-- 4. ОБНОВЛЕНИЕ НЕСКОЛЬКИХ ПОЛЕЙ С ПРОВЕРКОЙ (оптимистичная блокировка)
UPDATE notification_settings
SET settings = settings || jsonb_build_object(
    'email', true,
    'push', false,
    'updated_at', to_jsonb(NOW())
)
WHERE user_id = 1
    AND (settings ->> 'email')::boolean = false --проверка старого значения
    AND (settings ->> 'email')::boolean = true;

-- 5. ИНДЕКС НА ЧАСТЬ JSONB (partial index) — для быстрого поиска активных email
CREATE INDEX idx_email_true ON notification_settings (user_id) WHERE settings @> '{"email": true}';
-- Запрос, использующий этот индекс:
SELECT user_id FROM notification_settings
WHERE settings @> '{"email": true}';
-- Пояснение: частичный индекс уменьшает размер и ускоряет выборку активных.     

-- 6. ПОИСК С ИСПОЛЬЗОВАНИЕМ ВСЕХ ОПЕРАТОРОВ: ?, ?|, ?&, @>, ->, ->>
-- Найти пользователей, у которых есть language и tags, и хотя бы один тег из списка
SELECT user_id, settings
FROM notification_settings
WHERE settings ? 'language'
 AND settings @> '{"email": true}'
 AND settings -> 'tags' ?| ARRAT '['urgent', 'critical']';

 -- 7. УДАЛЕНИЕ ПОЛЯ С ГЛУБОКИМ ПУТЕМ (jsonb #-) и переименование
-- Удаляем whatsapp из channels
UPDATE notification_settings
SET settings = settings #- '{channels, whatsapp}'
WHERE settings @> '{"channels": {"whatsapp": true}}'
RETURNING user_id, settings;

-- Переименовываем поле 'telegram' в 'tg' (через удаление и добавление)
UPDATE notification_settings
SET settings = jsonb_set(
    settings - 'channels',
    '{channels, tg}',
    settings->'channels'->'telegram'
)
WHERE settings ? 'channels'
  AND settings->'channels' ? 'telegram';
-- Пояснение: сложные манипуляции с путями.

-- 8. ИСПОЛЬЗОВАНИЕ ОКОННЫХ ФУНКЦИЙ ДЛЯ РАНЖИРОВАНИЯ ПО КОЛИЧЕСТВУ НАСТРОЕК
SELECT
    user_id,
    jsonb_array_length(settings->'tags') AS tag_count,
    (SELECT COUNT(*) FROM jsonb_each(settings) WHERE key NOT IN ('tags', 'channels')) AS other_fields,
    ROW_NUMBER() OVER (ORDER BY jsonb_array_length(settings->'tags') DESC) AS tag_rank
FROM notification_settings
WHERE settings ? 'tags';
-- Пояснение: оконная функция + подзапрос для подсчета полей.

-- 9. ПОСТРОЕНИЕ ОТЧЕТА: сколько пользователей имеют каждый тег
-- с фильтром по email=true и группировкой по language

WITH tag_expanded AS (
    SELECT
        user_id,
        tag,
        settings ->> 'language' AS language,
        settings @> '{"email": true}' AS has_email
    FROM notification_settings,
         jsonb_array_elements_text(settings->'tags') AS tag     
    WHERE settings ? 'tags'
)
SELECT
    tag,
    language,
    COUNT(*) AS user_count
FROM tag_expanded
WHERE has_email = true
    AND language IS NOT NULL
GROUP BY tag, language
ORDER BY tag, user_count DESC;

-- 10. ОПТИМИЗАЦИЯ: СОЗДАНИЕ ВИРТУАЛЬНОЙ КОЛОНКИ (GENERATED) ДЛЯ ЧАСТОГО ПОЛЯ
ALTER TABLE notification_settings
ADD COLUMN email_enabled boolean
GENERATED ALWAYS AS ((settings ->> 'email')::boolean) STORED;

-- Теперь можно создать обычный индекс и использовать в WHERE
CREATE INDEX idx_email_enabled ON notification_settings (email_enabled);

-- Запрос будет использовать индекс
SELECT user_id FROM notification_settings WHERE email_enabled = true;
-- Пояснение: генерируемые колонки упрощают индексацию и ускоряют запросы.
