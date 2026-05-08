<script setup>
import { ref, nextTick, onMounted, watch } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

const props = defineProps({
  mode: { type: String, required: true },
  conversationId: { type: String, default: 'default' },
})
const emit = defineEmits(['update:mode', 'conversationCreated'])

const modes = [
  { key: 'chat_stream', label: 'Chat Stream' },
  { key: 'chat', label: 'Chat' },
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
  return DOMPurify.sanitize(raw)
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

  if (props.mode === 'chat') {
    await sendNonStream(text)
  } else {
    await sendSSE(text)
  }

  sending.value = false
  scrollBottom()
}

async function sendNonStream(text) {
  try {
    const res = await fetch('/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: text, conversation_id: props.conversationId }),
    })
    if (!res.ok) {
      addMsg('error', `HTTP ${res.status}: ${await res.text()}`)
      return
    }
    const data = await res.json()
    if (data.error) {
      addMsg('error', data.error)
    } else {
      addMsg('assistant', data.reply || '(空回复)')
    }
  } catch (e) {
    addMsg('error', `请求失败: ${e.message}`)
  }
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
  chat: '输入消息，Enter 发送，Shift+Enter 换行',
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
        <div class="empty-icon">&#9741;</div>
        <div class="empty-text">
          当前模式: <strong>{{ mode }}</strong><br />
          发送一条消息开始对话
        </div>
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
        rows="2"
        :disabled="sending"
        @keydown="onKeydown"
      ></textarea>
        <button class="send-btn" :disabled="!input.trim() || sending" @click="send">
          <span v-if="sending">...</span>
          <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
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

.msg-area {
  flex: 1; overflow-y: auto; padding: 20px 24px;
  display: flex; flex-direction: column; gap: 16px;

  /* Firefox */
  scrollbar-width: thin;
  scrollbar-color: rgba(128,128,160,0.25) transparent;
}
/* WebKit (Chrome / Edge / Safari) */
.msg-area::-webkit-scrollbar { width: 6px; }
.msg-area::-webkit-scrollbar-track { background: transparent; }
.msg-area::-webkit-scrollbar-thumb {
  background: rgba(128,128,160,0.25);
  border-radius: 3px;
}
.msg-area::-webkit-scrollbar-thumb:hover {
  background: rgba(128,128,160,0.45);
}

.empty-state { margin: auto; text-align: center; color: var(--text-muted); }
.empty-icon { font-size: 2.5rem; margin-bottom: 12px; opacity: 0.4; }
.empty-text { font-size: 0.85rem; line-height: 1.6; }
.empty-text strong { color: var(--text-secondary); }

.msg-row { display: flex; }

.msg-row.user { justify-content: flex-end; }
.msg-row.user .msg-bubble {
  background: var(--bubble-user-bg); border: 1px solid var(--bubble-user-border);
  border-radius: 16px 16px 4px 16px; max-width: 75%;
}
.msg-row.assistant .msg-bubble {
  background: var(--bubble-assistant-bg); border: 1px solid var(--bubble-assistant-border);
  border-radius: 16px 16px 16px 4px; max-width: 85%;
}
.msg-row.error .msg-bubble {
  background: var(--bubble-error-bg); border: 1px solid var(--bubble-error-border);
  border-radius: 12px; max-width: 85%; color: var(--text-error);
}

.msg-bubble { padding: 10px 16px; position: relative; }

.msg-content {
  font-size: 0.88rem; line-height: 1.55; white-space: pre-wrap;
  word-break: break-word;
}

/* ---- markdown styles ---- */
.msg-content.md {
  white-space: normal;
}
.msg-content.md :deep(h1),
.msg-content.md :deep(h2),
.msg-content.md :deep(h3),
.msg-content.md :deep(h4) {
  margin: 0.8em 0 0.4em; line-height: 1.3;
}
.msg-content.md :deep(h1) { font-size: 1.25rem; }
.msg-content.md :deep(h2) { font-size: 1.1rem; }
.msg-content.md :deep(h3) { font-size: 1rem; }
.msg-content.md :deep(p) { margin: 0.4em 0; }
.msg-content.md :deep(p:first-child) { margin-top: 0; }
.msg-content.md :deep(p:last-child) { margin-bottom: 0; }
.msg-content.md :deep(ul),
.msg-content.md :deep(ol) { margin: 0.4em 0; padding-left: 1.5em; }
.msg-content.md :deep(li) { margin: 0.15em 0; }
.msg-content.md :deep(code) {
  background: var(--bg-input); border: 1px solid var(--border-input);
  border-radius: 4px; padding: 1px 5px; font-size: 0.82em;
  font-family: 'Consolas', 'Fira Code', monospace;
}
.msg-content.md :deep(pre) {
  background: var(--bg-input); border: 1px solid var(--border-input);
  border-radius: 8px; padding: 12px 14px; margin: 0.5em 0;
  overflow-x: auto; line-height: 1.5;
}
.msg-content.md :deep(pre code) {
  background: none; border: none; padding: 0; font-size: 0.82em;
}
.msg-content.md :deep(blockquote) {
  border-left: 3px solid var(--accent); margin: 0.5em 0;
  padding: 4px 12px; color: var(--text-secondary);
}
.msg-content.md :deep(table) {
  border-collapse: collapse; margin: 0.5em 0; width: 100%;
}
.msg-content.md :deep(th),
.msg-content.md :deep(td) {
  border: 1px solid var(--border-input); padding: 6px 10px;
  font-size: 0.82em; text-align: left;
}
.msg-content.md :deep(th) { background: var(--bg-input); font-weight: 600; }
.msg-content.md :deep(hr) {
  border: none; border-top: 1px solid var(--border-color); margin: 0.8em 0;
}
.msg-content.md :deep(a) { color: var(--accent); }
.msg-content.md :deep(strong) { font-weight: 600; }

.typing-cursor {
  display: inline; color: var(--accent);
  animation: blink 0.6s steps(1) infinite;
}
@keyframes blink { 50% { opacity: 0; } }

/* ---- bottom bar ---- */
.bottom-bar {
  padding: 8px 20px 14px;
  border-top: 1px solid var(--border-color);
  background: var(--bg-input-area);
}

.mode-selector {
  display: flex; gap: 4px; margin-bottom: 8px;
  padding: 0 4px;
}
.mode-pill {
  background: none; border: 1px solid transparent;
  color: var(--text-muted); padding: 4px 12px; border-radius: 8px;
  font-size: 0.75rem; cursor: pointer; transition: all 0.12s;
  white-space: nowrap;
}
.mode-pill:hover { color: var(--text-secondary); background: var(--bg-hover); }
.mode-pill.active {
  color: var(--accent); background: var(--accent-dim);
  border-color: var(--accent); opacity: 0.85;
}

.input-row {
  display: flex; gap: 10px; align-items: flex-end;
}
.input-row textarea {
  flex: 1; background: var(--bg-sidebar); border: 1px solid var(--border-input);
  border-radius: 10px; color: var(--text-primary); padding: 10px 14px;
  font-size: 0.88rem; outline: none; resize: none;
  font-family: inherit; line-height: 1.45;
  min-height: 44px; max-height: 120px;
}
.input-row textarea:focus { border-color: var(--border-focus); }

.send-btn {
  width: 42px; height: 42px; border-radius: 50%;
  background: var(--accent-dim); border: none; color: var(--accent-light);
  cursor: pointer; display: flex; align-items: center; justify-content: center;
  flex-shrink: 0; transition: background 0.12s;
}
.send-btn:hover { background: var(--accent); }
.send-btn:disabled { opacity: 0.35; cursor: not-allowed; }
</style>
