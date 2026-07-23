<script setup lang="ts">
import { useMarkdown } from '../../../shared/composables/useMarkdown'
import type { Message } from '../../../shared/types'
import { Sparkles, User } from 'lucide-vue-next'

const props = defineProps<{ message: Message }>()
const { render } = useMarkdown()
</script>

<template>
  <div v-if="props.message.role === 'user'" class="msg-row user">
    <div class="msg-bubble user-bubble">
      <div class="msg-content">{{ props.message.content }}</div>
    </div>
    <div class="avatar user-avatar">
      <User :size="14" :stroke-width="2.2" />
    </div>
  </div>
  <div v-else-if="props.message.role === 'assistant'" class="msg-row assistant">
    <div class="avatar assistant-avatar">
      <Sparkles :size="14" :stroke-width="2.2" />
    </div>
    <div class="msg-bubble assistant-bubble">
      <div class="msg-content md" v-html="render(props.message.content || '')"></div>
    </div>
  </div>
</template>

<style scoped>
.msg-row {
  display: flex;
  gap: 10px;
  align-items: flex-end;
}

.msg-row.user {
  justify-content: flex-end;
}

.avatar {
  width: 28px;
  height: 28px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: var(--shadow-sm);
}

.user-avatar {
  background: linear-gradient(135deg, #2a2a3a, #1a1a28);
  color: var(--text-secondary);
  border: 1px solid var(--border-glass);
}

.assistant-avatar {
  background: var(--accent-gradient);
  color: #07070b;
  box-shadow: 0 0 16px var(--accent-glow);
}

.msg-bubble {
  padding: 12px 16px;
  position: relative;
}

.msg-bubble.user-bubble {
  background: var(--bg-chat-bubble-user);
  border: 1px solid var(--border-focus);
  border-radius: var(--radius-xl) var(--radius-xl) var(--radius-xs) var(--radius-xl);
  max-width: 72%;
  backdrop-filter: var(--glass-blur-lite);
  -webkit-backdrop-filter: var(--glass-blur-lite);
}

.msg-bubble.assistant-bubble {
  background: var(--bg-chat-bubble);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-xl) var(--radius-xl) var(--radius-xl) var(--radius-xs);
  max-width: 82%;
  backdrop-filter: var(--glass-blur-lite);
  -webkit-backdrop-filter: var(--glass-blur-lite);
  box-shadow: var(--shadow-inner);
}

.msg-content {
  font-size: 0.875rem;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
  letter-spacing: -0.005em;
  color: var(--text-primary);
}

/* ---------- Markdown ---------- */
.msg-content.md {
  white-space: normal;
}

.msg-content.md :deep(h1),
.msg-content.md :deep(h2),
.msg-content.md :deep(h3),
.msg-content.md :deep(h4) {
  margin: 0.9em 0 0.45em;
  line-height: 1.3;
  font-weight: 600;
  font-family: var(--font-display);
  letter-spacing: -0.02em;
  color: var(--text-heading);
}

.msg-content.md :deep(h1) {
  font-size: 1.25rem;
}
.msg-content.md :deep(h2) {
  font-size: 1.08rem;
}
.msg-content.md :deep(h3) {
  font-size: 0.96rem;
}

.msg-content.md :deep(p) {
  margin: 0.4em 0;
}
.msg-content.md :deep(p:first-child) {
  margin-top: 0;
}
.msg-content.md :deep(p:last-child) {
  margin-bottom: 0;
}

.msg-content.md :deep(ul),
.msg-content.md :deep(ol) {
  margin: 0.4em 0;
  padding-left: 1.5em;
}
.msg-content.md :deep(li) {
  margin: 0.14em 0;
}

.msg-content.md :deep(strong) {
  font-weight: 650;
  color: var(--text-heading);
}

.msg-content.md :deep(code) {
  background: var(--bg-active);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-xs);
  padding: 1px 6px;
  font-size: 0.82em;
  font-family: var(--font-mono);
  color: var(--accent-bright);
}

.msg-content.md :deep(pre) {
  background: rgba(7, 7, 11, 0.6);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-md);
  padding: 14px 16px;
  margin: 0.7em 0;
  overflow-x: auto;
  line-height: 1.6;
}

.msg-content.md :deep(pre code) {
  background: none;
  border: none;
  padding: 0;
  font-size: 0.82em;
  color: var(--text-primary);
}

.msg-content.md :deep(blockquote) {
  border-left: 2px solid var(--accent-bright);
  margin: 0.6em 0;
  padding: 4px 14px;
  color: var(--text-secondary);
  background: var(--accent-dim);
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
}

.msg-content.md :deep(table) {
  border-collapse: collapse;
  margin: 0.6em 0;
  width: 100%;
  font-size: 0.84em;
}

.msg-content.md :deep(th),
.msg-content.md :deep(td) {
  border: 1px solid var(--border-color);
  padding: 7px 11px;
  text-align: left;
}

.msg-content.md :deep(th) {
  font-weight: 600;
  color: var(--text-heading);
  background: var(--bg-active);
}

.msg-content.md :deep(hr) {
  border: none;
  border-top: 1px solid var(--border-color);
  margin: 0.9em 0;
}

.msg-content.md :deep(a) {
  color: var(--accent-bright);
  text-decoration: none;
  border-bottom: 1px solid transparent;
  transition: border-color var(--transition-fast);
}

.msg-content.md :deep(a:hover) {
  border-bottom-color: var(--accent-bright);
}

.msg-content.md :deep(.katex-display) {
  margin: 0.9em 0;
  overflow-x: auto;
  overflow-y: hidden;
  padding: 4px 0;
}

.msg-content.md :deep(.katex) {
  font-size: 1.05em;
}

.msg-content.md :deep(.latex-error) {
  color: var(--text-error);
  font-size: 0.82em;
  background: rgba(255, 90, 110, 0.08);
  padding: 2px 6px;
  border-radius: var(--radius-xs);
}
</style>
