package __1_metadata

import (
	"context"
	"log"
	"time"

	pb "github.com/example/metadata-example/proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func main() {
	// Подключаемся к серверу
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.newUserServiceClient(conn)

	//СЦЕНАРИЙ 1: ВАЛИДНЫЙ ТОКЕН
	log.Println("\n=== Сценарий 1: Валидный JWT ===")

	// Генерируем JWT для пользователя "alice"
	token, _ := generateJWT("alice")

	// Создаём metadata с токеном
	md := metadata.Pairs(
		"authorization", "Bearer"+token,
		"x-client-version", "2.0.0",
	)

	ctx := metadata.NewOutgoingContext(context.Background(), md)

	// Переменные для захвата headers/trailers
	var header, trailer metadata.MD

	// Вызываем метод
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := client.GetUser(ctx, &pb.GetUserRequest{UserId: "1"},
		grpc.Header(&header), grpc.Trailer(&trailer))
	if err != nil {
		log.Printf("Ошибка: %v", err)
	} else {
		log.Printf("Ответ: ID=%s, Name=%s, Email=%s", resp.Id, resp.Name, resp.Email)
	}

	// Выводим захваченные headers и trailers
	log.Printf("Headers: %+v", header)
	log.Printf("Trailers: %+v", trailer)

	// ─── СЦЕНАРИЙ 2: НЕВАЛИДНЫЙ ТОКЕН ────────────────────────────────────

	log.Println("\n=== Сценарий 2: Невалидный JWT ===")

	invalidToken := "invalid-token"
	md2 := metadata.Pairs("authorization", "Bearer "+invalidToken)
	ctx2 := metadata.NewOutgoingContext(context.Background(), md2)

	ctx2, cancel2 := context.WithTimeout(ctx2, 5*time.Second)
	defer cancel2()

	_, err = client.GetUser(ctx2, &pb.GetUserRequest{UserId: "1"})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.Unauthenticated {
			log.Printf("✅ Ожидаемая ошибка: %v", st.Message())
		} else {
			log.Printf("❌ Неожиданная ошибка: %v", err)
		}
	}

	//СЦЕНАРИЙ 3: НЕТ ТОКЕНА

	log.Println("\n=== Сценарий 3: Нет токена ===")

	ctx3 := context.Background()
	ctx3, cancel3 := context.WithTimeout(ctx3, 5*time.Second)
	defer cancel3()

	_, err = client.GetUser(ctx3, &pb.GetUserRequest{UserId: "1"})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.Unauthenticated {
			log.Printf("Ожидаемая ошибка: %v", st.Message())
		} else {
			log.Printf("Неожиданная ошибка: %v", err)
		}
	}

	//СЦЕНАРИЙ 4: ИСПОЛЬЗОВАНИЕ APPENDTOOUTGOINGCONTEXT
	log.Println("\n=== Сценарий 4: AppendToOutgoingContext ===")

	// Создаём контекст с существующей metadata
	ctx4 := context.Background()
	ctx4 = metadata.AppendToOutgoingContext(ctx4,
		"authorization", "Bearer "+token,
		"x-request-id", "123")

	// Добавляем ещё один ключ
	ctx4 = metadata.AppendToOutgoingContext(ctx4, "x-extra", "extra-value")

	ctx4, cancel4 := context.WithTimeout(ctx4, 5*time.Second)
	defer cancel4()

	var header4, trailer4 metadata.MD
	resp4, err := client.GetUser(ctx4, &pb.GetUserRequest{UserId: "2"},
		grpc.Header(&header4),
		grpc.Trailer(&trailer4),
	)
	if err != nil {
		log.Printf("Ошибка: %v", err)
	} else {
		log.Printf("Пользователь: %s", resp4.Name)
	}
	log.Printf("Headers: %+v", header4)
}
