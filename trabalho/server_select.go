package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	maxFDs        = 1024
	selectBufSize = 4096
	selectTimeout = 60 * time.Second
)

var selectPort = flag.Int("port", 5002, "TCP port for the select-based server")

type clientState struct {
	fd   int
	addr string
}

func main() {
	flag.Parse()

	listenFD, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	if err != nil {
		log.Fatalf("socket error: %v", err)
	}
	defer unix.Close(listenFD)

	if err := unix.SetsockoptInt(listenFD, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		log.Fatalf("setsockopt SO_REUSEADDR: %v", err)
	}

	if err := unix.SetNonblock(listenFD, true); err != nil {
		log.Fatalf("failed to set listener non-blocking: %v", err)
	}

	sockaddr := &unix.SockaddrInet4{Port: *selectPort}
	if err := unix.Bind(listenFD, sockaddr); err != nil {
		log.Fatalf("bind error: %v", err)
	}

	if err := unix.Listen(listenFD, 128); err != nil {
		log.Fatalf("listen error: %v", err)
	}

	log.Printf("[select] listening on :%d", *selectPort)

	clients := map[int]*clientState{}
	buffer := make([]byte, selectBufSize)

	for {
		var readFDs unix.FdSet
		fdSet(listenFD, &readFDs)
		maxFD := listenFD

		for fd := range clients {
			fdSet(fd, &readFDs)
			if fd > maxFD {
				maxFD = fd
			}
		}

		timeout := unix.NsecToTimeval(selectTimeout.Nanoseconds())
		ready, err := unix.Select(maxFD+1, &readFDs, nil, nil, &timeout)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			log.Fatalf("select error: %v", err)
		}

		if ready == 0 {
			continue
		}

		if fdIsSet(listenFD, &readFDs) {
			acceptNewClient(listenFD, clients)
		}

		for fd := range clients {
			if fdIsSet(fd, &readFDs) {
				if err := handleSelectRead(fd, clients, buffer); err != nil {
					log.Printf("[select] closing fd %d: %v", fd, err)
					closeClient(fd, clients)
				}
			}
		}
	}
}

func acceptNewClient(listenFD int, clients map[int]*clientState) {
	connFD, sa, err := unix.Accept(listenFD)
	if err != nil {
		log.Printf("accept error: %v", err)
		return
	}

	if len(clients) >= maxFDs-1 {
		log.Printf("too many clients, rejecting fd %d", connFD)
		unix.Close(connFD)
		return
	}

	if err := unix.SetNonblock(connFD, true); err != nil {
		log.Printf("failed to set non-blocking mode: %v", err)
		unix.Close(connFD)
		return
	}

	addr := stringifySockaddr(sa)
	clients[connFD] = &clientState{fd: connFD, addr: addr}
	log.Printf("[select] accepted fd %d from %s", connFD, addr)
}

func handleSelectRead(fd int, clients map[int]*clientState, buffer []byte) error {
	n, err := unix.Read(fd, buffer)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("peer closed connection")
	}

	payload := strings.TrimSpace(string(buffer[:n]))
	response := fmt.Sprintf("[select %d] %s\n", fd, strings.ToUpper(payload))

	if _, err := unix.Write(fd, []byte(response)); err != nil {
		return err
	}

	return nil
}

func closeClient(fd int, clients map[int]*clientState) {
	unix.Close(fd)
	delete(clients, fd)
}

func stringifySockaddr(sa unix.Sockaddr) string {
	switch v := sa.(type) {
	case *unix.SockaddrInet4:
		return fmt.Sprintf("%d.%d.%d.%d:%d", v.Addr[0], v.Addr[1], v.Addr[2], v.Addr[3], v.Port)
	case *unix.SockaddrInet6:
		return fmt.Sprintf("[%x:%x:%x:%x:%x:%x:%x:%x]:%d",
			uint16(v.Addr[0])<<8|uint16(v.Addr[1]),
			uint16(v.Addr[2])<<8|uint16(v.Addr[3]),
			uint16(v.Addr[4])<<8|uint16(v.Addr[5]),
			uint16(v.Addr[6])<<8|uint16(v.Addr[7]),
			uint16(v.Addr[8])<<8|uint16(v.Addr[9]),
			uint16(v.Addr[10])<<8|uint16(v.Addr[11]),
			uint16(v.Addr[12])<<8|uint16(v.Addr[13]),
			uint16(v.Addr[14])<<8|uint16(v.Addr[15]),
			v.Port)
	default:
		return "unknown"
	}
}

func fdSet(fd int, set *unix.FdSet) {
	set.Bits[fd/64] |= 1 << (uint(fd) % 64)
}

func fdIsSet(fd int, set *unix.FdSet) bool {
	return set.Bits[fd/64]&(1<<(uint(fd)%64)) != 0
}
