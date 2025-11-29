package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	maxEpollEvents = 256
	epollBufSize   = 4096
)

var epollPort = flag.Int("port", 5003, "TCP port for the epoll-based server")

func main() {
	flag.Parse()

	listenFD, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_NONBLOCK, 0)
	if err != nil {
		log.Fatalf("socket error: %v", err)
	}
	defer unix.Close(listenFD)

	if err := unix.SetsockoptInt(listenFD, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		log.Fatalf("setsockopt SO_REUSEADDR: %v", err)
	}

	addr := &unix.SockaddrInet4{Port: *epollPort}
	if err := unix.Bind(listenFD, addr); err != nil {
		log.Fatalf("bind error: %v", err)
	}

	if err := unix.Listen(listenFD, 512); err != nil {
		log.Fatalf("listen error: %v", err)
	}

	epollFD, err := unix.EpollCreate1(0)
	if err != nil {
		log.Fatalf("epoll create: %v", err)
	}
	defer unix.Close(epollFD)

	event := &unix.EpollEvent{Events: unix.EPOLLIN, Fd: int32(listenFD)}
	if err := unix.EpollCtl(epollFD, unix.EPOLL_CTL_ADD, listenFD, event); err != nil {
		log.Fatalf("epoll ctl add listen fd: %v", err)
	}

	log.Printf("[epoll] listening on :%d", *epollPort)

	events := make([]unix.EpollEvent, maxEpollEvents)
	buffer := make([]byte, epollBufSize)

	for {
		n, err := unix.EpollWait(epollFD, events, -1)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			log.Fatalf("epoll wait: %v", err)
		}

		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)
			switch {
			case fd == listenFD:
				acceptEpollClients(listenFD, epollFD)
			case events[i].Events&(unix.EPOLLHUP|unix.EPOLLERR) != 0:
				closeFD(fd)
			default:
				if err := handleEpollRead(fd, events[i].Events, buffer); err != nil {
					log.Printf("[epoll] closing fd %d: %v", fd, err)
					closeFD(fd)
				}
			}
		}
	}
}

func acceptEpollClients(listenFD, epollFD int) {
	for {
		connFD, sa, err := unix.Accept4(listenFD, unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC)
		if err != nil {
			if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
				return
			}
			log.Printf("accept4 error: %v", err)
			return
		}

		event := &unix.EpollEvent{Events: unix.EPOLLIN | unix.EPOLLRDHUP, Fd: int32(connFD)}
		if err := unix.EpollCtl(epollFD, unix.EPOLL_CTL_ADD, connFD, event); err != nil {
			log.Printf("epoll ctl add fd %d: %v", connFD, err)
			unix.Close(connFD)
			continue
		}

		log.Printf("[epoll] accepted fd %d from %s", connFD, stringifySockaddr(sa))
	}
}

func handleEpollRead(fd int, events uint32, buffer []byte) error {
	if events&unix.EPOLLIN == 0 {
		return fmt.Errorf("unexpected events mask %#x", events)
	}

	n, err := unix.Read(fd, buffer)
	if err != nil {
		return err
	}

	if n == 0 {
		return fmt.Errorf("peer closed connection")
	}

	payload := strings.TrimSpace(string(buffer[:n]))
	response := fmt.Sprintf("[epoll %d] %s\n", fd, strings.ToUpper(payload))

	_, err = unix.Write(fd, []byte(response))
	return err
}

func closeFD(fd int) {
	unix.Close(fd)
}
