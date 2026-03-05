<template>
  <section>
    <div class="toolbar">
      <h1>节点管理</h1>
      <form @submit.prevent="createNode" class="inline-form">
        <input v-model="form.name" placeholder="节点名，如 hk-edge-02" required />
        <input v-model="form.region" placeholder="区域，如 Hong Kong" required />
        <button type="submit">新增节点</button>
      </form>
    </div>

    <table>
      <thead>
        <tr><th>ID</th><th>名称</th><th>区域</th><th>状态</th><th>延迟</th><th>版本</th><th>操作</th></tr>
      </thead>
      <tbody>
        <tr v-for="n in nodes" :key="n.id">
          <td>{{ n.id }}</td>
          <td>
            <template v-if="editingId === n.id"><input v-model="editForm.name" /></template>
            <template v-else>{{ n.name }}</template>
          </td>
          <td>
            <template v-if="editingId === n.id"><input v-model="editForm.region" /></template>
            <template v-else>{{ n.region }}</template>
          </td>
          <td><span :class="['badge', n.status]">{{ n.status }}</span></td>
          <td>{{ n.latencyMs }} ms</td>
          <td>
            <template v-if="editingId === n.id"><input v-model="editForm.version" /></template>
            <template v-else>{{ n.version }}</template>
          </td>
          <td>
            <button @click="toggle(n.id)">{{ n.status === 'online' ? '下线' : '上线' }}</button>
            <button v-if="editingId !== n.id" @click="startEdit(n)">编辑</button>
            <button v-else @click="saveEdit(n.id)">保存</button>
            <button v-if="editingId === n.id" @click="cancelEdit">取消</button>
            <button @click="showHeartbeats(n.id)">详情</button>
            <button @click="removeNode(n.id)">删除</button>
          </td>
        </tr>
      </tbody>
    </table>

    <div v-if="heartbeatNodeId" style="margin-top:14px">
      <h3>节点 {{ heartbeatNodeId }} 最近心跳</h3>
      <table>
        <thead><tr><th>时间</th><th>UID</th><th>IP</th><th>版本</th><th>延迟</th></tr></thead>
        <tbody>
          <tr v-for="h in heartbeats" :key="h.createdAt + h.nodeUid">
            <td>{{ new Date(h.createdAt).toLocaleString() }}</td>
            <td>{{ h.nodeUid }}</td>
            <td>{{ h.nodeIp }}</td>
            <td>{{ h.version }}</td>
            <td>{{ h.latencyMs }} ms</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'

const nodes = ref([])
const form = ref({ name: '', region: '' })
const editingId = ref(null)
const editForm = ref({ name: '', region: '', version: '' })
const heartbeatNodeId = ref(null)
const heartbeats = ref([])

const load = async () => { nodes.value = await api.nodes() }

const createNode = async () => {
  await api.addNode(form.value)
  form.value = { name: '', region: '' }
  await load()
}

const toggle = async (id) => { await api.toggleNode(id); await load() }

const startEdit = (n) => {
  editingId.value = n.id
  editForm.value = { name: n.name, region: n.region, version: n.version }
}
const cancelEdit = () => { editingId.value = null }
const saveEdit = async (id) => {
  await api.updateNode(id, editForm.value)
  editingId.value = null
  await load()
}

const removeNode = async (id) => {
  if (!confirm(`确认删除节点 #${id} 吗？`)) return
  await api.deleteNode(id)
  if (heartbeatNodeId.value === id) { heartbeatNodeId.value = null; heartbeats.value = [] }
  await load()
}

const showHeartbeats = async (id) => {
  heartbeatNodeId.value = id
  heartbeats.value = await api.nodeHeartbeats(id)
}

onMounted(load)
</script>
