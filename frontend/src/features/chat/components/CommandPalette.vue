<script setup lang="ts">
import { CornerDownLeft, ArrowUp, ArrowDown } from 'lucide-vue-next'
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
      <span class="palette-hint">
        <span class="hint-group"><ArrowUp :size="10" /><ArrowDown :size="10" /></span>
        <span>选择</span>
        <span class="hint-key"><CornerDownLeft :size="11" /></span>
        <span>确认</span>
      </span>
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
  margin-bottom: 10px;
  background: var(--bg-elevated);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-lg);
  overflow: hidden;
  z-index: 100;
  animation: palette-enter 0.28s var(--ease-out-expo) both;
}

@keyframes palette-enter {
  from {
    opacity: 0;
    transform: translateY(8px) scale(0.98);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.palette-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border-hairline);
  background: var(--bg-secondary);
}

.palette-title {
  font-family: var(--font-display);
  font-size: 0.68rem;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.16em;
}

.palette-hint {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 0.64rem;
  color: var(--text-muted);
}

.hint-group {
  display: inline-flex;
  align-items: center;
}

.hint-key {
  display: inline-flex;
  color: var(--accent-bright);
}

.palette-list {
  max-height: 260px;
  overflow-y: auto;
  padding: 6px;
}

.palette-item {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 9px 12px;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: background var(--transition-fast), transform var(--transition-spring);
  border: 1px solid transparent;
}

.palette-item:hover,
.palette-item.active {
  background: var(--accent-soft);
  border-color: var(--border-focus);
}

.palette-item.active {
  transform: translateX(2px);
}

.item-main {
  display: flex;
  align-items: center;
  gap: 8px;
}

.item-name {
  font-size: 0.84rem;
  font-weight: 600;
  color: var(--accent-bright);
  font-family: var(--font-mono);
  letter-spacing: -0.01em;
}

.item-badge {
  font-size: 0.58rem;
  padding: 1px 7px;
  border-radius: var(--radius-pill);
  background: var(--bg-active);
  color: var(--text-muted);
  border: 1px solid var(--border-hairline);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 600;
}

.item-desc {
  font-size: 0.72rem;
  color: var(--text-muted);
  line-height: 1.4;
}
</style>
