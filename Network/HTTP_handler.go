package network

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	algorithm "github.com/Faizan2005/Balancer"
)

func (lb *LBProperties) HandleHTTP(peekReader *bufio.Reader, conn net.Conn) {
	defer conn.Close()
	startTime := time.Now()
	log.Println("[HTTP_HANDLER] New HTTP connection received.")

	// Capture buffered data and prepend it to the rest of the stream
	peeked, err := peekReader.Peek(peekReader.Buffered())
	if err != nil {
		log.Printf("[HTTP_HANDLER] Error peeking buffered data: %v", err)
		return
	}
	reader := io.MultiReader(bytes.NewReader(peeked), peekReader)

	// Parse HTTP request for routing
	req, err := http.ReadRequest(bufio.NewReader(reader))
	if err != nil {
		log.Printf("[HTTP_HANDLER] Error parsing HTTP request: %v", err)
		return
	}

	// Health checking all servers
	go func() {
		for {
			for sName, pool := range lb.L7LBProperties.L7Pools {
				// Perform health check for the current pool
				pool.L7HealthChecker()
				log.Printf("[HEALTH_CHECK] Health check completed for pool: %s", sName)
			}

			// Sleep for a specified duration before the next round of health checks
			time.Sleep(5 * time.Second)
		}
	}()

	server := lb.SelectL7Server(req)

	log.Printf("[HTTP_HANDLER] Selected backend: %s", server.GetAddress())

	// Update server connection count
	server.Lock()
	server.SetConnCount(server.GetConnCount() + 1)
	server.Unlock()

	backendConn, err := net.Dial("tcp", server.GetAddress())
	if err != nil {
		log.Printf("[HTTP_HANDLER] Failed to connect to backend %s: %v", server.GetAddress(), err)
		return
	}
	defer backendConn.Close()

	// Reuse reader for forwarding (request already parsed)
	go func() {
		_, err := io.Copy(backendConn, reader)
		if err != nil {
			log.Printf("[HTTP_HANDLER] Error copying client → backend: %v", err)
		}
	}()

	_, err = io.Copy(conn, backendConn)
	if err != nil {
		log.Printf("[HTTP_HANDLER] Error copying backend → client: %v", err)
	}

	log.Printf("[HTTP_HANDLER] TCP forwarding done for path %s", req.URL.Path)
	log.Printf("[HTTP_HANDLER] Total time taken: %v", time.Since(startTime))

	// Decrement server connection count
	server.Lock()
	server.SetConnCount(server.GetConnCount() - 1)
	server.Unlock()
}

func (lb *LBProperties) SelectL7Server(r *http.Request) algorithm.Server {
	// Priority 1: Cookie-based routing
	if cookie, err := r.Cookie("session_id"); err == nil {
		log.Printf("[ROUTER] SESSION cookie found: %s", cookie.Value)
		if server := lb.ClassifyCookieRequest(cookie.Value); server != nil {
			log.Printf("[ROUTER] Using cookie-based server: %s", server.GetAddress())
			return server
		}
	} else if err == http.ErrNoCookie {
		log.Printf("[ROUTER] SESSION cookie not available")
	} else {
		log.Printf("[ROUTER] Error retrieving SESSION cookie: %v", err)
	}

	// Priority 2: URL-based routing
	path := r.URL.Path
	urlType := ClassifyURLRequest(path)
	pool := lb.L7LBProperties.L7Pools[urlType]
	if pool == nil {
		log.Printf("[ROUTER] No server pool found for URL type: %s", urlType)
		return nil
	}

	l7Adapter := &algorithm.L7PoolAdapter{pool}
	algoName := algorithm.SelectAlgoL7(l7Adapter)
	if algoName == "" {
		log.Println("[ROUTER] No algorithm selected for URL")
		return nil
	}

	server := algorithm.ApplyAlgo(l7Adapter, algoName, lb.AlgorithmsMap)
	if server == nil {
		log.Println("[ROUTER] No server returned for URL-based routing")
		return nil
	}

	log.Printf("[ROUTER] Using URL-based server: %s", server.GetAddress())
	return server
}

func ClassifyURLRequest(path string) string {
	staticExt := []string{".jpg", ".jpeg", ".png", ".gif", ".css", ".js", ".ico", ".html"}

	for _, s := range staticExt {
		if strings.HasSuffix(path, s) {
			return "static"
		}

	}

	return "dynamic"
}

func (lb *LBProperties) ClassifyCookieRequest(sessionID string) algorithm.Server {
	cookiePool := lb.L7LBProperties.L7Pools["cookie"]

	// Check if session ID is already mapped
	if server, ok := cookiePool.StickyClients[sessionID]; ok {
		log.Printf("[COOKIE_ROUTING] Existing mapping found: %s -> %s", sessionID, server.Address)

		wrapped := algorithm.L7ServerAdapter{server}
		var serverInterface algorithm.Server = &wrapped
		return serverInterface
	}

	// No mapping found, use load balancing algorithm to pick server
	algoName := algorithm.SelectAlgoL7(&algorithm.L7PoolAdapter{cookiePool})
	server := algorithm.ApplyAlgo(&algorithm.L7PoolAdapter{cookiePool}, algoName, lb.AlgorithmsMap)

	adapter, ok := server.(*algorithm.L7ServerAdapter)
	if !ok {
		log.Printf("[COOKIE_ROUTING] Type assertion failed: not an L7ServerAdapter")
		return nil
	}

	cookiePool.StickyClients[sessionID] = adapter.L7BackendServer

	log.Printf("[COOKIE_ROUTING] New session mapped: %s -> %s", sessionID, server.GetAddress())

	return server
}
