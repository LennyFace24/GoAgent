<script setup lang="ts">
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
    <span class="tool-icon">&gt;_</span>
    <span class="tool-name">{{ name }}</span>
    <span class="tool-args">{{ summarizeArgs(args) }}</span>
  </div>
</template>

<style scoped>
.tool-call {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 12px; border-radius: 8px;
  background: var(--bg-hover); font-size: 0.8rem;
  color: var(--text-secondary);
}
.tool-icon {
  font-family: 'SF Mono', 'Consolas', monospace;
  font-size: 0.75rem; color: var(--accent); opacity: 0.7;
}
.tool-name {
  font-weight: 500; color: var(--text-primary);
  font-family: 'SF Mono', 'Consolas', monospace; font-size: 0.78rem;
}
.tool-args {
  color: var(--text-muted); font-size: 0.75rem;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  flex: 1; min-width: 0;
}
</style>
