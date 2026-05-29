package l05_Network

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

//QUIC
/*
QUIC — это уже современный протокол, разработанный Google и стандартизированный IETF, который стремится улучшить
производительность соединений, предоставляемых TCP, с добавлением функций безопасности, аналогичных TLS/SSL.
QUIC работает поверх UDP и предназначен для снижения задержек соединения, поддерживает мультиплексирование потоков без
взаимного блокирования и управляет потерей пакетов более лучше, чем TCP.

Основные черты QUIC:

* Уменьшение задержек:уменьшает задержку соединения за счет использования 0-RTT и 1-RTT рукопожатий.
* Безопасность: включает встроенное шифрование на уровне соединений.
* Мультиплексирование: позволяет нескольким потокам данных обмениваться данными в рамках одного соединения без взаимной блокировки.

Работа с протоколом QUIC в Go проходит с помощью библиотеки quic-go, которая представляет собой полноценную реализацию QUIC.
Эта библиотека поддерживает множество стандартов, включая HTTP/3.
*/

/*
Пример
Сервер будет слушать на определённом порту и отвечать на входящие сообщения от клиента:
*/
func mainServerQUIC() {
	listener, err := quic.ListenAddr("localhost:4242", generateTLSConfig(), nil)
	if err != nil {
		log.Fatal("Failed to listen:", err)
	}

	for {
		sess, err := listener.Accept(context.Background())
		if err != nil {
			log.Fatal("Failed to accept session:", err)
		}

		go func() {
			for {
				stream, err := sess.AcceptStream(context.Background())
				if err != nil {
					log.Fatal("Failed to accept stream:", err)
				}

				// эхо полученных данных обратно клиенту
				_, err = io.Copy(stream, stream)
				if err != nil {
					log.Fatal("Failed to echo data:", err)
				}
			}
		}()
	}
}

func generateTLSConfig() *tls.Config {
	key, cert := generateKeys() // Допустим, что функция generateKeys генерирует TLS ключ и сертификат
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"quic-echo-example"},
	}
}

func mainClientQUIC() {
	// В новых версиях DialAddr возвращает *quic.Conn (структуру)
	conn, err := quic.DialAddr(
		context.Background(),
		"localhost:4242",
		&tls.Config{InsecureSkipVerify: true, NextProtos: []string{"quic-echo"}},
		nil,
	)
	if err != nil {
		log.Fatal("DialAddr error:", err)
	}
	defer conn.CloseWithError(0, "")

	// Открываем стрим
	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Fatal("OpenStreamSync error:", err)
	}
	defer stream.Close()

	// Отправляем сообщение
	message := "Hello, QUIC!"
	_, err = stream.Write([]byte(message))
	if err != nil {
		log.Fatal("Write error:", err)
	}

	// Читаем ответ с таймаутом
	stream.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil {
		log.Fatal("Read error:", err)
	}

	fmt.Printf("Sent: %s\nReceived: %s\n", message, string(buf[:n]))
}
