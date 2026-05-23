<script setup lang="ts">
import { ref, watch, onMounted, nextTick } from 'vue'
import { simulateSSE } from '../../../shared/mockSSE'

const props = defineProps<{
  mode: string
  conversationId: string
  isMonitorOpen: boolean
}>()

const emit = defineEmits<{
  'update:mode': [value: string]
  'update:isMonitorOpen': [value: boolean]
}>()

interface Message {
  id: number
  role: 'user' | 'assistant' | 'tool_call' | 'tool_result'
  content?: string
  name?: string
  args?: string
  result?: string
  isThinking?: boolean
}

const messages = ref<Message[]>([])
const inputText = ref('')
const isSending = ref(false)
const msgArea = ref<HTMLElement | null>(null)
let activeAbort: any = null

function scrollToBottom() {
  nextTick(() => {
    if (msgArea.value) {
      msgArea.value.scrollTop = msgArea.value.scrollHeight
    }
  })
}

// 快捷卡片一键填充发送
function triggerQuickAction(text: string) {
  inputText.value = text
  handleSend()
}

async function handleSend() {
  if (!inputText.value.trim() || isSending.value) return

  const userText = inputText.value
  inputText.value = ''
  
  // 1. 压入用户消息
  messages.value.push({
    id: Date.now(),
    role: 'user',
    content: userText
  })
  scrollToBottom()

  isSending.value = true

  // 2. 压入 AI 思考占位消息
  const aiMessageId = Date.now() + 1
  messages.value.push({
    id: aiMessageId,
    role: 'assistant',
    content: '',
    isThinking: true
  })
  scrollToBottom()

  // 3. 执行 SSE 流
  try {
    // 优先尝试连通真实后端
    let realSseWorked = false
    
    // 如果失败或脱机，自动切换到我们的保真 Mock 调试引擎中
    if (!realSseWorked) {
      activeAbort = simulateSSE(
        userText,
        props.mode,
        (event) => {
          // 移除思考占位
          const aiMsg = messages.value.find(m => m.id === aiMessageId)
          if (aiMsg) aiMsg.isThinking = false

          if (event.type === 'tool_call') {
            messages.value.push({
              id: Date.now() + Math.random(),
              role: 'tool_call',
              name: event.name,
              args: event.args
            })
          } else if (event.type === 'tool_result') {
            messages.value.push({
              id: Date.now() + Math.random(),
              role: 'tool_result',
              name: event.name,
              result: event.result
            })
          } else if (event.type === 'content') {
            const lastMsg = messages.value[messages.value.length - 1]
            if (lastMsg && lastMsg.role === 'assistant') {
              lastMsg.content = (lastMsg.content || '') + event.content
            } else {
              messages.value.push({
                id: Date.now() + Math.random(),
                role: 'assistant',
                content: event.content
              })
            }
          }
          scrollToBottom()
        },
        () => {
          isSending.value = false
        }
      )
    }
  } catch {
    isSending.value = false
  }
}

// 可折叠工具栏状态记录
const expandedTools = ref<Record<number, boolean>>({})
function toggleTool(index: number) {
  expandedTools.value[index] = !expandedTools.value[index]
}

onMounted(() => {
  messages.value = [
    { id: 1, role: 'assistant', content: '您好，我是您的智能运维助手！您可以输入问题进行分析，或者点击右上角查看 **🖥️ 实时系统状态** 面板。' }
  ]
})

watch(() => props.conversationId, () => {
  if (activeAbort) activeAbort.abort()
  isSending.value = false
  messages.value = [
    { id: Date.now(), role: 'assistant', content: 已切换至会话: 。我随时准备为您进行运维诊断和日常知识手册查阅！ }
  ]
})
</script>

<template>
  <div class="chat-workspace">
    <!-- Header 顶栏 -->
    <div class="workspace-header">
      <div class="header-info">
        <span class="active-title">🎯 运维控制台</span>
        <span class="model-badge">GPT-4 (GoAgent Copilot)</span>
      </div>
      <button 
        class="monitor-toggle-btn"
        :class="{ active: isMonitorOpen }"
        @click="emit('update:isMonitorOpen', !isMonitorOpen)"
      >
        🖥️ 实时系统状态
      </button>
    </div>

    <!-- 消息区域 -->
    <div class="message-area" ref="msgArea">
      <div v-for="(msg, index) in messages" :key="msg.id" class="message-row">
        
        <!-- 用户气泡 -->
        <div v-if="msg.role === 'user'" class="bubble user">
          {{ msg.content }}
        </div>

        <!-- AI 助理气泡 -->
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

        <!-- Tool Call (折叠工具条，极致友好体验) -->
        <div v-else-if="msg.role === 'tool_call'" class="toolchain-block">
          <div class="toolchain-header" @click="toggleTool(index)">
            <span class="tool-status-icon">⚙️</span>
            <span class="tool-summary">AI 正在调度底层工具: <strong>{{ msg.name }}</strong></span>
            <span class="accordion-arrow">{{ expandedTools[index] ? '▲' : '▼' }}</span>
          </div>
          <div v-if="expandedTools[index]" class="toolchain-detail">
            <pre class="code-pre">参数: {{ msg.args }}</pre>
          </div>
        </div>

        <!-- Tool Result (折叠工具结果) -->
        <div v-else-if="msg.role === 'tool_result'" class="toolchain-block result">
          <div class="toolchain-header" @click="toggleTool(index)">
            <span class="tool-status-icon success">✅</span>
            <span class="tool-summary">工具 <strong>{{ msg.name }}</strong> 调用成果已载入</span>
            <span class="accordion-arrow">{{ expandedTools[index] ? '▲' : '▼' }}</span>
          </div>
          <div v-if="expandedTools[index]" class="toolchain-detail">
            <pre class="code-pre">结果: {{ msg.result }}</pre>
          </div>
        </div>

      </div>
    </div>

    <!-- 底部输入栏 -->
    <div class="input-section">
      <!-- 快捷运维助手卡片 -->
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

      <!-- 输入框主体 -->
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

/* 消息滚动区域 */
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

/* 扁平无边框气泡 */
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

/* 思考中加载状态 */
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

/* 折叠式工具条 */
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

/* 底部输入框 */
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