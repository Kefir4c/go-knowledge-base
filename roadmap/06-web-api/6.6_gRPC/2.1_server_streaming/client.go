package __1_server_streaming

import (
	"context"
	"io"
	"log"
	"time"

	pb "github.com/example/streaming-example/proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)

	//1. ListUsers

	log.Println("\n=== ListUsers (server-streaming) ===")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.ListUsers(ctx, &pb.ListUsersRequest{limit: 3})
	if err != nil {
		log.Fatalf("ListUsers ошибка: %v", err)
	}

	for {
		user, err := stream.Recv()
		if err == io.EOF {
			log.Println("Стрим завершён (EOF)")
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
		log.Printf("Получен пользователь: %s (ID: %s)", user.Name, user.Id)

	}

	//2. SuscribeToUpdates

	log.Println("\n=== SubscribeToUpdates (streaming обновлений) ===")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()

	stream2, err := client.SubscribeToUpdates(ctx2, &pb.SubscribeToUpdatesRequest{})
	if err != nil {
		log.Fatalf("SubscribeToUpdates ошибка: %v", err)
	}

	//Читаем обновления до таймаута или ошибки
	for {
		update, err := stream.Recv()
		if err == io.EOF {
			log.Println("Стрим подписки завершён (EOF)")
			break
		}
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				switch st.Code() {
				case codes.DeadlineExceeded:
					log.Println("Таймаут подписки")
				case codes.Canceled:
					log.Println("Подписка отменена")
				default:
					log.Printf("Ошибка подписки: %v", st.Message())
				}
			} else {
				log.Printf("Не-gRPC ошибка: %v", err)
			}
			break
		}
		log.Printf("Обновление: тип=%s, пользователь=%s, время=%v",
			update.Type.String(),
			update.User.Name,
			update.OccurredAt.AsTime(),
		)
	}
}
