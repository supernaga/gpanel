<template>
  <section>
    <div class="toolbar">
      <div>
        <h1>隧道</h1>
        <p class="hint">管理单节点 HTTP / SOCKS5 隧道入口。</p>
      </div>
      <form class="inline-form" @submit.prevent="createTunnel">
        <input v-model="form.name" placeholder="名称" required />
        <select v-model="form.mode"><option value="socks5">socks5</option><option value="http">http</option></select>
        <input v-model="form.listen" placeholder=":1080" required />
        <select v-model.number="form.nodeId">
          <option v-for="n in nodes" :key="n.id" :value="n.id">{{ n.name }}</option>
        </select>
        <button type="submit">创建</button>
      </form>
    </div>

    <table>
      <thead><tr><th>名称</th><th>模式</th><th>监听</th><th>节点</th><th>状态</th><th>说明</th></tr></thead>
      <tbody>
        <tr v-for="t in tunnels" :key="t.id">
          <td>{{ t.name }}</td>
          <td>{{ t.mode }}</td>
          <td>{{ t.listen }}</td>
          <td>{{ nodeName(t.nodeId) }}</td>
          <td><span :class="['badge', t.enabled ? 'online' : 'offline']">{{ t.enabled ? 'enabled' : 'disabled' }}</span></td>
          <td>{{ t.description || '-' }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'

const nodes = ref([])
const tunnels = ref([])
const form = ref({ name: '', mode: 'socks5', listen: ':1080', nodeId: 0 })

const nodeName = (id) => nodes.value.find((n) => n.id === id)?.name || `#${id}`
const load = async () => {
  nodes.value = await api.nodes()
  if (!form.value.nodeId && nodes.value.length) form.value.nodeId = nodes.value[0].id
  tunnels.value = await api.tunnels()
}
const createTunnel = async () => {
  await api.addTunnel(form.value)
  form.value = { ...form.value, name: '', listen: ':1080' }
  await load()
}

onMounted(load)
</script>
