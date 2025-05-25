package network

import (
	"net"

	backend "github.com/Faizan2005/Backend"
	algorithm "github.com/Faizan2005/Balancer"
)

type TransportOpts struct {
	ListenAddr string
}

type TCPTransport struct {
	TransportOpts
	Listener net.Listener
}

func NewTCPTransport(opts TransportOpts) *TCPTransport {
	return &TCPTransport{
		TransportOpts: opts,
	}
}

type L7LBProperties struct {
	L7Pools map[string]*backend.L7ServerPool
}

func NewL7LBProperties(pools map[string]*backend.L7ServerPool) *L7LBProperties {
	return &L7LBProperties{
		L7Pools: pools,
	}
}

type LBProperties struct {
	Transport             *TCPTransport
	L4ServerPoolInterface algorithm.ServerPool
	//L7ServerPoolInterface algorithm.ServerPool
	L4ServerPool *backend.L4BackendPool
	//L7ServerPool          *backend.L7ServerPool
	AlgorithmsMap  map[string]algorithm.LBStrategy
	L7LBProperties *L7LBProperties
}

func NewLBProperties(Transport TCPTransport, L4Pool backend.L4BackendPool, L7Prop *L7LBProperties) *LBProperties {
	algoMap := map[string]algorithm.LBStrategy{
		"round_robin":               algorithm.NewRRAlgo(),
		"weighted_round_robin":      algorithm.NewWRRAlgo(),
		"least_connection":          algorithm.NewLCountAlgo(),
		"weighted_least_connection": algorithm.NewWLCountAlgo(),
	}

	L4PoolAdapter := algorithm.L4PoolAdapter{&L4Pool}
	return &LBProperties{
		Transport:             &Transport,
		L4ServerPoolInterface: &L4PoolAdapter,
		L4ServerPool:          &L4Pool,
		AlgorithmsMap:         algoMap,
		L7LBProperties:        L7Prop,
	}
}
