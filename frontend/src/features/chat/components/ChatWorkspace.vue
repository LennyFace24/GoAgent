<script setup lang="ts">
import { ref, watch, onMounted, nextTick } from 'vue'

const props = defineProps<{
  mode: string
  conversationId: string
  isMonitorOpen: boolean
}>()

const emit = defineEmits<{
  'update:mode': [value: string]
  'update:isMonitorOpen': [value: boolean]
}>()

interface ToolCall {
  function: { name: string; arguments: string }
  id: string
}

interface ApiMessage {
  role: string
  content: string
  tool_calls?: ToolCall[]
  tool_name?: string
  tool_call_id?: string
}

interface Message {
  id: number
  role: 'user' | 'assistant' | 'tool_call' | 'tool_result' | 'permission_req'
  content?: string
  name?: string
  args?: string
  result?: string
  callId?: string
  requestId?: string
  reason?: string
  isThinking?: boolean
  responded?: boolean
}

const messages = ref<Message[]>([])
const inputText = ref('')
const isSending = ref(false)
const msgArea = ref<HTMLElement | null>(null)
let activeAbort: AbortController | null = null
let orderCounter = 0

function scrollToBottom() {
  nextTick(() => {
    if (msgArea.value) {
      msgArea.value.scrollTop = msgArea.value.scrollHeight
    }
  })
}

function triggerQuickAction(text: string) {
  inputText.value = text
  handleSend()
}

// 从后端加载对话历史
async function loadHistory() {
  messages.value = []
  orderCounter = 0
  try {
    const res = await fetch(`/conversation/${props.conversationId}`)
    if (!res.ok) {
      messages.value.push({
        id: Date.now(),
        role: 'assistant',
        content: '您好，我是您的智能运维助手！您可以输入问题进行分析，或者点击右上角查看 🖥️ 实时系统状态 面板。'
      })
      return
    }
    const data = await res.json()
    const lines: ApiMessage[] = data.messages || []
    for (const line of lines) {
      if (line.role === 'user') {
        messages.value.push({
          id: Date.now() + Math.random(),
          role: 'user',
          content: line.content,
        })
      } else if (line.role === 'assistant') {
        if (line.tool_calls) {
          for (const tc of line.tool_calls) {
            messages.value.push({
              id: Date.now() + Math.random(),
              role: 'tool_call',
              name: tc.function.name,
              args: tc.function.arguments,
              callId: tc.id,
            })
          }
        }
        if (line.content) {
          messages.value.push({
            id: Date.now() + Math.random(),
            role: 'assistant',
            content: line.content,
          })
        }
      } else if (line.role === 'tool') {
        messages.value.push({
          id: Date.now() + Math.random(),
          role: 'tool_result',
          name: line.tool_name || '',
          result: line.content,
          callId: line.tool_call_id || '',
        })
      }
    }
    if (messages.value.length === 0) {
      messages.value.push({
        id: Date.now(),
        role: 'assistant',
        content: '您好，我是您的智能运维助手！您可以输入问题进行分析，或者点击右上角查看 🖥️ 实时系统状态 面板。'
      })
    }
  } catch {
    messages.value.push({
      id: Date.now(),
      role: 'assistant',
      content: '您好，我是您的智能运维助手！您可以输入问题进行分析，或者点击右上角查看 🖥️ 实时系统状态 面板。'
    })
  }
}

async function handleSend() {
  if (!inputText.value.trim() || isSending.value) return

  const userText = inputText.value
  inputText.value = ''

  messages.value.push({
    id: Date.now(),
    role: 'user',
    content: userText
  })
  scrollToBottom()

  isSending.value = true

  const aiMessageId = Date.now() + 1
  messages.value.push({
    id: aiMessageId,
    role: 'assistant',
    content: '',
    isThinking: true
  })
  scrollToBottom()

  const abortCtrl = new AbortController()
  activeAbort = abortCtrl

  try {
    const endpoint = props.mode === 'aiops' ? '/ai_ops' : '/chat_stream'
    const res = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: userText, conversation_id: props.conversationId }),
      signal: abortCtrl.signal,
    })

    if (!res.ok) {
      const aiMsg = messages.value.find(m => m.id === aiMessageId)
      if (aiMsg) {
        aiMsg.isThinking = false
        aiMsg.content = `[HTTP ${res.status}] 请求失败`
      }
      return
    }

    const reader = res.body!.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      if (abortCtrl.signal.aborted) {
        reader.cancel()
        break
      }

      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      let currentEvent = ''
      for (let i = 0; i < lines.length; i++) {
        const line = lines[i]
        if (line.startsWith('event:')) {
          currentEvent = line.slice(6).trim()
          continue
        }
        if (!line.startsWith('data:')) continue
        const payload = line.slice(5).trim()
        if (!payload) continue

        if (currentEvent === 'delta') {
          try {
            const data = JSON.parse(payload)
            const aiMsg = messages.value.find(m => m.id === aiMessageId)
            if (aiMsg && data.content) {
              aiMsg.isThinking = false
              aiMsg.content = (aiMsg.content || '') + data.content
            }
          } catch { /* ignore */ }
          scrollToBottom()

        } else if (currentEvent === 'message') {
          try {
            const data = JSON.parse(payload)
            const aiMsg = messages.value.find(m => m.id === aiMessageId)
            if (aiMsg && data.content) {
              aiMsg.isThinking = false
              aiMsg.content = data.content
            }
          } catch { /* ignore */ }
          scrollToBottom()

        } else if (currentEvent === 'tool') {
          try {
            const ev = JSON.parse(payload)
            if (ev.type === 'tool_call') {
              const aiMsg = messages.value.find(m => m.id === aiMessageId)
              if (aiMsg) aiMsg.isThinking = false

              messages.value.push({
                id: Date.now() + Math.random(),
                role: 'tool_call',
                name: ev.name,
                args: ev.args,
                callId: ev.call_id,
              })
            } else if (ev.type === 'tool_result') {
              messages.value.push({
                id: Date.now() + Math.random(),
                role: 'tool_result',
                name: ev.name,
                result: ev.result,
                callId: ev.call_id,
              })
            } else if (ev.type === 'permission_request') {
              messages.value.push({
                id: Date.now() + Math.random(),
                role: 'permission_req',
                name: ev.name,
                args: ev.args,
                requestId: ev.request_id,
                reason: ev.reason,
                callId: ev.call_id,
                responded: false,
              })
            }
          } catch { /* ignore */ }
          scrollToBottom()

        } else if (currentEvent === 'done') {
          const aiMsg = messages.value.find(m => m.id === aiMessageId)
          if (aiMsg) aiMsg.isThinking = false
        } else if (currentEvent === 'error') {
          try {
            const data = JSON.parse(payload)
            const aiMsg = messages.value.find(m => m.id === aiMessageId)
            if (aiMsg) {
              aiMsg.isThinking = false
              aiMsg.content = (aiMsg.content || '') + `\n\n[错误: ${data.error}]`
            }
          } catch { /* ignore */ }
        }
      }
    }
  } catch (e) {
    if (abortCtrl.signal.aborted) return
    const aiMsg = messages.value.find(m => m.id === aiMessageId)
    if (aiMsg) {
      aiMsg.isThinking = false
      aiMsg.content = (aiMsg.content || '') + `\n\n[连接断开: ${e instanceof Error ? e.message : String(e)}]`
    }
  } finally {
    if (!abortCtrl.signal.aborted) {
      const aiMsg = messages.value.find(m => m.id === aiMessageId)
      if (aiMsg) aiMsg.isThinking = false
    }
    if (activeAbort === abortCtrl) {
      activeAbort = null
    }
    isSending.value = false
  }
}

async function respondPermission(msg: Message, approved: boolean) {
  if (msg.role !== 'permission_req' || !msg.requestId) return
  msg.responded = true
  try {
    await fetch('/permission/response', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: msg.requestId, approved }),
    })
  } catch { /* ignore */ }
}

const expandedTools = ref<Record<number, boolean>>({})
function toggleTool(index: number) {
  expandedTools.value[index] = !expandedTools.value[index]
}

onMounted(() => {
  loadHistory()
})

watch(() => props.conversationId, () => {
  if (activeAbort) activeAbort.abort()
  isSending.value = false
  loadHistory()
})
</script>

<template>
  <div class="chat-workspace">
    <div class="workspace-header">
      <div class="header-info">
        <span class="active-title">运维控制台</span>
        <span class="model-badge">GoAgent Copilot</span>
      </div>
      <button
        class="monitor-toggle-btn"
        :class="{ active: isMonitorOpen }"
        @click="emit('update:isMonitorOpen', !isMonitorOpen)"
      >
        实时系统状态
      </button>
    </div>

    <div class="message-area" ref="msgArea">
      <div v-for="(msg, index) in messages" :key="msg.id" class="message-row">

        <div v-if="msg.role === 'user'" class="bubble user">
          {{ msg.content }}
        </div>

        <div v-else-if="msg.role === 'assistant'" class="bubble assistant">
          <div v-if="msg.isThinking" class="thinking-loader">
            <span class="loader-dot"></span>
            <span class="loader-dot"></span>
            <span class="loader-dot"></span>
          </div>
          <div v-else class="markdown-body">
            {{ msg.content }}
            <span v-if="isSending && index === messages.length - 1" class="blinking-cursor">|</span>
          </div>
        </div>

        <div v-else-if="msg.role === 'tool_call'" class="toolchain-block">
          <div class="toolchain-header" @click="toggleTool(index)">
            <span class="tool-status-icon">⚙️</span>
            <span class="tool-summary">AI 正在调度工具: <strong>{{ msg.name }}</strong></span>
            <span class="accordion-arrow">{{ expandedTools[index] ? '▲' : '▼' }}</span>
          </div>
          <div v-if="expandedTools[index]" class="toolchain-detail">
            <pre class="code-pre">参数: {{ msg.args }}</pre>
          </div>
        </div>

        <div v-else-if="msg.role === 'tool_result'" class="toolchain-block result">
          <div class="toolchain-header" @click="toggleTool(index)">
            <span class="tool-status-icon success">✅</span>
            <span class="tool-summary">工具 <strong>{{ msg.name }}</strong> 执行完成</span>
            <span class="accordion-arrow">{{ expandedTools[index] ? '▲' : '▼' }}</span>
          </div>
          <div v-if="expandedTools[index]" class="toolchain-detail">
            <pre class="code-pre">结果: {{ msg.result }}</pre>
          </div>
        </div>

        <div v-else-if="msg.role === 'permission_req'" class="permission-card">
          <div class="perm-header">
            <span class="perm-icon">🔐</span>
            <span>工具 <strong>{{ msg.name }}</strong> 请求授权</span>
          </div>
          <div v-if="msg.reason" class="perm-reason">{{ msg.reason }}</div>
          <pre v-if="msg.args" class="code-pre perm-args">参数: {{ msg.args }}</pre>
          <div v-if="!msg.responded" class="perm-actions">
            <button class="perm-btn approve" @click="respondPermission(msg, true)">批准</button>
            <button class="perm-btn deny" @click="respondPermission(msg, false)">拒绝</button>
          </div>
          <div v-else class="perm-status">已{{ msg.approved ? '批准' : '拒绝' }}</div>
        </div>

      </div>
    </div>

    <div class="input-section">
      <div v-if="messages.length <= 1" class="quick-actions">
        <button class="quick-btn" @click="triggerQuickAction('🩺 一键全盘健康诊断')">
          🩺 一键全盘健康诊断
        </button>
        <button class="quick-btn" @click="triggerQuickAction('📚 检索知识库中的运维规范')">
          📚 检索知识库中的运维规范
        </button>
        <button class="quick-btn" @click="triggerQuickAction('df -h 磁盘水位查询')">
          💾 df -h 磁盘水位查询
        </button>
      </div>

      <div class="input-container">
        <textarea
          v-model="inputText"
          @keydown.enter.prevent="handleSend"
          placeholder="向 AI 助理提问或下达运维诊断指令... (Enter 发送)"
          class="smart-textarea"
          rows="1"
        ></textarea>
        <button class="send-btn" :disabled="isSending" @click="handleSend">
          {{ isSending ? '...' : '发送' }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chat-workspace {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: var(--bg-body);
  height: 100vh;
  position: relative;
}

.workspace-header {
  height: 60px;
  border-bottom: 1px solid var(--border-color);
  padding: 0 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-shrink: 0;
}

.header-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.active-title {
  font-weight: 700;
  font-size: 0.95rem;
  color: var(--text-heading);
}

.model-badge {
  font-size: 0.65rem;
  background: var(--bg-hover);
  border: 1px solid var(--border-color);
  color: var(--text-secondary);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-weight: 500;
}

.monitor-toggle-btn {
  background: var(--bg-input);
  border: 1px solid var(--border-color);
  color: var(--text-primary);
  padding: 6px 14px;
  border-radius: var(--radius-md);
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
  transition: var(--transition-fast);
}

.monitor-toggle-btn:hover, .monitor-toggle-btn.active {
  background: var(--accent);
  color: white;
  border-color: var(--accent);
}

.message-area {
  flex: 1;
  padding: 24px 40px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.message-row {
  display: flex;
  flex-direction: column;
}

.bubble {
  max-width: 80%;
  padding: 12px 18px;
  border-radius: var(--radius-lg);
  font-size: 0.88rem;
  line-height: 1.5;
  word-wrap: break-word;
}

.bubble.user {
  align-self: flex-end;
  background: var(--bg-chat-bubble-user);
  color: white;
  border-bottom-right-radius: 4px;
  box-shadow: var(--shadow-sm);
}

.bubble.assistant {
  align-self: flex-start;
  background: var(--bg-chat-bubble);
  color: var(--text-primary);
  border-bottom-left-radius: 4px;
  border: 1px solid var(--border-color);
  box-shadow: var(--shadow-sm);
}

.thinking-loader {
  display: flex;
  gap: 4px;
  padding: 4px 0;
}

.loader-dot {
  width: 6px; height: 6px;
  background: var(--text-muted);
  border-radius: 50%;
  animation: bounce 1.4s infinite ease-in-out both;
}

.loader-dot:nth-child(1) { animation-delay: -0.32s; }
.loader-dot:nth-child(2) { animation-delay: -0.16s; }

@keyframes bounce {
  0%, 80%, 100% { transform: scale(0); }
  40% { transform: scale(1); }
}

.blinking-cursor {
  font-weight: 700;
  color: var(--accent);
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  from, to { color: transparent }
  50% { color: var(--accent) }
}

.toolchain-block {
  align-self: flex-start;
  width: 100%;
  max-width: 600px;
  background: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  margin: 6px 0;
  overflow: hidden;
}

.toolchain-block.result {
  background: rgba(16, 163, 127, 0.02);
}

.toolchain-header {
  padding: 10px 14px;
  display: flex;
  align-items: center;
  cursor: pointer;
  user-select: none;
}

.tool-status-icon {
  margin-right: 8px;
  font-size: 1rem;
}

.tool-summary {
  font-size: 0.78rem;
  color: var(--text-secondary);
  flex: 1;
}

.tool-summary strong {
  color: var(--text-primary);
}

.accordion-arrow {
  font-size: 0.65rem;
  color: var(--text-muted);
}

.toolchain-detail {
  border-top: 1px solid var(--border-color);
  background: rgba(0,0,0,0.15);
  padding: 12px;
}

.code-pre {
  font-family: monospace;
  font-size: 0.75rem;
  color: var(--text-secondary);
  white-space: pre-wrap;
  margin: 0;
}

/* 权限确认卡片 */
.permission-card {
  align-self: flex-start;
  width: 100%;
  max-width: 500px;
  background: rgba(255, 193, 7, 0.06);
  border: 1px solid rgba(255, 193, 7, 0.3);
  border-radius: var(--radius-md);
  padding: 14px;
  margin: 6px 0;
}

.perm-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.82rem;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.perm-icon {
  font-size: 1.1rem;
}

.perm-reason {
  font-size: 0.75rem;
  color: var(--text-secondary);
  margin-bottom: 8px;
}

.perm-args {
  margin-bottom: 10px;
  font-size: 0.72rem;
  opacity: 0.8;
}

.perm-actions {
  display: flex;
  gap: 8px;
}

.perm-btn {
  padding: 6px 16px;
  border-radius: var(--radius-sm);
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
  border: none;
  transition: var(--transition-fast);
}

.perm-btn.approve {
  background: var(--accent);
  color: white;
}

.perm-btn.approve:hover {
  background: var(--accent-light);
}

.perm-btn.deny {
  background: transparent;
  color: var(--text-secondary);
  border: 1px solid var(--border-color);
}

.perm-btn.deny:hover {
  border-color: #e53e3e;
  color: #e53e3e;
}

.perm-status {
  font-size: 0.78rem;
  color: var(--text-muted);
}

.input-section {
  padding: 16px 40px 24px 40px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  flex-shrink: 0;
}

.quick-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.quick-btn {
  background: var(--bg-input);
  border: 1px solid var(--border-color);
  color: var(--text-secondary);
  padding: 6px 12px;
  border-radius: var(--radius-md);
  font-size: 0.72rem;
  font-weight: 500;
  cursor: pointer;
  transition: var(--transition-fast);
}

.quick-btn:hover {
  border-color: var(--accent);
  color: var(--text-primary);
}

.input-container {
  display: flex;
  background: var(--bg-input);
  border: 1px solid var(--border-input);
  border-radius: var(--radius-lg);
  padding: 8px 12px;
  align-items: center;
  box-shadow: var(--shadow-sm);
  transition: var(--transition-fast);
}

.input-container:focus-within {
  border-color: var(--accent);
  box-shadow: 0 0 10px rgba(16, 163, 127, 0.15);
}

.smart-textarea {
  flex: 1;
  background: transparent;
  border: none;
  resize: none;
  color: var(--text-primary);
  font-size: 0.85rem;
  padding: 8px 4px;
}

.smart-textarea:focus {
  outline: none;
}

.send-btn {
  background: var(--accent);
  color: white;
  border: none;
  padding: 8px 16px;
  border-radius: var(--radius-md);
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
  transition: var(--transition-fast);
}

.send-btn:hover {
  background: var(--accent-light);
}

.send-btn:disabled {
  background: var(--text-muted);
  cursor: not-allowed;
}
</style>
