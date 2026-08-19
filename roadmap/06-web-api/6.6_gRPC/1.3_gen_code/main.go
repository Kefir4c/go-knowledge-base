package main

/*
  ГЕНЕРАЦИЯ КОДА ЧЕРЕЗ PROTOC

  Этот файл содержит ИСЧЕРПЫВАЮЩУЮ ТЕОРИЮ по генерации кода из .proto-файлов
  с помощью protoc. Вся информация — из официальной документации protobuf.dev
  и grpc.io.

  СОДЕРЖАНИЕ:
    1.  Что такое protoc и зачем он нужен
    2.  Установка protoc (компилятор)
    3.  Установка Go-плагинов (protoc-gen-go, protoc-gen-go-grpc)
    4.  Команда protoc — полный разбор флагов
    5.  Что генерируется: .pb.go и _grpc.pb.go (подробно)
    6.  Продвинутые темы: go:generate, Makefile, buf
    7.  Частые ошибки и их решение
    8.  Ключевые выводы для собеседования

  1.  ЧТО ТАКОЕ PROTOC И ЗАЧЕМ ОН НУЖЕН
  protoc — это компилятор Protocol Buffers. Он читает .proto-файлы,
  парсит их и генерирует код на разных языках с помощью плагинов.

  В контексте Go protoc используется для генерации двух типов файлов:
    • .pb.go — содержат структуры сообщений, enum, функции сериализации.
    • _grpc.pb.go — содержат интерфейсы сервера и клиента для gRPC.

  protoc НЕ ЯВЛЯЕТСЯ ЧАСТЬЮ Go. Это отдельный инструмент, который нужно
  устанавливать отдельно. Он написан на C++ и распространяется как
  бинарный файл.

  2.  УСТАНОВКА PROTOC (КОМПИЛЯТОР)
  protoc можно установить несколькими способами в зависимости от ОС.

  2.1. macOS (через Homebrew)
    brew install protobuf
    После установки появляется команда protoc в /usr/local/bin.
    Проверить версию: protoc --version.

  2.2. Linux (Ubuntu/Debian)
    sudo apt update
    sudo apt install protobuf-compiler
    Или скачать бинарный архив с GitHub и распаковать в /usr/local.

  2.3. Windows
    Через Chocolatey: choco install protoc.
    Или скачать .zip с GitHub и добавить в PATH.

  2.4. Установка из исходников (редко, для экспериментальных версий)
    git clone https://github.com/protocolbuffers/protobuf.git
    cd protobuf
    ./autogen.sh
    ./configure
    make
    make install

  После установки protoc должен быть доступен в PATH.
  Убедиться: which protoc (Linux/macOS) или where protoc (Windows).

  3.  УСТАНОВКА GO-ПЛАГИНОВ (PROTOC-GEN-GO, PROTOC-GEN-GO-GRPC)
  protoc сам по себе не умеет генерировать Go-код. Для этого нужны плагины,
  которые protoc вызывает как внешние программы.

  3.1. protoc-gen-go — плагин для генерации сообщений (.pb.go)
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    Этот плагин генерирует:
      • Go-структуры для message.
      • Константы для enum.
      • Функции-геттеры для полей.
      • Методы для сериализации (Marshal/Unmarshal).
      • Реализацию интерфейса proto.Message.

  3.2. protoc-gen-go-grpc — плагин для генерации gRPC-сервисов (_grpc.pb.go)
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
    Этот плагин генерирует:
      • Интерфейс сервера (UserServiceServer).
      • Интерфейс клиента (UserServiceClient).
      • Реализацию клиента (userServiceClient).
      • Функцию для регистрации сервера (RegisterUserServiceServer).
      • Дескриптор сервиса (UserService_ServiceDesc).

  3.3. Где лежат плагины
    Оба плагина устанавливаются в $GOPATH/bin (или $GOBIN).
    protoc ищет их в PATH. Убедись, что $GOPATH/bin добавлен в PATH.

    export PATH=$PATH:$(go env GOPATH)/bin

  3.4. Версии плагинов
    • protoc-gen-go: v1.28.0+ (для google.golang.org/protobuf v1.28+)
    • protoc-gen-go-grpc: v1.2.0+ (для google.golang.org/grpc v1.2+)

    Используй актуальные версии (latest) для совместимости.

  4.  КОМАНДА PROTOC — ПОЛНЫЙ РАЗБОР ФЛАГОВ
  Базовая команда выглядит так:

    protoc --proto_path=proto \
           --go_out=. --go_opt=paths=source_relative \
           --go-grpc_out=. --go-grpc_opt=paths=source_relative \
           proto/hello.proto

  Разберём каждый флаг подробно.

  4.1. --proto_path (-I) — корневая директория для импортов

    Указывает, где protoc должен искать .proto-файлы, включая импорты.
    Если .proto-файл использует import "google/protobuf/timestamp.proto",
    то protoc будет искать его в директориях, перечисленных в --proto_path.

    Можно указать несколько:
      protoc --proto_path=proto --proto_path=third_party ...

    Если не указать, protoc ищет в текущей директории.

  4.2. --go_out — директория для .pb.go файлов
    Указывает, куда сохранять сгенерированные .pb.go файлы.
    protoc создаёт структуру папок в соответствии с go_package.

    Пример:
      option go_package = "github.com/example/api/user/v1;userv1";
      --go_out=. → создаст ./github.com/example/api/user/v1/user.pb.go

  4.3. --go_opt — опции для go-плагина
    Основные опции:
      • paths=source_relative — генерировать файлы рядом с .proto,
        а не по пути go_package.
      • module=github.com/example/api — генерировать файлы относительно
        указанного модуля (для Go модулей).

    Пример:
      --go_opt=paths=source_relative
      → user.pb.go появится в той же папке, что и user.proto.

  4.4. --go-grpc_out — директория для _grpc.pb.go файлов
    Аналогично --go_out, но для gRPC-файлов.

  4.5. --go-grpc_opt — опции для go-grpc-плагина
    Аналогично --go_opt, но для gRPC.
    Основная: paths=source_relative.

  4.6. paths=source_relative — ПОДРОБНО
    БЕЗ ЭТОГО:
      protoc генерирует файлы в структуре, соответствующей go_package.
      Если go_package = "github.com/example/api/user/v1;userv1",
      то файлы появятся в ./github.com/example/api/user/v1/.

    С ЭТИМ:
      protoc генерирует файлы РЯДОМ с .proto-файлом.
      user.pb.go и user_grpc.pb.go будут лежать в той же папке, что и user.proto.

    ПРАКТИЧЕСКИЙ СОВЕТ:
      Всегда используй paths=source_relative. Это упрощает структуру
      проекта и делает её предсказуемой.

  4.7. Указание нескольких .proto-файлов
    Можно указать несколько файлов или использовать wildcard:
      protoc ... proto/*.proto
      protoc ... proto/++/+.proto

4.8. Генерация всех файлов из директории

protoc ... --go_out=. proto/**

  5.  ЧТО ГЕНЕРИРУЕТСЯ: .PB.GO И _GRPC.PB.GO (ПОДРОБНО)
  5.1. .pb.go — структуры сообщений
    Этот файл содержит код для работы с сообщениями.

    ЧТО В НЁМ ЕСТЬ:
      • Go-структуры для всех message.
        Например, message User → type User struct { ... }

      • Константы для всех enum.
        Например, enum Status → const Status_STATUS_ACTIVE = 1

      • Функции-геттеры для полей.
        Например, func (x *User) GetId() int32

      • Методы для реализации proto.Message:
        Reset(), String(), ProtoMessage(), ProtoReflect(), Descriptor().

      • Функции для сериализации:
        proto.Marshal, proto.Unmarshal (работают через интерфейс).

      • Структуры для работы с oneof (обычно интерфейс или структура с указателями).

      • Структуры для работы с map (обычно маппинг).

    ВАЖНО:
      • Не редактируй .pb.go вручную — он перегенерируется.
      • Используй только геттеры, чтобы избежать nil-паники.

  5.2. _grpc.pb.go — интерфейсы gRPC-сервисов
    Этот файл содержит код для gRPC-коммуникации.

    ЧТО В НЁМ ЕСТЬ:
      • Интерфейс сервера (UserServiceServer).
        Содержит все RPC-методы. Должен быть реализован пользователем.

      • Интерфейс клиента (UserServiceClient).
        Содержит все RPC-методы для вызова с сервера.

      • Реализация клиента (userServiceClient).
        Структура, которая реализует интерфейс клиента и вызывает
        gRPC-методы через соединение.

      • Функция для регистрации сервера (RegisterUserServiceServer).
        Регистрирует реализацию сервера в gRPC-сервере.

      • Дескриптор сервиса (UserService_ServiceDesc).
        Используется gRPC-инфраструктурой для маршрутизации вызовов.

      • Unimplemented-структура (UnimplementedUserServiceServer).
        Обеспечивает обратную совместимость при добавлении новых методов.

    ВАЖНО:
      • Все серверные реализации должны встраивать UnimplementedUserServiceServer.
      • Клиент создаётся через NewUserServiceClient(conn).

  6.  ПРОДВИНУТЫЕ ТЕМЫ: GO:GENERATE, MAKEFILE, BUF
  6.1. go:generate — встраивание в Go-экосистему
    В .proto-файл можно добавить комментарий с директивой go:generate:

      //go:generate protoc --proto_path=. --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative hello.proto

    После этого можно генерировать код командой:
      go generate ./...

    Это удобно, потому что не нужно запоминать длинную команду.

  6.2. Makefile — автоматизация для команды
    Для проектов с большим количеством .proto-файлов используют Makefile:

      .PHONY: gen
      gen:
          protoc --proto_path=proto \
                 --go_out=. --go_opt=paths=source_relative \
                 --go-grpc_out=. --go-grpc_opt=paths=source_relative \
                 proto/++/+.proto

.PHONY: gen-clean
gen-clean:
find . -name "*.pb.go" -delete

Запуск:
make gen

6.3. buf — современный инструмент для управления .proto
buf — это альтернатива protoc, которая предоставляет:
• Линтинг (style checking).
• Форматирование.
• Генерацию кода (через плагины).
• Брейкинг-чейнджер (проверка на breaking changes).

Установка:
brew install bufbuild/buf/buf

Конфигурация (buf.gen.yaml):
version: v1
plugins:
- name: go
out: .
opt: paths=source_relative
- name: go-grpc
out: .
opt: paths=source_relative

Генерация:
buf generate

buf автоматически находит все .proto-файлы и генерирует код по конфигу.
Это удобнее, чем protoc, потому что не нужно указывать пути вручную.

7.  ЧАСТЫЕ ОШИБКИ И ИХ РЕШЕНИЕ
 "protoc-gen-go: program not found or is not executable"

Причина: protoc не может найти плагин.
Решение: Установить плагин и добавить $GOPATH/bin в PATH.

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
export PATH=$PATH:$(go env GOPATH)/bin

 "open ...: no such file or directory"

Причина: protoc не может найти .proto-файл по указанному пути.
Решение: Проверить --proto_path и правильность пути к файлу.

 "import google/protobuf/timestamp.proto: not found"

Причина: protoc не знает, где искать стандартные импорты protobuf.
Решение: Добавить путь к стандартным импортам.

protoc --proto_path=/usr/local/include --proto_path=. ...

 "Option go_package is required"

Причина: В .proto-файле отсутствует option go_package.
Решение: Добавить option go_package.

 "package ... is not in GOROOT"

Причина: Сгенерированный код ссылается на пакет, который не установлен.
Решение: Добавить require в go.mod и выполнить go mod tidy.

 "syntax = proto2; but protoc-gen-go only supports proto3"

Причина: Используется proto2, а плагин ожидает proto3.
Решение: Перейти на proto3 или указать опцию для поддержки proto2.

8.  КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
1.  protoc — компилятор Protocol Buffers, устанавливается отдельно.
Не является частью Go.

2.  Для Go нужны два плагина:
• protoc-gen-go — для сообщений (.pb.go)
• protoc-gen-go-grpc — для сервисов (_grpc.pb.go)

3.  Установка плагинов через go install, они должны быть в $PATH.

4.  Ключевые флаги protoc:
• --proto_path (-I) — корень поиска .proto-файлов.
• --go_out — директория для .pb.go.
• --go_opt=paths=source_relative — генерировать рядом с .proto.
• --go-grpc_out и --go-grpc_opt — аналогично для gRPC.

5.  paths=source_relative — стандартная практика, упрощает структуру.

6.  .pb.go содержит структуры сообщений, _grpc.pb.go — интерфейсы сервисов.

7.  Для автоматизации используют go:generate, Makefile или buf.

8.  buf — современный инструмент, который заменяет protoc + плагины
и добавляет линтинг, форматирование, проверку на breaking changes.

9.  Частые ошибки:
• Плагины не найдены в PATH.
• Пропущен --proto_path.
• Не указан go_package.
• Импорты не найдены (нужен путь к google/protobuf).
*/
