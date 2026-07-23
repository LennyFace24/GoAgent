<script setup lang="ts">
import { ShieldAlert, Check, X } from 'lucide-vue-next'
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
      <span class="perm-icon"><ShieldAlert :size="16" :stroke-width="2" /></span>
      <span class="perm-title">权限请求 · {{ message.name }}</span>
    </div>
    <div class="perm-reason" v-if="message.reason">{{ message.reason }}</div>
    <div class="perm-args" v-if="message.args">{{ summarizeArgs(message.args) }}</div>
    <div v-if="!message.responded" class="perm-actions">
      <button class="perm-btn deny" @click="emit('respond', false, false)">
        <X :size="13" :stroke-width="2.2" />
        <span>拒绝</span>
      </button>
      <button class="perm-btn allow" @click="emit('respond', true, false)">
        <Check :size="13" :stroke-width="2.4" />
        <span>允许</span>
      </button>
      <button class="perm-btn always" @click="emit('respond', true, true)">
        <Check :size="13" :stroke-width="2.4" />
        <span>始终允许</span>
      </button>
    </div>
    <div v-else class="perm-result" :class="{ approved: message.approved }">
      <component :is="message.approved ? Check : X" :size="13" :stroke-width="2.4" />
      <span>{{ message.approved ? '已允许' : '已拒绝' }}</span>
    </div>
  </div>
</template>

<style scoped>
.permission-card {
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-focus);
  padding: 15px 17px;
  background: var(--accent-soft);
  backdrop-filter: var(--glass-blur-lite);
  -webkit-backdrop-filter: var(--glass-blur-lite);
  max-width: 480px;
  box-shadow: 0 0 24px rgba(0, 255, 157, 0.08), var(--shadow-sm);
}

.perm-header {
  display: flex;
  align-items: center;
  gap: 9px;
  margin-bottom: 10px;
}

.perm-icon {
  display: flex;
  width: 30px;
  height: 30px;
  border-radius: var(--radius-md);
  background: var(--accent-gradient);
  color: #07070b;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: 0 0 12px var(--accent-glow);
}

.perm-title {
  font-size: 0.84rem;
  font-weight: 600;
  color: var(--text-heading);
  font-family: var(--font-display);
  letter-spacing: -0.01em;
}

.perm-reason {
  font-size: 0.78rem;
  color: var(--text-secondary);
  margin-bottom: 8px;
  line-height: 1.55;
}

.perm-args {
  font-size: 0.72rem;
  color: var(--text-muted);
  font-family: var(--font-mono);
  padding: 7px 10px;
  background: rgba(7, 7, 11, 0.4);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  margin-bottom: 12px;
  word-break: break-all;
}

.perm-actions {
  display: flex;
  gap: 8px;
}

.perm-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--border-input);
  font-size: 0.76rem;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-spring);
  background: var(--bg-elevated);
  color: var(--text-secondary);
}

.perm-btn.deny:hover {
  border-color: var(--text-error);
  color: var(--text-error);
  transform: translateY(-1px);
}

.perm-btn.allow {
  background: var(--accent-gradient);
  color: #07070b;
  border-color: transparent;
  box-shadow: 0 2px 12px var(--accent-glow);
}

.perm-btn.allow:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 18px var(--accent-glow);
}

.perm-btn.always {
  color: var(--accent-bright);
  border-color: var(--border-focus);
}

.perm-btn.always:hover {
  background: var(--accent-soft);
  transform: translateY(-1px);
}

.perm-result {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.76rem;
  color: var(--text-muted);
  font-style: italic;
}

.perm-result.approved {
  color: var(--accent-bright);
}
</style>
