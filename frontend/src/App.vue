<script setup lang="ts">
import { ref,onMounted } from 'vue'



import { useConversations } from './features/conversation/composables/useConversations'


import EmptyConversations from './features/chat/components/EmptyConversations.vue'
import SlimSidebar from './features/conversation/components/SlimSidebar.vue'
import SubSidebar from './features/conversation/components/SubSidebar.vue'
import ChatWorkspace from './features/chat/components/ChatWorkspace.vue'
import DashboardView from './features/tools/components/DashboardView.vue'
import MonitorDrawer from './features/tools/components/MonitorDrawer.vue'

// 1. Cherry Studio 多层布局状态管理
const activeNav = ref<string>('chat')
const activeConversationId = ref<string>('default')
const isMonitorOpen = ref<boolean>(false)
const dashboardView = ref<string>('overview')

function onSelect(id: string): void {
  activeConversationId.value = id
}

function onCreated(id: string): void {
  activeConversationId.value = id
}

function onDashboardViewChange(view: string): void {
  dashboardView.value = view
}

onMounted(async () => {
  const { load,create } = useConversations()
  const id = await create();
  if (id) {
    activeConversationId.value = id
  }
  await load()
})

</script>

<template>
  <div class="app-layout">
    <!-- 第一栏：极窄主功能菜单栏 (Mini Sidebar) -->
    <SlimSidebar 
      v-model:activeNav="activeNav"
    />
    
    <!-- 第二栏：对话或文档管理侧边栏 (Sub Sidebar) -->
    <SubSidebar
      :activeNav="activeNav"
      :activeId="activeConversationId"
      :dashboardView="dashboardView"
      @select="onSelect"
      @created="onCreated"
      @dashboard-view="onDashboardViewChange"
    />
    
    <!-- 第三栏：智能问答主视窗控制台 (Workspace) / 或者核心监控仪表盘 -->
    <keep-alive :include="['ChatWorkspace', 'DashboardView']">
      <ChatWorkspace
        v-if="activeNav !== 'dashboard' && activeConversationId !== ''" 
        :conversationId="activeConversationId"
        v-model:isMonitorOpen="isMonitorOpen"
      />
      <EmptyConversations
        v-else-if="activeNav !== 'dashboard' && activeConversationId === ''"
       />
      <DashboardView
        v-else
        :view="dashboardView"
      />
    </keep-alive>
    
    <!-- 第四栏：Prometheus 指标滑出动态监控抽屉 (Monitor Drawer) -->
    <MonitorDrawer
      :isOpen="isMonitorOpen"
      @close="isMonitorOpen = false"
    />
  </div>
</template>

<style>
@import './assets/styles/theme.css';
</style>

<style scoped>
.app-layout {
  display: flex; 
  height: 100vh;
  width: 100vw;
  overflow: hidden;
  background: var(--bg-body);
}
</style>