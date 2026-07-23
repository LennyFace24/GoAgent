<script lang="ts">
export default { name: 'ChatWorkspace' }
</script>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { Cpu, ScrollText, HardDrive, MessageCircle, Stethoscope, MonitorPlay } from 'lucide-vue-next'
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

const DEFAULT_WELCOME = '您好，我是您的智能运维助手。您可以输入问题进行分析，或点击右上角查看实时系统状态面板。'

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
      const stateLabel = data.percentage > 90 ? '危险' : data.percentage > 70 ? '警告' : '正常'
      messages.value.push({
        id: Date.now(),
        role: 'assistant',
        content: `**上下文状态**\n\n- Token 使用: ${data.current_tokens.toLocaleString()} / ${data.max_tokens.toLocaleString()} (${data.percentage.toFixed(1)}%)\n- 消息数量: ${data.message_count}\n- 状态: ${stateLabel}`,
      })
    }
  } catch { /* ignore */ }
}

function showHelp() {
  messages.value.push({
    id: Date.now(),
    role: 'assistant',
    content: `**可用命令**\n\n| 命令 | 说明 |\n|------|------|\n| /help | 显示此帮助信息 |\n| /clear | 清空当前对话 |\n| /compact | 触发上下文压缩 |\n| /context | 显示上下文状态 |\n| /model | 显示模型信息 |`,
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
    content: `**当前模型**: \`${modelName}\`\n\n**当前模式**: ${chatMode.value === 'aiops' ? '运维诊断' : '自由对话'}`,
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

// ---------- 快捷动作 & 模式 ----------

interface QuickAction {
  icon: typeof Cpu
  label: string
  prompt: string
}

const quickActions: QuickAction[] = [
  { icon: Cpu, label: '服务器 CPU/内存诊断', prompt: '查看当前服务器 CPU 和内存使用情况' },
  { icon: ScrollText, label: '查看最近错误日志', prompt: '列出最近 10 条系统错误日志' },
  { icon: HardDrive, label: 'df -h 磁盘水位查询', prompt: 'df -h 磁盘水位查询' },
]

interface ModeOption {
  key: 'chat' | 'aiops'
  icon: typeof MessageCircle
  label: string
}

const modeOptions: ModeOption[] = [
  { key: 'chat', icon: MessageCircle, label: '自由对话' },
  { key: 'aiops', icon: Stethoscope, label: '运维诊断' },
]
</script>

<template>
  <section class="chat-workspace">
    <!-- Header -->
    <header class="workspace-header">
      <div class="header-info">
        <span class="header-status-dot"></span>
        <span class="active-title">运维控制台</span>
        <span class="title-divider"></span>
        <span class="model-badge">GoAgent Copilot</span>
      </div>
      <div class="header-actions">
        <ContextStatus />
        <button
          class="monitor-toggle-btn"
          :class="{ active: isMonitorOpen }"
          @click="emit('update:isMonitorOpen', !isMonitorOpen)"
        >
          <MonitorPlay :size="14" :stroke-width="2" />
          <span>实时系统状态</span>
        </button>
      </div>
    </header>

    <!-- Messages -->
    <div class="message-area" ref="msgArea">
      <div class="message-stream">
        <div v-for="msg in messages" :key="msg.id" class="msg-item">
          <!-- user / assistant → MessageBubble -->
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
    </div>

    <!-- Input Section -->
    <div class="input-section">
      <!-- Quick Actions -->
      <div class="quick-actions">
        <button
          v-for="action in quickActions"
          :key="action.label"
          class="quick-btn"
          @click="triggerQuickAction(action.prompt)"
        >
          <component :is="action.icon" :size="13" :stroke-width="2" />
          <span>{{ action.label }}</span>
        </button>
      </div>

      <!-- Mode Switch -->
      <div class="mode-switch-bar">
        <button
          v-for="mode in modeOptions"
          :key="mode.key"
          class="mode-btn"
          :class="{ active: chatMode === mode.key }"
          @click="chatMode = mode.key"
        >
          <component :is="mode.icon" :size="13" :stroke-width="2" />
          <span>{{ mode.label }}</span>
        </button>
        <span class="mode-indicator" :class="{ right: chatMode === 'aiops' }"></span>
      </div>

      <!-- Input -->
      <InputBar
        :mode="chatMode"
        :sending="isSending"
        @send="handleSend"
        @stop="abort"
      />
    </div>
  </section>
</template>

<style scoped>
.chat-workspace {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: transparent;
  height: 100vh;
  position: relative;
  min-width: 0;
}

/* ---------- Header ---------- */
.workspace-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 28px;
  border-bottom: 1px solid var(--border-hairline);
  background: var(--bg-secondary);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  flex-shrink: 0;
}

.header-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--accent-bright);
  box-shadow: 0 0 10px var(--accent-glow);
  position: relative;
}

.active-title {
  font-family: var(--font-display);
  font-size: 1.02rem;
  font-weight: 600;
  letter-spacing: -0.02em;
  color: var(--text-heading);
}

.title-divider {
  width: 1px;
  height: 14px;
  background: var(--border-input);
}

.model-badge {
  font-family: var(--font-mono);
  font-size: 0.68rem;
  padding: 3px 9px;
  border-radius: var(--radius-pill);
  background: var(--bg-active);
  color: var(--text-secondary);
  letter-spacing: 0.02em;
  border: 1px solid var(--border-hairline);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.monitor-toggle-btn {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 7px 14px;
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-pill);
  background: var(--bg-elevated);
  backdrop-filter: var(--glass-blur-lite);
  -webkit-backdrop-filter: var(--glass-blur-lite);
  color: var(--text-secondary);
  font-size: 0.76rem;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-spring);
}

.monitor-toggle-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
  transform: translateY(-1px);
  border-color: var(--border-input);
}

.monitor-toggle-btn.active {
  background: var(--accent-soft);
  color: var(--accent-bright);
  border-color: var(--border-focus);
  box-shadow: 0 0 16px rgba(0, 255, 157, 0.15);
}

/* ---------- Message Area ---------- */
.message-area {
  flex: 1;
  overflow-y: auto;
  padding: 28px;
}

.message-stream {
  max-width: 920px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.msg-item {
  animation: msg-enter 0.42s var(--ease-out-expo) both;
}

@keyframes msg-enter {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* ---------- Input Section ---------- */
.input-section {
  padding: 14px 28px 22px;
  background: var(--bg-secondary);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  border-top: 1px solid var(--border-hairline);
  flex-shrink: 0;
}

.quick-actions {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
  max-width: 920px;
  margin-left: auto;
  margin-right: auto;
}

.quick-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 13px;
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-pill);
  background: var(--bg-elevated);
  color: var(--text-secondary);
  font-size: 0.74rem;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-spring);
}

.quick-btn:hover {
  background: var(--accent-soft);
  color: var(--accent-bright);
  border-color: var(--border-focus);
  transform: translateY(-2px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
}

.quick-btn:active {
  transform: translateY(0);
}

/* ---------- Mode Switch ---------- */
.mode-switch-bar {
  display: flex;
  gap: 2px;
  margin: 0 auto 12px;
  background: var(--bg-input);
  padding: 4px;
  border-radius: var(--radius-pill);
  position: relative;
  max-width: 920px;
  border: 1px solid var(--border-hairline);
}

.mode-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 12px;
  border: none;
  border-radius: var(--radius-pill);
  background: transparent;
  color: var(--text-secondary);
  font-size: 0.78rem;
  font-weight: 500;
  cursor: pointer;
  transition: color var(--transition-normal);
  position: relative;
  z-index: 2;
}

.mode-btn.active {
  color: var(--text-heading);
}

.mode-indicator {
  position: absolute;
  top: 4px;
  left: 4px;
  width: calc(50% - 4px);
  height: calc(100% - 8px);
  background: var(--accent-gradient);
  border-radius: var(--radius-pill);
  box-shadow: 0 2px 12px var(--accent-glow);
  transition: transform var(--transition-spring);
  z-index: 1;
}

.mode-indicator.right {
  transform: translateX(100%);
}
</style>
