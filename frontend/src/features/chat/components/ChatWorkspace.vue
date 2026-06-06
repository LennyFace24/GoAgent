<script lang="ts">
export default { name: 'ChatWorkspace' }
</script>

<script setup lang="ts">
import { ref, watch, onMounted, onActivated, nextTick } from 'vue'
import ContextStatus from './ContextStatus.vue'
import CommandPalette from './CommandPalette.vue'
import { useCommands, type Command } from '../composables/useCommands'
import { useChat } from '../composables/useChat'
import { Wrench, CheckCircle, Lock, ChevronUp, ChevronDown } from 'lucide-vue-next'

// ---------- Props & Emits ----------

const props = defineProps<{
  conversationId: string
  isMonitorOpen: boolean
}>()

const emit = defineEmits<{
  'update:isMonitorOpen': [value: boolean]
}>()

// ---------- 状态 ----------

const messages = ref<any[]>([])
const inputText = ref('')
const msgArea = ref<HTMLElement | null>(null)
const chatMode = ref<'chat' | 'aiops'>('aiops')

// ---------- Composables ----------

const { isSending, send, abort, loadHistory } = useChat({
  messages,
  conversationId: ref(props.conversationId),
  chatMode,
  scrollToBottom,
})

const {
  visible: paletteVisible,
  selectedIndex: commandSelectedIndex,
  filteredCommands,
  updateFromInput,
  onKeydown: onCommandKeydown,
  selectCommand,
} = useCommands()

// ---------- 监听 ----------

watch(inputText, (val) => {
  updateFromInput(val)
})

watch(() => props.conversationId, () => {
  abort()
  loadHistory(DEFAULT_WELCOME)
})

// ---------- 工具函数 ----------

const DEFAULT_WELCOME = '您好，我是您的智能运维助手！您可以输入问题进行分析，或者点击右上角查看 🖥️ 实时系统状态 面板。'

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

// ---------- 命令处理 ----------

function onCommandSelect(cmd: Command): void {
  const name = selectCommand(cmd)
  executeCommand(name)
}

function executeCommand(name: string): void {
  switch (name) {
    case 'clear':
      messages.value = []
      inputText.value = ''
      break
    case 'new':
      inputText.value = ''
      send('/new')
      break
    case 'compact':
      inputText.value = ''
      send('/compact')
      break
    case 'context':
      showContextStatus()
      inputText.value = ''
      break
    case 'help':
      showHelp()
      inputText.value = ''
      break
    case 'model':
      showModelInfo()
      inputText.value = ''
      break
    default:
      inputText.value = `/${name} `
      break
  }
}

// ---------- 内置命令 ----------

async function showContextStatus() {
  try {
    const res = await fetch('/context')
    if (res.ok) {
      const data = await res.json()
      messages.value.push({
        id: Date.now(),
        role: 'assistant',
        content: `📊 **上下文状态**\n\n- Token 使用: ${data.current_tokens.toLocaleString()} / ${data.max_tokens.toLocaleString()} (${data.percentage.toFixed(1)}%)\n- 消息数量: ${data.message_count}\n- 状态: ${data.percentage > 90 ? '🔴 危险' : data.percentage > 70 ? '🟡 警告' : '🟢 正常'}`,
      })
    }
  } catch { /* ignore */ }
}

function showHelp() {
  messages.value.push({
    id: Date.now(),
    role: 'assistant',
    content: `📖 **可用命令**

| 命令 | 说明 |
|------|------|
| /help | 显示此帮助信息 |
| /clear | 清空当前对话 |
| /new | 新建对话 |
| /compact | 触发上下文压缩 |
| /context | 显示上下文状态 |
| /model | 显示模型信息 |

输入 \`/\` 可快速选择命令。`,
  })
}

function showModelInfo() {
  messages.value.push({
    id: Date.now(),
    role: 'assistant',
    content: `🤖 **当前模式**: ${chatMode.value === 'aiops' ? '运维诊断' : '自由对话'}\n\n模型信息可通过系统监控面板查看。`,
  })
}

// ---------- 输入处理 ----------

function onInputKeydown(e: KeyboardEvent): void {
  if (paletteVisible.value) {
    const handled = onCommandKeydown(e)
    if (handled) return
  }
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    handleSend()
  }
}

function handleSend(): void {
  const text = inputText.value.trim()
  if (!text) return
  inputText.value = ''
  send(text)
}

// ---------- 权限响应 ----------

async function respondPermission(msg: any, approved: boolean) {
  if (msg.role !== 'permission_req' || !msg.requestId) return
  msg.responded = true
  try {
    await fetch('/api/permission/response', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: msg.requestId, approved }),
    })
  } catch { /* ignore */ }
}

// ---------- 工具折叠 ----------

const expandedTools = ref<Record<number, boolean>>({})
function toggleTool(index: number) {
  expandedTools.value[index] = !expandedTools.value[index]
}

// ---------- 生命周期 ----------

onMounted(() => {
  loadHistory(DEFAULT_WELCOME)
})

onActivated(() => {
  loadHistory(DEFAULT_WELCOME)
})
</script>

<template>
  <div class="chat-workspace">
    <!-- Header -->
    <div class="workspace-header">
      <div class="header-info">
        <span class="active-title">运维控制台</span>
        <span class="model-badge">GoAgent Copilot</span>
      </div>
      <div class="header-actions">
        <ContextStatus />
        <button
          class="monitor-toggle-btn"
          :class="{ active: isMonitorOpen }"
          @click="emit('update:isMonitorOpen', !isMonitorOpen)"
        >
          实时系统状态
        </button>
      </div>
    </div>

    <!-- Messages -->
    <div class="message-area" ref="msgArea">
      <div v-for="(msg, index) in messages" :key="msg.id" class="message-row">
        <!-- User Message -->
        <div v-if="msg.role === 'user'" class="bubble user">
          {{ msg.content }}
        </div>

        <!-- Assistant Message -->
        <div v-else-if="msg.role === 'assistant'" class="bubble assistant">
          <div v-if="msg.isThinking" class="thinking-loader">
            <span class="loader-dot"></span>
            <span class="loader-dot"></span>
            <span class="loader-dot"></span>
          </div>
          <div v-else-if="msg.isGenerating || msg.content" class="markdown-body">
            {{ msg.content }}
            <span v-if="msg.isGenerating" class="blinking-cursor">|</span>
          </div>
        </div>

        <!-- Tool Call -->
        <div v-else-if="msg.role === 'tool_call'" class="toolchain-block">
          <div class="toolchain-header" @click="toggleTool(index)">
            <Wrench :size="14" class="tool-status-icon" />
            <span class="tool-summary">AI 正在调度工具: <strong>{{ msg.name }}</strong></span>
            <component :is="expandedTools[index] ? ChevronUp : ChevronDown" :size="14" class="accordion-arrow" />
          </div>
          <div v-if="expandedTools[index]" class="toolchain-detail">
            <pre class="code-pre">参数: {{ msg.args }}</pre>
          </div>
        </div>

        <!-- Tool Result -->
        <div v-else-if="msg.role === 'tool_result'" class="toolchain-block result">
          <div class="toolchain-header" @click="toggleTool(index)">
            <CheckCircle :size="14" class="tool-status-icon success" />
            <span class="tool-summary">工具 <strong>{{ msg.name }}</strong> 执行完成</span>
            <component :is="expandedTools[index] ? ChevronUp : ChevronDown" :size="14" class="accordion-arrow" />
          </div>
          <div v-if="expandedTools[index]" class="toolchain-detail">
            <pre class="code-pre">{{ msg.result }}</pre>
          </div>
        </div>

        <!-- Permission Request -->
        <div v-else-if="msg.role === 'permission_req'" class="toolchain-block permission">
          <div class="toolchain-header">
            <Lock :size="14" class="tool-status-icon" />
            <span class="tool-summary">权限请求: <strong>{{ msg.name }}</strong></span>
          </div>
          <div class="toolchain-detail">
            <p class="perm-reason">{{ msg.reason }}</p>
            <div class="perm-actions" v-if="!msg.responded">
              <button class="perm-btn deny" @click="respondPermission(msg, false)">拒绝</button>
              <button class="perm-btn allow" @click="respondPermission(msg, true)">允许</button>
            </div>
            <p v-else class="perm-status">已响应</p>
          </div>
        </div>
      </div>
    </div>

    <!-- Quick Actions -->
    <div class="input-section">
      <div class="quick-actions">
        <button class="quick-btn" @click="triggerQuickAction('查看当前服务器 CPU 和内存使用情况')">
          🖥️ 服务器 CPU/内存诊断
        </button>
        <button class="quick-btn" @click="triggerQuickAction('列出最近 10 条系统错误日志')">
          📋 查看最近错误日志
        </button>
        <button class="quick-btn" @click="triggerQuickAction('df -h 磁盘水位查询')">
          💾 df -h 磁盘水位查询
        </button>
      </div>

      <!-- Mode Switch -->
      <div class="mode-switch-bar">
        <button
          class="mode-btn"
          :class="{ active: chatMode === 'chat' }"
          @click="chatMode = 'chat'"
        >💬 自由对话</button>
        <button
          class="mode-btn"
          :class="{ active: chatMode === 'aiops' }"
          @click="chatMode = 'aiops'"
        >🩺 运维诊断</button>
      </div>

      <!-- Input -->
      <div class="input-container">
        <div class="input-wrapper">
          <CommandPalette
            :commands="filteredCommands"
            :selectedIndex="commandSelectedIndex"
            :visible="paletteVisible"
            @select="onCommandSelect"
          />
          <textarea
            v-model="inputText"
            @keydown="onInputKeydown"
            placeholder="向 AI 助理提问或下达运维诊断指令... 输入 / 查看可用命令"
            class="smart-textarea"
            rows="1"
          ></textarea>
        </div>
        <button class="send-btn" :disabled="!inputText.trim() || isSending" @click="handleSend">
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

/* Header */
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
  padding: 2px 8px;
  border-radius: 6px;
  color: var(--text-muted);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
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

.monitor-toggle-btn:hover,
.monitor-toggle-btn.active {
  background: var(--accent);
  color: white;
  border-color: var(--accent);
}

/* Messages */
.message-area {
  flex: 1;
  overflow-y: auto;
  padding: 24px 32px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.message-row {
  display: flex;
  flex-direction: column;
}

.bubble {
  max-width: 85%;
  padding: 12px 16px;
  border-radius: 12px;
  font-size: 0.85rem;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.bubble.user {
  align-self: flex-end;
  background: var(--accent);
  color: white;
  border-bottom-right-radius: 4px;
}

.bubble.assistant {
  align-self: flex-start;
  background: var(--bg-secondary);
  color: var(--text-primary);
  border-bottom-left-radius: 4px;
  border: 1px solid var(--border-color);
}

/* Thinking Loader */
.thinking-loader {
  display: flex;
  gap: 6px;
  padding: 4px 0;
}

.loader-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--text-muted);
  animation: bounce 1.4s infinite ease-in-out;
}

.loader-dot:nth-child(1) { animation-delay: -0.32s; }
.loader-dot:nth-child(2) { animation-delay: -0.16s; }

@keyframes bounce {
  0%, 80%, 100% { transform: scale(0); }
  40% { transform: scale(1); }
}

.blinking-cursor {
  animation: blink 1s infinite;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}

/* Tool Blocks */
.toolchain-block {
  align-self: flex-start;
  max-width: 85%;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 10px;
  overflow: hidden;
}

.toolchain-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  cursor: pointer;
  transition: background var(--transition-fast);
}

.toolchain-header:hover {
  background: var(--bg-hover);
}

.tool-status-icon {
  font-size: 0.9rem;
}

.tool-summary {
  font-size: 0.78rem;
  color: var(--text-secondary);
}

.tool-summary strong {
  color: var(--accent);
}

.accordion-arrow {
  margin-left: auto;
  font-size: 0.7rem;
  color: var(--text-muted);
}

.toolchain-detail {
  padding: 0 14px 12px;
  border-top: 1px solid var(--border-color);
}

.code-pre {
  background: var(--bg-primary);
  padding: 10px;
  border-radius: 6px;
  font-size: 0.75rem;
  overflow-x: auto;
  color: var(--text-secondary);
  margin: 8px 0 0;
}

/* Permission */
.perm-reason {
  font-size: 0.8rem;
  color: var(--text-secondary);
  margin: 8px 0;
}

.perm-actions {
  display: flex;
  gap: 8px;
}

.perm-btn {
  padding: 6px 16px;
  border-radius: 6px;
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
  transition: var(--transition-fast);
}

.perm-btn.deny {
  background: var(--bg-input);
  border: 1px solid var(--border-color);
  color: var(--text-secondary);
}

.perm-btn.allow {
  background: var(--accent);
  border: 1px solid var(--accent);
  color: white;
}

.perm-status {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin: 8px 0 0;
}

/* Input Section */
.input-section {
  padding: 10px 32px 16px;
  border-top: 1px solid var(--border-color);
}

.quick-actions {
  display: flex;
  gap: 8px;
  margin-bottom: 10px;
  overflow-x: auto;
  padding-bottom: 4px;
}

.quick-btn {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  color: var(--text-secondary);
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 0.72rem;
  cursor: pointer;
  white-space: nowrap;
  transition: var(--transition-fast);
}

.quick-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.mode-switch-bar {
  display: flex;
  gap: 0;
  margin-bottom: 10px;
  border-bottom: 1px solid var(--border-color);
}

.mode-btn {
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  color: var(--text-muted);
  padding: 6px 14px 8px;
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.mode-btn:hover {
  color: var(--text-secondary);
}

.mode-btn.active {
  color: var(--text-primary);
  border-bottom-color: var(--accent);
}

.input-wrapper {
  flex: 1;
  position: relative;
}

.input-container {
  display: flex;
  gap: 8px;
  align-items: flex-end;
}

.smart-textarea {
  flex: 1;
  background: var(--bg-input);
  border: 1px solid var(--border-input);
  border-radius: 16px;
  color: var(--text-primary);
  padding: 10px 16px;
  font-size: 0.875rem;
  outline: none;
  resize: none;
  font-family: inherit;
  line-height: 1.5;
  min-height: 42px;
  max-height: 120px;
  transition: border-color var(--transition-fast);
}

.smart-textarea:focus {
  border-color: var(--border-focus);
}

.smart-textarea::placeholder {
  color: var(--text-muted);
}

.send-btn {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  background: var(--accent);
  border: none;
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: opacity var(--transition-fast);
}

.send-btn:hover {
  opacity: 0.85;
}

.send-btn:disabled {
  opacity: 0.25;
  cursor: not-allowed;
}
</style>
