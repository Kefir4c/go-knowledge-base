package deadline

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/"
)

// handleError — обработка gRPC-ошибок с разбором кодов и деталей
func handleError(err error, method string) {
	if err == nil {
		return
	}

	st, ok := status.FromError(err)
	if !ok {
		log.Printf("%s: Не-gRPC ошибка: %v", method, err)
		return
	}

	switch st.Code() {
	case codes.NotFound:
		log.Printf("%s: Пользователь не найден (код=NotFound, сообщение=%s)", method, st.Message())
	case codes.InvalidArgument:
		log.Printf("%s: Неверный аргумент (код=InvalidArgument)", method)
		// Выводим детали ошибки
		for _, detail := range st.Details() {
			if errDetail, ok := detail.(*pb.ErrorDetails); ok {
				log.Printf("  → Поле: %s, Причина: %s, Значение: %s",
					errDetail.Field, errDetail.Reason, errDetail.Value)
			}
		}
	case codes.AlreadyExists:
		log.Printf("%s: Пользователь уже существует (код=AlreadyExists, сообщение=%s)", method, st.Message())
	case codes.Unauthenticated:
		log.Printf("%s: Не авторизован (код=Unauthenticated, сообщение=%s)", method, st.Message())
	case codes.DeadlineExceeded:
		log.Printf("%s: Таймаут (код=DeadlineExceeded, сообщение=%s)", method, st.Message())
	case codes.Internal:
		log.Printf("%s: Внутренняя ошибка сервера (код=Internal, сообщение=%s)", method, st.Message())
	case codes.Canceled:
		log.Printf("%s: Запрос отменён (код=Canceled)", method)
	default:
		log.Printf("%s: Другая ошибка (код=%s, сообщение=%s)", method, st.Code().String(), st.Message())
	}
}

func main() {
	conn, err := grpc.NewClient("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)

	// Создаём контекст с metadata для авторизации
	md := metadata.Pairs("authorization", "Bearer valid-token")
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	//1. УСПЕШНЫЙ ЗАПРОС

	log.Println("\n=== 1. Успешный запрос ===")
	ctx1, cancel1 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel1()

	resp, err := client.GetUser(ctx1, &pb.GetUserRequest{UserId: "1"})
	if err != nil {
		log.Printf("❌ Ошибка: %v", err)
	} else {
		log.Printf("✅ Пользователь: ID=%s, Name=%s, Email=%s", resp.Id, resp.Name, resp.Email)
	}

	//2. NOT FOUND

	log.Println("\n=== 2. NotFound ===")
	ctx2, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()

	_, err = client.GetUser(ctx2, &pb.GetUserRequest{UserId: "999"})
	handleError(err, "GetUser (not found)")

	//3. INVALID ARGUME

	log.Println("\n=== 3. InvalidArgument (с деталями) ===")
	ctx3, cancel3 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel3()

	_, err = client.CreateUser(ctx3, &pb.CreateUserRequest{
		Email: "",
		Name:  "A",
		Age:   150,
	})
	handleError(err, "CreateUser (invalid)")

	//4. ALREADY EXISTS

	log.Println("\n=== 4. AlreadyExists ===")
	ctx4, cancel4 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel4()

	_, err = client.CreateUser(ctx4, &pb.CreateUserRequest{
		Email: "alice@example.com",
		Name:  "Alice2",
		Age:   28,
	})
	handleError(err, "CreateUser (already exists)")

	//5. UNAUTHENTICATE

	log.Println("\n=== 5. Unauthenticated ===")
	ctx5, cancel5 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5()

	_, err = client.GetUser(ctx5, &pb.GetUserRequest{UserId: "1"})
	handleError(err, "GetUser (no token)")

	//6. DEADLINE EXCE

	log.Println("\n=== 6. DeadlineExceeded ===")
	ctx6, cancel6 := context.WithTimeout(ctx, 2*time.Second)
	defer cancel6()

	_, err = client.GetUser(ctx6, &pb.GetUserRequest{UserId: "slow"})
	handleError(err, "GetUser (slow)")

	//7. УСПЕШНОЕ СОЗД

	log.Println("\n=== 7. Успешное создание пользователя ===")
	ctx7, cancel7 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel7()

	createResp, err := client.CreateUser(ctx7, &pb.CreateUserRequest{
		Email: "charlie@example.com",
		Name:  "Charlie",
		Age:   28,
	})
	if err != nil {
		handleError(err, "CreateUser (success)")
	} else {
		log.Printf("✅ Создан пользователь: ID=%s, Name=%s", createResp.User.Id, createResp.User.Name)
	}
}
