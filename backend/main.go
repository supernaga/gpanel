package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	db         *sql.DB
	jwtSecret  []byte
	agentToken string
	webhookURL string
}

type Claims struct {
	UserID int64  `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type DashboardSummary struct {
	OnlineNodes    int     `json:"onlineNodes"`
	TotalNodes     int     `json:"totalNodes"`
	CurrentTraffic float64 `json:"currentTrafficMbps"`
	ActiveClients  int     `json:"activeClients"`
	Alerts         int     `json:"alerts"`
}

type Node struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Region    string    `json:"region"`
	Status    string    `json:"status"`
	LatencyMs int       `json:"latencyMs"`
	Version   string    `json:"version"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Client struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Protocol  string    `json:"protocol"`
	NodeID    int64     `json:"nodeId"`
	Status    string    `json:"status"`
	RxMB      float64   `json:"rxMb"`
	TxMB      float64   `json:"txMb"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ForwardRule struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	ListenAddr  string    `json:"listenAddr"`
	TargetAddr  string    `json:"targetAddr"`
	Protocol    string    `json:"protocol"`
	Status      string    `json:"status"`
	NodeID      int64     `json:"nodeId"`
	Connections int       `json:"connections"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type TrafficRule struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Action    string    `json:"action"`
	Expr      string    `json:"expr"`
	Priority  int       `json:"priority"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Alert struct {
	ID        int64     `json:"id"`
	Level     string    `json:"level"`
	Source    string    `json:"source"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func mustEnv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func (a *App) initSchema(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'admin',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS nodes (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  region TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'online',
  latency_ms INT NOT NULL DEFAULT 0,
  version TEXT NOT NULL DEFAULT 'gost v3.0.0',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS clients (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  protocol TEXT NOT NULL,
  node_id BIGINT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'online',
  rx_mb DOUBLE PRECISION NOT NULL DEFAULT 0,
  tx_mb DOUBLE PRECISION NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS forwards (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  listen_addr TEXT NOT NULL,
  target_addr TEXT NOT NULL,
  protocol TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'enabled',
  node_id BIGINT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  connections INT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS rules (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  action TEXT NOT NULL,
  expr TEXT NOT NULL,
  priority INT NOT NULL DEFAULT 10,
  enabled BOOLEAN NOT NULL DEFAULT true,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS alerts (
  id BIGSERIAL PRIMARY KEY,
  level TEXT NOT NULL,
  source TEXT NOT NULL,
  message TEXT NOT NULL,
  read BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS agent_heartbeats (
  id BIGSERIAL PRIMARY KEY,
  node_name TEXT NOT NULL,
  node_uid TEXT NOT NULL,
  node_ip TEXT,
  version TEXT,
  latency_ms INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(node_uid)
);`
	_, err := a.db.ExecContext(ctx, schema)
	if err != nil {
		return err
	}
	return a.seed(ctx)
}

func (a *App) seed(ctx context.Context) error {
	adminUser := mustEnv("ADMIN_USER", "admin")
	adminPass := mustEnv("ADMIN_PASSWORD", "admin123")
	hash, _ := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
	_, err := a.db.ExecContext(ctx, `INSERT INTO users(username,password_hash,role)
VALUES($1,$2,'admin') ON CONFLICT (username) DO NOTHING`, adminUser, string(hash))
	if err != nil {
		return err
	}

	var cnt int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nodes`).Scan(&cnt); err != nil {
		return err
	}
	if cnt == 0 {
		_, err = a.db.ExecContext(ctx, `INSERT INTO nodes(name,region,status,latency_ms,version) VALUES
('hk-edge-01','Hong Kong','online',28,'gost v3.0.0'),
('sg-core-01','Singapore','online',46,'gost v3.0.0'),
('tokyo-relay-02','Tokyo','offline',0,'gost v3.0.0')`)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *App) makeToken(uid int64, role string) (string, error) {
	now := time.Now()
	claims := Claims{UserID: uid, Role: role, RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(now),
	}}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(a.jwtSecret)
}

func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}
		tok := strings.TrimSpace(h[7:])
		parsed, err := jwt.ParseWithClaims(tok, &Claims{}, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("bad signing method")
			}
			return a.jwtSecret, nil
		})
		if err != nil || !parsed.Valid {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		claims := parsed.Claims.(*Claims)
		if claims.Role == "viewer" && (r.Method == http.MethodPost || r.Method == http.MethodPatch || r.Method == http.MethodDelete) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "viewer is read-only"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) agentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(h), "bearer ") || strings.TrimSpace(h[7:]) != a.agentToken {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid agent token"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) createAlert(ctx context.Context, level, source, msg string) {
	_, _ = a.db.ExecContext(ctx, `INSERT INTO alerts(level,source,message,read) VALUES($1,$2,$3,false)`, level, source, msg)
	if a.webhookURL == "" {
		return
	}
	payload := map[string]string{"level": level, "source": source, "message": msg}
	b, _ := json.Marshal(payload)
	_, _ = http.Post(a.webhookURL, "application/json", strings.NewReader(string(b)))
}

func (a *App) runRuleEngine(ctx context.Context) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rows, err := a.db.QueryContext(ctx, `SELECT id,name,updated_at FROM nodes WHERE status='offline'`)
			if err != nil {
				continue
			}
			for rows.Next() {
				var id int64
				var name string
				var updated time.Time
				if rows.Scan(&id, &name, &updated) == nil && time.Since(updated) > 2*time.Minute {
					a.createAlert(ctx, "warn", fmt.Sprintf("node/%s", name), "Node offline > 2m")
				}
			}
			rows.Close()
		}
	}
}

func parseID(path, entity, action string) (int64, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != entity || parts[3] != action {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseInt(parts[2], 10, 64)
}

func main() {
	rand.Seed(time.Now().UnixNano())
	dsn := mustEnv("DB_DSN", "postgres://gpanel:gpanel@127.0.0.1:5432/gpanel?sslmode=disable")
	jwtSecret := mustEnv("JWT_SECRET", "dev-secret")
	agentToken := mustEnv("AGENT_TOKEN", "dev-agent-token")
	port := mustEnv("PORT", "8080")
	webhookURL := strings.TrimSpace(os.Getenv("WEBHOOK_URL"))

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	app := &App{db: db, jwtSecret: []byte(jwtSecret), agentToken: agentToken, webhookURL: webhookURL}
	ctx := context.Background()
	if err := app.initSchema(ctx); err != nil {
		log.Fatal(err)
	}
	go app.runRuleEngine(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })

	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.Username == "" || req.Password == "" {
			writeJSON(w, 400, map[string]string{"error": "invalid payload"})
			return
		}
		var uid int64
		var hash, role string
		err := app.db.QueryRow(`SELECT id,password_hash,role FROM users WHERE username=$1`, req.Username).Scan(&uid, &hash, &role)
		if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
			writeJSON(w, 401, map[string]string{"error": "invalid credentials"})
			return
		}
		tok, err := app.makeToken(uid, role)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "token error"})
			return
		}
		writeJSON(w, 200, map[string]any{"token": tok, "role": role, "username": req.Username})
	})

	mux.Handle("/api/agent/heartbeat", app.agentAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			NodeUID   string `json:"nodeUid"`
			NodeName  string `json:"nodeName"`
			NodeIP    string `json:"nodeIp"`
			Version   string `json:"version"`
			LatencyMs int    `json:"latencyMs"`
			Region    string `json:"region"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.NodeUID == "" || req.NodeName == "" {
			writeJSON(w, 400, map[string]string{"error": "invalid payload"})
			return
		}
		_, _ = app.db.Exec(`INSERT INTO agent_heartbeats(node_name,node_uid,node_ip,version,latency_ms,created_at)
VALUES($1,$2,$3,$4,$5,now())
ON CONFLICT (node_uid)
DO UPDATE SET node_name=EXCLUDED.node_name,node_ip=EXCLUDED.node_ip,version=EXCLUDED.version,latency_ms=EXCLUDED.latency_ms,created_at=now()`, req.NodeName, req.NodeUID, req.NodeIP, req.Version, req.LatencyMs)
		_, _ = app.db.Exec(`INSERT INTO nodes(name,region,status,latency_ms,version,updated_at)
VALUES($1,$2,'online',$3,$4,now())
ON CONFLICT DO NOTHING`, req.NodeName, first(req.Region, "Unknown"), req.LatencyMs, first(req.Version, "gost v3"))
		_, _ = app.db.Exec(`UPDATE nodes SET status='online', latency_ms=$2, version=$3, updated_at=now() WHERE name=$1`, req.NodeName, req.LatencyMs, first(req.Version, "gost v3"))
		writeJSON(w, 200, map[string]any{"ok": true})
	})))

	api := http.NewServeMux()

	api.HandleFunc("/api/dashboard/summary", func(w http.ResponseWriter, r *http.Request) {
		var s DashboardSummary
		_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE status='online'), COUNT(*) FROM nodes`).Scan(&s.OnlineNodes, &s.TotalNodes)
		_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE status='online') FROM clients`).Scan(&s.ActiveClients)
		_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE read=false) FROM alerts`).Scan(&s.Alerts)
		s.CurrentTraffic = float64(rand.Intn(700)+150) / 10
		writeJSON(w, 200, s)
	})

	api.HandleFunc("/api/nodes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows, err := app.db.Query(`SELECT id,name,region,status,latency_ms,version,updated_at FROM nodes ORDER BY id`)
			if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
			defer rows.Close()
			out := []Node{}
			for rows.Next() {
				var n Node
				_ = rows.Scan(&n.ID, &n.Name, &n.Region, &n.Status, &n.LatencyMs, &n.Version, &n.UpdatedAt)
				out = append(out, n)
			}
			writeJSON(w, 200, out)
		case http.MethodPost:
			var req struct{ Name, Region string }
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.Region == "" { writeJSON(w, 400, map[string]string{"error": "invalid payload"}); return }
			var n Node
			err := app.db.QueryRow(`INSERT INTO nodes(name,region,status,latency_ms,version,updated_at) VALUES($1,$2,'online',$3,'gost v3.0.0',now()) RETURNING id,name,region,status,latency_ms,version,updated_at`, req.Name, req.Region, rand.Intn(70)+20).
				Scan(&n.ID, &n.Name, &n.Region, &n.Status, &n.LatencyMs, &n.Version, &n.UpdatedAt)
			if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
			writeJSON(w, 201, n)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	api.HandleFunc("/api/nodes/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch { w.WriteHeader(http.StatusMethodNotAllowed); return }
		id, err := parseID(r.URL.Path, "nodes", "toggle")
		if err != nil { writeJSON(w, 400, map[string]string{"error": "bad path"}); return }
		_, _ = app.db.Exec(`UPDATE nodes SET status=CASE WHEN status='online' THEN 'offline' ELSE 'online' END, latency_ms=CASE WHEN status='online' THEN 0 ELSE $2 END, updated_at=now() WHERE id=$1`, id, rand.Intn(70)+20)
		var n Node
		err = app.db.QueryRow(`SELECT id,name,region,status,latency_ms,version,updated_at FROM nodes WHERE id=$1`, id).Scan(&n.ID, &n.Name, &n.Region, &n.Status, &n.LatencyMs, &n.Version, &n.UpdatedAt)
		if err != nil { writeJSON(w, 404, map[string]string{"error": "node not found"}); return }
		writeJSON(w, 200, n)
	})

	api.HandleFunc("/api/clients", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows, _ := app.db.Query(`SELECT id,name,protocol,node_id,status,rx_mb,tx_mb,updated_at FROM clients ORDER BY id`)
			defer rows.Close()
			out := []Client{}
			for rows.Next() { var c Client; _ = rows.Scan(&c.ID,&c.Name,&c.Protocol,&c.NodeID,&c.Status,&c.RxMB,&c.TxMB,&c.UpdatedAt); out = append(out,c) }
			writeJSON(w, 200, out)
		case http.MethodPost:
			var req struct{ Name, Protocol string; NodeID int64 }
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name=="" || req.Protocol=="" || req.NodeID <= 0 { writeJSON(w,400,map[string]string{"error":"invalid payload"}); return }
			var c Client
			err := app.db.QueryRow(`INSERT INTO clients(name,protocol,node_id,status,rx_mb,tx_mb,updated_at) VALUES($1,$2,$3,'online',$4,$5,now()) RETURNING id,name,protocol,node_id,status,rx_mb,tx_mb,updated_at`, req.Name, req.Protocol, req.NodeID, rand.Float64()*2000, rand.Float64()*1200).Scan(&c.ID,&c.Name,&c.Protocol,&c.NodeID,&c.Status,&c.RxMB,&c.TxMB,&c.UpdatedAt)
			if err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
			writeJSON(w,201,c)
		default: w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	api.HandleFunc("/api/clients/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch { w.WriteHeader(http.StatusMethodNotAllowed); return }
		id, err := parseID(r.URL.Path, "clients", "toggle")
		if err != nil { writeJSON(w,400,map[string]string{"error":"bad path"}); return }
		_, _ = app.db.Exec(`UPDATE clients SET status=CASE WHEN status='online' THEN 'offline' ELSE 'online' END, updated_at=now() WHERE id=$1`, id)
		var c Client
		err = app.db.QueryRow(`SELECT id,name,protocol,node_id,status,rx_mb,tx_mb,updated_at FROM clients WHERE id=$1`, id).Scan(&c.ID,&c.Name,&c.Protocol,&c.NodeID,&c.Status,&c.RxMB,&c.TxMB,&c.UpdatedAt)
		if err != nil { writeJSON(w,404,map[string]string{"error":"client not found"}); return }
		writeJSON(w,200,c)
	})

	api.HandleFunc("/api/forwards", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows,_ := app.db.Query(`SELECT id,name,listen_addr,target_addr,protocol,status,node_id,connections,updated_at FROM forwards ORDER BY id`)
			defer rows.Close(); out:=[]ForwardRule{}
			for rows.Next(){var f ForwardRule; _=rows.Scan(&f.ID,&f.Name,&f.ListenAddr,&f.TargetAddr,&f.Protocol,&f.Status,&f.NodeID,&f.Connections,&f.UpdatedAt); out=append(out,f)}
			writeJSON(w,200,out)
		case http.MethodPost:
			var req struct{ Name, ListenAddr, TargetAddr, Protocol string; NodeID int64 }
			if json.NewDecoder(r.Body).Decode(&req)!=nil || req.Name==""||req.ListenAddr==""||req.TargetAddr==""||req.Protocol==""||req.NodeID<=0 { writeJSON(w,400,map[string]string{"error":"invalid payload"}); return }
			var f ForwardRule
			err:=app.db.QueryRow(`INSERT INTO forwards(name,listen_addr,target_addr,protocol,status,node_id,connections,updated_at) VALUES($1,$2,$3,$4,'enabled',$5,$6,now()) RETURNING id,name,listen_addr,target_addr,protocol,status,node_id,connections,updated_at`, req.Name,req.ListenAddr,req.TargetAddr,req.Protocol,req.NodeID,rand.Intn(10)).Scan(&f.ID,&f.Name,&f.ListenAddr,&f.TargetAddr,&f.Protocol,&f.Status,&f.NodeID,&f.Connections,&f.UpdatedAt)
			if err!=nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
			writeJSON(w,201,f)
		default: w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	api.HandleFunc("/api/forwards/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch { w.WriteHeader(http.StatusMethodNotAllowed); return }
		id, err := parseID(r.URL.Path, "forwards", "toggle")
		if err != nil { writeJSON(w,400,map[string]string{"error":"bad path"}); return }
		_, _ = app.db.Exec(`UPDATE forwards SET status=CASE WHEN status='enabled' THEN 'disabled' ELSE 'enabled' END, updated_at=now() WHERE id=$1`, id)
		var f ForwardRule
		err = app.db.QueryRow(`SELECT id,name,listen_addr,target_addr,protocol,status,node_id,connections,updated_at FROM forwards WHERE id=$1`, id).Scan(&f.ID,&f.Name,&f.ListenAddr,&f.TargetAddr,&f.Protocol,&f.Status,&f.NodeID,&f.Connections,&f.UpdatedAt)
		if err != nil { writeJSON(w,404,map[string]string{"error":"forward not found"}); return }
		writeJSON(w,200,f)
	})

	api.HandleFunc("/api/rules", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows,_:=app.db.Query(`SELECT id,name,action,expr,priority,enabled,updated_at FROM rules ORDER BY priority DESC,id DESC`)
			defer rows.Close(); out:=[]TrafficRule{}
			for rows.Next(){var x TrafficRule; _=rows.Scan(&x.ID,&x.Name,&x.Action,&x.Expr,&x.Priority,&x.Enabled,&x.UpdatedAt); out=append(out,x)}
			writeJSON(w,200,out)
		case http.MethodPost:
			var req struct{Name,Action,Expr string; Priority int}
			if json.NewDecoder(r.Body).Decode(&req)!=nil || req.Name=="" || req.Action=="" || req.Expr=="" { writeJSON(w,400,map[string]string{"error":"invalid payload"}); return }
			var x TrafficRule
			err:=app.db.QueryRow(`INSERT INTO rules(name,action,expr,priority,enabled,updated_at) VALUES($1,$2,$3,$4,true,now()) RETURNING id,name,action,expr,priority,enabled,updated_at`, req.Name,req.Action,req.Expr,req.Priority).Scan(&x.ID,&x.Name,&x.Action,&x.Expr,&x.Priority,&x.Enabled,&x.UpdatedAt)
			if err!=nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
			writeJSON(w,201,x)
		default:w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	api.HandleFunc("/api/rules/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch { w.WriteHeader(http.StatusMethodNotAllowed); return }
		id, err := parseID(r.URL.Path, "rules", "toggle")
		if err != nil { writeJSON(w,400,map[string]string{"error":"bad path"}); return }
		_, _ = app.db.Exec(`UPDATE rules SET enabled = NOT enabled, updated_at=now() WHERE id=$1`, id)
		var x TrafficRule
		err = app.db.QueryRow(`SELECT id,name,action,expr,priority,enabled,updated_at FROM rules WHERE id=$1`, id).Scan(&x.ID,&x.Name,&x.Action,&x.Expr,&x.Priority,&x.Enabled,&x.UpdatedAt)
		if err != nil { writeJSON(w,404,map[string]string{"error":"rule not found"}); return }
		writeJSON(w,200,x)
	})

	api.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
		rows,_:=app.db.Query(`SELECT id,level,source,message,read,created_at FROM alerts ORDER BY id DESC LIMIT 200`)
		defer rows.Close(); out:=[]Alert{}
		for rows.Next(){var a Alert; _=rows.Scan(&a.ID,&a.Level,&a.Source,&a.Message,&a.Read,&a.CreatedAt); out=append(out,a)}
		writeJSON(w,200,out)
	})
	api.HandleFunc("/api/alerts/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch { w.WriteHeader(http.StatusMethodNotAllowed); return }
		id, err := parseID(r.URL.Path, "alerts", "read")
		if err != nil { writeJSON(w,400,map[string]string{"error":"bad path"}); return }
		_, _ = app.db.Exec(`UPDATE alerts SET read=true WHERE id=$1`, id)
		var a Alert
		err = app.db.QueryRow(`SELECT id,level,source,message,read,created_at FROM alerts WHERE id=$1`, id).Scan(&a.ID,&a.Level,&a.Source,&a.Message,&a.Read,&a.CreatedAt)
		if err != nil { writeJSON(w,404,map[string]string{"error":"alert not found"}); return }
		writeJSON(w,200,a)
	})

	api.HandleFunc("/ws/metrics", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil { return }
		defer conn.Close()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			var s DashboardSummary
			_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE status='online'), COUNT(*) FROM nodes`).Scan(&s.OnlineNodes, &s.TotalNodes)
			_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE status='online') FROM clients`).Scan(&s.ActiveClients)
			_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE read=false) FROM alerts`).Scan(&s.Alerts)
			s.CurrentTraffic = float64(rand.Intn(700)+150) / 10
			if conn.WriteJSON(s) != nil { return }
		}
	})

	mux.Handle("/api/", app.auth(api))

	addr := ":" + port
	log.Printf("gpanel backend listening on %s", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func first(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
