package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func MakeL7StaticTestServers() []*L7BackendServer {
	var servers []*L7BackendServer

	weights := []int{5, 3, 1} // Highly skewed weights

	for i := 0; i < 3; i++ {
		addr := fmt.Sprintf(":800%d", i)
		opts := L7ServerOpts{
			Address: addr,
			Weight:  weights[i],
		}
		server := NewL7Server(opts)
		log.Printf("[L7_TEST_SERVER] Creating static test server at %s with weight %d", addr, weights[i])
		go server.testStaticServerListener()
		servers = append(servers, server)
	}

	return servers
}

func (s *L7BackendServer) testStaticServerListener() {
	//fs := http.FileServer(http.Dir("./static"))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[L7_TEST_SERVER] Received request for: %s", r.URL.Path)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("HEY THERE! THIS IS A PONG MESSAGE\n"))
	})

	server := &http.Server{
		Addr:    s.Address,
		Handler: mux,
	}

	log.Printf("[L7_TEST_SERVER] Static server starting on %s", s.Address)
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Printf("[L7_TEST_SERVER] Error listening on port %s: %v", s.Address, err)
	} else {
		log.Printf("[L7_TEST_SERVER] Static server on %s stopped", s.Address)
	}
}

func MakeL7DynamicTestServers() []*L7BackendServer {
	var servers []*L7BackendServer

	weights := []int{5, 3, 1} // Highly skewed weights

	for i := 0; i < 3; i++ {
		addr := fmt.Sprintf(":801%d", i)
		opts := L7ServerOpts{
			Address: addr,
			Weight:  weights[i],
		}
		server := NewL7Server(opts)
		log.Printf("[L7_TEST_SERVER] Creating dynamic test server at %s with weight %d", addr, weights[i])
		go server.testDynamicServerListener()
		servers = append(servers, server)
	}

	return servers
}

func (s *L7BackendServer) testDynamicServerListener() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", dynamicHandlerFunc)

	server := &http.Server{
		Addr:    s.Address,
		Handler: mux,
	}

	log.Printf("[L7_TEST_SERVER] Dynamic server starting on %s", s.Address)
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Printf("[L7_TEST_SERVER] Error listening on port %s: %v", s.Address, err)
	} else {
		log.Printf("[L7_TEST_SERVER] Dynamic server on %s stopped", s.Address)
	}
}

func dynamicHandlerFunc(w http.ResponseWriter, r *http.Request) {
	log.Printf("[L7_TEST_SERVER] Dynamic handler received request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	response := map[string]string{
		"path":   r.URL.Path,
		"method": r.Method,
		"msg":    "Handled by API server",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[L7_TEST_SERVER] Error encoding response: %v", err)
	}
}

func MakeL7CookieTestServers() []*L7BackendServer {
	var servers []*L7BackendServer

	weights := []int{5, 3, 1} // Highly skewed weights

	for i := 0; i < 3; i++ {
		addr := fmt.Sprintf(":802%d", i)
		opts := L7ServerOpts{
			Address: addr,
			Weight:  weights[i],
		}
		server := NewL7Server(opts)
		log.Printf("[L7_TEST_SERVER] Creating cookie test server at %s with weight %d", addr, weights[i])
		go server.testCookieServerListener()
		servers = append(servers, server)
	}

	return servers
}

func (s *L7BackendServer) testCookieServerListener() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[COOKIE_BACKEND:%s] Request received: %s", s.Address, r.URL.Path)

		// Respond with a unique message
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Response from %s\n", s.Address)))
	})

	server := &http.Server{
		Addr:    s.Address,
		Handler: mux,
	}

	log.Printf("[COOKIE_BACKEND:%s] Starting cookie test server", s.Address)
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Printf("[COOKIE_BACKEND:%s] Listen error: %v", s.Address, err)
	}
}
