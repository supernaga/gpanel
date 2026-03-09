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

type ctxKey string

const (
	ctxUserID   ctxKey = "uid"
	ctxUserRole ctxKey = "role"
)

type App struct {
	db               *sql.DB
	jwtSecret        []byte
	agentToken       string
	webhookURL       string
	offlineMinutes     int
	alertDedupeMins    int
	taskTimeoutSecs    int
	taskMaxRetries     int
	taskDispatchPerNode int
	alertSilentHours   string
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

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type AuditLog struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"createdAt"`
}

type AgentTask struct {
	ID            int64      `json:"id"`
	NodeUID       string     `json:"nodeUid"`
	NodeName      string     `json:"nodeName"`
	Command       string     `json:"command"`
	Payload       string     `json:"payload"`
	Status        string     `json:"status"`
	Result        string     `json:"result"`
	RetryCount    int        `json:"retryCount"`
	MaxRetries    int        `json:"maxRetries"`
	TimeoutSecs   int        `json:"timeoutSecs"`
	Priority      int        `json:"priority"`
	CreatedAt     time.Time  `json:"createdAt"`
	Dispatched    *time.Time `json:"dispatchedAt,omitempty"`
	DoneAt        *time.Time `json:"doneAt,omitempty"`
}

type Tunnel struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Mode        string    `json:"mode"`
	Listen      string    `json:"listen"`
	NodeID      int64     `json:"nodeId"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Chain struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Protocol    string    `json:"protocol"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ChainHop struct {
	NodeID     int64  `json:"nodeId"`
	Type       string `json:"type"`
	ListenAddr string `json:"listenAddr"`
	TargetAddr string `json:"targetAddr"`
	Protocol   string `json:"protocol"`
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
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
  name TEXT UNIQUE NOT NULL,
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
  capabilities TEXT NOT NULL DEFAULT '[]',
  services TEXT NOT NULL DEFAULT '[]',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(node_uid)
);
CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  action TEXT NOT NULL,
  target TEXT NOT NULL,
  detail TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS agent_tasks (
  id BIGSERIAL PRIMARY KEY,
  node_uid TEXT NOT NULL DEFAULT '',
  node_name TEXT NOT NULL DEFAULT '',
  command TEXT NOT NULL,
  payload TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  result TEXT NOT NULL DEFAULT '',
  retry_count INT NOT NULL DEFAULT 0,
  max_retries INT NOT NULL DEFAULT 3,
  timeout_seconds INT NOT NULL DEFAULT 300,
  priority INT NOT NULL DEFAULT 50,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  dispatched_at TIMESTAMPTZ,
  done_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS tunnels (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  mode TEXT NOT NULL,
  listen TEXT NOT NULL,
  node_id BIGINT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  enabled BOOLEAN NOT NULL DEFAULT true,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS chains (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  path TEXT NOT NULL,
  protocol TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT false,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`
	_, err := a.db.ExecContext(ctx, schema)
	if err != nil {
		return err
	}
	_, _ = a.db.ExecContext(ctx, `ALTER TABLE agent_tasks ADD COLUMN IF NOT EXISTS retry_count INT NOT NULL DEFAULT 0`)
	_, _ = a.db.ExecContext(ctx, `ALTER TABLE agent_tasks ADD COLUMN IF NOT EXISTS max_retries INT NOT NULL DEFAULT 3`)
	_, _ = a.db.ExecContext(ctx, `ALTER TABLE agent_tasks ADD COLUMN IF NOT EXISTS timeout_seconds INT NOT NULL DEFAULT 300`)
	_, _ = a.db.ExecContext(ctx, `ALTER TABLE agent_tasks ADD COLUMN IF NOT EXISTS priority INT NOT NULL DEFAULT 50`)
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
	_, _ = a.db.ExecContext(ctx, `INSERT INTO users(username,password_hash,role)
VALUES('viewer',$1,'viewer') ON CONFLICT (username) DO NOTHING`, string(hash))

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
	_, _ = a.db.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES
('alert.offline_minutes',$1),
('alert.dedupe_minutes',$2),
('task.timeout_seconds',$3),
('task.max_retries',$4),
('task.dispatch_per_node',$5),
('alert.silent_hours',$6)
ON CONFLICT (key) DO NOTHING`, strconv.Itoa(a.offlineMinutes), strconv.Itoa(a.alertDedupeMins), strconv.Itoa(a.taskTimeoutSecs), strconv.Itoa(a.taskMaxRetries), strconv.Itoa(a.taskDispatchPerNode), a.alertSilentHours)
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
		ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
		ctx = context.WithValue(ctx, ctxUserRole, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(ctxUserRole).(string)
		if role != "admin" {
			writeJSON(w, 403, map[string]string{"error": "admin only"})
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

func (a *App) uid(ctx context.Context) int64 {
	v, _ := ctx.Value(ctxUserID).(int64)
	return v
}

func (a *App) audit(ctx context.Context, action, target, detail string) {
	uid := a.uid(ctx)
	if uid == 0 {
		return
	}
	_, _ = a.db.ExecContext(ctx, `INSERT INTO audit_logs(user_id,action,target,detail) VALUES($1,$2,$3,$4)`, uid, action, target, detail)
}

func (a *App) inSilentWindow(now time.Time) bool {
	if strings.TrimSpace(a.alertSilentHours) == "" { return false }
	parts := strings.Split(a.alertSilentHours, "-")
	if len(parts) != 2 { return false }
	start, e1 := strconv.Atoi(parts[0]); end, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || start < 0 || start > 23 || end < 0 || end > 23 { return false }
	h := now.Hour()
	if start <= end { return h >= start && h < end }
	return h >= start || h < end
}

func (a *App) createAlert(ctx context.Context, level, source, msg string) {
	var id int64
	err := a.db.QueryRowContext(ctx, `SELECT id FROM alerts WHERE source=$1 AND message=$2 AND read=false AND created_at > now() - make_interval(mins => $3) ORDER BY id DESC LIMIT 1`, source, msg, a.alertDedupeMins).Scan(&id)
	if err == nil {
		return
	}
	_, _ = a.db.ExecContext(ctx, `INSERT INTO alerts(level,source,message,read) VALUES($1,$2,$3,false)`, level, source, msg)
	if a.webhookURL == "" || a.inSilentWindow(time.Now()) {
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
			rows, err := a.db.QueryContext(ctx, `SELECT name,updated_at FROM nodes WHERE status='offline'`)
			if err != nil {
				continue
			}
			for rows.Next() {
				var name string
				var updated time.Time
				if rows.Scan(&name, &updated) == nil && time.Since(updated) > time.Duration(a.offlineMinutes)*time.Minute {
					a.createAlert(ctx, "warn", fmt.Sprintf("node/%s", name), fmt.Sprintf("Node offline > %dm", a.offlineMinutes))
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

func first(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func main() {
	rand.Seed(time.Now().UnixNano())
	dsn := mustEnv("DB_DSN", "postgres://gpanel:gpanel@127.0.0.1:5432/gpanel?sslmode=disable")
	jwtSecret := mustEnv("JWT_SECRET", "dev-secret")
	agentToken := mustEnv("AGENT_TOKEN", "dev-agent-token")
	port := mustEnv("PORT", "8080")
	webhookURL := strings.TrimSpace(os.Getenv("WEBHOOK_URL"))
	offlineMinutes, _ := strconv.Atoi(mustEnv("ALERT_OFFLINE_MINUTES", "2"))
	alertDedupeMins, _ := strconv.Atoi(mustEnv("ALERT_DEDUPE_MINUTES", "5"))
	taskTimeoutSecs, _ := strconv.Atoi(mustEnv("TASK_TIMEOUT_SECONDS", "300"))
	taskMaxRetries, _ := strconv.Atoi(mustEnv("TASK_MAX_RETRIES", "3"))
	taskDispatchPerNode, _ := strconv.Atoi(mustEnv("TASK_DISPATCH_PER_NODE", "1"))
	alertSilentHours := mustEnv("ALERT_SILENT_HOURS", "")

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	app := &App{db: db, jwtSecret: []byte(jwtSecret), agentToken: agentToken, webhookURL: webhookURL, offlineMinutes: offlineMinutes, alertDedupeMins: alertDedupeMins, taskTimeoutSecs: taskTimeoutSecs, taskMaxRetries: taskMaxRetries, taskDispatchPerNode: taskDispatchPerNode, alertSilentHours: alertSilentHours}
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
			NodeUID      string   `json:"nodeUid"`
			NodeName     string   `json:"nodeName"`
			NodeIP       string   `json:"nodeIp"`
			Version      string   `json:"version"`
			LatencyMs    int      `json:"latencyMs"`
			Region       string   `json:"region"`
			Capabilities []string `json:"capabilities"`
			Services     []string `json:"services"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.NodeUID == "" || req.NodeName == "" {
			writeJSON(w, 400, map[string]string{"error": "invalid payload"})
			return
		}
		capsJSON, _ := json.Marshal(req.Capabilities)
		servicesJSON, _ := json.Marshal(req.Services)
		_, _ = app.db.Exec(`INSERT INTO agent_heartbeats(node_name,node_uid,node_ip,version,latency_ms,capabilities,services,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,now())
ON CONFLICT (node_uid)
DO UPDATE SET node_name=EXCLUDED.node_name,node_ip=EXCLUDED.node_ip,version=EXCLUDED.version,latency_ms=EXCLUDED.latency_ms,capabilities=EXCLUDED.capabilities,services=EXCLUDED.services,created_at=now()`, req.NodeName, req.NodeUID, req.NodeIP, req.Version, req.LatencyMs, string(capsJSON), string(servicesJSON))
		_, _ = app.db.Exec(`INSERT INTO nodes(name,region,status,latency_ms,version,updated_at)
VALUES($1,$2,'online',$3,$4,now())
ON CONFLICT (name) DO UPDATE SET status='online',latency_ms=EXCLUDED.latency_ms,version=EXCLUDED.version,updated_at=now()`, req.NodeName, first(req.Region, "Unknown"), req.LatencyMs, first(req.Version, "gost v3"))
		writeJSON(w, 200, map[string]any{"ok": true})
	})))

	mux.Handle("/api/agent/tasks/next", app.agentAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nodeUID := strings.TrimSpace(r.URL.Query().Get("nodeUid"))
		nodeName := strings.TrimSpace(r.URL.Query().Get("nodeName"))
		if nodeUID == "" && nodeName == "" {
			writeJSON(w, 400, map[string]string{"error": "nodeUid or nodeName required"})
			return
		}
		tx, err := app.db.Begin()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer tx.Rollback()
		_, _ = tx.Exec(`UPDATE agent_tasks
SET status='pending', retry_count=retry_count+1, dispatched_at=NULL
WHERE status='dispatched' AND dispatched_at IS NOT NULL
  AND now() - dispatched_at > make_interval(secs => timeout_seconds)
  AND retry_count < max_retries`)
		_, _ = tx.Exec(`UPDATE agent_tasks
SET status='failed', result='timeout_exceeded', done_at=now()
WHERE status='dispatched' AND dispatched_at IS NOT NULL
  AND now() - dispatched_at > make_interval(secs => timeout_seconds)
  AND retry_count >= max_retries`)

		var inFlight int
		_ = tx.QueryRow(`SELECT COUNT(*) FROM agent_tasks WHERE status='dispatched' AND (node_uid=$1 OR node_name=$2)`, nodeUID, nodeName).Scan(&inFlight)
		if inFlight >= app.taskDispatchPerNode {
			writeJSON(w, 200, map[string]any{"task": nil, "reason": "dispatch_limit"})
			return
		}
		q := `SELECT id,node_uid,node_name,command,payload,status,result,retry_count,max_retries,timeout_seconds,priority,created_at,dispatched_at,done_at FROM agent_tasks
WHERE status='pending' AND (node_uid=$1 OR node_name=$2 OR (node_uid='' AND node_name=''))
ORDER BY priority DESC, id LIMIT 1 FOR UPDATE SKIP LOCKED`
		var t AgentTask
		err = tx.QueryRow(q, nodeUID, nodeName).Scan(&t.ID, &t.NodeUID, &t.NodeName, &t.Command, &t.Payload, &t.Status, &t.Result, &t.RetryCount, &t.MaxRetries, &t.TimeoutSecs, &t.Priority, &t.CreatedAt, &t.Dispatched, &t.DoneAt)
		if err == sql.ErrNoRows {
			writeJSON(w, 200, map[string]any{"task": nil})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_, _ = tx.Exec(`UPDATE agent_tasks SET status='dispatched', dispatched_at=now() WHERE id=$1`, t.ID)
		_ = tx.Commit()
		t.Status = "dispatched"
		now := time.Now()
		t.Dispatched = &now
		writeJSON(w, 200, map[string]any{"task": t})
	})))

	mux.Handle("/api/agent/tasks/", app.agentAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/ack") {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 5 || parts[0] != "api" || parts[1] != "agent" || parts[2] != "tasks" || parts[4] != "ack" {
			writeJSON(w, 400, map[string]string{"error": "bad path"})
			return
		}
		id, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid id"})
			return
		}
		var req struct {
			Status string `json:"status"`
			Result string `json:"result"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.Status == "" {
			writeJSON(w, 400, map[string]string{"error": "invalid payload"})
			return
		}
		if req.Status != "success" && req.Status != "failed" {
			writeJSON(w, 400, map[string]string{"error": "status must be success|failed"})
			return
		}
		if req.Result != "" && !json.Valid([]byte(req.Result)) {
			writeJSON(w, 400, map[string]string{"error": "result must be valid JSON string"})
			return
		}
		_, _ = app.db.Exec(`UPDATE agent_tasks SET status=$2,result=$3,done_at=now() WHERE id=$1`, id, req.Status, req.Result)
		writeJSON(w, 200, map[string]any{"ok": true})
	})))

	api := http.NewServeMux()
	createNodeTask := func(nodeName, command string, payload map[string]any) error {
		p, _ := json.Marshal(payload)
		_, err := app.db.Exec(`INSERT INTO agent_tasks(node_uid,node_name,command,payload,status,retry_count,max_retries,timeout_seconds,priority) VALUES('',$1,$2,$3,'pending',0,$4,$5,$6)`, nodeName, command, string(p), app.taskMaxRetries, app.taskTimeoutSecs, 60)
		return err
	}
	mustNodeName := func(nodeID int64) (string, error) {
		var nodeName string
		err := app.db.QueryRow(`SELECT name FROM nodes WHERE id=$1`, nodeID).Scan(&nodeName)
		return nodeName, err
	}
	scheduleForward := func(nodeName, forwardName, protocol, listenAddr, targetAddr string) error {
		return createNodeTask(nodeName, "gost.apply_forward", map[string]any{"name": forwardName, "protocol": protocol, "listen": listenAddr, "target": targetAddr})
	}
	scheduleTunnel := func(nodeName, tunnelName, mode, listen string) error {
		return createNodeTask(nodeName, "gost.apply_tunnel", map[string]any{"name": tunnelName, "mode": mode, "listen": listen})
	}
	orchestrateChain := func(chainName, path, protocol string) error {
		hops := []string{}
		for _, part := range strings.Split(path, "->") {
			p := strings.TrimSpace(part)
			if p != "" {
				hops = append(hops, p)
			}
		}
		if len(hops) == 0 {
			return fmt.Errorf("chain path is empty")
		}
		for i, hop := range hops {
			stepName := fmt.Sprintf("%s-hop-%d", chainName, i+1)
			if strings.Contains(hop, ":forward:") {
				parts := strings.SplitN(hop, ":forward:", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid forward hop: %s", hop)
				}
				nodeName := strings.TrimSpace(parts[0])
				rest := strings.TrimSpace(parts[1])
				mappingAndProtocol := strings.Split(rest, ":")
				if len(mappingAndProtocol) < 2 {
					return fmt.Errorf("invalid forward mapping: %s", hop)
				}
				stepProtocol := strings.TrimSpace(mappingAndProtocol[len(mappingAndProtocol)-1])
				mappingRaw := strings.Join(mappingAndProtocol[:len(mappingAndProtocol)-1], ":")
				mapping := strings.SplitN(mappingRaw, "=>", 2)
				if len(mapping) != 2 {
					return fmt.Errorf("invalid forward mapping: %s", hop)
				}
				listen := strings.TrimSpace(mapping[0])
				target := strings.TrimSpace(mapping[1])
				if stepProtocol == "" {
					stepProtocol = protocol
				}
				if err := scheduleForward(nodeName, stepName, stepProtocol, listen, target); err != nil {
					return err
				}
				continue
			}
			if strings.Contains(hop, ":tunnel:") {
				parts := strings.SplitN(hop, ":tunnel:", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid tunnel hop: %s", hop)
				}
				nodeName := strings.TrimSpace(parts[0])
				rest := strings.TrimSpace(parts[1])
				mode := firstNonEmpty(strings.TrimSpace(protocol), "socks5")
				listen := ":1080"
				restParts := strings.SplitN(rest, ":", 2)
				if len(restParts) >= 1 && strings.TrimSpace(restParts[0]) != "" {
					mode = strings.TrimSpace(restParts[0])
				}
				if len(restParts) == 2 && strings.TrimSpace(restParts[1]) != "" {
					listen = ":" + strings.TrimSpace(restParts[1])
				}
				if err := scheduleTunnel(nodeName, stepName, mode, listen); err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("unsupported chain hop type: %s", hop)
		}
		return nil
	}

	api.HandleFunc("/api/dashboard/summary", func(w http.ResponseWriter, r *http.Request) {
		var s DashboardSummary
		_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE status='online'), COUNT(*) FROM nodes`).Scan(&s.OnlineNodes, &s.TotalNodes)
		_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE status='online') FROM clients`).Scan(&s.ActiveClients)
		_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE read=false) FROM alerts`).Scan(&s.Alerts)
		s.CurrentTraffic = float64(rand.Intn(700)+150) / 10
		writeJSON(w, 200, s)
	})

	api.HandleFunc("/api/runtime/summary", func(w http.ResponseWriter, r *http.Request) {
		var nodes, forwards, tunnels, chains int
		_ = app.db.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&nodes)
		_ = app.db.QueryRow(`SELECT COUNT(*) FROM forwards`).Scan(&forwards)
		_ = app.db.QueryRow(`SELECT COUNT(*) FROM tunnels`).Scan(&tunnels)
		_ = app.db.QueryRow(`SELECT COUNT(*) FROM chains`).Scan(&chains)
		writeJSON(w, 200, map[string]int{"nodes": nodes, "forwards": forwards, "tunnels": tunnels, "chains": chains})
	})

	api.HandleFunc("/api/runtime/details", func(w http.ResponseWriter, r *http.Request) {
		nodeRows, _ := app.db.Query(`SELECT id,name,region,status,latency_ms,version,updated_at FROM nodes ORDER BY updated_at DESC, id ASC LIMIT 20`)
		defer nodeRows.Close()
		nodes := []Node{}
		for nodeRows.Next() {
			var n Node
			_ = nodeRows.Scan(&n.ID, &n.Name, &n.Region, &n.Status, &n.LatencyMs, &n.Version, &n.UpdatedAt)
			nodes = append(nodes, n)
		}

		heartbeatRows, _ := app.db.Query(`SELECT node_uid,node_name,node_ip,version,latency_ms,capabilities,services,created_at FROM agent_heartbeats ORDER BY created_at DESC LIMIT 20`)
		defer heartbeatRows.Close()
		heartbeats := []map[string]any{}
		for heartbeatRows.Next() {
			var nodeUID, nodeName, nodeIP, version, caps, services string
			var latency int
			var created time.Time
			_ = heartbeatRows.Scan(&nodeUID, &nodeName, &nodeIP, &version, &latency, &caps, &services, &created)
			heartbeats = append(heartbeats, map[string]any{"nodeUid": nodeUID, "nodeName": nodeName, "nodeIp": nodeIP, "version": version, "latencyMs": latency, "capabilities": caps, "services": services, "createdAt": created})
		}

		forwardRows, _ := app.db.Query(`SELECT id,name,listen_addr,target_addr,protocol,status,node_id,connections,updated_at FROM forwards ORDER BY id DESC LIMIT 20`)
		defer forwardRows.Close()
		forwards := []ForwardRule{}
		for forwardRows.Next() {
			var f ForwardRule
			_ = forwardRows.Scan(&f.ID, &f.Name, &f.ListenAddr, &f.TargetAddr, &f.Protocol, &f.Status, &f.NodeID, &f.Connections, &f.UpdatedAt)
			forwards = append(forwards, f)
		}

		tunnelRows, _ := app.db.Query(`SELECT id,name,mode,listen,node_id,enabled,description,created_at FROM tunnels ORDER BY id DESC LIMIT 20`)
		defer tunnelRows.Close()
		tunnels := []Tunnel{}
		for tunnelRows.Next() {
			var t Tunnel
			_ = tunnelRows.Scan(&t.ID, &t.Name, &t.Mode, &t.Listen, &t.NodeID, &t.Enabled, &t.Description, &t.CreatedAt)
			tunnels = append(tunnels, t)
		}

		chainRows, _ := app.db.Query(`SELECT id,name,path,protocol,enabled,description,created_at FROM chains ORDER BY id DESC LIMIT 20`)
		defer chainRows.Close()
		chains := []Chain{}
		for chainRows.Next() {
			var c Chain
			_ = chainRows.Scan(&c.ID, &c.Name, &c.Path, &c.Protocol, &c.Enabled, &c.Description, &c.CreatedAt)
			chains = append(chains, c)
		}

		taskRows, _ := app.db.Query(`SELECT id,node_uid,node_name,command,payload,status,result,retry_count,max_retries,timeout_seconds,priority,created_at,dispatched_at,done_at FROM agent_tasks ORDER BY id DESC LIMIT 20`)
		defer taskRows.Close()
		tasks := []AgentTask{}
		for taskRows.Next() {
			var t AgentTask
			_ = taskRows.Scan(&t.ID, &t.NodeUID, &t.NodeName, &t.Command, &t.Payload, &t.Status, &t.Result, &t.RetryCount, &t.MaxRetries, &t.TimeoutSecs, &t.Priority, &t.CreatedAt, &t.Dispatched, &t.DoneAt)
			tasks = append(tasks, t)
		}

		var pending, running, done, failed int
		_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE status='pending'), COUNT(*) FILTER (WHERE status='running'), COUNT(*) FILTER (WHERE status='done'), COUNT(*) FILTER (WHERE status='failed') FROM agent_tasks`).Scan(&pending, &running, &done, &failed)
		writeJSON(w, 200, map[string]any{"nodes": nodes, "heartbeats": heartbeats, "forwards": forwards, "tunnels": tunnels, "chains": chains, "tasks": tasks, "taskStats": map[string]int{"pending": pending, "running": running, "done": done, "failed": failed}})
	})

	api.HandleFunc("/api/tunnels", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows, _ := app.db.Query(`SELECT id,name,mode,listen,node_id,enabled,description,created_at FROM tunnels ORDER BY id DESC`)
			defer rows.Close()
			out := []Tunnel{}
			for rows.Next() {
				var t Tunnel
				_ = rows.Scan(&t.ID, &t.Name, &t.Mode, &t.Listen, &t.NodeID, &t.Enabled, &t.Description, &t.CreatedAt)
				out = append(out, t)
			}
			writeJSON(w, 200, out)
		case http.MethodPost:
			var req struct { Name, Mode, Listen string; NodeID int64 }
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.Mode == "" || req.Listen == "" || req.NodeID <= 0 { writeJSON(w, 400, map[string]string{"error": "invalid payload"}); return }
			_, err := app.db.Exec(`INSERT INTO tunnels(name,mode,listen,node_id,enabled,description) VALUES($1,$2,$3,$4,true,'')`, req.Name, req.Mode, req.Listen, req.NodeID)
			if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
			nodeName, err := mustNodeName(req.NodeID)
			if err == nil {
				_ = scheduleTunnel(nodeName, req.Name, req.Mode, req.Listen)
			}
			app.audit(r.Context(), "tunnel.create", fmt.Sprintf("tunnel/%s", req.Name), req.Mode)
			writeJSON(w, 201, map[string]any{"ok": true})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	api.HandleFunc("/api/tunnels/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "api" || parts[1] != "tunnels" {
			writeJSON(w, 400, map[string]string{"error": "bad path"})
			return
		}
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid id"})
			return
		}
		action := parts[3]
		switch {
		case action == "update" && r.Method == http.MethodPatch:
			var req struct { Name, Mode, Listen string; NodeID int64 }
			if json.NewDecoder(r.Body).Decode(&req) != nil { writeJSON(w, 400, map[string]string{"error": "invalid payload"}); return }
			_, err := app.db.Exec(`UPDATE tunnels SET name=COALESCE(NULLIF($2,''),name), mode=COALESCE(NULLIF($3,''),mode), listen=COALESCE(NULLIF($4,''),listen), node_id=COALESCE(NULLIF($5,0),node_id), description='updated' WHERE id=$1`, id, req.Name, req.Mode, req.Listen, req.NodeID)
			if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
			app.audit(r.Context(), "tunnel.update", fmt.Sprintf("tunnel/%d", id), "updated")
			writeJSON(w, 200, map[string]any{"ok": true})
		case action == "delete" && r.Method == http.MethodDelete:
			res, _ := app.db.Exec(`DELETE FROM tunnels WHERE id=$1`, id)
			aff, _ := res.RowsAffected()
			if aff == 0 { writeJSON(w, 404, map[string]string{"error": "tunnel not found"}); return }
			app.audit(r.Context(), "tunnel.delete", fmt.Sprintf("tunnel/%d", id), "deleted")
			writeJSON(w, 200, map[string]any{"ok": true})
		case action == "toggle" && r.Method == http.MethodPatch:
			var t Tunnel
			err := app.db.QueryRow(`SELECT id,name,mode,listen,node_id,enabled,description,created_at FROM tunnels WHERE id=$1`, id).Scan(&t.ID, &t.Name, &t.Mode, &t.Listen, &t.NodeID, &t.Enabled, &t.Description, &t.CreatedAt)
			if err != nil { writeJSON(w, 404, map[string]string{"error": "tunnel not found"}); return }
			enabled := !t.Enabled
			_, _ = app.db.Exec(`UPDATE tunnels SET enabled=$2, description=$3 WHERE id=$1`, id, enabled, map[bool]string{true:"enabled", false:"disabled"}[enabled])
			nodeName, err := mustNodeName(t.NodeID)
			if err == nil {
				if enabled { _ = createNodeTask(nodeName, "gost.start", map[string]any{"name": t.Name}) } else { _ = createNodeTask(nodeName, "gost.stop", map[string]any{"name": t.Name}) }
			}
			app.audit(r.Context(), "tunnel.toggle", fmt.Sprintf("tunnel/%d", id), fmt.Sprintf("enabled=%v", enabled))
			writeJSON(w, 200, map[string]any{"ok": true, "enabled": enabled})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	api.HandleFunc("/api/chains", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows, _ := app.db.Query(`SELECT id,name,path,protocol,enabled,description,created_at FROM chains ORDER BY id DESC`)
			defer rows.Close()
			out := []Chain{}
			for rows.Next() {
				var c Chain
				_ = rows.Scan(&c.ID, &c.Name, &c.Path, &c.Protocol, &c.Enabled, &c.Description, &c.CreatedAt)
				out = append(out, c)
			}
			writeJSON(w, 200, out)
		case http.MethodPost:
			var req struct {
				Name      string     `json:"name"`
				Path      string     `json:"path"`
				Protocol  string     `json:"protocol"`
				Hops      []ChainHop `json:"hops"`
				Status    string     `json:"status"`
				EntryNode int64      `json:"entryNodeId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid payload", "detail": err.Error()})
				return
			}
			if strings.TrimSpace(req.Name) == "" {
				writeJSON(w, 400, map[string]string{"error": "invalid payload", "detail": "name is required"})
				return
			}
			if strings.TrimSpace(req.Path) == "" && len(req.Hops) > 0 {
				segments := []string{}
				proto := strings.TrimSpace(req.Protocol)
				for idx, hop := range req.Hops {
					if hop.NodeID <= 0 || strings.TrimSpace(hop.Type) == "" {
						writeJSON(w, 400, map[string]string{"error": "invalid payload", "detail": fmt.Sprintf("hop %d requires nodeId and type", idx)})
						return
					}
					if proto == "" && strings.TrimSpace(hop.Protocol) != "" {
						proto = strings.TrimSpace(hop.Protocol)
					}
					if hop.Type == "forward" {
						if strings.TrimSpace(hop.ListenAddr) == "" || strings.TrimSpace(hop.TargetAddr) == "" {
							writeJSON(w, 400, map[string]string{"error": "invalid payload", "detail": fmt.Sprintf("forward hop %d requires listenAddr and targetAddr", idx)})
							return
						}
						nodeName, err := mustNodeName(hop.NodeID)
						if err != nil {
							writeJSON(w, 400, map[string]string{"error": "invalid payload", "detail": fmt.Sprintf("forward hop %d node %d not found", idx, hop.NodeID)})
							return
						}
						segments = append(segments, fmt.Sprintf("%s:forward:%s=>%s:%s", nodeName, hop.ListenAddr, hop.TargetAddr, firstNonEmpty(strings.TrimSpace(hop.Protocol), strings.TrimSpace(req.Protocol))))
					} else if hop.Type == "tunnel" {
						listen := strings.TrimSpace(hop.ListenAddr)
						if listen == "" {
							listen = ":1080"
						}
						mode := strings.TrimSpace(hop.Protocol)
						if mode == "" {
							mode = "socks5"
						}
						nodeName, err := mustNodeName(hop.NodeID)
						if err != nil {
							writeJSON(w, 400, map[string]string{"error": "invalid payload", "detail": fmt.Sprintf("tunnel hop %d node %d not found", idx, hop.NodeID)})
							return
						}
						segments = append(segments, fmt.Sprintf("%s:tunnel:%s%s", nodeName, mode, listen))
					} else {
						writeJSON(w, 400, map[string]string{"error": "invalid payload", "detail": fmt.Sprintf("unsupported hop type %q at index %d", hop.Type, idx)})
						return
					}
				}
				req.Path = strings.Join(segments, " -> ")
				req.Protocol = firstNonEmpty(strings.TrimSpace(req.Protocol), proto)
			}
			if strings.TrimSpace(req.Path) == "" {
				writeJSON(w, 400, map[string]string{"error": "invalid payload", "detail": "path is empty after payload normalization"})
				return
			}
			description := "pending orchestration"
			enabled := false
			if err := orchestrateChain(req.Name, req.Path, req.Protocol); err == nil {
				description = "tasks scheduled"
				enabled = true
			} else {
				description = err.Error()
			}
			_, err := app.db.Exec(`INSERT INTO chains(name,path,protocol,enabled,description) VALUES($1,$2,$3,$4,$5)`, req.Name, req.Path, req.Protocol, enabled, description)
			if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
			app.audit(r.Context(), "chain.create", fmt.Sprintf("chain/%s", req.Name), description)
			writeJSON(w, 201, map[string]any{"ok": true, "enabled": enabled, "description": description, "path": req.Path, "protocol": req.Protocol})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	api.HandleFunc("/api/chains/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "api" || parts[1] != "chains" {
			writeJSON(w, 400, map[string]string{"error": "bad path"})
			return
		}
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid id"})
			return
		}
		action := parts[3]
		switch {
		case action == "update" && r.Method == http.MethodPatch:
			var req struct { Name, Path, Protocol string }
			if json.NewDecoder(r.Body).Decode(&req) != nil { writeJSON(w, 400, map[string]string{"error": "invalid payload"}); return }
			_, err := app.db.Exec(`UPDATE chains SET name=COALESCE(NULLIF($2,''),name), path=COALESCE(NULLIF($3,''),path), protocol=COALESCE(NULLIF($4,''),protocol), description='updated', enabled=false WHERE id=$1`, id, req.Name, req.Path, req.Protocol)
			if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
			app.audit(r.Context(), "chain.update", fmt.Sprintf("chain/%d", id), "updated")
			writeJSON(w, 200, map[string]any{"ok": true})
		case action == "delete" && r.Method == http.MethodDelete:
			res, _ := app.db.Exec(`DELETE FROM chains WHERE id=$1`, id)
			aff, _ := res.RowsAffected()
			if aff == 0 { writeJSON(w, 404, map[string]string{"error": "chain not found"}); return }
			app.audit(r.Context(), "chain.delete", fmt.Sprintf("chain/%d", id), "deleted")
			writeJSON(w, 200, map[string]any{"ok": true})
		case action == "toggle" && r.Method == http.MethodPatch:
			var chain Chain
			err := app.db.QueryRow(`SELECT id,name,path,protocol,enabled,description,created_at FROM chains WHERE id=$1`, id).Scan(&chain.ID, &chain.Name, &chain.Path, &chain.Protocol, &chain.Enabled, &chain.Description, &chain.CreatedAt)
			if err != nil { writeJSON(w, 404, map[string]string{"error": "chain not found"}); return }
			enabled := !chain.Enabled
			description := chain.Description
			if enabled {
				if err := orchestrateChain(chain.Name, chain.Path, chain.Protocol); err == nil {
					description = "tasks scheduled"
				} else {
					description = err.Error()
					enabled = false
				}
			}
			_, _ = app.db.Exec(`UPDATE chains SET enabled=$2, description=$3 WHERE id=$1`, id, enabled, description)
			app.audit(r.Context(), "chain.toggle", fmt.Sprintf("chain/%d", id), fmt.Sprintf("enabled=%v", enabled))
			writeJSON(w, 200, map[string]any{"ok": true, "enabled": enabled, "description": description})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	api.HandleFunc("/api/settings/alerts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, map[string]any{"offlineMinutes": app.offlineMinutes, "dedupeMinutes": app.alertDedupeMins, "taskTimeoutSeconds": app.taskTimeoutSecs, "taskMaxRetries": app.taskMaxRetries, "taskDispatchPerNode": app.taskDispatchPerNode, "alertSilentHours": app.alertSilentHours})
		case http.MethodPatch:
			var req struct {
				OfflineMinutes      int    `json:"offlineMinutes"`
				DedupeMinutes       int    `json:"dedupeMinutes"`
				TaskTimeoutSecs     int    `json:"taskTimeoutSeconds"`
				TaskMaxRetries      int    `json:"taskMaxRetries"`
				TaskDispatchPerNode int    `json:"taskDispatchPerNode"`
				AlertSilentHours    string `json:"alertSilentHours"`
			}
			if json.NewDecoder(r.Body).Decode(&req) != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid payload"})
				return
			}
			if req.OfflineMinutes > 0 { app.offlineMinutes = req.OfflineMinutes; _, _ = app.db.Exec(`UPDATE settings SET value=$2,updated_at=now() WHERE key=$1`, "alert.offline_minutes", strconv.Itoa(req.OfflineMinutes)) }
			if req.DedupeMinutes > 0 { app.alertDedupeMins = req.DedupeMinutes; _, _ = app.db.Exec(`UPDATE settings SET value=$2,updated_at=now() WHERE key=$1`, "alert.dedupe_minutes", strconv.Itoa(req.DedupeMinutes)) }
			if req.TaskTimeoutSecs > 0 { app.taskTimeoutSecs = req.TaskTimeoutSecs; _, _ = app.db.Exec(`UPDATE settings SET value=$2,updated_at=now() WHERE key=$1`, "task.timeout_seconds", strconv.Itoa(req.TaskTimeoutSecs)) }
			if req.TaskMaxRetries >= 0 { app.taskMaxRetries = req.TaskMaxRetries; _, _ = app.db.Exec(`UPDATE settings SET value=$2,updated_at=now() WHERE key=$1`, "task.max_retries", strconv.Itoa(req.TaskMaxRetries)) }
			if req.TaskDispatchPerNode > 0 { app.taskDispatchPerNode = req.TaskDispatchPerNode; _, _ = app.db.Exec(`UPDATE settings SET value=$2,updated_at=now() WHERE key=$1`, "task.dispatch_per_node", strconv.Itoa(req.TaskDispatchPerNode)) }
			if strings.TrimSpace(req.AlertSilentHours) != "" { app.alertSilentHours = req.AlertSilentHours; _, _ = app.db.Exec(`UPDATE settings SET value=$2,updated_at=now() WHERE key=$1`, "alert.silent_hours", req.AlertSilentHours) }
			app.audit(r.Context(), "settings.alerts.update", "settings/alerts", "updated")
			writeJSON(w, 200, map[string]any{"offlineMinutes": app.offlineMinutes, "dedupeMinutes": app.alertDedupeMins, "taskTimeoutSeconds": app.taskTimeoutSecs, "taskMaxRetries": app.taskMaxRetries, "taskDispatchPerNode": app.taskDispatchPerNode, "alertSilentHours": app.alertSilentHours})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	api.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows, _ := app.db.Query(`SELECT id,username,role,created_at FROM users ORDER BY id`)
			defer rows.Close()
			out := []User{}
			for rows.Next() {
				var u User
				_ = rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt)
				out = append(out, u)
			}
			writeJSON(w, 200, out)
		case http.MethodPost:
			var req struct {
				Username string `json:"username"`
				Password string `json:"password"`
				Role     string `json:"role"`
			}
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.Username == "" || req.Password == "" || (req.Role != "admin" && req.Role != "viewer") {
				writeJSON(w, 400, map[string]string{"error": "invalid payload"})
				return
			}
			hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
			var u User
			err := app.db.QueryRow(`INSERT INTO users(username,password_hash,role) VALUES($1,$2,$3) RETURNING id,username,role,created_at`, req.Username, string(hash), req.Role).
				Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			app.audit(r.Context(), "user.create", fmt.Sprintf("user/%s", u.Username), "created")
			writeJSON(w, 201, u)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	api.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id, err := parseID(r.URL.Path, "users", "update")
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad path, expected /api/users/{id}/update"})
			return
		}
		var req struct {
			Role     string `json:"role"`
			Password string `json:"password"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid payload"})
			return
		}
		if req.Role != "" {
			_, _ = app.db.Exec(`UPDATE users SET role=$2 WHERE id=$1`, id, req.Role)
		}
		if req.Password != "" {
			hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
			_, _ = app.db.Exec(`UPDATE users SET password_hash=$2 WHERE id=$1`, id, string(hash))
		}
		app.audit(r.Context(), "user.update", fmt.Sprintf("user/%d", id), "role/password updated")
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	api.HandleFunc("/api/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		q := `SELECT id,user_id,action,target,detail,created_at FROM audit_logs WHERE 1=1`
		args := []any{}
		if v := strings.TrimSpace(r.URL.Query().Get("userId")); v != "" {
			q += fmt.Sprintf(" AND user_id=$%d", len(args)+1)
			args = append(args, v)
		}
		if v := strings.TrimSpace(r.URL.Query().Get("action")); v != "" {
			q += fmt.Sprintf(" AND action=$%d", len(args)+1)
			args = append(args, v)
		}
		if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
			q += fmt.Sprintf(" AND created_at >= $%d", len(args)+1)
			args = append(args, v)
		}
		if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
			q += fmt.Sprintf(" AND created_at <= $%d", len(args)+1)
			args = append(args, v)
		}
		q += " ORDER BY id DESC LIMIT 200"
		rows, _ := app.db.Query(q, args...)
		defer rows.Close()
		out := []AuditLog{}
		for rows.Next() {
			var x AuditLog
			_ = rows.Scan(&x.ID, &x.UserID, &x.Action, &x.Target, &x.Detail, &x.CreatedAt)
			out = append(out, x)
		}
		writeJSON(w, 200, out)
	})

	api.HandleFunc("/api/agent/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows, _ := app.db.Query(`SELECT id,node_uid,node_name,command,payload,status,result,retry_count,max_retries,timeout_seconds,priority,created_at,dispatched_at,done_at FROM agent_tasks ORDER BY id DESC LIMIT 200`)
			defer rows.Close()
			out := []AgentTask{}
			for rows.Next() {
				var t AgentTask
				_ = rows.Scan(&t.ID, &t.NodeUID, &t.NodeName, &t.Command, &t.Payload, &t.Status, &t.Result, &t.RetryCount, &t.MaxRetries, &t.TimeoutSecs, &t.Priority, &t.CreatedAt, &t.Dispatched, &t.DoneAt)
				out = append(out, t)
			}
			writeJSON(w, 200, out)
		case http.MethodPost:
			var req struct {
				NodeUID      string `json:"nodeUid"`
				NodeName     string `json:"nodeName"`
				Command      string `json:"command"`
				Payload      string `json:"payload"`
				MaxRetries   int    `json:"maxRetries"`
				TimeoutSecs  int    `json:"timeoutSecs"`
				Priority     int    `json:"priority"`
			}
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.Command == "" {
				writeJSON(w, 400, map[string]string{"error": "invalid payload"})
				return
			}
			if req.Payload != "" && !json.Valid([]byte(req.Payload)) {
				writeJSON(w, 400, map[string]string{"error": "payload must be valid JSON string"})
				return
			}
			if req.MaxRetries <= 0 { req.MaxRetries = app.taskMaxRetries }
			if req.TimeoutSecs <= 0 { req.TimeoutSecs = app.taskTimeoutSecs }
			if req.Priority == 0 { req.Priority = 50 }
			var t AgentTask
			err := app.db.QueryRow(`INSERT INTO agent_tasks(node_uid,node_name,command,payload,status,retry_count,max_retries,timeout_seconds,priority) VALUES($1,$2,$3,$4,'pending',0,$5,$6,$7) RETURNING id,node_uid,node_name,command,payload,status,result,retry_count,max_retries,timeout_seconds,priority,created_at,dispatched_at,done_at`, req.NodeUID, req.NodeName, req.Command, req.Payload, req.MaxRetries, req.TimeoutSecs, req.Priority).
				Scan(&t.ID, &t.NodeUID, &t.NodeName, &t.Command, &t.Payload, &t.Status, &t.Result, &t.RetryCount, &t.MaxRetries, &t.TimeoutSecs, &t.Priority, &t.CreatedAt, &t.Dispatched, &t.DoneAt)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			app.audit(r.Context(), "agent.task.create", fmt.Sprintf("task/%d", t.ID), t.Command)
			writeJSON(w, 201, t)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	api.HandleFunc("/api/nodes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows, err := app.db.Query(`SELECT id,name,region,status,latency_ms,version,updated_at FROM nodes ORDER BY id`)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
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
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.Region == "" {
				writeJSON(w, 400, map[string]string{"error": "invalid payload"})
				return
			}
			var n Node
			err := app.db.QueryRow(`INSERT INTO nodes(name,region,status,latency_ms,version,updated_at) VALUES($1,$2,'online',$3,'gost v3.0.0',now()) RETURNING id,name,region,status,latency_ms,version,updated_at`, req.Name, req.Region, rand.Intn(70)+20).
				Scan(&n.ID, &n.Name, &n.Region, &n.Status, &n.LatencyMs, &n.Version, &n.UpdatedAt)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			app.audit(r.Context(), "node.create", fmt.Sprintf("node/%d", n.ID), n.Name)
			writeJSON(w, 201, n)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	api.HandleFunc("/api/nodes/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 3 || parts[0] != "api" || parts[1] != "nodes" {
			writeJSON(w, 400, map[string]string{"error": "bad path"})
			return
		}
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid id"})
			return
		}

		if len(parts) == 4 && parts[3] == "toggle" && r.Method == http.MethodPatch {
			_, _ = app.db.Exec(`UPDATE nodes SET status=CASE WHEN status='online' THEN 'offline' ELSE 'online' END, latency_ms=CASE WHEN status='online' THEN 0 ELSE $2 END, updated_at=now() WHERE id=$1`, id, rand.Intn(70)+20)
			var n Node
			err = app.db.QueryRow(`SELECT id,name,region,status,latency_ms,version,updated_at FROM nodes WHERE id=$1`, id).Scan(&n.ID, &n.Name, &n.Region, &n.Status, &n.LatencyMs, &n.Version, &n.UpdatedAt)
			if err != nil {
				writeJSON(w, 404, map[string]string{"error": "node not found"})
				return
			}
			app.audit(r.Context(), "node.toggle", fmt.Sprintf("node/%d", n.ID), n.Status)
			writeJSON(w, 200, n)
			return
		}

		if len(parts) == 4 && parts[3] == "update" && r.Method == http.MethodPatch {
			var req struct {
				Name    string `json:"name"`
				Region  string `json:"region"`
				Version string `json:"version"`
			}
			if json.NewDecoder(r.Body).Decode(&req) != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid payload"})
				return
			}
			_, _ = app.db.Exec(`UPDATE nodes SET name=COALESCE(NULLIF($2,''),name), region=COALESCE(NULLIF($3,''),region), version=COALESCE(NULLIF($4,''),version), updated_at=now() WHERE id=$1`, id, req.Name, req.Region, req.Version)
			var n Node
			err = app.db.QueryRow(`SELECT id,name,region,status,latency_ms,version,updated_at FROM nodes WHERE id=$1`, id).Scan(&n.ID, &n.Name, &n.Region, &n.Status, &n.LatencyMs, &n.Version, &n.UpdatedAt)
			if err != nil {
				writeJSON(w, 404, map[string]string{"error": "node not found"})
				return
			}
			app.audit(r.Context(), "node.update", fmt.Sprintf("node/%d", n.ID), n.Name)
			writeJSON(w, 200, n)
			return
		}

		if len(parts) == 4 && parts[3] == "heartbeats" && r.Method == http.MethodGet {
			rows, err := app.db.Query(`SELECT node_uid,node_name,node_ip,version,latency_ms,created_at FROM agent_heartbeats WHERE node_name=(SELECT name FROM nodes WHERE id=$1) ORDER BY created_at DESC LIMIT 50`, id)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			defer rows.Close()
			out := []map[string]any{}
			for rows.Next() {
				var uid, name, ip, version string
				var latency int
				var created time.Time
				_ = rows.Scan(&uid, &name, &ip, &version, &latency, &created)
				out = append(out, map[string]any{"nodeUid": uid, "nodeName": name, "nodeIp": ip, "version": version, "latencyMs": latency, "createdAt": created})
			}
			writeJSON(w, 200, out)
			return
		}

		if len(parts) == 4 && parts[3] == "delete" && r.Method == http.MethodDelete {
			res, _ := app.db.Exec(`DELETE FROM nodes WHERE id=$1`, id)
			aff, _ := res.RowsAffected()
			if aff == 0 {
				writeJSON(w, 404, map[string]string{"error": "node not found"})
				return
			}
			app.audit(r.Context(), "node.delete", fmt.Sprintf("node/%d", id), "deleted")
			writeJSON(w, 200, map[string]any{"ok": true})
			return
		}

		if len(parts) == 5 && parts[3] == "gost" && r.Method == http.MethodPost {
			action := parts[4]
			var req struct {
				Name   string `json:"name"`
				Mode   string `json:"mode"`
				Listen string `json:"listen"`
				Target string `json:"target"`
				Protocol string `json:"protocol"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			var nodeName string
			err = app.db.QueryRow(`SELECT name FROM nodes WHERE id=$1`, id).Scan(&nodeName)
			if err != nil {
				writeJSON(w, 404, map[string]string{"error": "node not found"})
				return
			}
			var cmd string
			payload := map[string]any{"name": req.Name, "mode": req.Mode, "listen": req.Listen, "target": req.Target, "protocol": req.Protocol}
			switch action {
			case "install": cmd = "gost.install"
			case "start": cmd = "gost.start"
			case "stop": cmd = "gost.stop"
			case "restart": cmd = "gost.restart"
			case "status": cmd = "gost.status"
			case "apply-forward": cmd = "gost.apply_forward"
			case "apply-tunnel": cmd = "gost.apply_tunnel"
			default:
				writeJSON(w, 400, map[string]string{"error": "unsupported gost action"})
				return
			}
			if req.Name == "" { payload["name"] = fmt.Sprintf("node-%d", id) }
			if err := createNodeTask(nodeName, cmd, payload); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			app.audit(r.Context(), "node.gost", fmt.Sprintf("node/%d", id), cmd)
			writeJSON(w, 200, map[string]any{"ok": true, "command": cmd, "node": nodeName})
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	api.HandleFunc("/api/clients", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows, _ := app.db.Query(`SELECT id,name,protocol,node_id,status,rx_mb,tx_mb,updated_at FROM clients ORDER BY id`)
			defer rows.Close()
			out := []Client{}
			for rows.Next() {
				var c Client
				_ = rows.Scan(&c.ID, &c.Name, &c.Protocol, &c.NodeID, &c.Status, &c.RxMB, &c.TxMB, &c.UpdatedAt)
				out = append(out, c)
			}
			writeJSON(w, 200, out)
		case http.MethodPost:
			var req struct{ Name, Protocol string; NodeID int64 }
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.Protocol == "" || req.NodeID <= 0 {
				writeJSON(w, 400, map[string]string{"error": "invalid payload"})
				return
			}
			var c Client
			err := app.db.QueryRow(`INSERT INTO clients(name,protocol,node_id,status,rx_mb,tx_mb,updated_at) VALUES($1,$2,$3,'online',$4,$5,now()) RETURNING id,name,protocol,node_id,status,rx_mb,tx_mb,updated_at`, req.Name, req.Protocol, req.NodeID, rand.Float64()*2000, rand.Float64()*1200).Scan(&c.ID, &c.Name, &c.Protocol, &c.NodeID, &c.Status, &c.RxMB, &c.TxMB, &c.UpdatedAt)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			app.audit(r.Context(), "client.create", fmt.Sprintf("client/%d", c.ID), c.Name)
			writeJSON(w, 201, c)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	api.HandleFunc("/api/clients/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id, err := parseID(r.URL.Path, "clients", "toggle")
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad path"})
			return
		}
		_, _ = app.db.Exec(`UPDATE clients SET status=CASE WHEN status='online' THEN 'offline' ELSE 'online' END, updated_at=now() WHERE id=$1`, id)
		var c Client
		err = app.db.QueryRow(`SELECT id,name,protocol,node_id,status,rx_mb,tx_mb,updated_at FROM clients WHERE id=$1`, id).Scan(&c.ID, &c.Name, &c.Protocol, &c.NodeID, &c.Status, &c.RxMB, &c.TxMB, &c.UpdatedAt)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "client not found"})
			return
		}
		app.audit(r.Context(), "client.toggle", fmt.Sprintf("client/%d", c.ID), c.Status)
		writeJSON(w, 200, c)
	})

	api.HandleFunc("/api/forwards", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows, _ := app.db.Query(`SELECT id,name,listen_addr,target_addr,protocol,status,node_id,connections,updated_at FROM forwards ORDER BY id`)
			defer rows.Close()
			out := []ForwardRule{}
			for rows.Next() {
				var f ForwardRule
				_ = rows.Scan(&f.ID, &f.Name, &f.ListenAddr, &f.TargetAddr, &f.Protocol, &f.Status, &f.NodeID, &f.Connections, &f.UpdatedAt)
				out = append(out, f)
			}
			writeJSON(w, 200, out)
		case http.MethodPost:
			var req struct{ Name, ListenAddr, TargetAddr, Protocol string; NodeID int64 }
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.ListenAddr == "" || req.TargetAddr == "" || req.Protocol == "" || req.NodeID <= 0 {
				writeJSON(w, 400, map[string]string{"error": "invalid payload"})
				return
			}
			var f ForwardRule
			err := app.db.QueryRow(`INSERT INTO forwards(name,listen_addr,target_addr,protocol,status,node_id,connections,updated_at) VALUES($1,$2,$3,$4,'enabled',$5,$6,now()) RETURNING id,name,listen_addr,target_addr,protocol,status,node_id,connections,updated_at`, req.Name, req.ListenAddr, req.TargetAddr, req.Protocol, req.NodeID, rand.Intn(10)).Scan(&f.ID, &f.Name, &f.ListenAddr, &f.TargetAddr, &f.Protocol, &f.Status, &f.NodeID, &f.Connections, &f.UpdatedAt)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			nodeName, err := mustNodeName(req.NodeID)
			if err == nil {
				_ = scheduleForward(nodeName, req.Name, req.Protocol, req.ListenAddr, req.TargetAddr)
			}
			app.audit(r.Context(), "forward.create", fmt.Sprintf("forward/%d", f.ID), f.Name)
			writeJSON(w, 201, f)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	api.HandleFunc("/api/forwards/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "api" || parts[1] != "forwards" {
			writeJSON(w, 400, map[string]string{"error": "bad path"})
			return
		}
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid id"})
			return
		}
		action := parts[3]
		switch {
		case action == "update" && r.Method == http.MethodPatch:
			var req struct{ Name, ListenAddr, TargetAddr, Protocol string; NodeID int64 }
			if json.NewDecoder(r.Body).Decode(&req) != nil { writeJSON(w, 400, map[string]string{"error": "invalid payload"}); return }
			_, err := app.db.Exec(`UPDATE forwards SET name=COALESCE(NULLIF($2,''),name), listen_addr=COALESCE(NULLIF($3,''),listen_addr), target_addr=COALESCE(NULLIF($4,''),target_addr), protocol=COALESCE(NULLIF($5,''),protocol), node_id=COALESCE(NULLIF($6,0),node_id), updated_at=now() WHERE id=$1`, id, req.Name, req.ListenAddr, req.TargetAddr, req.Protocol, req.NodeID)
			if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
			app.audit(r.Context(), "forward.update", fmt.Sprintf("forward/%d", id), "updated")
			writeJSON(w, 200, map[string]any{"ok": true})
		case action == "delete" && r.Method == http.MethodDelete:
			res, _ := app.db.Exec(`DELETE FROM forwards WHERE id=$1`, id)
			aff, _ := res.RowsAffected()
			if aff == 0 { writeJSON(w, 404, map[string]string{"error": "forward not found"}); return }
			app.audit(r.Context(), "forward.delete", fmt.Sprintf("forward/%d", id), "deleted")
			writeJSON(w, 200, map[string]any{"ok": true})
		case action == "toggle" && r.Method == http.MethodPatch:
			_, _ = app.db.Exec(`UPDATE forwards SET status=CASE WHEN status='enabled' THEN 'disabled' ELSE 'enabled' END, updated_at=now() WHERE id=$1`, id)
			var f ForwardRule
			err = app.db.QueryRow(`SELECT id,name,listen_addr,target_addr,protocol,status,node_id,connections,updated_at FROM forwards WHERE id=$1`, id).Scan(&f.ID, &f.Name, &f.ListenAddr, &f.TargetAddr, &f.Protocol, &f.Status, &f.NodeID, &f.Connections, &f.UpdatedAt)
			if err != nil {
				writeJSON(w, 404, map[string]string{"error": "forward not found"})
				return
			}
			nodeName, err := mustNodeName(f.NodeID)
			if err == nil {
				if f.Status == "enabled" { _ = createNodeTask(nodeName, "gost.start", map[string]any{"name": f.Name}) } else { _ = createNodeTask(nodeName, "gost.stop", map[string]any{"name": f.Name}) }
			}
			app.audit(r.Context(), "forward.toggle", fmt.Sprintf("forward/%d", f.ID), f.Status)
			writeJSON(w, 200, f)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	api.HandleFunc("/api/rules", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows, _ := app.db.Query(`SELECT id,name,action,expr,priority,enabled,updated_at FROM rules ORDER BY priority DESC,id DESC`)
			defer rows.Close()
			out := []TrafficRule{}
			for rows.Next() {
				var x TrafficRule
				_ = rows.Scan(&x.ID, &x.Name, &x.Action, &x.Expr, &x.Priority, &x.Enabled, &x.UpdatedAt)
				out = append(out, x)
			}
			writeJSON(w, 200, out)
		case http.MethodPost:
			var req struct{ Name, Action, Expr string; Priority int }
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.Action == "" || req.Expr == "" {
				writeJSON(w, 400, map[string]string{"error": "invalid payload"})
				return
			}
			var x TrafficRule
			err := app.db.QueryRow(`INSERT INTO rules(name,action,expr,priority,enabled,updated_at) VALUES($1,$2,$3,$4,true,now()) RETURNING id,name,action,expr,priority,enabled,updated_at`, req.Name, req.Action, req.Expr, req.Priority).Scan(&x.ID, &x.Name, &x.Action, &x.Expr, &x.Priority, &x.Enabled, &x.UpdatedAt)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			app.audit(r.Context(), "rule.create", fmt.Sprintf("rule/%d", x.ID), x.Name)
			writeJSON(w, 201, x)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	api.HandleFunc("/api/rules/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id, err := parseID(r.URL.Path, "rules", "toggle")
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad path"})
			return
		}
		_, _ = app.db.Exec(`UPDATE rules SET enabled = NOT enabled, updated_at=now() WHERE id=$1`, id)
		var x TrafficRule
		err = app.db.QueryRow(`SELECT id,name,action,expr,priority,enabled,updated_at FROM rules WHERE id=$1`, id).Scan(&x.ID, &x.Name, &x.Action, &x.Expr, &x.Priority, &x.Enabled, &x.UpdatedAt)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "rule not found"})
			return
		}
		app.audit(r.Context(), "rule.toggle", fmt.Sprintf("rule/%d", x.ID), fmt.Sprintf("enabled=%v", x.Enabled))
		writeJSON(w, 200, x)
	})

	api.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		rows, _ := app.db.Query(`SELECT id,level,source,message,read,created_at FROM alerts ORDER BY id DESC LIMIT 200`)
		defer rows.Close()
		out := []Alert{}
		for rows.Next() {
			var a Alert
			_ = rows.Scan(&a.ID, &a.Level, &a.Source, &a.Message, &a.Read, &a.CreatedAt)
			out = append(out, a)
		}
		writeJSON(w, 200, out)
	})
	api.HandleFunc("/api/alerts/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id, err := parseID(r.URL.Path, "alerts", "read")
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad path"})
			return
		}
		_, _ = app.db.Exec(`UPDATE alerts SET read=true WHERE id=$1`, id)
		var a Alert
		err = app.db.QueryRow(`SELECT id,level,source,message,read,created_at FROM alerts WHERE id=$1`, id).Scan(&a.ID, &a.Level, &a.Source, &a.Message, &a.Read, &a.CreatedAt)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "alert not found"})
			return
		}
		app.audit(r.Context(), "alert.read", fmt.Sprintf("alert/%d", id), "read")
		writeJSON(w, 200, a)
	})

	api.HandleFunc("/ws/metrics", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			var s DashboardSummary
			_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE status='online'), COUNT(*) FROM nodes`).Scan(&s.OnlineNodes, &s.TotalNodes)
			_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE status='online') FROM clients`).Scan(&s.ActiveClients)
			_ = app.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE read=false) FROM alerts`).Scan(&s.Alerts)
			s.CurrentTraffic = float64(rand.Intn(700)+150) / 10
			if conn.WriteJSON(s) != nil {
				return
			}
		}
	})

	mux.Handle("/api/settings/alerts", app.auth(app.requireAdmin(api)))
	mux.Handle("/api/users", app.auth(app.requireAdmin(api)))
	mux.Handle("/api/users/", app.auth(app.requireAdmin(api)))
	mux.Handle("/api/audit-logs", app.auth(app.requireAdmin(api)))
	mux.Handle("/api/agent/tasks", app.auth(app.requireAdmin(api)))
	mux.Handle("/api/", app.auth(api))

	addr := ":" + port
	log.Printf("gpanel backend listening on %s", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}
