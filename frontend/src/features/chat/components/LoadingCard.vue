<script setup lang="ts">
import { Sparkles } from 'lucide-vue-next'
</script>

<template>
  <div class="loading-card">
    <div class="avatar assistant-avatar">
      <Sparkles :size="14" :stroke-width="2.2" />
    </div>
    <div class="loading-bubble">
      <span class="dot"></span>
      <span class="dot"></span>
      <span class="dot"></span>
      <span class="loading-text">AI 正在思考</span>
    </div>
  </div>
</template>

<style scoped>
.loading-card {
  display: flex;
  gap: 10px;
  align-items: flex-end;
}

.avatar {
  width: 28px;
  height: 28px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  background: var(--accent-gradient);
  color: #07070b;
  box-shadow: 0 0 16px var(--accent-glow);
}

.loading-bubble {
  display: flex;
  align-items: center;
  gap: 5px;
  background: var(--bg-chat-bubble);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-xl) var(--radius-xl) var(--radius-xl) var(--radius-xs);
  padding: 12px 16px;
  backdrop-filter: var(--glass-blur-lite);
  -webkit-backdrop-filter: var(--glass-blur-lite);
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent-bright);
  animation: dot-bounce 1.2s infinite var(--ease-in-out-quint);
}

.dot:nth-child(2) {
  animation-delay: 0.15s;
}
.dot:nth-child(3) {
  animation-delay: 0.3s;
}

@keyframes dot-bounce {
  0%,
  60%,
  100% {
    transform: translateY(0);
    opacity: 0.4;
  }
  30% {
    transform: translateY(-5px);
    opacity: 1;
  }
}

.loading-text {
  font-size: 0.8rem;
  color: var(--text-secondary);
  margin-left: 6px;
  letter-spacing: 0.01em;
}
</style>
