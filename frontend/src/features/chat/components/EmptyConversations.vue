<script setup lang="ts">
import { MessageSquarePlus } from 'lucide-vue-next'
</script>

<template>
  <div class="empty-conversation">
    <div class="empty-glow" aria-hidden="true"></div>
    <div class="empty-content">
      <div class="empty-icon-wrap">
        <MessageSquarePlus :size="40" :stroke-width="1.2" />
      </div>
      <p class="empty-eyebrow">NO CONVERSATION</p>
      <h2 class="empty-title">尚未开启<br />任何对话</h2>
      <p class="empty-hint">点击左侧 + 新建会话，开启你的运维之旅</p>
    </div>
  </div>
</template>

<style scoped>
.empty-conversation {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  position: relative;
  overflow: hidden;
}

.empty-glow {
  position: absolute;
  width: 480px;
  height: 480px;
  border-radius: 50%;
  background: radial-gradient(circle, var(--accent-glow) 0%, transparent 60%);
  filter: blur(40px);
  opacity: 0.5;
  animation: glow-drift 8s infinite var(--ease-in-out-quint);
  pointer-events: none;
}

@keyframes glow-drift {
  0%,
  100% {
    transform: translate(0, 0) scale(1);
  }
  50% {
    transform: translate(20px, -20px) scale(1.1);
  }
}

.empty-content {
  text-align: center;
  color: var(--text-secondary);
  position: relative;
  z-index: 1;
}

.empty-icon-wrap {
  display: inline-flex;
  color: var(--accent-bright);
  margin-bottom: 28px;
  opacity: 0.7;
  filter: drop-shadow(0 0 24px var(--accent-glow));
  animation: icon-float 4s infinite var(--ease-in-out-quint);
}

@keyframes icon-float {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-8px);
  }
}

.empty-eyebrow {
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.4em;
  color: var(--text-muted);
  margin-bottom: 14px;
  font-weight: 500;
}

.empty-title {
  font-family: var(--font-display);
  font-size: 3.2rem;
  font-weight: 600;
  letter-spacing: -0.04em;
  line-height: 0.95;
  color: var(--text-display);
  margin-bottom: 18px;
  background: linear-gradient(180deg, var(--text-heading) 0%, var(--text-secondary) 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.empty-hint {
  font-size: 0.85rem;
  opacity: 0.7;
  letter-spacing: 0.01em;
}
</style>
