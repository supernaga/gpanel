<template>
  <section>
    <div class="toolbar">
      <div>
        <h1>运行态</h1>
        <p class="hint">查看期望配置与节点实际运行状态的差异。</p>
      </div>
      <button @click="load">刷新</button>
    </div>

    <div class="cards">
      <div class="card"><p>节点数</p><strong>{{ data.nodes }}</strong></div>
      <div class="card"><p>转发数</p><strong>{{ data.forwards }}</strong></div>
      <div class="card"><p>隧道数</p><strong>{{ data.tunnels }}</strong></div>
      <div class="card"><p>链路数</p><strong>{{ data.chains }}</strong></div>
    </div>

    <p class="hint" style="margin-top:16px">下一阶段这里会接入 agent 状态回采、gost 服务状态和配置对账。</p>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
const data = ref({ nodes: 0, forwards: 0, tunnels: 0, chains: 0 })
const load = async () => { data.value = await api.runtimeSummary() }
onMounted(load)
</script>
