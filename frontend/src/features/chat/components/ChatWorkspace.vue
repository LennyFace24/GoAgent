<script lang="ts">
export default { name: 'ChatWorkspace' }
</script>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import ContextStatus from './ContextStatus.vue'
import InputBar from './InputBar.vue'
import ThinkingBlock from './ThinkingBlock.vue'
import ToolCallBlock from './ToolCallBlock.vue'
import ToolResultBlock from './ToolResultBlock.vue'
import PermissionCard from './PermissionCard.vue'
import MessageBubble from './MessageBubble.vue'
import LoadingCard from './LoadingCard.vue'

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

const msgArea = ref<HTMLElement | null>(null)
const chatMode = ref<'chat' | 'aiops'>('aiops')
const conversationIdRef = ref(props.conversationId)

watch(() => props.conversationId, (newId) => {
  conversationIdRef.value = newId
})

// ---------- Composables ----------

const DEFAULT_WELCOME = '您好，我是您的智能运维助手！您可以输入问题进行分析，或者点击右上角查看 🖥️ 实时系统状态 面板。'

const { messages, isSending, send, abort, respondPermission } = useChat({
  conversationId: conversationIdRef,
  chatMode,
  scrollToBottom,
  defaultWelcome: DEFAULT_WELCOME,
})

// 新消息自动滚底
watch(() => messages.value.length, () => {
  scrollToBottom()
})

// ---------- 工具函数 ----------

function scrollToBottom() {
  nextTick(() => {
    if (msgArea.value) msgArea.value.scrollTop = msgArea.value.scrollHeight
  })
}

function triggerQuickAction(text: string) {
  send(text)
}

// ---------- 命令处理 ----------

function executeCommand(name: string): void {
  switch (name) {
    case 'clear':
      messages.value = []
      break
    case 'compact':
      send('/compact')
      break
    case 'context':
      showContextStatus()
      break
    case 'help':
      showHelp()
      break
    case 'model':
      showModelInfo()
      break
    default:
      // 未识别的命令交给后端处理
      send(`/${name}`)
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
    content: `📖 **可用命令**\n\n| 命令 | 说明 |\n|------|------|\n| /help | 显示此帮助信息 |\n| /clear | 清空当前对话 |\n| /compact | 触发上下文压缩 |\n| /context | 显示上下文状态 |\n| /model | 显示模型信息 |`,
  })
}

async function showModelInfo() {
  let modelName = '未知'
  try {
    const res = await fetch('/api/model', { credentials: 'include' })
    if (res.ok) {
      const data = await res.json()
      modelName = data.model || modelName
    }
  } catch { /* ignore */ }
  messages.value.push({
    id: Date.now(),
    role: 'assistant',
    content: `🤖 **当前模型**: \`${modelName}\`\n\n**当前模式**: ${chatMode.value === 'aiops' ? '运维诊断' : '自由对话'}`,
  })
}

// ---------- 输入处理 ----------

// InputBar 发送：斜杠命令走本地副作用，其余发给后端
function handleSend(text: string): void {
  const trimmed = text.trim()
  if (!trimmed) return
  if (trimmed.startsWith('/')) {
    const name = trimmed.slice(1).split(/\s+/)[0]
    executeCommand(name)
  } else {
    send(trimmed)
  }
}

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
      <InputBar
        :mode="chatMode"
        :sending="isSending"
        @send="handleSend"
        @stop="abort"
      />
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
</style>
