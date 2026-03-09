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
      <thead><tr><th>名称</th><th>模式</th><th>监听</th><th>节点</th><th>期望状态</th><th>实际状态</th><th>服务</th><th>说明</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="t in tunnels" :key="t.id">
          <td><template v-if="editingId===t.id"><input v-model="editForm.name" /></template><template v-else>{{ t.name }}</template></td>
          <td><template v-if="editingId===t.id"><select v-model="editForm.mode"><option value="socks5">socks5</option><option value="http">http</option></select></template><template v-else>{{ t.mode }}</template></td>
          <td><template v-if="editingId===t.id"><input v-model="editForm.listen" /></template><template v-else>{{ t.listen }}</template></td>
          <td>
            <template v-if="editingId===t.id">
              <select v-model.number="editForm.nodeId"><option v-for="n in nodes" :key="n.id" :value="n.id">{{ n.name }}</option></select>
            </template>
            <template v-else>{{ nodeName(t.nodeId) }}</template>
          </td>
          <td><span :class="['badge', t.enabled ? 'online' : 'offline']">{{ t.enabled ? 'enabled' : 'disabled' }}</span></td>
          <td><span :class="['badge', tunnelMismatch(t) ? 'offline' : (tunnelState(t.id)?.actualRunning ? 'online' : '')]">{{ tunnelState(t.id)?.actualRunning ? 'running' : 'not-running' }}</span></td>
          <td>{{ tunnelState(t.id)?.serviceName || '-' }}</td>
          <td>{{ t.description || '-' }}</td>
          <td>
            <button @click="toggleTunnel(t.id)">{{ t.enabled ? '停用' : '启用' }}</button>
            <button v-if="editingId !== t.id" @click="startEdit(t)">编辑</button>
            <button v-else @click="saveEdit(t.id)">保存</button>
            <button v-if="editingId === t.id" @click="cancelEdit">取消</button>
            <button @click="removeTunnel(t.id)">删除</button>
          </td>
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
const runtime = ref({ tunnelStates: [] })
const form = ref({ name: '', mode: 'socks5', listen: ':1080', nodeId: 0 })
const editingId = ref(null)
const editForm = ref({ name: '', mode: 'socks5', listen: ':1080', nodeId: 0 })

const tunnelState = (id) => (runtime.value.tunnelStates || []).find(item => item.id === id)
const tunnelMismatch = (tunnel) => {
  const state = tunnelState(tunnel.id)
  if (!state) return false
  return tunnel.enabled ? !state.actualRunning : state.actualRunning
}
const nodeName = (id) => nodes.value.find((n) => n.id === id)?.name || `#${id}`
const load = async () => {
  const [nodeRows, tunnelRows, details] = await Promise.all([api.nodes(), api.tunnels(), api.runtimeDetails()])
  nodes.value = nodeRows
  if (!form.value.nodeId && nodes.value.length) form.value.nodeId = nodes.value[0].id
  tunnels.value = tunnelRows
  runtime.value = details
}
const createTunnel = async () => {
  await api.addTunnel(form.value)
  form.value = { ...form.value, name: '', listen: ':1080' }
  await load()
}
const startEdit = (t) => {
  editingId.value = t.id
  editForm.value = { name: t.name, mode: t.mode, listen: t.listen, nodeId: t.nodeId }
}
const cancelEdit = () => { editingId.value = null }
const saveEdit = async (id) => {
  await api.updateTunnel(id, editForm.value)
  editingId.value = null
  await load()
}
const removeTunnel = async (id) => {
  if (!confirm(`确认删除隧道 #${id} 吗？`)) return
  await api.deleteTunnel(id)
  await load()
}
const toggleTunnel = async (id) => { await api.toggleTunnel(id); await load() }

onMounted(load)
</script>
