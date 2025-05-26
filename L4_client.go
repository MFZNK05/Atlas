package main

import (
	"fmt"
	"net"
	"sync"
	"time"
)

func runClient(id int, holdTime time.Duration) {
	conn, err := net.Dial("tcp", "localhost:3000")
	if err != nil {
		fmt.Printf("[Client %d] Connection error: %v\n", id, err)
		return
	}
	defer conn.Close()

	msg := fmt.Sprintf("Hello from client %d", id)
	conn.Write([]byte(msg))

	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	fmt.Printf("[Client %d] Received: %s\n", id, string(buf[:n]))

	time.Sleep(holdTime)
}

func ClientServer() {
	var wg sync.WaitGroup
	clientCount := 20

	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if id < 20 {
				runClient(id, 8*time.Second)
			} else {
				runClient(id, 1*time.Second)
			}
		}(i)

		time.Sleep(100 * time.Millisecond) // small delay to stagger connections
	}

	wg.Wait()
}
