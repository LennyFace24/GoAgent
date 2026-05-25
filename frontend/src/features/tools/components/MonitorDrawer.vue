<script setup lang="ts">
import { ref, onUnmounted, watch } from 'vue'
import type { MetricsData } from '../types'

const props = defineProps<{
  isOpen: boolean
}>()

const emit = defineEmits<{
  'close': []
}>()

const data = ref<MetricsData | null>(null)
const loading = ref(false)
const error = ref('')
let intervalId: ReturnType<typeof setInterval> | null = null

async function fetchMetrics() {
  loading.value = true
  error.value = ''
  try {
    const res = await fetch('/api/metrics')
    if (!res.ok) {
      error.value = `HTTP ${res.status}`
      data.value = null
      return
    }
    const json: MetricsData = await res.json()
    if (json.status === 'error') {
      error.value = json.error || '未知错误'
      data.value = null
      return
    }
    data.value = json
    error.value = ''
  } catch (e) {
    error.value = 'Prometheus 连接不上'
    data.value = null
  } finally {
    loading.value = false
  }
}

watch(() => props.isOpen, (open) => {
  if (open) {
    fetchMetrics()
    intervalId = setInterval(fetchMetrics, 3000) // 3秒刷新一次
  } else {
    if (intervalId) {
      clearInterval(intervalId)
      intervalId = null
    }
  }
})

onUnmounted(() => {
  if (intervalId) clearInterval(intervalId)
})
</script>

<template>
  <div class="monitor-drawer" :class="{ open: isOpen }">
    <div class="drawer-header">
      <span class="header-title">🖥️ 实时系统状态</span>
      <button class="close-btn" @click="emit('close')">×</button>
    </div>

    <div class="drawer-content">
      <!-- 连接不上 -->
      <div v-if="error" class="status-card error">
        <div class="indicator-group">
          <span class="status-dot red"></span>
          <span class="status-text">{{ error }}</span>
        </div>
      </div>

      <!-- 首次加载中 -->
      <div v-else-if="loading && !data" class="status-card">
        <div class="indicator-group">
          <span class="status-dot"></span>
          <span class="status-text">正在连接 Prometheus...</span>
        </div>
      </div>

      <!-- 真实数据 -->
      <template v-else-if="data">
        <!-- 系统运行状态 -->
        <div class="status-card" :class="data.down_count > 0 ? 'warn' : 'ok'">
          <span class="card-title">系统运行状态</span>
          <div class="indicator-group">
            <span class="status-dot" :class="data.down_count > 0 ? 'yellow' : 'green'"></span>
            <span class="status-text">
              {{ data.down_count > 0 ? `${data.down_count} 个目标离线` : '全系统运行正常 (UP)' }}
            </span>
          </div>
          <div class="target-counts">
            <span class="up-label">{{ data.up_count }} UP</span>
            <span v-if="data.down_count > 0" class="down-label"> / {{ data.down_count }} DOWN</span>
          </div>
        </div>

        <!-- CPU 使用率 -->
        <div class="metric-card">
          <div class="metric-header">
            <span>CPU 使用率</span>
            <span class="metric-val">{{ data.cpu?.toFixed(1) ?? '--' }}%</span>
          </div>
          <div class="bar-bg">
            <div class="bar-fill" :style="{ width: (data.cpu ?? 0) + '%' }" :class="{ warning: (data.cpu ?? 0) > 80 }">
            </div>
          </div>
        </div>

        <!-- 内存使用率 -->
        <div class="metric-card">
          <div class="metric-header">
            <span>内存使用率</span>
            <span class="metric-val">{{ data.memory?.toFixed(1) ?? '--' }}%</span>
          </div>
          <div class="bar-bg">
            <div class="bar-fill" :style="{ width: (data.memory ?? 0) + '%' }"
              :class="{ warning: (data.memory ?? 0) > 80 }"></div>
          </div>
          <div v-if="data.mem_used_gb != null && data.mem_total_gb != null" class="metric-sub">
            {{ data.mem_used_gb.toFixed(1) }} / {{ data.mem_total_gb.toFixed(1) }} GB
          </div>
        </div>

        <!-- Swap 使用率 (omitempty) -->
        <div v-if="data.swap_use != null" class="metric-card">
          <div class="metric-header">
            <span>Swap 使用率</span>
            <span class="metric-val">{{ data.swap_use.toFixed(1) }}%</span>
          </div>
          <div class="bar-bg">
            <div class="bar-fill" :style="{ width: data.swap_use + '%' }" :class="{ warning: data.swap_use > 80 }"></div>
          </div>
        </div>

        <!-- 存储空间 -->
        <div class="metric-card">
          <div class="metric-header">
            <span>存储空间</span>
            <span class="metric-val">{{ data.disk?.toFixed(1) ?? '--' }}%</span>
          </div>
          <div class="bar-bg">
            <div class="bar-fill" :style="{ width: (data.disk ?? 0) + '%' }"></div>
          </div>
        </div>

        <!-- 磁盘 IO 繁忙度 (omitempty) -->
        <div v-if="data.disk_io_util != null" class="metric-card">
          <div class="metric-header">
            <span>磁盘 IO 繁忙度</span>
            <span class="metric-val">{{ data.disk_io_util.toFixed(1) }}%</span>
          </div>
          <div class="bar-bg">
            <div class="bar-fill" :style="{ width: data.disk_io_util + '%' }" :class="{ warning: data.disk_io_util > 80 }"></div>
          </div>
        </div>

        <!-- Inode 使用率 (omitempty) -->
        <div v-if="data.inode_use != null" class="metric-card">
          <div class="metric-header">
            <span>Inode 使用率</span>
            <span class="metric-val">{{ data.inode_use.toFixed(1) }}%</span>
          </div>
          <div class="bar-bg">
            <div class="bar-fill" :style="{ width: data.inode_use + '%' }" :class="{ warning: data.inode_use > 80 }"></div>
          </div>
        </div>

        <!-- 系统负载 -->
        <div class="stats-row">
          <div class="mini-card">
            <span class="mini-label">Load 1m</span>
            <span class="mini-val">{{ data.load1m?.toFixed(1) ?? '--' }}</span>
          </div>
          <div class="mini-card">
            <span class="mini-label">Load 5m</span>
            <span class="mini-val">{{ data.load5m?.toFixed(1) ?? '--' }}</span>
          </div>
          <div class="mini-card">
            <span class="mini-label">Load 15m</span>
            <span class="mini-val">{{ data.load15m?.toFixed(1) ?? '--' }}</span>
          </div>
        </div>

        <!-- 网络吞吐 -->
        <div class="stats-row">
          <div class="mini-card">
            <span class="mini-label">网络接收</span>
            <span class="mini-val">{{ data.network_rx?.toFixed(1) ?? '--' }} Mb/s</span>
          </div>
          <div class="mini-card">
            <span class="mini-label">网络发送</span>
            <span class="mini-val">{{ data.network_tx?.toFixed(1) ?? '--' }} Mb/s</span>
          </div>
        </div>

        <!-- TCP TIME_WAIT 和 OOM Kills (omitempty) -->
        <div v-if="data.tcp_tw != null || data.oom_kills_1h != null" class="stats-row">
          <div v-if="data.tcp_tw != null" class="mini-card">
            <span class="mini-label">TCP TIME_WAIT</span>
            <span class="mini-val">{{ data.tcp_tw.toFixed(0) }} 个</span>
          </div>
          <div v-if="data.oom_kills_1h != null" class="mini-card">
            <span class="mini-label">OOM Kills (1h)</span>
            <span class="mini-val" :style="{ color: data.oom_kills_1h > 0 ? '#e53e3e' : 'inherit', fontWeight: data.oom_kills_1h > 0 ? 'bold' : 'normal' }">{{ data.oom_kills_1h.toFixed(0) }} 次</span>
          </div>
        </div>

        <!-- 采集目标状态 -->
        <div v-if="data.targets && data.targets.length > 0" class="alerts-section">
          <span class="section-title">采集目标状态</span>
          <div class="alerts-list">
            <div v-for="(t, i) in data.targets" :key="i" class="alert-item" :class="t.up ? 'success' : 'warn'">
              <span class="alert-icon">{{ t.up ? '✓' : '⚠️' }}</span>
              <div class="alert-body">
                <span class="alert-desc">{{ t.job }}</span>
                <span class="alert-time">{{ t.instance }}</span>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.monitor-drawer {
  width: 280px;
  background: var(--bg-sidebar-sub);
  border-left: 1px solid var(--border-color);
  position: fixed;
  right: -300px;
  top: 0;
  height: 100vh;
  transition: right 0.3s ease;
  z-index: 100;
  display: flex;
  flex-direction: column;
}

.monitor-drawer.open {
  right: 0;
}

.drawer-header {
  padding: 20px;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-title {
  font-size: 0.88rem;
  font-weight: 700;
  color: var(--text-heading);
}

.close-btn {
  background: transparent;
  border: none;
  font-size: 1.5rem;
  color: var(--text-secondary);
  cursor: pointer;
}

.close-btn:hover {
  color: var(--text-primary);
}

.drawer-content {
  padding: 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.status-card {
  background: var(--bg-input);
  padding: 14px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-color);
}

.status-card.ok {
  border-color: rgba(16, 163, 127, 0.3);
}

.status-card.warn {
  border-color: rgba(255, 159, 64, 0.3);
}

.status-card.error {
  border-color: rgba(229, 62, 62, 0.3);
  background: rgba(229, 62, 62, 0.04);
}

.card-title {
  font-size: 0.72rem;
  color: var(--text-muted);
  font-weight: 700;
  text-transform: uppercase;
  margin-bottom: 8px;
  display: block;
}

.indicator-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-muted);
  animation: pulse 1.5s infinite;
}

.status-dot.green {
  background: var(--accent);
  box-shadow: 0 0 6px var(--accent);
  animation: none;
}

.status-dot.yellow {
  background: #ff9f40;
  box-shadow: 0 0 6px #ff9f40;
  animation: none;
}

.status-dot.red {
  background: #e53e3e;
  box-shadow: 0 0 6px #e53e3e;
  animation: none;
}

@keyframes pulse {

  0%,
  100% {
    opacity: 0.3;
  }

  50% {
    opacity: 1;
  }
}

.status-text {
  font-size: 0.8rem;
  font-weight: 500;
  color: var(--text-primary);
}

.target-counts {
  margin-top: 6px;
  font-size: 0.7rem;
}

.up-label {
  color: var(--accent);
}

.down-label {
  color: #e53e3e;
}

.metric-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.metric-header {
  display: flex;
  justify-content: space-between;
  font-size: 0.78rem;
  color: var(--text-secondary);
}

.metric-val {
  font-weight: 600;
  color: var(--text-heading);
}

.metric-sub {
  font-size: 0.68rem;
  color: var(--text-muted);
  margin-top: 2px;
}

.bar-bg {
  width: 100%;
  height: 8px;
  background: var(--bg-input);
  border-radius: 4px;
  overflow: hidden;
}

.bar-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 4px;
  transition: width 0.5s ease-in-out;
}

.bar-fill.warning {
  background: #e53e3e;
}

.stats-row {
  display: flex;
  gap: 10px;
}

.mini-card {
  flex: 1;
  background: var(--bg-input);
  padding: 10px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.mini-label {
  font-size: 0.65rem;
  color: var(--text-muted);
  font-weight: 600;
}

.mini-val {
  font-size: 0.8rem;
  font-weight: 700;
  color: var(--text-heading);
}

.alerts-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section-title {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
}

.alerts-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.alert-item {
  display: flex;
  gap: 8px;
  padding: 8px 10px;
  border-radius: var(--radius-md);
  font-size: 0.75rem;
  border: 1px solid var(--border-color);
}

.alert-item.warn {
  background: rgba(255, 159, 64, 0.05);
}

.alert-item.success {
  background: rgba(16, 163, 127, 0.04);
}

.alert-icon {
  font-size: 0.9rem;
  flex-shrink: 0;
}

.alert-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;
}

.alert-desc {
  color: var(--text-primary);
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.alert-time {
  font-size: 0.65rem;
  color: var(--text-muted);
}
</style>