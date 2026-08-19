package __4_unary_rpc
import (
"context"
"log"
"sync"
"time"

"google.golang.org/grpc"
"google.golang.org/grpc/codes"
"google.golang.org/grpc/credentials/insecure"
"google.golang.org/grpc/metadata"
"google.golang.org/grpc/status"
pb "github.com/" замени на свой путь
)

var(
	once sync.Once
	conn *grpc.ClientConn
)

// getConn — переиспользуемое соединение (connection pooling)
func getConn() *grpc.ClientConn {
	once.Do(func() {
		var err error
		conn, err = grpc.NewClient("localhost:50051",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Fatal(err)
		}
	})
	return conn
}

func main(){
	conn:= getConn()
	defer conn.Close()

	client:= pb.NewUserServiceClient(conn)

	// 1.GetUser
	ctx,cancel:= context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()

	// Добавляем metadata с JWT-токеном
	md:= metadata.Pairs("authorization", "Bearer secret-token")
	ctx = metadata.NewOutgoingContext(ctx,md)

	resp,err:= client.GetUser(ctx,&pb.GetUserRequest{Id: "1"})
	if err != nil {
		handleError(err, "GetUser")
	} else {
		log.Printf("✅ GetUser: ID=%s, Name=%s, Email=%s, Age=%d", resp.Id, resp.Name, resp.Email, resp.Age)
	}

	//2. GetUser (не найден)
	_, err = client.GetUser(ctx, &pb.GetUserRequest{Id: "999"})
	handleError(err, "GetUser (not found)")

	//3. CreateUser (успех)
	createResp,err:= client.CreateUser(ctx, &pb.CreateUserRequest{
		Email: "KokoJoja.kj@yandex.ru",
		Name: "KokoJAja",
		Age: 39,
	})
	if err != nil {
		handleError(err, "CreateUser")
	} else {
		log.Printf("✅ Created user: ID=%s, Name=%s", createResp.User.Id, createResp.User.Name)
	}

	//4. CreateUser (уже существует)
	_, err = client.CreateUser(ctx, &pb.CreateUserRequest{
		Email: "alice@google.com",
		Name:  "Pepe",
		Age:   25,
	})
	handleError(err, "CreateUser (duplicate)")

	//5. UpdateUser (частичное обновление)
	updateResp, err := client.UpdateUser(ctx, &pb.UpdateUserRequest{
		User: &pb.User{
			Id:    "2",
			Name:  "Robert",
			Email: "bob@example.com",
		},
		UpdateMask: nil, // nil = обновить все поля
	})
	if err != nil {
		handleError(err, "UpdateUser")
	} else {
		log.Printf("✅ Updated user: ID=%s, Name=%s", updateResp.User.Id, updateResp.User.Name)
	}

	//6. UpdateUser (частичное обновление с маской)
	// Обновляем только имя
	updateResp2,err:= client.Updateruser(ctx, &pb.UpdateUserRequest{
		User: &pb.User{
			Id: "1",
			Name: "Rumka"
		},
		UpdateMask: &pb.FieldMask{Paths: []string{"name"}},
	})
	if err != nil {
		handleError(err, "UpdateUser (partial)")
	} else {
		log.Printf("✅ Partial update: ID=%s, Name=%s, Email=%s", updateResp2.User.Id, updateResp2.User.Name, updateResp2.User.Email)
	}
}

// handleError — обработка gRPC-ошибок с разбором статусов
func handleError(err error, method string) {
	if err == nil {
		return
	}
	st, ok := status.FromError(err)
	if ok {
		switch st.Code() {
		case codes.NotFound:
			log.Printf("%s: пользователь не найден (%s)", method, st.Message())
		case codes.InvalidArgument:
			log.Printf("%s: неверный аргумент (%s)", method, st.Message())
		case codes.AlreadyExists:
			log.Printf("%s: пользователь уже существует (%s)", method, st.Message())
		case codes.Unauthenticated:
			log.Printf("%s: не авторизован (%s)", method, st.Message())
		case codes.DeadlineExceeded:
			log.Printf("%s: таймаут (%s)", method, st.Message())
		default:
			log.Printf("%s: ошибка (код=%s, сообщение=%s)", method, st.Code().String(), st.Message())
		}
	} else {
		log.Printf("%s: не-gRPC ошибка: %v", method, err)
	}
}