package l05_Network

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

//TCP
/*
TCP — это очень надежный, ориентированный на соединение протокол. Он обеспечивает упорядоченную передачу данных, автоматом исправляя ошибки.
Основные черты TCP:
 *Надежность: подтверждения и повторная отправка потерянных пакетов.
 *Упорядоченность: передача данных в том порядке, в котором они были отправлены.
 *Контроль перегрузки: предотвращение коллапса сети за счет контроля скорости передачи данных.
Go имеет пакет net для создания серверов и клиентов TCP. В этом пакете есть несколько функций, которые позволяют управлять сетевыми соединениями.
*/

// Пример
/*
Реализуем простую систему обмена сообщениями между сервером и клиентом.
Сервер будет слушать входящие TCP подключения, принимать сообщения от клиентов, и отправлять простое подтверждение о получении сообщения:
*/

func main() {

}

func mainServer() {
	// определяем порт для прослушивания
	const PORT = ":9090"
	listener, err := net.Listen("tcp", PORT)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	// закрываем listener при завершении программы
	defer listener.Close()
	fmt.Println("Server is listening on " + PORT)

	for {
		// принимаем входящее подключение
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err.Error())
			os.Exit(1)
		}
		fmt.Println("Connect with", conn.RemoteAddr().String())
		// обрабатываем подключение в отдельной горутине
		go handleRequest(conn)
	}
}

func handleRequest(conn net.Conn) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// читаем данные от клиента
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		clientMessage := scanner.Text()
		fmt.Printf("Received from client: %s\n", clientMessage)

		// ответ с таймаутом на запись
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_, err := conn.Write([]byte("Ok\n"))
		if err != nil {
			return
		}

		// сброс таймаута для следующего чтения
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading:", err.Error())
		}
	}
}

func mainClient() {
	// соединяемся с сервером
	conn, err := net.Dial("tcp", "localhost:9090")
	if err != nil {
		fmt.Println("Error connecting:", err.Error())
		os.Exit(1)
	}
	defer conn.Close()

	// Читаем ответы от сервера в отдельной горутине
	go func() {
		reader := bufio.NewReader(conn)
		for {
			response, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("Read error: %v", err)
				}
				return
			}
			fmt.Printf("\rServer: %s> ", response[:len(response)-1])
		}
	}()

	// Основной цикл отправки сообщений
	scaner := bufio.NewScanner(os.Stdin)
	for scaner.Scan() {
		text := scaner.Text()
		if text == "/quit" {
			return
		}
		_, err := conn.Write([]byte(text + "\n"))
		if err != nil {
			log.Printf("Write error: %v", err)
			return
		}
		fmt.Print("> ")
	}
}
