<template>
  <section>
    <div class="toolbar">
      <div>
        <h1>运行态</h1>
        <p class="hint">查看节点、链路和最近任务的当前状态。</p>
      </div>
      <button @click="load">刷新</button>
    </div>

    <div class="cards">
      <div class="card"><p>节点数</p><strong>{{ summary.nodes }}</strong></div>
      <div class="card"><p>转发数</p><strong>{{ summary.forwards }}</strong></div>
      <div class="card"><p>隧道数</p><strong>{{ summary.tunnels }}</strong></div>
      <div class="card"><p>链路数</p><strong>{{ summary.chains }}</strong></div>
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

    <div class="toolbar" style="margin-top:20px"><h2>链路状态</h2></div>
    <table>
      <thead><tr><th>名称</th><th>路径</th><th>协议</th><th>状态</th><th>说明</th></tr></thead>
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
const details = ref({ nodes: [], chains: [], tasks: [] })
const formatTime = (value) => value ? new Date(value).toLocaleString() : '-'
const load = async () => {
  const [s, d] = await Promise.all([api.runtimeSummary(), api.runtimeDetails()])
  summary.value = s
  details.value = d
}

onMounted(load)
</script>
