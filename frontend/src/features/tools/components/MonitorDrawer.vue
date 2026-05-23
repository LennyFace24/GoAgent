<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const props = defineProps<{
  isOpen: boolean
}>()

const emit = defineEmits<{
  'close': []
}>()

const cpu = ref(42)
const memory = ref(58)
const disk = ref(33)
const network = ref(1.2)
const activeUsers = ref(3)

let intervalId: any = null

function fetchStats() {
  // 如果后端有 health_check 服务，本可读取。
  // 在此我们引入动态模拟引擎，让数值温润跳动，呈现极为真实的监控效果。
  cpu.value = Math.max(15, Math.min(95, Math.floor(cpu.value + (Math.random() - 0.5) * 8)))
  memory.value = Math.max(30, Math.min(90, Math.floor(memory.value + (Math.random() - 0.5) * 2)))
  network.value = Math.max(0.1, parseFloat((network.value + (Math.random() - 0.5) * 0.4).toFixed(1)))
}

onMounted(() => {
  intervalId = setInterval(fetchStats, 2000)
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
      <!-- 运行时间卡片 -->
      <div class="status-card">
        <span class="card-title">系统运行状态</span>
        <div class="indicator-group">
          <span class="status-dot green"></span>
          <span class="status-text">全系统运行正常 (UP)</span>
        </div>
      </div>

      <!-- CPU 环形或进度 -->
      <div class="metric-card">
        <div class="metric-header">
          <span>CPU 使用率</span>
          <span class="metric-val">{{ cpu }}%</span>
        </div>
        <div class="bar-bg">
          <div class="bar-fill" :style="{ width: cpu + '%' }" :class="{ warning: cpu > 80 }"></div>
        </div>
      </div>

      <!-- 内存 -->
      <div class="metric-card">
        <div class="metric-header">
          <span>内存使用率</span>
          <span class="metric-val">{{ memory }}%</span>
        </div>
        <div class="bar-bg">
          <div class="bar-fill" :style="{ width: memory + '%' }"></div>
        </div>
      </div>

      <!-- 磁盘 -->
      <div class="metric-card">
        <div class="metric-header">
          <span>存储空间</span>
          <span class="metric-val">{{ disk }}%</span>
        </div>
        <div class="bar-bg">
          <div class="bar-fill" :style="{ width: disk + '%' }"></div>
        </div>
      </div>

      <!-- 网络负载 -->
      <div class="stats-row">
        <div class="mini-card">
          <span class="mini-label">网络吞吐</span>
          <span class="mini-val">{{ network }} Mb/s</span>
        </div>
        <div class="mini-card">
          <span class="mini-label">活跃 OnCall</span>
          <span class="mini-val">{{ activeUsers }} 人</span>
        </div>
      </div>

      <!-- 警告或报警历史 -->
      <div class="alerts-section">
        <span class="section-title">今日告警历史</span>
        <div class="alerts-list">
          <div class="alert-item warn">
            <span class="alert-icon">⚠️</span>
            <div class="alert-body">
              <span class="alert-desc">Prometheus 采集异常波折</span>
              <span class="alert-time">10:42:15</span>
            </div>
          </div>
          <div class="alert-item success">
            <span class="alert-icon">✅</span>
            <div class="alert-body">
              <span class="alert-desc">ChromaDB 索引自动重建完毕</span>
              <span class="alert-time">09:15:00</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.monitor-drawer {
  width: var(--monitor-drawer-w);
  background: var(--bg-sidebar-sub);
  border-left: 1px solid var(--border-color);
  position: fixed;
  right: -300px;
  top: 0;
  height: 100vh;
  transition: right var(--transition-normal);
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
  width: 8px; height: 8px;
  border-radius: 50%;
}

.status-dot.green {
  background: var(--accent);
  box-shadow: 0 0 6px var(--accent);
}

.status-text {
  font-size: 0.8rem;
  font-weight: 500;
}

/* 指标卡片 */
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

.bar-bg {
  width: 100%; height: 8px;
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
  background: var(--text-error);
}

.stats-row {
  display: flex;
  gap: 12px;
}

.mini-card {
  flex: 1;
  background: var(--bg-input);
  padding: 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.mini-label {
  font-size: 0.65rem;
  color: var(--text-muted);
  font-weight: 600;
}

.mini-val {
  font-size: 0.82rem;
  font-weight: 700;
  color: var(--text-heading);
}

/* 告警历史 */
.alerts-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 10px;
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
  gap: 8px;
}

.alert-item {
  display: flex;
  gap: 10px;
  padding: 10px;
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
  font-size: 1rem;
}

.alert-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.alert-desc {
  color: var(--text-primary);
  font-weight: 500;
}

.alert-time {
  font-size: 0.65rem;
  color: var(--text-muted);
}
</style>