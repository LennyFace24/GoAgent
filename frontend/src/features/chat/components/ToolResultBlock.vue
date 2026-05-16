<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{
  name: string
  result?: string
}>()

const collapsed = ref(true)

function summarizeResult(result?: string): string {
  if (!result) return ''
  const firstLine = result.split('\n')[0]
  return firstLine.length > 80 ? firstLine.slice(0, 80) + '...' : firstLine
}
</script>

<template>
  <div class="tool-result" @click="collapsed = !collapsed">
    <div class="tool-result-header">
      <span class="tool-chevron" :class="{ open: !collapsed }">&#9654;</span>
      <span class="tool-name">{{ name }}</span>
      <span class="tool-summary" v-if="collapsed">{{ summarizeResult(result) }}</span>
      <span class="tool-status" v-if="collapsed">{{ result ? (result.length + ' chars') : '' }}</span>
    </div>
    <pre v-show="!collapsed" class="tool-result-body">{{ result }}</pre>
  </div>
</template>

<style scoped>
.tool-result {
  border-radius: 8px; border: 1px solid var(--border-color);
  overflow: hidden; cursor: pointer;
  transition: border-color var(--transition-fast);
}
.tool-result:hover { border-color: var(--border-focus); }

.tool-result-header {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 12px; background: var(--bg-hover);
  font-size: 0.78rem;
}
.tool-chevron {
  font-size: 0.6rem; color: var(--text-muted);
  transition: transform var(--transition-fast);
}
.tool-chevron.open { transform: rotate(90deg); }
.tool-name {
  font-weight: 500; color: var(--text-primary);
  font-family: 'SF Mono', 'Consolas', monospace; font-size: 0.78rem;
}
.tool-summary {
  color: var(--text-muted); flex: 1; min-width: 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  font-size: 0.75rem;
}
.tool-status {
  color: var(--text-muted); font-size: 0.7rem;
  margin-left: auto;
}

.tool-result-body {
  margin: 0; padding: 12px;
  font-size: 0.78rem; line-height: 1.5;
  font-family: 'SF Mono', 'Consolas', 'Fira Code', monospace;
  color: var(--text-secondary);
  background: var(--bg-input);
  max-height: 400px; overflow-y: auto;
  white-space: pre-wrap; word-break: break-all;
}
</style>
