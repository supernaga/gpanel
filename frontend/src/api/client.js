const API_BASE = import.meta.env.VITE_API_BASE || ''

const getToken = () => localStorage.getItem('gpanel_token') || ''
const wsEndpoint = () => {
  if (API_BASE.startsWith('http')) return `${API_BASE.replace(/^http/, 'ws')}/ws/metrics`
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${location.host}/ws/metrics`
}

const req = (path, options = {}) => {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) }
  const token = getToken()
  if (token) headers.Authorization = `Bearer ${token}`
  return fetch(`${API_BASE}${path}`, { ...options, headers }).then(async (r) => {
    const data = await r.json().catch(() => ({}))
    if (!r.ok) throw new Error(data.error || `HTTP ${r.status}`)
    return data
  })
}

export const api = {
  login: (payload) => req('/api/auth/login', { method: 'POST', body: JSON.stringify(payload), headers: {} }),

  summary: () => req('/api/dashboard/summary'),
  runtimeSummary: () => req('/api/runtime/summary'),
  runtimeDetails: () => req('/api/runtime/details'),

  tunnels: () => req('/api/tunnels'),
  addTunnel: (payload) => req('/api/tunnels', { method: 'POST', body: JSON.stringify(payload) }),
  updateTunnel: (id, payload) => req(`/api/tunnels/${id}/update`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteTunnel: (id) => req(`/api/tunnels/${id}/delete`, { method: 'DELETE' }),
  toggleTunnel: (id) => req(`/api/tunnels/${id}/toggle`, { method: 'PATCH' }),

  chains: () => req('/api/chains'),
  addChain: (payload) => req('/api/chains', { method: 'POST', body: JSON.stringify(payload) }),
  updateChain: (id, payload) => req(`/api/chains/${id}/update`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteChain: (id) => req(`/api/chains/${id}/delete`, { method: 'DELETE' }),
  toggleChain: (id) => req(`/api/chains/${id}/toggle`, { method: 'PATCH' }),

  users: () => req('/api/users'),
  addUser: (payload) => req('/api/users', { method: 'POST', body: JSON.stringify(payload) }),
  updateUser: (id, payload) => req(`/api/users/${id}/update`, { method: 'PATCH', body: JSON.stringify(payload) }),
  auditLogs: (params = {}) => {
    const q = new URLSearchParams(params).toString()
    return req(`/api/audit-logs${q ? `?${q}` : ''}`)
  },
  alertSettings: () => req('/api/settings/alerts'),
  updateAlertSettings: (payload) => req('/api/settings/alerts', { method: 'PATCH', body: JSON.stringify(payload) }),

  agentTasks: () => req('/api/agent/tasks'),
  addAgentTask: (payload) => req('/api/agent/tasks', { method: 'POST', body: JSON.stringify(payload) }),

  nodes: () => req('/api/nodes'),
  addNode: (payload) => req('/api/nodes', { method: 'POST', body: JSON.stringify(payload) }),
  rotateNodeToken: (id) => req(`/api/nodes/${id}/token`, { method: 'POST' }),
  toggleNode: (id) => req(`/api/nodes/${id}/toggle`, { method: 'PATCH' }),
  updateNode: (id, payload) => req(`/api/nodes/${id}/update`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteNode: (id) => req(`/api/nodes/${id}/delete`, { method: 'DELETE' }),
  nodeHeartbeats: (id) => req(`/api/nodes/${id}/heartbeats`),
  nodeGostAction: (id, action, payload = {}) => req(`/api/nodes/${id}/gost/${action}`, { method: 'POST', body: JSON.stringify(payload) }),

  clients: () => req('/api/clients'),
  addClient: (payload) => req('/api/clients', { method: 'POST', body: JSON.stringify(payload) }),
  toggleClient: (id) => req(`/api/clients/${id}/toggle`, { method: 'PATCH' }),

  forwards: () => req('/api/forwards'),
  addForward: (payload) => req('/api/forwards', { method: 'POST', body: JSON.stringify(payload) }),
  updateForward: (id, payload) => req(`/api/forwards/${id}/update`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteForward: (id) => req(`/api/forwards/${id}/delete`, { method: 'DELETE' }),
  toggleForward: (id) => req(`/api/forwards/${id}/toggle`, { method: 'PATCH' }),

  rules: () => req('/api/rules'),
  addRule: (payload) => req('/api/rules', { method: 'POST', body: JSON.stringify(payload) }),
  toggleRule: (id) => req(`/api/rules/${id}/toggle`, { method: 'PATCH' }),

  alerts: () => req('/api/alerts'),
  readAlert: (id) => req(`/api/alerts/${id}/read`, { method: 'PATCH' }),

  wsConfig: () => {
    const token = getToken()
    return {
      url: wsEndpoint(),
      protocols: token ? ['gpanel.v1', token] : ['gpanel.v1'],
    }
  },
}
