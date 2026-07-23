<script setup lang="ts">
import { ref } from 'vue'
import { Brain, ChevronRight } from 'lucide-vue-next'
import { useMarkdown } from '../../../shared/composables/useMarkdown'

defineProps<{
  content: string
}>()

const { render } = useMarkdown()
const collapsed = ref(false)
</script>

<template>
  <div class="thinking-block" :class="{ collapsed }">
    <div class="thinking-header" @click="collapsed = !collapsed">
      <span class="thinking-icon"><Brain :size="14" :stroke-width="1.8" /></span>
      <span class="thinking-label">AI 思考过程</span>
      <span class="thinking-toggle">
        <ChevronRight :size="14" :stroke-width="2.2" />
      </span>
    </div>
    <div v-if="!collapsed" class="thinking-content md" v-html="render(content)"></div>
  </div>
</template>

<style scoped>
.thinking-block {
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-lg);
  background: var(--bg-secondary);
  backdrop-filter: var(--glass-blur-lite);
  -webkit-backdrop-filter: var(--glass-blur-lite);
  overflow: hidden;
  margin: 2px 0;
  box-shadow: var(--shadow-sm);
}

.thinking-header {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 9px 13px;
  cursor: pointer;
  user-select: none;
  transition: background var(--transition-fast);
}

.thinking-header:hover {
  background: var(--bg-hover);
}

.thinking-icon {
  display: flex;
  color: var(--signal-violet);
  flex-shrink: 0;
}

.thinking-label {
  font-size: 0.76rem;
  font-weight: 500;
  color: var(--text-secondary);
  flex: 1;
  letter-spacing: 0.01em;
}

.thinking-toggle {
  display: flex;
  color: var(--text-muted);
  transition: transform var(--transition-spring);
}

.thinking-block:not(.collapsed) .thinking-toggle {
  transform: rotate(90deg);
}

.thinking-content {
  padding: 11px 14px;
  border-top: 1px solid var(--border-hairline);
  font-size: 0.8rem;
  line-height: 1.65;
  color: var(--text-secondary);
  white-space: normal;
}

.thinking-content.md :deep(p) {
  margin: 0.35em 0;
}
.thinking-content.md :deep(p:first-child) {
  margin-top: 0;
}
.thinking-content.md :deep(p:last-child) {
  margin-bottom: 0;
}

.thinking-content.md :deep(code) {
  background: var(--bg-hover);
  border-radius: var(--radius-xs);
  padding: 1px 5px;
  font-size: 0.82em;
  font-family: var(--font-mono);
}

.thinking-content.md :deep(pre) {
  background: rgba(7, 7, 11, 0.5);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-md);
  padding: 10px 12px;
  margin: 0.5em 0;
  overflow-x: auto;
}

.thinking-content.md :deep(pre code) {
  background: none;
  border: none;
  padding: 0;
}
</style>
