const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080'

export const api = {
  summary: () => fetch(`${API_BASE}/api/dashboard/summary`).then(r => r.json()),
  nodes: () => fetch(`${API_BASE}/api/nodes`).then(r => r.json()),
  addNode: (payload) => fetch(`${API_BASE}/api/nodes`, {
    method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify(payload)
  }).then(r => r.json()),
  toggleNode: (id) => fetch(`${API_BASE}/api/nodes/${id}/toggle`, { method: 'PATCH' }).then(r => r.json()),
  wsUrl: () => `${API_BASE.replace('http', 'ws')}/ws/metrics`
}
