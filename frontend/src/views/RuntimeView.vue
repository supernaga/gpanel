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

    <div class="toolbar" style="margin-top:20px"><h2>转发状态</h2></div>
    <table>
      <thead><tr><th>名称</th><th>监听</th><th>目标</th><th>协议</th><th>节点</th><th>期望状态</th></tr></thead>
      <tbody>
        <tr v-for="f in details.forwards" :key="f.id">
          <td>{{ f.name }}</td>
          <td>{{ f.listenAddr }}</td>
          <td>{{ f.targetAddr }}</td>
          <td>{{ f.protocol }}</td>
          <td>#{{ f.nodeId }}</td>
          <td><span :class="['badge', f.status === 'enabled' ? 'online' : 'offline']">{{ f.status }}</span></td>
        </tr>
      </tbody>
    </table>

    <div class="toolbar" style="margin-top:20px"><h2>隧道状态</h2></div>
    <table>
      <thead><tr><th>名称</th><th>模式</th><th>监听</th><th>节点</th><th>期望状态</th><th>说明</th></tr></thead>
      <tbody>
        <tr v-for="t in details.tunnels" :key="t.id">
          <td>{{ t.name }}</td>
          <td>{{ t.mode }}</td>
          <td>{{ t.listen }}</td>
          <td>#{{ t.nodeId }}</td>
          <td><span :class="['badge', t.enabled ? 'online' : 'offline']">{{ t.enabled ? 'enabled' : 'disabled' }}</span></td>
          <td>{{ t.description }}</td>
        </tr>
      </tbody>
    </table>

    <div class="toolbar" style="margin-top:20px"><h2>链路状态</h2></div>
    <table>
      <thead><tr><th>名称</th><th>路径</th><th>协议</th><th>期望状态</th><th>说明</th></tr></thead>
      <tbody>
        <tr v-for="c in details.chains" :key="c.id">
          <td>{{ c.name }}</td>
          <td>{{ c.path }}</td>
          <td>{{ c.protocol }}</td>
          <td><span :class="['badge', c.enabled ? 'online' : 'offline']">{{ c.enabled ? 'enabled' : 'draft' }}</span></td>
          <td>{{ c.description }}</td>
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
          <td><span :class="['badge', t.status === 'done' ? 'online' : (t.status === 'failed' ? 'offline' : '')]">{{ t.status }}</span></td>
          <td>{{ t.priority }}</td>
          <td>{{ formatTime(t.createdAt) }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'

const summary = ref({ nodes: 0, forwards: 0, tunnels: 0, chains: 0 })
const details = ref({ nodes: [], forwards: [], tunnels: [], chains: [], tasks: [], taskStats: { pending: 0, running: 0, done: 0, failed: 0 } })
const formatTime = (value) => value ? new Date(value).toLocaleString() : '-'
const load = async () => {
  const [s, d] = await Promise.all([api.runtimeSummary(), api.runtimeDetails()])
  summary.value = s
  details.value = d
}

onMounted(load)
</script>
