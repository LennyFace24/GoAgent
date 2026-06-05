<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

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
    <div class="context-icon">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
      </svg>
    </div>
    <div class="context-info">
      <div class="context-percentage">{{ usage.percentage.toFixed(1) }}%</div>
      <div class="context-detail">
        {{ formatTokens(usage.current_tokens) }} / {{ formatTokens(usage.max_tokens) }}
      </div>
    </div>
    <div class="context-bar">
      <div
        class="context-bar-fill"
        :style="{ width: Math.min(usage.percentage, 100) + '%' }"
      />
    </div>
  </div>
</template>

<style scoped>
.context-status {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 8px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  font-size: 0.75rem;
  transition: all 0.2s;
}

.context-status.warning {
  border-color: var(--warning);
}

.context-status.critical {
  border-color: var(--error);
}

.context-icon {
  color: var(--text-muted);
  flex-shrink: 0;
}

.context-status.warning .context-icon {
  color: var(--warning);
}

.context-status.critical .context-icon {
  color: var(--error);
}

.context-info {
  flex: 1;
  min-width: 0;
}

.context-percentage {
  font-weight: 600;
  color: var(--text-primary);
}

.context-status.warning .context-percentage {
  color: var(--warning);
}

.context-status.critical .context-percentage {
  color: var(--error);
}

.context-detail {
  color: var(--text-muted);
  font-size: 0.65rem;
  white-space: nowrap;
}

.context-bar {
  width: 60px;
  height: 4px;
  background: var(--bg-primary);
  border-radius: 2px;
  overflow: hidden;
  flex-shrink: 0;
}

.context-bar-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 2px;
  transition: width 0.3s ease;
}

.context-status.warning .context-bar-fill {
  background: var(--warning);
}

.context-status.critical .context-bar-fill {
  background: var(--error);
}
</style>
