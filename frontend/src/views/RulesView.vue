<template>
  <section>
    <div class="toolbar">
      <h1>规则管理</h1>
      <form @submit.prevent="create" class="inline-form">
        <input v-model="form.name" placeholder="规则名" required />
        <select v-model="form.action"><option>allow</option><option>deny</option><option>limit</option></select>
        <input v-model="form.expr" placeholder="匹配表达式" required />
        <input v-model.number="form.priority" type="number" min="0" placeholder="优先级" required />
        <button type="submit">新增</button>
      </form>
    </div>
    <table>
      <thead><tr><th>ID</th><th>名称</th><th>动作</th><th>表达式</th><th>优先级</th><th>状态</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="r in rows" :key="r.id">
          <td>{{ r.id }}</td><td>{{ r.name }}</td><td>{{ r.action }}</td><td>{{ r.expr }}</td><td>{{ r.priority }}</td>
          <td><span :class="['badge', r.enabled ? 'online' : 'offline']">{{ r.enabled ? 'enabled' : 'disabled' }}</span></td>
          <td><button @click="toggle(r.id)">{{ r.enabled ? '禁用' : '启用' }}</button></td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
const rows = ref([])
const form = ref({ name: '', action: 'allow', expr: '', priority: 10 })
const load = async () => rows.value = await api.rules()
const create = async () => { await api.addRule(form.value); form.value = { name: '', action: 'allow', expr: '', priority: 10 }; await load() }
const toggle = async (id) => { await api.toggleRule(id); await load() }
onMounted(load)
</script>
