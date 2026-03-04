<template>
  <section>
    <div class="toolbar"><h1>Agent 任务</h1><button @click="load">刷新</button></div>
    <form class="inline-form" @submit.prevent="create" style="margin-bottom:10px">
      <input v-model="form.nodeUid" placeholder="nodeUid(可选)" />
      <input v-model="form.nodeName" placeholder="nodeName(可选)" />
      <input v-model="form.command" placeholder="command" required />
      <input v-model="form.payload" placeholder="payload(JSON)" />
      <input v-model.number="form.maxRetries" type="number" min="1" placeholder="重试次数" />
      <input v-model.number="form.timeoutSecs" type="number" min="10" placeholder="超时秒" />
      <input v-model.number="form.priority" type="number" min="1" max="100" placeholder="优先级(1-100)" />
      <button type="submit">下发任务</button>
    </form>

    <table>
      <thead><tr><th>ID</th><th>节点UID</th><th>节点名</th><th>命令</th><th>状态</th><th>优先级</th><th>重试</th><th>超时</th><th>结果</th><th>创建时间</th></tr></thead>
      <tbody>
        <tr v-for="t in tasks" :key="t.id">
          <td>{{ t.id }}</td><td>{{ t.nodeUid }}</td><td>{{ t.nodeName }}</td><td>{{ t.command }}</td><td>{{ t.status }}</td><td>{{ t.priority }}</td><td>{{ t.retryCount }}/{{ t.maxRetries }}</td><td>{{ t.timeoutSecs }}s</td><td>{{ t.result }}</td><td>{{ new Date(t.createdAt).toLocaleString() }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
const tasks = ref([])
const form = ref({ nodeUid: '', nodeName: '', command: '', payload: '{}', maxRetries: 3, timeoutSecs: 300, priority: 50 })
const load = async () => tasks.value = await api.agentTasks()
const create = async () => { await api.addAgentTask(form.value); form.value = { nodeUid: '', nodeName: '', command: '', payload: '{}', maxRetries: 3, timeoutSecs: 300, priority: 50 }; await load() }
onMounted(load)
</script>
