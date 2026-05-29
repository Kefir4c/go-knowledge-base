package l05_Network

import (
	"fmt"
	"log"
	"net"
	"time"
)

//UDP
/*
UDP — это простой протокол без установления соединения, который не гарантирует доставку, порядок или интегральность данных. Но зато, он дает минимум задержки.
Основные черты UDP:
* Отсутствие процесса установления соединения уменьшает задержку.
* Меньше накладных расходов, больше производительности.
*/

// Пример
/*
Реализуем простую систему обмена сообщениями между сервером и клиентом.
Сервер будет слушать входящие TCP подключения, принимать сообщения от клиентов, и отправлять простое подтверждение о получении сообщения:
*/

const (
	maxDatagramSize = 65536
	readTimeout     = 30 * time.Second
)

func mainServerUDP() {
	conn, err := net.ListenPacket("udp", ":8080")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer conn.Close()

	log.Println("UDP echo server listening on :8080")

	buf := make([]byte, maxDatagramSize)

	for {
		conn.SetReadDeadline(time.Now().Add(readTimeout))

		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // просто таймаут, продолжаем
			}
			log.Printf("Read error: %v", err)
			continue
		}

		// Не блокируем основной цикл, если обработка долгая
		go func(data []byte, clientAddr net.Addr) {
			// Эхо-ответ
			if _, err := conn.WriteTo(data, clientAddr); err != nil {
				log.Printf("Write error to %s: %v", clientAddr, err)
			}
		}(buf[:n], addr)
	}
}

func mainClientUDP() {
	// 1. Разбираем адрес сервера
	addr, err := net.ResolveUDPAddr("udp", "localhost:8080")
	if err != nil {
		fmt.Println("Error resolving address:", err)
		return
	}

	// 2. Создаём "соединённый" UDP-сокет
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Println("Error creating socket:", err)
		return
	}
	defer conn.Close()

	// 3. Устанавливаем таймаут на чтение (важно!)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// 4. Отправляем данные
	data := "Hello, server"
	n, err := conn.Write([]byte(data))
	if err != nil {
		fmt.Println("Error sending datagram:", err)
		return
	}
	fmt.Printf("Sent %d bytes: %s\n", n, data)

	buf := make([]byte, 1024)

	// 6. Читаем ответ
	n, err = conn.Read(buf)
	if err != nil {
		// Проверяем, не таймаут ли это
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("Timeout: server did not respond")
		} else {
			fmt.Println("Error reading datagram:", err)
		}
		return
	}

	// 7. Выводим только полученные байты
	if n == 0 {
		fmt.Println("Received empty response")
		return
	}
	fmt.Printf("Received %d bytes from server: %s\n", n, string(buf[:n]))
}
