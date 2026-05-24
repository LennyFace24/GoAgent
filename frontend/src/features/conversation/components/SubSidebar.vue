<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import type { Conversation } from '../types'

const props = defineProps<{
  activeNav: string
  activeId: string
}>()

const emit = defineEmits<{
  'select': [id: string]
  'created': [id: string]
}>()

const conversations = ref<Conversation[]>([])
const searchQuery = ref('')
const isUploading = ref(false)
const uploadProgress = ref(0)
const docsList = ref<string[]>([
  'RAG 运维手册.md',
  'Prometheus指标查询规范.txt',
  '故障排查黄金定律.pdf'
])

async function fetchConvs() {
  try {
    const res = await fetch('/api/conversations')
    if (res.ok) {
      const data = await res.json()
      conversations.value = data.conversations || []
    }
  } catch {
    // Mock 模式数据 fallback
    conversations.value = [
      { id: 'conv_default', title: '全系统健康一键诊断', updated_at: new Date().toISOString() },
      { id: 'conv_1', title: '服务器 CPU 异常告警排查', updated_at: new Date().toISOString() },
      { id: 'conv_2', title: 'ChromaDB 语义向量库检索异常', updated_at: new Date().toISOString() }
    ]
  }
}

async function createNewConv() {
  const title = props.activeNav === 'aiops' ? '新诊断任务' : '新会话对话'
  try {
    const res = await fetch('/api/conversation', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title })
    })
    if (res.ok) {
      const data = await res.json()
      await fetchConvs()
      emit('created', data.id)
    }
  } catch {
    const newId = 'conv_' + Date.now()
    conversations.value.unshift({ id: newId, title, updated_at: new Date().toISOString() })
    emit('created', newId)
  }
}

async function deleteConv(id: string, event: Event) {
  event.stopPropagation()
  try {
    await fetch('/api/conversation/' + id, { method: 'DELETE' })
    await fetchConvs()
    if (props.activeId === id && conversations.value.length > 0) {
      emit('select', conversations.value[0].id)
    }
  } catch {
    conversations.value = conversations.value.filter(c => c.id !== id)
    if (props.activeId === id && conversations.value.length > 0) {
      emit('select', conversations.value[0].id)
    }
  }
}

// 拖拽上传 RAG 文件
function handleDrop(e: DragEvent) {
  e.preventDefault()
  const files = e.dataTransfer?.files
  if (files && files.length > 0) {
    uploadFile(files[0])
  }
}

function handleFileSelect(e: Event) {
  const target = e.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    uploadFile(target.files[0])
  }
}

function uploadFile(file: File) {
  isUploading.value = true
  uploadProgress.value = 10
  
  const interval = setInterval(() => {
    uploadProgress.value += 15
    if (uploadProgress.value >= 100) {
      clearInterval(interval)
      isUploading.value = false
      docsList.value.unshift(file.name)
    }
  }, 300)
}

onMounted(() => {
  fetchConvs()
})

watch(() => props.activeNav, () => {
  fetchConvs()
})
</script>

<template>
  <div class="sub-sidebar">
    <!-- 头部搜索与操作 -->
    <div class="header-search">
      <input 
        v-model="searchQuery"
        type="text" 
        :placeholder="activeNav === 'rag' ? '搜索本地文档...' : '搜索对话历史...'" 
        class="search-input"
      />
    </div>

    <!-- 常规对话 / AIOps 诊断会话列表 -->
    <div v-if="activeNav === 'chat' || activeNav === 'aiops'" class="sidebar-list">
      <div class="list-title">
        <span>{{ activeNav === 'aiops' ? '诊断任务' : '会话列表' }}</span>
        <button class="new-btn" @click="createNewConv" title="新建会话">+</button>
      </div>
      
      <div class="items-container">
        <div 
          v-for="conv in conversations" 
          :key="conv.id"
          class="conv-item"
          :class="{ active: activeId === conv.id }"
          @click="emit('select', conv.id)"
        >
          <span class="item-icon">💬</span>
          <span class="item-title">{{ conv.title }}</span>
          <button class="delete-item-btn" @click="deleteConv(conv.id, )">×</button>
        </div>
      </div>
    </div>

    <!-- 本地知识库模式 (RAG) -->
    <div v-else-if="activeNav === 'rag'" class="sidebar-list">
      <div class="list-title">
        <span>知识手册</span>
      </div>
      
      <!-- 拖拽上传框 -->
      <div 
        class="upload-box"
        @dragover.prevent
        @drop="handleDrop"
      >
        <div v-if="!isUploading" class="upload-inner">
          <span class="upload-icon">📤</span>
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
      <div class="docs-list-title">已同步手册 ({{ docsList.length }})</div>
      <div class="docs-container">
        <div v-for="doc in docsList" :key="doc" class="doc-item">
          <span class="doc-icon">📄</span>
          <span class="doc-name">{{ doc }}</span>
          <span class="doc-badge">已向量化</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.sub-sidebar {
  width: var(--sidebar-sub-w);
  background: var(--bg-sidebar-sub);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  height: 100vh;
  flex-shrink: 0;
}

.header-search {
  padding: 16px;
  border-bottom: 1px solid var(--border-color);
}

.search-input {
  width: 100%;
  padding: 8px 12px;
  background: var(--bg-input);
  border: 1px solid var(--border-input);
  border-radius: var(--radius-md);
  color: var(--text-primary);
  font-size: 0.8rem;
  transition: var(--transition-fast);
}

.search-input:focus {
  outline: none;
  border-color: var(--accent);
}

.sidebar-list {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 16px;
  overflow-y: auto;
}

.list-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  font-weight: 700;
  margin-bottom: 12px;
}

.new-btn {
  background: transparent;
  border: none;
  color: var(--text-secondary);
  font-size: 1.2rem;
  cursor: pointer;
  line-height: 1;
}

.new-btn:hover {
  color: var(--accent);
}

.items-container {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.conv-item {
  display: flex;
  align-items: center;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: var(--transition-fast);
  color: var(--text-secondary);
  position: relative;
}

.conv-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.conv-item.active {
  background: var(--bg-active);
  color: var(--accent);
  font-weight: 500;
}

.item-icon {
  margin-right: 8px;
  font-size: 0.95rem;
}

.item-title {
  font-size: 0.82rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.delete-item-btn {
  display: none;
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 1rem;
  cursor: pointer;
}

.conv-item:hover .delete-item-btn {
  display: block;
}

.delete-item-btn:hover {
  color: var(--text-error);
}

/* 知识库上传框 */
.upload-box {
  border: 2px dashed var(--border-color);
  border-radius: var(--radius-lg);
  padding: 20px;
  text-align: center;
  cursor: pointer;
  transition: var(--transition-fast);
  background: var(--bg-input);
  margin-bottom: 20px;
}

.upload-box:hover {
  border-color: var(--accent);
}

.upload-inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  position: relative;
}

.file-input-hidden {
  position: absolute;
  top: 0; left: 0; width: 100%; height: 100%;
  opacity: 0;
  cursor: pointer;
}

.upload-icon {
  font-size: 1.8rem;
  margin-bottom: 8px;
}

.upload-text {
  font-size: 0.72rem;
  color: var(--text-secondary);
}

.progress-container {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.progress-bar-bg {
  width: 100%;
  height: 6px;
  background: var(--border-color);
  border-radius: 3px;
  overflow: hidden;
  margin-bottom: 8px;
}

.progress-bar-fill {
  height: 100%;
  background: var(--accent);
  width: 0%;
  transition: width 0.1s ease;
}

.progress-text {
  font-size: 0.68rem;
  color: var(--text-secondary);
}

/* 知识库文档列表 */
.docs-list-title {
  font-size: 0.7rem;
  font-weight: 700;
  color: var(--text-muted);
  margin-bottom: 10px;
}

.docs-container {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.doc-item {
  display: flex;
  align-items: center;
  padding: 8px;
  background: var(--bg-input);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

.doc-icon {
  font-size: 0.9rem;
  margin-right: 8px;
}

.doc-name {
  font-size: 0.75rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.doc-badge {
  font-size: 0.6rem;
  background: var(--accent-dim);
  color: var(--accent);
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  zoom: 0.9;
}
</style>