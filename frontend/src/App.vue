<template>
  <div v-if="!authed" class="login-wrap">
    <section class="login-card">
      <h2>GPanel 登录</h2>
      <form @submit.prevent="doLogin" class="inline-form" style="display:flex;flex-direction:column;gap:8px">
        <input v-model="login.username" placeholder="用户名" required />
        <input v-model="login.password" placeholder="密码" type="password" required />
        <button type="submit">登录</button>
      </form>
      <p class="hint">默认账号见 deploy/.env（ADMIN_USER / ADMIN_PASSWORD）</p>
      <p v-if="err" class="danger">{{ err }}</p>
    </section>
  </div>

  <div v-else class="layout">
    <aside class="sidebar">
      <h2>GPanel</h2>
      <button :class="{active:tab==='dashboard'}" @click="tab='dashboard'">仪表盘</button>
      <button :class="{active:tab==='nodes'}" @click="tab='nodes'">节点</button>
      <button :class="{active:tab==='forwards'}" @click="tab='forwards'">转发</button>
      <button :class="{active:tab==='tunnels'}" @click="tab='tunnels'">隧道</button>
      <button :class="{active:tab==='chains'}" @click="tab='chains'">链路</button>
      <button :class="{active:tab==='runtime'}" @click="tab='runtime'">运行态</button>
      <button :class="{active:tab==='tasks'}" @click="tab='tasks'">任务</button>
      <button :class="{active:tab==='alerts'}" @click="tab='alerts'">告警</button>
      <button v-if="isAdmin" :class="{active:tab==='users'}" @click="tab='users'">用户与审计</button>
      <button @click="logout">退出登录</button>
    </aside>

    <main class="content">
      <DashboardView v-if="tab==='dashboard'" />
      <NodesView v-else-if="tab==='nodes'" />
      <ForwardsView v-else-if="tab==='forwards'" />
      <TunnelsView v-else-if="tab==='tunnels'" />
      <ChainsView v-else-if="tab==='chains'" />
      <RuntimeView v-else-if="tab==='runtime'" />
      <AgentTasksView v-else-if="tab==='tasks'" />
      <AlertsView v-else-if="tab==='alerts'" />
      <UsersAuditView v-else />
    </main>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { api } from './api/client'
import DashboardView from './views/DashboardView.vue'
import NodesView from './views/NodesView.vue'
import ForwardsView from './views/ForwardsView.vue'
import TunnelsView from './views/TunnelsView.vue'
import ChainsView from './views/ChainsView.vue'
import RuntimeView from './views/RuntimeView.vue'
import AgentTasksView from './views/AgentTasksView.vue'
import AlertsView from './views/AlertsView.vue'
import UsersAuditView from './views/UsersAuditView.vue'

const tab = ref('dashboard')
const authed = ref(!!localStorage.getItem('gpanel_token'))
const err = ref('')
const login = ref({ username: 'admin', password: '' })
const isAdmin = computed(() => (localStorage.getItem('gpanel_role') || '') === 'admin')

const doLogin = async () => {
  err.value = ''
  try {
    const res = await api.login(login.value)
    localStorage.setItem('gpanel_token', res.token)
    localStorage.setItem('gpanel_role', res.role)
    authed.value = true
  } catch (e) {
    err.value = e.message || '登录失败'
  }
}

const logout = () => {
  localStorage.removeItem('gpanel_token')
  localStorage.removeItem('gpanel_role')
  authed.value = false
}
</script>
