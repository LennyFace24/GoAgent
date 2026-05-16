<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import ChatView from './components/ChatView.vue'
import type { Conversation, SearchResult } from './types'

const activeMode = ref<string>('chat_stream')
const activeConversationId = ref<string>('default')

// ---- Conversations ----
const conversations = ref<Conversation[]>([])

async function loadConversations(): Promise<void> {
  try {
    const res = await fetch('/conversations')
    if (!res.ok) return
    const data = await res.json()
    conversations.value = data.conversations || []
    if (conversations.value.length === 0) {
      await createConversation()
    } else if (!conversations.value.find((c: Conversation) => c.id === activeConversationId.value)) {
      activeConversationId.value = conversations.value[0].id
    }
  } catch { /* ignore */ }
}

async function createConversation(): Promise<void> {
  try {
    const res = await fetch('/conversation', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: '新对话' }),
    })
    if (!res.ok) return
    const data = await res.json()
    if (data.conversation) {
      conversations.value.unshift(data.conversation)
      activeConversationId.value = data.conversation.id
    }
  } catch { /* ignore */ }
}

function selectConversation(id: string): void {
  activeConversationId.value = id
}

async function deleteConversation(id: string): Promise<void> {
  try {
    await fetch(`/conversation/${id}`, { method: 'DELETE' })
    conversations.value = conversations.value.filter((c: Conversation) => c.id !== id)
    if (activeConversationId.value === id) {
      activeConversationId.value = conversations.value.length > 0 ? conversations.value[0].id : 'default'
    }
  } catch { /* ignore */ }
}

onMounted(() => { loadConversations() })

// ---- Sidebar resize ----
const SIDEBAR_DEFAULT = 280
const SIDEBAR_MIN = 200
const SIDEBAR_MAX = 420
const COLLAPSE_THRESHOLD = 60

const sidebarWidth = ref<number>(SIDEBAR_DEFAULT)
const collapsed = ref<boolean>(false)
const dragging = ref<boolean>(false)
let startX = 0
let startW = 0

function onDragStart(e: MouseEvent): void {
  e.preventDefault()
  dragging.value = true
  startX = e.clientX
  startW = sidebarWidth.value
  document.addEventListener('mousemove', onDragMove)
  document.addEventListener('mouseup', onDragEnd)
}

function onDragMove(e: MouseEvent): void {
  const delta = e.clientX - startX
  const newW = startW + delta
  if (newW < COLLAPSE_THRESHOLD) {
    collapsed.value = true
    sidebarWidth.value = 0
  } else {
    collapsed.value = false
    sidebarWidth.value = Math.max(SIDEBAR_MIN, Math.min(SIDEBAR_MAX, newW))
  }
}

function onDragEnd(): void {
  dragging.value = false
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup', onDragEnd)
}

function toggleSidebar(): void {
  if (collapsed.value) {
    collapsed.value = false
    sidebarWidth.value = SIDEBAR_DEFAULT
  } else {
    collapsed.value = true
  }
}

onUnmounted(() => {
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup', onDragEnd)
})

// ---- File upload ----
const selectedFile = ref<File | null>(null)
const uploadMsg = ref<string>('')
const uploading = ref<boolean>(false)

function onFileChange(e: Event): void {
  const target = e.target as HTMLInputElement
  selectedFile.value = target.files?.[0] || null
  uploadMsg.value = ''
}

async function uploadFile(): Promise<void> {
  if (!selectedFile.value || uploading.value) return
  uploading.value = true
  uploadMsg.value = ''
  try {
    const form = new FormData()
    form.append('file', selectedFile.value)
    const res = await fetch('/upload_file', { method: 'POST', body: form })
    const data = await res.json()
    uploadMsg.value = data.error ? `错误: ${data.error}` : `已上传: ${data.file}`
  } catch (e) {
    uploadMsg.value = `失败: ${e instanceof Error ? e.message : String(e)}`
  } finally {
    uploading.value = false
  }
}

// ---- Search ----
const searchQuery = ref<string>('')
const searchResults = ref<SearchResult[]>([])
const searching = ref<boolean>(false)

async function doSearch(): Promise<void> {
  if (!searchQuery.value.trim() || searching.value) return
  searching.value = true
  searchResults.value = []
  try {
    const res = await fetch('/search', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query: searchQuery.value }),
    })
    const data = await res.json()
    searchResults.value = data.results || []
  } catch (e) {
    searchResults.value = [{ error: e instanceof Error ? e.message : String(e) } as SearchResult]
  } finally {
    searching.value = false
  }
}
</script>

<template>
  <div class="app-layout" :class="{ dragging }">
    <!-- Sidebar -->
    <aside
      class="sidebar"
      :class="{ collapsed }"
      :style="{ width: collapsed ? 0 : sidebarWidth + 'px' }"
    >
      <div class="sidebar-inner">
        <div class="sidebar-brand">goagent</div>

        <!-- Conversations -->
        <div class="sidebar-section">
          <button class="new-chat-btn" @click="createConversation">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
            新对话
          </button>
          <div class="conv-list">
            <div
              v-for="conv in conversations" :key="conv.id"
              :class="['conv-item', { active: activeConversationId === conv.id }]"
              @click="selectConversation(conv.id)"
            >
              <span class="conv-title">{{ conv.title || '新对话' }}</span>
              <button class="conv-del" @click.stop="deleteConversation(conv.id)" title="删除">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              </button>
            </div>
          </div>
        </div>

        <div class="section-divider"></div>

        <!-- File tools -->
        <div class="sidebar-section">
          <div class="section-title">文件</div>
          <label class="file-drop-zone">
            <input type="file" accept=".txt,.md,.json,.csv" @change="onFileChange" />
            <span v-if="!selectedFile">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
              选择文件
            </span>
            <span v-else class="file-selected">{{ selectedFile.name }}</span>
          </label>
          <button class="sidebar-btn" :disabled="!selectedFile || uploading" @click="uploadFile">
            {{ uploading ? '上传中...' : '上传' }}
          </button>
          <div class="upload-msg" v-if="uploadMsg">{{ uploadMsg }}</div>
        </div>

        <div class="section-divider"></div>

        <!-- Search -->
        <div class="sidebar-section">
          <div class="section-title">搜索</div>
          <div class="search-input-wrap">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
            <input class="sidebar-input" v-model="searchQuery" placeholder="搜索文档..."
              @keydown.enter="doSearch" />
          </div>
          <div class="search-results" v-if="searchResults.length">
            <div v-for="(r, i) in searchResults" :key="i" class="search-item">
              <div class="search-text">{{ typeof r === 'string' ? r : (r.content || r.text || JSON.stringify(r)) }}</div>
            </div>
          </div>
        </div>

        <div class="sidebar-footer">v0.1</div>
      </div>

      <!-- Drag handle -->
      <div class="drag-handle" @mousedown="onDragStart"></div>
    </aside>

    <!-- Expand button (when collapsed) -->
    <button v-if="collapsed" class="expand-btn" @click="toggleSidebar">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg>
    </button>

    <!-- Main chat area -->
    <ChatView v-model:mode="activeMode" :conversationId="activeConversationId" @conversationCreated="loadConversations" />
  </div>
</template>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }

:root {
  --bg-body: #111113;
  --bg-sidebar: #161618;
  --bg-input-area: #111113;
  --bg-input: #1c1c1e;
  --bg-hover: rgba(255,255,255,0.04);
  --bg-active: rgba(255,255,255,0.06);
  --border-color: rgba(255,255,255,0.06);
  --border-input: rgba(255,255,255,0.08);
  --border-focus: rgba(255,255,255,0.15);
  --text-primary: #e8e8e8;
  --text-secondary: #8e8e93;
  --text-muted: #636366;
  --text-heading: #f5f5f5;
  --text-error: #ff6b6b;
  --accent: #6e6ef7;
  --accent-dim: rgba(110,110,247,0.1);
  --accent-light: #8e8ef7;
  --bubble-user-bg: rgba(110,110,247,0.08);
  --bubble-user-border: rgba(110,110,247,0.15);
  --bubble-assistant-bg: rgba(255,255,255,0.03);
  --bubble-assistant-border: rgba(255,255,255,0.06);
  --bubble-error-bg: rgba(255,107,107,0.08);
  --bubble-error-border: rgba(255,107,107,0.15);
}

@media (prefers-color-scheme: light) {
  :root {
    --bg-body: #ffffff;
    --bg-sidebar: #f9f9fb;
    --bg-input-area: #ffffff;
    --bg-input: #f2f2f7;
    --bg-hover: rgba(0,0,0,0.03);
    --bg-active: rgba(0,0,0,0.05);
    --border-color: rgba(0,0,0,0.06);
    --border-input: rgba(0,0,0,0.08);
    --border-focus: rgba(0,0,0,0.2);
    --text-primary: #1c1c1e;
    --text-secondary: #8e8e93;
    --text-muted: #aeaeb2;
    --text-heading: #000;
    --text-error: #ff3b30;
    --accent: #5856d6;
    --accent-dim: rgba(88,86,214,0.08);
    --accent-light: #5856d6;
    --bubble-user-bg: rgba(88,86,214,0.06);
    --bubble-user-border: rgba(88,86,214,0.12);
    --bubble-assistant-bg: rgba(0,0,0,0.02);
    --bubble-assistant-border: rgba(0,0,0,0.05);
    --bubble-error-bg: rgba(255,59,48,0.06);
    --bubble-error-border: rgba(255,59,48,0.12);
  }
}

body {
  font-family: -apple-system, 'SF Pro Text', 'Segoe UI', system-ui, sans-serif;
  background: var(--bg-body); color: var(--text-primary);
  height: 100vh; overflow: hidden;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}
#app { height: 100%; }
</style>

<style scoped>
.app-layout { display: flex; height: 100vh; position: relative; }
.app-layout.dragging { cursor: col-resize; user-select: none; }
.app-layout.dragging * { pointer-events: none; }

/* ---- Sidebar ---- */
.sidebar {
  flex-shrink: 0;
  background: var(--bg-sidebar);
  display: flex;
  position: relative;
  overflow: hidden;
  transition: width 0.2s ease;
  border-right: 1px solid var(--border-color);
}
.sidebar.collapsed { width: 0 !important; border-right: none; }
.sidebar-inner {
  width: 100%; min-width: 0;
  display: flex; flex-direction: column;
  padding: 20px 16px; overflow-y: auto;
}

.sidebar-brand {
  font-size: 0.82rem; font-weight: 500; color: var(--text-muted);
  margin-bottom: 24px; white-space: nowrap;
  letter-spacing: 0.08em; text-transform: lowercase;
}

.sidebar-section { margin-bottom: 16px; }
.section-title {
  font-size: 0.68rem; font-weight: 500; text-transform: uppercase;
  letter-spacing: 0.06em; color: var(--text-muted); margin-bottom: 10px;
}
.section-divider {
  height: 1px; background: var(--border-color); margin: 4px 0 16px;
}

/* ---- New chat button ---- */
.new-chat-btn {
  width: 100%; background: none; border: 1px solid var(--border-input);
  color: var(--text-secondary); padding: 8px 14px; border-radius: 8px;
  font-size: 0.8rem; cursor: pointer; margin-bottom: 12px;
  transition: all 0.15s ease;
  display: flex; align-items: center; justify-content: center; gap: 6px;
}
.new-chat-btn:hover { border-color: var(--border-focus); color: var(--text-primary); background: var(--bg-hover); }

/* ---- Conversation list ---- */
.conv-list { max-height: 280px; overflow-y: auto; }
.conv-item {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 10px 8px 12px; border-radius: 6px; cursor: pointer;
  font-size: 0.8rem; color: var(--text-secondary);
  transition: all 0.12s ease; margin-bottom: 1px;
  border-left: 2px solid transparent;
}
.conv-item:hover { background: var(--bg-hover); }
.conv-item.active {
  background: var(--bg-active); color: var(--text-primary);
  border-left-color: var(--accent);
}
.conv-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.conv-del {
  background: none; border: none; color: var(--text-muted); cursor: pointer;
  padding: 2px; line-height: 1; opacity: 0; border-radius: 4px;
  transition: opacity 0.12s, color 0.12s;
  display: flex; align-items: center; justify-content: center;
}
.conv-item:hover .conv-del { opacity: 1; }
.conv-del:hover { color: var(--text-error); }

/* ---- File upload ---- */
.file-drop-zone {
  display: flex; align-items: center; justify-content: center; gap: 6px;
  padding: 12px; border: 1px dashed var(--border-input); border-radius: 8px;
  cursor: pointer; font-size: 0.78rem; color: var(--text-muted);
  transition: all 0.15s ease; text-align: center;
}
.file-drop-zone:hover { border-color: var(--border-focus); color: var(--text-secondary); }
.file-drop-zone input { display: none; }
.file-selected { color: var(--accent-light); font-size: 0.75rem; word-break: break-all; }

.sidebar-btn {
  width: 100%; background: none; border: 1px solid var(--border-input);
  color: var(--text-secondary); padding: 7px 14px; border-radius: 8px;
  font-size: 0.78rem; cursor: pointer; margin-top: 8px;
  transition: all 0.15s ease;
}
.sidebar-btn:hover { border-color: var(--border-focus); color: var(--text-primary); }
.sidebar-btn:disabled { opacity: 0.3; cursor: not-allowed; }

.upload-msg { font-size: 0.72rem; color: var(--text-muted); margin-top: 8px; }

/* ---- Search ---- */
.search-input-wrap {
  position: relative; display: flex; align-items: center;
}
.search-input-wrap svg {
  position: absolute; left: 10px; color: var(--text-muted); pointer-events: none;
}
.sidebar-input {
  width: 100%; background: var(--bg-input); border: 1px solid var(--border-input);
  border-radius: 8px; color: var(--text-primary); padding: 8px 10px 8px 32px;
  font-size: 0.8rem; outline: none;
  transition: border-color 0.15s ease;
}
.sidebar-input:focus { border-color: var(--border-focus); }
.sidebar-input::placeholder { color: var(--text-muted); }

.search-results { margin-top: 10px; max-height: 240px; overflow-y: auto; }
.search-item {
  padding: 8px 10px; margin-bottom: 4px; border-radius: 6px;
  background: var(--bg-input); border: 1px solid var(--border-color);
}
.search-text {
  font-size: 0.75rem; color: var(--text-secondary);
  max-height: 60px; overflow: hidden; text-overflow: ellipsis;
  display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical;
}

.sidebar-footer {
  margin-top: auto; font-size: 0.68rem; color: var(--text-muted);
  text-align: center; padding-top: 16px; white-space: nowrap;
}

/* ---- Drag handle ---- */
.drag-handle {
  position: absolute; top: 0; right: -3px;
  width: 6px; height: 100%; cursor: col-resize;
  z-index: 10;
}
.drag-handle:hover,
.app-layout.dragging .drag-handle {
  background: var(--accent); opacity: 0.2;
  border-radius: 2px;
}

/* ---- Expand button ---- */
.expand-btn {
  position: absolute; top: 14px; left: 8px; z-index: 20;
  width: 28px; height: 28px; border-radius: 6px;
  background: var(--bg-sidebar); border: 1px solid var(--border-color);
  color: var(--text-muted); cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: all 0.15s ease;
}
.expand-btn:hover { background: var(--bg-hover); color: var(--text-primary); }
</style>
