package __3_graceful_shutdown

import (
	"context"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewTaskServiceClient(conn)

	// 1. Unary RPC
	log.Println("=== Unary ProcessTask ===")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.ProcessTask(ctx, &pb.ProcessTaskRequest{
		taskId:  "task-1",
		DelayMs: 500,
	})
	if err != nil {
		st, _ := status.FromError(err)
		log.Printf("Ошибка: %v", st.Message())
	} else {
		log.Printf("Задача %s: статус=%s, прогресс=%d%%", resp.TaskId, resp.Status, resp.Progress)
	}

	// 2. SERVER-STREAMING
	log.Println("\n=== Server-Streaming ProcessTaskStream (задержка 1с) ===")
	log.Println("Нажми Ctrl+C на сервере — стрим завершится корректно")

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	stream, err := client.ProcessTaskStream(ctx2, &pb.ProcessTaskRequest{
		TaskId:  "task-stream-1",
		DelayMs: 1000, // 1 секунда задержки
	})
	if err != nil {
		log.Fatalf("❌ Stream error: %v", err)
	}

	for {
		status, err := stream.Recv()
		if err == io.EOF {
			log.Println("Стрим завершён")
			break
		}
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code() == 1 {
				log.Println("Стрим отменён (сервер завершается)")
			} else {
				log.Printf("Ошибка стрима: %v", st.Message())
			}
			break
		}
		log.Printf("Задача %s: статус=%s, прогресс=%d%%",
			status.TaskId, status.Status, status.Progress)
	}
}
