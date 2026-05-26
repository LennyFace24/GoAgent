<script lang="ts">
export default { name: 'DashboardView' }
</script>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, onActivated, onDeactivated, watch, nextTick } from 'vue'
import type { MetricsData } from '../types'
import { BarChart3, AlertTriangle, TrendingUp, PieChart } from 'lucide-vue-next'

const props = defineProps<{
  view?: string
}>()

const data = ref<MetricsData | null>(null)
const history = ref<MetricsData[]>([])
const loading = ref(false)
const error = ref('')
const selectedLineMetric = ref<keyof MetricsData>('cpu')

let intervalId: ReturnType<typeof setInterval> | null = null

// Canvas 元素引用
const lineCanvas = ref<HTMLCanvasElement | null>(null)
const pieCanvas = ref<HTMLCanvasElement | null>(null)

// 折线图动画状态
interface Point { x: number; y: number }
let prevPoints: Point[] = []
let animationFrameId: number | null = null

// 触发折线指标切换
const lineMetricsList = [
  { key: 'cpu', label: 'CPU 使用率 (%)' },
  { key: 'memory', label: '内存使用率 (%)' },
  { key: 'swap_use', label: 'Swap 使用率 (%)', check: true },
  { key: 'disk_io_util', label: '磁盘 IO 繁忙度 (%)', check: true },
  { key: 'inode_use', label: 'Inode 使用率 (%)', check: true },
  { key: 'load1m', label: '1 分钟系统负载' },
  { key: 'network_rx', label: '网络接收带宽 (Mb/s)' }
]

const availableLineMetrics = ref(lineMetricsList)

async function fetchMetrics() {
  try {
    const res = await fetch('/api/metrics')
    if (!res.ok) {
      error.value = `HTTP ${res.status}`
      return
    }
    const json: MetricsData = await res.json()
    if (json.status === 'error') {
      error.value = json.error || '未知错误'
      return
    }
    data.value = json
    error.value = ''
    
    // 更新历史数据
    history.value.push(JSON.parse(JSON.stringify(json)))
    if (history.value.length > 20) {
      history.value.shift()
    }
    
    // 动态校验支持的折线选项
    availableLineMetrics.value = lineMetricsList.filter(m => {
      if (!m.check) return true
      return json[m.key] !== undefined && json[m.key] !== null
    })

    // 如果选中的指标当前不可用，回退至 cpu
    if (!availableLineMetrics.value.some(m => m.key === selectedLineMetric.value)) {
      selectedLineMetric.value = 'cpu'
    }

    // 绘制图表
    nextTick(() => {
      drawCharts()
    })
  } catch (e) {
    error.value = 'Prometheus 监控服务连接失败'
  }
}

// 缓动函数 (ease-out cubic)
function easeOutCubic(t: number): number {
  return 1 - Math.pow(1 - t, 3)
}

// 计算目标点坐标
function calcTargetPoints(): Point[] {
  const canvas = lineCanvas.value
  if (!canvas) return []
  const hData = history.value
  if (hData.length === 0) return []

  const width = canvas.clientWidth
  const height = canvas.clientHeight
  const paddingLeft = 45
  const paddingRight = 15
  const paddingTop = 20
  const paddingBottom = 30
  const chartWidth = width - paddingLeft - paddingRight
  const chartHeight = height - paddingTop - paddingBottom

  const key = selectedLineMetric.value
  let maxVal = 100
  if (key === 'load1m') {
    const maxLoad = Math.max(...hData.map(d => Number(d.load1m ?? 0)))
    maxVal = Math.max(maxLoad * 1.2, 2.0)
  } else if (key === 'network_rx') {
    const maxNet = Math.max(...hData.map(d => Number(d.network_rx ?? 0)))
    maxVal = Math.max(maxNet * 1.2, 5.0)
  }

  return hData.map((d, index) => {
    const val = Number(d[key] ?? 0)
    const ratioX = hData.length > 1 ? index / (hData.length - 1) : 0
    const ratioY = maxVal > 0 ? val / maxVal : 0
    return {
      x: paddingLeft + ratioX * chartWidth,
      y: paddingTop + chartHeight * (1 - ratioY)
    }
  })
}

// 绘制静态背景（网格线 + 轴标签），不含折线
function drawLineChartBg(ctx: CanvasRenderingContext2D, width: number, height: number) {
  const paddingLeft = 45
  const paddingRight = 15
  const paddingTop = 20
  const paddingBottom = 30
  const chartHeight = height - paddingTop - paddingBottom

  const key = selectedLineMetric.value
  let maxVal = 100
  if (key === 'load1m') {
    const maxLoad = Math.max(...history.value.map(d => Number(d.load1m ?? 0)))
    maxVal = Math.max(maxLoad * 1.2, 2.0)
  } else if (key === 'network_rx') {
    const maxNet = Math.max(...history.value.map(d => Number(d.network_rx ?? 0)))
    maxVal = Math.max(maxNet * 1.2, 5.0)
  }

  ctx.strokeStyle = getThemeColor('--border-color', 'rgba(0, 0, 0, 0.06)')
  ctx.lineWidth = 1
  ctx.font = '10px sans-serif'
  ctx.fillStyle = getThemeColor('--text-muted', '#86868b')

  const gridLinesCount = 4
  for (let i = 0; i <= gridLinesCount; i++) {
    const ratio = i / gridLinesCount
    const y = paddingTop + chartHeight * (1 - ratio)
    ctx.beginPath()
    ctx.moveTo(paddingLeft, y)
    ctx.lineTo(width - paddingRight, y)
    ctx.stroke()
    const labelVal = ratio * maxVal
    ctx.fillText(labelVal.toFixed(key === 'load1m' ? 1 : 0), 10, y + 4)
  }

  ctx.fillStyle = getThemeColor('--text-muted', '#86868b')
  ctx.font = '9px monospace'
  if (history.value.length > 0) {
    ctx.fillText('20s前', paddingLeft, height - 10)
    ctx.fillText('现在', width - paddingRight - 20, height - 10)
  }
}

// 绘制折线（给定一组点坐标）
function drawLineAtPoints(ctx: CanvasRenderingContext2D, points: Point[], width: number, height: number) {
  if (points.length === 0) return

  const paddingTop = 20
  const paddingBottom = 30
  const accentColor = getThemeColor('--accent', '#10a37f')

  // 渐变填充区域
  const gradient = ctx.createLinearGradient(0, paddingTop, 0, height - paddingBottom)
  gradient.addColorStop(0, hexToRgbA(accentColor, 0.25))
  gradient.addColorStop(1, hexToRgbA(accentColor, 0.0))

  ctx.beginPath()
  ctx.moveTo(points[0].x, height - paddingBottom)
  for (const p of points) {
    ctx.lineTo(p.x, p.y)
  }
  ctx.lineTo(points[points.length - 1].x, height - paddingBottom)
  ctx.closePath()
  ctx.fillStyle = gradient
  ctx.fill()

  // 绘制折线
  ctx.beginPath()
  ctx.moveTo(points[0].x, points[0].y)
  for (let i = 1; i < points.length; i++) {
    ctx.lineTo(points[i].x, points[i].y)
  }
  ctx.strokeStyle = accentColor
  ctx.lineWidth = 2.5
  ctx.stroke()

  // 实心焦点
  const lastP = points[points.length - 1]
  ctx.beginPath()
  ctx.arc(lastP.x, lastP.y, 4, 0, 2 * Math.PI)
  ctx.fillStyle = accentColor
  ctx.fill()
  ctx.strokeStyle = '#fff'
  ctx.lineWidth = 1.5
  ctx.stroke()
}

// 在 Canvas 上绘制一帧
function renderFrame(points: Point[]) {
  const canvas = lineCanvas.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const dpr = window.devicePixelRatio || 1
  const width = canvas.clientWidth
  const height = canvas.clientHeight
  canvas.width = width * dpr
  canvas.height = height * dpr
  ctx.scale(dpr, dpr)
  ctx.clearRect(0, 0, width, height)

  drawLineChartBg(ctx, width, height)
  drawLineAtPoints(ctx, points, width, height)
}

// 主入口：计算目标点，启动动画
function drawLineChart() {
  const targetPoints = calcTargetPoints()
  if (targetPoints.length === 0) return

  // 取消未完成的动画
  if (animationFrameId !== null) {
    cancelAnimationFrame(animationFrameId)
    animationFrameId = null
  }

  // 首次绘制无动画
  if (prevPoints.length === 0 || prevPoints.length !== targetPoints.length) {
    prevPoints = targetPoints
    renderFrame(targetPoints)
    return
  }

  // 执行缓动动画
  const duration = 400
  const startTime = performance.now()
  const fromPoints = [...prevPoints]

  function animate(now: number) {
    const elapsed = now - startTime
    const progress = Math.min(elapsed / duration, 1)
    const eased = easeOutCubic(progress)

    const interpolated = targetPoints.map((target, i) => ({
      x: fromPoints[i].x + (target.x - fromPoints[i].x) * eased,
      y: fromPoints[i].y + (target.y - fromPoints[i].y) * eased
    }))

    renderFrame(interpolated)

    if (progress < 1) {
      animationFrameId = requestAnimationFrame(animate)
    } else {
      prevPoints = targetPoints
      animationFrameId = null
    }
  }

  animationFrameId = requestAnimationFrame(animate)
}

// 采用 HTML5 Canvas 纯手绘内存分配环形比例图 (Donut Pie Chart)
function drawPieChart() {
  const canvas = pieCanvas.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const dpr = window.devicePixelRatio || 1
  const width = canvas.clientWidth
  const height = canvas.clientHeight
  canvas.width = width * dpr
  canvas.height = height * dpr
  ctx.scale(dpr, dpr)

  ctx.clearRect(0, 0, width, height)

  const activeData = data.value
  if (!activeData) return

  // 提取物理内存以及 Swap 状态
  const memTotal = activeData.mem_total_gb ?? 8.0
  const memUsed = activeData.mem_used_gb ?? 3.5
  const memFree = Math.max(0, memTotal - memUsed)
  
  const segments = [
    { label: '物理内存已用', value: memUsed, color: '#10a37f' },
    { label: '物理内存空闲', value: memFree, color: getThemeColor('--border-color', 'rgba(0, 0, 0, 0.1)') }
  ]

  // 如果支持 Swap 且有数值，将其动态加入分配饼图
  if (activeData.swap_use != null) {
    const swapTotal = 2.0 // Mock swap total size
    const swapUsed = (activeData.swap_use / 100) * swapTotal
    segments.push({ label: 'Swap 虚拟内存已用', value: swapUsed, color: '#ff9f40' })
  }

  const totalVal = segments.reduce((sum, s) => sum + s.value, 0)
  const centerX = width * 0.35
  const centerY = height * 0.5
  const radius = Math.min(width * 0.25, centerY * 0.75)

  let startAngle = -Math.PI / 2

  // 1. 绘制扇区
  for (const seg of segments) {
    const sliceAngle = (seg.value / totalVal) * (2 * Math.PI)
    
    ctx.beginPath()
    ctx.arc(centerX, centerY, radius, startAngle, startAngle + sliceAngle)
    ctx.lineTo(centerX, centerY)
    ctx.fillStyle = seg.color
    ctx.fill()
    
    startAngle += sliceAngle
  }

  // 2. 绘制中心孔洞 (Donut 环形设计)
  ctx.beginPath()
  ctx.arc(centerX, centerY, radius * 0.6, 0, 2 * Math.PI)
  ctx.fillStyle = getThemeColor('--bg-sidebar-sub', '#ffffff')
  ctx.fill()

  // 3. 环形中心文字
  ctx.fillStyle = getThemeColor('--text-secondary', '#6e6e73')
  ctx.font = 'bold 11px sans-serif'
  ctx.textAlign = 'center'
  ctx.fillText('物理总内存', centerX, centerY - 5)
  ctx.fillStyle = getThemeColor('--text-primary', '#e5e5eb')
  ctx.font = 'bold 13px monospace'
  ctx.fillText(`${memTotal.toFixed(1)} GB`, centerX, centerY + 10)

  // 4. 右侧绘制图例 Legend
  ctx.textAlign = 'left'
  ctx.font = '11px sans-serif'
  const legendX = width * 0.7
  segments.forEach((seg, i) => {
    const legendY = centerY - (segments.length * 15) / 2 + i * 25

    // 图例色块
    ctx.beginPath()
    ctx.arc(legendX - 10, legendY - 3, 5, 0, 2 * Math.PI)
    ctx.fillStyle = seg.color
    ctx.fill()

    // 标签文字
    ctx.fillStyle = getThemeColor('--text-primary', '#e5e5eb')
    ctx.font = 'bold 11px sans-serif'
    ctx.fillText(`${seg.label}`, legendX, legendY)

    // 容量数值
    ctx.fillStyle = getThemeColor('--text-secondary', '#6e6e73')
    ctx.font = '10px monospace'
    ctx.fillText(`${seg.value.toFixed(1)} GB (${((seg.value / totalVal) * 100).toFixed(0)}%)`, legendX, legendY + 12)
  })
}

function drawCharts() {
  drawLineChart()
  drawPieChart()
}

// 辅助方法：实时动态抓取 CSS 变量颜色
function getThemeColor(variableName: string, fallback: string): string {
  if (typeof window === 'undefined') return fallback
  const root = document.documentElement
  const color = root.style.getPropertyValue(variableName) || getComputedStyle(root).getPropertyValue(variableName)
  return color ? color.trim() : fallback
}

// 辅助方法：Hex 转 Rgba 供渐变渲染
function hexToRgbA(hex: string, alpha: number): string {
  let c: any
  if (/^#([A-Fa-f0-9]{3}){1,2}$/.test(hex)) {
    c = hex.substring(1).split('')
    if (c.length == 3) {
      c = [c[0], c[0], c[1], c[1], c[2], c[2]]
    }
    c = '0x' + c.join('')
    return 'rgba(' + [(c >> 16) & 255, (c >> 8) & 255, c & 255].join(',') + ',' + alpha + ')'
  }
  return hex
}

// keep-alive: 组件激活时启动定时器，失活时暂停
onActivated(() => {
  fetchMetrics()
  intervalId = setInterval(fetchMetrics, 3000)
  window.addEventListener('resize', drawCharts)
})

onDeactivated(() => {
  if (intervalId) { clearInterval(intervalId); intervalId = null }
  if (animationFrameId !== null) { cancelAnimationFrame(animationFrameId); animationFrameId = null }
  window.removeEventListener('resize', drawCharts)
})

// 组件彻底销毁时清理（keep-alive max 溢出时触发）
onUnmounted(() => {
  if (intervalId) clearInterval(intervalId)
  if (animationFrameId !== null) cancelAnimationFrame(animationFrameId)
  window.removeEventListener('resize', drawCharts)
})

watch(selectedLineMetric, () => {
  nextTick(() => drawLineChart())
})

watch(() => props.view, () => {
  nextTick(() => drawCharts())
})
</script>

<template>
  <div class="dashboard-workspace">
    <!-- Header -->
    <header class="dashboard-header">
      <div class="header-left">
        <h2 class="title"><BarChart3 :size="20" style="vertical-align: middle; margin-right: 6px;" />系统核心监控指标</h2>
        <p class="subtitle">通过 Prometheus 实时采集的主机物理及虚拟指标诊断中心</p>
      </div>
      <div class="header-right">
        <span class="pulse-indicator" :class="{ loading: loading }"></span>
        <span class="status-label">{{ loading ? '正在载入指标...' : '' }}</span>
      </div>
    </header>

    <div class="dashboard-scrollable-content">
      <!-- 异常状态显示 -->
      <div v-if="error" class="error-banner">
        <AlertTriangle class="banner-icon" :size="18" />
        <span class="banner-text">{{ error }}</span>
      </div>

      <!-- 十大核心指标速览面板 -->
      <div class="metrics-grid" v-if="data && (view === 'overview' || !view)">
        <!-- CPU Card -->
        <div class="grid-card accent">
          <div class="card-head">
            <span>CPU 使用率</span>
            <span class="badge">CPU</span>
          </div>
          <div class="card-body">
            <h3 class="metric-val-large">{{ data.cpu?.toFixed(1) ?? '--' }} <span class="unit">%</span></h3>
            <div class="dashboard-mini-bar">
              <div class="bar-fill" :style="{ width: (data.cpu ?? 0) + '%' }" :class="{ warning: (data.cpu ?? 0) > 80 }"></div>
            </div>
          </div>
        </div>

        <!-- Memory Card -->
        <div class="grid-card">
          <div class="card-head">
            <span>物理内存</span>
            <span class="badge gray">RAM</span>
          </div>
          <div class="card-body">
            <h3 class="metric-val-large">{{ data.memory?.toFixed(1) ?? '--' }} <span class="unit">%</span></h3>
            <p class="sub-desc" v-if="data.mem_used_gb != null && data.mem_total_gb != null">
              已使用 {{ data.mem_used_gb.toFixed(1) }} / {{ data.mem_total_gb.toFixed(1) }} GB
            </p>
          </div>
        </div>

        <!-- Disk Card -->
        <div class="grid-card">
          <div class="card-head">
            <span>磁盘空间</span>
            <span class="badge gray">DISK</span>
          </div>
          <div class="card-body">
            <h3 class="metric-val-large">{{ data.disk?.toFixed(1) ?? '--' }} <span class="unit">%</span></h3>
            <p class="sub-desc">宿主机根分区存储使用状态</p>
          </div>
        </div>

        <!-- Load 1m Card -->
        <div class="grid-card">
          <div class="card-head">
            <span>系统平均负载</span>
            <span class="badge gray">LOAD</span>
          </div>
          <div class="card-body">
            <h3 class="metric-val-large">{{ data.load1m?.toFixed(2) ?? '--' }}</h3>
            <p class="sub-desc">5m: {{ data.load5m?.toFixed(2) ?? '--' }} | 15m: {{ data.load15m?.toFixed(2) ?? '--' }}</p>
          </div>
        </div>

        <!-- 1. Swap 使用率 Card (omitempty) -->
        <div class="grid-card" v-if="data.swap_use != null">
          <div class="card-head">
            <span>Swap 使用率</span>
            <span class="badge orange">SWAP</span>
          </div>
          <div class="card-body">
            <h3 class="metric-val-large">{{ data.swap_use.toFixed(1) }} <span class="unit">%</span></h3>
            <p class="sub-desc">虚拟磁盘 Swap 占用百分比</p>
          </div>
        </div>

        <!-- 2. 磁盘 IO 繁忙度 Card (omitempty) -->
        <div class="grid-card" v-if="data.disk_io_util != null">
          <div class="card-head">
            <span>磁盘 IO 繁忙度</span>
            <span class="badge blue">IO</span>
          </div>
          <div class="card-body">
            <h3 class="metric-val-large">{{ data.disk_io_util.toFixed(1) }} <span class="unit">%</span></h3>
            <p class="sub-desc">磁盘 IO 请求时间占比 (Util %)</p>
          </div>
        </div>

        <!-- 3. Inode 使用率 Card (omitempty) -->
        <div class="grid-card" v-if="data.inode_use != null">
          <div class="card-head">
            <span>Inode 使用率</span>
            <span class="badge cyan">INODE</span>
          </div>
          <div class="card-body">
            <h3 class="metric-val-large">{{ data.inode_use.toFixed(1) }} <span class="unit">%</span></h3>
            <p class="sub-desc">文件索引节点 Inode 占用比</p>
          </div>
        </div>

        <!-- 4. TCP TIME_WAIT Card (omitempty) -->
        <div class="grid-card" v-if="data.tcp_tw != null">
          <div class="card-head">
            <span>TCP TIME_WAIT</span>
            <span class="badge purple">TCP</span>
          </div>
          <div class="card-body">
            <h3 class="metric-val-large">{{ data.tcp_tw.toFixed(0) }} <span class="unit">个</span></h3>
            <p class="sub-desc">当前 TIME_WAIT 状态连接计数</p>
          </div>
        </div>

        <!-- 5. 过去 1 小时 OOM Kills Card (omitempty) -->
        <div class="grid-card" v-if="data.oom_kills_1h != null" :class="{ danger: data.oom_kills_1h > 0 }">
          <div class="card-head">
            <span>1H OOM Kills</span>
            <span class="badge red">OOM</span>
          </div>
          <div class="card-body">
            <h3 class="metric-val-large" :style="{ color: data.oom_kills_1h > 0 ? '#e53e3e' : 'inherit' }">
              {{ data.oom_kills_1h.toFixed(0) }} <span class="unit">次</span>
            </h3>
            <p class="sub-desc" :style="{ color: data.oom_kills_1h > 0 ? '#e53e3e' : 'inherit', fontWeight: data.oom_kills_1h > 0 ? 'bold' : 'normal' }">
              {{ data.oom_kills_1h > 0 ? '警告: 系统检测到 OOM Kills!' : '内核未发生内存溢出强杀' }}
            </p>
          </div>
        </div>
      </div>

      <!-- Canvas 设计图表看板 -->
      <div class="charts-flex-container" v-if="data" :class="{ 'single-chart': view === 'line' || view === 'pie' }">
        <!-- 核心折线趋势 Canvas -->
        <div class="chart-box main-trend" v-show="view !== 'pie'">
          <div class="chart-head-selector">
            <span class="chart-box-title"><TrendingUp :size="16" style="vertical-align: middle; margin-right: 4px;" />实时指标波动历史折线 (最近 20 点)</span>
            <select v-model="selectedLineMetric" class="metric-selector-dropdown">
              <option v-for="m in availableLineMetrics" :key="m.key" :value="m.key">
                {{ m.label }}
              </option>
            </select>
          </div>
          <div class="canvas-wrapper">
            <canvas ref="lineCanvas"></canvas>
          </div>
        </div>

        <!-- 内存分配 Donut Canvas -->
        <div class="chart-box memory-pie" v-show="view !== 'line'">
          <span class="chart-box-title"><PieChart :size="16" style="vertical-align: middle; margin-right: 4px;" />内存分配比例分析饼图</span>
          <div class="canvas-wrapper">
            <canvas ref="pieCanvas"></canvas>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dashboard-workspace {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: var(--bg-body);
  height: 100vh;
  overflow: hidden;
}

.dashboard-header {
  padding: 24px 40px;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-shrink: 0;
}

.dashboard-header .title {
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--text-heading, #1d1d1f);
  margin: 0 0 4px 0;
}

.dashboard-header .subtitle {
  font-size: 0.8rem;
  color: var(--text-muted, #86868b);
  margin: 0;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--bg-sidebar-mini);
  padding: 8px 16px;
  border-radius: var(--radius-lg, 12px);
  border: 1px solid var(--border-color);
}

.pulse-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #10a37f;
}

.pulse-indicator.loading {
  animation: pulse-glow 1s infinite alternate;
}

@keyframes pulse-glow {
  0% { transform: scale(1); opacity: 0.5; }
  100% { transform: scale(1.3); opacity: 1; }
}

.status-label {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-primary);
}

.dashboard-scrollable-content {
  flex: 1;
  padding: 24px 40px 40px 40px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.error-banner {
  background: rgba(229, 62, 62, 0.08);
  border: 1px solid rgba(229, 62, 62, 0.2);
  border-radius: var(--radius-md, 8px);
  padding: 12px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.banner-icon {
  display: flex;
  align-items: center;
  color: #e53e3e;
  flex-shrink: 0;
}

.banner-text {
  font-size: 0.82rem;
  color: #e53e3e;
  font-weight: 500;
}

/* 核心指标九宫格 Grid */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 16px;
  flex-shrink: 0;
}

.grid-card {
  background: var(--bg-sidebar-sub, #ffffff);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg, 12px);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-shadow: var(--shadow-sm);
  transition: transform 0.2s, box-shadow 0.2s;
}

.grid-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.grid-card.accent {
  border-color: rgba(16, 163, 127, 0.4);
}

.grid-card.danger {
  border-color: rgba(229, 62, 62, 0.4);
  background: rgba(229, 62, 62, 0.02);
}

.card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-secondary);
}

.badge {
  font-size: 0.6rem;
  padding: 2px 6px;
  border-radius: var(--radius-sm, 4px);
  font-weight: 700;
  color: #fff;
  background: var(--accent, #10a37f);
}

.badge.gray {
  background: var(--text-muted);
}
.badge.orange {
  background: #ff9f40;
}
.badge.blue {
  background: #3182ce;
}
.badge.cyan {
  background: #00b5d8;
}
.badge.purple {
  background: #805ad5;
}
.badge.red {
  background: #e53e3e;
}

.metric-val-large {
  font-size: 1.6rem;
  font-weight: 700;
  margin: 0;
  font-family: monospace;
  color: var(--text-main);
}

.metric-val-large .unit {
  font-size: 0.88rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin-left: 2px;
}

.sub-desc {
  font-size: 0.68rem;
  color: var(--text-muted);
  margin: 4px 0 0 0;
}

.dashboard-mini-bar {
  width: 100%;
  height: 4px;
  background: var(--bg-sidebar-mini);
  border-radius: 2px;
  overflow: hidden;
  margin-top: 8px;
}

.bar-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 2px;
}
.bar-fill.warning {
  background: #e53e3e;
}

/* 两个大图表排列容器 */
.charts-flex-container {
  display: flex;
  gap: 20px;
  flex: 1;
  min-height: 280px;
}

.chart-box {
  background: var(--bg-sidebar-sub, #ffffff);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg, 12px);
  padding: 20px;
  box-shadow: var(--shadow-sm);
  display: flex;
  flex-direction: column;
}

.chart-box.main-trend {
  flex: 1.6;
}

.chart-box.memory-pie {
  flex: 1;
}

.chart-box-title {
  font-size: 0.82rem;
  font-weight: 700;
  color: var(--text-main);
  margin-bottom: 12px;
}

.chart-head-selector {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.metric-selector-dropdown {
  background: var(--bg-input);
  border: 1px solid var(--border-color);
  color: var(--text-primary);
  padding: 6px 12px;
  border-radius: var(--radius-md);
  font-size: 0.75rem;
  font-weight: 600;
  cursor: pointer;
  outline: none;
}

.metric-selector-dropdown:focus {
  border-color: var(--accent);
}

.canvas-wrapper {
  flex: 1;
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 200px;
}

.canvas-wrapper canvas {
  position: absolute;
  top: 0;
  left: 0;
  width: 100% !important;
  height: 100% !important;
}

.charts-flex-container.single-chart {
  flex: 1;
}

.charts-flex-container.single-chart .chart-box {
  flex: 1;
}
</style>