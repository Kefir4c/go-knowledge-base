package __3_bidirectional

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/"
)

/*
  РАЗДЕЛ 2.3: BIDIRECTIONAL STREAMING RPC
  Bidirectional Streaming RPC — это самый мощный и гибкий тип RPC в gRPC,
  который позволяет КЛИЕНТУ и СЕРВЕРУ одновременно отправлять и получать
  потоки сообщений. Это как два независимых канала в одном соединении.

  В этом разделе мы разберём:
    1.  Что такое Bidirectional Streaming RPC и когда он используется
    2.  Определение в .proto-файле (stream и там, и там)
    3.  Реализация сервера: независимый приём и отправка
    4.  Реализация клиента: независимая отправка и приём
    5.  Важные нюансы: порядок сообщений, обработка ошибок, горутины
    6.  Полный рабочий пример (теория + код)
    7.  Bidirectional vs все остальные типы RPC
    8.  Частые ошибки и best practices

  1.  ЧТО ТАКОЕ BIDIRECTIONAL STREAMING RPC И КОГДА ОН ИСПОЛЬЗУЕТСЯ
  Bidirectional Streaming RPC — это вызов, при котором:
    • Клиент отправляет ПОТОК сообщений на сервер.
    • Сервер отправляет ПОТОК сообщений клиенту.
    • Оба потока работают НЕЗАВИСИМО и АСИНХРОННО.
    • Порядок сообщений гарантируется в КАЖДОМ потоке отдельно.

  Аналогия: это как телефонный разговор — обе стороны говорят и слушают
  одновременно (или по очереди, но независимо).

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Реализация чатов (real-time messaging).
    • Интерактивные системы (игры, коллаборация).
    • Командно-контрольные системы (управление роботами).
    • Мониторинг и управление (отправка команд и получение статусов).
    • Системы с двусторонним обменом данными (трейдинг, биржи).
    • Любой сценарий, где обе стороны инициируют отправку.

  Преимущества:
    • Полная двусторонняя связь в реальном времени.
    • Оба потока независимы — не блокируют друг друга.
    • Можно использовать для интерактивных приложений.
    • Один открытый канал для всего обмена.

  2.  ОПРЕДЕЛЕНИЕ В .PROTO-ФАЙЛЕ
  В .proto-файле Bidirectional Streaming описывается с ключевым словом stream
  И для запроса, И для ответа:

    service ChatService {
      rpc Chat (stream ChatMessage) returns (stream ChatMessage) {}
    }

  Полный пример:
    syntax = "proto3";
    package chat.v1;
    option go_package = "github.com/example/api/chat/v1;chatpb";

    message ChatMessage {
      string user_id = 1;
      string text = 2;
      google.protobuf.Timestamp sent_at = 3;
    }

    service ChatService {
      rpc Chat (stream ChatMessage) returns (stream ChatMessage) {}
    }

  После генерации кода:
    • chat_grpc.pb.go будет содержать методы:
      - Chat(ChatService_ChatServer) error — на сервере
      - Chat(ctx context.Context, opts ...) (ChatService_ChatClient, error) — на клиенте

  3.  РЕАЛИЗАЦИЯ СЕРВЕРА

  3.1. Сигнатура метода
    func (s *server) Chat(stream pb.ChatService_ChatServer) error

  Сервер принимает поток (интерфейс стрима), который одновременно
  позволяет и читать, и отправлять.

  3.2. Типичная структура сервера
    func (s *server) Chat(stream pb.ChatService_ChatServer) error {
      ctx := stream.Context()

      // Канал для остановки горутин
      done := make(chan bool)
      defer close(done)

      // Городина для отправки сообщений (если нужно)
      go func() {
        for {
          select {
          case <-done:
            return
          case <-ctx.Done():
            return
          case msg := <-s.outgoing:
            if err := stream.Send(msg); err != nil {
              return
            }
          }
        }
      }()

      // Основной цикл приёма сообщений
      for {
        select {
        case <-ctx.Done():
          return status.Error(codes.Canceled, "client cancelled")
        default:
        }

        msg, err := stream.Recv()
        if err == io.EOF {
          return nil // клиент закрыл стрим
        }
        if err != nil {
          return err
        }

        // Обработка входящего сообщения
        response := s.processMessage(msg)

        // Отправка ответа (может быть в той же горутине или в отдельной)
        if err := stream.Send(response); err != nil {
          return err
        }
      }
    }

  3.3. Два подхода к отправке
    a) СИНХРОННЫЙ (в той же горутине, что и приём):
       for {
         msg, _ := stream.Recv()
         // обработать
         stream.Send(response) // блокируется, пока клиент не получит
       }

    b) АСИНХРОННЫЙ (отдельная горутина для отправки):
       go func() {
         for msg := range outgoingCh {
           stream.Send(msg) // не блокирует основную горутину
         }
       }()
       for {
         msg, _ := stream.Recv()
         outgoingCh <- response
       }

  3.4. Контекст в Bidirectional Streaming
    ctx := stream.Context()

    • Используется для проверки дедлайна:
      if ctx.Err() == context.DeadlineExceeded { ... }

    • Используется для отмены:
      select {
      case <-ctx.Done():
        return status.Error(codes.Canceled, "cancelled")
      default:
      }

  3.5. Закрытие стрима
    • Сервер закрывает стрим, возвращая nil (успех) или error (ошибка).
    • После возврата из метода стрим автоматически закрывается.
    • Нельзя отправлять после возврата из метода.

  4.  РЕАЛИЗАЦИЯ КЛИЕНТА

  4.1. Вызов метода
    stream, err := client.Chat(ctx)
    if err != nil {
      log.Fatal(err)
    }

  4.2. Отправка и приём в разных горутинах
    // ── Горутина для отправки ──
    go func() {
      for _, msg := range messages {
        if err := stream.Send(msg); err != nil {
          log.Printf("Ошибка отправки: %v", err)
          return
        }
      }
      stream.CloseSend() // закрываем сторону отправки
    }()

    // ── Основная горутина для приёма ──
    for {
      msg, err := stream.Recv()
      if err == io.EOF {
        log.Println("Стрим завершён")
        break
      }
      if err != nil {
        log.Printf("Ошибка приёма: %v", err)
        break
      }
      log.Printf("Получено: %+v", msg)
    }

  4.3. Закрытие стрима со стороны клиента
    stream.CloseSend() — закрывает сторону отправки.
    После этого клиент всё ещё может получать сообщения.
    Сервер получит io.EOF при следующем Recv().

  4.4. Полная остановка
    • Клиент может отменить контекст (cancel()) — стрим закроется с обеих сторон.
    • Сервер получит ctx.Done().
    • Клиент получит ошибку при следующем Recv().

  5.  ВАЖНЫЕ НЮАНСЫ

  5.1. Порядок сообщений
    • В КАЖДОМ направлении сообщения приходят в том порядке,
      в котором были отправлены.
    • Нет гарантии, что сообщение A от сервера придёт раньше
      сообщения B от клиента — потоки независимы.

  5.2. Обработка ошибок
    • Если одна сторона закрывает стрим, другая получает:
      - io.EOF (если закрыто корректно)
      - ошибку (если закрыто с ошибкой)
    • Используй status.FromError() для разбора ошибок.

  5.3. Горутины и синхронизация
    • При использовании отдельных горутин для отправки и приёма
      нужно синхронизировать их завершение (sync.WaitGroup, каналы).

  5.4. Дедлайны
    • Дедлайн устанавливается на клиенте через context.WithTimeout.
    • Если дедлайн истёк, сервер получит DeadlineExceeded в ctx.
    • Клиент получит DeadlineExceeded при следующем Recv().

  5.5. Закрытие стрима
    • stream.CloseSend() — закрывает только отправку (клиент).
    • Сервер должен вернуть nil или ошибку, чтобы закрыть стрим полностью.
    • Нельзя отправить сообщение после CloseSend().

  6.  ПОЛНЫЙ РАБОЧИЙ ПРИМЕР (ТЕОРИЯ + КОД)
  6.1. .proto-файл (proto/chat.proto)

    syntax = "proto3";
    package chat.v1;
    option go_package = "github.com/example/streaming-example/proto/chat;chatpb";

    import "google/protobuf/timestamp.proto";

    message ChatMessage {
      string user_id = 1;
      string text = 2;
      google.protobuf.Timestamp sent_at = 3;
    }

    service ChatService {
      rpc Chat (stream ChatMessage) returns (stream ChatMessage) {}
    }

  6.2. Сервер (server/main.go)

    type ChatServer struct {
      pb.UnimplementedChatServiceServer
      mu    sync.RWMutex
      rooms map[string][]string // room_id -> list of users
    }

    func NewChatServer() *ChatServer {
      return &ChatServer{
        rooms: make(map[string][]string),
      }
    }

    // Chat — bidirection streaming
    func (s *ChatServer) Chat(stream pb.ChatService_ChatServer) error {
      ctx := stream.Context()
      userID := fmt.Sprintf("user-%d", time.Now().UnixNano())

      log.Printf("[%s] Подключился к чату", userID)

      // Канал для отправки сообщений (буферизированный)
      outgoing := make(chan *pb.ChatMessage, 10)
      defer close(outgoing)

      // ── Горутина для отправки ──
      go func() {
        for msg := range outgoing {
          if err := stream.Send(msg); err != nil {
            log.Printf("[%s] Ошибка отправки: %v", userID, err)
            return
          }
        }
      }()

      // Основной цикл приёма
      for {
        select {
        case <-ctx.Done():
          log.Printf("[%s] Контекст отменён: %v", userID, ctx.Err())
          return status.Error(codes.Canceled, "client cancelled")
        default:
        }

        msg, err := stream.Recv()
        if err == io.EOF {
          log.Printf("[%s] Клиент закрыл стрим", userID)
          return nil
        }
        if err != nil {
          log.Printf("[%s] Ошибка чтения: %v", userID, err)
          return err
        }

        log.Printf("[%s] Получено сообщение: %s", userID, msg.Text)

        // Эхо-ответ с дополнением
        response := &pb.ChatMessage{
          UserId: "echo-server",
          Text:   fmt.Sprintf("Echo: %s (от %s)", msg.Text, msg.UserId),
          SentAt: timestamppb.Now(),
        }

        // Отправляем ответ через канал (не блокирует приём)
        select {
        case outgoing <- response:
          log.Printf("[%s] Отправлен ответ: %s", userID, response.Text)
        case <-ctx.Done():
          return status.Error(codes.Canceled, "context cancelled")
        }
      }
    }

    func main() {
      server := NewChatServer()

      s := grpc.NewServer()
      pb.RegisterChatServiceServer(s, server)

      lis, _ := net.Listen("tcp", ":50051")
      log.Println(Чат-сервер запущен на :50051")

      go func() {
        if err := s.Serve(lis); err != nil {
          log.Fatal(err)
        }
      }()

      stop := make(chan os.Signal, 1)
      signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
      <-stop

      log.Println(Остановка сервера...")
      s.GracefulStop()
      log.Println(Сервер остановлен")
    }

  6.3. Клиент (client/main.go)
    func main() {
      conn, err := grpc.NewClient("localhost:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
      )
      if err != nil {
        log.Fatal(err)
      }
      defer conn.Close()

      client := pb.NewChatServiceClient(conn)

      ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
      defer cancel()

      stream, err := client.Chat(ctx)
      if err != nil {
        log.Fatal(err)
      }

      var wg sync.WaitGroup
      wg.Add(2)

      // ── Горутина для отправки сообщений ──
      go func() {
        defer wg.Done()
        messages := []string{
          "Hello, server!",
          "How are you?",
          "This is bidirectional streaming!",
          "Closing in a moment...",
        }

        for i, text := range messages {
          msg := &pb.ChatMessage{
            UserId: "client-1",
            Text:   text,
            SentAt: timestamppb.Now(),
          }
          if err := stream.Send(msg); err != nil {
            log.Printf("Ошибка отправки: %v", err)
            return
          }
          log.Printf("Отправлено: %s", text)
          time.Sleep(1 * time.Second)

          // Если это последнее сообщение — закрываем отправку
          if i == len(messages)-1 {
            if err := stream.CloseSend(); err != nil {
              log.Printf("Ошибка CloseSend: %v", err)
            } else {
              log.Println("Отправка закрыта")
            }
          }
        }
      }()

      // ── Горутина для приёма сообщений ──
      go func() {
        defer wg.Done()
        for {
          msg, err := stream.Recv()
          if err == io.EOF {
            log.Println("Сервер закрыл стрим")
            break
          }
          if err != nil {
            st, ok := status.FromError(err)
            if ok {
              switch st.Code() {
              case codes.DeadlineExceeded:
                log.Println("Таймаут")
              case codes.Canceled:
                log.Println("Отменено")
              default:
                log.Printf("Ошибка: %v", st.Message())
              }
            } else {
              log.Printf("Не-gRPC ошибка: %v", err)
            }
            break
          }
          log.Printf("Получено: %s (от %s)", msg.Text, msg.UserId)
        }
      }()

      wg.Wait()
      log.Println("Клиент завершил работу")
    }

  7.  BIDIRECTIONAL VS ВСЕ ОСТАЛЬНЫЕ ТИПЫ RPC
  ┌─────────────────────┬─────────────┬─────────────────────┬─────────────────────┬─────────────────────────────┐
  │ Характеристика      │ Unary       │ Server-Streaming    │ Client-Streaming    │ Bidirectional               │
  ├─────────────────────┼─────────────┼─────────────────────┼─────────────────────┼─────────────────────────────┤
  │ Запросов            │ 1           │ 1                   │ N (поток)           │ N (поток)                   │
  ├─────────────────────┼─────────────┼─────────────────────┼─────────────────────┼─────────────────────────────┤
  │ Ответов             │ 1           │ N (поток)           │ 1                   │ N (поток)                   │
  ├─────────────────────┼─────────────┼─────────────────────┼─────────────────────┼─────────────────────────────┤
  │ Кто шлёт поток      │ -           │ Сервер              │ Клиент              │ Оба                         │
  ├─────────────────────┼─────────────┼─────────────────────┼─────────────────────┼─────────────────────────────┤
  │ Независимость       │ -           │ Только от сервера   │ Только от клиента   │ Полная (оба независимы)     │
  ├─────────────────────┼─────────────┼─────────────────────┼─────────────────────┼─────────────────────────────┤
  │ Use case            │ CRUD        │ Списки, подписки    │ Загрузка, пакетная  │ Чаты, игры, управление      │
  └─────────────────────┴─────────────┴─────────────────────┴─────────────────────┴─────────────────────────────┘

  8.  ЧАСТЫЕ ОШИБКИ И BEST PRACTICES

  8.1. Частые ошибки
* Закрыть стрим, но продолжать отправку. stream.Send() после CloseSend() вызовет ошибку.
* Не обрабатывать ctx.Done() в циклах. Сервер может продолжать работу после отмены клиентом.
* Отправлять сообщения в основной горутине без буфера. Это заблокирует приём, если клиент медленно читает.
* Игнорировать ошибки при stream.Send() и stream.Recv(). При обрыве соединения ошибки будут, их нужно обрабатывать.
* Использовать одну горутину для отправки и приёма. Одно направление заблокирует другое.

  8.2. Best Practices
* Всегда используй отдельные горутины для отправки и приёма (на клиенте И на сервере).
* Используй буферизированные каналы для передачи сообщений между горутинами.
* Проверяй ctx.Done() в всех циклах.
* Закрывай стрим корректно через stream.CloseSend() (клиент) или возврат из метода (сервер).
* Обрабатывай ошибки с помощью status.FromError().
* Используй sync.WaitGroup для синхронизации горутин.
* Добавляй логирование для отладки (особенно в реальном времени).
* Устанавливай разумные дедлайны (или не устанавливай вовсе для долгих соединений).

  9.  КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1.  Bidirectional Streaming RPC — «поток запросов ↔ поток ответов».
  2.  Определяется в .proto: rpc Chat(stream Request) returns (stream Response).
  3.  Обе стороны могут отправлять и принимать независимо.
  4.  Сервер реализует метод, который читает и отправляет в одном или
      разных циклах/горутинах.
  5.  Клиент читает и отправляет в разных горутинах.
  6.  Порядок сообщений гарантируется в КАЖДОМ направлении отдельно.
  7.  stream.CloseSend() — закрывает сторону отправки на клиенте.
  8.  io.EOF — сигнал, что другая сторона закрыла стрим.
  9.  Использовать для: чаты, игры, управление, мониторинг.
  10. Обрабатывай ctx.Done() для корректного завершения при отмене.
*/

const defaultRoom = "general"

// КЛИЕНТСКОЕ СОЕДИНЕНИЕ
// Client — представляет подключённого клиента
type Client struct {
	ID        string
	Room      string
	SendChat  chan *pb.ChatMessage // буферизированный канал для исходящих сообщений
	Stream    pb.ChatService_ChatServer
	Context   context.Context
	Cancel    context.CancelFunc
	mu        sync.Mutex
	connected bool
}

// ЧАТ-СЕРВЕР
type ChatServer struct {
	pb.UnimplementedChatServiceServer
	mu      sync.RWMutex
	rooms   map[string]map[string]*Client // room -> userID -> Client
	clients map[string]*Client            // userID -> Client (глобальный реестр)
}

func NewChatServer() *ChatServer {
	return &ChatServer{
		rooms:   make(map[string]map[string]*Client),
		clients: make(map[string]*Client),
	}
}

// РЕГИСТРАЦИЯ/ДЕРЕГИСТРАЦИЯ КЛИЕНТОВ
// registerClient — добавляет клиента в комнату и глобальный реестр
func (s *ChatServer) registerClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Добавляем в глобальный реестр
	s.clients[client.ID] = client

	// Добавляем в комнату
	if _, ok := s.rooms[client.Room]; !ok {
		s.rooms[client.Room] = make(map[string]*Client)
	}
	s.rooms[client.Room][client.ID] = client
	log.Printf("[%s] Подключился к комнате %s", client.ID, client.Room)
}

// unregisterClient — удаляет клиента и закрывает его канал
func (s *ChatServer) unregisterClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Удаляем из комнаты
	if roomClients, ok := s.rooms[client.Room]; ok {
		delete(roomClients, client.ID)
		if len(roomClients) == 0 {
			delete(s.rooms, client.Room) // удаляем пустую комнату
		}
	}

	// Удаляем из глобального реестра
	delete(s.clients, client.ID)

	// Закрываем канал отправки (чтобы горутина отправки завершилась)
	if client.SendChan != nil {
		close(client.SendChan)
		client.SendChan = nil
	}
	log.Printf("[%s] Отключился", client.ID)
}

// getRoomClients — возвращает копию списка клиентов в комнате
func (s *ChatServer) getRoomClients(room string) []*Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	roomClients, ok := s.rooms[room]
	if !ok {
		return nil
	}
	result := make([]*Client, 0, len(roomClients))
	for _, client := range roomClients {
		result = append(result, client)
	}
	return result
}

// BROADCAST
// broadcastToRoom — рассылает сообщение всем клиентам в комнате (кроме отправителя)
func (s *ChatServer) broadcastToRoom(room string, message *pb.ChatMessage, sendID string) {
	clients := s.getRoomClients(room)
	for _, c := range clients {
		if c.ID == sendID {
			continue
		}
		select {
		case c.SendChat <- message:
			// успешно отправлено в канал
		default:
			log.Printf("⚠️ Канал клиента %s переполнен, пропускаем сообщение", c.ID)
		}
	}
}

// ОБРАБОТКА КОМАНД
// handleCommand — обрабатывает команды из текста сообщения
func (s *ChatServer) handlerCommand(client *Client, text string) (response string, handled bool) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", false
	}
	switch parts[0] {
	case "/join":
		if len(parts) < 2 {
			return "Укажите имя комнаты: /join <room>", true
		}
		newRoom := parts[1]
		if newRoom == client.Room {
			return fmt.Sprintf("Вы уже в комнате %s", newRoom), true
		}

		// Перемещаем клиента в новую комнату
		s.mu.Lock()
		// Удаляем из старой комнаты
		if oldRoomClient, ok := s.rooms[client.Room]; ok {
			delete(oldRoomClient, client.ID)
			if len(oldRoomClient) == 0 {
				delete(s.rooms, client.Room)
			}
		}
		// Добавляем в новую
		if _, ok := s.rooms[newRoom]; !ok {
			s.rooms[newRoom] = make(map[string]*Client)
		}
		s.rooms[newRoom][client.ID] = client
		client.Room = newRoom
		s.mu.Unlock()

		// Оповещаем клиента и всех в комнате
		msg := fmt.Sprintf("Перешли в комнату %s", newRoom)
		return msg, true
	case "/leave":
		// Возвращаем в general
		if client.Room == defaultRoom {
			return "Вы уже в общей комнате", true
		}
		return s.handleCommand(client, "/join "+defaultRoom)

	case "/users":
		clients := s.getRoomClients(client.Room)
		if len(clients) == 0 {
			return "В комнате никого нет", true
		}
		userList := make([]string, 0, len(clients))
		for _, c := range clients {
			userList = append(userList, c.ID)
		}
		return fmt.Sprintf("Пользователи в комнате %s: %s", client.Room, strings.Join(userList, ", ")), true

	case "/echo":
		if len(parts) < 2 {
			return "Укажите текст: /echo <текст>", true
		}
		return strings.Join(parts[1:], " "), true

	default:
		return "", false
	}
}

// CHAT — BIDIRECTIONAL STREAMING (ОСНОВНАЯ ЛОГИКА)
func (s *ChatServer) Chat(stream pb.ChatService_ChatServer) error {
	// Генерируем уникальный ID для клиента
	userID := fmt.Sprintf("client-%d", time.Now().UnixNano())

	// Контекст стрима
	ctx := stream.Context()

	//роверяем дедлайн
	if ctx == context.DeadlineExceeded {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	// Создаём клиента
	clientCtx, cancel := context.WithCancel(ctx)
	client := &Client{
		ID:       userID,
		Room:     defaultRoom,
		SendChan: make(chan *pb.ChatMessage, 10), // буфер 10 сообщений
		Stream:   stream,
		Context:  clientCtx,
		Cancel:   cancel,
	}

	// Регистрируем клиента
	s.registerClient(client)

	// Гарантируем удаление при выходе
	defer func() {
		client.Cancel()
		s.unregisterClient(client)
	}()

	//ГОРУТИНА ДЛЯ ОТПРАВКИ СООБЩЕНИЙ
	go func() {
		for msg := range client.SendChat {
			if err := stream.Sand(msg); err != nil {
				log.Printf("[%s] Ошибка отправки: %v", client.ID, err)
				// При ошибке отправки прерываем стрим
				cancel()
				return
			}
		}
	}()

	// ГОРУТИНА ДЛЯ ОБРАБОТКИ ИСХОДЯЩИХ СООБЩЕНИЙ (BROADCAST)
	// Мы не будем отправлять все сообщения через broadcast из этой же горутины,
	// потому что Recv() блокирует. Вместо этого мы будем читать и обрабатывать
	// в основном цикле, а отправку эха делать через канал клиента.

	//ОСНОВНОЙ ЦИКЛ ПРИЁМА
	for {
		select {
		case <-clientCtx.Done():
			log.Printf("[%s] Контекст отменён", client.ID)
			return nil
		default:
		}

		msg, err := stream.Recv()
		if err == io.EOF {
			log.Printf("[%s] Клиент закрыл стрим", client.ID)
			return nil
		}
		if err != nil {
			log.Printf("[%s] Ошибка чтения: %v", client.ID, err)
			return err
		}

		log.Printf("[%s] %s: %s", client.ID, msg.UserId, msg.Text)

		// Проверяем, не команда ли это
		if responseText, handled := s.handlerCommand(client, msg.Text); handled {
			// Отправляем ответ как обычное сообщение
			responseMsg := &pb.ChatMessage{
				Room:    client.Room,
				User_id: "system",
				Text:    responseText,
				Sent_at: timestamppb.Now(),
			}
			select {
			case client.SendChan <- responseMsg:
			default:
				log.Printf("[%s] Канал отправки переполнен, пропускаем системное сообщение", client.ID)
			}
			continue // пропускаем broadcast для команд
		}

		// Если это обычное сообщение — рассылаем всем в комнате
		if msg.Room == "" {
			msg.Room = client.Room
		}
		if msg.User_id == "" {
			msg.User_id = client.ID
		}
		if msg.Sent_at == nil {
			msg.Sent_at = timestamppb.Now()
		}

		// Рассылаем всем в комнате, кроме отправителя
		s.broadcastToRoom(msg.Room, msg, client.ID)

		// Отправляем эхо обратно отправителю (как подтверждение)
		echoMsg := &pb.ChatMessage{
			Room:    client.Room,
			User_id: "echo",
			Text:    fmt.Sprintf("%s", msg.Text),
			Sent_at: timestamppb.Now(),
		}
		select {
		case client.SendChan <- echoMsg:
		default:
			log.Printf("[%s] Канал отправки переполнен, пропускаем эхо", client.ID)
		}
	}
}

//INTERCEPTOR ДЛЯ СТРИМОВ (ЛОГИРОВАНИЕ)

func StreamLoggingInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	log.Printf("🔁 [STREAM] Начало: %s", info.FullMethod)
	start := time.Now()
	err := handler(srv, ss)
	log.Printf("🔁 [STREAM] Конец: %s, ошибка: %v, длительность: %v",
		info.FullMethod, err, time.Since(start))
	return err
}

func main() {
	server := NewChatServer()

	s := grpc.NewServer(
		grpc.StreamInterceptor(StreamLoggingInterceptor),
	)

	pb.RegisterChatServiceServer(s, server)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Чат-сервер запущен на :50051")
	log.Println("Комнаты: /join <room>, /leave, /users, /echo <text>")

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Остановка сервера...")
	s.GracefulStop()
	log.Println("Сервер остановлен")
}
