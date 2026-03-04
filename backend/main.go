package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
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

type Store struct {
	mu     sync.RWMutex
	nodes  []Node
	nextID int
}

func newStore() *Store {
	now := time.Now()
	seed := []Node{
		{ID: 1, Name: "hk-edge-01", Region: "Hong Kong", Status: "online", LatencyMs: 28, Version: "gost v3.0.0", UpdatedAt: now},
		{ID: 2, Name: "sg-core-01", Region: "Singapore", Status: "online", LatencyMs: 46, Version: "gost v3.0.0", UpdatedAt: now},
		{ID: 3, Name: "tokyo-relay-02", Region: "Tokyo", Status: "offline", LatencyMs: 0, Version: "gost v3.0.0", UpdatedAt: now},
	}
	return &Store{nodes: seed, nextID: len(seed) + 1}
}

func (s *Store) listNodes() []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Node, len(s.nodes))
	copy(out, s.nodes)
	return out
}

func (s *Store) addNode(n Node) Node {
	s.mu.Lock()
	defer s.mu.Unlock()
	n.ID = s.nextID
	s.nextID++
	n.Status = "online"
	n.UpdatedAt = time.Now()
	n.Version = "gost v3.0.0"
	s.nodes = append(s.nodes, n)
	return n
}

func (s *Store) toggleNode(id int) (Node, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *Store) summary() DashboardSummary {
	nodes := s.listNodes()
	online := 0
	for _, n := range nodes {
		if n.Status == "online" {
			online++
		}
	}
	return DashboardSummary{
		OnlineNodes:    online,
		TotalNodes:     len(nodes),
		CurrentTraffic: float64(rand.Intn(700)+150) / 10,
		ActiveClients:  rand.Intn(300) + 120,
		Alerts:         rand.Intn(3),
	}
}

func parseAllowedOrigins(env string) map[string]bool {
	out := map[string]bool{}
	for _, v := range strings.Split(env, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			out[v] = true
		}
	}
	return out
}

func originAllowed(origin string, allowAny bool, allowed map[string]bool) bool {
	if allowAny {
		return true
	}
	if origin == "" {
		return false
	}
	return allowed[origin]
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func withCORS(next http.Handler, allowAny bool, allowed map[string]bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if originAllowed(origin, allowAny, allowed) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	rand.Seed(time.Now().UnixNano())
	store := newStore()
	mux := http.NewServeMux()

	corsEnv := strings.TrimSpace(os.Getenv("CORS_ALLOW_ORIGIN"))
	if corsEnv == "" {
		corsEnv = "*"
	}
	corsAllowAny := corsEnv == "*"
	corsAllowed := parseAllowedOrigins(corsEnv)

	wsEnv := strings.TrimSpace(os.Getenv("WS_ALLOW_ORIGIN"))
	if wsEnv == "" {
		wsEnv = "*"
	}
	wsAllowAny := wsEnv == "*"
	wsAllowed := parseAllowedOrigins(wsEnv)

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return originAllowed(r.Header.Get("Origin"), wsAllowAny, wsAllowed)
		},
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/dashboard/summary", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, store.summary())
	})

	mux.HandleFunc("/api/nodes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, store.listNodes())
		case http.MethodPost:
			var req struct {
				Name   string `json:"name"`
				Region string `json:"region"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
				writeJSON(w, 400, map[string]string{"error": "invalid payload"})
				return
			}
			n := store.addNode(Node{Name: req.Name, Region: req.Region, LatencyMs: rand.Intn(70) + 20})
			writeJSON(w, 201, n)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/nodes/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || !strings.HasSuffix(r.URL.Path, "/toggle") {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 {
			writeJSON(w, 400, map[string]string{"error": "bad path"})
			return
		}
		id, err := strconv.Atoi(parts[2])
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid id"})
			return
		}
		n, ok := store.toggleNode(id)
		if !ok {
			writeJSON(w, 404, map[string]string{"error": "node not found"})
			return
		}
		writeJSON(w, 200, n)
	})

	mux.HandleFunc("/ws/metrics", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			s := store.summary()
			if err := conn.WriteJSON(s); err != nil {
				return
			}
		}
	})

	addr := ":8080"
	log.Printf("gost-panel backend listening on %s", addr)
	log.Printf("CORS_ALLOW_ORIGIN=%s WS_ALLOW_ORIGIN=%s", corsEnv, wsEnv)
	if err := http.ListenAndServe(addr, withCORS(mux, corsAllowAny, corsAllowed)); err != nil {
		log.Fatal(err)
	}
}
