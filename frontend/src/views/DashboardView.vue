<template>
  <section>
    <h1>系统总览</h1>
    <div class="cards">
      <div class="card"><p>在线节点</p><strong>{{ summary.onlineNodes }}/{{ summary.totalNodes }}</strong></div>
      <div class="card"><p>当前流量</p><strong>{{ summary.currentTrafficMbps }} Mbps</strong></div>
      <div class="card"><p>活跃客户端</p><strong>{{ summary.activeClients }}</strong></div>
      <div class="card"><p>告警数</p><strong :class="{danger: summary.alerts > 0}">{{ summary.alerts }}</strong></div>
    </div>
    <p class="hint">实时数据每 2 秒刷新（WebSocket）</p>
  </section>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '../api/client'

const summary = ref({ onlineNodes: 0, totalNodes: 0, currentTrafficMbps: 0, activeClients: 0, alerts: 0 })
let ws

onMounted(async () => {
  summary.value = await api.summary()
  const { url, protocols } = api.wsConfig()
  ws = new WebSocket(url, protocols)
  ws.onmessage = (e) => {
    summary.value = JSON.parse(e.data)
  }
})

onUnmounted(() => {
  if (ws) ws.close()
})
</script>
