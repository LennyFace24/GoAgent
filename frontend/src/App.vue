<script setup lang="ts">
import { ref } from 'vue'
import Sidebar from './features/conversation/components/Sidebar.vue'
import ChatView from './features/chat/components/ChatView.vue'
import ToolsPanel from './features/tools/components/ToolsPanel.vue'

const activeMode = ref<string>('chat_stream')
const activeConversationId = ref<string>('default')

// Sidebar 暴露 activeId 后同步
function onSidebarReady(sidebar: InstanceType<typeof Sidebar>) {
  if (sidebar?.activeId) {
    activeConversationId.value = sidebar.activeId
  }
}
</script>

<template>
  <div class="app-layout">
    <Sidebar ref="sidebarRef" />
    <ChatView
      v-model:mode="activeMode"
      :conversationId="activeConversationId"
    />
    <ToolsPanel />
  </div>
</template>

<style>
@import './assets/styles/theme.css';
</style>

<style scoped>
.app-layout {
  display: flex; height: 100vh;
}
</style>
