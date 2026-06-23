<template>
  <div class="empty-conversation">
    <div class="empty-content">
      <svg class="empty-icon" xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
      </svg>
      <p class="empty-title">暂无对话</p>
      <p class="empty-hint">点击左侧 + 新建会话</p>
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
}
.empty-content {
  text-align: center;
  color: var(--text-muted);
}
.empty-icon {
  margin-bottom: 16px;
  opacity: 0.4;
}
.empty-title {
  font-size: 1.2rem;
  margin-bottom: 8px;
}
.empty-hint {
  font-size: 0.85rem;
  opacity: 0.6;
}
</style>
