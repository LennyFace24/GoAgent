<script setup lang="ts">
import { ref } from 'vue'
import { MessageSquare, BookOpen, BarChart3, Sun, Moon } from 'lucide-vue-next'

const props = defineProps<{
  activeNav: string
}>()

const emit = defineEmits<{
  'update:activeNav': [value: string]
}>()

const isDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)

function toggleTheme() {
  isDark.value = !isDark.value
  const root = document.documentElement
  if (isDark.value) {
    root.style.setProperty('--bg-body', '#101014')
    root.style.setProperty('--bg-sidebar-mini', '#14141d')
    root.style.setProperty('--bg-sidebar-sub', '#181822')
    root.style.setProperty('--bg-chat-bubble', '#1e1e2d')
    root.style.setProperty('--text-primary', '#e5e5eb')
    root.style.setProperty('--border-color', 'rgba(255, 255, 255, 0.07)')
  } else {
    root.style.setProperty('--bg-body', '#f4f5f8')
    root.style.setProperty('--bg-sidebar-mini', '#eef1f6')
    root.style.setProperty('--bg-sidebar-sub', '#ffffff')
    root.style.setProperty('--bg-chat-bubble', '#ffffff')
    root.style.setProperty('--text-primary', '#1c1c1e')
    root.style.setProperty('--border-color', 'rgba(0, 0, 0, 0.06)')
  }
}
</script>

<template>
  <div class="slim-sidebar">
    <div class="logo-area">
      <div class="logo-pulse"></div>
    </div>
    
    <div class="nav-icons">
      <button
        class="nav-btn"
        :class="{ active: activeNav === 'chat' || activeNav === 'aiops' }"
        @click="emit('update:activeNav', 'chat')"
        title="智能对话"
      >
        <MessageSquare class="btn-icon" :size="22" />
        <span class="btn-tooltip">智能对话</span>
      </button>

      <button
        class="nav-btn"
        :class="{ active: activeNav === 'rag' }"
        @click="emit('update:activeNav', 'rag')"
        title="本地知识库管理 (RAG)"
      >
        <BookOpen class="btn-icon" :size="22" />
        <span class="btn-tooltip">知识手册</span>
      </button>

      <button
        class="nav-btn"
        :class="{ active: activeNav === 'dashboard' }"
        @click="emit('update:activeNav', 'dashboard')"
        title="核心监控指标 (Dashboard)"
      >
        <BarChart3 class="btn-icon" :size="22" />
        <span class="btn-tooltip">核心指标</span>
      </button>
    </div>

    <div class="footer-icons">
      <button class="nav-btn theme-toggle" @click="toggleTheme" title="切换深浅主题">
        <Moon v-if="isDark" class="btn-icon" :size="22" />
        <Sun v-else class="btn-icon" :size="22" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.slim-sidebar {
  width: var(--sidebar-mini-w);
  background: var(--bg-sidebar-mini);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px 0;
  height: 100vh;
  flex-shrink: 0;
}

.logo-area {
  margin-bottom: 30px;
}
.logo-pulse {
  width: 24px; height: 24px;
  border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 10px var(--accent);
  position: relative;
}
.logo-pulse::after {
  content: '';
  position: absolute; width: 100%; height: 100%;
  border: 2px solid var(--accent);
  border-radius: 50%;
  animation: pulse 2s infinite ease-out;
}

@keyframes pulse {
  0% { transform: scale(1); opacity: 0.6; }
  100% { transform: scale(1.8); opacity: 0; }
}

.nav-icons {
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
}

.nav-btn {
  width: 44px; height: 44px;
  border-radius: var(--radius-lg);
  border: none;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  transition: var(--transition-normal);
}

.nav-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
  transform: translateY(-1px);
}

.nav-btn.active {
  background: var(--accent-dim);
  color: var(--accent);
}

.btn-icon {
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 气泡悬浮提示 */
.btn-tooltip {
  position: absolute;
  left: 60px;
  background: var(--bg-sidebar-sub);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
  padding: 6px 12px;
  border-radius: var(--radius-md);
  font-size: 0.78rem;
  font-weight: 500;
  box-shadow: var(--shadow-md);
  opacity: 0;
  pointer-events: none;
  transform: translateX(-10px);
  transition: var(--transition-normal);
  white-space: nowrap;
  z-index: 999;
}

.nav-btn:hover .btn-tooltip {
  opacity: 1;
  transform: translateX(0);
}

.footer-icons {
  margin-top: auto;
}
</style>