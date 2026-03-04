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
          <td>{{ r.id }}</td><td>{{ r.name }}</td><td>{{ r.listenAddr }}</td><td>{{ r.targetAddr }}</td><td>{{ r.protocol }}</td><td>{{ r.nodeId }}</td>
          <td><span :class="['badge', r.status==='enabled'?'online':'offline']">{{ r.status }}</span></td>
          <td>{{ r.connections }}</td>
          <td><button @click="toggle(r.id)">{{ r.status==='enabled' ? '停用' : '启用' }}</button></td>
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
const load = async () => rows.value = await api.forwards()
const create = async () => { await api.addForward(form.value); await load() }
const toggle = async (id) => { await api.toggleForward(id); await load() }
onMounted(load)
</script>
