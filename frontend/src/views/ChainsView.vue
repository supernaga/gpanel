<template>
  <section>
    <div class="toolbar">
      <div>
        <h1>链路编排</h1>
        <p class="hint">在多个节点之间定义转发拓扑，例如 B → C → D。</p>
      </div>
      <form class="inline-form" @submit.prevent="createChain">
        <input v-model="form.name" placeholder="链路名称" required />
        <input v-model="form.path" placeholder="如 B -> C -> D（兼容旧格式）" />
        <select v-model="form.protocol"><option value="tcp">tcp</option><option value="udp">udp</option><option value="socks5">socks5</option><option value="http">http</option></select>
        <button type="submit">创建</button>
      </form>
    </div>

    <div class="toolbar" style="margin-top:12px">
      <h2>最小双节点链路</h2>
      <form class="inline-form" @submit.prevent="createTwoNodeChain">
        <input v-model="twoNodeForm.name" placeholder="链路名称" required />
        <select v-model.number="twoNodeForm.forwardNodeId">
          <option v-for="n in selectableNodes" :key="`f-${n.id}`" :value="n.id">Forward: {{ n.name }} ({{ n.status }})</option>
        </select>
        <input v-model="twoNodeForm.listenAddr" placeholder=":19021" required />
        <input v-model="twoNodeForm.targetAddr" placeholder="8.8.8.8:53" required />
        <select v-model.number="twoNodeForm.tunnelNodeId">
          <option v-for="n in selectableNodes" :key="`t-${n.id}`" :value="n.id">Tunnel: {{ n.name }} ({{ n.status }})</option>
        </select>
        <select v-model="twoNodeForm.tunnelMode"><option value="socks5">socks5</option><option value="http">http</option></select>
        <input v-model="twoNodeForm.tunnelListen" placeholder=":11081" required />
        <button type="submit">创建双节点链路</button>
      </form>
      <p class="hint">当前在线节点 {{ onlineNodeCount }} / {{ nodes.length }}。表单会优先排序在线节点；若只有一个在线节点，也允许先用 offline 草稿节点占位。</p>
    </div>

    <table>
      <thead><tr><th>名称</th><th>路径</th><th>协议</th><th>期望状态</th><th>实际状态</th><th>说明</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="c in sortedChains" :key="c.id">
          <td>
            <template v-if="editingId === c.id"><input v-model="editForm.name" /></template>
            <template v-else>{{ c.name }}</template>
          </td>
          <td>
            <template v-if="editingId === c.id"><input v-model="editForm.path" /></template>
            <template v-else>{{ c.path }}</template>
          </td>
          <td>
            <template v-if="editingId === c.id">
              <select v-model="editForm.protocol"><option value="tcp">tcp</option><option value="udp">udp</option><option value="socks5">socks5</option><option value="http">http</option></select>
            </template>
            <template v-else>{{ c.protocol }}</template>
          </td>
          <td><span :class="['badge', c.enabled ? 'online' : 'offline']">{{ c.enabled ? 'enabled' : 'draft' }}</span></td>
          <td><span :class="['badge', chainMismatch(c) ? 'offline' : (chainState(c.id)?.allRunning ? 'online' : '')]">{{ chainState(c.id)?.allRunning ? 'all-running' : 'partial' }}</span></td>
          <td>{{ c.description || '待绑定真实节点任务' }}</td>
          <td>
            <button @click="toggle(c.id)">{{ c.enabled ? '停用' : '启用' }}</button>
            <button v-if="editingId !== c.id" @click="startEdit(c)">编辑</button>
            <button v-else @click="saveEdit(c.id)">保存</button>
            <button v-if="editingId === c.id" @click="cancelEdit">取消</button>
            <button @click="removeChain(c.id)">删除</button>
          </td>
        </tr>
      </tbody>
    </table>

    <div class="toolbar" style="margin-top:20px"><h2>Hop 运行状态</h2></div>
    <table>
      <thead><tr><th>链路</th><th>Hop</th><th>节点</th><th>服务</th><th>状态</th></tr></thead>
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
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'

const chains = ref([])
const nodes = ref([])
const runtime = ref({ chainStates: [] })
const form = ref({ name: '', path: '', protocol: 'tcp' })
const twoNodeForm = ref({ name: '', forwardNodeId: 0, listenAddr: ':19021', targetAddr: '8.8.8.8:53', tunnelNodeId: 0, tunnelMode: 'socks5', tunnelListen: ':11081' })
const editingId = ref(null)
const editForm = ref({ name: '', path: '', protocol: 'tcp' })

const chainState = (id) => (runtime.value.chainStates || []).find(item => item.id === id)
const selectableNodes = computed(() => [...nodes.value].sort((a, b) => {
  const ao = a.status === 'online' ? 1 : 0
  const bo = b.status === 'online' ? 1 : 0
  if (ao !== bo) return bo - ao
  return a.id - b.id
}))
const onlineNodeCount = computed(() => nodes.value.filter(n => n.status === 'online').length)
const chainMismatch = (chain) => {
  const state = chainState(chain.id)
  if (!state) return false
  return chain.enabled ? !state.allRunning : state.allRunning
}
const sortedChains = computed(() => [...chains.value].sort((a, b) => {
  const am = chainMismatch(a) ? 1 : 0
  const bm = chainMismatch(b) ? 1 : 0
  if (am !== bm) return bm - am
  return b.id - a.id
}))
const chainHopRows = computed(() => (runtime.value.chainStates || []).flatMap(chain =>
  (chain.hops || []).map(hop => ({
    key: `${chain.id}-${hop.index}`,
    chainName: chain.name,
    index: hop.index,
    nodeName: hop.nodeName,
    serviceName: hop.serviceName,
    actualRunning: hop.actualRunning,
  }))
))
const load = async () => {
  const [chainRows, details, nodeRows] = await Promise.all([api.chains(), api.runtimeDetails(), api.nodes()])
  chains.value = chainRows
  runtime.value = details
  nodes.value = nodeRows
  const orderedNodes = [...nodeRows].sort((a, b) => {
    const ao = a.status === 'online' ? 1 : 0
    const bo = b.status === 'online' ? 1 : 0
    if (ao !== bo) return bo - ao
    return a.id - b.id
  })
  if (!twoNodeForm.value.forwardNodeId && orderedNodes.length) twoNodeForm.value.forwardNodeId = orderedNodes[0].id
  if (!twoNodeForm.value.tunnelNodeId && orderedNodes.length) twoNodeForm.value.tunnelNodeId = orderedNodes[Math.min(1, orderedNodes.length - 1)].id
}
const createChain = async () => {
  await api.addChain(form.value)
  form.value = { name: '', path: '', protocol: 'tcp' }
  await load()
}
const createTwoNodeChain = async () => {
  await api.addChain({
    name: twoNodeForm.value.name,
    protocol: form.value.protocol,
    hops: [
      {
        nodeId: twoNodeForm.value.forwardNodeId,
        type: 'forward',
        listenAddr: twoNodeForm.value.listenAddr,
        targetAddr: twoNodeForm.value.targetAddr,
        protocol: form.value.protocol,
      },
      {
        nodeId: twoNodeForm.value.tunnelNodeId,
        type: 'tunnel',
        listenAddr: twoNodeForm.value.tunnelListen,
        protocol: twoNodeForm.value.tunnelMode,
      },
    ],
  })
  twoNodeForm.value = { ...twoNodeForm.value, name: '', listenAddr: ':19021', targetAddr: '8.8.8.8:53', tunnelMode: 'socks5', tunnelListen: ':11081' }
  await load()
}
const startEdit = (c) => {
  editingId.value = c.id
  editForm.value = { name: c.name, path: c.path, protocol: c.protocol }
}
const cancelEdit = () => { editingId.value = null }
const saveEdit = async (id) => {
  await api.updateChain(id, editForm.value)
  editingId.value = null
  await load()
}
const removeChain = async (id) => {
  if (!confirm(`确认删除链路 #${id} 吗？`)) return
  await api.deleteChain(id)
  await load()
}
const toggle = async (id) => { await api.toggleChain(id); await load() }

onMounted(load)
</script>
