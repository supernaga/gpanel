<template>
  <section>
    <div class="toolbar">
      <h1>客户端管理</h1>
      <form @submit.prevent="create" class="inline-form">
        <input v-model="form.name" placeholder="客户端名" required />
        <input v-model="form.protocol" placeholder="协议，如 socks5/http" required />
        <input v-model.number="form.nodeId" type="number" min="1" placeholder="节点ID" required />
        <button type="submit">新增</button>
      </form>
    </div>

    <table>
      <thead><tr><th>ID</th><th>名称</th><th>协议</th><th>节点</th><th>状态</th><th>流量(Rx/Tx)</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="c in rows" :key="c.id">
          <td>{{ c.id }}</td><td>{{ c.name }}</td><td>{{ c.protocol }}</td><td>{{ c.nodeId }}</td>
          <td><span :class="['badge', c.status]">{{ c.status }}</span></td>
          <td>{{ c.rxMb }} / {{ c.txMb }} MB</td>
          <td><button @click="toggle(c.id)">{{ c.status==='online' ? '禁用' : '启用' }}</button></td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
const rows = ref([])
const form = ref({ name: '', protocol: 'socks5', nodeId: 1 })
const load = async () => rows.value = await api.clients()
const create = async () => { await api.addClient(form.value); form.value = { name: '', protocol: 'socks5', nodeId: 1 }; await load() }
const toggle = async (id) => { await api.toggleClient(id); await load() }
onMounted(load)
</script>
