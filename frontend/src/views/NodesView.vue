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
          <td>{{ n.name }}</td>
          <td>{{ n.region }}</td>
          <td><span :class="['badge', n.status]">{{ n.status }}</span></td>
          <td>{{ n.latencyMs }} ms</td>
          <td>{{ n.version }}</td>
          <td><button @click="toggle(n.id)">{{ n.status === 'online' ? '下线' : '上线' }}</button></td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'

const nodes = ref([])
const form = ref({ name: '', region: '' })

const load = async () => {
  nodes.value = await api.nodes()
}

const createNode = async () => {
  await api.addNode(form.value)
  form.value = { name: '', region: '' }
  await load()
}

const toggle = async (id) => {
  await api.toggleNode(id)
  await load()
}

onMounted(load)
</script>
