<script setup lang="ts">
import type { Conversation } from '../types'

defineProps<{
  conversation: Conversation
  active: boolean
}>()

const emit = defineEmits<{
  select: []
  delete: []
}>()
</script>

<template>
  <div :class="['conv-item', { active }]" @click="emit('select')">
    <span class="conv-title">{{ conversation.title || '新对话' }}</span>
    <button class="conv-del" @click.stop="emit('delete')" title="删除">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
        stroke-linecap="round" stroke-linejoin="round">
        <line x1="18" y1="6" x2="6" y2="18" />
        <line x1="6" y1="6" x2="18" y2="18" />
      </svg>
    </button>
  </div>
</template>

<style scoped>
.conv-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.8rem;
  color: var(--text-secondary);
  transition: all var(--transition-fast);
  margin-bottom: 1px;
  border-left: 2px solid transparent;
}

.conv-item:hover {
  background: var(--bg-hover);
}

.conv-item.active {
  background: var(--bg-active);
  color: var(--text-primary);
  border-left-color: var(--accent);
}

.conv-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conv-del {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 2px;
  line-height: 1;
  opacity: 0;
  border-radius: 4px;
  transition: opacity var(--transition-fast), color var(--transition-fast);
  display: flex;
  align-items: center;
  justify-content: center;
}

.conv-item:hover .conv-del {
  opacity: 1;
}

.conv-del:hover {
  color: var(--text-error);
}
</style>
