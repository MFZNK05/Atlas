package backend

import (
	"log"
	"net"
	"time"
)

func (pool *L4BackendPool) L4HealthChecker() {
	for {

		pool.Mutex.Lock()

		for _, s := range pool.Servers {
			conn, err := net.DialTimeout("tcp", s.Address, 2*time.Second)
			if err != nil {
				s.Alive = false
				s.LastChecked = time.Now()
				log.Printf("[HealthCheck] %s is down, timestamp %s", s.Address, time.Now())
			} else {
				s.Alive = true
				s.LastChecked = time.Now()
				log.Printf("[HealthCheck] %s is up and running, timestamp %s", s.Address, time.Now())
				conn.Close()
			}

		}

		pool.Mutex.Unlock()
		time.Sleep(3 * time.Second)
	}
}

func (pool *L7ServerPool) L7HealthChecker() {
	for {

		pool.Mutex.Lock()

		for _, s := range pool.Servers {
			conn, err := net.DialTimeout("tcp", s.Address, 2*time.Second)
			if err != nil {
				s.Alive = false
				s.LastChecked = time.Now()
				log.Printf("[HealthCheck] %s is down, timestamp %s", s.Address, time.Now())
			} else {
				s.Alive = true
				s.LastChecked = time.Now()
				log.Printf("[HealthCheck] %s is up and running, timestamp %s", s.Address, time.Now())
				conn.Close()
			}

		}

		pool.Mutex.Unlock()
		time.Sleep(3 * time.Second)
	}
}
