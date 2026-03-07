<template>
  <section>
    <div class="toolbar">
      <div>
        <h1>链路编排</h1>
        <p class="hint">在多个节点之间定义转发拓扑，例如 B → C → D。</p>
      </div>
      <form class="inline-form" @submit.prevent="createChain">
        <input v-model="form.name" placeholder="链路名称" required />
        <input v-model="form.path" placeholder="如 B -> C -> D" required />
        <select v-model="form.protocol"><option value="tcp">tcp</option><option value="udp">udp</option><option value="socks5">socks5</option><option value="http">http</option></select>
        <button type="submit">创建</button>
      </form>
    </div>

    <table>
      <thead><tr><th>名称</th><th>路径</th><th>协议</th><th>状态</th><th>说明</th></tr></thead>
      <tbody>
        <tr v-for="c in chains" :key="c.id">
          <td>{{ c.name }}</td>
          <td>{{ c.path }}</td>
          <td>{{ c.protocol }}</td>
          <td><span :class="['badge', c.enabled ? 'online' : 'offline']">{{ c.enabled ? 'enabled' : 'draft' }}</span></td>
          <td>{{ c.description || '待绑定真实节点任务' }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'

const chains = ref([])
const form = ref({ name: '', path: '', protocol: 'tcp' })
const load = async () => { chains.value = await api.chains() }
const createChain = async () => {
  await api.addChain(form.value)
  form.value = { name: '', path: '', protocol: 'tcp' }
  await load()
}
onMounted(load)
</script>
