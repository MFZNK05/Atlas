package balancer

import (
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"time"

	backend "github.com/Faizan2005/Backend"
)

// Interface for selecting lb algorithm for different situations
type LBStrategy interface {
	ImplementAlgo(pool ServerPool) Server
}

type Server interface {
	IsAlive() bool
	GetConnCount() int
	SetConnCount(int)
	GetWeight() int
	Lock()
	Unlock()
	GetAddress() string
	GetLastChecked() time.Time
}

type ServerPool interface {
	GetServers() []Server
	GetServer(int) Server
	GetIndex() int
	SetIndex(int)
	Lock()
	Unlock()
}

type L4ServerAdapter struct {
	*backend.L4BackendServer
}

func (s *L4ServerAdapter) IsAlive() bool              { return s.Alive }
func (s *L4ServerAdapter) GetConnCount() int          { return s.ConnCount }
func (s *L4ServerAdapter) SetConnCount(connCount int) { s.ConnCount = connCount }
func (s *L4ServerAdapter) GetWeight() int             { return s.Weight }
func (s *L4ServerAdapter) GetAddress() string         { return s.Address }
func (s *L4ServerAdapter) GetLastChecked() time.Time  { return s.LastChecked }
func (s *L4ServerAdapter) Lock()                      { s.Mx.Lock() }
func (s *L4ServerAdapter) Unlock()                    { s.Mx.Unlock() }

type L4PoolAdapter struct {
	*backend.L4BackendPool
}

func (p *L4PoolAdapter) GetServers() []Server {
	servers := []Server{}
	for _, s := range p.Servers {
		servers = append(servers, &L4ServerAdapter{s})
	}
	return servers
}

func (p *L4PoolAdapter) GetServer(index int) Server {
	s := p.Servers[index]
	return &L4ServerAdapter{s}
}

func (p *L4PoolAdapter) GetIndex() int      { return p.Index }
func (p *L4PoolAdapter) SetIndex(index int) { p.Index = index }
func (p *L4PoolAdapter) Lock()              { p.Mutex.RLock() }
func (p *L4PoolAdapter) Unlock()            { p.Mutex.RUnlock() }

type L7ServerAdapter struct {
	*backend.L7BackendServer
}

func (s *L7ServerAdapter) IsAlive() bool             { return s.Alive }
func (s *L7ServerAdapter) GetConnCount() int         { return s.ReqCount }
func (s *L7ServerAdapter) SetConnCount(reqCount int) { s.ReqCount = reqCount }
func (s *L7ServerAdapter) GetWeight() int            { return s.Weight }
func (s *L7ServerAdapter) GetAddress() string        { return s.Address }
func (s *L7ServerAdapter) GetLastChecked() time.Time { return s.LastChecked }
func (s *L7ServerAdapter) Lock()                     { s.Mx.Lock() }
func (s *L7ServerAdapter) Unlock()                   { s.Mx.Unlock() }

type L7PoolAdapter struct {
	*backend.L7ServerPool
}

func (p *L7PoolAdapter) GetServers() []Server {
	servers := []Server{}
	for _, s := range p.Servers {
		servers = append(servers, &L7ServerAdapter{s})
	}
	return servers
}

func (p *L7PoolAdapter) GetServer(index int) Server {
	s := p.Servers[index]
	return &L7ServerAdapter{s}
}

func (p *L7PoolAdapter) GetIndex() int      { return p.Index }
func (p *L7PoolAdapter) SetIndex(index int) { p.Index = index }
func (p *L7PoolAdapter) Lock()              { p.Mutex.RLock() }
func (p *L7PoolAdapter) Unlock()            { p.Mutex.RUnlock() }

// Implementing RR algo
type AlgoRR struct{}

func (rr *AlgoRR) ImplementAlgo(pool ServerPool) Server {
	pool.Lock()
	defer pool.Unlock()

	servers := pool.GetServers()
	n := len(servers)
	startIndex := pool.GetIndex()

	log.Printf("Round Robin: Starting selection from index %d", startIndex)

	for i := 0; i < n; i++ {
		index := (startIndex + i) % n
		server := pool.GetServer(index)

		if server != nil && server.IsAlive() {
			log.Printf("Round Robin: Selected server %s at index %d", server.GetAddress(), index)
			pool.SetIndex((index + 1) % n) // Wrap around
			return server
		}
	}

	log.Println("Round Robin: No healthy server found")
	return nil
}

type AlgoWRR struct {
	counter int
}

func (wrr *AlgoWRR) ImplementAlgo(pool ServerPool) Server {
	pool.Lock()
	defer pool.Unlock()

	total := 0
	for _, s := range pool.GetServers() {
		if s.IsAlive() {
			total += s.GetWeight()
		}
	}

	if total == 0 {
		log.Println("Weighted Round Robin: No healthy servers available")
		return nil // No healthy servers
	}

	wrr.counter = (wrr.counter + 1) % total
	log.Printf("Weighted Round Robin: Current counter value is %d", wrr.counter)

	sum := 0
	for _, s := range pool.GetServers() {
		if !s.IsAlive() {
			continue
		}
		sum += s.GetWeight()
		if wrr.counter < sum {
			log.Printf("Weighted Round Robin: Selected server %s with weight %d", s.GetAddress(), s.GetWeight())
			return s
		}
	}

	log.Println("Weighted Round Robin: No server selected")
	return nil
}

type AlgoLeastConn struct{}

func (lc *AlgoLeastConn) ImplementAlgo(pool ServerPool) Server {
	pool.Lock()
	defer pool.Unlock()

	var selected Server
	minConns := int(^uint(0) >> 1) // Max int

	log.Println("Least Connections: Evaluating servers for least connections")

	for _, s := range pool.GetServers() {
		if !s.IsAlive() {
			continue
		}
		s.Lock()
		cCount := s.GetConnCount()
		s.Unlock()

		log.Printf("Least Connections: Server %s has %d connections", s.GetAddress(), cCount)

		if selected == nil || cCount < minConns {
			selected = s
			minConns = cCount
			log.Printf("Least Connections: New selected server %s with %d connections", s.GetAddress(), cCount)
		}
	}

	if selected != nil {
		log.Printf("Least Connections: Selected server %s with %d connections", selected.GetAddress(), minConns)
		return selected
	}

	log.Println("Least Connections: No server selected")
	return nil
}

func SelectAlgoL4(pool ServerPool) string {
	if HasUnevenWeights(pool) {
		return "weighted_least_connection"
	}
	return "least_connection"
}

func SelectAlgoL7(pool ServerPool) string {
	if HasLoadImbalance(pool) {
		if HasUnevenWeights(pool) {
			return "weighted_least_connection"
		}
		return "least_connection"
	}

	if HasUnevenWeights(pool) {
		return "weighted_round_robin"
	}

	return "round_robin"
}

func HasUnevenWeights(pool ServerPool) bool {
	pool.Lock()
	defer pool.Unlock()

	if len(pool.GetServers()) == 0 {
		return false
	}

	refServer := pool.GetServer(0)
	ref := refServer.GetWeight()

	var serverSet = []Server{}
	for i := 1; i < len(pool.GetServers()); i++ {
		serverSet = append(serverSet, pool.GetServer(i))
	}

	for _, s := range serverSet {
		if s.GetWeight() != ref {
			return true
		}
	}
	return false
}

func NewRRAlgo() LBStrategy {
	return &AlgoRR{}
}

func NewWRRAlgo() LBStrategy {
	return &AlgoWRR{
		counter: 0}
}

func NewLCountAlgo() LBStrategy {
	return &AlgoLeastConn{}
}

func NewWLCountAlgo() LBStrategy {
	return &AlgoWLeastConn{}
}

func IPHash(pool ServerPool, host_ip string) Server {
	pool.Lock()
	defer pool.Unlock()

	ip, port, err := net.SplitHostPort(host_ip)
	if err != nil {
		fmt.Printf("Error splitting host and port from %s: %v\n", host_ip, err)
		return nil
	}

	fmt.Printf("[IPHash] Client IP: %s, Port: %s\n", ip, port)

	hash := fnv.New32a()
	hash.Write([]byte(ip))
	hashValue := hash.Sum32()
	n := len(pool.GetServers())
	index := int(hashValue) % n

	fmt.Printf("[IPHash] FNV Hash Value: %d, Backend Index: %d\n", hashValue, index)

	// Walk forward if hashed server is dead
	for i := 0; i < n; i++ {
		candidate := pool.GetServer((index + i) % n)
		if candidate.IsAlive() {
			fmt.Printf("[IPHash] Selected Backend: %s\n", candidate.GetAddress())
			return candidate
		}
	}

	fmt.Println("[IPHash] No alive server found")
	return nil
}

func HasLoadImbalance(pool ServerPool) bool {
	pool.Lock()
	defer pool.Unlock()

	if len(pool.GetServers()) < 2 {
		return false
	}

	max, min := pool.GetServer(0).GetConnCount(), pool.GetServer(0).GetConnCount()

	for _, s := range pool.GetServers() {
		if s.GetConnCount() > max {
			max = s.GetConnCount()
		}

		if s.GetConnCount() < min {
			min = s.GetConnCount()
		}
	}

	return max-min >= 5
}

func ApplyAlgo(pool ServerPool, algoName string, algo map[string]LBStrategy) Server {
	strategy, exists := algo[algoName]
	if !exists {
		log.Printf("Algorithm %s not implemented", algoName)
	}

	server := strategy.ImplementAlgo(pool)
	if server != nil {
		return server // You could support deeper chaining too
	}

	return nil
}

type AlgoWLeastConn struct{}

func (wlc *AlgoWLeastConn) ImplementAlgo(pool ServerPool) Server {
	pool.Lock()
	defer pool.Unlock()

	var selected Server
	minScore := int(^uint(0) >> 1) // Max int

	log.Println("Weighted Least Connections: Evaluating servers for least connections")

	for _, s := range pool.GetServers() {
		if !s.IsAlive() {
			continue
		}
		s.Lock()
		weight := s.GetWeight()
		if weight == 0 {
			weight = 1
		}
		score := s.GetConnCount() / weight
		s.Unlock()

		log.Printf("Weighted Least Connections: Server %s has score %d", s.GetAddress(), score)

		if selected == nil || score < minScore {
			selected = s
			minScore = score
			log.Printf("Weighted Least Connections: New selected server %s with score %d", s.GetAddress(), score)
		}
	}

	if selected != nil {
		log.Printf("Weighted Least Connections: Selected server %s with score %d", selected.GetAddress(), minScore)
		return selected
	}

	log.Println("Weighted Least Connections: No server selected")
	return nil
}
