<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { Gauge } from 'lucide-vue-next'

interface ContextUsage {
  current_tokens: number
  max_tokens: number
  percentage: number
  message_count: number
}

const usage = ref<ContextUsage>({
  current_tokens: 0,
  max_tokens: 200000,
  percentage: 0,
  message_count: 0
})

const status = ref<'normal' | 'warning' | 'critical'>('normal')
let interval: ReturnType<typeof setInterval> | null = null

async function fetchContextStatus() {
  try {
    const res = await fetch('/context')
    if (res.ok) {
      const data = await res.json()
      usage.value = data

      // 更新状态
      if (data.percentage > 90) {
        status.value = 'critical'
      } else if (data.percentage > 70) {
        status.value = 'warning'
      } else {
        status.value = 'normal'
      }
    }
  } catch (err) {
    // 静默失败
  }
}

function formatTokens(tokens: number): string {
  if (tokens >= 1000000) {
    return (tokens / 1000000).toFixed(1) + 'M'
  }
  if (tokens >= 1000) {
    return (tokens / 1000).toFixed(1) + 'K'
  }
  return tokens.toString()
}

const barColor = computed(() => {
  if (status.value === 'critical') return 'var(--text-error)'
  if (status.value === 'warning') return 'var(--text-warning)'
  return 'var(--accent-bright)'
})

onMounted(() => {
  fetchContextStatus()
  interval = setInterval(fetchContextStatus, 5000) // 每 5 秒刷新
})

onUnmounted(() => {
  if (interval) {
    clearInterval(interval)
  }
})
</script>

<template>
  <div class="context-status" :class="status">
    <span class="context-icon"><Gauge :size="13" :stroke-width="2" /></span>
    <div class="context-info">
      <div class="context-percentage">{{ usage.percentage.toFixed(1) }}<span class="pct">%</span></div>
      <div class="context-detail">
        {{ formatTokens(usage.current_tokens) }} / {{ formatTokens(usage.max_tokens) }}
      </div>
    </div>
    <div class="context-bar">
      <div
        class="context-bar-fill"
        :style="{ width: Math.min(usage.percentage, 100) + '%', background: barColor }"
      />
    </div>
  </div>
</template>

<style scoped>
.context-status {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 6px 12px;
  border-radius: var(--radius-pill);
  background: var(--bg-elevated);
  border: 1px solid var(--border-glass);
  backdrop-filter: var(--glass-blur-lite);
  -webkit-backdrop-filter: var(--glass-blur-lite);
  font-size: 0.72rem;
  transition: all var(--transition-normal);
}

.context-status.warning {
  border-color: rgba(255, 180, 84, 0.4);
}

.context-status.critical {
  border-color: rgba(255, 90, 110, 0.45);
  box-shadow: 0 0 12px rgba(255, 90, 110, 0.18);
}

.context-icon {
  display: flex;
  color: var(--text-muted);
  flex-shrink: 0;
  transition: color var(--transition-fast);
}

.context-status.warning .context-icon {
  color: var(--text-warning);
}

.context-status.critical .context-icon {
  color: var(--text-error);
}

.context-info {
  display: flex;
  flex-direction: column;
  min-width: 0;
  line-height: 1.2;
}

.context-percentage {
  font-family: var(--font-mono);
  font-weight: 600;
  font-size: 0.78rem;
  color: var(--text-heading);
}

.context-status.warning .context-percentage {
  color: var(--text-warning);
}

.context-status.critical .context-percentage {
  color: var(--text-error);
}

.pct {
  font-size: 0.6em;
  opacity: 0.7;
  margin-left: 1px;
}

.context-detail {
  color: var(--text-muted);
  font-size: 0.62rem;
  font-family: var(--font-mono);
  white-space: nowrap;
}

.context-bar {
  width: 52px;
  height: 4px;
  background: var(--bg-active);
  border-radius: var(--radius-pill);
  overflow: hidden;
  flex-shrink: 0;
}

.context-bar-fill {
  height: 100%;
  border-radius: var(--radius-pill);
  transition: width 0.5s var(--ease-out-expo);
}
</style>
