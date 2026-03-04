<template>
  <section>
    <div class="toolbar"><h1>用户与审计</h1><button @click="load">刷新</button></div>

    <h3>用户管理</h3>
    <form class="inline-form" @submit.prevent="createUser" style="margin-bottom:10px">
      <input v-model="userForm.username" placeholder="用户名" required />
      <input v-model="userForm.password" type="password" placeholder="密码" required />
      <select v-model="userForm.role"><option>admin</option><option>viewer</option></select>
      <button type="submit">新增用户</button>
    </form>
    <table>
      <thead><tr><th>ID</th><th>用户名</th><th>角色</th><th>创建时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="u in users" :key="u.id">
          <td>{{ u.id }}</td><td>{{ u.username }}</td><td>{{ u.role }}</td><td>{{ new Date(u.createdAt).toLocaleString() }}</td>
          <td><button @click="toggleRole(u)">切换角色</button></td>
        </tr>
      </tbody>
    </table>

    <h3 style="margin-top:18px">审计日志</h3>
    <table>
      <thead><tr><th>ID</th><th>用户ID</th><th>动作</th><th>目标</th><th>详情</th><th>时间</th></tr></thead>
      <tbody>
        <tr v-for="l in logs" :key="l.id">
          <td>{{ l.id }}</td><td>{{ l.userId }}</td><td>{{ l.action }}</td><td>{{ l.target }}</td><td>{{ l.detail }}</td><td>{{ new Date(l.createdAt).toLocaleString() }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
const users = ref([])
const logs = ref([])
const userForm = ref({ username: '', password: '', role: 'viewer' })
const load = async () => { users.value = await api.users(); logs.value = await api.auditLogs() }
const createUser = async () => { await api.addUser(userForm.value); userForm.value = { username: '', password: '', role: 'viewer' }; await load() }
const toggleRole = async (u) => { await api.updateUser(u.id, { role: u.role === 'admin' ? 'viewer' : 'admin' }); await load() }
onMounted(load)
</script>
