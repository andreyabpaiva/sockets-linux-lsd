package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"time"
)

var (
	targetHost = flag.String("host", "127.0.0.1", "server hostname or IP")
	targetPort = flag.Int("port", 5000, "server TCP port")
	message    = flag.String("message", "hello from client", "payload sent to the server")
	count      = flag.Int("count", 1, "number of sequential requests to send")
	timeout    = flag.Duration("timeout", 5*time.Second, "per-connection timeout")
)

func main() {
	flag.Parse()

	for i := 0; i < *count; i++ {
		if err := dialOnce(i + 1); err != nil {
			log.Fatalf("request %d failed: %v", i+1, err)
		}
	}
}

func dialOnce(seq int) error {
	address := fmt.Sprintf("%s:%d", *targetHost, *targetPort)
	conn, err := net.DialTimeout("tcp", address, *timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(*timeout)); err != nil {
		return err
	}

	payload := fmt.Sprintf("%s #%d\n", *message, seq)
	if _, err := conn.Write([]byte(payload)); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	fmt.Printf("[response %d] %s", seq, response)
	return nil
}
