<script setup lang="ts">
import { useMarkdown } from '../../../shared/composables/useMarkdown'
import type { Message } from '../../../shared/types'

const props = defineProps<{ message: Message }>()
const { render } = useMarkdown()
</script>

<template>
  <div v-if="props.message.role === 'user'" class="msg-row user">
    <div class="msg-bubble user-bubble">
      <div class="msg-content">{{ props.message.content }}</div>
    </div>
  </div>
  <div v-else-if="props.message.role === 'assistant'" class="msg-row assistant">
    <div class="msg-bubble assistant-bubble">
      <div class="msg-content md" v-html="render(props.message.content || '')"></div>
    </div>
  </div>


</template>

<style scoped>
.msg-row {
  display: flex;
}

.msg-row.user {
  justify-content: flex-end;
}

.msg-bubble.user-bubble {
  background: var(--accent-dim);
  border: 1px solid rgba(110, 110, 247, 0.15);
  border-radius: 16px 16px 4px 16px;
  max-width: 72%;
  padding: 10px 16px;
}
.msg-bubble.assistant-bubble {
  background: var(--bg-chat-bubble);
  border: 1px solid var(--border-color);
  border-radius: 16px 16px 16px 4px;
  max-width: 82%;
  padding: 10px 16px;
}


.msg-body {
  max-width: 82%;
  position: relative;
}

.msg-content {
  font-size: 0.875rem;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

/* Markdown */
.msg-content.md {
  white-space: normal;
}

.msg-content.md :deep(h1),
.msg-content.md :deep(h2),
.msg-content.md :deep(h3),
.msg-content.md :deep(h4) {
  margin: 0.8em 0 0.4em;
  line-height: 1.35;
  font-weight: 600;
}

.msg-content.md :deep(h1) {
  font-size: 1.2rem;
}

.msg-content.md :deep(h2) {
  font-size: 1.05rem;
}

.msg-content.md :deep(h3) {
  font-size: 0.95rem;
}

.msg-content.md :deep(p) {
  margin: 0.35em 0;
}

.msg-content.md :deep(p:first-child) {
  margin-top: 0;
}

.msg-content.md :deep(p:last-child) {
  margin-bottom: 0;
}

.msg-content.md :deep(ul),
.msg-content.md :deep(ol) {
  margin: 0.35em 0;
  padding-left: 1.4em;
}

.msg-content.md :deep(li) {
  margin: 0.12em 0;
}

.msg-content.md :deep(code) {
  background: var(--bg-hover);
  border-radius: 4px;
  padding: 1px 5px;
  font-size: 0.82em;
  font-family: 'SF Mono', 'Consolas', 'Fira Code', monospace;
}

.msg-content.md :deep(pre) {
  background: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: 10px;
  padding: 14px 16px;
  margin: 0.6em 0;
  overflow-x: auto;
  line-height: 1.55;
}

.msg-content.md :deep(pre code) {
  background: none;
  border: none;
  padding: 0;
  font-size: 0.82em;
}

.msg-content.md :deep(blockquote) {
  border-left: 2px solid var(--border-focus);
  margin: 0.5em 0;
  padding: 2px 12px;
  color: var(--text-secondary);
}

.msg-content.md :deep(table) {
  border-collapse: collapse;
  margin: 0.5em 0;
  width: 100%;
}

.msg-content.md :deep(th),
.msg-content.md :deep(td) {
  border: 1px solid var(--border-color);
  padding: 6px 10px;
  font-size: 0.82em;
  text-align: left;
}

.msg-content.md :deep(th) {
  font-weight: 600;
  color: var(--text-secondary);
}

.msg-content.md :deep(hr) {
  border: none;
  border-top: 1px solid var(--border-color);
  margin: 0.8em 0;
}

.msg-content.md :deep(a) {
  color: var(--accent-light);
}

.msg-content.md :deep(strong) {
  font-weight: 600;
}

.msg-content.md :deep(.katex-display) {
  margin: 0.8em 0;
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
  background: rgba(255, 107, 107, 0.08);
  padding: 2px 6px;
  border-radius: 4px;
}
</style>

