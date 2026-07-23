<script setup lang="ts">
import { ref, watch } from 'vue'
import { Send, Square } from 'lucide-vue-next'
import CommandPalette from './CommandPalette.vue'
import { useCommands, type Command } from '../composables/useCommands'

// ---------- 输入栏组件（纯输入：只负责文本输入、命令面板、发送） ----------

const props = defineProps<{
  mode: string
  sending: boolean
}>()

const emit = defineEmits<{
  send: [text: string]
  stop: []
}>()

const input = ref<string>('')
const isComposing = ref(false) // 标记是否正在进行 IME 组合输入

const placeholders: Record<string, string> = {
  chat: '向 AI 助理提问或下达运维诊断指令... 输入 / 查看可用命令',
  aiops: '描述故障现象，如：API 响应时间从 200ms 飙升到 3s，错误率 12%',
}

// 斜杠命令
const {
  visible: paletteVisible,
  selectedIndex,
  filteredCommands,
  updateFromInput,
  onKeydown: onCommandKeydown,
  selectCommand,
} = useCommands()

// 监听输入变化，更新命令面板
watch(input, (val) => {
  updateFromInput(val)
})

// IME 组合输入事件
function onCompositionStart(): void {
  isComposing.value = true
}

function onCompositionEnd(e: CompositionEvent): void {
  isComposing.value = false
  // 组合结束后，手动同步 input 的值（某些 IME 不会自动触发 input 事件）
  input.value = (e.target as HTMLTextAreaElement).value
}

// 选中命令：填入输入框供用户确认后再发送，不立即执行
function pickCommand(cmd: Command): void {
  input.value = `/${cmd.name} `
  selectCommand(cmd)
}

// 键盘事件处理
function onKeydown(e: KeyboardEvent): void {
  // 命令面板可见时
  if (paletteVisible.value) {
    // Enter：把选中的命令填入输入框（供确认），不直接发送
    if (e.key === 'Enter' && !e.shiftKey) {
      const cmd = filteredCommands.value[selectedIndex.value]
      if (cmd) {
        e.preventDefault()
        pickCommand(cmd)
        return
      }
    }
    // 其余导航键（↑↓ Esc）交给命令面板
    const handled = onCommandKeydown(e)
    if (handled) return
  }

  // Enter 发送（IME 组合输入期间跳过，避免误发送）
  if (e.key === 'Enter' && !e.shiftKey && !isComposing.value) {
    e.preventDefault()
    doSend()
  }
}

function doSend(): void {
  const text = input.value.trim()
  if (!text) return
  input.value = ''
  emit('send', text)
}

// 按钮点击：发送中 → 停止；否则 → 发送
function onButtonClick(): void {
  if (props.sending) {
    emit('stop')
  } else {
    doSend()
  }
}
</script>

<template>
  <div class="input-bar">
    <div class="input-wrapper">
      <CommandPalette
        :commands="filteredCommands"
        v-model:selectedIndex="selectedIndex"
        :visible="paletteVisible"
        @select="pickCommand"
      />
      <div class="input-row">
        <textarea
          v-model="input"
          :placeholder="placeholders[mode] || '输入消息...'"
          rows="1"
          :disabled="sending"
          @keydown="onKeydown"
          @compositionstart="onCompositionStart"
          @compositionend="onCompositionEnd"
        ></textarea>
        <button
          class="send-btn"
          :class="{ 'is-stop': sending }"
          :disabled="!sending && !input.trim()"
          @click="onButtonClick"
          :title="sending ? '停止生成' : '发送'"
        >
          <Square v-if="sending" :size="15" :stroke-width="2.4" fill="currentColor" />
          <Send v-else :size="16" :stroke-width="2.2" />
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.input-bar {
  display: flex;
  max-width: 920px;
  margin: 0 auto;
}

.input-wrapper {
  flex: 1;
  position: relative;
}

.input-row {
  display: flex;
  gap: 10px;
  align-items: flex-end;
  padding: 6px;
  background: var(--bg-input);
  border: 1px solid var(--border-input);
  border-radius: var(--radius-xl);
  backdrop-filter: var(--glass-blur-lite);
  -webkit-backdrop-filter: var(--glass-blur-lite);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  box-shadow: var(--shadow-sm);
}

.input-row:focus-within {
  border-color: var(--border-focus);
  box-shadow: 0 0 0 4px var(--accent-dim), var(--shadow-md);
}

.input-row textarea {
  flex: 1;
  background: transparent;
  border: none;
  color: var(--text-primary);
  padding: 10px 12px;
  font-size: 0.88rem;
  font-family: var(--font-sans);
  letter-spacing: -0.005em;
  outline: none;
  resize: none;
  line-height: 1.55;
  min-height: 30px;
  max-height: 140px;
}

.input-row textarea::placeholder {
  color: var(--text-muted);
}

.send-btn {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-lg);
  background: var(--accent-gradient);
  border: none;
  color: #07070b;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: transform var(--transition-spring), opacity var(--transition-fast),
    box-shadow var(--transition-fast);
  box-shadow: 0 4px 16px var(--accent-glow);
}

.send-btn:hover:not(:disabled) {
  transform: translateY(-2px) scale(1.04);
  box-shadow: 0 6px 22px var(--accent-glow);
}

.send-btn:active:not(:disabled) {
  transform: translateY(0) scale(0.96);
}

.send-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  box-shadow: none;
}

.send-btn.is-stop {
  background: linear-gradient(135deg, #ff5a6e, #e53e3e);
  color: #fff;
  box-shadow: 0 4px 16px rgba(255, 90, 110, 0.4);
}
</style>
