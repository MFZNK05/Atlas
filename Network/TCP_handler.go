package network

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	algorithm "github.com/Faizan2005/Balancer"
)

func (p *LBProperties) ListenAndAccept() error {
	var err error

	p.Transport.Listener, err = net.Listen("tcp", p.Transport.ListenAddr)
	if err != nil {
		log.Printf("Failed to listen on %s: %v", p.Transport.ListenAddr, err)
		return err
	}

	go p.loopAndAccept()

	// Start health checks once (not per-connection)
	go p.L4ServerPool.L4HealthChecker()
	go p.startL7HealthChecks()

	return nil
}

func (p *LBProperties) loopAndAccept() {
	for {
		conn, err := p.Transport.Listener.Accept()
		if err != nil {
			log.Printf("Failed to establish connection with %s: %v", p.Transport.ListenAddr, err)
			return
		}

		go p.handleConn(conn)
	}
}

func (p *LBProperties) handleConn(conn net.Conn) {
	//	peer := NewTCPPeer(conn)
	log.Printf("Connection established with %s", conn.RemoteAddr())

	reader := bufio.NewReader(conn)
	data, err := reader.Peek(8)
	if err != nil {
		log.Println("Error peeking:", err)
	}

	if isHTTP(data[:]) {
		go p.HandleHTTP(reader, conn)
		return
	}

	defer func() {
		log.Printf("Closing connection with client %s", conn.RemoteAddr())
		conn.Close()
	}()

	algoName := algorithm.SelectAlgoL4(p.L4ServerPoolInterface)

	log.Printf("Selected algo to implement (%s)", algoName)
	// algo := p.AlgorithmsMap[algoName]
	// server := algo.ImplementAlgo(p.ServerPool)
	server := algorithm.ApplyAlgo(p.L4ServerPoolInterface, algoName, p.AlgorithmsMap)

	server.Lock()
	server.SetConnCount(server.GetConnCount() + 1)
	server.Unlock()

	backendConn, err := net.DialTimeout("tcp", server.GetAddress(), 5*time.Second)
	if err != nil {
		log.Printf("Failed to dial backend: %v", err)
		return
	}

	go io.Copy(backendConn, conn) // client → server
	io.Copy(conn, backendConn)    // server → client
	log.Print("echoed msg from server to client")

	server.Lock()
	server.SetConnCount(server.GetConnCount() - 1)
	server.Unlock()

	defer func() {
		log.Printf("Closing backend connection with server %s", backendConn.RemoteAddr())
		backendConn.Close()
	}()
}

func (p *LBProperties) startL7HealthChecks() {
	for _, pool := range p.L7LBProperties.L7Pools {
		go pool.L7HealthChecker() // each pool gets its own goroutine (blocks forever)
	}
}

func isHTTP(data []byte) bool {
	methods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"}

	for _, m := range methods {
		if strings.HasPrefix(string(data), m+" ") {
			fmt.Printf("Detected HTTP method: %s\n", m)
			return true
		}
	}

	fmt.Println("Not an HTTP method")
	return false
}
