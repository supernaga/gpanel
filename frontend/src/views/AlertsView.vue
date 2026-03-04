<template>
  <section>
    <div class="toolbar"><h1>告警通知</h1><button @click="load">刷新</button></div>
    <table>
      <thead><tr><th>ID</th><th>级别</th><th>来源</th><th>消息</th><th>状态</th><th>时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="a in rows" :key="a.id">
          <td>{{ a.id }}</td><td>{{ a.level }}</td><td>{{ a.source }}</td><td>{{ a.message }}</td>
          <td><span :class="['badge', a.read ? 'offline' : 'online']">{{ a.read ? 'read' : 'unread' }}</span></td>
          <td>{{ new Date(a.createdAt).toLocaleString() }}</td>
          <td><button :disabled="a.read" @click="markRead(a.id)">标记已读</button></td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
const rows = ref([])
const load = async () => rows.value = await api.alerts()
const markRead = async (id) => { await api.readAlert(id); await load() }
onMounted(load)
</script>
