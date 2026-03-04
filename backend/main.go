package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type DashboardSummary struct {
	OnlineNodes    int     `json:"onlineNodes"`
	TotalNodes     int     `json:"totalNodes"`
	CurrentTraffic float64 `json:"currentTrafficMbps"`
	ActiveClients  int     `json:"activeClients"`
	Alerts         int     `json:"alerts"`
}

type Node struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Region    string    `json:"region"`
	Status    string    `json:"status"`
	LatencyMs int       `json:"latencyMs"`
	Version   string    `json:"version"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Client struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Protocol  string    `json:"protocol"`
	NodeID    int       `json:"nodeId"`
	Status    string    `json:"status"`
	RxMB      float64   `json:"rxMb"`
	TxMB      float64   `json:"txMb"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ForwardRule struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	ListenAddr  string    `json:"listenAddr"`
	TargetAddr  string    `json:"targetAddr"`
	Protocol    string    `json:"protocol"`
	Status      string    `json:"status"`
	NodeID      int       `json:"nodeId"`
	Connections int       `json:"connections"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type TrafficRule struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Action    string    `json:"action"` // allow/deny/limit
	Expr      string    `json:"expr"`
	Priority  int       `json:"priority"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Alert struct {
	ID        int       `json:"id"`
	Level     string    `json:"level"` // info/warn/critical
	Source    string    `json:"source"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}

type Store struct {
	mu       sync.RWMutex
	nodes    []Node
	clients  []Client
	forwards []ForwardRule
	rules    []TrafficRule
	alerts   []Alert
}

func newStore() *Store {
	now := time.Now()
	return &Store{
		nodes: []Node{
			{ID: 1, Name: "hk-edge-01", Region: "Hong Kong", Status: "online", LatencyMs: 28, Version: "gost v3.0.0", UpdatedAt: now},
			{ID: 2, Name: "sg-core-01", Region: "Singapore", Status: "online", LatencyMs: 46, Version: "gost v3.0.0", UpdatedAt: now},
			{ID: 3, Name: "tokyo-relay-02", Region: "Tokyo", Status: "offline", LatencyMs: 0, Version: "gost v3.0.0", UpdatedAt: now},
		},
		clients: []Client{
			{ID: 1, Name: "prod-app-1", Protocol: "socks5", NodeID: 1, Status: "online", RxMB: 1024.5, TxMB: 856.3, UpdatedAt: now},
			{ID: 2, Name: "edge-crawler", Protocol: "http", NodeID: 2, Status: "online", RxMB: 320.2, TxMB: 91.8, UpdatedAt: now},
		},
		forwards: []ForwardRule{
			{ID: 1, Name: "ssh-relay", ListenAddr: ":2222", TargetAddr: "10.0.0.8:22", Protocol: "tcp", Status: "enabled", NodeID: 1, Connections: 3, UpdatedAt: now},
			{ID: 2, Name: "web-proxy", ListenAddr: ":8443", TargetAddr: "10.0.0.20:443", Protocol: "tcp", Status: "enabled", NodeID: 2, Connections: 9, UpdatedAt: now},
		},
		rules: []TrafficRule{
			{ID: 1, Name: "allow-office", Action: "allow", Expr: "src in 10.0.0.0/24", Priority: 10, Enabled: true, UpdatedAt: now},
			{ID: 2, Name: "deny-scanner", Action: "deny", Expr: "ua contains masscan", Priority: 100, Enabled: true, UpdatedAt: now},
		},
		alerts: []Alert{
			{ID: 1, Level: "warn", Source: "node/tokyo-relay-02", Message: "Node offline for 12m", Read: false, CreatedAt: now.Add(-12 * time.Minute)},
			{ID: 2, Level: "info", Source: "forward/web-proxy", Message: "Connections peaked at 120/s", Read: false, CreatedAt: now.Add(-2 * time.Minute)},
		},
	}
}

func (s *Store) summary() DashboardSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	online := 0
	activeClients := 0
	alerts := 0
	for _, n := range s.nodes {
		if n.Status == "online" {
			online++
		}
	}
	for _, c := range s.clients {
		if c.Status == "online" {
			activeClients++
		}
	}
	for _, a := range s.alerts {
		if !a.Read {
			alerts++
		}
	}
	return DashboardSummary{
		OnlineNodes:    online,
		TotalNodes:     len(s.nodes),
		CurrentTraffic: float64(rand.Intn(700)+150) / 10,
		ActiveClients:  activeClients,
		Alerts:         alerts,
	}
}

func nextID[T any](items []T) int { return len(items) + 1 }

func (s *Store) listNodes() []Node              { s.mu.RLock(); defer s.mu.RUnlock(); out := make([]Node, len(s.nodes)); copy(out, s.nodes); return out }
func (s *Store) listClients() []Client          { s.mu.RLock(); defer s.mu.RUnlock(); out := make([]Client, len(s.clients)); copy(out, s.clients); return out }
func (s *Store) listForwards() []ForwardRule    { s.mu.RLock(); defer s.mu.RUnlock(); out := make([]ForwardRule, len(s.forwards)); copy(out, s.forwards); return out }
func (s *Store) listRules() []TrafficRule       { s.mu.RLock(); defer s.mu.RUnlock(); out := make([]TrafficRule, len(s.rules)); copy(out, s.rules); return out }
func (s *Store) listAlerts() []Alert            { s.mu.RLock(); defer s.mu.RUnlock(); out := make([]Alert, len(s.alerts)); copy(out, s.alerts); return out }

func (s *Store) addNode(n Node) Node {
	s.mu.Lock(); defer s.mu.Unlock()
	n.ID = nextID(s.nodes)
	n.Status = "online"
	n.UpdatedAt = time.Now()
	n.Version = "gost v3.0.0"
	n.LatencyMs = rand.Intn(70) + 20
	s.nodes = append(s.nodes, n)
	return n
}

func (s *Store) toggleNode(id int) (Node, bool) {
	s.mu.Lock(); defer s.mu.Unlock()
	for i := range s.nodes {
		if s.nodes[i].ID == id {
			if s.nodes[i].Status == "online" {
				s.nodes[i].Status = "offline"
				s.nodes[i].LatencyMs = 0
			} else {
				s.nodes[i].Status = "online"
				s.nodes[i].LatencyMs = rand.Intn(70) + 20
			}
			s.nodes[i].UpdatedAt = time.Now()
			return s.nodes[i], true
		}
	}
	return Node{}, false
}

func (s *Store) addClient(c Client) Client {
	s.mu.Lock(); defer s.mu.Unlock()
	c.ID = nextID(s.clients)
	c.Status = "online"
	c.UpdatedAt = time.Now()
	s.clients = append(s.clients, c)
	return c
}
func (s *Store) toggleClient(id int) (Client, bool) {
	s.mu.Lock(); defer s.mu.Unlock()
	for i := range s.clients {
		if s.clients[i].ID == id {
			if s.clients[i].Status == "online" { s.clients[i].Status = "offline" } else { s.clients[i].Status = "online" }
			s.clients[i].UpdatedAt = time.Now()
			return s.clients[i], true
		}
	}
	return Client{}, false
}

func (s *Store) addForward(f ForwardRule) ForwardRule {
	s.mu.Lock(); defer s.mu.Unlock()
	f.ID = nextID(s.forwards)
	f.Status = "enabled"
	f.UpdatedAt = time.Now()
	s.forwards = append(s.forwards, f)
	return f
}
func (s *Store) toggleForward(id int) (ForwardRule, bool) {
	s.mu.Lock(); defer s.mu.Unlock()
	for i := range s.forwards {
		if s.forwards[i].ID == id {
			if s.forwards[i].Status == "enabled" { s.forwards[i].Status = "disabled" } else { s.forwards[i].Status = "enabled" }
			s.forwards[i].UpdatedAt = time.Now()
			return s.forwards[i], true
		}
	}
	return ForwardRule{}, false
}

func (s *Store) addRule(r TrafficRule) TrafficRule {
	s.mu.Lock(); defer s.mu.Unlock()
	r.ID = nextID(s.rules)
	r.Enabled = true
	r.UpdatedAt = time.Now()
	s.rules = append(s.rules, r)
	return r
}
func (s *Store) toggleRule(id int) (TrafficRule, bool) {
	s.mu.Lock(); defer s.mu.Unlock()
	for i := range s.rules {
		if s.rules[i].ID == id {
			s.rules[i].Enabled = !s.rules[i].Enabled
			s.rules[i].UpdatedAt = time.Now()
			return s.rules[i], true
		}
	}
	return TrafficRule{}, false
}

func (s *Store) markAlertRead(id int) (Alert, bool) {
	s.mu.Lock(); defer s.mu.Unlock()
	for i := range s.alerts {
		if s.alerts[i].ID == id {
			s.alerts[i].Read = true
			return s.alerts[i], true
		}
	}
	return Alert{}, false
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
		next.ServeHTTP(w, r)
	})
}

func parseIDFromToggle(path, prefix string) (int, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != prefix || parts[3] != "toggle" {
		return 0, strconv.ErrSyntax
	}
	return strconv.Atoi(parts[2])
}

func main() {
	rand.Seed(time.Now().UnixNano())
	store := newStore()
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/dashboard/summary", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, store.summary()) })

	mux.HandleFunc("/api/nodes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, store.listNodes())
		case http.MethodPost:
			var req struct{ Name, Region string }
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Region == "" { writeJSON(w, 400, map[string]string{"error": "invalid payload"}); return }
			writeJSON(w, 201, store.addNode(Node{Name: req.Name, Region: req.Region}))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/nodes/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch { w.WriteHeader(http.StatusMethodNotAllowed); return }
		id, err := parseIDFromToggle(r.URL.Path, "nodes"); if err != nil { writeJSON(w, 400, map[string]string{"error":"bad path"}); return }
		n, ok := store.toggleNode(id); if !ok { writeJSON(w, 404, map[string]string{"error":"node not found"}); return }
		writeJSON(w, 200, n)
	})

	mux.HandleFunc("/api/clients", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, store.listClients())
		case http.MethodPost:
			var req struct{ Name, Protocol string; NodeID int }
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Protocol == "" || req.NodeID <= 0 { writeJSON(w, 400, map[string]string{"error":"invalid payload"}); return }
			writeJSON(w, 201, store.addClient(Client{Name: req.Name, Protocol: req.Protocol, NodeID: req.NodeID}))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/clients/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch { w.WriteHeader(http.StatusMethodNotAllowed); return }
		id, err := parseIDFromToggle(r.URL.Path, "clients"); if err != nil { writeJSON(w, 400, map[string]string{"error":"bad path"}); return }
		c, ok := store.toggleClient(id); if !ok { writeJSON(w, 404, map[string]string{"error":"client not found"}); return }
		writeJSON(w, 200, c)
	})

	mux.HandleFunc("/api/forwards", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, store.listForwards())
		case http.MethodPost:
			var req struct{ Name, ListenAddr, TargetAddr, Protocol string; NodeID int }
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.ListenAddr == "" || req.TargetAddr == "" || req.Protocol == "" || req.NodeID <= 0 { writeJSON(w, 400, map[string]string{"error":"invalid payload"}); return }
			writeJSON(w, 201, store.addForward(ForwardRule{Name: req.Name, ListenAddr: req.ListenAddr, TargetAddr: req.TargetAddr, Protocol: req.Protocol, NodeID: req.NodeID, Connections: rand.Intn(5)}))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/forwards/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch { w.WriteHeader(http.StatusMethodNotAllowed); return }
		id, err := parseIDFromToggle(r.URL.Path, "forwards"); if err != nil { writeJSON(w, 400, map[string]string{"error":"bad path"}); return }
		f, ok := store.toggleForward(id); if !ok { writeJSON(w, 404, map[string]string{"error":"forward not found"}); return }
		writeJSON(w, 200, f)
	})

	mux.HandleFunc("/api/rules", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, store.listRules())
		case http.MethodPost:
			var req struct{ Name, Action, Expr string; Priority int }
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Action == "" || req.Expr == "" { writeJSON(w, 400, map[string]string{"error":"invalid payload"}); return }
			writeJSON(w, 201, store.addRule(TrafficRule{Name: req.Name, Action: req.Action, Expr: req.Expr, Priority: req.Priority}))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/rules/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch { w.WriteHeader(http.StatusMethodNotAllowed); return }
		id, err := parseIDFromToggle(r.URL.Path, "rules"); if err != nil { writeJSON(w, 400, map[string]string{"error":"bad path"}); return }
		rule, ok := store.toggleRule(id); if !ok { writeJSON(w, 404, map[string]string{"error":"rule not found"}); return }
		writeJSON(w, 200, rule)
	})

	mux.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
		writeJSON(w, 200, store.listAlerts())
	})
	mux.HandleFunc("/api/alerts/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || !strings.HasSuffix(r.URL.Path, "/read") { w.WriteHeader(http.StatusMethodNotAllowed); return }
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "api" || parts[1] != "alerts" || parts[3] != "read" { writeJSON(w, 400, map[string]string{"error":"bad path"}); return }
		id, err := strconv.Atoi(parts[2]); if err != nil { writeJSON(w, 400, map[string]string{"error":"invalid id"}); return }
		a, ok := store.markAlertRead(id); if !ok { writeJSON(w, 404, map[string]string{"error":"alert not found"}); return }
		writeJSON(w, 200, a)
	})

	mux.HandleFunc("/ws/metrics", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil { return }
		defer conn.Close()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s := store.summary()
			if err := conn.WriteJSON(s); err != nil { return }
		}
	})

	addr := ":8080"
	log.Printf("gost-panel backend listening on %s", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil { log.Fatal(err) }
}
