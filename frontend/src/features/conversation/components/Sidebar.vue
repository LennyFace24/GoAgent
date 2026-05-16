<script setup lang="ts">
import { onMounted } from 'vue'
import { useConversations } from '../composables/useConversations'
import ConversationItem from './ConversationItem.vue'
import type { Conversation } from '../types'

const props = defineProps<{
  activeId: string
}>()

const emit = defineEmits<{
  select: [id: string]
  created: [id: string]
}>()

const { conversations, load, create, remove } = useConversations()

onMounted(() => { load() })

async function handleCreate(): Promise<void> {
  const newId = await create()
  if (newId) {
    emit('created', newId)
  }
}

function handleSelect(id: string): void {
  emit('select', id)
}

async function handleDelete(id: string): Promise<void> {
  await remove(id)
  // 如果删除的是当前活跃的，emit 一个新的 activeId
  if (props.activeId === id && conversations.value.length > 0) {
    emit('select', conversations.value[0].id)
  }
}
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-inner">
      <div class="sidebar-brand">goagent</div>

      <button class="new-chat-btn" @click="handleCreate">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        新对话
      </button>

      <div class="conv-list">
        <ConversationItem
          v-for="conv in conversations"
          :key="conv.id"
          :conversation="conv"
          :active="activeId === conv.id"
          @select="handleSelect(conv.id)"
          @delete="handleDelete(conv.id)"
        />
      </div>

      <div class="sidebar-footer">v0.1</div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: var(--sidebar-left-w);
  flex-shrink: 0;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border-color);
  display: flex; flex-direction: column;
  overflow: hidden;
}
.sidebar-inner {
  flex: 1; display: flex; flex-direction: column;
  padding: 20px 16px; overflow-y: auto;
}
.sidebar-brand {
  font-size: 0.82rem; font-weight: 500; color: var(--text-muted);
  margin-bottom: 24px; white-space: nowrap;
  letter-spacing: 0.08em; text-transform: lowercase;
}

.new-chat-btn {
  width: 100%; background: none; border: 1px solid var(--border-input);
  color: var(--text-secondary); padding: 8px 14px; border-radius: 8px;
  font-size: 0.8rem; cursor: pointer; margin-bottom: 12px;
  transition: all var(--transition-fast);
  display: flex; align-items: center; justify-content: center; gap: 6px;
}
.new-chat-btn:hover {
  border-color: var(--border-focus); color: var(--text-primary);
  background: var(--bg-hover);
}

.conv-list {
  flex: 1; overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: rgba(128,128,160,0.15) transparent;
}

.sidebar-footer {
  margin-top: auto; font-size: 0.68rem; color: var(--text-muted);
  text-align: center; padding-top: 16px; white-space: nowrap;
}
</style>
