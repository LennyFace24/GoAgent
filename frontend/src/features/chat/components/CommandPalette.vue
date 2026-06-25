<script setup lang="ts">
import type { Command } from '../composables/useCommands'

// ---------- 命令选择面板 ----------

defineProps<{
  commands: Command[]
  selectedIndex: number
  visible: boolean
}>()

const emit = defineEmits<{
  select: [cmd: Command]
  'update:selectedIndex': [index: number]
}>()

const categoryLabels: Record<string, string> = {
  general: '通用',
  conversation: '对话',
  context: '上下文',
}

function getCategoryLabel(category: string): string {
  return categoryLabels[category] || category
}
</script>

<template>
  <div class="command-palette" v-if="visible && commands.length > 0">
    <div class="palette-header">
      <span class="palette-title">斜杠命令</span>
      <span class="palette-hint">↑↓ 选择 · Enter 确认</span>
    </div>
    <div class="palette-list">
      <div
        v-for="(cmd, index) in commands"
        :key="cmd.name"
        :class="['palette-item', { active: index === selectedIndex }]"
        @click="emit('select', cmd)"
        @mouseenter="$emit('update:selectedIndex', index)"
      >
        <div class="item-main">
          <span class="item-name">/{{ cmd.name }}</span>
          <span class="item-badge">{{ getCategoryLabel(cmd.category) }}</span>
        </div>
        <span class="item-desc">{{ cmd.description }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.command-palette {
  position: absolute;
  bottom: 100%;
  left: 0;
  width: 100%;
  margin-bottom: 8px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  box-shadow: var(--shadow-lg);
  overflow: hidden;
  z-index: 100;
}

.palette-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 14px;
  border-bottom: 1px solid var(--border-color);
}

.palette-title {
  font-size: 0.7rem;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.palette-hint {
  font-size: 0.65rem;
  color: var(--text-muted);
}

.palette-list {
  max-height: 240px;
  overflow-y: auto;
  padding: 4px;
}

.palette-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background var(--transition-fast);
}

.palette-item:hover,
.palette-item.active {
  background: var(--bg-hover);
}

.item-main {
  display: flex;
  align-items: center;
  gap: 8px;
}

.item-name {
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--accent);
  font-family: monospace;
}

.item-badge {
  font-size: 0.6rem;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--bg-hover);
  color: var(--text-muted);
  border: 1px solid var(--border-color);
}

.item-desc {
  font-size: 0.72rem;
  color: var(--text-muted);
  line-height: 1.4;
}
</style>
