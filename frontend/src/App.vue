<script setup lang="ts">
import { ref } from 'vue'
import SlimSidebar from './features/conversation/components/SlimSidebar.vue'
import SubSidebar from './features/conversation/components/SubSidebar.vue'
import ChatWorkspace from './features/chat/components/ChatWorkspace.vue'
import MonitorDrawer from './features/tools/components/MonitorDrawer.vue'

// 1. Cherry Studio 多层布局状态管理
const activeNav = ref<string>('chat')
const activeConversationId = ref<string>('default')
const isMonitorOpen = ref<boolean>(false)

function onSelect(id: string): void {
  activeConversationId.value = id
}

function onCreated(id: string): void {
  activeConversationId.value = id
}
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
      @select="onSelect"
      @created="onCreated"
    />
    
    <!-- 第三栏：智能问答主视窗控制台 (Workspace) -->
    <ChatWorkspace
      :mode="activeNav"
      :conversationId="activeConversationId"
      v-model:isMonitorOpen="isMonitorOpen"
    />
    
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