package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

var (
	threadPort    = flag.Int("port", 5001, "TCP port for the goroutine-per-connection server")
	sessionIDSeed uint64
	readTimeout   = 45 * time.Second
	writeTimeout  = 5 * time.Second
)

func main() {
	flag.Parse()

	addr := fmt.Sprintf(":%d", *threadPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	log.Printf("[goroutine] listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		sessionID := atomic.AddUint64(&sessionIDSeed, 1)
		go handleConnection(conn, sessionID)
	}
}

func handleConnection(conn net.Conn, sessionID uint64) {
	defer conn.Close()

	if _, err := conn.Write([]byte("Welcome to the goroutine-based echo server.\n")); err != nil {
		log.Printf("[session %d] greeting failed: %v", sessionID, err)
		return
	}

	scanner := bufio.NewScanner(conn)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			log.Printf("[session %d] read deadline: %v", sessionID, err)
			return
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				log.Printf("[session %d] scan error: %v", sessionID, err)
			}
			return
		}

		line := scanner.Text()
		response := fmt.Sprintf("[session %d] %s\n", sessionID, strings.ToUpper(strings.TrimSpace(line)))

		if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
			log.Printf("[session %d] write deadline: %v", sessionID, err)
			return
		}

		if _, err := conn.Write([]byte(response)); err != nil {
			log.Printf("[session %d] write error: %v", sessionID, err)
			return
		}
	}
}
