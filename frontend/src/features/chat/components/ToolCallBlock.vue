<script setup lang="ts">
import { Terminal } from 'lucide-vue-next'

defineProps<{
  name: string
  args?: string
}>()

function summarizeArgs(args?: string): string {
  if (!args) return ''
  try {
    const obj: Record<string, unknown> = JSON.parse(args)
    const entries = Object.entries(obj)
    if (entries.length === 0) return ''
    const [, val] = entries[0]
    const str = typeof val === 'string' ? val : JSON.stringify(val)
    return str.length > 60 ? str.slice(0, 60) + '...' : str
  } catch {
    return args.length > 60 ? args.slice(0, 60) + '...' : args
  }
}
</script>

<template>
  <div class="tool-call">
    <span class="tool-icon"><Terminal :size="12" :stroke-width="2.2" /></span>
    <span class="tool-name">{{ name }}</span>
    <span class="tool-args">{{ summarizeArgs(args) }}</span>
  </div>
</template>

<style scoped>
.tool-call {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 7px 12px;
  border-radius: var(--radius-pill);
  background: var(--bg-elevated);
  border: 1px solid var(--border-glass);
  font-size: 0.76rem;
  color: var(--text-secondary);
  width: fit-content;
  max-width: 100%;
  backdrop-filter: var(--glass-blur-lite);
  -webkit-backdrop-filter: var(--glass-blur-lite);
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

.tool-args {
  color: var(--text-muted);
  font-size: 0.72rem;
  font-family: var(--font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
  border-left: 1px solid var(--border-hairline);
  padding-left: 9px;
}
</style>
