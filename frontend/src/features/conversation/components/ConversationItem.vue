<script setup lang="ts">
import { X } from 'lucide-vue-next'
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
      <X :size="12" :stroke-width="2.4" />
    </button>
  </div>
</template>

<style scoped>
.conv-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 9px 12px;
  border-radius: var(--radius-md);
  cursor: pointer;
  font-size: 0.8rem;
  color: var(--text-secondary);
  transition: all var(--transition-spring);
  margin-bottom: 2px;
  border-left: 2px solid transparent;
  position: relative;
}

.conv-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
  transform: translateX(2px);
}

.conv-item.active {
  background: var(--accent-soft);
  color: var(--accent-bright);
  border-left-color: var(--accent-bright);
}

.conv-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: -0.005em;
}

.conv-del {
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  width: 22px;
  height: 22px;
  border-radius: var(--radius-xs);
  opacity: 0;
  transform: scale(0.8);
  transition: all var(--transition-spring);
}

.conv-item:hover .conv-del {
  opacity: 1;
  transform: scale(1);
}

.conv-del:hover {
  color: var(--text-error);
  background: rgba(255, 90, 110, 0.1);
}
</style>
