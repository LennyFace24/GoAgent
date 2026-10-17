<script setup lang="ts">
import { ref, onMounted } from 'vue'

import { useConversations } from './features/conversation/composables/useConversations'

import EmptyConversations from './features/chat/components/EmptyConversations.vue'
import SlimSidebar from './features/conversation/components/SlimSidebar.vue'
import SubSidebar from './features/conversation/components/SubSidebar.vue'
import ChatWorkspace from './features/chat/components/ChatWorkspace.vue'
import DashboardView from './features/tools/components/DashboardView.vue'
import MonitorDrawer from './features/tools/components/MonitorDrawer.vue'

// 1. Cherry Studio 多层布局状态管理
const activeNav = ref<string>('chat')
const activeConversationId = ref<string>('default')
const dashboardView = ref<string>('overview')

function onSelect(id: string): void {
  activeConversationId.value = id
}

function onCreated(id: string): void {
  activeConversationId.value = id
}

function onDashboardViewChange(view: string): void {
  dashboardView.value = view
}

onMounted(async () => {
  const { load, create } = useConversations()
  const id = await create()
  if (id) {
    activeConversationId.value = id
  }
  await load()
})
</script>

<template>
  <div class="app-layout">
    <!-- 颗粒纹理叠层 (SVG noise) -->
    <div class="grain-overlay" aria-hidden="true"></div>
    <!-- 顶部渐隐遮罩，强化景深 -->
    <div class="vignette" aria-hidden="true"></div>

    <!-- 第一栏：极窄主功能菜单栏 (Mini Sidebar) -->
    <SlimSidebar v-model:activeNav="activeNav" />

    <!-- 第二栏：对话或文档管理侧边栏 (Sub Sidebar) -->
    <SubSidebar
      :activeNav="activeNav"
      :activeId="activeConversationId"
      :dashboardView="dashboardView"
      @select="onSelect"
      @created="onCreated"
      @dashboard-view="onDashboardViewChange"
    />

    <!-- 第三栏：智能问答主视窗 (Workspace) / 监控仪表盘 -->
    <keep-alive :include="['ChatWorkspace', 'DashboardView']">
      <ChatWorkspace
        v-if="activeNav !== 'dashboard' && activeConversationId !== ''"
        :conversationId="activeConversationId"
      />
      <EmptyConversations v-else-if="activeNav !== 'dashboard' && activeConversationId === ''" />
      <DashboardView v-else :view="dashboardView" />
    </keep-alive>

    <!-- 第四栏：实时系统状态（常驻右侧） -->
    <MonitorDrawer />

  </div>
</template>

<style>
@import './assets/styles/theme.css';
</style>

<style scoped>
.app-layout {
  display: flex;
  height: 100vh;
  width: 100vw;
  overflow: hidden;
  background: var(--bg-body);
  position: relative;
}

/* 颗粒纹理：SVG fractal noise，赋予数字画布以胶片质感 */
.grain-overlay {
  position: fixed;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  opacity: 0.035;
  mix-blend-mode: overlay;
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='200' height='200'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2' stitchTiles='stitch'/></filter><rect width='100%25' height='100%25' filter='url(%23n)'/></svg>");
  background-size: 200px 200px;
}

/* 暗角，强化中心焦点景深 */
.vignette {
  position: fixed;
  inset: 0;
  z-index: 2;
  pointer-events: none;
  background: radial-gradient(ellipse 120% 100% at 50% 50%, transparent 40%, rgba(0, 0, 0, 0.55) 100%);
}

/* 子层级全部抬升到画布之上 */
.app-layout > :not(.grain-overlay):not(.vignette) {
  position: relative;
  z-index: 3;
}
</style>
