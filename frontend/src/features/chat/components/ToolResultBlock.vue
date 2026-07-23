<script setup lang="ts">
import { ref } from 'vue'
import { ChevronRight, Terminal } from 'lucide-vue-next'

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
      <span class="tool-chevron" :class="{ open: !collapsed }">
        <ChevronRight :size="13" :stroke-width="2.4" />
      </span>
      <span class="tool-icon"><Terminal :size="12" :stroke-width="2" /></span>
      <span class="tool-name">{{ name }}</span>
      <span class="tool-summary" v-if="collapsed">{{ summarizeResult(result) }}</span>
      <span class="tool-status" v-if="collapsed">{{ result ? (result.length + ' chars') : '' }}</span>
    </div>
    <pre v-show="!collapsed" class="tool-result-body">{{ result }}</pre>
  </div>
</template>

<style scoped>
.tool-result {
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-glass);
  overflow: hidden;
  cursor: pointer;
  transition: border-color var(--transition-fast), transform var(--transition-spring);
  background: var(--bg-elevated);
  backdrop-filter: var(--glass-blur-lite);
  -webkit-backdrop-filter: var(--glass-blur-lite);
}

.tool-result:hover {
  border-color: var(--border-focus);
  transform: translateY(-1px);
}

.tool-result-header {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 9px 13px;
  background: var(--bg-secondary);
  font-size: 0.76rem;
}

.tool-chevron {
  display: flex;
  color: var(--text-muted);
  transition: transform var(--transition-spring), color var(--transition-fast);
}

.tool-chevron.open {
  transform: rotate(90deg);
  color: var(--accent-bright);
}

.tool-icon {
  display: flex;
  color: var(--accent-bright);
  flex-shrink: 0;
}

.tool-name {
  font-weight: 600;
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 0.74rem;
  letter-spacing: -0.01em;
}

.tool-summary {
  color: var(--text-muted);
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.72rem;
  font-family: var(--font-mono);
  border-left: 1px solid var(--border-hairline);
  padding-left: 9px;
}

.tool-status {
  color: var(--text-muted);
  font-size: 0.66rem;
  font-family: var(--font-mono);
  margin-left: auto;
  background: var(--bg-active);
  padding: 2px 7px;
  border-radius: var(--radius-pill);
}

.tool-result-body {
  margin: 0;
  padding: 13px 15px;
  font-size: 0.76rem;
  line-height: 1.6;
  font-family: var(--font-mono);
  color: var(--text-secondary);
  background: rgba(7, 7, 11, 0.5);
  max-height: 400px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
  border-top: 1px solid var(--border-hairline);
}
</style>
