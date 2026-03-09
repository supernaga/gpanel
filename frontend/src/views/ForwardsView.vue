<template>
  <section>
    <div class="toolbar">
      <h1>端口转发</h1>
      <form @submit.prevent="create" class="inline-form">
        <input v-model="form.name" placeholder="规则名" required />
        <input v-model="form.listenAddr" placeholder="监听地址，如 :8081" required />
        <input v-model="form.targetAddr" placeholder="目标地址，如 10.0.0.5:80" required />
        <input v-model="form.protocol" placeholder="协议 tcp/udp" required />
        <input v-model.number="form.nodeId" type="number" min="1" placeholder="节点ID" required />
        <button type="submit">新增</button>
      </form>
    </div>
    <table>
      <thead><tr><th>ID</th><th>名称</th><th>监听</th><th>目标</th><th>协议</th><th>节点</th><th>状态</th><th>连接数</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="r in rows" :key="r.id">
          <td>{{ r.id }}</td>
          <td><template v-if="editingId===r.id"><input v-model="editForm.name" /></template><template v-else>{{ r.name }}</template></td>
          <td><template v-if="editingId===r.id"><input v-model="editForm.listenAddr" /></template><template v-else>{{ r.listenAddr }}</template></td>
          <td><template v-if="editingId===r.id"><input v-model="editForm.targetAddr" /></template><template v-else>{{ r.targetAddr }}</template></td>
          <td><template v-if="editingId===r.id"><input v-model="editForm.protocol" /></template><template v-else>{{ r.protocol }}</template></td>
          <td><template v-if="editingId===r.id"><input v-model.number="editForm.nodeId" type="number" min="1" /></template><template v-else>{{ r.nodeId }}</template></td>
          <td><span :class="['badge', r.status==='enabled'?'online':'offline']">{{ r.status }}</span></td>
          <td>{{ r.connections }}</td>
          <td>
            <button @click="toggle(r.id)">{{ r.status==='enabled' ? '停用' : '启用' }}</button>
            <button v-if="editingId !== r.id" @click="startEdit(r)">编辑</button>
            <button v-else @click="saveEdit(r.id)">保存</button>
            <button v-if="editingId === r.id" @click="cancelEdit">取消</button>
            <button @click="removeForward(r.id)">删除</button>
          </td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
const rows = ref([])
const form = ref({ name: '', listenAddr: ':9000', targetAddr: '127.0.0.1:80', protocol: 'tcp', nodeId: 1 })
const editingId = ref(null)
const editForm = ref({ name: '', listenAddr: '', targetAddr: '', protocol: 'tcp', nodeId: 1 })
const load = async () => rows.value = await api.forwards()
const create = async () => { await api.addForward(form.value); await load() }
const toggle = async (id) => { await api.toggleForward(id); await load() }
const startEdit = (r) => {
  editingId.value = r.id
  editForm.value = { name: r.name, listenAddr: r.listenAddr, targetAddr: r.targetAddr, protocol: r.protocol, nodeId: r.nodeId }
}
const cancelEdit = () => { editingId.value = null }
const saveEdit = async (id) => { await api.updateForward(id, editForm.value); editingId.value = null; await load() }
const removeForward = async (id) => {
  if (!confirm(`确认删除转发 #${id} 吗？`)) return
  await api.deleteForward(id)
  await load()
}
onMounted(load)
</script>
