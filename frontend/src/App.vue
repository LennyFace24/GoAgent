<script setup lang="ts">
import { ref, onMounted } from 'vue'
import Sidebar from './features/conversation/components/Sidebar.vue'
import ChatView from './features/chat/components/ChatView.vue'
import ToolsPanel from './features/tools/components/ToolsPanel.vue'

const activeMode = ref<string>('chat_stream')
const activeConversationId = ref<string>('default')

onMounted(async () => {
  // 加载对话列表，设置默认 activeId
  try {
    const res = await fetch('/conversations')
    if (res.ok) {
      const data = await res.json()
      const convs = data.conversations || []
      if (convs.length > 0) {
        activeConversationId.value = convs[0].id
      }
    }
  } catch { /* ignore */ }
})

function onSelect(id: string): void {
  activeConversationId.value = id
}

function onCreated(id: string): void {
  activeConversationId.value = id
}
</script>

<template>
  <div class="app-layout">
    <Sidebar
      :activeId="activeConversationId"
      @select="onSelect"
      @created="onCreated"
    />
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
