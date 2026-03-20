package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

var safeServiceName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type TaskEnvelope struct {
	Task *Task `json:"task"`
}

type Task struct {
	ID       int64  `json:"id"`
	Command  string `json:"command"`
	Payload  string `json:"payload"`
	NodeName string `json:"nodeName"`
}

type GostPayload struct {
	Name     string `json:"name"`
	Mode     string `json:"mode"`
	Listen   string `json:"listen"`
	Target   string `json:"target"`
	Protocol string `json:"protocol"`
}

type Agent struct {
	panelURL   string
	token      string
	nodeUID    string
	nodeName   string
	nodeIP     string
	version    string
	client     *resty.Client
	serviceDir string
}

func main() {
	a := &Agent{
		panelURL:   strings.TrimRight(os.Getenv("PANEL_URL"), "/"),
		token:      os.Getenv("AGENT_TOKEN"),
		nodeUID:    firstNonEmpty(os.Getenv("NODE_UID"), hostID()),
		nodeName:   firstNonEmpty(os.Getenv("NODE_NAME"), "node-"+hostName()),
		nodeIP:     firstNonEmpty(os.Getenv("NODE_IP"), localIP()),
		version:    firstNonEmpty(os.Getenv("AGENT_VERSION"), "v0.3.0"),
		serviceDir: "/etc/systemd/system",
	}
	if a.panelURL == "" || a.token == "" {
		panic("PANEL_URL and AGENT_TOKEN are required")
	}
	a.client = resty.New().SetTimeout(20 * time.Second)

	for {
		a.heartbeat()
		a.pollOnce()
		time.Sleep(10 * time.Second)
	}
}

func (a *Agent) heartbeat() {
	payload := map[string]any{
		"nodeUid":      a.nodeUID,
		"nodeName":     a.nodeName,
		"nodeIp":       a.nodeIP,
		"version":      a.version,
		"latencyMs":    20,
		"region":       "Unknown",
		"capabilities": []string{"gost.install", "gost.apply_forward", "gost.apply_tunnel", "gost.start", "gost.stop", "gost.restart", "gost.status"},
		"services":     listGostServices(),
	}
	_, _ = a.client.R().SetHeader("Authorization", "Bearer "+a.token).SetBody(payload).Post(a.panelURL + "/api/agent/heartbeat")
}

func (a *Agent) pollOnce() {
	var env TaskEnvelope
	resp, err := a.client.R().SetHeader("Authorization", "Bearer "+a.token).SetResult(&env).Get(a.panelURL + "/api/agent/tasks/next?nodeUid=" + a.nodeUID + "&nodeName=" + a.nodeName)
	if err != nil || resp.IsError() || env.Task == nil {
		return
	}
	status, result := a.executeTask(env.Task)
	a.ack(env.Task.ID, status, result)
}

func (a *Agent) ack(id int64, status string, result map[string]any) {
	b, _ := json.Marshal(result)
	payload := map[string]any{"status": status, "result": string(b)}
	_, _ = a.client.R().SetHeader("Authorization", "Bearer "+a.token).SetBody(payload).Post(fmt.Sprintf("%s/api/agent/tasks/%d/ack", a.panelURL, id))
}

func (a *Agent) executeTask(t *Task) (string, map[string]any) {
	var p GostPayload
	_ = json.Unmarshal([]byte(t.Payload), &p)
	var out string
	var err error

	switch t.Command {
	case "gost.install":
		out, err = installGost()
	case "gost.apply_forward":
		out, err = a.applyForward(p)
	case "gost.apply_tunnel":
		out, err = a.applyTunnel(p)
	case "gost.start":
		out, err = ensureGostAndRun(func() (string, error) { return serviceAction(p.Name, "start") })
	case "gost.stop":
		out, err = serviceAction(p.Name, "stop")
	case "gost.restart":
		out, err = ensureGostAndRun(func() (string, error) { return serviceAction(p.Name, "restart") })
	case "gost.status":
		out, err = serviceAction(p.Name, "status")
	default:
		err = fmt.Errorf("unsupported command: %s", t.Command)
	}
	if err != nil {
		return "failed", map[string]any{"ok": false, "command": t.Command, "node": a.nodeName, "error": outOrErr(out, err)}
	}
	return "done", map[string]any{"ok": true, "command": t.Command, "node": a.nodeName, "output": out}
}

func (a *Agent) applyTunnel(p GostPayload) (string, error) {
	if p.Name == "" {
		p.Name = "tunnel"
	}
	if p.Listen == "" {
		p.Listen = ":1080"
	}
	mode := p.Mode
	if mode == "" {
		mode = "socks5"
	}
	var uri string
	switch mode {
	case "socks5":
		uri = "socks5://" + p.Listen
	case "http":
		uri = "http://" + p.Listen
	default:
		return "", fmt.Errorf("unsupported tunnel mode: %s", mode)
	}
	return ensureGostAndRun(func() (string, error) {
		return writeServiceAndStart(p.Name, fmt.Sprintf("/usr/local/bin/gost -L %s", uri))
	})
}

func (a *Agent) applyForward(p GostPayload) (string, error) {
	if p.Name == "" {
		p.Name = "forward"
	}
	if p.Listen == "" || p.Target == "" {
		return "", fmt.Errorf("listen/target required")
	}
	proto := p.Protocol
	if proto == "" {
		proto = "tcp"
	}
	return ensureGostAndRun(func() (string, error) {
		return writeServiceAndStart(p.Name, fmt.Sprintf("/usr/local/bin/gost -L %s://%s/%s", proto, p.Listen, p.Target))
	})
}

func installGost() (string, error) {
	if _, err := os.Stat("/usr/local/bin/gost"); err == nil {
		return "/usr/local/bin/gost already installed", nil
	}
	if _, err := exec.LookPath("gost"); err == nil {
		return "gost already installed in PATH", nil
	}
	arch := map[string]string{"x86_64": "amd64", "aarch64": "arm64"}[strings.TrimSpace(runBestEffort("uname -m"))]
	if arch == "" {
		arch = "amd64"
	}
	tag := strings.TrimSpace(runBestEffort("curl -fsSL https://api.github.com/repos/go-gost/gost/releases/latest | sed -n 's/.*\"tag_name\": \"\\([^\"]*\\)\".*/\\1/p' | head -n 1"))
	if tag == "" {
		return "", fmt.Errorf("failed to resolve latest gost release tag")
	}
	version := strings.TrimPrefix(tag, "v")
	asset := fmt.Sprintf("gost_%s_linux_%s.tar.gz", version, arch)
	url := fmt.Sprintf("https://github.com/go-gost/gost/releases/download/%s/%s", tag, asset)
	cmd := fmt.Sprintf("set -e; tmpdir=$(mktemp -d); cd \"$tmpdir\"; curl -fsSL -o gost.tgz %s; tar -xzf gost.tgz; install -m 0755 gost /usr/local/bin/gost", shellEscape(url))
	out, err := run(cmd)
	if err != nil {
		return out, err
	}
	if _, err := os.Stat("/usr/local/bin/gost"); err != nil {
		return out, fmt.Errorf("gost install finished but /usr/local/bin/gost not found")
	}
	return strings.TrimSpace(out + "\ninstalled /usr/local/bin/gost from " + url), nil
}

func ensureGostAndRun(fn func() (string, error)) (string, error) {
	installOut, err := installGost()
	if err != nil {
		return installOut, err
	}
	runOut, runErr := fn()
	combined := strings.TrimSpace(strings.TrimSpace(installOut) + "\n" + strings.TrimSpace(runOut))
	if runErr != nil {
		return combined, runErr
	}
	return combined, nil
}

func writeServiceAndStart(name, execLine string) (string, error) {
	service, err := serviceUnitName(name)
	if err != nil {
		return "", err
	}
	unit := fmt.Sprintf("[Unit]\nDescription=GOST %s\nAfter=network.target\n\n[Service]\nType=simple\nExecStart=%s\nRestart=always\nRestartSec=2\n\n[Install]\nWantedBy=multi-user.target\n", name, execLine)
	path := filepath.Join("/etc/systemd/system", service)
	if err := os.WriteFile(path, []byte(unit), 0644); err != nil {
		return "", err
	}
	cmd := fmt.Sprintf("systemctl daemon-reload && systemctl enable --now %s", service)
	out, err := run(cmd)
	if err != nil {
		return out, err
	}
	return verifyServiceRunning(name)
}

func verifyServiceRunning(name string) (string, error) {
	service, err := serviceUnitName(name)
	if err != nil {
		return "", err
	}
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		state := strings.TrimSpace(runBestEffort(fmt.Sprintf("systemctl is-active %s 2>/dev/null || true", service)))
		if state == "active" {
			statusOut := strings.TrimSpace(runBestEffort(fmt.Sprintf("systemctl status %s --no-pager", service)))
			if statusOut == "" {
				statusOut = service + " active"
			}
			return statusOut, nil
		}
		time.Sleep(1500 * time.Millisecond)
	}
	statusOut := strings.TrimSpace(runBestEffort(fmt.Sprintf("systemctl status %s --no-pager || true", service)))
	journalOut := strings.TrimSpace(runBestEffort(fmt.Sprintf("journalctl -u %s -n 40 --no-pager || true", service)))
	combined := strings.TrimSpace(statusOut)
	if journalOut != "" {
		if combined != "" {
			combined += "\n--- journalctl ---\n"
		}
		combined += journalOut
	}
	if combined == "" {
		combined = service + " failed to reach active state"
	}
	return combined, fmt.Errorf("%s failed to reach active state", service)
}

func serviceAction(name, action string) (string, error) {
	service, err := serviceUnitName(name)
	if err != nil {
		return "", err
	}
	cmd := fmt.Sprintf("systemctl %s %s", action, service)
	if action == "status" {
		cmd = fmt.Sprintf("systemctl status %s --no-pager", service)
	}
	return run(cmd)
}

func listGostServices() []string {
	out := strings.TrimSpace(runBestEffort("systemctl list-units --type=service --all 'gost-*.service' --no-legend | awk '{print $1}'"))
	if out == "" {
		return []string{}
	}
	items := []string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			items = append(items, line)
		}
	}
	return items
}

func run(cmd string) (string, error) {
	c := exec.Command("bash", "-lc", cmd)
	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = &buf
	err := c.Run()
	out := buf.String()
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("%s", msg)
	}
	return out, nil
}

func runBestEffort(cmd string) string {
	out, _ := run(cmd)
	return out
}

func validateServiceName(name string) error {
	if !safeServiceName.MatchString(name) {
		return fmt.Errorf("invalid service name %q", name)
	}
	return nil
}

func serviceUnitName(name string) (string, error) {
	if err := validateServiceName(name); err != nil {
		return "", err
	}
	return "gost-" + name + ".service", nil
}

func shellEscape(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
func firstNonEmpty(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
func hostID() string {
	b, _ := os.ReadFile("/etc/machine-id")
	if len(bytes.TrimSpace(b)) > 0 {
		return strings.TrimSpace(string(b))
	}
	return hostName()
}
func hostName() string {
	h, _ := os.Hostname()
	if h == "" {
		return "unknown"
	}
	return h
}
func localIP() string { return strings.TrimSpace(runBestEffort("hostname -I | awk '{print $1}'")) }
func outOrErr(out string, err error) string {
	if strings.TrimSpace(out) != "" {
		return out
	}
	return err.Error()
}
