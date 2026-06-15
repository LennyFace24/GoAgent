<script lang="ts">
export default { name: 'ChatWorkspace' }
</script>

<script setup lang="ts">
import { ref, watch, onMounted, onActivated, nextTick } from 'vue'
import type { Message } from '../../../shared/types'
import ContextStatus from './ContextStatus.vue'
import CommandPalette from './CommandPalette.vue'
import ThinkingBlock from './ThinkingBlock.vue'
import ToolCallBlock from './ToolCallBlock.vue'
import ToolResultBlock from './ToolResultBlock.vue'
import PermissionCard from './PermissionCard.vue'
import MessageBubble from './MessageBubble.vue'
import LoadingCard from './LoadingCard.vue'

import { useCommands, type Command } from '../composables/useCommands'
import { useChat } from '../composables/useChat'

// ---------- Props & Emits ----------

const props = defineProps<{
  conversationId: string
  isMonitorOpen: boolean
}>()

const emit = defineEmits<{
  'update:isMonitorOpen': [value: boolean]
}>()

// ---------- 状态 ----------

const messages = ref<Message[]>([])
const inputText = ref('')
const msgArea = ref<HTMLElement | null>(null)
const chatMode = ref<'chat' | 'aiops'>('aiops')
const conversationIdRef = ref(props.conversationId)

watch(() => props.conversationId, (newId) => {
  conversationIdRef.value = newId
})

// 新消息自动滚底
watch(() => messages.value.length, () => {
  scrollToBottom()
})

// ---------- Composables ----------

const DEFAULT_WELCOME = '您好，我是您的智能运维助手！您可以输入问题进行分析，或者点击右上角查看 🖥️ 实时系统状态 面板。'

const { isSending, send, abort, loadHistory, respondPermission } = useChat({
  messages,
  conversationId: conversationIdRef,
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

function scrollToBottom() {
  nextTick(() => {
    if (msgArea.value) msgArea.value.scrollTop = msgArea.value.scrollHeight
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
    const res = await fetch('/api/context/status', {
      credentials: 'include',
    })


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
    content: `📖 **可用命令**\n\n| 命令 | 说明 |\n|------|------|\n| /help | 显示此帮助信息 |\n| /clear | 清空当前对话 |\n| /new | 新建对话 |\n| /compact | 触发上下文压缩 |\n| /context | 显示上下文状态 |\n| /model | 显示模型信息 |`,
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

function handleSend() {
  const text = inputText.value.trim()
  if (!text) return
  inputText.value = ''
  send(text)
}

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

    <!-- Messages：每条消息按 role 渲染，user 右对齐，其余左对齐 -->
    <div class="message-area" ref="msgArea">
      <div
        v-for="msg in messages"
        :key="msg.id"
        class="msg-item"
      >



        <!-- user / assistant → MessageBubble（内部已有气泡区分）-->
        <MessageBubble
          v-if="msg.role === 'user' || msg.role === 'assistant'"
          :message="msg"
        />

        <!-- thinking → ThinkingBlock -->
        <ThinkingBlock
          v-else-if="msg.role === 'thinking'"
          :content="msg.content || ''"
        />

        <!-- tool_call → ToolCallBlock -->
        <ToolCallBlock
          v-else-if="msg.role === 'tool_call'"
          :name="msg.name"
          :args="msg.args"
        />

        <!-- tool_result → ToolResultBlock -->
        <ToolResultBlock
          v-else-if="msg.role === 'tool_result'"
          :name="msg.name"
          :result="msg.result"
        />

        <!-- permission_req → PermissionCard -->
        <PermissionCard
          v-else-if="msg.role === 'permission_req'"
          :message="msg"
          @respond="(approved: boolean, always: boolean) => respondPermission(msg, approved, always)"
        />
      </div>

      <!-- AI 思考中 Loading -->
      <LoadingCard v-if="isSending" />
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
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border-color);
  background: var(--bg-secondary);
}

.header-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.active-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.model-badge {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 10px;
  background: var(--bg-active);
  color: var(--text-secondary);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.monitor-toggle-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-secondary);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.monitor-toggle-btn:hover {
  background: var(--bg-hover);
}

.monitor-toggle-btn.active {
  background: var(--accent);
  color: white;
  border-color: var(--accent);
}

/* Message Area */
.message-area {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}

/* 每条消息的外层容器 */
.msg-item {
  margin-bottom: 12px;
}

/* Input Section */
.input-section {
  padding: 16px 20px;
  border-top: 1px solid var(--border-color);
  background: var(--bg-secondary);
}

.quick-actions {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.quick-btn {
  padding: 6px 12px;
  border: 1px solid var(--border-color);
  border-radius: 16px;
  background: var(--bg-secondary);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.quick-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.mode-switch-bar {
  display: flex;
  gap: 4px;
  margin-bottom: 12px;
  background: var(--bg-body);
  padding: 4px;
  border-radius: 8px;
}

.mode-btn {
  flex: 1;
  padding: 6px 12px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.mode-btn:hover {
  background: var(--bg-hover);
}

.mode-btn.active {
  background: var(--accent);
  color: #fff;
  font-weight: 600;
  box-shadow: 0 1px 4px rgba(0,0,0,0.15);
}

.input-container {
  display: flex;
  gap: 8px;
}

.input-wrapper {
  flex: 1;
  position: relative;
}

.smart-textarea {
  width: 100%;
  padding: 10px 14px;
  border: 1px solid var(--border-input);
  border-radius: 8px;
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 14px;
  resize: none;
  outline: none;
  transition: border-color 0.2s;
}

.smart-textarea:focus {
  border-color: var(--accent);
}

.smart-textarea::placeholder {
  color: var(--text-secondary);
}

.send-btn {
  padding: 10px 20px;
  border: none;
  border-radius: 8px;
  background: var(--accent);
  color: white;
  font-size: 14px;
  cursor: pointer;
  transition: opacity 0.2s;
}

.send-btn:hover:not(:disabled) {
  opacity: 0.9;
}

.send-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
