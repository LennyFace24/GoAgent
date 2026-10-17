<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Activity, Radio } from 'lucide-vue-next'
import type { MetricsData } from '../types'

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

// 常驻：挂载即拉取，每 3 秒刷新
onMounted(() => {
  fetchMetrics()
  intervalId = setInterval(fetchMetrics, 3000)
})

onUnmounted(() => {
  if (intervalId) clearInterval(intervalId)
})
</script>

<template>
  <div class="monitor-drawer">
    <div class="drawer-header">
      <div class="header-title-wrap">
        <span class="header-icon"><Activity :size="15" :stroke-width="2" /></span>
        <span class="header-title">实时系统状态</span>
      </div>
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
          <span class="card-title">
            <Radio :size="11" :stroke-width="2.2" />
            <span>系统运行状态</span>
          </span>
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

        <!-- Swap 使用率 -->
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

        <!-- 磁盘 IO 繁忙度 -->
        <div v-if="data.disk_io_util != null" class="metric-card">
          <div class="metric-header">
            <span>磁盘 IO 繁忙度</span>
            <span class="metric-val">{{ data.disk_io_util.toFixed(1) }}%</span>
          </div>
          <div class="bar-bg">
            <div class="bar-fill" :style="{ width: data.disk_io_util + '%' }" :class="{ warning: data.disk_io_util > 80 }"></div>
          </div>
        </div>

        <!-- Inode 使用率 -->
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
      </template>

      <!-- 无数据且非加载中 -->
      <div v-else class="status-card">
        <div class="indicator-group">
          <span class="status-text">暂无数据</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.monitor-drawer {
  width: var(--monitor-drawer-w);
  background: var(--bg-sidebar-sub);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  border-left: 1px solid var(--border-glass);
  height: 100vh;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}

.drawer-header {
  padding: 18px 20px;
  border-bottom: 1px solid var(--border-hairline);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-icon {
  display: flex;
  color: var(--accent-bright);
}

.header-title {
  font-family: var(--font-display);
  font-size: 0.88rem;
  font-weight: 600;
  color: var(--text-heading);
  letter-spacing: -0.01em;
}

.drawer-content {
  padding: 18px 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 18px;
  flex: 1;
}

.status-card {
  background: var(--bg-input);
  padding: 14px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-hairline);
}

.status-card.ok { border-color: rgba(74, 222, 128, 0.3); }
.status-card.warn { border-color: rgba(250, 204, 21, 0.35); }
.status-card.error { border-color: rgba(248, 113, 113, 0.4); }

.card-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin-bottom: 10px;
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
  flex-shrink: 0;
}

.status-dot.green { background: #4ade80; box-shadow: 0 0 8px rgba(74, 222, 128, 0.6); }
.status-dot.yellow { background: #facc15; box-shadow: 0 0 8px rgba(250, 204, 21, 0.6); }
.status-dot.red { background: #f87171; box-shadow: 0 0 8px rgba(248, 113, 113, 0.6); }

.status-text {
  font-size: 0.78rem;
  color: var(--text-primary);
}

.target-counts {
  margin-top: 8px;
  font-size: 0.72rem;
}

.up-label { color: #4ade80; }
.down-label { color: #f87171; }

.metric-card {
  background: var(--bg-input);
  padding: 12px 14px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-hairline);
}

.metric-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.78rem;
  margin-bottom: 8px;
}

.metric-header span:first-child { color: var(--text-secondary); }
.metric-val { font-family: var(--font-mono); color: var(--text-primary); font-weight: 600; }

.bar-bg {
  height: 6px;
  background: var(--bg-hover);
  border-radius: 99px;
  overflow: hidden;
}

.bar-fill {
  height: 100%;
  background: var(--accent-gradient);
  border-radius: 99px;
  transition: width 0.5s var(--ease-out-expo);
}

.bar-fill.warning { background: linear-gradient(135deg, #facc15, #f59e0b); }

.metric-sub {
  margin-top: 6px;
  font-size: 0.7rem;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.stats-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.mini-card {
  background: var(--bg-input);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-lg);
  padding: 10px;
  text-align: center;
}

.mini-label {
  display: block;
  font-size: 0.68rem;
  color: var(--text-muted);
  margin-bottom: 4px;
}

.mini-val {
  font-family: var(--font-mono);
  font-size: 0.82rem;
  color: var(--text-primary);
}
</style>
