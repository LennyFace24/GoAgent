<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useConversations } from '../composables/useConversations'
import {
  MessageSquare,
  BarChart3,
  TrendingUp,
  PieChart,
  Upload,
  FileText,
  Plus,
  X,
  Search,
  Layers,
} from 'lucide-vue-next'

const props = defineProps<{
  activeNav: string
  activeId: string
  dashboardView?: string
}>()

const emit = defineEmits<{
  'select': [id: string]
  'created': [id: string]
  'dashboard-view': [view: string]
}>()

const { conversations, load, create, remove } = useConversations()
const searchQuery = ref('')
const isUploading = ref(false)
const uploadProgress = ref(0)
const docsList = ref<string[]>([])

// ---------- 宽度可调 + 收起/展开 ----------
const MIN_WIDTH = 160
const MAX_WIDTH = 420
const COLLAPSE_THRESHOLD = 96 // 拖到低于该宽度时瞬间收起
const sidebarWidth = ref(280) // 当前宽度（初始与 CSS 变量一致）
const collapsed = ref(false)

// 拖拽调整宽度
let dragStartX = 0
let dragStartWidth = 0
let dragging = false

function onResizeStart(e: PointerEvent) {
  dragging = true
  dragStartX = e.clientX
  dragStartWidth = sidebarWidth.value
  // 拖拽期间禁用文本选择，避免选中内容干扰
  document.body.style.userSelect = 'none'
  window.addEventListener('pointermove', onResizeMove)
  window.addEventListener('pointerup', onResizeEnd)
}

function onResizeMove(e: PointerEvent) {
  if (!dragging) return
  const delta = e.clientX - dragStartX
  const next = dragStartWidth + delta
  if (next <= COLLAPSE_THRESHOLD) {
    // 拖到过窄 → 瞬间收起
    collapsed.value = true
    sidebarWidth.value = 0
    return
  }
  // 从收起状态拖出则先恢复
  if (collapsed.value && next > COLLAPSE_THRESHOLD) {
    collapsed.value = false
  }
  sidebarWidth.value = Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, next))
}

function onResizeEnd() {
  dragging = false
  document.body.style.userSelect = ''
  window.removeEventListener('pointermove', onResizeMove)
  window.removeEventListener('pointerup', onResizeEnd)
}

// 收起/展开切换
function toggleCollapse() {
  collapsed.value = !collapsed.value
  sidebarWidth.value = collapsed.value ? 0 : 280
}

// 展开（拖拽手柄点击恢复）
function expand() {
  if (collapsed.value) {
    collapsed.value = false
    sidebarWidth.value = 280
  }
}
</script>

<template>

  <aside class="sub-sidebar" :class="{ collapsed, dragging }" :style="{ width: sidebarWidth + 'px' }">
    <!-- 收起状态：窄条显示展开按钮 -->
    <div v-if="collapsed" class="sub-sidebar-collapsed-bar">
      <button class="expand-btn" @click="expand" title="展开会话列表">
        <MessageSquare :size="16" :stroke-width="1.8" />
      </button>
    </div>

    <template v-else>
      <!-- 头部搜索与操作 -->
      <div class="header-search" v-if="activeNav !== 'dashboard'">
        <Search class="search-glyph" :size="14" :stroke-width="2" />
        <input
          v-model="searchQuery"
          type="text"
          :placeholder="activeNav === 'rag' ? '搜索本地文档...' : '搜索对话历史...'"
          class="search-input"
        />
        <button class="collapse-btn" @click="toggleCollapse" title="收起侧栏">
          <X :size="14" :stroke-width="2.2" />
        </button>
      </div>

      <!-- 对话列表 -->
      <div v-if="activeNav === 'chat' || activeNav === 'aiops'" class="sidebar-list">
        <div class="list-title">
          <span class="title-label">会话</span>
          <span class="title-count">{{ conversations.length }}</span>
          <button class="new-btn" @click="handleCreate" title="新建会话">
            <Plus :size="16" :stroke-width="2.2" />
          </button>
        </div>

        <div class="items-container">
          <div
            v-for="conv in conversations"
            :key="conv.id"
            class="conv-item"
            :class="{ active: activeId === conv.id }"
            @click="emit('select', conv.id)"
          >
            <span class="item-glyph"><MessageSquare :size="14" :stroke-width="1.8" /></span>
            <span class="item-title">{{ conv.title }}</span>
            <button class="delete-item-btn" @click="handleDelete(conv.id, $event)" title="删除">
              <X :size="13" :stroke-width="2.2" />
            </button>
          </div>
        </div>
      </div>

      <!-- 指标仪表盘模式 (Dashboard) -->
      <div v-else-if="activeNav === 'dashboard'" class="sidebar-list">
        <div class="list-title">
          <span class="title-label">监控</span>
        </div>
        <div class="items-container">
          <div
            class="conv-item"
            :class="{ active: (dashboardView ?? 'overview') === 'overview' }"
            @click="emit('dashboard-view', 'overview')"
          >
            <span class="item-glyph"><BarChart3 :size="14" :stroke-width="1.8" /></span>
            <span class="item-title">核心指标监控</span>
          </div>
          <div
            class="conv-item"
            :class="{ active: dashboardView === 'line' }"
            @click="emit('dashboard-view', 'line')"
          >
            <span class="item-glyph"><TrendingUp :size="14" :stroke-width="1.8" /></span>
            <span class="item-title">实时趋势折线</span>
          </div>
          <div
            class="conv-item"
            :class="{ active: dashboardView === 'pie' }"
            @click="emit('dashboard-view', 'pie')"
          >
            <span class="item-glyph"><PieChart :size="14" :stroke-width="1.8" /></span>
            <span class="item-title">资源配比饼图</span>
          </div>
        </div>
      </div>

      <!-- 本地知识库模式 (RAG) -->
      <div v-else-if="activeNav === 'rag'" class="sidebar-list">
        <div class="list-title">
          <span class="title-label">知识手册</span>
        </div>

        <!-- 拖拽上传框 -->
        <div class="upload-box" @dragover.prevent @drop="handleDrop">
          <div v-if="!isUploading" class="upload-inner">
            <Upload class="upload-icon" :size="26" :stroke-width="1.6" />
            <span class="upload-text">拖拽文件或点击上传</span>
            <input
              type="file"
              class="file-input-hidden"
              @change="handleFileSelect"
              accept=".txt,.md,.pdf,.json"
            />
          </div>
          <div v-else class="progress-container">
            <div class="progress-bar-bg">
              <div class="progress-bar-fill" :style="{ width: uploadProgress + '%' }"></div>
            </div>
            <span class="progress-text">入库向量化中 {{ uploadProgress }}%</span>
          </div>
        </div>

        <!-- 已上传文档列表 -->
        <div class="docs-list-title">
          <Layers :size="11" :stroke-width="2" />
          <span>已同步手册</span>
          <span class="docs-count">{{ docsList.length }}</span>
        </div>
        <div class="docs-container">
          <div v-for="doc in docsList" :key="doc" class="doc-item">
            <FileText class="doc-icon" :size="14" :stroke-width="1.8" />
            <span class="doc-name">{{ doc }}</span>
            <span class="doc-badge">已向量化</span>
          </div>
        </div>
      </div>
    </template>

    <!-- 右侧拖拽调整宽度手柄（收起时不可见） -->
    <div
      v-if="!collapsed"
      class="resize-handle"
      @pointerdown="onResizeStart"
      title="拖拽调整宽度"
    ></div>
  </aside>
</template>

<style scoped>
.sub-sidebar {
  background: var(--bg-sidebar-sub);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  border-right: 1px solid var(--border-glass);
  display: flex;
  flex-direction: column;
  height: 100vh;
  flex-shrink: 0;
  overflow: hidden;
  position: relative;
  /* 收起/展开宽度过渡 */
  transition: width 0.3s var(--ease-out-expo);
}

/* 拖拽过程中禁用过渡，保证实时跟手 */
.sub-sidebar.dragging {
  transition: none;
}

/* 拖拽手柄：右侧边缘细条 */
.resize-handle {
  position: absolute;
  top: 0;
  right: -3px;
  width: 6px;
  height: 100%;
  cursor: col-resize;
  z-index: 10;
}

.resize-handle:hover {
  background: rgba(255, 255, 255, 0.06);
}

.resize-handle::after {
  content: '';
  position: absolute;
  top: 0;
  right: 2px;
  width: 2px;
  height: 100%;
  background: transparent;
  transition: background 0.2s;
}

.resize-handle:hover::after {
  background: var(--accent-bright);
  opacity: 0.5;
}

/* 收起状态：窄条 */
.sub-sidebar-collapsed-bar {
  width: 100%;
  height: 100vh;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 14px;
}

.expand-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-input);
  border: 1px solid var(--border-glass);
  color: var(--text-secondary);
  width: 34px;
  height: 34px;
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: all var(--transition-spring);
}

.expand-btn:hover {
  color: var(--accent-bright);
  border-color: var(--border-focus);
  transform: translateY(-1px);
}

/* 收起按钮（搜索栏右侧） */
.collapse-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: var(--text-muted);
  width: 26px;
  height: 26px;
  border-radius: var(--radius-md);
  cursor: pointer;
  flex-shrink: 0;
  transition: all var(--transition-fast);
}

.collapse-btn:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}
.header-search {
  padding: 18px 18px 14px;
  border-bottom: 1px solid var(--border-hairline);
  display: flex;
  align-items: center;
  gap: 10px;
}

.search-glyph {
  color: var(--text-muted);
  flex-shrink: 0;
  transition: color var(--transition-fast);
}

.search-input {
  flex: 1;
  background: transparent;
  border: none;
  color: var(--text-primary);
  font-size: 0.82rem;
  font-family: var(--font-sans);
  letter-spacing: -0.01em;
  outline: none;
}

.search-input::placeholder {
  color: var(--text-muted);
}

.header-search:focus-within .search-glyph {
  color: var(--accent-bright);
}

/* ---------- 列表 ---------- */
.sidebar-list {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 18px;
  overflow-y: auto;
}

.list-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: var(--font-display);
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.18em;
  color: var(--text-muted);
  font-weight: 600;
  margin-bottom: 14px;
}

.title-label {
  flex: 1;
}

.title-count {
  font-family: var(--font-mono);
  font-size: 0.68rem;
  color: var(--text-muted);
  background: var(--bg-active);
  padding: 1px 7px;
  border-radius: var(--radius-pill);
  letter-spacing: 0;
}

.new-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: 1px solid var(--border-glass);
  color: var(--text-secondary);
  width: 26px;
  height: 26px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all var(--transition-spring);
}

.new-btn:hover {
  background: var(--accent-soft);
  border-color: var(--border-focus);
  color: var(--accent-bright);
  transform: rotate(90deg) scale(1.05);
}

.items-container {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.conv-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: background var(--transition-fast), color var(--transition-fast),
    transform var(--transition-spring);
  color: var(--text-secondary);
  position: relative;
  border: 1px solid transparent;
}

.conv-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
  transform: translateX(2px);
}

.conv-item.active {
  background: var(--accent-soft);
  color: var(--accent-bright);
  border-color: var(--border-focus);
}

.conv-item.active::before {
  content: '';
  position: absolute;
  left: -18px;
  top: 50%;
  width: 3px;
  height: 60%;
  background: var(--accent-gradient);
  border-radius: var(--radius-pill);
  box-shadow: 0 0 8px var(--accent-glow);
  transform: translateY(-50%);
}

.item-glyph {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  opacity: 0.7;
}

.conv-item.active .item-glyph {
  opacity: 1;
}

.item-title {
  font-size: 0.82rem;
  font-weight: 450;
  letter-spacing: -0.005em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.delete-item-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: var(--text-muted);
  width: 22px;
  height: 22px;
  border-radius: var(--radius-xs);
  cursor: pointer;
  opacity: 0;
  transform: scale(0.8);
  transition: all var(--transition-spring);
}

.conv-item:hover .delete-item-btn {
  opacity: 1;
  transform: scale(1);
}

.delete-item-btn:hover {
  color: var(--text-error);
  background: rgba(255, 90, 110, 0.1);
}

/* ---------- 知识库上传 ---------- */
.upload-box {
  border: 1.5px dashed var(--border-input);
  border-radius: var(--radius-lg);
  padding: 24px 16px;
  text-align: center;
  cursor: pointer;
  transition: all var(--transition-normal);
  background: var(--bg-input);
  margin-bottom: 22px;
  position: relative;
  overflow: hidden;
}

.upload-box::before {
  content: '';
  position: absolute;
  inset: 0;
  background: var(--aurora-1);
  opacity: 0;
  transition: opacity var(--transition-normal);
  pointer-events: none;
}

.upload-box:hover {
  border-color: var(--accent-bright);
  transform: translateY(-2px);
}

.upload-box:hover::before {
  opacity: 1;
}

.upload-inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  position: relative;
}

.file-input-hidden {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  cursor: pointer;
}

.upload-icon {
  display: flex;
  color: var(--accent-bright);
}

.upload-text {
  font-size: 0.74rem;
  color: var(--text-secondary);
  letter-spacing: 0.01em;
}

.progress-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  position: relative;
}

.progress-bar-bg {
  width: 100%;
  height: 4px;
  background: var(--bg-active);
  border-radius: var(--radius-pill);
  overflow: hidden;
}

.progress-bar-fill {
  height: 100%;
  background: var(--accent-gradient);
  width: 0%;
  transition: width 0.2s var(--ease-out-expo);
}

.progress-text {
  font-size: 0.68rem;
  color: var(--text-secondary);
  font-family: var(--font-mono);
}

/* ---------- 文档列表 ---------- */
.docs-list-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.66rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.14em;
  color: var(--text-muted);
  margin-bottom: 12px;
}

.docs-count {
  margin-left: auto;
  font-family: var(--font-mono);
  background: var(--bg-active);
  padding: 1px 7px;
  border-radius: var(--radius-pill);
  letter-spacing: 0;
}

.docs-container {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.doc-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 10px;
  background: var(--bg-input);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-hairline);
  transition: all var(--transition-fast);
}

.doc-item:hover {
  border-color: var(--border-glass);
  transform: translateX(2px);
}

.doc-icon {
  flex-shrink: 0;
  color: var(--text-secondary);
}

.doc-name {
  font-size: 0.74rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  color: var(--text-primary);
}

.doc-badge {
  font-size: 0.58rem;
  background: var(--accent-soft);
  color: var(--accent-bright);
  padding: 2px 7px;
  border-radius: var(--radius-pill);
  letter-spacing: 0.04em;
  font-weight: 600;
  text-transform: uppercase;
}
</style>
