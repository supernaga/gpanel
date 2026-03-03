<template>
  <div class="layout">
    <aside class="sidebar">
      <h2>GOST 管理面板</h2>
      <button :class="{active:tab==='dashboard'}" @click="tab='dashboard'">仪表盘</button>
      <button :class="{active:tab==='nodes'}" @click="tab='nodes'">节点管理</button>
      <button disabled>客户端管理</button>
      <button disabled>端口转发</button>
      <button disabled>规则管理</button>
      <button disabled>告警通知</button>
    </aside>

    <main class="content">
      <DashboardView v-if="tab==='dashboard'" />
      <NodesView v-else />
    </main>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import DashboardView from './views/DashboardView.vue'
import NodesView from './views/NodesView.vue'

const tab = ref('dashboard')
</script>
