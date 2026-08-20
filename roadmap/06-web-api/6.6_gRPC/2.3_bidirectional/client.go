package __3_bidirectional

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/"
)

func main() {
	// Подключаемся к серверу
	conn, err := grpc.NewClient("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewChatServiceClient(conn)

	// Контекст с долгим таймаутом (для бесконечного чата)
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()

	stream, err := client.Chat(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Генерируем уникальный ID пользователя (можно передать как флаг)
	userID := fmt.Sprintf("cli-%d", time.Now().UnixNano())
	log.Printf("👤 Подключился как: %s", userID)

	var wg sync.WaitGroup
	wg.Add(2)

	// ── 1. ГОРУТИНА ДЛЯ ОТПРАВКИ СООБЩЕНИЙ (из stdin) ─────────────────
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Println("Введите сообщение (или команду /help для справки):")

		for {
			// Ждём ввод пользователя
			scanner.Scan()
			text := scanner.Text()
			if strings.TrimSpace(text) == "" {
				continue
			}

			// Проверяем команду выхода
			if text == "/quit" || text == "/exit" {
				log.Println("Выход...")
				if err := stream.CloseSend(); err != nil {
					log.Printf("Ошибка CloseSend: %v", err)
				}
				cancel()
				return
			}

			// Отправляем сообщение
			msg := &pb.ChatMessage{
				Room:    "general", // можно было бы хранить комнату локально
				User_id: userID,
				Text:    text,
				Sent_at: timestamppb.Now(),
			}
			if err := stream.Send(msg); err != nil {
				log.Printf("Ошибка отправки: %v", err)
				return
			}
			log.Printf("Отправлено: %s", text)
		}
	}()

	// ── 2. ГОРУТИНА ДЛЯ ПРИЁМА СООБЩЕНИЙ ──────────────────────────────
	go func() {
		defer wg.Done()
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				log.Println("Сервер закрыл стрим")
				cancel()
				return
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
				cancel()
				return
			}

			// Выводим сообщение (с цветом или просто текстом)
			if msg.User_id == "system" || msg.User_id == "echo" {
				fmt.Printf("\n⚠️ [%s] %s\n", msg.User_id, msg.Text)
			} else {
				fmt.Printf("\n📨 [%s] %s\n", msg.User_id, msg.Text)
			}
			fmt.Print("Введите сообщение: ")
		}
	}()

	// Обрабатываем системные сигналы для graceful завершения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Получен сигнал завершения")
		cancel()
		stream.CloseSend()
	}()

	wg.Wait()
	log.Println("Клиент завершил работу")
}
