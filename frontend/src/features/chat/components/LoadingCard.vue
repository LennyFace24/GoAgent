<script setup lang="ts">
import { Loader2 } from 'lucide-vue-next'
</script>

<template>
  <div class="loading-card">
    <div class="loading-bubble">
      <Loader2 :size="18" class="spin-icon" />
      <span class="loading-text">AI 正在思考...</span>
    </div>
  </div>
</template>

<style scoped>
.loading-card {
  display: flex;
}

.loading-bubble {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--bg-chat-bubble);
  border: 1px solid var(--border-color);
  border-radius: 16px 16px 16px 4px;
  padding: 10px 16px;
}

.spin-icon {
  color: var(--accent);
  animation: spin 1s linear infinite;
}

.loading-text {
  font-size: 0.875rem;
  color: var(--text-secondary);
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
