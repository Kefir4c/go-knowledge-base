package clientstreaming

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/"
)

const chunkSize = 64 * 1024 // 64KB — оптимальный размер для gRPC [citation:5]

func main() {
	conn, err := grpc.NewClient("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewFileServiceClient(conn)

	//1. UPLOAD FILE

	if err := uploadFile(client); err != nil {
		log.Printf("Ошибка загрузки: %v", err)
	}

	//2. DOWNLOAD FILE

	if err := downloadFile(client, "file-1234567890"); err != nil {
		log.Printf("Ошибка скачивания: %v", err)
	}
}

func uploadFile(client pb.FileServiceClient) error {
	log.Println("\n=== Загрузка файла ===")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := client.UploadFile(ctx)
	if err != nil {
		return err
	}

	// Открываем файл для загрузки
	filePath := "testfile.jpg" // укажи свой файл
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Файл testfile.jpg не найден, создаём тестовый файл")
		// Создаём тестовый файл
		file, err = createTestFile()
		if err != nil {
			return err
		}
		defer file.Close()
	} else {
		defer file.Close()
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	// 1. Отправляем метаданные
	metadata := &pb.FileMetadata{
		Filename:    fileInfo.Name(),
		ContentType: "image/jpeg",
		Size:        fileInfo.Size(),
		UserId:      "user-123",
	}
	if err := stream.Send(&pb.UploadFileRequest{
		Data: &pb.UploadFileRequest_Metadata{Metadata: metadata},
	}); err != nil {
		return err
	}
	log.Printf("Отправлены метаданные: %s, размер=%d", fileInfo.Name(), fileInfo.Size())

	// 2. Отправляем чанки
	buffer := make([]byte, chunkSize)
	var sent int64

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if err := stream.Send(&pb.UploadFileRequest{
			Data: &pb.UploadFileRequest_Chunk{Chunk: buffer[:n]},
		}); err != nil {
			return err
		}

		sent += int64(n)
		log.Printf("Отправлен чанк: %d байт (всего %.1f%%)",
			n, float64(sent)/float64(fileInfo.Size())*100)
	}

	// 3. Закрываем стрим и получаем ответ
	log.Println("Завершение загрузки...")
	resp, err := stream.CloseAndRecv()
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.DeadlineExceeded:
				return fmt.Errorf("таймаут загрузки")
			default:
				return fmt.Errorf("ошибка сервера: %v", st.Message())
			}
		}
		return err
	}

	log.Printf("Файл загружен! ID: %s, Сообщение: %s", resp.FileId, resp.Message)
	log.Printf("Время загрузки: %v", resp.UploadedAt.AsTime())
	return nil
}

func downloadFile(client pb.FileServiceClient, fileID string) error {
	log.Println("\n=== Скачивание файла ===")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := client.DownloadFile(ctx, &pb.DownloadFileRequest{FileId: fileID})
	if err != nil {
		return err
	}

	// Получаем информацию о файле
	firstResp, err := stream.Recv()
	if err != nil {
		return err
	}

	info := firstResp.GetInfo()
	if info == nil {
		return fmt.Errorf("ожидалась информация о файле")
	}
	log.Printf("Информация: имя=%s, размер=%d", info.Filename, info.Size)

	// Создаём файл для сохранения
	outputPath := filepath.Join("./downloads", info.Filename)
	if err := os.MkdirAll("./downloads", 0755); err != nil {
		return err
	}
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Получаем чанки
	var received int64
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		chunk := resp.GetChunk()
		if chunk == nil {
			continue
		}

		if _, err := outFile.Write(chunk); err != nil {
			return err
		}
		received += int64(len(chunk))
		log.Printf("Получен чанк: %d байт (всего %.1f%%)",
			len(chunk), float64(received)/float64(info.Size)*100)
	}

	log.Printf("Файл сохранён: %s", outputPath)
	return nil
}

//ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ

func createTestFile() (*os.File, error) {
	// Создаём директорию, если её нет
	if err := os.MkdirAll(".", 0755); err != nil {
		return nil, err
	}

	file, err := os.Create("testfile.jpg")
	if err != nil {
		return nil, err
	}

	// Записываем тестовые данные (имитация JPEG)
	data := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, // JPEG SOI + APP0
		0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, // JFIF
	}
	for i := 0; i < 1024; i++ {
		data = append(data, byte(i%256))
	}
	if _, err := file.Write(data); err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}
	return file, nil
}
