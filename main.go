package main

import (
	"fmt"
	"sync"
	"time"

	backend "github.com/Faizan2005/Backend"
	netw "github.com/Faizan2005/Network"
)

func main() {
	opts := netw.TransportOpts{
		ListenAddr: ":3000",
	}

	transport := netw.NewTCPTransport(opts)

	L4pool := backend.L4BackendPool{
		Servers: backend.MakeL4TestServers(),
		Mutex:   *new(sync.RWMutex),
	}

	staticPoolOpts := backend.L7PoolOpts{
		Name:    "static",
		Servers: backend.MakeL7StaticTestServers(),
	}

	staticPool := backend.NewL7ServerPool(staticPoolOpts)

	dynamicPoolOpts := backend.L7PoolOpts{
		Name:    "dynamic",
		Servers: backend.MakeL7DynamicTestServers(),
	}

	dynamicPool := backend.NewL7ServerPool(dynamicPoolOpts)

	cookiePoolOpts := backend.L7PoolOpts{
		Name:    "cookie",
		Servers: backend.MakeL7CookieTestServers(),
	}

	cookiePool := backend.NewL7ServerPool(cookiePoolOpts)

	L7pools := map[string]*backend.L7ServerPool{
		"static":  staticPool,
		"dynamic": dynamicPool,
		"cookie":  cookiePool,
	}

	L7Prop := netw.NewL7LBProperties(L7pools)

	p := netw.NewLBProperties(*transport, L4pool, L7Prop)

	if err := p.ListenAndAccept(); err != nil {
		panic(err)
	}

	go ClientServer()

	var wg sync.WaitGroup
	numEach := 20 // number of requests per type

	for i := 0; i < numEach; i++ {
		wg.Add(3)
		go SendStaticRequest(&wg)
		go SendDynamicRequest(&wg)
		go SendCookieRequest(&wg)
		time.Sleep(100 * time.Millisecond) // small delay to simulate staggered clients
	}

	wg.Wait()

	go func() {
		for {
			time.Sleep(3 * time.Second)

			fmt.Println("=== Backend Server States (L4) ===")
			for _, srv := range L4pool.Servers {
				fmt.Printf("Server: %s  ConnCount: %d  Weight: %d\n", srv.Address, srv.ConnCount, srv.Weight)
			}
			fmt.Println("==================================")

			fmt.Println("=== Backend Server States (L7) ===")
			for poolName, pool := range L7pools {
				fmt.Printf("Pool: %s\n", poolName)
				for _, srv := range pool.Servers {
					fmt.Printf("  Server: %s    ReqCount: %d    Weight: %d\n", srv.Address, srv.ReqCount, srv.Weight)
				}
				fmt.Println("----------------------------------")
			}
		}
	}()

	select {}
}
