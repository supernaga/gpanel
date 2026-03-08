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
      <thead><tr><th>名称</th><th>路径</th><th>协议</th><th>状态</th><th>说明</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="c in chains" :key="c.id">
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
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'

const chains = ref([])
const form = ref({ name: '', path: '', protocol: 'tcp' })
const editingId = ref(null)
const editForm = ref({ name: '', path: '', protocol: 'tcp' })

const load = async () => { chains.value = await api.chains() }
const createChain = async () => {
  await api.addChain(form.value)
  form.value = { name: '', path: '', protocol: 'tcp' }
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
