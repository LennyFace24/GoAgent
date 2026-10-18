<script lang="ts">
export default { name: 'ChatWorkspace' }
</script>

<script setup lang="ts">
import { ref, watch, nextTick, onMounted } from 'vue'
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
import { useModelSwitcher } from '../composables/useModelSwitcher'
import { useCommands } from '../composables/useCommands'

// ---------- Props & Emits ----------

const props = defineProps<{
  conversationId: string
}>()
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
// 从 useCommands 获取命令列表（含 builtin server/file-based/skill 命令）

const {
  commands: allCommands,
} = useCommands()

// 解析斜杠命令文本：/name arg1 arg2 → { name, args }
function parseCommand(text: string): { name: string; args: string[] } | null {
  const parts = text.trim().slice(1).split(/\s+/)
  const name = parts[0] || ''
  if (!name) return null
  return { name, args: parts.slice(1) }
}

// 执行 server 命令：调 /api/commands/execute，结果按类型处理
async function executeServerCommand(name: string, args: string[], raw: string): Promise<void> {
  // user 原文先进缓冲（与后端落库一致）
  messages.value.push({ id: Date.now(), role: 'user', content: raw })
  try {
    const res = await fetch('/api/commands/execute', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ name, args, conversation_id: conversationIdRef.value }),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      messages.value.push({
        id: Date.now(),
        role: 'assistant',
        content: `[命令错误 ${res.status}] ${err.error || '执行失败'}`,
      })
      return
    }
    const data = await res.json()
    if (data.type === 'file') {
      // file-based 命令：渲染文本作为普通消息发出（进管线）
      send(data.rendered)
    } else {
      // builtin 命令：结果直接展示
      messages.value.push({ id: Date.now(), role: 'assistant', content: data.result || '' })
    }
  } catch {
    messages.value.push({ id: Date.now(), role: 'assistant', content: '[命令执行失败，请重试]' })
  }
}

async function executeCommand(text: string): Promise<void> {
  const parsed = parseCommand(text)
  if (!parsed) return
  const { name, args } = parsed

  // 1. client 命令：本地处理
  if (name === 'clear') {
    messages.value = []
    return
  }
  if (name === 'help') {
    showHelp()
    return
  }
  if (name === 'model') {
    // /model → 弹出模型选择窗口
    showModelPicker()
    return
  }

  // 2. 已知命令（builtin server / file-based / skill）→ 后端执行
  const known = allCommands.value.find((c) => c.name === name)
  if (known) {
    await executeServerCommand(name, args, text.trim())
    return
  }

  // 3. 未知命令：原样发给后端（与 omp 一致，不拒绝）
  send(text.trim())
}

// /model 命令：弹出模型选择窗口
const modelPickerVisible = ref(false)
const modelPickerBusy = ref(false)

async function showModelPicker(): Promise<void> {
  await loadModels()
  modelPickerVisible.value = true
}

async function pickModel(id: string): Promise<void> {
  if (modelPickerBusy.value) return
  modelPickerBusy.value = true
  try {
    const ok = await switchModel(id)
    if (ok) {
      modelPickerVisible.value = false
      messages.value.push({
        id: Date.now(),
        role: 'assistant',
        content: `已切换到模型: \`${id}\``,
      })
    }
  } finally {
    modelPickerBusy.value = false
  }
}

function closeModelPicker(): void {
  modelPickerVisible.value = false
}

// handleSend：斜杠命令走 executeCommand，其余发后端
function handleSend(text: string): void {
  const trimmed = text.trim()
  if (!trimmed) return
  if (trimmed.startsWith('/')) {
    executeCommand(trimmed)
  } else {
    send(trimmed)
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
  let modelName = currentModel.value || '未知'
  if (!currentModel.value) {
    try {
      const res = await fetch('/api/model', { credentials: 'include' })
      if (res.ok) {
        const data = await res.json()
        modelName = data.current || modelName
      }
    } catch { /* ignore */ }
  }
  messages.value.push({
    id: Date.now(),
    role: 'assistant',
    content: `**当前模型**: \`${modelName}\`\n\n**当前模式**: ${chatMode.value === 'aiops' ? '运维诊断' : '自由对话'}`,
  })
}

// ---------- 模型切换 ----------

const {
  models,
  currentModel,
  switchError: modelSwitchError,
  loadModels,
  switchModel,
} = useModelSwitcher()

onMounted(loadModels)




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
        <!-- 模型选择器 -->
        <div class="model-selector">
          <select
            v-model="currentModel"
            @change="switchModel(($event.target as HTMLSelectElement).value)"
            title="切换模型"
          >
            <option v-for="m in models" :key="m.id" :value="m.id">{{ m.name }}</option>
          </select>
          <span v-if="modelSwitchError" class="model-switch-error">{{ modelSwitchError }}</span>
        </div>
        <ContextStatus />
        <button class="monitor-indicator" disabled title="实时系统状态常驻右侧">
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
			@respond="(approved: boolean, always: boolean, global: boolean) => respondPermission(msg, approved, always, global)"
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

    <!-- 模型选择模态窗口 -->
    <div v-if="modelPickerVisible" class="model-picker-overlay" @click.self="closeModelPicker">
      <div class="model-picker">
        <div class="model-picker-header">
          <span>选择模型</span>
          <button class="model-picker-close" @click="closeModelPicker">✕</button>
        </div>
        <div class="model-picker-list">
          <button
            v-for="m in models"
            :key="m.id"
            class="model-picker-item"
            :class="{ active: m.id === currentModel }"
            :disabled="modelPickerBusy"
            @click="pickModel(m.id)"
          >
            <span class="model-picker-name">{{ m.name }}</span>
            <span v-if="m.id === currentModel" class="model-picker-check">✓ 当前</span>
          </button>
        </div>
        <div v-if="modelSwitchError" class="model-picker-error">{{ modelSwitchError }}</div>
      </div>
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

.model-selector {
  display: flex;
  align-items: center;
  gap: 6px;
}

.model-selector select {
  background: var(--bg-input);
  border: 1px solid var(--border-input);
  border-radius: var(--radius-lg);
  color: var(--text-primary);
  font-size: 0.78rem;
  padding: 4px 8px;
  outline: none;
  max-width: 180px;
  cursor: pointer;
}

.model-selector select:focus {
  border-color: var(--border-focus);
}

.model-switch-error {
  color: var(--text-error);
  font-size: 0.72rem;
  white-space: nowrap;
}

.monitor-indicator {
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

.monitor-indicator:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
  transform: translateY(-1px);
  border-color: var(--border-input);
}

.monitor-indicator.active {
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

/* ---------- 模型选择模态窗口 ---------- */
.model-picker-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
}

.model-picker {
  background: var(--bg-secondary);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-lg);
  width: 360px;
  max-height: 70vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.model-picker-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border-hairline);
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-heading);
}

.model-picker-close {
  background: transparent;
  border: none;
  color: var(--text-secondary);
  font-size: 0.9rem;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: var(--radius-md);
}

.model-picker-close:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.model-picker-list {
  padding: 10px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.model-picker-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: var(--bg-input);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-lg);
  color: var(--text-primary);
  font-size: 0.82rem;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.model-picker-item:hover:not(:disabled) {
  border-color: var(--border-focus);
  background: var(--bg-hover);
}

.model-picker-item.active {
  border-color: var(--accent-bright);
  background: var(--accent-soft);
}

.model-picker-item:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.model-picker-check {
  color: var(--accent-bright);
  font-size: 0.72rem;
}

.model-picker-error {
  padding: 10px 18px;
  color: var(--text-error);
  font-size: 0.78rem;
  border-top: 1px solid var(--border-hairline);
}
</style>
