<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import ChatView from './components/ChatView.vue'

const activeMode = ref('chat_stream')
const activeConversationId = ref('default')

// ---- Conversations ----
const conversations = ref([])

async function loadConversations() {
  try {
    const res = await fetch('/conversations')
    console.log('loadConversations status:', res.status)
    if (!res.ok) {
      console.error('loadConversations failed:', await res.text())
      return
    }
    const data = await res.json()
    console.log('loadConversations response:', data)
    conversations.value = data.conversations || []
    if (conversations.value.length === 0) {
      await createConversation()
    } else if (!conversations.value.find(c => c.id === activeConversationId.value)) {
      activeConversationId.value = conversations.value[0].id
    }
  } catch (e) {
    console.error('loadConversations error:', e)
  }
}

async function createConversation() {
  try {
    const res = await fetch('/conversation', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: '新对话' }),
    })
    console.log('createConversation status:', res.status)
    if (!res.ok) {
      console.error('createConversation failed:', await res.text())
      return
    }
    const data = await res.json()
    console.log('createConversation response:', data)
    if (data.conversation) {
      conversations.value.unshift(data.conversation)
      activeConversationId.value = data.conversation.id
    }
  } catch (e) {
    console.error('createConversation error:', e)
  }
}

function selectConversation(id) {
  activeConversationId.value = id
}

async function deleteConversation(id) {
  try {
    await fetch(`/conversation/${id}`, { method: 'DELETE' })
    conversations.value = conversations.value.filter(c => c.id !== id)
    if (activeConversationId.value === id) {
      activeConversationId.value = conversations.value.length > 0 ? conversations.value[0].id : 'default'
    }
  } catch { /* ignore */ }
}

onMounted(() => { loadConversations() })

// ---- Sidebar resize ----
const SIDEBAR_DEFAULT = 260
const SIDEBAR_MIN = 180
const SIDEBAR_MAX = 400
const COLLAPSE_THRESHOLD = 60

const sidebarWidth = ref(SIDEBAR_DEFAULT)
const collapsed = ref(false)
const dragging = ref(false)
let startX = 0
let startW = 0

function onDragStart(e) {
  e.preventDefault()
  dragging.value = true
  startX = e.clientX
  startW = sidebarWidth.value
  document.addEventListener('mousemove', onDragMove)
  document.addEventListener('mouseup', onDragEnd)
}

function onDragMove(e) {
  const delta = e.clientX - startX
  let newW = startW + delta
  if (newW < COLLAPSE_THRESHOLD) {
    collapsed.value = true
    sidebarWidth.value = 0
  } else {
    collapsed.value = false
    sidebarWidth.value = Math.max(SIDEBAR_MIN, Math.min(SIDEBAR_MAX, newW))
  }
}

function onDragEnd() {
  dragging.value = false
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup', onDragEnd)
}

function toggleSidebar() {
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
const selectedFile = ref(null)
const uploadMsg = ref('')
const uploading = ref(false)

async function uploadFile() {
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
    uploadMsg.value = `失败: ${e.message}`
  } finally {
    uploading.value = false
  }
}

// ---- Search ----
const searchQuery = ref('')
const searchResults = ref([])
const searching = ref(false)

async function doSearch() {
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
    searchResults.value = [{ error: e.message }]
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
        <div class="sidebar-brand">GoAgent</div>

        <!-- Conversations -->
        <div class="sidebar-section">
          <button class="new-chat-btn" @click="createConversation">+ 新对话</button>
          <div class="conv-list">
            <div
              v-for="conv in conversations" :key="conv.id"
              :class="['conv-item', { active: activeConversationId === conv.id }]"
              @click="selectConversation(conv.id)"
            >
              <span class="conv-title">{{ conv.title || '新对话' }}</span>
              <button class="conv-del" @click.stop="deleteConversation(conv.id)" title="删除">&times;</button>
            </div>
          </div>
        </div>

        <!-- File tools -->
        <div class="sidebar-section">
          <div class="section-title">文件</div>
          <label class="file-label">
            <input type="file" accept=".txt,.md,.json,.csv"
              @change="e => selectedFile = e.target.files[0]" />
          </label>
          <div class="file-name" v-if="selectedFile">{{ selectedFile.name }}</div>
          <button class="sidebar-btn" :disabled="!selectedFile || uploading" @click="uploadFile">
            {{ uploading ? '上传中...' : '上传' }}
          </button>
          <div class="upload-msg" v-if="uploadMsg">{{ uploadMsg }}</div>
        </div>

        <div class="sidebar-section">
          <div class="section-title">搜索</div>
          <input class="sidebar-input" v-model="searchQuery" placeholder="搜索文档..."
            @keydown.enter="doSearch" />
          <button class="sidebar-btn" :disabled="!searchQuery.trim() || searching" @click="doSearch">
            {{ searching ? '搜索中...' : '搜索' }}
          </button>
          <div class="search-results" v-if="searchResults.length">
            <div v-for="(r, i) in searchResults" :key="i" class="search-item">
              <div class="search-idx">#{{ i + 1 }}</div>
              <div class="search-text">{{ typeof r === 'string' ? r : JSON.stringify(r, null, 2) }}</div>
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
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg>
    </button>

    <!-- Main chat area -->
    <ChatView v-model:mode="activeMode" :conversationId="activeConversationId" @conversationCreated="loadConversations" />
  </div>
</template>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }

:root {
  --bg-body: #0f0f13;
  --bg-sidebar: #121218;
  --bg-input-area: #0c0c14;
  --bg-input: #0a0a12;
  --bg-hover: #1a1a28;
  --bg-active: #1e2a40;
  --border-color: #1e1e28;
  --border-input: #252530;
  --border-focus: #3a4a50;
  --text-primary: #e0e0e0;
  --text-secondary: #999;
  --text-muted: #555;
  --text-heading: #f0f0f0;
  --text-error: #d4787a;
  --accent: #6c8cff;
  --accent-dim: #2a3a60;
  --accent-light: #b0c0f0;
  --bubble-user-bg: #1e3050;
  --bubble-user-border: #2a4570;
  --bubble-assistant-bg: #181820;
  --bubble-assistant-border: #262632;
  --bubble-error-bg: #281818;
  --bubble-error-border: #3a2020;
}

@media (prefers-color-scheme: light) {
  :root {
    --bg-body: #f5f5f7;
    --bg-sidebar: #eeeef2;
    --bg-input-area: #f9f9fb;
    --bg-input: #f0f0f4;
    --bg-hover: rgba(0,0,0,0.04);
    --bg-active: #dde4f8;
    --border-color: #d8d8e0;
    --border-input: #d0d0da;
    --border-focus: #8899cc;
    --text-primary: #1a1a24;
    --text-secondary: #555;
    --text-muted: #888;
    --text-heading: #111;
    --text-error: #c44;
    --accent: #4466cc;
    --accent-dim: #dde4f8;
    --accent-light: #3355aa;
    --bubble-user-bg: #dde4f8;
    --bubble-user-border: #c0ccf0;
    --bubble-assistant-bg: #f0f0f4;
    --bubble-assistant-border: #e0e0e8;
    --bubble-error-bg: #fce8e8;
    --bubble-error-border: #f4c8c8;
  }
}

body {
  font-family: 'Segoe UI', system-ui, -apple-system, sans-serif;
  background: var(--bg-body); color: var(--text-primary);
  height: 100vh; overflow: hidden;
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
}
.sidebar.collapsed { width: 0 !important; }
.sidebar-inner {
  width: 100%; min-width: 0;
  display: flex; flex-direction: column;
  padding: 16px; overflow-y: auto;
}

.sidebar-brand { font-size: 1.1rem; font-weight: 700; color: var(--text-heading); margin-bottom: 20px; white-space: nowrap; }
.sidebar-section { margin-bottom: 18px; }
.section-title { font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.06em; color: var(--text-muted); margin-bottom: 8px; }

.file-label { display: block; margin-bottom: 6px; }
.file-label input { width: 100%; font-size: 0.75rem; color: var(--text-secondary); }
.file-name { font-size: 0.75rem; color: var(--accent); margin-bottom: 6px; word-break: break-all; }

.sidebar-btn {
  width: 100%; background: var(--bg-input); border: 1px solid var(--border-input);
  color: var(--text-secondary); padding: 6px 14px; border-radius: 6px;
  font-size: 0.8rem; cursor: pointer; margin-top: 6px;
  transition: background 0.12s;
}
.sidebar-btn:hover { background: var(--bg-hover); }
.sidebar-btn:disabled { opacity: 0.4; cursor: not-allowed; }

.upload-msg { font-size: 0.75rem; color: var(--text-muted); margin-top: 6px; }

.sidebar-input {
  width: 100%; background: var(--bg-input); border: 1px solid var(--border-input);
  border-radius: 6px; color: var(--text-primary); padding: 7px 10px;
  font-size: 0.8rem; outline: none; margin-bottom: 2px;
}
.sidebar-input:focus { border-color: var(--border-focus); }

.search-results { margin-top: 8px; max-height: 200px; overflow-y: auto; }
.search-item { display: flex; gap: 6px; margin-bottom: 6px; }
.search-idx { font-size: 0.7rem; color: var(--accent); min-width: 18px; padding-top: 4px; }
.search-text {
  font-size: 0.72rem; color: var(--text-secondary); background: var(--bg-input);
  border-radius: 4px; padding: 6px 8px; flex: 1;
  max-height: 80px; overflow-y: auto; white-space: pre-wrap;
}

.sidebar-footer {
  margin-top: auto; font-size: 0.7rem; color: var(--text-muted);
  text-align: center; padding-top: 12px; white-space: nowrap;
}

.new-chat-btn {
  width: 100%; background: var(--accent-dim); border: 1px solid var(--accent);
  color: var(--accent-light); padding: 7px 14px; border-radius: 8px;
  font-size: 0.8rem; cursor: pointer; margin-bottom: 10px;
  transition: background 0.12s;
}
.new-chat-btn:hover { background: var(--accent); color: #fff; }

.conv-list { max-height: 240px; overflow-y: auto; }
.conv-item {
  display: flex; align-items: center; justify-content: space-between;
  padding: 7px 10px; border-radius: 6px; cursor: pointer;
  font-size: 0.78rem; color: var(--text-secondary);
  transition: background 0.12s; margin-bottom: 2px;
}
.conv-item:hover { background: var(--bg-hover); }
.conv-item.active { background: var(--bg-active); color: var(--accent); }
.conv-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.conv-del {
  background: none; border: none; color: var(--text-muted); cursor: pointer;
  font-size: 1rem; padding: 0 4px; line-height: 1; opacity: 0;
  transition: opacity 0.12s, color 0.12s;
}
.conv-item:hover .conv-del { opacity: 1; }
.conv-del:hover { color: var(--text-error); }

/* ---- Drag handle ---- */
.drag-handle {
  position: absolute; top: 0; right: -4px;
  width: 8px; height: 100%; cursor: col-resize;
  z-index: 10;
}
.drag-handle:hover,
.app-layout.dragging .drag-handle {
  background: var(--accent); opacity: 0.3;
  border-radius: 2px;
}

/* ---- Expand button ---- */
.expand-btn {
  position: absolute; top: 12px; left: 8px; z-index: 20;
  width: 28px; height: 28px; border-radius: 6px;
  background: var(--bg-sidebar); border: 1px solid var(--border-color);
  color: var(--text-secondary); cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: background 0.12s;
}
.expand-btn:hover { background: var(--bg-hover); color: var(--text-primary); }
</style>
