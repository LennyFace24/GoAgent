<script setup>
import { ref, nextTick, onMounted, watch } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { latexExtension } from '../latex.js'

marked.use(latexExtension)

const props = defineProps({
  mode: { type: String, required: true },
  conversationId: { type: String, default: 'default' },
})
const emit = defineEmits(['update:mode', 'conversationCreated'])

const modes = [
  { key: 'chat_stream', label: 'Chat Stream' },
  { key: 'ai_ops', label: 'AI Ops' },
]

function setMode(key) {
  emit('update:mode', key)
}

const messages = ref([])
const input = ref('')
const sending = ref(false)
const msgBox = ref(null)

function renderMd(text) {
  const raw = marked.parse(text, { breaks: true, gfm: true })
  return DOMPurify.sanitize(raw, {
    ADD_TAGS: ['math', 'semantics', 'mrow', 'mi', 'mo', 'mn', 'msup', 'msub', 'mfrac',
      'msqrt', 'mroot', 'mstyle', 'munder', 'mover', 'munderover', 'mspace',
      'mpadded', 'menclose', 'annotation', 'annotation-xml',
      'mglyph', 'maligngroup', 'malignmark', 'mtable', 'mtr', 'mtd', 'mlabeledtr'],
    ADD_ATTR: ['mathvariant', 'mathsize', 'mathcolor', 'mathbackground',
      'scriptsizemultiplier', 'scriptminsize', 'accent', 'accentunder',
      'form', 'fence', 'separator', 'stretchy', 'symmetric', 'largeop',
      'movablelimits', 'data-latex'],
  })
}

function scrollBottom() {
  nextTick(() => {
    if (msgBox.value) msgBox.value.scrollTop = msgBox.value.scrollHeight
  })
}

function addMsg(role, content, streaming = false) {
  messages.value.push({ id: Date.now() + Math.random(), role, content, streaming })
  scrollBottom()
}

function appendLast(content) {
  const last = messages.value[messages.value.length - 1]
  if (last) { last.content += content; scrollBottom() }
}

function finishLast() {
  const last = messages.value[messages.value.length - 1]
  if (last) last.streaming = false
}

async function send() {
  const text = input.value.trim()
  if (!text || sending.value) return
  input.value = ''
  sending.value = true
  addMsg('user', text)

  await sendSSE(text)

  sending.value = false
  scrollBottom()
}

async function sendSSE(text) {
  const endpoint = props.mode === 'ai_ops' ? '/ai_ops' : '/chat_stream'
  const assistantMsg = { id: Date.now() + Math.random(), role: 'assistant', content: '', streaming: true }
  messages.value.push(assistantMsg)
  scrollBottom()

  let aborted = false

  try {
    const res = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: text, conversation_id: props.conversationId }),
    })

    if (!res.ok) {
      finishLast()
      appendLast(`\n\n[HTTP ${res.status}]`)
      return
    }

    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      const lines = buffer.split('\n')
      buffer = lines.pop()

      for (const line of lines) {
        if (!line.startsWith('data:')) continue
        const payload = line.slice(5).trim()
        if (!payload || payload === '"done"' || payload === 'done') continue

        try {
          const data = JSON.parse(payload)
          if (data.content) {
            appendLast(data.content)
          } else if (data.error) {
            appendLast(`\n\n[错误: ${data.error}]`)
          }
        } catch {
          // ignore unparseable SSE events
        }
      }
    }
  } catch (e) {
    appendLast(`\n\n[连接断开: ${e.message}]`)
  } finally {
    finishLast()
  }
}

function onKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

const placeholders = {
  chat_stream: '输入消息，Enter 发送，Shift+Enter 换行',
  ai_ops: '描述故障现象，如：API 响应时间从 200ms 飙升到 3s，错误率 12%',
}

async function loadHistory() {
  messages.value = []
  try {
    const res = await fetch(`/conversation/${props.conversationId}`)
    if (!res.ok) return
    const data = await res.json()
    const lines = data.messages || []
    for (const line of lines) {
      if (line.role === 'user' || line.role === 'assistant') {
        messages.value.push({
          id: Date.now() + Math.random(),
          role: line.role,
          content: line.content,
          streaming: false,
        })
      }
    }
    scrollBottom()
  } catch { /* ignore */ }
}

onMounted(() => { loadHistory() })
watch(() => props.conversationId, () => { loadHistory() })
</script>

<template>
  <div class="chat-container">
    <!-- Messages -->
    <div class="msg-area" ref="msgBox">
      <div v-if="messages.length === 0" class="empty-state">
        <div class="empty-text">发送一条消息开始对话</div>
      </div>

      <div
        v-for="msg in messages" :key="msg.id"
        :class="['msg-row', msg.role]"
      >
        <div class="msg-bubble">
          <div v-if="msg.role === 'assistant'" class="msg-content md" v-html="renderMd(msg.content)"></div>
          <div v-else class="msg-content">{{ msg.content }}</div>
          <span v-if="msg.streaming" class="typing-cursor">|</span>
        </div>
      </div>
    </div>

    <!-- Mode selector + Input -->
    <div class="bottom-bar">
      <div class="mode-selector">
        <button
          v-for="m in modes" :key="m.key"
          :class="['mode-pill', { active: mode === m.key }]"
          @click="setMode(m.key)"
        >{{ m.label }}</button>
      </div>
      <div class="input-row">
        <textarea
          v-model="input"
          :placeholder="placeholders[mode] || '输入消息...'"
          rows="1"
          :disabled="sending"
          @keydown="onKeydown"
        ></textarea>
        <button class="send-btn" :disabled="!input.trim() || sending" @click="send">
          <svg v-if="!sending" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
          <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10" stroke-dasharray="32" stroke-dashoffset="12"><animateTransform attributeName="transform" type="rotate" values="0 12 12;360 12 12" dur="0.8s" repeatCount="indefinite"/></circle></svg>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chat-container {
  flex: 1; display: flex; flex-direction: column;
  min-width: 0; height: 100vh;
}

/* ---- Messages ---- */
.msg-area {
  flex: 1; overflow-y: auto; padding: 32px 48px;
  display: flex; flex-direction: column; gap: 20px;
  scrollbar-width: thin;
  scrollbar-color: rgba(128,128,160,0.15) transparent;
}
.msg-area::-webkit-scrollbar { width: 5px; }
.msg-area::-webkit-scrollbar-track { background: transparent; }
.msg-area::-webkit-scrollbar-thumb {
  background: rgba(128,128,160,0.15); border-radius: 3px;
}
.msg-area::-webkit-scrollbar-thumb:hover {
  background: rgba(128,128,160,0.3);
}

.empty-state {
  margin: auto; text-align: center;
}
.empty-text {
  font-size: 0.82rem; color: var(--text-muted); letter-spacing: 0.02em;
}

/* ---- Message bubbles ---- */
.msg-row { display: flex; }
.msg-row.user { justify-content: flex-end; }

.msg-row.user .msg-bubble {
  background: var(--bubble-user-bg); border: 1px solid var(--bubble-user-border);
  border-radius: 18px 18px 4px 18px; max-width: 72%;
}
.msg-row.assistant .msg-bubble {
  background: var(--bubble-assistant-bg); border: 1px solid var(--bubble-assistant-border);
  border-radius: 18px 18px 18px 4px; max-width: 82%;
}
.msg-row.error .msg-bubble {
  background: var(--bubble-error-bg); border: 1px solid var(--bubble-error-border);
  border-radius: 14px; max-width: 82%; color: var(--text-error);
}

.msg-bubble { padding: 10px 16px; position: relative; }

.msg-content {
  font-size: 0.875rem; line-height: 1.6; white-space: pre-wrap;
  word-break: break-word;
}

/* ---- Markdown ---- */
.msg-content.md { white-space: normal; }
.msg-content.md :deep(h1),
.msg-content.md :deep(h2),
.msg-content.md :deep(h3),
.msg-content.md :deep(h4) {
  margin: 0.8em 0 0.4em; line-height: 1.35; font-weight: 600;
}
.msg-content.md :deep(h1) { font-size: 1.2rem; }
.msg-content.md :deep(h2) { font-size: 1.05rem; }
.msg-content.md :deep(h3) { font-size: 0.95rem; }
.msg-content.md :deep(p) { margin: 0.35em 0; }
.msg-content.md :deep(p:first-child) { margin-top: 0; }
.msg-content.md :deep(p:last-child) { margin-bottom: 0; }
.msg-content.md :deep(ul),
.msg-content.md :deep(ol) { margin: 0.35em 0; padding-left: 1.4em; }
.msg-content.md :deep(li) { margin: 0.12em 0; }
.msg-content.md :deep(code) {
  background: var(--bg-hover); border-radius: 4px;
  padding: 1px 5px; font-size: 0.82em;
  font-family: 'SF Mono', 'Consolas', 'Fira Code', monospace;
}
.msg-content.md :deep(pre) {
  background: var(--bg-input); border: 1px solid var(--border-color);
  border-radius: 10px; padding: 14px 16px; margin: 0.6em 0;
  overflow-x: auto; line-height: 1.55;
}
.msg-content.md :deep(pre code) {
  background: none; border: none; padding: 0; font-size: 0.82em;
}
.msg-content.md :deep(blockquote) {
  border-left: 2px solid var(--border-focus); margin: 0.5em 0;
  padding: 2px 12px; color: var(--text-secondary);
}
.msg-content.md :deep(table) {
  border-collapse: collapse; margin: 0.5em 0; width: 100%;
}
.msg-content.md :deep(th),
.msg-content.md :deep(td) {
  border: 1px solid var(--border-color); padding: 6px 10px;
  font-size: 0.82em; text-align: left;
}
.msg-content.md :deep(th) { font-weight: 600; color: var(--text-secondary); }
.msg-content.md :deep(hr) {
  border: none; border-top: 1px solid var(--border-color); margin: 0.8em 0;
}
.msg-content.md :deep(a) { color: var(--accent-light); }
.msg-content.md :deep(strong) { font-weight: 600; }
.msg-content.md :deep(.katex-display) {
  margin: 0.8em 0; overflow-x: auto; overflow-y: hidden; padding: 4px 0;
}
.msg-content.md :deep(.katex) { font-size: 1.05em; }
.msg-content.md :deep(.latex-error) {
  color: var(--text-error); font-size: 0.82em;
  background: var(--bubble-error-bg); padding: 2px 6px; border-radius: 4px;
}

.typing-cursor {
  display: inline; color: var(--accent);
  animation: blink 0.6s steps(1) infinite;
}
@keyframes blink { 50% { opacity: 0; } }

/* ---- Bottom bar ---- */
.bottom-bar {
  padding: 10px 32px 16px;
  border-top: 1px solid var(--border-color);
}

.mode-selector {
  display: flex; gap: 0; margin-bottom: 10px;
  border-bottom: 1px solid var(--border-color);
}
.mode-pill {
  background: none; border: none; border-bottom: 2px solid transparent;
  color: var(--text-muted); padding: 6px 14px 8px;
  font-size: 0.75rem; font-weight: 500; cursor: pointer;
  transition: all 0.15s ease; white-space: nowrap;
}
.mode-pill:hover { color: var(--text-secondary); }
.mode-pill.active {
  color: var(--text-primary); border-bottom-color: var(--accent);
}

.input-row {
  display: flex; gap: 8px; align-items: flex-end;
}
.input-row textarea {
  flex: 1; background: var(--bg-input); border: 1px solid var(--border-input);
  border-radius: 12px; color: var(--text-primary); padding: 10px 14px;
  font-size: 0.875rem; outline: none; resize: none;
  font-family: inherit; line-height: 1.5;
  min-height: 42px; max-height: 120px;
  transition: border-color 0.15s ease;
}
.input-row textarea:focus { border-color: var(--border-focus); }
.input-row textarea::placeholder { color: var(--text-muted); }

.send-btn {
  width: 42px; height: 42px; border-radius: 10px;
  background: var(--accent); border: none; color: #fff;
  cursor: pointer; display: flex; align-items: center; justify-content: center;
  flex-shrink: 0; transition: all 0.15s ease;
}
.send-btn:hover { opacity: 0.85; }
.send-btn:disabled { opacity: 0.25; cursor: not-allowed; }
</style>
