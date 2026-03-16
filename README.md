<p align="center">
  <h1 align="center">Atlas</h1>
  <p align="center">
    Hybrid L4/L7 Load Balancer
    <br />
    <strong>Auto Protocol Detection &middot; 5 Algorithms &middot; Content-Aware Routing</strong>
  </p>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24-00ADD8?style=flat-square&logo=go" alt="Go 1.24" />
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="MIT License" />
  <img src="https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker" alt="Docker" />
  <img src="https://img.shields.io/badge/Layer-L4%2FL7-orange?style=flat-square" alt="L4/L7" />
</p>

---

## What is Atlas?

**Atlas** is a hybrid Layer 4 / Layer 7 load balancer written in Go. It automatically detects whether incoming traffic is raw TCP or HTTP by peeking at the first 8 bytes of each connection, then routes it through the appropriate pipeline — no configuration needed.

Atlas implements five load balancing algorithms with runtime hot-swap via the **Strategy Pattern**, and unifies L4 TCP and L7 HTTP backend pools through the **Adapter Pattern** — a single algorithm interface works transparently across both transport layers.

### Why Atlas?

Most load balancers force you to choose: L4 (fast, dumb) or L7 (smart, slower). Atlas handles both on a single port. Connections that look like HTTP get content-aware routing (URL classification, cookie affinity); everything else gets efficient TCP forwarding. The algorithm auto-selector watches real-time load metrics and switches strategies on the fly.

---

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Load Balancing Algorithms](#load-balancing-algorithms)
- [Routing Logic](#routing-logic)
- [Design Patterns](#design-patterns)
- [Auto Algorithm Selection](#auto-algorithm-selection)
- [Health Checking](#health-checking)
- [Package Structure](#package-structure)
- [Docker](#docker)
- [Roadmap](#roadmap)
- [License](#license)

---

## Features

- **Auto L4/L7 Detection** — 8-byte peek classifies connections as TCP or HTTP without buffering the entire request
- **5 Load Balancing Algorithms** — Round Robin, Weighted Round Robin, Least Connections, Weighted Least Connections, IP Hash (FNV-1a)
- **Content-Aware L7 Routing** — URL path classification (static vs dynamic), cookie-based sticky sessions, isolated backend pools
- **Strategy Pattern** — Algorithms are hot-swappable at runtime via a common `LBStrategy` interface
- **Adapter Pattern** — L4 and L7 backend servers share a unified `Server` interface despite different internal structures
- **Intelligent Auto-Selection** — Detects load imbalance and weight skew, automatically picks the optimal algorithm
- **Health Checking** — TCP dial every 3 seconds per backend, automatic alive/dead state tracking
- **Docker Support** — Multi-stage Alpine build, non-root user, single binary

---

## Quick Start

### Build & Run

```sh
git clone https://github.com/MFZNK05/Atlas.git
cd Atlas
go build -o atlas .
./atlas
```

Atlas starts on `:3000`, spawns test backends (L4 on `:9000-9002`, L7 on `:8000-8022`), and runs demo traffic.

### Test with curl

```sh
# L7 static content routing
curl http://localhost:3000/index.html

# L7 dynamic API routing
curl http://localhost:3000/api/data

# L7 cookie-based sticky session
curl -b "session_id=user123" http://localhost:3000/api/data

# L4 raw TCP
echo "hello" | nc localhost 3000
```

---

## Architecture

```
                        ┌──────────────────────────────┐
                        │     Client Connection        │
                        └──────────────┬───────────────┘
                                       │
                                       ▼
                        ┌──────────────────────────────┐
                        │     TCP Listener (:3000)     │
                        │     Peek first 8 bytes       │
                        └──────┬───────────────┬───────┘
                               │               │
                    GET/POST/...?          Raw TCP
                               │               │
                    ┌──────────▼──────┐  ┌─────▼──────────┐
                    │   L7 Pipeline   │  │  L4 Pipeline    │
                    │                 │  │                 │
                    │ Parse HTTP req  │  │ SelectAlgoL4()  │
                    │ Check cookie    │  │ ApplyAlgo()     │
                    │ Classify URL    │  │ Dial backend    │
                    │ Select pool     │  │ io.Copy ↔       │
                    │ SelectAlgoL7()  │  │                 │
                    │ ApplyAlgo()     │  └──┬──┬──┬────────┘
                    │ Dial backend    │     │  │  │
                    │ io.Copy ↔       │     │  │  │
                    └──┬──┬──┬────────┘     │  │  │
                       │  │  │              │  │  │
           ┌───────────┘  │  └──────┐   ┌──┘  │  └──────┐
           ▼              ▼         ▼   ▼     ▼         ▼
     ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
     │ Static   │  │ Dynamic  │  │ Cookie   │  │ L4 Pool  │
     │ Pool     │  │ Pool     │  │ Pool     │  │          │
     │:8000-8002│  │:8010-8012│  │:8020-8022│  │:9000-9002│
     └──────────┘  └──────────┘  └──────────┘  └──────────┘
```

---

## Load Balancing Algorithms

| Algorithm | Key | How It Works | Best For |
|-----------|-----|-------------|----------|
| **Round Robin** | `round_robin` | Cycles through servers sequentially, skipping dead ones | Equal-capacity servers, uniform requests |
| **Weighted Round Robin** | `weighted_round_robin` | Counter-based distribution proportional to server weights | Heterogeneous server capacities |
| **Least Connections** | `least_connection` | Picks the alive server with the fewest active connections | Variable request duration |
| **Weighted Least Connections** | `weighted_least_connection` | `connections / weight` score — lower wins | Mixed capacity + variable duration |
| **IP Hash** | `ip_hash` | FNV-1a hash of client IP → deterministic server mapping, walks forward if dead | Session stickiness without cookies |

All algorithms implement the `LBStrategy` interface and can be swapped at runtime without restarting.

---

## Routing Logic

### L4 (Raw TCP)

1. Connection arrives on `:3000`
2. Peek 8 bytes — does not match any HTTP method
3. `SelectAlgoL4()` picks algorithm based on pool state
4. Selected server gets the connection via bidirectional `io.Copy`
5. Connection count tracked per server (incremented on connect, decremented on close)

### L7 (HTTP)

Three-priority routing chain:

```
1. Cookie Check
   └─ session_id cookie found?
      ├─ YES → StickyClients map → same server as last time
      └─ NO ↓

2. URL Classification
   └─ Path ends in .jpg/.css/.js/.html/...?
      ├─ YES → Static pool
      └─ NO  → Dynamic pool

3. Algorithm Selection
   └─ SelectAlgoL7(pool) → picks optimal algorithm
   └─ ApplyAlgo() → returns best server from pool
```

**Cookie-based sticky sessions:** When a request has a `session_id` cookie, Atlas maps it to a specific backend. All subsequent requests with the same session go to the same server — no re-balancing. New sessions get a server via the current algorithm and the mapping is stored.

---

## Design Patterns

### Strategy Pattern

Every algorithm implements a single interface:

```go
type LBStrategy interface {
    ImplementAlgo(pool ServerPool) Server
}
```

Algorithms are stored in a map and selected by name at runtime:

```go
algorithms := map[string]LBStrategy{
    "round_robin":                NewRRAlgo(),
    "weighted_round_robin":       NewWRRAlgo(),
    "least_connection":           NewLCountAlgo(),
    "weighted_least_connection":  NewWLCountAlgo(),
}

server := ApplyAlgo(pool, "least_connection", algorithms)
```

Switching from Round Robin to Weighted Least Connections is a single string change — no code modification, no restart.

### Adapter Pattern

L4 and L7 servers have different internal structures (`ConnCount` vs `ReqCount`, different mutex types), but algorithms shouldn't care:

```go
// Unified interface
type Server interface {
    IsAlive() bool
    GetConnCount() int
    GetWeight() int
    GetAddress() string
    Lock()
    Unlock()
}

// L4 adapter maps ConnCount → GetConnCount()
type L4ServerAdapter struct { *backend.L4BackendServer }
func (s *L4ServerAdapter) GetConnCount() int { return s.ConnCount }

// L7 adapter maps ReqCount → GetConnCount()
type L7ServerAdapter struct { *backend.L7BackendServer }
func (s *L7ServerAdapter) GetConnCount() int { return s.ReqCount }
```

A single algorithm implementation works for both L4 TCP pools and L7 HTTP pools — zero code duplication.

---

## Auto Algorithm Selection

Atlas monitors pool state and picks the optimal algorithm automatically:

```
                    ┌─────────────────────┐
                    │ Check pool state    │
                    └──────┬──────────────┘
                           │
                    ┌──────▼──────────────┐
                    │ Load imbalanced?    │  (max - min connections >= 5)
                    └──┬──────────────┬───┘
                      YES             NO
                       │               │
                ┌──────▼──────┐  ┌─────▼──────────┐
                │Uneven wts?  │  │ Uneven weights? │
                └──┬──────┬───┘  └──┬──────────┬───┘
                  YES     NO      YES          NO
                   │       │       │            │
                   ▼       ▼       ▼            ▼
                 WLC      LC     WRR           RR
```

**L4:** Simplified — only checks weight skew (WLC vs LC).

**L7:** Full decision tree — checks both load imbalance (connection spread ≥ 5) and weight heterogeneity. Four possible outcomes match four algorithms.

---

## Health Checking

Every backend is probed via TCP dial every 3 seconds:

- **Timeout:** 2 seconds per dial
- **Alive:** TCP handshake succeeds → `server.Alive = true`
- **Dead:** Connection refused or timeout → `server.Alive = false`
- **Timestamp:** `server.LastChecked` updated on every probe

All algorithms skip dead servers. IP Hash walks forward to the next alive server if the hashed target is down.

---

## Package Structure

```
Atlas/
├── main.go                  Entry point — bootstrap pools, start listener, demo clients
├── Balancer/
│   └── algorithm.go         Strategy Pattern: 5 algorithms + Adapter Pattern + auto-select
├── Backend/
│   ├── L4serverConfig.go    L4 server and pool structs
│   ├── L7serverConfig.go    L7 server and pool structs (with StickyClients map)
│   ├── health_check.go      TCP health check goroutines (per pool)
│   ├── echo_tcp.go          L4 test backend servers
│   └── echo_http.go         L7 test backend servers (static, dynamic, cookie)
├── Network/
│   ├── types.go             LBProperties, transport config, L7 properties
│   ├── TCP_handler.go       Listener, 8-byte peek, L4/L7 detection, L4 forwarding
│   └── HTTP_handler.go      HTTP parsing, cookie routing, URL classification, L7 forwarding
├── Dockerfile               Multi-stage Alpine build (non-root)
├── Makefile                 build / run / test targets
└── go.mod
```

---

## Docker

```sh
# Build
docker build -t atlas .

# Run
docker run -p 3000:3000 atlas
```

Multi-stage build: Go 1.24 builder → Alpine runtime. Non-root user (`appuser`). Single statically-linked binary.

---

## Roadmap

Atlas has strong algorithmic and architectural foundations. These are the planned enhancements, in priority order:

### 1. Simulation Mode + Live Dashboard

**What:** A built-in web dashboard that visualizes algorithm behavior in real-time. Run `atlas simulate --pattern=spike --backends=5` and watch in a browser as different algorithms distribute the same traffic pattern differently.

**Why:** No open-source load balancer ships a simulation mode. This transforms Atlas from "a load balancer" into "a tool that teaches you how load balancing works." Synthetic traffic patterns (steady, burst, spike, degraded-backend) demonstrate algorithm trade-offs without requiring real infrastructure.

**What it looks like:**
- Embedded single-page app via Go's `embed.FS` (no external dependencies)
- Real-time per-backend metrics: connections, latency, health
- Live traffic distribution bar chart (updates via SSE)
- Algorithm comparison view: run the same traffic through RR, LC, P2C side-by-side
- Fake backends as goroutines with configurable latency distributions

### 2. Power of 2 Choices (P2C) + Consistent Hashing

**What:** Two industry-grade algorithms used by Google, gRPC, Linkerd, and Envoy.

**P2C (Power of 2 Choices):** Pick 2 random backends, route to the one with fewer connections. Provably exponentially better than random selection — O(log log N) maximum load vs O(log N) for random. Requires zero global state and scales to thousands of backends.

**Consistent Hashing with Bounded Loads:** Ketama-style hash ring with virtual nodes. When a backend is removed, only K/N keys need to remap (instead of all keys). Google's "bounded loads" variant caps the maximum load on any single node, preventing hot spots.

**Why:** These algorithms elevate Atlas from textbook implementations to the same class used in production at scale. P2C is ~30 lines of code but represents cutting-edge load balancing research.

### 3. Prometheus Metrics + Structured Logging

**What:** A `/metrics` endpoint exposing Prometheus-format counters and histograms.

**Planned metrics:**
- `atlas_requests_total{backend, algorithm, pool}` — request counter
- `atlas_backend_connections_active{backend}` — gauge
- `atlas_backend_latency_seconds{backend}` — histogram (p50, p95, p99)
- `atlas_backend_health{backend}` — 0/1 gauge
- `atlas_algorithm_selections_total{algorithm}` — counter

**Structured logging:** Replace `log.Printf` with `slog` (Go stdlib) for JSON-formatted log output with fields for backend, algorithm, latency, and client IP.

**Why:** Table stakes for any serious Go infrastructure project. Without metrics, Atlas can't participate in the Prometheus/Grafana ecosystem that ops teams expect.

### 4. Circuit Breaker + HTTP Health Checks

**What:** Per-backend circuit breaker with three states:

```
Closed (normal) → Open (after N failures) → Half-Open (probe single request) → Closed
```

Plus HTTP health checks that send `GET /health` and verify the response status code (instead of just TCP dial).

**Why:** The circuit breaker is one of the most-asked-about patterns in distributed systems interviews. It prevents cascading failures — when a backend is struggling, the circuit breaker stops sending it traffic entirely (instead of slowly killing it with requests it can't handle). The half-open state allows automatic recovery detection.

### 5. YAML Configuration + Hot Reload + Docker Discovery

**What:**
- **YAML config file:** Define backends, algorithms, health check intervals, and pool assignments in a config file instead of hard-coded Go
- **Hot reload:** Watch the config file with `fsnotify`, atomically swap backends and algorithms without restarting
- **Docker discovery:** Watch Docker daemon events for containers with `atlas.backend=true` label, automatically register/deregister backends as containers start and stop

**Why:** Hot reload ties into the dashboard — change the config, watch the traffic redistribution live. Docker discovery makes the demo compelling: `docker run --label atlas.backend=true nginx` and watch it appear in the dashboard within seconds.

---

## License

This project is open source under the [MIT License](LICENSE).

---

<p align="center">
  <sub>Built with Go. Ships as a single binary.</sub>
</p>
