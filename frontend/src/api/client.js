const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080'

const j = (r) => r.json()

export const api = {
  summary: () => fetch(`${API_BASE}/api/dashboard/summary`).then(j),

  nodes: () => fetch(`${API_BASE}/api/nodes`).then(j),
  addNode: (payload) => fetch(`${API_BASE}/api/nodes`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload)
  }).then(j),
  toggleNode: (id) => fetch(`${API_BASE}/api/nodes/${id}/toggle`, { method: 'PATCH' }).then(j),

  clients: () => fetch(`${API_BASE}/api/clients`).then(j),
  addClient: (payload) => fetch(`${API_BASE}/api/clients`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload)
  }).then(j),
  toggleClient: (id) => fetch(`${API_BASE}/api/clients/${id}/toggle`, { method: 'PATCH' }).then(j),

  forwards: () => fetch(`${API_BASE}/api/forwards`).then(j),
  addForward: (payload) => fetch(`${API_BASE}/api/forwards`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload)
  }).then(j),
  toggleForward: (id) => fetch(`${API_BASE}/api/forwards/${id}/toggle`, { method: 'PATCH' }).then(j),

  rules: () => fetch(`${API_BASE}/api/rules`).then(j),
  addRule: (payload) => fetch(`${API_BASE}/api/rules`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload)
  }).then(j),
  toggleRule: (id) => fetch(`${API_BASE}/api/rules/${id}/toggle`, { method: 'PATCH' }).then(j),

  alerts: () => fetch(`${API_BASE}/api/alerts`).then(j),
  readAlert: (id) => fetch(`${API_BASE}/api/alerts/${id}/read`, { method: 'PATCH' }).then(j),

  wsUrl: () => `${API_BASE.replace('http', 'ws')}/ws/metrics`
}
