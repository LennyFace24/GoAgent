<script setup lang="ts">
import { ref } from 'vue'
import { MessageSquare, BookOpen, BarChart3, Sun, Moon, Sparkles } from 'lucide-vue-next'

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
    root.style.setProperty('--bg-body', '#07070b')
    root.style.setProperty('--bg-canvas', '#0a0a10')
    root.style.setProperty('--bg-sidebar-mini', 'rgba(14, 14, 22, 0.72)')
    root.style.setProperty('--bg-sidebar-sub', 'rgba(18, 18, 28, 0.68)')
    root.style.setProperty('--bg-chat-bubble', 'rgba(24, 24, 36, 0.66)')
    root.style.setProperty('--text-primary', '#ecedf2')
    root.style.setProperty('--text-heading', '#ffffff')
    root.style.setProperty('--border-color', 'rgba(255, 255, 255, 0.06)')
  } else {
    root.style.setProperty('--bg-body', '#f5f6fa')
    root.style.setProperty('--bg-canvas', '#eef0f6')
    root.style.setProperty('--bg-sidebar-mini', 'rgba(255, 255, 255, 0.72)')
    root.style.setProperty('--bg-sidebar-sub', 'rgba(255, 255, 255, 0.82)')
    root.style.setProperty('--bg-chat-bubble', 'rgba(255, 255, 255, 0.85)')
    root.style.setProperty('--text-primary', '#18181f')
    root.style.setProperty('--text-heading', '#000000')
    root.style.setProperty('--border-color', 'rgba(0, 0, 0, 0.07)')
  }
}
</script>

<template>
  <aside class="slim-sidebar">
    <!-- 品牌 Logo -->
    <div class="logo-area">
      <div class="logo-core">
        <Sparkles :size="16" :stroke-width="2.4" />
      </div>
    </div>

    <nav class="nav-icons">
      <button
        class="nav-btn"
        :class="{ active: activeNav === 'chat' || activeNav === 'aiops' }"
        @click="emit('update:activeNav', 'chat')"
        title="智能对话"
      >
        <MessageSquare class="btn-icon" :size="20" :stroke-width="1.75" />
        <span class="btn-tooltip">智能对话</span>
        <span class="active-rail"></span>
      </button>

      <button
        class="nav-btn"
        :class="{ active: activeNav === 'rag' }"
        @click="emit('update:activeNav', 'rag')"
        title="本地知识库 (RAG)"
      >
        <BookOpen class="btn-icon" :size="20" :stroke-width="1.75" />
        <span class="btn-tooltip">知识手册</span>
        <span class="active-rail"></span>
      </button>

      <button
        class="nav-btn"
        :class="{ active: activeNav === 'dashboard' }"
        @click="emit('update:activeNav', 'dashboard')"
        title="核心监控 (Dashboard)"
      >
        <BarChart3 class="btn-icon" :size="20" :stroke-width="1.75" />
        <span class="btn-tooltip">核心指标</span>
        <span class="active-rail"></span>
      </button>
    </nav>

    <div class="footer-icons">
      <button class="nav-btn theme-toggle" @click="toggleTheme" title="切换深浅主题">
        <Moon v-if="isDark" class="btn-icon" :size="20" :stroke-width="1.75" />
        <Sun v-else class="btn-icon" :size="20" :stroke-width="1.75" />
        <span class="btn-tooltip">{{ isDark ? '浅色模式' : '深色模式' }}</span>
      </button>
    </div>
  </aside>
</template>

<style scoped>
.slim-sidebar {
  width: var(--sidebar-mini-w);
  background: var(--bg-sidebar-mini);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  border-right: 1px solid var(--border-glass);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 24px 0;
  height: 100vh;
  flex-shrink: 0;
  position: relative;
}

.slim-sidebar::before {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  width: 1px;
  height: 100%;
  background: linear-gradient(
    180deg,
    transparent 0%,
    var(--accent-glow) 50%,
    transparent 100%
  );
  opacity: 0.4;
}

/* ---------- Logo ---------- */
.logo-area {
  margin-bottom: 36px;
  position: relative;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.logo-core {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  background: var(--accent-gradient);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #07070b;
  box-shadow: 0 0 24px var(--accent-glow), var(--shadow-inner);
  position: relative;
  z-index: 2;
}

/* ---------- Nav ---------- */
.nav-icons {
  display: flex;
  flex-direction: column;
  gap: 14px;
  flex: 1;
}

.nav-btn {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-lg);
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  transition: transform var(--transition-spring), background var(--transition-normal),
    color var(--transition-normal), border-color var(--transition-normal);
}

.nav-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
  transform: translateY(-2px) scale(1.04);
  border-color: var(--border-glass);
}

.nav-btn:active {
  transform: translateY(0) scale(0.98);
}

.nav-btn.active {
  background: var(--accent-soft);
  color: var(--accent-bright);
  border-color: var(--border-focus);
  box-shadow: 0 0 16px rgba(0, 255, 157, 0.12);
}

.btn-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform var(--transition-spring);
}

.nav-btn:hover .btn-icon {
  transform: scale(1.08);
}

/* 弹簧式活动指示器 */
.active-rail {
  position: absolute;
  left: -14px;
  top: 50%;
  width: 3px;
  height: 0;
  border-radius: var(--radius-pill);
  background: var(--accent-gradient);
  box-shadow: 0 0 12px var(--accent-glow);
  transform: translateY(-50%);
  transition: height var(--transition-spring);
}

.nav-btn.active .active-rail {
  height: 24px;
}

/* ---------- Tooltip ---------- */
.btn-tooltip {
  position: absolute;
  left: calc(100% + 14px);
  background: var(--bg-elevated);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  color: var(--text-primary);
  border: 1px solid var(--border-glass);
  padding: 6px 12px;
  border-radius: var(--radius-md);
  font-size: 0.74rem;
  font-weight: 500;
  letter-spacing: 0.01em;
  box-shadow: var(--shadow-md);
  opacity: 0;
  pointer-events: none;
  transform: translateX(-8px);
  transition: opacity var(--transition-normal), transform var(--transition-spring);
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
