package __2_interceptors

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/example/interceptors-example/proto/user"
)

// clientLoggingInterceptor — клиентский интерсептор для логирования
func clientLoggingInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		log.Printf("🔜 [CLIENT] Вызов: %s, Запрос: %+v", method, req)
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		log.Printf("🔙 [CLIENT] Ответ: %+v, Ошибка: %v, Время: %v", reply, err, time.Since(start))
		return err
	}
}

// Вспомогательные функции
func generateJWT(userID string) string {
	payload := fmt.Sprintf(`{"user_id":"%s","exp":%d}`, userID, time.Now().Add(time.Hour).Unix())
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func main() {
	// Клиент с интерсептором для логирования
	conn, err := grpc.NewClient("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(clientLoggingInterceptor()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)

	// Генерируем JWT и request_id
	token := generateJWT("user-123")
	requestID := generateRequestID()

	md := metadata.Pairs(
		"authorization", "Bearer"+token,
		"x-request-id", requestID,
	)
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	//1. Unary RPC
	log.Println("\n=== Unary: GetUser ===")
	ctx1, cancel1 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel1()

	resp, err := client.GetUset(ctx1, &pb.GetUserRequest{UserId: "1"})
	if err != nil {
		st, _ := status.FromError(err)
		log.Printf("Ошибка: %v", st.Message())
	} else {
		log.Printf("Пользователь: ID=%s, Name=%s, Email=%s", resp.Id, resp.Name, resp.Email)
	}

	//2. Server-Streaming RPC

	log.Println("\n=== Server-Streaming: ListUsers ===")
	ctx2, cancel2 := context.WithTimeout(ctx, 10*time.Second)
	defer cancel2()

	stream, err := client.ListUsers(ctx2, &pb.ListUsersRequest{Limit: 3})
	if err != nil {
		log.Fatal(err)
	}

	for {
		user, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			st, _ := status.FromError(err)
			log.Printf("Stream ошибка: %v", st.Message())
			break
		}
		log.Printf("Получен пользователь: %s", user.Name)
	}
}
