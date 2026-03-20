<template>
  <section>
    <div class="toolbar">
      <div>
        <h1>运行态</h1>
        <p class="hint">查看节点、资源和最近任务的当前状态，用于联调前巡检。</p>
      </div>
      <button @click="load">刷新</button>
    </div>

    <div class="cards">
      <div class="card"><p>节点数</p><strong>{{ summary.nodes }}</strong></div>
      <div class="card"><p>转发数</p><strong>{{ summary.forwards }}</strong></div>
      <div class="card"><p>隧道数</p><strong>{{ summary.tunnels }}</strong></div>
      <div class="card"><p>链路数</p><strong>{{ summary.chains }}</strong></div>
    </div>

    <div class="cards" style="margin-top:16px">
      <div class="card"><p>待执行任务</p><strong>{{ details.taskStats.pending || 0 }}</strong></div>
      <div class="card"><p>运行中任务</p><strong>{{ details.taskStats.running || 0 }}</strong></div>
      <div class="card"><p>完成任务</p><strong>{{ details.taskStats.done || 0 }}</strong></div>
      <div class="card"><p>失败任务</p><strong class="danger">{{ details.taskStats.failed || 0 }}</strong></div>
    </div>

    <div class="cards" style="margin-top:16px">
      <div class="card"><p>转发异常</p><strong :class="forwardMismatchCount ? 'danger' : ''">{{ forwardMismatchCount }}</strong></div>
      <div class="card"><p>隧道异常</p><strong :class="tunnelMismatchCount ? 'danger' : ''">{{ tunnelMismatchCount }}</strong></div>
      <div class="card"><p>链路异常</p><strong :class="chainMismatchCount ? 'danger' : ''">{{ chainMismatchCount }}</strong></div>
      <div class="card"><p>总异常</p><strong :class="totalMismatchCount ? 'danger' : ''">{{ totalMismatchCount }}</strong></div>
    </div>

    <div class="toolbar" style="margin-top:20px"><h2>节点状态</h2></div>
    <table>
      <thead><tr><th>节点</th><th>地域</th><th>状态</th><th>延迟</th><th>版本</th><th>更新时间</th></tr></thead>
      <tbody>
        <tr v-for="n in details.nodes" :key="n.id">
          <td>{{ n.name }}</td>
          <td>{{ n.region }}</td>
          <td><span :class="['badge', n.status === 'online' ? 'online' : 'offline']">{{ n.status }}</span></td>
          <td>{{ n.latencyMs }} ms</td>
          <td>{{ n.version }}</td>
          <td>{{ formatTime(n.updatedAt) }}</td>
        </tr>
      </tbody>
    </table>

    <div class="toolbar" style="margin-top:20px"><h2>Agent 快照</h2></div>
    <table>
      <thead><tr><th>节点</th><th>IP</th><th>版本</th><th>能力</th><th>服务</th><th>心跳时间</th></tr></thead>
      <tbody>
        <tr v-for="hb in details.heartbeats" :key="hb.nodeUid">
          <td>{{ hb.nodeName }}</td>
          <td>{{ hb.nodeIp }}</td>
          <td>{{ hb.version }}</td>
          <td>{{ parseList(hb.capabilities).join(', ') || '-' }}</td>
          <td>{{ parseList(hb.services).join(', ') || '-' }}</td>
          <td>{{ formatTime(hb.createdAt) }}</td>
        </tr>
      </tbody>
    </table>

    <div class="toolbar" style="margin-top:20px"><h2>转发状态</h2></div>
    <table>
      <thead><tr><th>名称</th><th>监听</th><th>目标</th><th>协议</th><th>节点</th><th>期望状态</th><th>实际状态</th><th>服务</th></tr></thead>
      <tbody>
        <tr v-for="f in details.forwards" :key="f.id">
          <td>{{ f.name }}</td>
          <td>{{ f.listenAddr }}</td>
          <td>{{ f.targetAddr }}</td>
          <td>{{ f.protocol }}</td>
          <td>{{ findForwardState(f.id)?.nodeName || ('#' + f.nodeId) }}</td>
          <td><span :class="['badge', f.status === 'enabled' ? 'online' : 'offline']">{{ f.status }}</span></td>
          <td><span :class="['badge', forwardMismatch(f) ? 'offline' : (findForwardState(f.id)?.actualRunning ? 'online' : '')]">{{ findForwardState(f.id)?.actualRunning ? 'running' : 'not-running' }}</span></td>
          <td>{{ findForwardState(f.id)?.serviceName || '-' }}</td>
        </tr>
      </tbody>
    </table>

    <div class="toolbar" style="margin-top:20px"><h2>隧道状态</h2></div>
    <table>
      <thead><tr><th>名称</th><th>模式</th><th>监听</th><th>节点</th><th>期望状态</th><th>实际状态</th><th>服务</th><th>说明</th></tr></thead>
      <tbody>
        <tr v-for="t in details.tunnels" :key="t.id">
          <td>{{ t.name }}</td>
          <td>{{ t.mode }}</td>
          <td>{{ t.listen }}</td>
          <td>{{ findTunnelState(t.id)?.nodeName || ('#' + t.nodeId) }}</td>
          <td><span :class="['badge', t.enabled ? 'online' : 'offline']">{{ t.enabled ? 'enabled' : 'disabled' }}</span></td>
          <td><span :class="['badge', tunnelMismatch(t) ? 'offline' : (findTunnelState(t.id)?.actualRunning ? 'online' : '')]">{{ findTunnelState(t.id)?.actualRunning ? 'running' : 'not-running' }}</span></td>
          <td>{{ findTunnelState(t.id)?.serviceName || '-' }}</td>
          <td>{{ t.description }}</td>
        </tr>
      </tbody>
    </table>

    <div class="toolbar" style="margin-top:20px"><h2>链路状态</h2></div>
    <table>
      <thead><tr><th>名称</th><th>路径</th><th>协议</th><th>期望状态</th><th>实际状态</th><th>说明</th></tr></thead>
      <tbody>
        <tr v-for="c in details.chains" :key="c.id">
          <td>{{ c.name }}</td>
          <td>{{ c.path }}</td>
          <td>{{ c.protocol }}</td>
          <td><span :class="['badge', c.enabled ? 'online' : 'offline']">{{ c.enabled ? 'enabled' : 'draft' }}</span></td>
          <td><span :class="['badge', chainMismatch(c) ? 'offline' : (findChainState(c.id)?.allRunning ? 'online' : '')]">{{ findChainState(c.id)?.allRunning ? 'all-running' : 'partial' }}</span></td>
          <td>{{ c.description }}</td>
        </tr>
      </tbody>
    </table>

    <div class="toolbar" style="margin-top:20px"><h2>链路 Hop 状态</h2></div>
    <table>
      <thead><tr><th>链路</th><th>Hop</th><th>节点</th><th>服务</th><th>实际状态</th></tr></thead>
      <tbody>
        <tr v-for="row in chainHopRows" :key="row.key">
          <td>{{ row.chainName }}</td>
          <td>#{{ row.index }}</td>
          <td>{{ row.nodeName }}</td>
          <td>{{ row.serviceName }}</td>
          <td><span :class="['badge', row.actualRunning ? 'online' : 'offline']">{{ row.actualRunning ? 'running' : 'not-running' }}</span></td>
        </tr>
      </tbody>
    </table>

    <div class="toolbar" style="margin-top:20px"><h2>最近任务</h2></div>
    <table>
      <thead><tr><th>ID</th><th>节点</th><th>命令</th><th>状态</th><th>优先级</th><th>创建时间</th></tr></thead>
      <tbody>
        <tr v-for="t in details.tasks" :key="t.id">
          <td>#{{ t.id }}</td>
          <td>{{ t.nodeName || t.nodeUid || '-' }}</td>
          <td>{{ t.command }}</td>
          <td><span :class="['badge', taskStatusClass(t.status)]">{{ taskStatusLabel(t.status) }}</span></td>
          <td>{{ t.priority }}</td>
          <td>{{ formatTime(t.createdAt) }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'

const summary = ref({ nodes: 0, forwards: 0, tunnels: 0, chains: 0 })
const details = ref({ nodes: [], heartbeats: [], forwards: [], tunnels: [], chains: [], tasks: [], taskStats: { pending: 0, running: 0, done: 0, failed: 0 }, forwardStates: [], tunnelStates: [], chainStates: [] })
const formatTime = (value) => value ? new Date(value).toLocaleString() : '-'
const parseList = (value) => {
  try { return JSON.parse(value || '[]') } catch { return [] }
}
const findForwardState = (id) => (details.value.forwardStates || []).find(item => item.id === id)
const findTunnelState = (id) => (details.value.tunnelStates || []).find(item => item.id === id)
const findChainState = (id) => (details.value.chainStates || []).find(item => item.id === id)
const taskStatusLabel = (status) => {
  if (status === 'dispatched') return 'running'
  if (status === 'success') return 'done'
  return status
}
const taskStatusClass = (status) => {
  const normalized = taskStatusLabel(status)
  if (normalized === 'done') return 'online'
  if (normalized === 'failed') return 'offline'
  return ''
}
const forwardMismatch = (forward) => {
  const state = findForwardState(forward.id)
  if (!state) return false
  return forward.status === 'enabled' ? !state.actualRunning : state.actualRunning
}
const tunnelMismatch = (tunnel) => {
  const state = findTunnelState(tunnel.id)
  if (!state) return false
  return tunnel.enabled ? !state.actualRunning : state.actualRunning
}
const chainMismatch = (chain) => {
  const state = findChainState(chain.id)
  if (!state) return false
  return chain.enabled ? !state.allRunning : state.allRunning
}
const chainHopRows = computed(() => (details.value.chainStates || []).flatMap(chain =>
  (chain.hops || []).map(hop => ({
    key: `${chain.id}-${hop.index}`,
    chainName: chain.name,
    index: hop.index,
    nodeName: hop.nodeName,
    serviceName: hop.serviceName,
    actualRunning: hop.actualRunning,
  }))
))
const forwardMismatchCount = computed(() => (details.value.forwards || []).filter(forwardMismatch).length)
const tunnelMismatchCount = computed(() => (details.value.tunnels || []).filter(tunnelMismatch).length)
const chainMismatchCount = computed(() => (details.value.chains || []).filter(chainMismatch).length)
const totalMismatchCount = computed(() => forwardMismatchCount.value + tunnelMismatchCount.value + chainMismatchCount.value)
const load = async () => {
  const [s, d] = await Promise.all([api.runtimeSummary(), api.runtimeDetails()])
  summary.value = s
  details.value = d
}

onMounted(load)
</script>
