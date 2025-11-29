package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	childFDIndex   = 3
	readTimeout    = 30 * time.Second
	writeTimeout   = 5 * time.Second
	defaultMessage = "Type something to receive an echoed response.\n"
)

var (
	listenPort = flag.Int("port", 5000, "TCP port used by the parent listener")
	childMode  = flag.Bool("child", false, "internal flag used when handling a connection in a forked process")
)

func main() {
	flag.Parse()

	if *childMode {
		runChild()
		return
	}

	addr := fmt.Sprintf(":%d", *listenPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	log.Printf("[fork] listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		if err := spawnChild(conn); err != nil {
			log.Printf("child spawn error: %v", err)
			conn.Close()
		}
	}
}

func spawnChild(conn net.Conn) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return fmt.Errorf("connection is not TCP")
	}

	file, err := tcpConn.File()
	if err != nil {
		return fmt.Errorf("duplicating connection descriptor: %w", err)
	}
	defer file.Close()

	// The original connection must be closed because File returns a duplicate descriptor.
	conn.Close()

	cmd := exec.Command(os.Args[0], "--child", fmt.Sprintf("--port=%d", *listenPort))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = []*os.File{file}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting child process: %w", err)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("child exited with error: %v", err)
		}
	}()

	return nil
}

func runChild() {
	connFile := os.NewFile(uintptr(childFDIndex), "inherited-conn")
	if connFile == nil {
		log.Fatal("missing inherited connection descriptor")
	}
	defer connFile.Close()

	conn, err := net.FileConn(connFile)
	if err != nil {
		log.Fatalf("failed to build net.Conn from descriptor: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(defaultMessage)); err != nil {
		log.Printf("failed to send greeting: %v", err)
		return
	}

	handleSession(conn)
}

func handleSession(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			log.Printf("read deadline error: %v", err)
			return
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				log.Printf("read error: %v", err)
			}
			return
		}

		payload := strings.TrimSpace(scanner.Text())
		if payload == "" {
			continue
		}

		response := fmt.Sprintf("[fork child %d] %s\n", os.Getpid(), strings.ToUpper(payload))
		if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
			log.Printf("write deadline error: %v", err)
			return
		}
		if _, err := conn.Write([]byte(response)); err != nil {
			log.Printf("write error: %v", err)
			return
		}
	}
}
