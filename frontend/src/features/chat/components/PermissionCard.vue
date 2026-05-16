<script setup lang="ts">
import type { PermissionMessage } from '../../../shared/types'

const props = defineProps<{ message: PermissionMessage }>()
const emit = defineEmits<{
  respond: [approved: boolean, always: boolean]
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
  <div class="permission-card">
    <div class="perm-header">
      <span class="perm-icon">!</span>
      <span class="perm-title">权限请求: {{ message.name }}</span>
    </div>
    <div class="perm-reason">{{ message.reason }}</div>
    <div class="perm-args" v-if="message.args">{{ summarizeArgs(message.args) }}</div>
    <div v-if="!message.responded" class="perm-actions">
      <button class="perm-btn deny" @click="emit('respond', false, false)">拒绝</button>
      <button class="perm-btn allow" @click="emit('respond', true, false)">允许</button>
      <button class="perm-btn always" @click="emit('respond', true, true)">始终允许</button>
    </div>
    <div v-else class="perm-result">
      {{ message.approved ? '已允许' : '已拒绝' }}
    </div>
  </div>
</template>

<style scoped>
.permission-card {
  border-radius: 10px; border: 1px solid var(--accent);
  padding: 14px 16px; background: var(--accent-dim);
  max-width: 480px;
}
.perm-header {
  display: flex; align-items: center; gap: 8px; margin-bottom: 8px;
}
.perm-icon {
  width: 20px; height: 20px; border-radius: 50%;
  background: var(--accent); color: #fff;
  display: flex; align-items: center; justify-content: center;
  font-size: 0.7rem; font-weight: 700;
}
.perm-title {
  font-size: 0.82rem; font-weight: 600; color: var(--text-primary);
}
.perm-reason {
  font-size: 0.78rem; color: var(--text-secondary); margin-bottom: 6px;
}
.perm-args {
  font-size: 0.72rem; color: var(--text-muted);
  font-family: 'SF Mono', 'Consolas', monospace;
  padding: 6px 8px; background: var(--bg-hover); border-radius: 6px;
  margin-bottom: 10px; word-break: break-all;
}
.perm-actions {
  display: flex; gap: 8px;
}
.perm-btn {
  padding: 6px 14px; border-radius: 6px; border: 1px solid var(--border-input);
  font-size: 0.78rem; cursor: pointer; transition: all var(--transition-fast);
}
.perm-btn.deny {
  background: none; color: var(--text-secondary);
}
.perm-btn.deny:hover { border-color: var(--text-error); color: var(--text-error); }
.perm-btn.allow {
  background: var(--accent); color: #fff; border-color: var(--accent);
}
.perm-btn.allow:hover { opacity: 0.85; }
.perm-btn.always {
  background: none; color: var(--accent); border-color: var(--accent);
}
.perm-btn.always:hover { background: var(--accent-dim); }
.perm-result {
  font-size: 0.78rem; color: var(--text-muted); font-style: italic;
}
</style>
