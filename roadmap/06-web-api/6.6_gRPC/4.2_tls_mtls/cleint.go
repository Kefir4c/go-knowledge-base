package __2_tls_mtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	pb "github.com/"
)

// loadTLSClientCredentials — для обычного TLS
func loadTLSClientCredentials() (credentials.TransportCredentials, error) {
	// Используем системное хранилище CA
	return credentials.NewClientTLSFromCert(nil, ""), nil
}

// loadMTLSClientCredentials — для mTLS с клиентским сертификатом
func loadMTLSClientCredentials() (credentials.TransportCredentials, error) {
	// 1. Загружаем клиентский сертификат и ключ
	clientCert, err := tls.LoadX509KeyPair(
		"certs/client_cert.pem",
		"certs/cleint_key.pem",
	)
	if err != nil {
		return nil, err
	}

	// 2. Загружаем CA-сертификат для проверки сервера
	caCert, err := os.ReadFile("certs/ca_cert.pem")
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, err
	}

	// 3. Создаём TLS-конфиг с mTLS
	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS13,
		ServerName:   "localhost",
	}

	return credentials.NewTLS(&tlsConfig), nil
}

func main() {
	// Выбираем режим по переменной окружения
	mode := os.Getenv("TLS_MODE")
	if mode == "" {
		mode = "mtls"
	}

	var creds credentials.TransportCredentials
	var err error

	if mode == "mtls" {
		log.Println("🔐 Клиент подключается с mTLS")
		creds, err = loadMTLSClientCredentials()
	} else {
		log.Println("🔒 Клиент подключается с TLS")
		creds, err = loadTLSClientCredentials()
	}
	if err != nil {
		log.Fatal(err)
	}

	// Создаём соединение
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)

	// Выполняем запрос
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetUser(ctx, &pb.GetUserREquest{userId: "1"})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.PermissionDenied:
				log.Printf("Доступ запрещён: %v", st.Message())
			case codes.Unauthenticated:
				log.Printf("Не аутентифицирован: %v", st.Message())
			default:
				log.Printf("Ошибка: %v", st.Message())
			}
		} else {
			log.Printf("Не-gRPC ошибка: %v", err)
		}
		return
	}
	log.Printf("Пользователь: ID=%s, Name=%s, Email=%s", resp.Id, resp.Name, resp.Email)
}
