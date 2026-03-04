<template>
  <div class="layout">
    <aside class="sidebar">
      <h2>GOST 管理面板</h2>
      <button :class="{active:tab==='dashboard'}" @click="tab='dashboard'">仪表盘</button>
      <button :class="{active:tab==='nodes'}" @click="tab='nodes'">节点管理</button>
      <button :class="{active:tab==='clients'}" @click="tab='clients'">客户端管理</button>
      <button :class="{active:tab==='forwards'}" @click="tab='forwards'">端口转发</button>
      <button :class="{active:tab==='rules'}" @click="tab='rules'">规则管理</button>
      <button :class="{active:tab==='alerts'}" @click="tab='alerts'">告警通知</button>
    </aside>

    <main class="content">
      <DashboardView v-if="tab==='dashboard'" />
      <NodesView v-else-if="tab==='nodes'" />
      <ClientsView v-else-if="tab==='clients'" />
      <ForwardsView v-else-if="tab==='forwards'" />
      <RulesView v-else-if="tab==='rules'" />
      <AlertsView v-else />
    </main>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import DashboardView from './views/DashboardView.vue'
import NodesView from './views/NodesView.vue'
import ClientsView from './views/ClientsView.vue'
import ForwardsView from './views/ForwardsView.vue'
import RulesView from './views/RulesView.vue'
import AlertsView from './views/AlertsView.vue'

const tab = ref('dashboard')
</script>
