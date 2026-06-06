<script setup lang="ts">
import { ref } from 'vue'
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
      <span class="thinking-icon">💭</span>
      <span class="thinking-label">AI 思考过程</span>
      <span class="thinking-toggle">{{ collapsed ? '▶' : '▼' }}</span>
    </div>
    <div v-if="!collapsed" class="thinking-content md" v-html="render(content)"></div>
  </div>
</template>

<style scoped>
.thinking-block {
  border: 1px solid var(--border-color);
  border-radius: 10px;
  background: var(--bg-secondary);
  overflow: hidden;
  margin: 4px 0;
}

.thinking-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
}

.thinking-header:hover {
  background: var(--bg-hover);
}

.thinking-icon {
  font-size: 0.9rem;
}

.thinking-label {
  font-size: 0.78rem;
  font-weight: 500;
  color: var(--text-secondary);
  flex: 1;
}

.thinking-toggle {
  font-size: 0.7rem;
  color: var(--text-muted);
  transition: transform 0.2s;
}

.thinking-content {
  padding: 10px 14px;
  border-top: 1px solid var(--border-color);
  font-size: 0.82rem;
  line-height: 1.6;
  color: var(--text-secondary);
  white-space: normal;
}

.thinking-content.md :deep(p) {
  margin: 0.3em 0;
}

.thinking-content.md :deep(p:first-child) {
  margin-top: 0;
}

.thinking-content.md :deep(p:last-child) {
  margin-bottom: 0;
}

.thinking-content.md :deep(code) {
  background: var(--bg-hover);
  border-radius: 4px;
  padding: 1px 5px;
  font-size: 0.82em;
  font-family: 'SF Mono', 'Consolas', 'Fira Code', monospace;
}

.thinking-content.md :deep(pre) {
  background: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: 8px;
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
